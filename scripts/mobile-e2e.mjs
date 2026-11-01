// Responsive layout and keyboard/touch workflow checks. Run through jobgate.
// --baseline-ref freezes an existing Git revision into ignored bin/e2e artifacts.
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
const allowed = new Set(["--url", "--html", "--output", "--baseline-ref", "--compare", "--browser", "--channel"]);
for (let i = 0; i < args.length; i += 2) {
  assert.ok(allowed.has(args[i]) && args[i + 1] && !args[i + 1].startsWith("--"), `Unknown or incomplete option: ${args[i]}`);
}
const option = (name, fallback) => args.includes(name) ? args[args.indexOf(name) + 1] : fallback;
const output = path.resolve(root, option("--output", `bin/e2e/mobile-${Date.now()}`));
assert.ok(output.startsWith(path.join(root, "bin/e2e") + path.sep), "Artifacts must stay below bin/e2e");
await fs.mkdir(output, { recursive: true });
const baselineRef = option("--baseline-ref");
let html = path.resolve(root, option("--html", "index.html"));
if (baselineRef) {
  assert.match(baselineRef, /^[0-9a-f]{7,40}$/i, "Baseline must be an explicit Git commit ID");
  html = path.join(output, "baseline.html");
  await fs.writeFile(html, execFileSync("git", ["show", `${baselineRef}:index.html`], { cwd: root, maxBuffer: 8 * 1024 * 1024, windowsHide: true }));
}
const comparePath = option("--compare");
const baseline = comparePath ? JSON.parse(await fs.readFile(path.resolve(root, comparePath), "utf8")) : null;
const serverURL = option("--url");
if (serverURL) {
  const parsed = new URL(serverURL);
  assert.equal(parsed.protocol, "http:");
  assert.ok(["127.0.0.1", "localhost", "[::1]"].includes(parsed.hostname), "Test server must be loopback");
}
const require = createRequire(import.meta.url);
const { chromium } = require(process.env.SCENE_PALETTE_PLAYWRIGHT || "playwright");
const sizes = [[360, 640], [390, 844], [430, 932], [768, 1024], [1440, 1000]];
const report = { startedAt: new Date().toISOString(), baselineRef: baselineRef || null, source: html,
  coverage: "Headless Chromium viewport/touch emulation; native Android/iOS/in-app browsers and OS file dialogs require device testing.",
  status: "running", checks: [], layouts: [], errors: [], externalRequests: [] };
const passed = name => { report.checks.push(name); console.log(`PASS ${name}`); };
const readPlate = page => page.evaluate(() => JSON.parse(JSON.stringify(snapshot())));
const canonical = plate => { const value = structuredClone(plate); delete value.createdAt; delete value.review; return value; };
const settled = page => page.waitForFunction(() => state.core.length === 8 && !document.getElementById("extractBtn").disabled);
const frame = page => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
let browser, currentPage;

async function newPage(width, height, url = pathToFileURL(html).href) {
  const context = await browser.newContext({ viewport: { width, height }, hasTouch: width <= 1000,
    reducedMotion: "reduce", acceptDownloads: true, serviceWorkers: "block" });
  if (url.startsWith("file:")) await context.setOffline(true);
  await context.route("**/*", route => {
    const requestURL = new URL(route.request().url());
    if (["http:", "https:"].includes(requestURL.protocol) && !["127.0.0.1", "localhost", "[::1]"].includes(requestURL.hostname)) {
      report.externalRequests.push(requestURL.href); return route.abort();
    }
    return route.continue();
  });
  const page = await context.newPage();
  currentPage = page;
  page.setDefaultTimeout(12000);
  page.on("pageerror", error => report.errors.push(error.stack || error.message));
  await page.goto(url);
  await page.waitForFunction(() => typeof state !== "undefined");
  return page;
}

async function openAncestors(page, selector) {
  const ids = await page.locator(selector).first().evaluate(element => {
    const ids = [];
    for (let parent = element.parentElement; parent; parent = parent.parentElement) {
      if (parent.tagName === "DETAILS") { if (!parent.id) throw new Error("Testable disclosure needs an ID"); ids.unshift(parent.id); }
    }
    return ids;
  });
  for (const id of ids) {
    if (!(await page.locator(`#${id}`).evaluate(element => element.open))) await page.locator(`#${id} > summary`).click();
  }
}

async function chooseSample(page, id = "blue") {
  const selector = `#samplePicker [data-sample="${id}"]`;
  await openAncestors(page, selector);
  await page.locator(selector).click();
  await page.waitForFunction(id => state.sampleId === id && state.stills.length === 1 && state.core.length === 8 && !document.getElementById("extractBtn").disabled, id);
  await frame(page);
}

async function geometry(page) {
  return page.evaluate(() => {
    const rect = selector => {
      const element = document.querySelector(selector);
      if (!element || !element.getClientRects().length) return null;
      const r = element.getBoundingClientRect();
      return { x: r.x + scrollX, top: r.y + scrollY, width: r.width, height: r.height, bottom: r.bottom + scrollY };
    };
    return { viewport: { width: innerWidth, height: innerHeight }, scrollY, pageWidth: document.documentElement.scrollWidth,
      pageHeight: document.documentElement.scrollHeight, header: rect("header"), aside: rect("aside"), main: rect("main"),
      drop: rect("#drop"), forge: rect("#extractBtn"), stage: rect("#stage"), chips: rect("#chips"),
      firstRow: rect("#chips .cell"), export: rect("#export"), focused: document.activeElement.id };
  });
}

async function captureWidths() {
  for (const [width, height] of sizes) {
    const page = await newPage(width, height);
    const label = `${width}x${height}`;
    const initial = await geometry(page);
    await page.screenshot({ path: path.join(output, `${label}-01-initial.png`) });
    assert.equal(await page.evaluate(() => state.core.length), 0);
    await chooseSample(page);
    const generated = await geometry(page);
    const plate = canonical(await readPlate(page));
    await page.screenshot({ path: path.join(output, `${label}-generated-focus.png`) });
    await page.evaluate(() => scrollTo(0, 0)); await frame(page);
    await page.screenshot({ path: path.join(output, `${label}-02-generated-top.png`) });
    await page.locator("#chips").screenshot({ path: path.join(output, `${label}-03-palette.png`) });
    if (await page.locator("#stage").isVisible()) await page.locator("#stage").screenshot({ path: path.join(output, `${label}-04-source.png`) });
    report.layouts.push({ width, height, initial, generated, plate });
    assert.equal(plate.core.length, 8);
    if (!baselineRef) {
      assert.ok(initial.pageWidth <= width + 1, `${label}: initial page overflows horizontally`);
      assert.ok(generated.pageWidth <= width + 1, `${label}: horizontal page overflow (${generated.pageWidth})`);
      assert.deepEqual(plate.core.map(token => token.id), ["ground", "panel", "ink", "muted", "observed", "attention", "rupture", "gold"],
        `${label}: friendly labels must retain the serialized color-role identifiers`);
      assert.deepEqual(await page.locator("#chips .role-label").allTextContents(),
        ["Background", "Surface", "Primary text", "Secondary text", "Information", "Attention", "Error", "Human action"],
        `${label}: fixed role labels must explain the color roles alongside editable names`);
      for (const [index, token] of plate.core.entries()) {
        const role = page.locator("#chips .role-label").nth(index);
        assert.ok(await role.isVisible(), `${label}: role ${token.id} needs a visible explanation`);
        assert.ok(await role.evaluate(element => element.scrollWidth <= element.clientWidth + 1),
          `${label}: role ${token.id} label is clipped`);
        const identity = [await role.getAttribute("title"), await page.locator(`[data-name="${index}"]`).getAttribute("aria-label")].join(" ");
        assert.ok(identity.includes(token.id), `${label}: technical role ${token.id} must remain discoverable`);
      }
      assert.match(await page.locator("#stageCanvas").getAttribute("aria-label"), /^Source image 1 of 1:/);
      assert.equal(await page.locator("#stageCanvas").getAttribute("aria-describedby"), "pickerHelp pickerStatus");
      assert.equal(await page.locator("#pickerStatus").getAttribute("aria-live"), "polite");
      if (width <= 1000) {
        assert.ok(initial.drop.top < height, `${label}: image chooser is below the first viewport`);
        assert.ok(initial.forge.bottom <= height, `${label}: the initial Forge action should fit in the first viewport`);
        assert.ok(generated.firstRow.top < height, `${label}: first palette row starts below the first viewport (${generated.firstRow.top})`);
        assert.ok(generated.chips.height <= 2 * height, `${label}: all eight cards need over two viewports`);
        assert.equal(generated.focused, "paletteHeading", `${label}: successful extraction should focus the palette`);
        assert.ok(generated.chips.bottom - generated.scrollY <= 2 * height,
          `${label}: all eight roles must fit within two viewports after extraction navigation`);
        for (const id of ["mobileTools", "mobileSettings", "mobileSource", "mobileReview"]) {
          assert.equal(await page.locator(`#${id}`).evaluate(element => element.open), false, `${label}: ${id} should start compact`);
        }
        for (const selector of ["#extractBtn", "#drop", "#chips .cellbar button", "#chips .hex", "#chips .info input"]) {
          const targets = await page.locator(selector).evaluateAll(elements => elements.map(element => {
            const r = element.getBoundingClientRect(); return { width: r.width, height: r.height };
          }));
          assert.ok(targets.length && targets.every(r => r.width >= 44 && r.height >= 44), `${label}: ${selector} has a target below 44px`);
        }
      }
      if (baseline) {
        const before = baseline.layouts.find(item => item.width === width);
        assert.ok(before, `No baseline for ${width}`);
        assert.deepEqual(plate, before.plate, `${label}: responsive work changed generated palette`);
        if (width <= 1000) {
          assert.ok(generated.chips.top <= before.generated.chips.top * 0.6, `${label}: palette top did not improve by at least 40%`);
        } else {
          for (const element of ["header", "aside", "main", "drop", "forge", "stage", "chips", "export"]) {
            for (const key of ["x", "top", "width", "height"]) {
              assert.ok(Math.abs(generated[element][key] - before.generated[element][key]) <= 2,
                `Desktop ${element}.${key} changed: ${before.generated[element][key]} -> ${generated[element][key]}`);
            }
          }
        }
      }
      if (width <= 1000) {
        for (const id of ["mobileTools", "mobileSettings", "mobileSource", "mobileReview", "mobileExport"]) {
          await openAncestors(page, `#${id}`);
          if (!(await page.locator(`#${id}`).evaluate(element => element.open))) await page.locator(`#${id} > summary`).click();
        }
        await frame(page);
        assert.ok((await geometry(page)).pageWidth <= width + 1, `${label}: expanded tools or approval copy overflow horizontally`);
        assert.deepEqual(canonical(await readPlate(page)), plate, `${label}: opening tools must not change the palette`);
        await page.screenshot({ path: path.join(output, `${label}-05-expanded-tools.png`), fullPage: true });
      }
    }
    passed(`${label}: ${baselineRef ? "baseline captured" : "palette layout and unchanged output verified"} (palette ${Math.round(generated.chips.top)}–${Math.round(generated.chips.bottom)}px)`);
    await page.context().close(); currentPage = null;
  }
}

async function assertFocus(page, selector, message) {
  // Edit renders are intentionally deferred until native focus traversal finishes.
  // Observe after that render, not the transient destination before it.
  await frame(page);
  assert.equal(await page.locator(selector).evaluate(element => element === document.activeElement), true, message);
}

async function clipboardChecks(page) {
  await page.evaluate(() => {
    window.__clipboardTest = { descriptor: Object.getOwnPropertyDescriptor(navigator, "clipboard"), original: document.execCommand, mode: "success", calls: 0 };
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: async () => {
      if (window.__clipboardTest.mode === "success") { window.__clipboardTest.calls++; return; }
      throw new Error("Test clipboard denied");
    } } });
    document.execCommand = () => {
      window.__clipboardTest.calls++;
      if (window.__clipboardTest.mode === "throw") throw new Error("Test fallback denied");
      return window.__clipboardTest.mode === "fallback-success";
    };
  });
  try {
    for (const mode of ["success", "fallback-success", "false", "throw"]) {
      await page.evaluate(mode => { window.__clipboardTest.mode = mode; window.__clipboardTest.calls = 0; }, mode);
      const copy = page.locator('.hex[data-copy="0"]');
      await copy.focus(); await copy.press("Enter");
      await page.waitForFunction(() => window.__clipboardTest.calls === 1);
      await frame(page);
      const toast = await page.locator("#toastMsg").textContent();
      if (mode === "success" || mode === "fallback-success") assert.ok(toast.includes((await readPlate(page)).core[0].hex), "Copy should announce the copied HEX");
      else assert.match(toast, /blocked|failed|could not|unable/i, `${mode}: clipboard denial must not announce success`);
      await assertFocus(page, '.hex[data-copy="0"]', `${mode}: clipboard fallback lost focus`);
    }
  } finally {
    await page.evaluate(() => {
      const saved = window.__clipboardTest;
      if (saved.descriptor) Object.defineProperty(navigator, "clipboard", saved.descriptor); else delete navigator.clipboard;
      document.execCommand = saved.original; delete window.__clipboardTest;
    });
  }
  passed("Clipboard native success, permission denial with successful fallback, false/throw failures, and focus restoration");
}

async function dirtyExportChecks(page) {
  await openAncestors(page, '#tabs [data-fmt="json"]');
  await page.locator('#tabs [data-fmt="json"]').click();
  assert.equal((await readPlate(page)).review.agentEligible, true, "Dirty-export fixture must begin sealed");
  await page.evaluate(() => {
    window.__exportCopyTest = { descriptor: Object.getOwnPropertyDescriptor(navigator, "clipboard"), text: null };
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: {
      writeText: async text => { window.__exportCopyTest.text = text; }
    } });
  });
  try {
    await page.locator('[data-name="0"]').fill("Just edited before Copy");
    await page.locator("#copyBtn").click({ delay: 80 });
    await page.waitForFunction(() => window.__exportCopyTest.text !== null);
    const copied = JSON.parse(await page.evaluate(() => window.__exportCopyTest.text));
    assert.equal(copied.core[0].name, "Just edited before Copy", "Copy must include the edit that blurs on its pointerdown");
    assert.equal(copied.review.status, "edited");
    assert.equal(copied.review.agentEligible, false, "Copy must never include a stale seal after editing");
  } finally {
    await page.evaluate(() => {
      const saved = window.__exportCopyTest;
      if (saved.descriptor) Object.defineProperty(navigator, "clipboard", saved.descriptor); else delete navigator.clipboard;
      delete window.__exportCopyTest;
    });
  }
  await openAncestors(page, "#sealPlate"); await page.locator("#sealPlate").click();
  assert.equal((await readPlate(page)).review.agentEligible, true);
  await page.locator('[data-job="0"]').fill("Just edited before Download");
  const event = page.waitForEvent("download");
  await page.locator("#downloadBtn").click({ delay: 80 });
  const saved = path.join(output, "mobile-dirty-export.json");
  await (await event).saveAs(saved);
  const downloaded = JSON.parse(await fs.readFile(saved, "utf8"));
  assert.equal(downloaded.core[0].job, "Just edited before Download", "Download must include the edit that blurs on its pointerdown");
  assert.equal(downloaded.review.status, "edited");
  assert.equal(downloaded.review.agentEligible, false, "Download must never include a stale seal after editing");
  passed("Dirty name→Copy and purpose→Download include current values and invalidate the formerly sealed export immediately");
}

async function workflows() {
  const page = await newPage(390, 844);
  await chooseSample(page);
  await page.locator('[data-name="0"]').fill("Mobile ground");
  await page.locator('[data-name="0"]').press("Tab");
  assert.equal((await readPlate(page)).core[0].name, "Mobile ground");
  await assertFocus(page, '.hex[data-copy="0"]', "Name change must keep Tab moving to the next field");
  await page.locator('[data-name="0"]').fill("Shift Tab ground");
  await page.locator('[data-name="0"]').press("Shift+Tab");
  await assertFocus(page, '[data-lock="0"]', "Name change must keep Shift+Tab moving to the preceding control");
  assert.equal((await readPlate(page)).core[0].name, "Shift Tab ground");
  await page.locator('[data-job="0"]').fill("Mobile pointer purpose");
  await page.locator('[data-name="1"]').click();
  await assertFocus(page, '[data-name="1"]', "Pointer blur must retain focus in the field the user clicked");
  assert.equal((await readPlate(page)).core[0].job, "Mobile pointer purpose");
  await page.locator('[data-name="1"]').fill("Pointer panel");
  // Hold across several animation frames: a redraw on blur must not remove
  // the pointer target before mouseup synthesizes its click.
  await page.locator('[data-lock="1"]').click({ delay: 80 });
  await assertFocus(page, '[data-lock="1"]', "Blur rerender must not swallow the following pointer action");
  assert.equal((await readPlate(page)).core[1].name, "Pointer panel");
  assert.equal((await readPlate(page)).core[1].locked, true, "The pointer action following blur must run exactly once");
  await page.locator('[data-lock="0"]').focus(); await page.locator('[data-lock="0"]').press("Space");
  await assertFocus(page, '[data-lock="0"]', "Lock rerender must retain the operated control's focus");
  assert.equal((await readPlate(page)).core[0].locked, true);
  await page.locator('[data-job="0"]').fill("Held pointer purpose");
  await page.locator('[data-why="ground"]').click({ delay: 80 }); await frame(page);
  assert.equal((await readPlate(page)).core[0].job, "Held pointer purpose");
  assert.ok(await page.locator("#whyPanel").isVisible());
  assert.ok(await page.locator("#whyPanel").evaluate(element => {
    const r = element.getBoundingClientRect(); return r.top < innerHeight && r.bottom > 0 && element.contains(document.activeElement);
  }), "Why must reveal and focus the explanation");
  await page.locator("#closeWhy").click();
  await assertFocus(page, '[data-why="ground"]', "Why close must restore its trigger focus");
  passed("Mobile edits retain Tab, Shift+Tab and pointer destinations; lock and Why/close preserve focus");
  await openAncestors(page, "#reviewChanges");
  await page.locator("#reviewChanges").focus(); await page.locator("#reviewChanges").press("Enter");
  await assertFocus(page, "#reviewChanges", "Review rerender must retain focus");
  await page.locator("#sealPlate").click();
  assert.equal((await readPlate(page)).review.agentEligible, true);
  const sealed = await readPlate(page);
  const sealedExports = await page.evaluate(() => Object.fromEntries(["css", "godot", "json", "gpl", "hex"].map(format => [format, formatPlate(snapshot(), format)])));
  // Choose a mixed compact disclosure state; normal rendering must not reset it.
  for (const id of ["mobileTools", "mobileSettings", "mobileSource", "mobileReview", "mobileExport"]) {
    const expected = ["mobileTools", "mobileReview", "mobileExport"].includes(id);
    if ((await page.locator(`#${id}`).evaluate(element => element.open)) !== expected) await page.locator(`#${id} > summary`).click();
  }
  await page.locator('.hex[data-copy="0"]').focus();
  const disclosureState = await page.evaluate(() => Object.fromEntries(["mobileTools", "mobileSettings", "mobileSource", "mobileReview", "mobileExport"].map(id => [id, document.getElementById(id).open])));
  for (const width of [1000, 1001, 1440, 390]) {
    await page.setViewportSize({ width, height: 1000 }); await frame(page);
    assert.deepEqual(await readPlate(page), sealed, `Resize to ${width} changed palette or review/seal`);
    for (const id of ["extractBtn", "file", "plateName", "colorPolicy", "reviewBar", "stageCanvas", "chips"]) {
      assert.equal(await page.locator(`#${id}`).count(), 1, `Resize duplicated ${id}`);
    }
    if (width > 1000) for (const id of ["mobileTools", "mobileSettings", "mobileSource", "mobileReview"]) {
      assert.equal(await page.locator(`#${id}`).evaluate(element => element.open), true, `${id} must remain exposed on desktop`);
    }
    if (width <= 1000) assert.deepEqual(await page.evaluate(() => Object.fromEntries(["mobileTools", "mobileSettings", "mobileSource", "mobileReview", "mobileExport"].map(id => [id, document.getElementById(id).open]))), disclosureState,
      `Resize to ${width} lost the user's compact disclosure choices`);
    assert.deepEqual(await page.evaluate(() => Object.fromEntries(["css", "godot", "json", "gpl", "hex"].map(format => [format, formatPlate(snapshot(), format)]))), sealedExports,
      `Resize to ${width} changed sealed export bytes`);
  }
  for (const selector of ["#plateName", "#colorPolicy", "#extractBtn"]) {
    await page.setViewportSize({ width: 1001, height: 1000 }); await frame(page);
    await page.locator(selector).focus();
    if (selector === "#plateName") await page.locator(selector).evaluate(element => element.setSelectionRange(2, 5));
    for (const width of [1000, 1001]) {
      await page.setViewportSize({ width, height: 1000 });
      await assertFocus(page, selector, `${selector}: crossing to ${width}px must retain the moving control's focus`);
      assert.ok(await page.locator(selector).isVisible(), `${selector}: resize must not hide the focused control`);
      if (selector === "#plateName") assert.deepEqual(await page.locator(selector).evaluate(element => [element.selectionStart, element.selectionEnd]), [2, 5]);
      assert.deepEqual(await readPlate(page), sealed, "Moving focused controls must not edit or unseal the plate");
    }
  }
  await page.setViewportSize({ width: 390, height: 844 }); await frame(page);
  passed("Breakpoint transitions preserve disclosures, focused moving controls, sealed snapshots and exact export bytes");
  for (const id of ["mobileTools", "mobileSettings", "mobileSource", "mobileReview", "mobileExport"]) {
    await page.setViewportSize({ width:390, height:844 }); await frame(page);
    await page.locator(`#${id} > summary`).focus();
    await page.setViewportSize({ width:1001, height:1000 }); await frame(page);
    assert.equal(await page.evaluate(id => {
      const active = document.activeElement;
      return document.getElementById(id).contains(active) && active.tagName !== "SUMMARY" && active.getClientRects().length > 0;
    }, id), true, `${id}: hidden desktop summary must transfer focus to a visible control`);
    assert.deepEqual(await readPlate(page), sealed, "Summary focus transfer must preserve the seal");
  }
  await page.setViewportSize({ width:390, height:844 }); await frame(page);
  passed("Mobile disclosure headings transfer focus to their controls when desktop hides the headings");
  await clipboardChecks(page);
  await dirtyExportChecks(page);
  for (const id of ["violet", "amber", "blue"]) {
    await chooseSample(page, id);
    assert.equal((await readPlate(page)).polarity, id === "blue" ? "light" : "dark");
    await openAncestors(page, "#tryOn");
    await page.locator("#tryOn").uncheck();
    assert.equal(await page.evaluate(() => document.documentElement.style.getPropertyValue("--bg")), "");
    await page.locator("#tryOn").check();
    assert.equal(await page.evaluate(() => document.documentElement.style.getPropertyValue("--bg")), (await readPlate(page)).core[0].hex);
  }
  passed("All three samples auto-extract on mobile and Try On remains reversible");
  const chooserEvent = page.waitForEvent("filechooser");
  await page.locator("#drop").click();
  await (await chooserEvent).setFiles(path.join(root, "testdata/reference-images/pastel-prisms.png"));
  await page.waitForFunction(() => state.sampleId === null && state.stills.length === 1);
  assert.equal(await page.evaluate(() => state.core.length), 0, "Manual upload should await Forge");
  await page.locator("#extractBtn").click(); await settled(page); await frame(page);
  await assertFocus(page, "#paletteHeading", "Explicit Forge should focus the palette heading");
  assert.equal((await readPlate(page)).sources[0].file, "pastel-prisms.png");
  await openAncestors(page, "#colorPolicy");
  await page.locator("#colorPolicy").selectOption("source-hues"); await settled(page);
  assert.equal((await readPlate(page)).params.colorPolicy, "source-hues");
  await page.locator("#colorPolicy").selectOption("readable-ui"); await settled(page);
  assert.equal((await readPlate(page)).params.colorPolicy, "readable-ui");
  const sourceRole = await page.evaluate(() => state.core.find(token => {
    const raw = recordedSourcePaint(token); return raw && raw.hex !== token.hex;
  })?.id);
  assert.ok(sourceRole, "Fixture must expose a source restoration action");
  const beforeSource = await readPlate(page);
  await page.locator(`[data-why="${sourceRole}"]`).click();
  await page.locator("#useSourcePaint").click();
  assert.equal((await readPlate(page)).core.find(token => token.id === sourceRole).locked, true);
  assert.equal((await readPlate(page)).review.status, "edited");
  await page.locator("#toastAction").click();
  assert.deepEqual(await readPlate(page), beforeSource);
  passed("Manual upload/Forge, palette focus, both colour policies, source restoration and Undo");
  await chooseSample(page, "blue");
  const diagnosticRole = await page.locator("[data-colour-why]").first().getAttribute("data-colour-why");
  assert.ok(diagnosticRole, "Blue fixture must have a large-adjustment diagnostic link");
  await page.locator(`[data-colour-why="${diagnosticRole}"]`).click();
  await page.locator("#useSourcePaint").click(); await frame(page);
  assert.equal(await page.locator(`[data-colour-why="${diagnosticRole}"]`).count(), 0,
    "Restored-source diagnostic should disappear for this regression fixture");
  await page.locator("#closeWhy").click();
  await assertFocus(page, `[data-why="${diagnosticRole}"]`, "Closing a diagnostic explanation must fall back to the role when its original trigger disappeared");
  passed("Diagnostic→Use source colour→Close restores role focus when the diagnostic link disappears");
  await page.locator('[data-pick="observed"]').click(); await frame(page);
  assert.equal(await page.locator("#mobileSource").evaluate(element => element.open), true);
  await assertFocus(page, "#stageCanvas", "Pick should expand and focus the source canvas");
  await page.locator("#stageCanvas").press("ArrowRight"); await page.locator("#stageCanvas").press("Enter");
  assert.equal((await readPlate(page)).core.find(token => token.id === "observed").origin.kind, "picked");
  assert.equal((await readPlate(page)).core.find(token => token.id === "observed").locked, true);
  passed("Pick opens the real source canvas; keyboard sampling preserves picked/locked semantics");
  const exportedPlate = await readPlate(page);
  for (const format of ["css", "godot", "json", "gpl", "hex"]) {
    await openAncestors(page, `#tabs [data-fmt="${format}"]`);
    await page.locator(`#tabs [data-fmt="${format}"]`).click();
    const expected = await page.evaluate(format => formatPlate(snapshot(), format), format);
    assert.equal(await page.locator("#export").inputValue(), expected);
    if (format === "json" || format === "hex") {
      const event = page.waitForEvent("download");
      await page.locator("#downloadBtn").click();
      const download = await event;
      const saved = path.join(output, `mobile-export.${format === "json" ? "json" : "txt"}`);
      await download.saveAs(saved);
      assert.equal(await fs.readFile(saved, "utf8"), expected);
    }
  }
  assert.deepEqual(await readPlate(page), exportedPlate, "Viewing/exporting formats must not edit the palette");
  passed("All five export formats remain reachable; JSON and HEX downloads match the existing formatter");
  await openAncestors(page, "#clearAll"); await page.locator("#clearAll").click();
  const bytes = await fs.readFile(path.join(root, "testdata/reference-images/pastel-prisms.png"));
  await page.locator("#file").setInputFiles(Array.from({ length: 8 }, (_, i) => ({ name: `mobile-source-${i + 1}.png`, mimeType: "image/png", buffer: bytes })));
  await page.waitForFunction(() => state.stills.length === 8);
  assert.equal(await page.locator("#thumbs [data-open]").count(), 8);
  await openAncestors(page, '#thumbs [data-open="7"]');
  await page.locator('#thumbs [data-open="7"]').click();
  assert.equal(await page.evaluate(() => state.sel), 7);
  assert.ok((await geometry(page)).pageWidth <= 391, "Eight sources must not create horizontal page overflow");
  await page.locator('#thumbs [data-del="7"]').click();
  assert.equal(await page.evaluate(() => state.stills.length), 7);
  await page.locator("#toastAction").click();
  assert.equal(await page.evaluate(() => state.stills.length), 8);
  passed("Eight sources remain selectable/removable with Undo and without page overflow");
  await page.context().close(); currentPage = null;
  if (serverURL) {
    const goPage = await newPage(390, 844, serverURL);
    await chooseSample(goPage);
    assert.equal(await goPage.evaluate(() => state.backend), "go");
    assert.equal((await readPlate(goPage)).core.length, 8);
    assert.ok((await geometry(goPage)).chips.top < 844);
    await goPage.context().close(); currentPage = null;
    passed("Compact mobile sample workflow also runs through the local Go backend");
  }
}

try {
  const executablePath = option("--browser", process.env.SCENE_PALETTE_BROWSER);
  const channel = option("--channel", process.env.SCENE_PALETTE_BROWSER_CHANNEL || (process.platform === "win32" ? "msedge" : undefined));
  browser = await chromium.launch({ headless: true, executablePath, channel: executablePath ? undefined : channel, args: ["--disable-gpu"] });
  report.browser = browser.version();
  await captureWidths();
  if (!baselineRef) await workflows();
  assert.deepEqual(report.errors, [], "Unexpected browser errors");
  assert.deepEqual(report.externalRequests, [], "Unexpected external requests");
  report.status = "passed";
} catch (error) {
  report.status = "failed"; report.failure = error.stack || String(error);
  console.error(report.failure); process.exitCode = 1;
  if (currentPage) {
    report.failureContext = await currentPage.evaluate(() => ({
      viewport: { width: innerWidth, height: innerHeight }, pageWidth: document.documentElement.scrollWidth,
      active: { tag: document.activeElement.tagName, id: document.activeElement.id, className: document.activeElement.className },
      disclosures: Object.fromEntries([...document.querySelectorAll("details[id]")].map(element => [element.id, element.open])),
      toast: document.getElementById("toastMsg")?.textContent
    })).catch(() => null);
    await currentPage.screenshot({ path: path.join(output, "failure.png"), fullPage: true }).catch(() => {});
  }
} finally {
  if (browser) await browser.close();
  report.finishedAt = new Date().toISOString();
  await fs.writeFile(path.join(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  console.log(`Mobile verification ${report.status}: ${path.join(output, "report.json")}`);
}

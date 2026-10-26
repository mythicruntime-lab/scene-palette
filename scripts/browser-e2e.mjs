// End-to-end checks against a running local Go server and the file:// fallback.
// No packages are required by the app. For this development check, install
// playwright or expose it through NODE_PATH / SCENE_PALETTE_PLAYWRIGHT.
// Run long checks through jobgate on the shared workstation.
import assert from "node:assert/strict";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath, pathToFileURL } from "node:url";
import { deflateSync, inflateRawSync } from "node:zlib";

const require = createRequire(import.meta.url);
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
function option(name, fallback) {
  const at = args.indexOf(name);
  if (at === -1) return fallback;
  assert.ok(args[at + 1] && !args[at + 1].startsWith("--"), `${name} needs a value`);
  return args[at + 1];
}
const baseURL = new URL(option("--url", "http://127.0.0.1:8765/"));
const standaloneHTML = path.resolve(root, option("--html", "index.html"));
assert.ok(["127.0.0.1", "localhost", "[::1]"].includes(baseURL.hostname), "E2E server must be local");
assert.equal(baseURL.protocol, "http:", "E2E uses the local HTTP server");
const output = path.resolve(root, option("--output", `bin/e2e/${new Date().toISOString().replace(/[:.]/g, "-")}`));
const artifactRoot = path.resolve(root, "bin/e2e");
assert.ok(output === artifactRoot || output.startsWith(artifactRoot + path.sep), "Test outputs must stay under bin/e2e");
await fs.mkdir(output, { recursive: true });

let playwright;
try {
  playwright = require(process.env.SCENE_PALETTE_PLAYWRIGHT || "playwright");
} catch {
  throw new Error("Browser verification requires playwright; install it or set SCENE_PALETTE_PLAYWRIGHT / NODE_PATH");
}

const report = { startedAt: new Date().toISOString(), url: baseURL.href, checks: [], network: [], errors: [] };
let browser, page;
let expectedFailure = false;
function passed(name, detail) {
  report.checks.push({ name, status: "passed", ...(detail ? { detail } : {}) });
  console.log(`PASS ${name}`);
}
const imagePath = name => path.join(root, "testdata/reference-images", name);
const sha256 = value => crypto.createHash("sha256").update(value).digest("hex");

function watch(target, label) {
  target.on("pageerror", error => report.errors.push(`${label}: ${error.stack || error.message}`));
  target.on("console", message => {
    if (message.type() !== "error" || expectedFailure) return;
    const source = message.location().url || "";
    if (source.endsWith("/favicon.ico")) return;
    report.errors.push(`${label} console: ${message.text()}`);
  });
  target.on("response", response => {
    const url = new URL(response.url());
    if (url.pathname.startsWith("/api/")) report.network.push({ page: label, path: url.pathname, status: response.status() });
  });
}

async function readPlate(target) {
  return target.evaluate(() => JSON.parse(JSON.stringify(snapshot())));
}
function semantic(plate) {
  const token = t => ({
    id: t.id || t.key, name: t.name, job: t.job, hex: t.hex, rgb: t.rgb,
    locked: !!t.locked, derived: !!t.derived, origin: t.origin,
    oklch: t.oklch, contrast: t.contrast
  });
  return {
    schema: plate.schema, kind: plate.kind, name: plate.name,
    generator: plate.generator, polarity: plate.polarity, lightScene: plate.lightScene,
    medianL: plate.medianL, params: plate.params,
    sources: plate.sources, tags: plate.tags, family: plate.family,
    core: plate.core.map(token), found: plate.found.map(token), notes: plate.notes,
    facets: plate.facets, signature: plate.signature
  };
}
function lockedIdentity(token) {
  return {
    id: token.id || null, name: token.name, job: token.job, hex: token.hex,
    locked: !!token.locked, derived: !!token.derived, origin: token.origin
  };
}
async function forge(target, go = true) {
  const response = go ? target.waitForResponse(r => new URL(r.url()).pathname === "/api/extract" && r.request().method() === "POST") : null;
  await target.locator("#extractBtn").click();
  if (response) {
    const result = await response;
    assert.equal(result.status(), 200, `Go extraction failed: ${await result.text()}`);
  }
  await target.waitForFunction(() => state.core.length === 8 && !document.getElementById("extractBtn").disabled);
  return readPlate(target);
}
async function loadImages(target, files, name) {
  const hasWork = await target.evaluate(() => !!(state.stills.length || state.core.length));
  if (hasWork) await target.locator("#clearAll").click();
  await target.locator("#plateName").fill(name);
  await target.locator("#file").setInputFiles(files);
  await target.waitForFunction(count => state.stills.length === count, files.length);
}
async function forcePolarity(target, mode) {
  await target.locator(`[data-pol="${mode}"]`).click();
  await target.waitForFunction(expected => state.polarity === expected && state.lightScene === (expected === "light") && !document.getElementById("extractBtn").disabled, mode);
}
async function assertGeneratedReadable(target, plate, label) {
  assert.equal(plate.core.length, 8, `${label}: incomplete generated core`);
  const checks = await target.evaluate(p => {
    const ground = p.core.find(t => t.id === "ground"), panel = p.core.find(t => t.id === "panel");
    return p.core.filter(t => !["ground", "panel"].includes(t.id)).map(t => ({
      role: t.id, locked: !!t.locked, target: t.id === "ink" ? 4.5 : 3,
      onGround: contrast(t, ground), onPanel: contrast(t, panel)
    }));
  }, plate);
  assert.equal(checks.length, 6, `${label}: missing core inks`);
  for (const check of checks) {
    assert.equal(check.locked, false, `${label}: generated readability case unexpectedly contains a locked ${check.role}`);
    assert.ok(check.onGround >= check.target && check.onPanel >= check.target,
      `${label}: ${check.role} needs ${check.target}:1 on both surfaces; ground=${check.onGround}, panel=${check.onPanel}`);
  }
}
async function clickImage(target, xFraction, yFraction) {
  const canvas = target.locator("#stageCanvas");
  const position = await canvas.evaluate((element, fractions) => {
    const box = element.getBoundingClientRect();
    const scale = Math.min(box.width / element.width, box.height / element.height);
    const width = element.width * scale, height = element.height * scale;
    return { x: (box.width - width) / 2 + width * fractions[0], y: (box.height - height) / 2 + height * fractions[1] };
  }, [xFraction, yFraction]);
  await canvas.click({ position });
}
async function download(target, selector, file) {
  const event = target.waitForEvent("download");
  await target.locator(selector).click();
  const result = await event;
  await result.saveAs(path.join(output, file));
  return fs.readFile(path.join(output, file));
}
async function delayedExtraction(target, options = {}) {
  let started, release, completed, requests = 0;
  const start = new Promise(resolve => { started = resolve; });
  const hold = new Promise(resolve => { release = resolve; });
  const done = new Promise(resolve => { completed = resolve; });
  await target.route("**/api/extract", async route => {
    if (options.firstOnly && requests++ > 0) return route.continue();
    try {
      const response = await route.fetch();
      started();
      await hold;
      await route.fulfill({ response });
    } catch (error) {
      if (!/abort|cancel|closed|disposed/i.test(String(error))) throw error;
    } finally { completed(); }
  });
  return {
    async started() {
      let timer;
      try {
        await Promise.race([start, new Promise((_, reject) => {
          timer = setTimeout(() => reject(new Error("Delayed extraction never reached the server")), 15000);
        })]);
      } finally { clearTimeout(timer); }
    },
    async finish() {
      release();
      await done;
      await target.waitForFunction(() => goPending === null);
      await target.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      await target.unroute("**/api/extract");
    }
  };
}

const crcTable = Array.from({ length: 256 }, (_, value) => {
  let crc = value;
  for (let bit = 0; bit < 8; bit++) crc = (crc & 1) ? 0xedb88320 ^ (crc >>> 1) : crc >>> 1;
  return crc >>> 0;
});
function crc32(bytes) {
  let crc = 0xffffffff;
  for (const byte of bytes) crc = crcTable[(crc ^ byte) & 255] ^ (crc >>> 8);
  return (crc ^ 0xffffffff) >>> 0;
}
function unzip(bytes) {
  let end = bytes.length - 22;
  while (end >= Math.max(0, bytes.length - 65557) && bytes.readUInt32LE(end) !== 0x06054b50) end--;
  assert.ok(end >= 0, "ZIP end record missing");
  const count = bytes.readUInt16LE(end + 10);
  let position = bytes.readUInt32LE(end + 16);
  const files = new Map();
  for (let i = 0; i < count; i++) {
    assert.equal(bytes.readUInt32LE(position), 0x02014b50, "Bad central ZIP record");
    const compression = bytes.readUInt16LE(position + 10);
    const crc = bytes.readUInt32LE(position + 16);
    const compressedSize = bytes.readUInt32LE(position + 20);
    const size = bytes.readUInt32LE(position + 24);
    const nameLength = bytes.readUInt16LE(position + 28);
    const extraLength = bytes.readUInt16LE(position + 30);
    const commentLength = bytes.readUInt16LE(position + 32);
    const local = bytes.readUInt32LE(position + 42);
    const name = bytes.subarray(position + 46, position + 46 + nameLength).toString("utf8");
    assert.ok(!name.includes("\\") && !name.startsWith("/") && !name.split("/").includes(".."), `Unsafe ZIP path: ${name}`);
    assert.ok(!files.has(name), `Duplicate ZIP entry: ${name}`);
    assert.equal(bytes.readUInt32LE(local), 0x04034b50, "Bad local ZIP record");
    const start = local + 30 + bytes.readUInt16LE(local + 26) + bytes.readUInt16LE(local + 28);
    const compressed = bytes.subarray(start, start + compressedSize);
    assert.ok(compression === 0 || compression === 8, `Unsupported ZIP compression ${compression}`);
    const data = compression === 0 ? Buffer.from(compressed) : inflateRawSync(compressed);
    assert.equal(data.length, size, `ZIP length: ${name}`);
    assert.equal(crc32(data), crc, `ZIP CRC: ${name}`);
    files.set(name, data);
    position += 46 + nameLength + extraLength + commentLength;
  }
  return files;
}
function checkPreview(bytes) {
  assert.ok(bytes, "Missing preview.png");
  assert.equal(bytes.subarray(0, 8).toString("hex"), "89504e470d0a1a0a");
  assert.equal(bytes.readUInt32BE(16), 960);
  assert.equal(bytes.readUInt32BE(20), 250);
}
async function checkSingleBundle(bytes, expectedPlate, retained) {
  const files = unzip(bytes);
  assert.ok(![...files.keys()].some(name => name.endsWith("index.json")), "A single bundle must not overwrite the vault index");
  const palettePath = [...files.keys()].find(name => name.endsWith("/palette.json"));
  assert.ok(palettePath, "Missing bundle palette.json");
  const folder = palettePath.slice(0, -"palette.json".length);
  for (const name of ["tokens.css", "plate.gd", "palette.gpl", "notes.md", "preview.png"]) assert.ok(files.has(folder + name), `Missing ${name}`);
  checkPreview(files.get(folder + "preview.png"));
  const plate = JSON.parse(files.get(palettePath));
  assert.equal(plate.name, expectedPlate.name);
  assert.deepEqual(plate.core, expectedPlate.core);
  assert.deepEqual(plate.review, expectedPlate.review);
  for (const source of plate.sources) {
    if (retained) {
      assert.notEqual(source.retained, false);
      const original = files.get(folder + source.path);
      assert.ok(original, `Missing original ${source.file}`);
      assert.equal(source.hash.algo, "sha256");
      assert.equal(sha256(original), source.hash.value);
      assert.deepEqual(original, await fs.readFile(imagePath(source.file)));
    } else {
      assert.equal(source.path, null);
      assert.equal(source.retained, false);
    }
  }
  return plate;
}

// A synthetic original exercises canvas downscaling and transparency without
// adding a large fixture or another image-processing dependency to the repo.
function syntheticPNG(width, height) {
  const raw = Buffer.alloc((width * 4 + 1) * height);
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const at = y * (width * 4 + 1) + 1 + x * 4;
      raw[at] = Math.floor(x / 13) % 256;
      raw[at + 1] = Math.floor(y / 7) % 256;
      raw[at + 2] = (x + y) % 256;
      raw[at + 3] = x < width / 4 ? 0 : x < width / 2 ? 210 : 255;
    }
  }
  function chunk(type, data) {
    const tag = Buffer.from(type);
    const length = Buffer.alloc(4); length.writeUInt32BE(data.length);
    const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(Buffer.concat([tag, data])));
    return Buffer.concat([length, tag, data, crc]);
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width); ihdr.writeUInt32BE(height, 4); ihdr[8] = 8; ihdr[9] = 6;
  return Buffer.concat([Buffer.from("89504e470d0a1a0a", "hex"), chunk("IHDR", ihdr), chunk("IDAT", deflateSync(raw)), chunk("IEND", Buffer.alloc(0))]);
}

async function checkShelfSecurity(target, label) {
  const fixture = JSON.parse(await fs.readFile(path.join(root,
    "testdata/reference-palettes/empty-neon-street/palette.json"), "utf8"));
  const upload = async value => target.locator("#plateFile").setInputFiles({
    name:"security-shelf.json", mimeType:"application/json", buffer:Buffer.from(JSON.stringify(value))
  });
  await target.locator("#importBtn").click();
  await upload(fixture);
  await target.waitForFunction(() => state.core.length === 8);
  await target.locator("#sealPlate").click();
  await target.locator("#saveBtn").click();
  const seed = await target.evaluate(() => JSON.parse(JSON.stringify(state.shelf[0])));
  const marker = '<img data-shelf-attack src="data:image/png;base64,AAAA" onerror="window.__shelfSecuritySentinel=(window.__shelfSecuritySentinel||0)+1">';
  const invalid = [];
  for (const mutate of [
    p => { p.core[0].hex = '#101010">' + marker; },
    p => { p.found = [{ ...p.core[2], id:null, key:"found_1", hex:marker }]; },
    p => { p.core[0].id = marker; },
    p => { p.stills = marker; },
    p => { p.core[2].origin.kind = { toString:null, valueOf:null }; },
    p => { p.core[2].origin.repairs = [{ from:{ toString:null, valueOf:null } }]; },
    p => { p.sources[0].natural = [{ toString:null, valueOf:null }, 80]; },
    p => { p.params.variant = { toString:null, valueOf:null }; },
    p => { p.review.history = [{ ...p.review, invalidatedBy:{ toString:null, valueOf:null } }]; },
    p => { p.name = { html:marker }; },
    p => { p.notes = [{ html:marker }]; },
    p => { p.core[1] = p.core[0]; },
    p => { p.found = [null]; }
  ]) {
    const plate = structuredClone(seed); plate.id = "hostile-" + invalid.length;
    mutate(plate); invalid.push(plate);
  }
  const invalidSingle = structuredClone(seed);
  invalidSingle.core[2].origin.kind = { toString:null, valueOf:null };
  const errorOffset = report.errors.length;
  await target.locator("#importBtn").click();
  await upload(invalidSingle);
  await target.waitForFunction(() => document.getElementById("toastMsg").textContent.includes("invalid kind"));
  await target.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
  const rejectionErrors = report.errors.splice(errorOffset);
  assert.equal(rejectionErrors.length, 1, "Expected exactly the logged single-palette validation rejection");
  assert.ok(rejectionErrors[0].startsWith(label + " console: Error: invalid kind\n"), rejectionErrors[0]);
  (report.expectedValidationErrors ||= []).push(...rejectionErrors);
  assert.equal((await readPlate(target)).review.agentEligible, true, "Rejected single import changed the existing sealed plate");
  const safeText = structuredClone(seed);
  safeText.id = "safe-text"; safeText.name = marker + "\n*/untrusted name";
  safeText.core[2].name = marker;
  safeText.core[2].origin.extra = { text:marker };
  safeText.found = [{ ...safeText.core[2], id:marker, key:"found_1" }];
  await target.locator("#shelfImport").click();
  await upload({ plates:[...invalid, safeText] });
  await target.waitForFunction(() => state.shelf.length === 2);
  assert.match(await target.locator("#toastMsg").textContent(), new RegExp(invalid.length + " invalid entries skipped"));
  const inspect = async () => {
    await target.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    assert.equal(await target.evaluate(() => window.__shelfSecuritySentinel || 0), 0, `${label}: executable shelf content ran`);
    assert.equal(await target.locator("[data-shelf-attack]").count(), 0, `${label}: injected node reached the DOM`);
  };
  await inspect();
  await target.locator('[data-use="0"]').click();
  await target.locator('[data-a="0"]').click();
  await target.locator('[data-b="1"]').click();
  await target.locator("#pairBtn").click();
  await target.locator('[data-why="ink"]').click();
  for (const format of ["css", "godot", "hex", "gpl", "json"]) {
    await target.locator(`[data-fmt="${format}"]`).click();
    assert.ok((await target.locator("#export").inputValue()).length);
  }
  const outputSafety = await target.evaluate(() => {
    const plate = state.shelf[0];
    return {
      css:formatPlate(plate, "css"), godot:formatPlate(plate, "godot"),
      themeCSS:themeExport(state.theme, "css"), themeGodot:themeExport(state.theme, "godot")
    };
  });
  for (const [kind, content] of Object.entries(outputSafety)) {
    assert.ok(!content.includes("--<img") && !content.includes('"<img'), `${label}: unsafe ${kind} identifier`);
    if (kind.toLowerCase().includes("css")) assert.ok(!content.includes("*/untrusted"), "CSS comment escaped through name");
  }
  await inspect();
  // Bypass file import to exercise the independent localStorage boundary. Startup
  // must not write back or erase the rejected records from the saved input.
  const stored = JSON.stringify([seed, ...invalid]);
  await target.evaluate(value => localStorage.setItem("scene-palette.shelf.v1", value), stored);
  await target.reload();
  await target.waitForFunction(() => state.shelf.length === 1);
  assert.match(await target.locator("#toastMsg").textContent(), new RegExp(invalid.length + " invalid saved shelf entries skipped"));
  assert.equal(await target.evaluate(() => localStorage.getItem("scene-palette.shelf.v1")), stored);
  assert.equal(await target.evaluate(() => plateReview(state.shelf[0]).agentEligible), true, "Valid seal lost on restore");
  await target.locator('[data-use="0"]').click();
  assert.equal((await readPlate(target)).review.agentEligible, true, "Valid seal lost on Use");
  await inspect();
  await target.evaluate(() => localStorage.removeItem("scene-palette.shelf.v1"));
  await target.reload();
  await target.waitForFunction(() => state.shelf.length === 0 && state.core.length === 0);
  passed(`Hostile shelf import and restore are inert; valid seals and escaped text survive: ${label}`);
}


async function checkFoundInkIdentity(target, label) {
  const fixture = JSON.parse(await fs.readFile(path.join(root,
    "testdata/reference-palettes/empty-neon-street/palette.json"), "utf8"));
  fixture.name = "E2E found identity";
  fixture.found = fixture.core.slice(2, 6).map((token, index) => ({ ...token,
    id:null, key:"found_" + (index + 1), name:"Identity ink " + (index + 1) }));
  delete fixture.review;
  const upload = async value => target.locator("#plateFile").setInputFiles({
    name:"found-identity.json", mimeType:"application/json", buffer:Buffer.from(JSON.stringify(value))
  });
  await target.locator("#importBtn").click();
  await upload(fixture);
  await target.waitForFunction(() => state.found.length === 4);
  await target.locator("#sealPlate").click();
  const sealed = await readPlate(target);
  assert.equal(sealed.review.agentEligible, true);
  const hostile = structuredClone(sealed);
  hostile.id = "transient-found-identity";
  hostile.found[0].uid = { toString:null, valueOf:null };
  delete hostile.found[1].uid;
  hostile.found[2].uid = hostile.found[3].uid = 7;
  assert.equal(await target.evaluate(plate => plateReview(plate).agentEligible, hostile), true,
    "Transient UID extension fields must not change the durable fingerprint");
  await target.locator("#shelfImport").click();
  await upload({ kind:"scene-palette-shelf", version:1, plates:[hostile] });
  await target.waitForFunction(() => state.shelf.length === 1);
  const stored = await target.evaluate(() => localStorage.getItem("scene-palette.shelf.v1"));
  const inspect = async stage => {
    const ids = await target.evaluate(() => state.found.map(token => token.uid));
    assert.ok(ids.every(Number.isSafeInteger), `${label}/${stage}: found IDs must be safe editor numbers`);
    assert.equal(new Set(ids).size, 4, `${label}/${stage}: found IDs must be unique`);
    const current = await readPlate(target);
    assert.deepEqual(semantic(current), semantic(sealed), `${label}/${stage}: UID allocation changed durable colour data`);
    assert.equal(current.review.agentEligible, true, `${label}/${stage}: UID allocation invalidated a valid seal`);
    assert.deepEqual(current.review.fingerprint, sealed.review.fingerprint);
    for (let index = 0; index < 4; index++) {
      await target.locator(`[data-f-why="${index}"]`).click();
      const expected = sealed.found[index];
      assert.equal(await target.locator("#whyPanel .why-head strong").textContent(),
        `Why ${expected.name} is ${expected.hex}`, `${label}/${stage}: Why targeted the wrong found ink`);
      assert.equal(await target.evaluate(() => state.whyRole), "found:" + ids[index]);
      assert.deepEqual(await target.locator("[data-f-why]").evaluateAll(buttons =>
        buttons.map(button => button.getAttribute("aria-expanded"))),
        ids.map((_, at) => String(at === index)));
    }
    assert.equal(await target.evaluate(() => localStorage.getItem("scene-palette.shelf.v1")), stored,
      "Opening the editor must preserve the original saved UID extensions");
    assert.equal((await readPlate(target)).review.agentEligible, true, "Reading found origins changed the seal");
  };
  await target.locator('[data-use="0"]').click();
  await inspect("shelf file");
  await target.reload();
  await target.waitForFunction(() => state.shelf.length === 1);
  await target.locator('[data-use="0"]').click();
  await inspect("storage restore");
  await target.locator("#importBtn").click();
  await upload(hostile);
  await target.waitForFunction(() => state.activeShelfId === null);
  await inspect("single palette");
  await target.evaluate(() => localStorage.removeItem("scene-palette.shelf.v1"));
  await target.reload();
  passed(`Malicious, missing, and duplicate found UIDs cannot break Why or saved seals: ${label}`);
}

async function selectColourPolicy(target, policy) {
  await target.locator("#colorPolicy").selectOption(policy);
  await target.waitForFunction(expected => state.colorPolicy === expected && goPending === null &&
    document.getElementById("colorPolicy").value === expected, policy);
}
async function prepareColourFixture(target, image, name, policy, go) {
  await loadImages(target, [imagePath(image)], name);
  await selectColourPolicy(target, policy);
  await target.evaluate(async () => { await setPolarity("auto"); });
  return forge(target, go);
}
async function checkColourPolicyParity(goTarget, htmlTarget) {
  for (const [image, neutral] of [["monochrome-architecture.png", true], ["pastel-prisms.png", false]]) {
    for (const policy of ["readable-ui", "source-hues"]) {
      const name = `E2E colour policy ${image}`;
      await prepareColourFixture(goTarget, image, name, policy, true);
      await prepareColourFixture(htmlTarget, image, name, policy, false);
      for (const polarity of ["auto", "dark", "light"]) {
        if (polarity !== "auto") {
          await forcePolarity(goTarget, polarity);
          await forcePolarity(htmlTarget, polarity);
        }
        const actual = await readPlate(goTarget), expected = await readPlate(htmlTarget);
        assert.deepEqual(semantic(actual), semantic(expected), `${image}: ${policy}/${polarity} exact engine parity`);
        for (const [target, plate, label] of [[goTarget, actual, "Go"], [htmlTarget, expected, "standalone"]]) {
          assert.equal(plate.params.colorPolicy, policy);
          assert.equal(plate.params.polarityMode, polarity);
          assert.equal(await target.locator("#colorPolicy").inputValue(), policy);
          assert.match(await target.locator("#colourDiagnostics").innerText(), policy === "source-hues" ? /^Source hues/ : /^Readable UI/);
          await assertGeneratedReadable(target, plate, `${label}: ${image} ${policy}/${polarity}`);
          if (neutral && policy === "source-hues") {
            for (const token of plate.core) assert.ok(token.rgb.every(channel => channel === token.rgb[0]),
              `${label}: Source hues invented chromatic paint for grayscale ${token.id}: ${token.hex}`);
          }
          if (policy === "readable-ui") {
            const distances = await target.evaluate(p => p.core.slice(4).flatMap((a, i) =>
              p.core.slice(4 + i + 1).map(b => deltaE(a, b))), plate);
            assert.ok(distances.every(distance => distance >= .07), `${label}: final readable-UI accents collided`);
          }
        }
      }
    }
    passed(`Both colour policies match exactly and remain readable in Auto/Dark/Light: ${image}`,
      neutral ? "Source hues preserves neutral RGB in every core role" : "Limited/pastel hue fixture");
  }
}
async function checkColourRoundtripAndPaint(target, label, go) {
  await prepareColourFixture(target, "pastel-prisms.png", "E2E source paint", "source-hues", go);
  await target.locator("#sealPlate").click();
  const sealed = await readPlate(target);
  assert.equal(sealed.review.agentEligible, true);
  const sourceRole = sealed.core.find(t => ["observed", "attention", "rupture", "gold"].includes(t.id) &&
    /^#[a-f\d]{6}$/i.test(t.origin?.raw || "") && t.origin.raw.toUpperCase() !== t.hex.toUpperCase()) ||
    sealed.core.find(t => /^#[a-f\d]{6}$/i.test(t.origin?.raw || "") && t.origin.raw.toUpperCase() !== t.hex.toUpperCase());
  assert.ok(sourceRole, "Source-paint regression needs an actual recorded adjustment");
  const role = sourceRole.id;
  await target.locator(`[data-why="${role}"]`).click();
  assert.equal(await target.locator("#whyPanel .source-paint").count(), 2);
  assert.match(await target.locator("#whyPanel").innerText(), /Before adjustment/);
  assert.ok((await target.locator("#colourDiagnostics").innerText()).length > 0);
  await target.evaluate(() => renderColourDiagnostics());
  assert.deepEqual(await readPlate(target), sealed, `${label}: reading colour diagnostics changed a sealed palette`);
  await target.locator("#closeWhy").click();
  assert.deepEqual(await readPlate(target), sealed);
  await target.locator("#deriveBtn").click();
  await target.waitForFunction(() => !!state.theme && goPending === null);
  const theme = await target.evaluate(() => JSON.parse(JSON.stringify(state.theme)));
  for (const side of [theme.dark, theme.light]) assert.equal(side.params.colorPolicy, "source-hues", "Counterpart lost colour policy");
  assert.deepEqual(await readPlate(target), sealed, "Deriving a counterpart changed the visible sealed palette");
  await target.locator("#saveBtn").click();
  await target.locator('[data-fmt="json"]').click();
  const savedPath = `colour-policy-${label}.json`;
  const exported = JSON.parse(await download(target, "#downloadBtn", savedPath));
  assert.deepEqual(exported, sealed);
  await target.reload();
  await target.waitForFunction(() => state.shelf.length === 1);
  await target.locator('[data-use="0"]').click();
  assert.equal(await target.locator("#colorPolicy").inputValue(), "source-hues");
  assert.deepEqual(semantic(await readPlate(target)), semantic(sealed), "Shelf reload changed the recorded colour policy or palette");
  assert.equal((await readPlate(target)).review.agentEligible, true);
  await target.locator("#importBtn").click();
  await target.locator("#plateFile").setInputFiles(path.join(output, savedPath));
  await target.waitForFunction(() => state.activeShelfId === null && state.colorPolicy === "source-hues");
  assert.deepEqual(semantic(await readPlate(target)), semantic(sealed));
  assert.equal((await readPlate(target)).review.agentEligible, true);
  passed(`Source-hues policy survives counterpart, JSON, and shelf round trips: ${label}`);

  const beforePaint = await readPlate(target);
  await target.locator(`[data-why="${role}"]`).click();
  await target.locator("#useSourcePaint").click();
  const restored = await readPlate(target);
  const changed = restored.core.find(token => token.id === role);
  assert.equal(changed.hex, sourceRole.origin.raw.toUpperCase());
  assert.equal(changed.locked, true);
  const rawChannels = [1, 3, 5].map(offset => parseInt(changed.hex.slice(offset, offset + 2), 16));
  assert.deepEqual(changed.rgb, rawChannels);
  const { repairs: oldRepairs = [], ...oldOrigin } = sourceRole.origin;
  const { repairs: newRepairs, ...newOrigin } = changed.origin;
  assert.deepEqual(newOrigin, oldOrigin, "Use source colour lost recorded provenance");
  assert.deepEqual(newRepairs.slice(0, -1), oldRepairs);
  assert.deepEqual(newRepairs.at(-1), { op:"restore-source", from:sourceRole.hex, to:changed.hex, by:"human" });
  assert.equal(restored.review.status, "edited");
  assert.equal(restored.review.agentEligible, false);
  assert.ok(restored.review.history.length > beforePaint.review.history.length, "Superseded seal was not archived");
  for (const token of restored.core.filter(token => token.id !== role)) {
    assert.equal(token.hex, beforePaint.core.find(old => old.id === token.id).hex, "Use source colour changed another role");
  }
  await target.locator("#toastAction").click();
  assert.deepEqual(await readPlate(target), beforePaint, "Undo did not restore the palette and its matching seal");
  passed(`Source paint restores exact RGB, locks, records the human edit, and supports sealed Undo: ${label}`);

  const hostile = structuredClone(exported);
  const rawMarker = '#FFFFFF"><img data-colour-attack src="data:,bad" onerror="window.__colourDiagnosticsSentinel=1">';
  hostile.core.find(token => token.id === role).origin.raw = rawMarker;
  await target.locator("#importBtn").click();
  await target.locator("#plateFile").setInputFiles({ name:"hostile-origin.json", mimeType:"application/json", buffer:Buffer.from(JSON.stringify(hostile)) });
  await target.waitForFunction(id => state.core.find(token => token.id === id)?.origin.raw.includes("data-colour-attack"), role);
  await target.locator(`[data-why="${role}"]`).click();
  await target.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  assert.equal(await target.locator("[data-colour-attack]").count(), 0);
  assert.equal(await target.evaluate(() => window.__colourDiagnosticsSentinel || 0), 0);
  assert.equal(await target.locator("#whyPanel .source-comparison").count(), 0);
  assert.equal(await target.locator("#useSourcePaint").count(), 0);
  assert.equal((await readPlate(target)).core.find(token => token.id === role).origin.raw, rawMarker,
    "Display validation silently rewrote recorded provenance");
  assert.equal((await readPlate(target)).review.agentEligible, false, "Changed raw provenance retained a stale seal");
  passed(`Hostile origin.raw remains inert and cannot create source swatches: ${label}`);

  await target.evaluate(() => localStorage.removeItem("scene-palette.shelf.v1"));
  await target.reload();
  assert.equal(await target.locator("#shelfImport").isVisible(), true, "Fresh empty shelf must offer Import shelf");
  await target.locator("#shelfImport").click();
  await target.locator("#plateFile").setInputFiles({ name:"restored-shelf.json", mimeType:"application/json",
    buffer:Buffer.from(JSON.stringify({ kind:"scene-palette-shelf", version:1, plates:[exported] })) });
  await target.waitForFunction(() => state.shelf.length === 1);
  await target.locator('[data-use="0"]').click();
  assert.deepEqual(semantic(await readPlate(target)), semantic(sealed));
  assert.equal((await readPlate(target)).review.agentEligible, true);
  await target.evaluate(() => localStorage.removeItem("scene-palette.shelf.v1"));
  await target.reload();
  passed(`Fresh empty shelf can restore a saved Source-hues palette: ${label}`);
}
async function checkColourPolicyRaces(goTarget, htmlTarget) {
  const prepare = async name => {
    await prepareColourFixture(goTarget, "empty-neon-street.png", name, "readable-ui", true);
    await prepareColourFixture(htmlTarget, "empty-neon-street.png", name, "readable-ui", false);
  };
  await prepare("E2E cancelled colour policy");
  const before = await readPlate(goTarget);
  const gate = await delayedExtraction(goTarget, { firstOnly:true });
  await goTarget.locator("#colorPolicy").selectOption("source-hues");
  await gate.started();
  await goTarget.locator("#colorPolicy").selectOption("readable-ui");
  await gate.finish();
  assert.equal(await goTarget.locator("#colorPolicy").inputValue(), "readable-ui");
  assert.deepEqual(await readPlate(goTarget), before, "Late Source hues response overwrote the restored Readable UI choice");
  passed("Returning to Readable UI cancels a delayed Source hues response");

  for (const action of ["Forge", "Remix", "polarity"]) {
    await prepare(`E2E policy then ${action}`);
    const baseline = await readPlate(goTarget);
    assert.ok(baseline.params.variantCount > 1, "Policy race fixture needs multiple variants");
    const pending = await delayedExtraction(goTarget, { firstOnly:true });
    await goTarget.locator("#colorPolicy").selectOption("source-hues");
    await pending.started();
    await goTarget.locator(action === "Forge" ? "#extractBtn" : action === "Remix" ? "#remixBtn" : '[data-pol="light"]').click();
    await pending.finish();
    await goTarget.waitForFunction(() => !document.getElementById("extractBtn").disabled);
    const actual = await readPlate(goTarget);
    const policy = actual.params.colorPolicy;
    assert.ok(["readable-ui", "source-hues"].includes(policy));
    assert.equal(await goTarget.evaluate(() => state.colorPolicy), policy, `${action}: state and snapshot policies diverged`);
    assert.equal(await goTarget.locator("#colorPolicy").inputValue(), policy, `${action}: selector advertises a policy that was not applied`);
    assert.match(await goTarget.locator("#colourDiagnostics").innerText(), policy === "source-hues" ? /^Source hues/ : /^Readable UI/);
    if (action === "Forge") assert.equal(actual.params.variant, 0);
    if (action === "Remix") assert.equal(actual.params.variant, (baseline.params.variant + 1) % actual.params.variantCount);
    if (action === "polarity") assert.equal(actual.params.polarityMode, "light");
    await selectColourPolicy(htmlTarget, policy);
    await htmlTarget.evaluate(async mode => { await setPolarity(mode); }, actual.params.polarityMode);
    await htmlTarget.evaluate(async variant => { await applyVariant(variant); }, actual.params.variant);
    assert.deepEqual(semantic(actual), semantic(await readPlate(htmlTarget)), `${action}: displayed result does not match its recorded policy`);
    passed(`Pending Source hues followed by ${action} keeps selector, palette, and engine policy consistent`);
  }
}


try {
  const executablePath = option("--browser", process.env.SCENE_PALETTE_BROWSER);
  let channel = option("--channel", process.env.SCENE_PALETTE_BROWSER_CHANNEL);
  if (!channel && !executablePath && process.platform === "win32") {
    // Use an installed browser in a fresh, temporary test context. No profile
    // or settings from the owner's interactive browser are opened.
    const edge = "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe";
    if (await fs.stat(edge).then(() => true, () => false)) channel = "msedge";
  }
  browser = await playwright.chromium.launch({ headless: true, executablePath, channel, args: ["--disable-gpu"] });
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, acceptDownloads: true, serviceWorkers: "block" });
  context.setDefaultTimeout(15000);
  await context.route("**/*", route => {
    const url = new URL(route.request().url());
    if (["http:", "https:"].includes(url.protocol) && url.origin !== baseURL.origin) {
      report.errors.push(`Unexpected external request: ${url.origin}${url.pathname}`);
      return route.abort();
    }
    return route.continue();
  });
  page = await context.newPage(); watch(page, "Go");
  const info = await context.request.get(new URL("/api/info", baseURL).href);
  assert.equal(info.status(), 200);
  const capabilities = await info.json();
  assert.equal(capabilities.engine, "go");
  assert.equal(capabilities.version, 1);
  report.referenceHTML = { path: standaloneHTML, sha256: sha256(await fs.readFile(standaloneHTML)) };
  await page.goto(baseURL.href);
  await page.locator("#extractBtn").waitFor();
  const fallback = await context.newPage(); watch(fallback, "standalone");
  await fallback.goto(pathToFileURL(standaloneHTML).href);
  await fallback.locator("#extractBtn").waitFor();
  // Native Chromium/Edge cube roots have differed from the reference arithmetic
  // by one ULP. Near-neutral hues expose that drift before palette rounding.
  for (const [target, label] of [[page, "Go"], [fallback, "standalone"]]) {
    const neutralHues = await target.evaluate(() => [6, 22].map(value =>
      rgbToLch({ r:value, g:value, b:value }).H));
    assert.deepEqual(neutralHues, [89.87556269699212, 89.87556266224881],
      label + ": real-browser neutral hue arithmetic must match Go at full precision");
    passed("Real-browser colour arithmetic matches the Go reference at full precision: " + label);
  }
  await checkShelfSecurity(page, "Go");
  await checkShelfSecurity(fallback, "standalone");
  await checkFoundInkIdentity(page, "Go");
  await checkFoundInkIdentity(fallback, "standalone");

  const synthetic = path.join(output, "oversized-transparent.png");
  await fs.writeFile(synthetic, syntheticPNG(1800, 900));
  const scenarios = [
    ["dark", [imagePath("empty-neon-street.png")]],
    ["light", [imagePath("pastel-prisms.png")]],
    ["derived", [imagePath("monochrome-architecture.png")]],
    ["pooled", [imagePath("empty-neon-street.png"), imagePath("pastel-prisms.png")]],
    ["downscaled transparency", [synthetic]]
  ];
  for (const [label, images] of scenarios) {
    const name = `E2E ${label}`;
    await loadImages(page, images, name);
    await loadImages(fallback, images, name);
    const actual = await forge(page);
    const expected = await forge(fallback, false);
    for (let sourceIndex = 0; sourceIndex < images.length; sourceIndex++) {
      const png = await fs.readFile(images[sourceIndex]);
      const natural = [png.readUInt32BE(16), png.readUInt32BE(20)];
      assert.deepEqual(actual.sources[sourceIndex].natural, natural, `${label}: original dimensions lost`);
      assert.deepEqual(expected.sources[sourceIndex].natural, natural, `${label}: standalone original dimensions lost`);
      assert.equal(actual.sources[sourceIndex].hash.value, sha256(png));
    }
    assert.deepEqual(semantic(actual), semantic(expected), `${label}: Go / standalone parity`);
    assert.equal(actual.params.polarityMode, "auto", `${label}: baseline must use Auto`);
    assert.equal(actual.params.variant, 0, `${label}: Forge must reset to first variant`);
    await assertGeneratedReadable(page, actual, `${label}: Go Auto`);
    await assertGeneratedReadable(fallback, expected, `${label}: standalone Auto`);
    report.extractionVersions = { go: actual.generator.extraction, reference: expected.generator.extraction };
    passed(`Go and standalone palette parity: ${label}`, `${actual.params.variantCount} variants; ${actual.found.length} found inks`);
    for (const mode of ["dark", "light"]) {
      await forcePolarity(page, mode);
      await forcePolarity(fallback, mode);
      const forcedActual = await readPlate(page), forcedExpected = await readPlate(fallback);
      for (const plate of [forcedActual, forcedExpected]) {
        assert.equal(plate.params.polarityMode, mode, `${label}: requested ${mode} polarity was not applied`);
        assert.equal(plate.lightScene, mode === "light", `${label}: requested ${mode} layout was not applied`);
        assert.ok(plate.params.variantCount > 0, `${label}: forced ${mode} produced no variants`);
        assert.equal(plate.params.variant, 0, `${label}: forced ${mode} did not retain the requested first variant`);
      }
      assert.deepEqual(semantic(forcedActual), semantic(forcedExpected), `${label}: forced ${mode} Go / standalone parity`);
      await assertGeneratedReadable(page, forcedActual, `${label}: Go forced ${mode}`);
      await assertGeneratedReadable(fallback, forcedExpected, `${label}: standalone forced ${mode}`);
      assert.match(await page.locator("#matrix .mnote").innerText(), /Every token clears its target on both surfaces/);
    }
    for (const target of [page, fallback]) {
      await target.evaluate(async () => { await setPolarity("auto"); });
      const restored = await readPlate(target);
      assert.equal(restored.params.polarityMode, "auto", `${label}: return to Auto failed`);
      await assertGeneratedReadable(target, restored, `${label}: restored Auto`);
    }
    passed(`Generated palettes remain readable on both surfaces in forced dark and light: ${label}`);
  }
  await checkColourRoundtripAndPaint(page, "Go", true);
  await checkColourRoundtripAndPaint(fallback, "standalone", false);
  await checkColourPolicyRaces(page, fallback);
  await checkColourPolicyParity(page, fallback);
  await prepareColourFixture(page, "empty-neon-street.png", "E2E after colour workflows", "readable-ui", true);
  await fallback.close();

  // Deliberate imported user choices must remain visible and warn accurately;
  // generated-palette repair must not silently alter a locked manual ink.
  const manualWarningFixture = { ...await readPlate(page), name: "E2E intentional manual contrast warning", found: [], review: undefined };
  manualWarningFixture.core = manualWarningFixture.core.map(t => {
    const value = t.id === "panel" ? 0 : ["ground", "ink"].includes(t.id) ? 255 : 117;
    return { ...t, r: value, g: value, b: value, rgb: [value, value, value], hex: "#" + value.toString(16).padStart(2, "0").repeat(3).toUpperCase(),
      locked: t.id === "ink", derived: false, origin: { kind: "imported", reason: "Intentional manual colors in a disposable E2E fixture" } };
  });
  const manualWarningPath = path.join(output, "manual-contrast-warning.json");
  await fs.writeFile(manualWarningPath, JSON.stringify(manualWarningFixture, null, 2) + "\n");
  await page.locator("#plateFile").setInputFiles(manualWarningPath);
  await page.waitForFunction(() => document.getElementById("plateName").value === "E2E intentional manual contrast warning" && token("ink").locked);
  const groundOnlyFailure = await page.evaluate(() => {
    const inks = state.core.filter(t => !["ground", "panel"].includes(t.id));
    const failedOnGround = inks.filter(t => contrast(t, token("ground")) < (t.id === "ink" ? 4.5 : 3));
    return failedOnGround.length === 1 && failedOnGround[0].id === "ink" &&
      contrast(token("ink"), token("ground")) === 1 &&
      inks.every(t => contrast(t, token("panel")) >= (t.id === "ink" ? 4.5 : 3));
  });
  assert.ok(groundOnlyFailure, "Intentional manual fixture must preserve exactly the ground-only ink failure");
  assert.match(await page.locator("#matrix .mnote").innerText(), /below target on ground or panel/);
  await page.locator("#matrix").screenshot({ path: path.join(output, "manual-ground-contrast-warning.png") });
  passed("Intentional locked manual ink is preserved and warns when only ground contrast fails");

  await loadImages(page, [imagePath("empty-neon-street.png")], "E2E fixture - do not use");
  await page.evaluate(async () => { await setPolarity("auto"); });
  const initial = await forge(page);
  assert.ok(initial.params.variantCount > 1, "Fixture needs multiple variants for Remix regression");
  await assertGeneratedReadable(page, initial, "Clean generated fixture after manual warning test");
  assert.match(await page.locator("#matrix .mnote").innerText(), /Every token clears its target on both surfaces/);
  await page.locator('[data-pick="observed"]').click();
  await clickImage(page, 0.25, 0.25);
  await page.waitForFunction(() => !state.arm && state.core[4].origin.kind === "picked");
  await page.locator('[data-name="4"]').fill("Pinned fixture");
  await page.locator('[data-name="4"]').press("Tab");
  await page.locator('[data-job="4"]').fill("test-only role purpose");
  await page.locator('[data-job="4"]').press("Tab");
  await page.locator('[data-lock="0"]').click();
  await clickImage(page, 0.6, 0.6);
  const pinnedPlate = await readPlate(page);
  const pickedRole = lockedIdentity(pinnedPlate.core[4]);
  const lockedGround = lockedIdentity(pinnedPlate.core[0]);
  const pin = pinnedPlate.found.find(token => token.locked);
  assert.ok(pin && pin.origin.kind === "picked");
  const pinnedInk = lockedIdentity(pin);
  async function assertPins(label) {
    const current = await readPlate(page);
    assert.deepEqual(lockedIdentity(current.core[4]), pickedRole, `${label}: picked role changed`);
    assert.deepEqual(lockedIdentity(current.core[0]), lockedGround, `${label}: locked ground changed`);
    assert.deepEqual(lockedIdentity(current.found.find(token => token.locked)), pinnedInk, `${label}: pinned ink changed`);
  }
  await page.locator("#remixBtn").click();
  await page.waitForFunction(() => state.variant === 1 && !document.getElementById("extractBtn").disabled);
  await assertPins("Remix");
  await forcePolarity(page, "light");
  await assertPins("Polarity");
  const metadataFixture = {
    ...await readPlate(page), tags: ["e2e-fixture"], family: "e2e-family",
    createdAt: "2026-08-28T00:00:00.000Z"
  };
  const metadataPath = path.join(output, "metadata-fixture.json");
  await fs.writeFile(metadataPath, JSON.stringify(metadataFixture, null, 2) + "\n");
  await page.locator("#plateFile").setInputFiles(metadataPath);
  await page.waitForFunction(() => state.plateMeta.family === "e2e-family");
  await forge(page);
  await assertPins("Forge");
  passed("Picked roles, locked wording, origins, and pinned inks survive Remix, polarity, and Forge");

  const beforeCounterpart = await readPlate(page);
  await page.locator("#deriveBtn").click();
  await page.waitForFunction(() => !!state.theme);
  const pair = await page.evaluate(() => JSON.parse(JSON.stringify(state.theme)));
  assert.equal(pair.paired, true);
  assert.equal(pair.dark.lightScene, false);
  assert.equal(pair.light.lightScene, true);
  for (const side of [pair.dark, pair.light]) {
    assert.deepEqual(side.tags, metadataFixture.tags);
    assert.equal(side.family, metadataFixture.family);
    assert.equal(side.createdAt, metadataFixture.createdAt);
  }
  assert.deepEqual(semantic(await readPlate(page)), semantic(beforeCounterpart), "Counterpart changed the visible plate");
  passed("Derive counterpart preserves the active workspace");

  // Explicitly test the approval control using disposable fixtures only. These
  // test documents are never promoted into a user palette vault or shelf.
  await page.locator("#sealPlate").click();
  await page.waitForFunction(() => state.review.status === "sealed");
  await page.locator("#saveBtn").click();
  await page.waitForFunction(() => state.shelf.length === 1);
  const sealed = await readPlate(page);
  await page.locator('[data-fmt="json"]').click();
  const exported = JSON.parse(await download(page, "#downloadBtn", "sealed-fixture.json"));
  assert.deepEqual(exported, sealed);
  async function checkPlate(plate) {
    const response = await context.request.post(new URL("/api/check", baseURL).href, { data: plate });
    assert.equal(response.status(), 200, `Check failed: ${await response.text()}`);
    return response.json();
  }
  const verified = await checkPlate(exported);
  assert.equal(verified.contractValid, true);
  assert.equal(verified.agentEligible, true);
  const tampered = { ...exported, name: "E2E changed after sealing" };
  assert.equal((await checkPlate(tampered)).agentEligible, false);
  passed("Browser JSON export and Go seal verification agree; changed content fails closed");

  const bundleResponse = page.waitForResponse(r => new URL(r.url()).pathname === "/api/bundle");
  const bundleBytes = await download(page, "#bundleBtn", "retained-sources.zip");
  assert.equal((await bundleResponse).status(), 200);
  const bundled = await checkSingleBundle(bundleBytes, sealed, true);
  assert.equal((await checkPlate(bundled)).agentEligible, true);
  passed("Go single bundle contains valid exports, preview, original source bytes, and matching seal");
  await page.screenshot({ path: path.join(output, "sealed-palette.png"), fullPage: true });

  await loadImages(page, [imagePath("pastel-prisms.png")], "E2E second fixture");
  await forge(page);
  await forcePolarity(page, "dark");
  await page.locator("#saveBtn").click();
  await page.waitForFunction(() => state.shelf.length === 2);
  await page.locator('[data-a="0"]').click();
  await page.locator('[data-b="1"]').click();
  await page.locator("#pairBtn").click();
  await page.waitForFunction(() => !!state.theme && state.theme.paired);
  assert.match(await page.locator("#themeOut").inputValue(), /data-theme="light"/);
  await page.locator('[data-tfmt="godot"]').click();
  assert.match(await page.locator("#themeOut").inputValue(), /plate\(light: bool(?: = false)?\)/);
  for (const surface of ["forms", "table", "approval", "harness"]) {
    await page.locator(`[data-surf="${surface}"]`).click();
    assert.ok((await page.locator("#harnessHost").innerText()).trim());
  }
  passed("Shelf comparison, preview surfaces, and paired CSS/Godot theme exports");

  await page.locator("#unpairBtn").click();
  const pairingGate = await delayedExtraction(page);
  await page.locator("#deriveBtn").click();
  await pairingGate.started();
  await page.locator("#pairBtn").click();
  const chosenPair = await page.evaluate(() => [state.theme.dark.id, state.theme.light.id].sort());
  assert.ok(chosenPair.every(Boolean), "Pairing should use the saved shelf identities");
  await pairingGate.finish();
  assert.deepEqual(await page.evaluate(() => [state.theme.dark.id, state.theme.light.id].sort()), chosenPair);
  passed("A pending derived counterpart cannot overwrite a later A/B pairing");

  const vaultResponse = page.waitForResponse(r => new URL(r.url()).pathname === "/api/vault");
  const vaultFiles = unzip(await download(page, "#vaultExport", "palette-vault.zip"));
  assert.equal((await vaultResponse).status(), 200);
  const index = JSON.parse(vaultFiles.get("index.json"));
  assert.equal(index.count, 2);
  const documents = index.entries.map(entry => JSON.parse(vaultFiles.get(entry.path)));
  assert.equal(documents.filter(plate => plate.sources.some(source => source.retained === false)).length, 1);
  for (const entry of index.entries) {
    const plate = JSON.parse(vaultFiles.get(entry.path));
    assert.equal(plate.name, entry.name);
    assert.deepEqual(Object.fromEntries(plate.core.map(token => [token.id || token.key, token.hex])), entry.roles);
    checkPreview(vaultFiles.get(entry.path.replace("palette.json", "preview.png")));
  }
  passed("Go vault export indexes both saved palettes and labels unavailable source bytes");

  const shelf = await page.evaluate(() => JSON.parse(JSON.stringify(state.shelf)));
  await page.reload();
  await page.waitForFunction(() => state.shelf.length === 2);
  assert.deepEqual(await page.evaluate(() => JSON.parse(JSON.stringify(state.shelf))), shelf);
  await page.locator('[data-use="1"]').click();
  assert.equal((await readPlate(page)).review.agentEligible, true);
  await page.locator("#plateFile").setInputFiles(path.join(output, "sealed-fixture.json"));
  await page.waitForFunction(() => state.activeShelfId === null && state.core.length === 8);
  const reimported = await readPlate(page);
  assert.deepEqual(semantic(reimported), semantic(sealed));
  assert.equal(reimported.review.agentEligible, true, "Importing the browser's own sealed JSON lost its seal");
  assert.equal(reimported.review.fingerprint.value, sealed.review.fingerprint.value);
  assert.equal((await checkPlate(reimported)).agentEligible, true);
  const missingSourceBundle = await checkSingleBundle(await download(page, "#bundleBtn", "metadata-only-sources.zip"), reimported, false);
  assert.equal((await checkPlate(missingSourceBundle)).agentEligible, true);
  passed("Shelf reload and JSON import preserve metadata and seals; missing-source bundles remain truthful");

  await loadImages(page, [imagePath("empty-neon-street.png")], "E2E request recovery");
  await forge(page);
  const beforeFailure = semantic(await readPlate(page));
  expectedFailure = true;
  await page.route("**/api/extract", route => route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ error: "E2E injected extraction failure" }) }));
  await page.locator("#extractBtn").click();
  await page.waitForFunction(() => !document.getElementById("extractBtn").disabled && document.getElementById("toastMsg").textContent.includes("E2E injected"));
  assert.deepEqual(semantic(await readPlate(page)), beforeFailure, "Failed request changed the current palette");
  await page.unroute("**/api/extract");
  expectedFailure = false;
  passed("Failed Go request preserves the prior palette and reports the error");

  await page.locator('[data-pol="auto"]').click();
  await page.waitForFunction(() => state.polarity === "auto" && goPending === null);
  const beforePolarityRace = semantic(await readPlate(page));
  const polarityGate = await delayedExtraction(page);
  await page.locator('[data-pol="dark"]').click();
  await polarityGate.started();
  await page.locator('[data-pol="auto"]').click();
  await polarityGate.finish();
  assert.equal(await page.evaluate(() => state.polarity), "auto");
  assert.equal(await page.locator('[data-pol="auto"]').getAttribute("aria-pressed"), "true");
  assert.deepEqual(semantic(await readPlate(page)), beforePolarityRace);
  passed("Returning to the current Auto mode cancels a pending forced-polarity result");

  const clearGate = await delayedExtraction(page);
  await page.locator("#extractBtn").click();
  await clearGate.started();
  await page.locator("#clearAll").click();
  await page.waitForFunction(() => state.core.length === 0 && state.stills.length === 0);
  await clearGate.finish();
  await page.waitForFunction(() => !document.getElementById("extractBtn").disabled);
  assert.deepEqual(await page.evaluate(() => ({ core: state.core.length, stills: state.stills.length })), { core: 0, stills: 0 });
  passed("Late Go response cannot restore a workspace cleared while extraction was pending");

  // A test-only delay at the browser decode boundary exposes partial ingestion
  // deterministically. Restore the native function before leaving this page.
  await page.evaluate(() => {
    const original = window.createImageBitmap;
    globalThis.__e2eBitmapGate = { original, calls: 0, waiting: false, finished: false, release: null };
    window.createImageBitmap = async (...values) => {
      const bitmap = await original(...values);
      const gate = globalThis.__e2eBitmapGate;
      if (++gate.calls === 2) {
        gate.waiting = true;
        await new Promise(resolve => { gate.release = resolve; });
        gate.finished = true;
      }
      return bitmap;
    };
  });
  try {
    await page.locator("#file").setInputFiles([imagePath("empty-neon-street.png"), imagePath("pastel-prisms.png")]);
    await page.waitForFunction(() => globalThis.__e2eBitmapGate.waiting);
    assert.equal(await page.evaluate(() => state.stills.length), 0, "A partially decoded batch became visible");
    await page.locator("#clearAll").click();
    await page.evaluate(() => globalThis.__e2eBitmapGate.release());
    await page.waitForFunction(() => globalThis.__e2eBitmapGate.finished);
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    assert.deepEqual(await page.evaluate(() => ({ core: state.core.length, stills: state.stills.length })), { core: 0, stills: 0 });
  } finally {
    await page.evaluate(() => {
      window.createImageBitmap = globalThis.__e2eBitmapGate.original;
      globalThis.__e2eBitmapGate.release?.();
      delete globalThis.__e2eBitmapGate;
    });
  }
  passed("Image batches commit together; cancelling decode leaves no partial stills");

  assert.deepEqual(report.errors, [], "Unexpected browser errors");
  assert.ok(report.network.some(entry => entry.path === "/api/extract" && entry.status === 200));
  assert.ok(report.network.some(entry => entry.path === "/api/bundle" && entry.status === 200));
  assert.ok(report.network.some(entry => entry.path === "/api/vault" && entry.status === 200));
  report.status = "passed";
} catch (error) {
  report.status = "failed";
  report.failure = error.stack || String(error);
  console.error(report.failure);
  process.exitCode = 1;
  if (page) {
    report.browserState = await page.evaluate(() => ({
      url: location.href, toast: document.getElementById("toastMsg")?.textContent,
      engine: state.backend, stills: state.stills.length, core: state.core.length,
      variant: state.variant, polarity: state.polarity, review: state.review.status
    })).catch(() => null);
    await page.screenshot({ path: path.join(output, "failure.png"), fullPage: true }).catch(() => {});
  }
} finally {
  report.finishedAt = new Date().toISOString();
  await fs.writeFile(path.join(output, "report.json"), JSON.stringify(report, null, 2) + "\n");
  if (browser) await browser.close();
  console.log(`Browser verification ${report.status}: ${report.checks.length} checks. Report: ${path.join(output, "report.json")}`);
}

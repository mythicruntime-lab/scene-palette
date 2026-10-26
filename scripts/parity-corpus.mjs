// Exact browser/Go parity and inspectable image-corpus diagnostics.
// Dev-only Playwright dependency; the distributed HTML remains zero-install.
// Run long suites through the workstation's jobgate. See --help for the manifest.
import assert from "node:assert/strict";
import crypto from "node:crypto";
import fs from "node:fs/promises";
import path from "node:path";
import { createRequire } from "node:module";
import { isDeepStrictEqual } from "node:util";
import { fileURLToPath, pathToFileURL } from "node:url";

const require = createRequire(import.meta.url);
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const args = process.argv.slice(2);
if (args.includes("--help")) {
  console.log(`Usage: node scripts/parity-corpus.mjs [--url http://127.0.0.1:8765/]
  [--manifest path/to/manifest.json] [--output bin/corpus/RUN] [--html index.html]
  [--browser /path/to/browser] [--channel msedge] [--require-readable]

Manifest: {"images":[{"id":"illustration-01","file":"images/illustration.png",
  "category":"illustration","source":"source title or URL","corpus":"fresh"}]}
Image paths are relative to the manifest. All additional metadata is preserved.
corpus can be fresh or regression. Explicit manifests default to fresh; omitting
--manifest uses the ten generated reference patterns and marks them regression.
Optional sha256 values are verified. A fresh label describes the test corpus,
not proof that a human has performed a usability study.

Every available variant: baseline auto/dark/light, then auto at fast/accurate
quality, low/high signal floors, and zero found inks (up to 48 plates/image).
Also tests pooled first+last images in all three modes. This is a bounded settings
matrix, not every Cartesian combination. Artifacts include report.html,
summary.json, cases.json, differences.json, thumbnails and default palettes.
Exit 1 means parity, input, browser or execution failure. --require-readable
also fails if any generated, unlocked core ink misses its contrast target on
either surface in either engine. Locked/manual exclusions are reported. Found
inks and other diagnostic quality flags do not affect the exit code.

Install playwright for development or set SCENE_PALETTE_PLAYWRIGHT / NODE_PATH.
SCENE_PALETTE_BROWSER and SCENE_PALETTE_BROWSER_CHANNEL are also supported.`);
  process.exit(0);
}
const validOptions = new Set(["--url", "--manifest", "--output", "--html", "--browser", "--channel"]);
for (let i = 0; i < args.length; i++) {
  if (args[i] === "--require-readable") continue;
  assert.ok(validOptions.has(args[i]), `Unknown option: ${args[i]}`);
  assert.ok(args[i + 1] && !args[i + 1].startsWith("--"), `${args[i]} needs a value`);
  i++;
}
const requireReadable = args.includes("--require-readable");
function option(name, fallback) {
  const at = args.indexOf(name);
  return at === -1 ? fallback : args[at + 1];
}
const baseURL = new URL(option("--url", "http://127.0.0.1:8765/"));
const standaloneHTML = path.resolve(root, option("--html", "index.html"));
assert.ok(["127.0.0.1", "localhost", "[::1]"].includes(baseURL.hostname), "Server must be local");
assert.equal(baseURL.protocol, "http:", "Use the local HTTP server");
assert.ok(!baseURL.username && !baseURL.password, "Server URL must not contain credentials");
const output = path.resolve(root, option("--output", `bin/corpus/${new Date().toISOString().replace(/[:.]/g, "-")}`));
const artifactRoot = path.join(root, "bin");
assert.ok(output.startsWith(artifactRoot + path.sep), "Outputs must stay in an ignored bin subdirectory");
await fs.mkdir(output, { recursive: true });
// Never mix a new run with stale evidence from an earlier one.
assert.equal((await fs.readdir(output)).length, 0, `Output directory must be empty: ${output}`);
await fs.mkdir(path.join(output, "images"));
await fs.mkdir(path.join(output, "palettes"));
const sha256 = bytes => crypto.createHash("sha256").update(bytes).digest("hex");
const writeJSON = (file, value) => fs.writeFile(path.join(output, file), JSON.stringify(value, null, 2) + "\n");
const configurations = [
  { id: "balanced", budget: 140000, chromaMin: 0.085, foundInks: 6, modes: ["auto", "dark", "light"] },
  { id: "fast", budget: 45000, chromaMin: 0.085, foundInks: 6, modes: ["auto"] },
  { id: "accurate", budget: 400000, chromaMin: 0.085, foundInks: 6, modes: ["auto"] },
  { id: "faint-signals", budget: 140000, chromaMin: 0.04, foundInks: 10, modes: ["auto"] },
  { id: "strong-signals", budget: 140000, chromaMin: 0.16, foundInks: 6, modes: ["auto"] },
  { id: "no-found-inks", budget: 140000, chromaMin: 0.085, foundInks: 0, modes: ["auto"] }
];
const report = {
  schema: 1, startedAt: new Date().toISOString(), url: baseURL.href, runnerNodeVersion: process.version,
  methodology: {
    reference: "Selected standalone HTML JavaScript extraction in file://, using actual decoded working canvases; observed extraction versions recorded below",
    candidate: "Local Go API through the served app, using the same input files and browser",
    comparison: "Exact deep equality of every snapshot field except createdAt and review; no numeric tolerance",
    interaction: "File-input uploads, settings and Forge through the UI; awaited actual setPolarity/applyVariant functions select cached builds",
    isolation: "Separate clean Go and standalone browser contexts for every image and pooled case; external requests blocked and standalone context offline",
    settings: configurations, pooling: "First and last image, equal still weighting, baseline in all three polarities",
    diagnostics: "Measured contrast uses unrounded colors on both surfaces: ink >=4.5, other core nonsurfaces >=3. Found inks are raw paint and have no readability guarantee. Accent DeltaE is Euclidean OKLab (not CIEDE2000). Flags are evidence for visual review, not subjective quality verdicts.",
    thresholds: { inkContrast: 4.5, otherCoreContrast: 3, nearAccentDeltaE: 0.07, significantRawToFinalDeltaE: 0.10 },
    readabilityGate: { enabled: requireReadable, scope: "Generated unlocked core inks on both surfaces, in both engines; locked or manual-origin tokens explicitly excluded and reported. Raw found inks are excluded." },
    limitation: "This is automated testing in fresh browser profiles, with a contact sheet for separate visual inspection; no recruited-human usability study is claimed. Synthetic fixtures and retained references do not establish fresh-source coverage. Original committed golden-reference tests must be run separately."
  }, extractionVersions: { go: [], reference: [] }, images: [], pools: [], errors: [], differences: [], cases: []
};
let browser;

// Comparing the full durable snapshot makes future extraction fields part of
// parity automatically. Only wall-clock metadata and review workflow are omitted.
function comparable(plate) {
  const { createdAt, review, ...extraction } = plate;
  return extraction;
}
function diff(expected, actual, at = "$") {
  if (isDeepStrictEqual(expected, actual)) return [];
  if (expected && actual && typeof expected === "object" && typeof actual === "object" && Array.isArray(expected) === Array.isArray(actual)) {
    const keys = new Set([...Object.keys(expected), ...Object.keys(actual)]);
    return [...keys].flatMap(key => diff(expected[key], actual[key], `${at}${Array.isArray(expected) ? `[${key}]` : `.${key}`}`));
  }
  return [{ path: at, expected: expected === undefined ? { missing: true } : expected, actual: actual === undefined ? { missing: true } : actual }];
}
async function readPlate(page) {
  return page.evaluate(() => JSON.parse(JSON.stringify(snapshot())));
}
function assertSelection(plate, config, mode, variant, label) {
  assert.equal(plate.core.length, 8, `${label}: generated core is incomplete`);
  assert.ok(Number.isInteger(plate.params.variantCount) && plate.params.variantCount > 0, `${label}: generated variants are missing`);
  for (const field of ["budget", "chromaMin", "foundInks"]) assert.equal(plate.params[field], config[field], `${label}: ${field} was not applied`);
  assert.equal(plate.params.polarityMode, mode, `${label}: requested polarity was not applied`);
  if (mode !== "auto") assert.equal(plate.lightScene, mode === "light", `${label}: forced scene polarity was not applied`);
  if (variant !== undefined) {
    assert.ok(variant < plate.params.variantCount, `${label}: requested variant does not exist`);
    assert.equal(plate.params.variant, variant, `${label}: requested variant was not applied`);
  }
}
async function setControls(page, config) {
  await page.locator("#quality").selectOption(String(config.budget));
  for (const [id, value] of [["chroma", config.chromaMin * 1000], ["extra", config.foundInks]]) {
    await page.locator(`#${id}`).evaluate((input, next) => {
      input.value = String(next);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    }, value);
  }
  await page.evaluate(async () => { await setPolarity("auto"); });
  assert.equal(await page.evaluate(() => state.polarity), "auto", "Auto polarity selection failed");
}
async function forge(page, isGo, config) {
  const response = isGo ? page.waitForResponse(r => new URL(r.url()).pathname === "/api/extract" && r.request().method() === "POST") : null;
  await page.locator("#extractBtn").click();
  if (response) {
    const result = await response;
    assert.equal(result.status(), 200, `Go extraction failed: ${await result.text()}`);
  }
  await page.waitForFunction(expected => state.core.length === 8 && !document.getElementById("extractBtn").disabled &&
    state.params.budget === expected.budget && state.params.chromaMin === expected.chromaMin && state.params.foundInks === expected.foundInks, config);
  assert.equal(await page.evaluate(() => !!state.goHistogram), isGo, "Unexpected extraction engine");
  assertSelection(await readPlate(page), config, "auto", 0, isGo ? "Go Forge" : "JavaScript Forge");
}
async function metrics(page, plate) {
  return page.evaluate(p => {
    const color = token => ({ r: token.rgb[0], g: token.rgb[1], b: token.rgb[2] });
    const parse = value => /^#[a-f\d]{6}$/i.test(value || "") ? { r: parseInt(value.slice(1, 3), 16), g: parseInt(value.slice(3, 5), 16), b: parseInt(value.slice(5, 7), 16) } : null;
    const ground = color(p.core.find(t => t.id === "ground"));
    const panel = color(p.core.find(t => t.id === "panel"));
    const readability = p.core.filter(t => !["ground", "panel"].includes(t.id)).map(t => {
      const target = t.id === "ink" ? 4.5 : 3;
      const onGround = contrast(color(t), ground), onPanel = contrast(color(t), panel);
      const locked = !!t.locked;
      const generated = ["structure", "signal", "derived", "fallback"].includes(t.origin?.kind);
      return { role: t.id, target, onGround, onPanel, passes: onGround >= target && onPanel >= target,
        locked, originKind: t.origin?.kind || "unknown", gateEligible: generated && !locked,
        excludedReason: locked ? "locked token" : generated ? null : "manual or unknown origin" };
    });
    const accents = p.core.filter(t => ["observed", "attention", "rupture", "gold"].includes(t.id));
    const accentDistances = accents.flatMap((a, i) => accents.slice(i + 1).map(b => ({ roles: [a.id, b.id], deltaE: deltaE(color(a), color(b)) })));
    const rawToFinal = p.core.flatMap(t => {
      const raw = parse(t.origin?.raw);
      return raw ? [{ role: t.id, originKind: t.origin.kind, raw: t.origin.raw, final: t.hex, deltaE: deltaE(raw, color(t)), repairs: (t.origin.repairs || []).length }] : [];
    });
    const repairSteps = p.core.flatMap(t => (t.origin?.repairs || []).flatMap(r => {
      const from = parse(r.from), to = parse(r.to);
      return from && to ? [{ role: t.id, operation: r.op, against: r.against, from: r.from, to: r.to, deltaE: deltaE(from, to) }] : [];
    }));
    const derivedRoles = accents.filter(t => t.derived).map(t => t.id);
    const lowContrast = readability.filter(t => !t.passes);
    const nearAccents = accentDistances.filter(pair => pair.deltaE < 0.07);
    const substantialChanges = rawToFinal.filter(t => t.deltaE > 0.10);
    const signalRawToFinal = rawToFinal.filter(t => t.originKind === "signal");
    const substantialSignals = signalRawToFinal.filter(t => t.deltaE > 0.10);
    const flags = [];
    if (lowContrast.length) flags.push({ kind: "contrast-below-target", roles: lowContrast.map(t => t.role) });
    if (nearAccents.length) flags.push({ kind: "similar-accents", pairs: nearAccents.map(p => p.roles) });
    if (derivedRoles.length) flags.push({ kind: "derived-accents", roles: derivedRoles });
    if (substantialChanges.length) flags.push({ kind: "substantial-source-adjustment", roles: substantialChanges.map(t => t.role) });
    if (substantialSignals.length) flags.push({ kind: "substantial-signal-adjustment", roles: substantialSignals.map(t => t.role) });
    return { readability, accentDistances, minimumAccentDeltaE: Math.min(...accentDistances.map(p => p.deltaE)),
      derivedCount: derivedRoles.length, derivedRoles, rawToFinal, signalRawToFinal, repairSteps,
      maximumSignalRawToFinalDeltaE: signalRawToFinal.length ? Math.max(...signalRawToFinal.map(t => t.deltaE)) : null,
      autoPolarity: state.autoLight ? "light" : "dark", actualPolarity: p.polarity, medianL: p.medianL,
      variantCount: p.params.variantCount, foundCount: p.found.length, flags };
  }, plate);
}
async function thumbnail(page, file) {
  const png = await page.evaluate(() => {
    // The stage is a render target and may not have painted before Forge.
    // Read the decoded working image, which the extraction also consumes.
    const original = state.stills[0]?.canvas;
    if (!original || !original.width || !original.height) throw new Error("Decoded input canvas is unavailable");
    const canvas = document.createElement("canvas");
    const scale = Math.min(1, 520 / Math.max(original.width, original.height));
    canvas.width = Math.max(1, Math.round(original.width * scale));
    canvas.height = Math.max(1, Math.round(original.height * scale));
    canvas.getContext("2d").drawImage(original, 0, 0, canvas.width, canvas.height);
    return canvas.toDataURL("image/png").split(",")[1];
  });
  await fs.writeFile(path.join(output, file), Buffer.from(png, "base64"));
}
async function makePage(address, label) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: "block", offline: address.startsWith("file:") });
  context.setDefaultTimeout(20000);
  await context.route("**/*", route => {
    const url = new URL(route.request().url());
    if (["http:", "https:"].includes(url.protocol) && (address.startsWith("file:") || url.origin !== baseURL.origin)) {
      report.errors.push({ sample: label, error: `Blocked unexpected external request: ${url.origin}${url.pathname}` });
      return route.abort();
    }
    return route.continue();
  });
  const page = await context.newPage();
  page.on("pageerror", error => report.errors.push({ sample: label, error: error.stack || error.message }));
  page.on("console", message => {
    if (message.type() === "error" && !message.location().url?.endsWith("/favicon.ico")) report.errors.push({ sample: label, error: message.text() });
  });
  page.on("response", response => {
    const url = new URL(response.url());
    if (url.pathname.startsWith("/api/") && !response.ok()) report.errors.push({ sample: label, error: `API ${response.request().method()} ${url.pathname}: HTTP ${response.status()}` });
  });
  page.on("requestfailed", request => {
    const url = new URL(request.url());
    if (url.pathname.startsWith("/api/")) report.errors.push({ sample: label, error: `API ${request.method()} ${url.pathname}: ${request.failure()?.errorText || "network request failed"}` });
  });
  await page.goto(address);
  await page.waitForFunction(() => typeof snapshot === "function");
  return { page, context };
}
async function runSample(sample, files, matrix) {
  const holders = [];
  sample.cases = 0;
  sample.mismatches = 0;
  sample.readabilityFailureCases = 0;
  sample.readabilityExcludedTokens = 0;
  sample.qualityFlags = {};
  try {
    const goHolder = await makePage(baseURL.href, `${sample.id}/go`); holders.push(goHolder);
    const jsHolder = await makePage(pathToFileURL(standaloneHTML).href, `${sample.id}/javascript`); holders.push(jsHolder);
    const goPage = goHolder.page, jsPage = jsHolder.page;
    for (const page of [goPage, jsPage]) {
      await page.locator("#plateName").fill(sample.label || sample.id);
      await page.locator("#file").setInputFiles(files);
      await page.waitForFunction(count => state.stills.length === count, files.length);
    }
    sample.thumbnail = `images/${sample.id}.png`;
    sample.thumbnailSource = path.basename(files[0]);
    await thumbnail(goPage, sample.thumbnail);
    for (const config of matrix) {
      for (const page of [goPage, jsPage]) await setControls(page, config);
      await forge(goPage, true, config);
      await forge(jsPage, false, config);
      for (const mode of config.modes) {
        for (const [engine, page] of [["Go", goPage], ["JavaScript", jsPage]]) {
          await page.evaluate(async selected => { await setPolarity(selected); }, mode);
          assertSelection(await readPlate(page), config, mode, undefined, `${engine} polarity selection`);
        }
        const [goCount, jsCount] = await Promise.all([goPage, jsPage].map(page => page.evaluate(() => state.sets.length)));
        assert.ok(goCount > 0 && jsCount > 0, "Extraction did not produce any variants");
        if (goCount !== jsCount) {
          sample.mismatches++;
          report.differences.push({ sample: sample.id, config: config.id, mode, fields: [{ path: "$.params.variantCount", expected: jsCount, actual: goCount }] });
        }
        // Missing variants fail parity above. Compare shared indices without
        // allowing the app's modulo wrapping to claim nonexistent coverage.
        for (let variant = 0; variant < Math.min(goCount, jsCount); variant++) {
          const caseId = `${sample.id}/${config.id}/${mode}/${variant}`;
          // These are the production handlers, awaited so Go's asynchronous
          // cached rebuild cannot race the snapshot. No extraction is mocked.
          for (const page of [goPage, jsPage]) await page.evaluate(async v => { await applyVariant(v); }, variant);
          const [candidate, reference] = await Promise.all([readPlate(goPage), readPlate(jsPage)]);
          assertSelection(candidate, config, mode, variant, `${caseId} Go`);
          assertSelection(reference, config, mode, variant, `${caseId} JavaScript`);
          for (const [engine, plate] of [["go", candidate], ["reference", reference]]) {
            const version = plate.generator?.extraction ?? null;
            if (!report.extractionVersions[engine].includes(version)) report.extractionVersions[engine].push(version);
          }
          const fields = diff(comparable(reference), comparable(candidate));
          const measured = await metrics(goPage, candidate);
          const referenceReadability = (await metrics(jsPage, reference)).readability;
          const allReadability = [["go", measured.readability], ["reference", referenceReadability]];
          const readabilityFailures = allReadability.flatMap(([engine, rows]) => rows.filter(r => r.gateEligible && !r.passes).map(r => ({ engine, ...r })));
          const readabilityExcluded = allReadability.flatMap(([engine, rows]) => rows.filter(r => !r.gateEligible).map(r => ({ engine, ...r })));
          const result = { id: caseId, sample: sample.id, config: config.id, mode, variant, parity: fields.length ? "failed" : "passed",
            readability: readabilityFailures.length ? "failed" : "passed", readabilityFailures, readabilityExcluded, referenceReadability, metrics: measured };
          report.cases.push(result); sample.cases++;
          sample.readabilityExcludedTokens += readabilityExcluded.length;
          if (readabilityFailures.length) {
            sample.readabilityFailureCases++;
            if (requireReadable) console.error(`FAIL ${caseId}: ${readabilityFailures.length} generated core contrast failures across both engines`);
          }
          if (fields.length) {
            sample.mismatches++;
            report.differences.push({ id: caseId, fields, expected: comparable(reference), actual: comparable(candidate) });
            console.error(`FAIL ${caseId}: ${fields.length} differing fields`);
          }
          for (const flag of measured.flags) sample.qualityFlags[flag.kind] = (sample.qualityFlags[flag.kind] || 0) + 1;
          if (config.id === "balanced" && mode === "auto" && variant === 0) {
            sample.defaultMetrics = measured;
            sample.palette = `palettes/${sample.id}.json`;
            sample.defaultCore = candidate.core.map(t => ({ role: t.id, name: t.name, hex: t.hex, derived: t.derived }));
            sample.defaultFound = candidate.found.map(t => ({ name: t.name, hex: t.hex }));
            await writeJSON(sample.palette, candidate);
          }
        }
      }
    }
    console.log(`${sample.mismatches || requireReadable && sample.readabilityFailureCases ? "FAIL" : "PASS"} ${sample.id}: ${sample.cases} exact comparisons; ${sample.readabilityFailureCases} generated-readability failures; ${sample.readabilityExcludedTokens} token exclusions; diagnostic flags ${JSON.stringify(sample.qualityFlags)}`);
  } catch (error) {
    sample.error = error.stack || String(error);
    report.errors.push({ sample: sample.id, error: sample.error });
    console.error(`ERROR ${sample.id}: ${error.message || error}`);
  } finally {
    for (const holder of holders) await holder.context.close();
  }
}
const escapeHTML = value => String(value ?? "").replace(/[&<>"']/g, c => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
function card(sample) {
  const esc = escapeHTML, m = sample.defaultMetrics;
  const swatches = (sample.defaultCore || []).map(t => `<div class="token"><i style="background:${esc(t.hex)}"></i><b>${esc(t.role)}${t.derived ? " *" : ""}</b><code>${esc(t.hex)}</code></div>`).join("");
  const flags = m?.flags.map(f => `<li>${esc(f.kind)}: ${esc((f.roles || f.pairs || []).join(", "))}</li>`).join("") || "<li>No default diagnostic flags.</li>";
  return `<article><header><h2>${esc(sample.label || sample.id)}</h2><span>${esc(sample.corpus || "pooled")} · ${sample.mismatches || sample.error || requireReadable && sample.readabilityFailureCases ? "CHECK FAILURES" : "required checks pass"}</span></header>
    <p>${esc(sample.category || (sample.tags || []).join(" · "))}</p>
    ${sample.thumbnail ? `<img src="${sample.thumbnail}" alt="Input: ${esc(sample.thumbnailSource || sample.label || sample.id)}">` : ""}
    ${sample.corpus === "pooled" ? `<p>Thumbnail shows the first pooled source: ${esc(sample.thumbnailSource)}. Extraction uses both sources.</p>` : ""}
    <div class="swatches">${swatches}</div>
    ${m ? `<p><b>${esc(m.actualPolarity)}</b> auto plate · ${m.variantCount} variants · ${m.derivedCount}/4 derived accents · ${m.foundCount} found inks</p>
      <p>Closest accent ΔE ${m.minimumAccentDeltaE.toFixed(4)} · ${m.readability.filter(x => x.passes).length}/6 core inks meet both surfaces</p>` : ""}
    <ul>${flags}</ul><details><summary>Evidence and all-case diagnostic counts</summary><pre>${esc(JSON.stringify({ source: sample.source, metadata: sample.metadata, inputSha256: sample.sha256, cases: sample.cases, readabilityFailureCases: sample.readabilityFailureCases, readabilityExcludedTokens: sample.readabilityExcludedTokens, qualityFlags: sample.qualityFlags, defaultMetrics: m, error: sample.error }, null, 2))}</pre></details>
    ${sample.palette ? `<p><a href="${sample.palette}">Default palette JSON</a></p>` : ""}</article>`;
}
async function finish() {
  report.completedAt = new Date().toISOString();
  const allSamples = [...report.images, ...report.pools];
  report.totals = {
    images: report.images.length, uniqueImages: new Set(report.images.map(s => s.sha256)).size,
    freshImages: report.images.filter(s => s.corpus === "fresh").length,
    regressionImages: report.images.filter(s => s.corpus === "regression").length,
    categories: [...new Set(report.images.map(s => s.category).filter(Boolean))],
    pooledCases: report.pools.length, completedSamples: allSamples.filter(s => !s.error && s.cases > 0).length,
    comparisons: report.cases.length, mismatchingComparisons: report.cases.filter(c => c.parity === "failed").length,
    differences: report.differences.length, errors: report.errors.length,
    diagnosticCases: report.cases.filter(c => c.metrics.flags.length).length,
    contrastFailureCases: report.cases.filter(c => c.metrics.readability.some(r => !r.passes)).length,
    generatedReadabilityFailureCases: report.cases.filter(c => c.readability === "failed").length,
    readabilityExcludedTokens: report.cases.reduce((n, c) => n + c.readabilityExcluded.length, 0),
    unreadableExcludedTokens: report.cases.reduce((n, c) => n + c.readabilityExcluded.filter(r => !r.passes).length, 0)
  };
  report.parityStatus = report.errors.length || report.differences.length ? "failed" : "passed";
  report.readabilityStatus = report.errors.length ? "incomplete" : report.totals.generatedReadabilityFailureCases ? "failed" : "passed";
  report.status = report.parityStatus === "failed" || requireReadable && report.readabilityStatus !== "passed" ? "failed" : "passed";
  await writeJSON("cases.json", report.cases);
  await writeJSON("differences.json", report.differences);
  const { cases, differences, ...summary } = report;
  summary.artifacts = { cases: "cases.json", differences: "differences.json", contactSheet: "report.html" };
  await writeJSON("summary.json", summary);
  await fs.writeFile(path.join(output, "report.html"), `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Scene Palette corpus report</title>
    <style>body{margin:0;background:#11161c;color:#e6ecf2;font:15px/1.5 system-ui,sans-serif}main{max-width:1500px;margin:auto;padding:32px}h1{margin:0}h2{font-size:19px;margin:0}a{color:#8bd7ff}p{color:#b7c4d1}article{background:#1b2430;border:1px solid #354351;border-radius:10px;padding:20px;min-width:0}article img{display:block;width:100%;height:280px;object-fit:contain;background:#10151b}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(360px,1fr));gap:20px}.swatches{display:grid;grid-template-columns:repeat(4,1fr);gap:8px;margin-top:16px}.token{font-size:11px;display:grid;gap:2px}.token i{height:38px;border-radius:4px;border:1px solid #ffffff30}.token code{color:#b7c4d1}header span{font-size:12px;color:#9cc7b2}pre{white-space:pre-wrap;overflow-wrap:anywhere;font-size:11px;max-height:480px;overflow:auto}li{font-size:13px}details{border-top:1px solid #354351;padding-top:10px}summary{cursor:pointer}.overview{background:#1b2430;padding:20px;margin:24px 0;border-radius:8px}</style>
    <main><h1>Scene Palette corpus report</h1><p>${escapeHTML(report.startedAt)} · overall required checks: <b>${escapeHTML(report.status)}</b> · exact Go / standalone JavaScript parity: <b>${escapeHTML(report.parityStatus)}</b></p>
    <div class="overview"><p>${report.totals.comparisons} comparisons · ${report.totals.mismatchingComparisons} mismatches · ${report.totals.errors} execution errors.</p>
    <p>${report.totals.freshImages} fresh images · ${report.totals.regressionImages} retained regression images · ${report.totals.uniqueImages} unique input hashes. Fresh-source coverage needs its own diverse manifest; retained examples are not a substitute.</p>
    <p>Generated-core readability: <b>${escapeHTML(report.readabilityStatus)}</b> · required gate ${requireReadable ? "enabled" : "disabled"} · ${report.totals.generatedReadabilityFailureCases} failing cases · ${report.totals.readabilityExcludedTokens} locked/manual token exclusions (${report.totals.unreadableExcludedTokens} below target). Excluded tokens do not inherit a generated-readability pass. Observed extraction versions: Go ${escapeHTML(report.extractionVersions.go.join(", "))}; standalone ${escapeHTML(report.extractionVersions.reference.join(", "))}.</p>
    <p>Cards show the default auto palette. * marks a derived accent. Contrast is checked without rounding: body ink 4.5:1 and other core inks 3:1 on both ground and panel. Found inks are unmodified source colors. Close accents (&lt;0.07 OKLab ΔE), derived accents, and large source adjustments (&gt;0.10 ΔE) are diagnostics for review, not automatic aesthetic failures. Adjustments to real signal accents are recorded separately from derived colors and structural repairs.</p>
    <p>Both engines use a fresh browser profile for each sample; the standalone HTML runs with network access disabled. This is automated fresh-profile testing with a contact sheet for separate visual inspection, not a recruited-human usability study.</p>
    <p>All variants are tested at baseline auto/dark/light; auto also varies budgets 45k/140k/400k, signal floors .04/.085/.16 and found limits 0/6/10. This bounded matrix does not claim every combination. Cached builds call actual production handlers after UI file uploads and Forge.</p>
    <p><a href="summary.json">Summary and provenance</a> · <a href="cases.json">Every case and metric</a> · <a href="differences.json">Exact differences</a></p></div>
    <div class="grid">${allSamples.map(card).join("\n")}</div></main></html>`);
  console.log(`${report.status.toUpperCase()}: ${report.totals.comparisons} comparisons, ${report.totals.differences} mismatches, ${report.errors.length} errors, ${report.totals.generatedReadabilityFailureCases} generated-readability failures (gate ${requireReadable ? "enabled" : "disabled"}), ${report.totals.readabilityExcludedTokens} token exclusions; report ${path.join(output, "report.html")}`);
  if (report.status !== "passed") process.exitCode = 1;
}

try {
  const explicitManifest = option("--manifest");
  const manifestPath = path.resolve(root, explicitManifest || "testdata/reference-images/manifest.json");
  const manifestBytes = await fs.readFile(manifestPath);
  const manifest = JSON.parse(manifestBytes);
  assert.ok(Array.isArray(manifest.images) && manifest.images.length, "Manifest requires a nonempty images array");
  report.manifest = { path: manifestPath, sha256: sha256(manifestBytes), metadata: Object.fromEntries(Object.entries(manifest).filter(([key]) => key !== "images")) };
  const selected = explicitManifest ? manifest.images : manifest.images.filter(entry => entry.fixture);
  const used = new Set();
  const inputs = [];
  for (const entry of selected) {
    const file = explicitManifest ? entry.file : entry.fixture.file;
    assert.equal(typeof file, "string", "Each image requires a file path");
    const id = entry.id || entry.label || path.parse(file).name;
    assert.match(id, /^[a-zA-Z0-9][a-zA-Z0-9_-]*$/, `Use a simple unique image id: ${id}`);
    assert.ok(!used.has(id), `Duplicate image id: ${id}`); used.add(id);
    const absolute = path.resolve(path.dirname(manifestPath), file);
    const bytes = await fs.readFile(absolute), hash = sha256(bytes);
    const expectedHash = explicitManifest ? entry.sha256 : entry.fixture.sha256;
    if (expectedHash) assert.equal(hash, expectedHash.toLowerCase(), `Input hash mismatch: ${file}`);
    const corpus = explicitManifest ? entry.corpus || "fresh" : "regression";
    assert.ok(["fresh", "regression"].includes(corpus), `Unknown corpus classification: ${corpus}`);
    report.images.push({ id, label: entry.label || id, file, category: entry.category || null, tags: entry.tags || [], corpus,
      source: entry.source || (explicitManifest ? null : "Retained user-supplied reference fixture"), sha256: hash, bytes: bytes.length, metadata: entry });
    inputs.push(absolute);
  }
  assert.ok(inputs.length, "Manifest did not select any image files");
  let playwright;
  try { playwright = require(process.env.SCENE_PALETTE_PLAYWRIGHT || "playwright"); }
  catch { throw new Error("Install development playwright or set SCENE_PALETTE_PLAYWRIGHT / NODE_PATH"); }
  const executablePath = option("--browser", process.env.SCENE_PALETTE_BROWSER);
  let channel = option("--channel", process.env.SCENE_PALETTE_BROWSER_CHANNEL);
  if (!channel && !executablePath && process.platform === "win32") {
    const edge = "C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe";
    if (await fs.stat(edge).then(() => true, () => false)) channel = "msedge";
  }
  browser = await playwright.chromium.launch({ headless: true, executablePath, channel, args: ["--disable-gpu"] });
  report.browser = { version: browser.version(), channel: channel || null, executablePath: executablePath || null };
  const infoContext = await browser.newContext();
  try {
    const info = await infoContext.request.get(new URL("/api/info", baseURL).href, { timeout: 15000 });
    assert.equal(info.status(), 200, "Local Go /api/info is unavailable");
    report.server = await info.json(); assert.equal(report.server.engine, "go");
  } finally { await infoContext.close(); }
  report.standaloneHTML = { path: standaloneHTML, sha256: sha256(await fs.readFile(standaloneHTML)) };
  for (let i = 0; i < report.images.length; i++) await runSample(report.images[i], [inputs[i]], configurations);
  if (inputs.length > 1) {
    const pool = { id: "pooled-first-last", label: "Pooled first + last image", source: [report.images[0].id, report.images.at(-1).id], corpus: "pooled" };
    assert.ok(!used.has(pool.id), `Reserved image id: ${pool.id}`);
    report.pools.push(pool);
    await runSample(pool, [inputs[0], inputs.at(-1)], [configurations[0]]);
  }
} catch (error) {
  report.errors.push({ error: error.stack || String(error) });
  console.error(error.stack || error);
} finally {
  if (browser) await browser.close();
  await finish();
}

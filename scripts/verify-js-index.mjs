import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const source = fs.readFileSync(path.join(root, "index.html"), "utf8").replaceAll("\r\n", "\n");
const vaultRoot = path.resolve(root, process.argv[2] || path.join("testdata", "vault"));
const goldenPath = path.resolve(vaultRoot, process.argv[3] || "index.golden.json");

function sliceSource(startMarker, endMarker) {
  const start = source.indexOf(startMarker);
  if (start < 0) throw new Error(`Missing source marker: ${startMarker}`);
  const end = source.indexOf(endMarker, start);
  if (end < 0) throw new Error(`Missing source marker: ${endMarker}`);
  return source.slice(start, end);
}

const kebabSource = sliceSource("const kebab =", "\nconst extOf");
const buildIndexSource = sliceSource("function buildIndex(palettes)", "\n\n/* ---------- zip:");
const provenanceSource = sliceSource("function sourceIdentity(source)", "\nfunction reviewSources()");
const hashSource = sliceSource("function fnv1a64(u8)", "\n\n/* Human review");
const fingerprintSource = sliceSource("function stableValue(value)", "\n\nfunction reviewToken");
const colorMathSource = sliceSource("const clamp =", "\n/* ============================================================\n   2. names");
const assessmentSource = sliceSource("function reviewRGB(token)", "\nfunction sealMatches");
const historySource = sliceSource("function normalizeReviewHistory(history)", "\nfunction sealMatches");
const shelfRevisionSource = sliceSource("function shelfRevisionValue(plate)", "\nfunction savePlate()");
const plateImportSource = sliceSource("const HEXRE =", "\nfunction applyPlate");
const shelfValidationSource = sliceSource("// Shelf JSON is untrusted", "\nconst SHELF_KEY =");
const palettes = fs.readdirSync(vaultRoot, { withFileTypes: true })
  .filter(entry => entry.isDirectory())
  .map(entry => {
    const document = JSON.parse(fs.readFileSync(path.join(vaultRoot, entry.name, "palette.json"), "utf8"));
    document.folder = entry.name;
    return document;
  })
  .sort((a, b) => String(a.name || "").localeCompare(String(b.name || "")));

const context = {
  palettes,
  result: null,
  SCHEMA_VERSION: 1,
  TOOL_VERSION: "2.0.0",
  plateFacets() {
    throw new Error("fixture is missing frozen facets");
  },
  plateSignature() {
    throw new Error("fixture is missing a frozen signature");
  }
};
vm.createContext(context);
vm.runInContext(`${kebabSource};\n${buildIndexSource}\nresult = buildIndex(palettes);`, context);

const actual = JSON.parse(JSON.stringify(context.result));
const expected = JSON.parse(fs.readFileSync(goldenPath, "utf8"));
assert.deepStrictEqual(actual, expected);

const importedSource = {
  file: "imported.png", natural: [800, 450], bytes: 1234,
  hash: { algo: "sha256", value: "imported-hash" },
  path: "sources/05-imported.png"
};
const workspaceSource = {
  file: "workspace.png", natural: [640, 360], bytes: 5678,
  hash: { algo: "sha256", value: "workspace-hash" }
};
const provenance = {
  state: {
    plateSources: [importedSource],
    stills: [{ source: workspaceSource, bytes: new Uint8Array([1, 2, 3]) }]
  },
  importedSource,
  workspaceSource,
  current: null,
  merged: null,
  attachments: null,
  unsafe: null,
  preserved: null,
  clonePlain(value) {
    return value == null ? value : JSON.parse(JSON.stringify(value));
  },
  kebab(value) {
    return String(value || "plate").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "plate";
  },
  extOf() {
    return "png";
  }
};
vm.createContext(provenance);
vm.runInContext(`${provenanceSource}
current = currentPlateSources();
merged = mergePlateSources(current, [workspaceSource]);
attachments = sourceBytesFor({ sources: [importedSource, workspaceSource] });
unsafe = retainedSourceRecord({ ...workspaceSource, path: "../escape.png" }, 0);
preserved = retainedSourceRecord(importedSource, 0);`, provenance);

assert.deepStrictEqual(JSON.parse(JSON.stringify(provenance.current)), [importedSource]);
assert.equal(provenance.merged.length, 2);
assert.equal(provenance.attachments[0].bytes, null);
assert.deepStrictEqual(Array.from(provenance.attachments[1].bytes), [1, 2, 3]);
assert.equal(provenance.unsafe.path, "sources/01-workspace.png");
assert.equal(provenance.preserved.path, importedSource.path);

const roleIDs = ["ground", "panel", "ink", "muted", "observed", "attention", "rupture", "gold"];
const colors = [
  [0, 0, 0], [26, 26, 26], [255, 255, 255], [170, 170, 170],
  [255, 255, 255], [255, 255, 255], [255, 255, 255], [255, 255, 255]
];
const assessment = {
  ROLES: roleIDs.map(id => ({ id })),
  core: roleIDs.map((id, index) => {
    const [r, g, b] = colors[index];
    return {
      id, r, g, b,
      hex: "#" + [r, g, b].map(value => value.toString(16).padStart(2, "0")).join("").toUpperCase(),
      derived: false,
      origin: { kind:index < 2 ? "structure" : "signal" }
    };
  }),
  found: [],
  sources: [{ file:"source.png", hash:{ algo:"sha256", value:"hash" } }],
  clearResult: null,
  attentionResult: null,
  sanitizeName(value) {
    return String(value == null ? "" : value).replace(/[\r\n]+/g, " ").trim().slice(0, 40);
  }
};
vm.createContext(assessment);
vm.runInContext(`${colorMathSource}
${assessmentSource}
clearResult = reviewAssessment(core, found, sources);
core[2] = { ...core[2], r:0, g:0, b:0, hex:"#000000" };
core[4] = { ...core[4], derived:true, origin:{ kind:"derived" } };
core[5] = { ...core[5], origin:{ kind:"imported" } };
attentionResult = reviewAssessment(core, found, []);`, assessment);

assert.equal(assessment.clearResult.status, "clear");
assert.equal(assessment.clearResult.attentionCount, 0);
assert.equal(assessment.attentionResult.status, "attention");
assert.equal(assessment.attentionResult.attentionCount, 4);
assert.equal(assessment.attentionResult.checks.find(check => check.id === "readability").roles.includes("ink"), true);
assert.equal(assessment.attentionResult.checks.find(check => check.id === "origins").roles.includes("attention"), true);
assert.equal(assessment.attentionResult.checks.find(check => check.id === "invention").roles.includes("observed"), true);

const fingerprintDocument = palettes.find(plate => plate.folder === "night-bay");
assert.ok(fingerprintDocument, "night-bay fingerprint fixture is missing");
const fingerprintAudit = {
  document:fingerprintDocument,
  TextEncoder,
  REVIEW_SCOPE:"palette-and-source-identities",
  LEGACY_REVIEW_SCOPE:"palette-content",
  sourceBound:null, sourceChanged:null, legacyBefore:null, legacyAfter:null,
  sanitizeName(value) {
    return String(value == null ? "" : value)
      .replace(/\*\//g, "*").replace(/[\r\n]+/g, " ").trim().slice(0, 40);
  }
};
vm.createContext(fingerprintAudit);
vm.runInContext(`${hashSource}
${fingerprintSource}
const changedSources = document.sources.map((source, index) => index
  ? source : { ...source, hash:{ ...source.hash, value:"changed-source-hash" } });
sourceBound = contentFingerprint(document.name, document.lightScene, document.core, document.found,
  document.sources, REVIEW_SCOPE);
sourceChanged = contentFingerprint(document.name, document.lightScene, document.core, document.found,
  changedSources, REVIEW_SCOPE);
legacyBefore = contentFingerprint(document.name, document.lightScene, document.core, document.found,
  document.sources, LEGACY_REVIEW_SCOPE);
legacyAfter = contentFingerprint(document.name, document.lightScene, document.core, document.found,
  changedSources, LEGACY_REVIEW_SCOPE);`, fingerprintAudit);

assert.notEqual(fingerprintAudit.sourceBound.value, fingerprintAudit.sourceChanged.value);
assert.equal(fingerprintAudit.legacyBefore.value, fingerprintAudit.legacyAfter.value);
assert.equal(fingerprintAudit.sourceBound.value, "11fa430c543fc1c5");

const importDocument = {
  ...fingerprintDocument,
  createdAt:"2026-08-28T21:00:00.000Z",
  generator:{ name:"scene-palette-legacy", version:"1.4.2", extraction:2 },
  medianL:0.18,
  params:{ sampleBudget:140000, signalFloor:0.085, polarityMode:"auto", variant:1, variantCount:3 },
  tags:["night", "human-curated"],
  family:"harbor study",
  notes:["keep this human note"],
  core:fingerprintDocument.core.map((token, index) => ({ ...token, locked:index === 0 })),
  found:fingerprintDocument.found.map(token => ({
    ...token, id:"found_human", job:"human accent", locked:true, derived:true
  })),
  review:{ version:1, status:"proposed", agentEligible:false }
};
const plateImportAudit = {
  text:JSON.stringify(importDocument), parsed:null,
  state:{ seq:1 },
  ROLES:roleIDs.map(id => ({ id, job:id })),
  clonePlain(value) {
    return value == null ? value : JSON.parse(JSON.stringify(value));
  },
  safeComment(value) {
    return String(value == null ? "" : value).replace(/\*\//g, "*").replace(/[\r\n]+/g, " ").trim();
  },
  sanitizeName(value) {
    return String(value == null ? "" : value)
      .replace(/\*\//g, "*").replace(/[\r\n]+/g, " ").trim().slice(0, 40);
  },
  clamp(value, low, high) { return Math.max(low, Math.min(high, value)); },
  hex(color) {
    return "#" + [color.r, color.g, color.b]
      .map(value => Math.round(value).toString(16).padStart(2, "0")).join("").toUpperCase();
  },
  nameOf() { return "imported"; },
  relLum() { return 0; }
};
vm.createContext(plateImportAudit);
vm.runInContext(`${plateImportSource}\n${shelfValidationSource}\nparsed = parsePlateJSON(text);`, plateImportAudit);

assert.equal(plateImportAudit.parsed.createdAt, importDocument.createdAt);
assert.deepStrictEqual(JSON.parse(JSON.stringify(plateImportAudit.parsed.generator)), importDocument.generator);
assert.equal(plateImportAudit.parsed.medianL, importDocument.medianL);
assert.deepStrictEqual(JSON.parse(JSON.stringify(plateImportAudit.parsed.params)), importDocument.params);
assert.deepStrictEqual(JSON.parse(JSON.stringify(plateImportAudit.parsed.tags)), importDocument.tags);
assert.equal(plateImportAudit.parsed.family, importDocument.family);
assert.deepStrictEqual(JSON.parse(JSON.stringify(plateImportAudit.parsed.notes)), importDocument.notes);
assert.equal(plateImportAudit.parsed.sources.length, importDocument.sources.length);
assert.equal(plateImportAudit.parsed.review.status, importDocument.review.status);
assert.equal(plateImportAudit.parsed.core[0].locked, true);
assert.equal(plateImportAudit.parsed.found[0].id, "found_human");
assert.equal(plateImportAudit.parsed.found[0].job, "human accent");
assert.equal(plateImportAudit.parsed.found[0].locked, true);
assert.equal(plateImportAudit.parsed.found[0].derived, true);
fingerprintAudit.importedDocument = importDocument;
fingerprintAudit.parsedDocument = JSON.parse(JSON.stringify(plateImportAudit.parsed));
vm.runInContext(`
importedBefore = contentFingerprint(importedDocument.name, importedDocument.lightScene,
  importedDocument.core, importedDocument.found, importedDocument.sources, REVIEW_SCOPE);
importedAfter = contentFingerprint(parsedDocument.name, parsedDocument.lightScene,
  parsedDocument.core, parsedDocument.found, parsedDocument.sources, REVIEW_SCOPE);`, fingerprintAudit);
assert.equal(fingerprintAudit.importedAfter.value, fingerprintAudit.importedBefore.value);

// The tolerant single-palette importer shares the metadata type boundary.
plateImportAudit.text = JSON.stringify({ ...importDocument, core:importDocument.core.map((token, i) =>
  i === 2 ? { ...token, origin:{ kind:{ toString:null, valueOf:null } } } : token) });
assert.throws(() => vm.runInContext("parsePlateJSON(text)", plateImportAudit), /invalid kind/);

// Generated found inks have output keys but no semantic IDs. Import must not
// turn found_1 into a new identity and invalidate an otherwise matching seal.
const generatedDocument = JSON.parse(JSON.stringify(importDocument));
for (const token of generatedDocument.found) delete token.id;
plateImportAudit.text = JSON.stringify(generatedDocument);
vm.runInContext("parsed = parsePlateJSON(text);", plateImportAudit);
assert.equal(plateImportAudit.parsed.found[0].id, null);
fingerprintAudit.importedDocument = generatedDocument;
fingerprintAudit.parsedDocument = JSON.parse(JSON.stringify(plateImportAudit.parsed));
vm.runInContext(`
importedBefore = contentFingerprint(importedDocument.name, importedDocument.lightScene,
  importedDocument.core, importedDocument.found, importedDocument.sources, REVIEW_SCOPE);
importedAfter = contentFingerprint(parsedDocument.name, parsedDocument.lightScene,
  parsedDocument.core, parsedDocument.found, parsedDocument.sources, REVIEW_SCOPE);`, fingerprintAudit);
assert.equal(fingerprintAudit.importedAfter.value, fingerprintAudit.importedBefore.value);

const seal = (value, history = []) => ({
  version:1, status:"sealed", agentEligible:true,
  sealedAt:"2026-08-29T12:34:56.000Z", sealedBy:"human", scope:"palette-content",
  fingerprint:{ algo:"fnv1a64", value },
  decision:{ version:1, baseline:true, changeCount:0, fieldCount:0 },
  assessment:{ version:1, status:"clear", attentionCount:0 },
  history
});
const historyAudit = {
  state:{ review:seal("0000000000000001") },
  first:null, repeated:null, second:null, bounded:null,
  seal,
  REVIEW_SCOPE:"palette-and-source-identities",
  LEGACY_REVIEW_SCOPE:"palette-content",
  reviewScope(scope) {
    return scope === "palette-and-source-identities" || scope === "palette-content" ? scope : null;
  },
  sanitizeName(value) {
    return String(value == null ? "" : value)
      .replace(/\*\//g, "*").replace(/[\r\n]+/g, " ").trim().slice(0, 40);
  }
};
vm.createContext(historyAudit);
vm.runInContext(`${historySource}
first = archiveCurrentSeal("observed renamed by human");
state.review = { version:1, status:"edited", history:first };
repeated = archiveCurrentSeal("continued typing");
state.review = seal("0000000000000002", first);
second = archiveCurrentSeal("source still removed");
bounded = normalizeReviewHistory(Array.from({ length:25 }, (_, index) => ({
  version:1, sealedAt:"2026-08-29T12:34:56.000Z", sealedBy:"human", scope:"palette-content",
  fingerprint:{ algo:"fnv1a64", value:index.toString(16).padStart(16, "0") },
  invalidatedAt:"2026-08-30T09:00:00.000Z", invalidatedBy:"bounded history"
})));`, historyAudit);

assert.equal(historyAudit.first.length, 1);
assert.equal(historyAudit.first[0].invalidatedBy, "observed renamed by human");
assert.equal(historyAudit.repeated.length, 1);
assert.equal(historyAudit.second.length, 2);
assert.equal(historyAudit.second[1].invalidatedBy, "source still removed");
assert.equal(historyAudit.bounded.length, 20);
assert.equal(historyAudit.bounded[0].fingerprint.value, "0000000000000005");

const shelfRevisionAudit = {
  prior: {
    id:"p7_0", createdAt:"2026-08-29T12:00:00.000Z", name:"Night bay",
    tags:["approved"], family:"harbor", core:[{ id:"ground", hex:"#101010" }],
    found:[], review:{ version:1, status:"proposed", agentEligible:false },
    extension:{ owner:"human" }
  },
  next: {
    createdAt:"2026-08-30T12:00:00.000Z", name:"Night bay",
    tags:[], family:null, core:[{ id:"ground", hex:"#101010" }],
    found:[], review:{ version:1, status:"sealed", agentEligible:true }
  },
  state:{ seq:20 },
  merged:null, changed:null, createdAtOnly:null, importMerge:null, normalized:null,
  clonePlain(value) {
    return value == null ? value : JSON.parse(JSON.stringify(value));
  },
  stableValue:function stableValue(value) {
    if (Array.isArray(value)) return value.map(stableValue);
    if (value && typeof value === "object") {
      return Object.fromEntries(Object.keys(value).filter(key => value[key] !== undefined).sort()
        .map(key => [key, stableValue(value[key])]));
    }
    return value;
  }
};
vm.createContext(shelfRevisionAudit);
vm.runInContext(`${shelfRevisionSource}
merged = mergeShelfRevision(prior, next);
changed = shelfRevisionValue(prior) !== shelfRevisionValue(merged);
createdAtOnly = shelfRevisionValue(prior) === shelfRevisionValue({
  ...prior, createdAt:"2026-08-30T12:00:00.000Z"
});
importMerge = mergeShelfImport([prior], [
  { ...prior, id:"exported-copy", createdAt:"2026-08-30T12:00:00.000Z" },
  { ...next, id:prior.id },
  {
    id:"day", createdAt:"2026-08-30T13:00:00.000Z", name:"Day facade",
    tags:[], family:null, core:[{ id:"ground", hex:"#F0F0F0" }], found:[],
    review:{ version:1, status:"proposed", agentEligible:false }
  }
], 2);
normalized = normalizeShelfIds([
  { ...prior, id:"shared" },
  { ...next, id:"shared" },
  { ...next, id:null, name:"Third plate" }
]);`, shelfRevisionAudit);

assert.equal(shelfRevisionAudit.merged.id, shelfRevisionAudit.prior.id);
assert.equal(shelfRevisionAudit.merged.createdAt, shelfRevisionAudit.prior.createdAt);
assert.equal(shelfRevisionAudit.merged.review.status, "sealed");
assert.deepStrictEqual(JSON.parse(JSON.stringify(shelfRevisionAudit.merged.tags)), ["approved"]);
assert.equal(shelfRevisionAudit.merged.family, "harbor");
assert.equal(shelfRevisionAudit.merged.extension.owner, "human");
assert.equal(shelfRevisionAudit.changed, true);
assert.equal(shelfRevisionAudit.createdAtOnly, true);
assert.equal(shelfRevisionAudit.importMerge.imported.length, 1);
assert.equal(shelfRevisionAudit.importMerge.duplicates, 1);
assert.equal(shelfRevisionAudit.importMerge.skippedCapacity, 1);
assert.equal(shelfRevisionAudit.importMerge.shelf.length, 2);
assert.equal(shelfRevisionAudit.importMerge.shelf[0].review.status, "sealed");
assert.notEqual(shelfRevisionAudit.importMerge.shelf[0].id, shelfRevisionAudit.prior.id);
assert.equal(shelfRevisionAudit.importMerge.shelf[1].id, shelfRevisionAudit.prior.id);
assert.equal(new Set(shelfRevisionAudit.normalized.map(plate => plate.id)).size, 3);
assert.equal(shelfRevisionAudit.normalized[0].id, "shared");

// Both shelf entry points must validate the complete durable record, while
// retaining legitimate extension fields and the exact sealed semantic content.
const shelfFixture = JSON.parse(fs.readFileSync(path.join(root,
  "testdata/reference-palettes/empty-neon-street/palette.json"), "utf8"));
shelfFixture.id = "stable-shelf-id";
shelfFixture.extension = { owner:"human", nested:[1, "keep me"] };
const shelfValidation = { ...plateImportAudit, fixture:shelfFixture, parsed:null, result:null };
vm.createContext(shelfValidation);
vm.runInContext(`${plateImportSource}\n${shelfValidationSource}\nresult = normalizeShelfEntry(fixture);`, shelfValidation);
assert.deepEqual(JSON.parse(JSON.stringify(shelfValidation.result)), shelfFixture,
  "A valid shelf record changed during normalization");
const hostile = '<img src="data:,invalid" onerror="window.__shelfSecuritySentinel=1">';
const malformed = [null, [], { core:Array(8).fill(null) }];
for (const mutate of [
  p => { p.core[0].hex = '#000000">' + hostile; },
  p => { p.core[0].id = hostile; },
  p => { p.core[0].key = "ink"; },
  p => { p.core[1] = p.core[0]; },
  p => { p.core[0].rgb[0] = (p.core[0].rgb[0] + 1) % 256; },
  p => { p.core[0].r = 1.25; },
  p => { p.core[0].r = "12"; },
  p => { p.found = [null]; },
  p => { p.found = [{ hex:hostile }]; },
  p => { p.core[2].origin.kind = { toString:null, valueOf:null }; },
  p => { p.core[2].origin.repairs = [{ from:{ toString:null, valueOf:null } }]; },
  p => { p.core[2].origin.natural = { x:{ toString:null, valueOf:null } }; },
  p => { p.core[2].origin.quantile = [{ toString:null, valueOf:null }, 1]; },
  p => { p.sources[0].natural = [{ toString:null, valueOf:null }, 80]; },
  p => { p.sources[0].path = { toString:null, valueOf:null }; },
  p => { p.generator.extraction = { toString:null, valueOf:null }; },
  p => { p.params.variant = { toString:null, valueOf:null }; },
  p => { p.review = { proposal:{ name:{ toString:null, valueOf:null } } }; },
  p => { p.review = { decision:{ changeCount:{ toString:null, valueOf:null } } }; },
  p => { p.review = { history:[{ invalidatedBy:{ toString:null, valueOf:null } }] }; },
  p => { p.stills = hostile; },
  p => { p.stills = -1; },
  p => { p.name = {}; },
  p => { p.sources = [null]; },
  p => { p.sources[0].hash = { algo:[], value:"x" }; },
  p => { p.review = []; },
  p => { p.params = []; },
  p => { p.notes = [{}]; },
  p => { p.lightScene = !p.lightScene; },
  p => { p.extension = "x".repeat(65537); },
  p => { p.extension = Array(100001).fill(0); },
  p => { p.extension = Array.from({ length:34 }).reduce(value => ({ child:value }), {}); }
]) {
  const value = structuredClone(shelfFixture); mutate(value); malformed.push(value);
}
shelfValidation.text = JSON.stringify({ plates:[shelfFixture, ...malformed] });
vm.runInContext("parsed = parseShelfJSON(text);", shelfValidation);
assert.equal(shelfValidation.parsed.skipped, malformed.length);
assert.deepEqual(JSON.parse(JSON.stringify(shelfValidation.parsed.plates)), [shelfFixture]);
for (const text of ['{}', '{"plates":{}}', JSON.stringify(Array(401).fill(null)), ' '.repeat(8 * 1024 * 1024 + 1)]) {
  shelfValidation.text = text;
  assert.throws(() => vm.runInContext("parseShelfJSON(text)", shelfValidation));
}
// Display text remains text, and harmless extension/provenance metadata survives.
shelfValidation.fixture = structuredClone(shelfFixture);
shelfValidation.fixture.name = hostile;
shelfValidation.fixture.core[2].name = hostile;
shelfValidation.fixture.core[2].origin.extra = { quoted:hostile };
vm.runInContext("result = normalizeShelfEntry(fixture);", shelfValidation);
assert.equal(shelfValidation.result.name, hostile);
assert.equal(shelfValidation.result.core[2].origin.extra.quoted, hostile);
// Legacy positional and key-only colours normalize without inventing paint.
shelfValidation.fixture = { core:shelfFixture.core.map(token => ({ hex:token.hex })), found:[] };
vm.runInContext("result = normalizeShelfEntry(fixture);", shelfValidation);
assert.deepEqual(Array.from(shelfValidation.result.core, token => token.hex), shelfFixture.core.map(token => token.hex));
shelfValidation.fixture = { ...shelfFixture, core:shelfFixture.core.map(({ id, ...token }) => token) };
vm.runInContext("result = normalizeShelfEntry(fixture);", shelfValidation);
assert.deepEqual(Array.from(shelfValidation.result.core, token => token.id), roleIDs);
// Fingerprints are recomputed from content, never trusted from imported eligibility flags.
shelfValidation.fixture = importDocument;
vm.runInContext("result = normalizeShelfEntry(fixture);", shelfValidation);
fingerprintAudit.importedDocument = importDocument;
fingerprintAudit.parsedDocument = JSON.parse(JSON.stringify(shelfValidation.result));
vm.runInContext(`
importedBefore = contentFingerprint(importedDocument.name, importedDocument.lightScene,
  importedDocument.core, importedDocument.found, importedDocument.sources, REVIEW_SCOPE);
importedAfter = contentFingerprint(parsedDocument.name, parsedDocument.lightScene,
  parsedDocument.core, parsedDocument.found, parsedDocument.sources, REVIEW_SCOPE);`, fingerprintAudit);
assert.equal(fingerprintAudit.importedAfter.value, fingerprintAudit.importedBefore.value);
fingerprintAudit.parsedDocument.core[0].name = "tampered";
vm.runInContext(`importedAfter = contentFingerprint(parsedDocument.name, parsedDocument.lightScene,
  parsedDocument.core, parsedDocument.found, parsedDocument.sources, REVIEW_SCOPE);`, fingerprintAudit);
assert.notEqual(fingerprintAudit.importedAfter.value, fingerprintAudit.importedBefore.value);


console.log(`JavaScript contracts: OK (${actual.count} indexed plates; source-bound ${fingerprintAudit.sourceBound.value}; provenance, metadata round-trips, review readiness, seal history, and safe shelf merges verified)`);

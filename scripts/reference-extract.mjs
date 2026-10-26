// Run the current browser extraction algorithm against committed PNG fixtures.
// This script intentionally extracts functions from index.html instead of
// maintaining a second JavaScript implementation.
import assert from "node:assert/strict";
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const require = createRequire(import.meta.url);
let sharp;
try {
  sharp = require(process.env.SCENE_PALETTE_SHARP || "sharp");
} catch {
  throw new Error("reference extraction needs the optional sharp package (install it or expose it through NODE_PATH)");
}

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const imageRoot = path.join(root, "testdata", "reference-images");
const paletteRoot = path.join(root, "testdata", "reference-palettes");
const stageOutput = path.join(root, "testdata", "histogram-signal.golden.json");
const manifest = JSON.parse(fs.readFileSync(path.join(imageRoot, "manifest.json"), "utf8"));
const source = fs.readFileSync(path.join(root, "index.html"), "utf8").replaceAll("\r\n", "\n");
const write = process.argv.includes("--write");
const check = process.argv.includes("--check") || !write;
const fixedTime = "2026-08-28T00:00:00.000Z";
const strideProbeBudget = 20000;

function sourceSlice(startMarker, endMarker) {
  const start = source.indexOf(startMarker);
  if (start < 0) throw new Error(`Missing index.html marker: ${startMarker}`);
  const end = source.indexOf(endMarker, start);
  if (end < 0) throw new Error(`Missing index.html marker: ${endMarker}`);
  return source.slice(start, end);
}

const buildPlateSource = sourceSlice("function buildPlate(reset)", "/** Auto / Dark / Light");
const instrumentedBuildPlateSource = buildPlateSource.replace(
  "    state.struct    = struct;",
  "    state.roleCands = roleCands;\n    state.struct    = struct;"
);
if (instrumentedBuildPlateSource === buildPlateSource) {
  throw new Error("Could not instrument buildPlate role candidates");
}

const oracleSource = [
  sourceSlice("const clamp =", "/* ============================================================\n   3. state"),
  sourceSlice("const QUANTILES =", "const $ ="),
  sourceSlice("const SCHEMA_VERSION", "const bytesToHex"),
  sourceSlice("const clonePlain =", "function captureWorkspace()"),
  sourceSlice("function fnv1a64(u8)", "async function hashBytes(buf)"),
  sourceSlice("function plateMetadata(value)", "function coerceColour(o)"),
  sourceSlice("function histogram(still", "/** Scan pixels once"),
  sourceSlice("async function extract()", "/** Build structure"),
  instrumentedBuildPlateSource,
  sourceSlice("function applyVariant(i)", "\nasync function remix()"),
  sourceSlice("const safeComment =", "/* ---------- preview surfaces"),
  sourceSlice("const kebab =", "/** Facts a machine derives"),
  sourceSlice("function plateFacets(plate)", "function notesMD(plate)")
].join("\n\n");

class FixedDate extends Date {
  constructor(...args) {
    super(args.length ? args[0] : fixedTime);
  }

  static now() {
    return new Date(fixedTime).getTime();
  }
}

function title(label) {
  return label.split("-").map(word => word[0].toUpperCase() + word.slice(1)).join(" ");
}

function oracle() {
  const errors = [];
  const controls = {
    quality: { value: "140000" },
    chroma: { value: "85" },
    extra: { value: "6" },
    extractBtn: { disabled: false, textContent: "" },
    plateName: { value: "Reference" }
  };
  const context = {
    console,
    Date: FixedDate,
    Uint8ClampedArray,
    TextEncoder,
    setTimeout,
    clearTimeout,
    $: id => {
      if (!controls[id]) throw new Error(`unexpected UI lookup in reference extractor: ${id}`);
      return controls[id];
    },
    render() {},
    toast(message, bad) {
      if (bad) errors.push(String(message));
    }
  };
  vm.createContext(context);
  vm.runInContext(
    `${oracleSource}\nglobalThis.__oracle = { state, ROLES, extract, buildPlate, applyVariant, snapshot, histogram, signalCandidates, rankedPool, assignRoles };`,
    context
  );
  return { ...context.__oracle, controls, errors };
}

function populatedBins(bins, signal = false) {
  return bins.flatMap((bin, index) => bin.n ? [{
    index,
    n: bin.n,
    L: bin.L,
    a: bin.a,
    b: bin.b,
    ...(signal ? { wsum: bin.wsum } : {})
  }] : []);
}

function accentColorSnapshot(candidate) {
  return {
    rgb: [candidate.r, candidate.g, candidate.b],
    L: candidate.L,
    C: candidate.C,
    H: candidate.H,
    area: candidate.area,
    salience: candidate.salience,
    derived: !!candidate.derived,
    src: candidate.src || null,
    repairs: candidate.repairs || [],
    derivedFrom: candidate.derivedFrom || null
  };
}

function accentStageSnapshot(engine) {
  const roles = engine.ROLES.filter(role => role.kind === "chroma");
  return {
    filteredBins: engine.state.cands.map(candidate => candidate.bin),
    roleCandidates: (engine.state.roleCands || []).map(accentColorSnapshot),
    sets: engine.state.sets.map(set => ({
      colors: set.map(accentColorSnapshot),
      assignedIndices: engine.assignRoles(set, roles).map(candidate => set.indexOf(candidate))
    }))
  };
}

function assembledTokenSnapshot(token, found = false) {
  return {
    ...(!found ? { id: token.id } : {}),
    name: token.name,
    job: token.job,
    rgb: [token.r, token.g, token.b],
    hex: token.hex,
    locked: !!token.locked,
    derived: !!token.derived,
    origin: token.origin || null
  };
}

function plateAssemblySnapshot(engine) {
  const current = engine.state.variant;
  const variants = engine.state.sets.map((unused, index) => {
    engine.applyVariant(index);
    return {
      index,
      core: engine.state.core.map(token => assembledTokenSnapshot(token)),
      found: engine.state.found.map(token => assembledTokenSnapshot(token, true)),
      notes: engine.state.notes.slice()
    };
  });
  engine.applyVariant(current);
  return { foundLimit: +engine.controls.extra.value, variants };
}

function structureSnapshot(engine) {
  const roleIds = ["ground", "panel", "ink", "muted"];
  return {
    mode: engine.state.polarity,
    medianL: engine.state.medianL,
    autoLight: engine.state.autoLight,
    lightScene: engine.state.lightScene,
    accents: accentStageSnapshot(engine),
    assembly: plateAssemblySnapshot(engine),
    roles: roleIds.map(id => {
      const color = engine.state.struct[id];
      return {
        id,
        rgb: [color.r, color.g, color.b],
        origin: engine.state.structOrigin[id]
      };
    })
  };
}

async function extractFixture(entry) {
  const fixturePath = path.join(imageRoot, entry.fixture.file);
  const bytes = fs.readFileSync(fixturePath);
  const decoded = await sharp(bytes).ensureAlpha().raw().toBuffer({ resolveWithObject: true });
  if (decoded.info.channels !== 4) throw new Error(`${entry.fixture.file}: expected RGBA pixels`);

  const engine = oracle();
  engine.controls.plateName.value = title(entry.label);
  const still = {
    uid: 1,
    name: entry.fixture.file,
    w: decoded.info.width,
    h: decoded.info.height,
    _img: new Uint8ClampedArray(decoded.data.buffer, decoded.data.byteOffset, decoded.data.byteLength),
    bytes: bytes.buffer.slice(bytes.byteOffset, bytes.byteOffset + bytes.byteLength),
    source: {
      file: entry.fixture.file,
      mime: "image/png",
      bytes: bytes.length,
      hash: { algo: "sha256", value: crypto.createHash("sha256").update(bytes).digest("hex") },
      natural: [decoded.info.width, decoded.info.height],
      working: [decoded.info.width, decoded.info.height],
      transforms: [{ op: "decode", via: "sharp-raw-reference-harness" }]
    }
  };
  engine.state.stills = [still];
  const budget = +engine.controls.quality.value;
  const chromaMin = +engine.controls.chroma.value / 1000;
  const rawHistogram = engine.histogram(still, budget, chromaMin);
  const strideProbe = entry.label === "empty-neon-street"
    ? engine.histogram(still, strideProbeBudget, chromaMin)
    : null;
  await engine.extract();
  if (engine.errors.length) throw new Error(`${entry.fixture.file}: ${engine.errors.join("; ")}`);
  const plate = engine.snapshot(title(entry.label));
  // Human review was added after these extraction fixtures. Check that a fresh
  // extraction is a proposal, then compare every extraction-owned field with
  // the unchanged baseline. Review fingerprints/round-trips have their own gate.
  assert.equal(plate.review.status, "proposed");
  assert.equal(plate.review.agentEligible, false);
  delete plate.review;
  plate.createdAt = fixedTime;
  const pooled = engine.state.hist;
  const candidates = engine.signalCandidates(
    pooled.sig, pooled.HB, pooled.BANDS, pooled.totalN, pooled.binStills
  );
  const structures = { auto: structureSnapshot(engine) };
  for (const mode of ["dark", "light"]) {
    engine.state.polarity = mode;
    engine.buildPlate(false);
    structures[mode] = structureSnapshot(engine);
  }
  if (engine.errors.length) throw new Error(`${entry.fixture.file}: ${engine.errors.join("; ")}`);
  const stage = {
    label: entry.label,
    file: entry.fixture.file,
    width: decoded.info.width,
    height: decoded.info.height,
    step: Math.max(1, Math.floor(Math.sqrt((decoded.info.width * decoded.info.height) / budget))),
    total: rawHistogram.total,
    weightedTotal: rawHistogram.lum.reduce((sum, bin) => sum + bin.n, 0),
    luminanceBins: populatedBins(rawHistogram.lum),
    signalBins: populatedBins(rawHistogram.sig, true),
    structures,
    ...(strideProbe ? { strideProbe: {
      budget: strideProbeBudget,
      step: Math.max(1, Math.floor(Math.sqrt((decoded.info.width * decoded.info.height) / strideProbeBudget))),
      total: strideProbe.total,
      weightedTotal: strideProbe.lum.reduce((sum, bin) => sum + bin.n, 0),
      luminanceBins: populatedBins(strideProbe.lum),
      signalBins: populatedBins(strideProbe.sig, true)
    } } : {}),
    candidates: candidates.map(candidate => ({
      rgb: [candidate.r, candidate.g, candidate.b],
      area: candidate.area,
      C: candidate.C,
      H: candidate.H,
      salience: candidate.salience,
      bin: candidate.bin,
      band: candidate.band,
      hueBin: candidate.hueBin,
      stills: candidate.stills
    })),
    rankedBins: engine.rankedPool(candidates, 8).map(candidate => candidate.bin)
  };
  return JSON.parse(JSON.stringify({ plate, stage }));
}

const selected = manifest.images.filter(entry => entry.fixture);
const summaries = [];
const stages = [];
const indexPalettes = [];
for (const entry of selected) {
  const { plate, stage } = await extractFixture(entry);
  const output = path.join(paletteRoot, entry.label, "palette.json");
  if (write) {
    fs.mkdirSync(path.dirname(output), { recursive: true });
    fs.writeFileSync(output, JSON.stringify(plate, null, 2) + "\n");
  }
  if (check) {
    const expected = JSON.parse(fs.readFileSync(output, "utf8"));
    assert.deepStrictEqual(plate, expected, `${entry.label} drifted from the JavaScript extraction baseline`);
  }
  summaries.push({
    name: plate.name,
    polarity: plate.polarity,
    variants: plate.params.variantCount,
    derived: plate.facets.derivedSlots,
    found: plate.facets.foundInks,
    ground: plate.core[0].hex,
    accents: plate.core.slice(4).map(token => token.hex)
  });
  stages.push(stage);
  indexPalettes.push({ ...plate, folder: entry.label });
}

const stageFixture = {
  schema: 1,
  source: { file: "index.html", extraction: indexPalettes[0].generator.extraction },
  params: { budget: 140000, chromaMin: 0.085, luminanceBins: 64, hueBins: 36, bands: 3 },
  images: stages
};
if (write) fs.writeFileSync(stageOutput, JSON.stringify(stageFixture, null, 2) + "\n");
if (check) {
  assert.deepStrictEqual(
    stageFixture,
    JSON.parse(fs.readFileSync(stageOutput, "utf8")),
    "histogram or signal-candidate ranking drifted from the JavaScript baseline"
  );
}

// Freeze the index from the actual browser implementation, alongside the plates.
const indexContext = { palettes: indexPalettes.sort((a, b) => a.name.localeCompare(b.name)) };
vm.createContext(indexContext);
vm.runInContext([
  sourceSlice("const SCHEMA_VERSION", "const bytesToHex"),
  sourceSlice("const kebab =", "\nconst extOf"),
  sourceSlice("function buildIndex(palettes)", "\n\n/* ---------- zip:"),
  "globalThis.result = buildIndex(palettes);"
].join("\n"), indexContext);
const index = JSON.parse(JSON.stringify(indexContext.result));
const indexPath = path.join(paletteRoot, "index.golden.json");
if (write) fs.writeFileSync(indexPath, JSON.stringify(index, null, 2) + "\n");
if (check) assert.deepStrictEqual(index, JSON.parse(fs.readFileSync(indexPath, "utf8")), "Reference vault index drifted from browser baseline");
console.log(JSON.stringify(summaries, null, 2));
console.log(`JavaScript extraction baseline: ${check ? "OK" : "written"} (${selected.length} images)`);
console.log(`JavaScript histogram/signal baseline: ${check ? "OK" : "written"} (${stages.length} images)`);

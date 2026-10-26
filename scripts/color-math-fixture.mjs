// Freeze the pure colour-math behavior currently implemented in index.html.
import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import vm from "node:vm";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const source = fs.readFileSync(path.join(root, "index.html"), "utf8").replaceAll("\r\n", "\n");
const output = path.join(root, "testdata", "color-math.golden.json");
const write = process.argv.includes("--write");
const check = process.argv.includes("--check") || !write;

function sourceSlice(startMarker, endMarker) {
  const start = source.indexOf(startMarker);
  if (start < 0) throw new Error(`Missing index.html marker: ${startMarker}`);
  const end = source.indexOf(endMarker, start);
  if (end < 0) throw new Error(`Missing index.html marker: ${endMarker}`);
  return source.slice(start, end);
}

const context = {};
vm.createContext(context);
vm.runInContext(
  sourceSlice("const clamp =", "/* ============================================================\n   2. names") +
  "\nglobalThis.__math = { rgbToOklab, rgbToLch, lchToRgb, relLum, contrast, forceContrast, forceContrastPair, capChroma, deltaE, hueGap, hex, referenceAtan2, referenceCbrt };",
  context
);
const math = context.__math;

// These full-precision hues decide a tied third-variant role assignment in
// empty-neon-street. Native Edge atan2 changed them by one ULP; the historical
// JavaScript fixtures and Go agree on these values. Rounded snapshots alone
// cannot detect the numerical cause of the resulting attention/rupture swap.
assert.equal(math.rgbToLch({ r:45, g:108, b:178 }).H, 253.41180955309724);
assert.equal(math.rgbToLch({ r:158, g:79, b:101 }).H, 3.728313764853919);
for (const [y, x, expected] of [
  [0, 1, 0], [-0, 1, -0], [0, -1, Math.PI], [-0, -1, -Math.PI],
  [1, 0, Math.PI / 2], [-1, 0, -Math.PI / 2],
  [Infinity, Infinity, Math.PI / 4], [-Infinity, -Infinity, -3 * Math.PI / 4],
  [1, Infinity, 0], [-1, Infinity, -0], [Infinity, 1, Math.PI / 2]
]) assert.ok(Object.is(math.referenceAtan2(y, x), expected), `referenceAtan2(${y}, ${x})`);
assert.ok(Number.isNaN(math.referenceAtan2(NaN, 1)));

// Edge 156 changed native cbrt at these inputs, rotating near-neutral hue
// and swapping sparse Source hues roles. The reference keeps exact historical
// floats; these assertions intentionally have no numeric tolerance.
assert.equal(math.rgbToLch({ r:6, g:6, b:6 }).H, 89.87556269699212);
assert.equal(math.rgbToLch({ r:22, g:22, b:22 }).H, 89.87556266224881);
assert.equal(math.rgbToLch({ r:23, g:23, b:23 }).H, 89.87556292841498);
for (const [input, expected] of [
  [0, 0], [-0, -0], [Infinity, Infinity], [-Infinity, -Infinity],
  [Number.MIN_VALUE, 1.7031839360032603e-108], [1e-300, 1e-100],
  [-1e-300, -1e-100], [1e-6, .01], [8, 2], [-8, -2], [1e300, 1e100]
]) assert.ok(Object.is(math.referenceCbrt(input), expected), `referenceCbrt(${input})`);
assert.ok(Number.isNaN(math.referenceCbrt(NaN)));

const gray = value => ({ r:value, g:value, b:value });
for (const [paint, ground, panel, target] of [
  [{ r:120, g:25, b:180 }, gray(0), gray(255), 4.5],
  [{ r:120, g:25, b:180 }, gray(0), gray(255), 3],
  [{ r:255, g:0, b:0 }, gray(0), gray(251), 4.5],
  [gray(80), gray(12), gray(32), 4.5],
  [gray(220), gray(245), gray(222), 3]
]) {
  const result = math.forceContrastPair(paint, ground, panel, target);
  assert.equal(result.readable, true, "Joint contrast unexpectedly infeasible");
  assert.ok(math.contrast(result.color, ground) >= target && math.contrast(result.color, panel) >= target);
  const legacy = math.forceContrast(math.forceContrast(paint, ground, target), panel, target);
  if (math.contrast(legacy, ground) >= target && math.contrast(legacy, panel) >= target)
    assert.equal(math.hex(result.color), math.hex(legacy), "A passing existing repair changed");
}
assert.equal(math.forceContrastPair(gray(120), gray(89), gray(170), 4.5).readable, false,
  "An infeasible surface pair must be reported, not given a false contrast guarantee");

function rgb(value) {
  const match = /^#([0-9a-f]{6})$/i.exec(value);
  if (!match) throw new Error(`Invalid fixture hex: ${value}`);
  const number = Number.parseInt(match[1], 16);
  return { r: number >> 16, g: (number >> 8) & 255, b: number & 255 };
}

const colorInputs = [
  "#000000", "#FFFFFF", "#808080", "#FF0000", "#00FF00", "#0000FF",
  "#131220", "#E3D3C9", "#EC6421", "#00747D", "#F1DFD1", "#341606"
];
const contrastInputs = [
  ["#000000", "#FFFFFF"],
  ["#131220", "#BAB7D7"],
  ["#E3D3C9", "#24202B"],
  ["#EC6421", "#00747D"]
];
const repairInputs = [
  ["#2E3159", "#131220", 3],
  ["#934B98", "#1F1A30", 3],
  ["#5F5966", "#E3D3C9", 4.5],
  ["#F48725", "#341606", 3]
];
const lchInputs = [
  [0.62, 0.14, 27],
  [0.8, 0.2, 205],
  [0.18, 0.3, 330],
  [1.2, 0.1, 65],
  [-0.2, 0.1, 100]
];

const fixture = {
  schema: 1,
  source: { file: "index.html", extraction: 3 },
  colors: colorInputs.map(input => {
    const value = rgb(input);
    const lab = math.rgbToOklab(value.r, value.g, value.b);
    const lch = math.rgbToLch(value);
    return {
      input,
      lab: [lab.L, lab.a, lab.b],
      lch: [lch.L, lch.C, lch.H],
      relativeLuminance: math.relLum(value),
      roundTrip: math.hex(math.lchToRgb(lch.L, lch.C, lch.H)),
      cap055: math.hex(math.capChroma(value, 0.055))
    };
  }),
  contrasts: contrastInputs.map(([a, b]) => ({
    a,
    b,
    ratio: math.contrast(rgb(a), rgb(b)),
    deltaE: math.deltaE(rgb(a), rgb(b)),
    hueGap: math.hueGap(math.rgbToLch(rgb(a)).H, math.rgbToLch(rgb(b)).H)
  })),
  repairs: repairInputs.map(([color, background, target]) => {
    const result = math.forceContrast(rgb(color), rgb(background), target);
    return { color, background, target, result: math.hex(result), ratio: math.contrast(result, rgb(background)) };
  }),
  lchToRgb: lchInputs.map(input => ({ input, result: math.hex(math.lchToRgb(...input)) }))
};

const durable = JSON.parse(JSON.stringify(fixture));
if (write) fs.writeFileSync(output, JSON.stringify(durable, null, 2) + "\n");
if (check) assert.deepStrictEqual(durable, JSON.parse(fs.readFileSync(output, "utf8")));
console.log(`JavaScript colour math fixture: ${check ? "OK" : "written"}`);

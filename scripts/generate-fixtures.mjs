// Deterministic, self-authored synthetic fixtures. SPDX-License-Identifier: MIT
// No source photographs, artwork, downloaded pixels, or external packages are used.
// --write regenerates PNGs/manifest; --check (default) verifies their exact bytes.
import assert from "node:assert/strict";
import crypto from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const directory = path.join(root, "testdata", "reference-images");
const args = process.argv.slice(2);
assert.ok(args.every(arg => ["--write", "--check"].includes(arg)), "Use --write or --check");
assert.ok(!(args.includes("--write") && args.includes("--check")), "Choose one operation");
const write = args.includes("--write");
const width = 384, height = 256;
const clamp = value => Math.max(0, Math.min(255, value));
const rgb = (base, offset = 0, alpha = 255) => [...base.map(value => clamp(value + offset)), alpha];
const inside = (x, y, left, top, right, bottom) => x >= left && x < right && y >= top && y < bottom;
// Fixed coordinate hash, not process-global randomness. All arithmetic is integer.
const noise = (x, y, seed, range = 5) =>
  (((Math.imul(x + 1, 374761393) ^ Math.imul(y + 1, 668265263) ^ seed) >>> 0) % range);
const vivid = [[240, 55, 92], [242, 165, 32], [180, 207, 49],
  [23, 188, 172], [43, 133, 232], [147, 70, 214], [229, 73, 191]];
const patterns = [
  {
    label: "empty-neon-street", seed: 101, tags: ["dark", "many-hues", "small-signals", "stride"],
    description: "Dark tiled field with seven vivid vertical bars and a neutral stepped lower band.",
    pixel(x, y) {
      if (y > 42 && y < 174) {
        for (let n = 0; n < vivid.length; n++) {
          if (inside(x, y, 13 + n * 52, 44 + n % 3 * 9, 44 + n * 52, 172 - n % 2 * 15))
            return rgb(vivid[n], noise(x, y, 101) - 2);
        }
      }
      if (y > 212) return rgb([44, 46, 55], Math.floor(x / 48) * 4);
      return rgb([13, 18, 27], Math.floor(y / 32) + noise(x, y, 101));
    }
  },
  {
    label: "red-teal-portrait", seed: 202, tags: ["dark", "two-hues", "sparse-signals"],
    description: "Red and teal nested geometric panels on a dark field; no depicted person.",
    pixel(x, y) {
      if ((x - 126) ** 2 + (y - 125) ** 2 < 70 ** 2) return rgb([195, 49, 64], Math.floor(y / 26));
      if (inside(x, y, 230, 38, 344, 198)) return rgb([25, 152, 154], Math.floor(x / 28));
      if (inside(x, y, 41, 210, 342, 235)) return rgb([176, 161, 139], noise(x, y, 202));
      return rgb([23, 26, 35], noise(x, y, 202));
    }
  },
  {
    label: "pastel-prisms", seed: 303, tags: ["light", "pastel", "many-hues"],
    description: "High-key pastel diagonal tiles with narrow neutral separators and six hue families.",
    pixel(x, y) {
      const paints = [[245, 184, 188], [246, 213, 158], [206, 232, 175],
        [173, 231, 219], [178, 207, 244], [217, 182, 238]];
      if (x < 18 || x > 365 || y < 18 || y > 237 || (x + y) % 67 < 5)
        return rgb([245, 241, 232], noise(x, y, 303, 3));
      return rgb(paints[Math.floor((x + y) / 65) % paints.length], noise(x, y, 303, 7) - 3);
    }
  },
  {
    label: "winter-cabin", seed: 404, tags: ["dark", "warm-cool", "many-hues"],
    description: "Cool stepped stripes with warm square accents and contrasting small hue patches.",
    pixel(x, y) {
      if (inside(x, y, 130, 85, 258, 175)) return rgb([176, 104, 52], noise(x, y, 404, 11));
      if (y > 193) return rgb([165, 184, 199], Math.floor(x / 64));
      if (y < 61 && x > 17 && x < 368) return rgb(vivid[Math.floor(x / 53) % vivid.length], -25);
      return rgb([27, 43, 69], Math.floor(y / 32) + noise(x, y, 404));
    }
  },
  {
    label: "desert-sunset", seed: 505, tags: ["light", "warm", "gradient"],
    description: "Piecewise warm gradient with ochre, brick, purple, and blue geometric bands.",
    pixel(x, y) {
      if (y < 130) return rgb([246, 180, 113], Math.floor(y / 12));
      const paints = [[212, 126, 55], [178, 79, 56], [126, 69, 115], [56, 99, 153]];
      return rgb(paints[Math.floor((x + y * 2) / 90) % paints.length], noise(x, y, 505, 9) - 4);
    }
  },
  {
    label: "monochrome-architecture", seed: 606, tags: ["dark", "grayscale", "derived", "hard-edge"],
    description: "Grayscale stair-step blocks covering dark, midtone, and highlight quantiles.",
    pixel(x, y) {
      let value = 18 + Math.floor(x / 48) * 4 + Math.floor(y / 48) * 3;
      if (inside(x, y, 49, 31, 123, 215)) value = 90;
      if (inside(x, y, 145, 73, 220, 234)) value = 145;
      if (inside(x, y, 260, 21, 331, 184)) value = 220;
      value += noise(x, y, 606, 3);
      return [value, value, value, 255];
    }
  },
  {
    label: "flower-field", seed: 707, tags: ["light", "dense-signals", "small-details"],
    description: "Dense multicolour circular dots over green and pale horizontal bands.",
    pixel(x, y) {
      const cx = x % 42 - 21, cy = y % 37 - 18;
      if (cx * cx + cy * cy < 11 * 11)
        return rgb(vivid[(Math.floor(x / 42) + Math.floor(y / 37) * 3) % vivid.length], noise(x, y, 707, 9) - 4);
      return rgb(y < 150 ? [204, 225, 159] : [105, 153, 77], noise(x, y, 707, 13) - 6);
    }
  },
  {
    label: "fog-field", seed: 808, tags: ["light", "low-chroma", "derived", "gradient"],
    description: "Low-chroma pale gradient with two broad muted bands and sparse gray marks.",
    pixel(x, y) {
      const value = 192 + Math.floor(x / 24) + Math.floor(y / 32);
      if (y > 200) return rgb([160, 171, 165], noise(x, y, 808));
      if (inside(x, y, 173, 91, 180, 183)) return rgb([135, 143, 139]);
      return rgb([value, value + 4, value + 1], noise(x, y, 808, 3));
    }
  },
  {
    label: "bauhaus-primary", seed: 909, tags: ["light", "primary-colours", "hard-edge", "many-hues"],
    description: "Original grid of saturated rectangles and circles on an ivory field.",
    pixel(x, y) {
      if (inside(x, y, 23, 22, 118, 117)) return rgb([221, 44, 39]);
      if ((x - 188) ** 2 + (y - 79) ** 2 < 54 ** 2) return rgb([30, 80, 195]);
      if (inside(x, y, 270, 28, 360, 128)) return rgb([246, 189, 25]);
      if (inside(x, y, 24, 150, 104, 230)) return rgb([20, 153, 126]);
      if (inside(x, y, 136, 153, 225, 228)) return rgb([133, 59, 169]);
      if (inside(x, y, 266, 157, 352, 229)) return rgb([234, 99, 31]);
      return rgb([240, 233, 215], noise(x, y, 909, 3));
    }
  },
  {
    label: "minimal-still-life", seed: 1010, tags: ["light", "neutral", "sparse", "alpha"],
    description: "Pale neutral field, dark rectangular mark, and fully transparent border.",
    pixel(x, y) {
      if (x < 6 || x > 377 || y < 6 || y > 249) return [0, 0, 0, 0];
      if (inside(x, y, 155, 88, 227, 178)) return rgb([107, 101, 93], noise(x, y, 1010));
      return rgb([231, 226, 217], Math.floor(y / 48) + noise(x, y, 1010, 3));
    }
  }
];

const crcTable = Array.from({ length: 256 }, (_, index) => {
  let value = index;
  for (let bit = 0; bit < 8; bit++) value = value & 1 ? 0xedb88320 ^ (value >>> 1) : value >>> 1;
  return value >>> 0;
});
function chunk(name, data) {
  const type = Buffer.from(name);
  const body = Buffer.concat([type, data]);
  let crc = 0xffffffff;
  for (const byte of body) crc = crcTable[(crc ^ byte) & 255] ^ (crc >>> 8);
  const output = Buffer.alloc(data.length + 12);
  output.writeUInt32BE(data.length); body.copy(output, 4);
  output.writeUInt32BE((crc ^ 0xffffffff) >>> 0, output.length - 4);
  return output;
}
function png(pattern) {
  const pixels = Buffer.alloc(height * (width * 4 + 1));
  for (let y = 0; y < height; y++) {
    const start = y * (width * 4 + 1);
    for (let x = 0; x < width; x++) {
      const color = pattern.pixel(x, y);
      assert.ok(color.length === 4 && color.every(v => Number.isInteger(v) && v >= 0 && v <= 255));
      pixels.set(color, start + 1 + x * 4);
    }
  }
  // Stored DEFLATE blocks have stable bytes independent of zlib/library versions.
  const blocks = [Buffer.from([0x78, 0x01])];
  for (let start = 0; start < pixels.length; start += 65535) {
    const data = pixels.subarray(start, start + 65535), header = Buffer.alloc(5);
    header[0] = start + data.length === pixels.length ? 1 : 0;
    header.writeUInt16LE(data.length, 1); header.writeUInt16LE(data.length ^ 65535, 3);
    blocks.push(header, data);
  }
  let a = 1, b = 0;
  for (const value of pixels) { a = (a + value) % 65521; b = (b + a) % 65521; }
  const adler = Buffer.alloc(4); adler.writeUInt32BE((b * 65536 + a) >>> 0); blocks.push(adler);
  const header = Buffer.alloc(13);
  header.writeUInt32BE(width); header.writeUInt32BE(height, 4); header[8] = 8; header[9] = 6;
  return Buffer.concat([Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]),
    chunk("IHDR", header), chunk("IDAT", Buffer.concat(blocks)), chunk("IEND", Buffer.alloc(0))]);
}
if (write) fs.mkdirSync(directory, { recursive: true });
const manifest = {
  schema: 1,
  description: "Ten self-authored procedural test patterns. Historical filenames are stable test labels, not photographs or reproductions of artwork.",
  license: "MIT",
  copyright: "Copyright (c) 2026 Scene Palette contributors",
  generator: { file: "scripts/generate-fixtures.mjs", version: 1, dimensions: [width, height],
    randomness: "Fixed integer coordinate hash and per-pattern seeds; no external inputs.",
    encoding: "8-bit RGBA PNG, filter zero, stored DEFLATE; deterministic bytes without external packages." },
  images: patterns.map((pattern, index) => {
    const bytes = png(pattern), file = pattern.label + ".png", target = path.join(directory, file);
    if (write) fs.writeFileSync(target, bytes);
    else assert.deepEqual(fs.readFileSync(target), bytes, file + " differs from its generator");
    return { number: index + 1, label: pattern.label, category: "synthetic", tags: pattern.tags,
      description: pattern.description, seed: pattern.seed,
      source: "Original procedural test pattern generated by Scene Palette contributors", license: "MIT",
      fixture: { file, width, height, bytes: bytes.length, sha256: crypto.createHash("sha256").update(bytes).digest("hex") } };
  })
};
const manifestPath = path.join(directory, "manifest.json");
if (write) fs.writeFileSync(manifestPath, JSON.stringify(manifest, null, 2) + "\n");
else assert.deepEqual(JSON.parse(fs.readFileSync(manifestPath, "utf8")), manifest);
console.log("Synthetic fixtures: " + (write ? "written" : "verified") + " (" + patterns.length + " images; deterministic bytes; MIT)");

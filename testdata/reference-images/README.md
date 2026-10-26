# Synthetic regression images

These ten PNGs are original procedural test patterns, licensed under the
repository's MIT license. They contain no downloaded artwork, photographs,
screenshots, or pixels copied from the earlier private fixture corpus.

The filenames are stable test labels. For example, `red-teal-portrait.png`
contains geometric shapes, not a portrait; `winter-cabin.png` is a colour
pattern. The manifest records the construction, seed, dimensions, and SHA-256
hash of every file.

Generate or verify them with Node.js, without extra packages:

```sh
node scripts/generate-fixtures.mjs --check
node scripts/generate-fixtures.mjs --write
```

The generator uses integer coordinate hashes and a fixed PNG encoder with stored
DEFLATE blocks. It has no clock, system randomness, external inputs, or dependency
on a particular zlib encoder. Regenerating with the same source produces identical
PNG and manifest bytes.

Coverage includes light/dark inputs, warm/cool hues, dense/sparse signals,
gradients, hard boundaries, grayscale, low chroma, transparent borders, and
different sampling strides. These are algorithm regression inputs, not a
representative sample of users' artwork.

After intentionally changing fixtures, regenerate their golden outputs from the
actual standalone HTML functions, then run all relevant checks:

```sh
node scripts/reference-extract.mjs --write
node scripts/reference-extract.mjs --check
go test ./...
```

The oracle requires the optional Sharp development package. It writes both the
palette/stage goldens and their index. Review those changes; never replace a
golden solely to silence an unexpected algorithm regression.

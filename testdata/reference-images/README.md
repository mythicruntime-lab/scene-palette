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

The oracle requires the optional Sharp development package. Its v6 outputs live in
`testdata/v6`. The earlier `testdata/reference-palettes` and
`testdata/histogram-signal.golden.json` files retain the v5 results. With the same
pixels, v6 moves Readable UI secondary text to meet 4.5:1 on both surfaces.
This changes its repair record, computed contrast, and colour signature. A moved
core colour can also free up a source candidate for the found-ink list. The same
images and raw histograms exercise both versions. Review changed expectations
against the intended algorithm before accepting them.

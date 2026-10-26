# Palette quality and validation — 2026-10-26

Before this public repository was created, the Windows Go engine and standalone
HTML were checked on 20 publicly available images plus 10 privately supplied
regression images. The results below describe that local development evaluation.
They are not results for the public repository's replacement pattern fixtures.
Testing used fresh browser profiles and visual inspection, not recruited users;
the deliberately challenging corpus does not support population-wide quality rates.

## Public regression fixtures

This repository contains ten deterministic, programmatically generated patterns,
with new image-derived goldens and fresh Git history. The private PNGs and their
goldens are not distributed. Fixture labels such as `empty-neon-street` are retained
as stable identifiers, but now refer to generated patterns with different pixels.
Synthetic tests check regression behavior, determinism, and exact Go/HTML parity;
they do not reproduce the historical real-image evaluation below.

To run the public patterns, start the app with `serve -addr 127.0.0.1:0`, then use
its printed URL and a new output directory:

```powershell
node scripts/parity-corpus.mjs --url http://127.0.0.1:PORT/ --html path\to\release\index.html --output bin/corpus/generated-run --require-readable
```

The current run's generated report is the evidence for that run. No historical
comparison count or pass result below should be treated as a result for these patterns.
See the README for development dependencies and other verification commands.

The initial public snapshot was checked on 2026-10-26: 238 exact Go/HTML palette
comparisons (all returned variants in the bounded matrix, including pooling),
zero generated-readability failures, zero excluded core tokens, and 23 passing
browser workflow checks. The eight-package Go suite, vet, fixture reproducibility,
and JavaScript oracle/contract checks also passed. These results cover the generated
patterns, not the historical media reviewed below.

## Historical corpus and local evidence

The earlier review covered 20 publicly available illustrations, game screenshots,
photographs, and concept images, plus ten privately supplied regression images.
Five illustration references came from one animation studio and two concept images
shared an artist, so this was not exhaustive style coverage.

Original source credits, URLs, file hashes, media, and detailed per-image reports
are retained in the separate development archive. They are not distributed here.
The observations below summarize that historical review without identifying works
or implying that the generated fixtures reproduce their visual characteristics.

For additional evaluation, supply images you are permitted to use through a custom
manifest. See the README and the corpus runner's help for the manifest format.

The historical comparison was exact across the complete durable snapshot, excluding
only wall-clock `createdAt` and human-review workflow state. Each engine had a
separate fresh profile; the standalone file ran offline. Settings covered every
returned variant at baseline Auto/Dark/Light, then Auto with sampling budgets
45k/140k/400k, signal floors .04/.085/.16, and found-ink limits 0/6/10.
First-plus-last pooling was also tested. This was a bounded matrix, not the full
Cartesian product. Requested settings/variant state and API success were checked
so stale results could not count as coverage. The readability gate used unrounded
ratios on both surfaces in both engines; the historical runs had no locked/manual
core-token exclusions. The public harness retains these comparison rules.

Parity applies to the same browser-decoded working pixels. The CLI decodes original
files itself; orientation, colour profiles, large-image resizing, and decoder differences
can produce different working pixels. This review does not claim all browsers or
all image formats behave identically.

## Historical parity outcome and compatibility fix

The private corpus initially exposed five failures in 281 comparisons: the third
variant of the original `empty-neon-street` image swapped attention and rupture.
Go agreed with the original Node-generated goldens. Newer Edge's native `atan2` differed by one ULP and changed
the winner of an almost tied role-assignment cost. The RGB colours themselves matched.

The standalone hue calculation uses the same reference arithmetic already in Go.
Both histogram hue bins and OKLCh conversion use it. The strict assignment comparison
is unchanged. That numerical fix used no tolerance, hue rounding, golden replacement,
schema change, or extraction-version change. Full-precision regression assertions
cover the offending hues. The subsequent readability change below deliberately
updates extraction to version 4 while retaining schema 1.

The historical v3 run covered 281 private-image and 505 public-image comparisons,
including pooling. The historical v4 rerun passed 280 private-image and 503
public-image comparisons: **783 exact matches, zero readability failures, zero
browser/API errors, and zero excluded core tokens**. Forced-polarity repairs changed
how many distinct variants were available; every returned variant was checked.
The local Go suite, vet, JavaScript contract/fixture checks, and 23 browser end-to-end
checks also passed on that development checkout. These counts describe the original
corpora and build, not a validation result for the new public pattern fixtures.

## Findings from the historical image evaluation

### Resolved in extraction v4: forced-polarity readability failures

All 20 fresh default Auto plates passed the intended body-ink 4.5:1 and other-core-ink
3:1 targets on both surfaces. However, 28 single-image variant cases across eight fresh
images failed when the opposite polarity was forced; the pooled case added one.
The private corpus added 11 failing forced-polarity cases. These were shared v3
algorithm failures, not Go-port mismatches or rounding at a threshold.

The v3 repair ran against ground and then panel. When those surfaces needed opposite
repair directions, the second repair could undo the first. A dark game scene forced Light
reached about 1.03:1 on ground for a core accent while passing panel; the lunar
interior forced Dark had body ink at 1.63:1 on ground while passing 4.5:1 on panel.

Version 4 preserves every passing sequential repair and solves failing cases against
both surfaces together. It searches lightness first, then neutral grays, then feasible
8-bit RGB colours. The last fallback matters for narrow bands: black and `#FBFBFB`
admit a 4.5:1 coloured ink even though no representable gray passes. All accepted
colours pass strict, unrounded ratios on both surfaces.

When generated surfaces cannot share any body ink, they are adjusted toward the
requested polarity while retaining panel separation. Origins and notes record the
changes. Locked colours remain exact. Unlocked inks adapt to locked surfaces where
possible; incompatible locked surfaces produce a warning. Raw found inks and manual
paint retain their existing meaning and are not promised automatic readability.

The historical regression run covered the narrow RGB case, impossible pinned
surfaces, surface repair provenance, all private images in all modes,
uniform/transparent inputs, and 240 direct Go/JavaScript solver cases. The public
suite retains mathematical and contract checks, and uses generated patterns for
image-dependent regression tests. Its browser suite checks generated readability
in Auto, Dark, and Light, including a deliberate manual failure for the ground
warning. Existing imports and seals retain their compatibility behavior; re-forging
uses v4. The historical Auto outputs were unchanged apart from the version marker;
new synthetic goldens do not claim equivalence to the private images they replace.

### P2: generated accents can lose the source's visual identity

Ten of the 20 default plates require at least one invented accent. The grayscale
portrait photograph produces four invented hues; a space photograph, an aurora, and
a snowy environment concept each need three. The labels and origin metadata disclose this,
but the resulting interface can look much more colourful than its reference.
A near-white drawing also gains strong teal/brown accents. This is a product
tradeoff between four semantic roles and fidelity to a sparse source.

**Next decision:** offer a source-faithful mode with fewer accent roles, or stronger
control over invented colours. Do not imply those colours were sampled from the image.

### P2: derived accents can be too similar to each other

Three default plates fall below the selector's 0.07 OKLab distance threshold:
a dark blue illustration (observed/rupture, 0.0668), a dim interior illustration
(attention/gold, 0.0679), and a snowy environment concept (attention/gold, 0.0675). The derived top-up path does not enforce the same
final distinctness test used for real four-colour sets. Similar roles are especially
weak in status-heavy UI. These are diagnostics in this colour space, not a claim
that a single distance threshold proves accessibility.

**Next fix:** test final repaired/derived accents jointly, including role distinction.

### P2: readability repair can strongly change pale source colours

Eleven default plates contain a real signal accent shifted by more than 0.10 OKLab
distance. In the lunar interior, pale yellow `#FBF598` becomes olive `#686200`
(0.469). A drawing's pale green `#CDECA4` becomes `#68833F` (0.331). The output is
readable, but the luminous or delicate mood is lost. This is distinguishable from
invention: the colour has a source, followed by a large recorded repair.

**Next improvement:** show source and repaired paint together and help the user choose
between fidelity and contrast. The existing pixel-pick/lock workflow can preserve an
exact colour, but the user must then assess its readability.

The generated-fixture checks and historical review answer different questions:
the former protect deterministic behavior and implementation parity; the latter
identify visual tradeoffs to consider when applying palettes to actual interfaces.
Neither establishes a universal measure of palette quality.

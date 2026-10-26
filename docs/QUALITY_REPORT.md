# Historical local palette review — 2026-10-26

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

`testdata/fresh-corpus.json` retains the 20 public source URLs, file hashes, credits,
and usage notes. It contains metadata only: the media is not included in this
repository or Windows packages. The five anime examples were all Ghibli, and two
concepts shared an artist, so this was not exhaustive style coverage.

Reproducing the public-image portion requires obtaining the source files separately
under their applicable terms and placing them at the manifest's local paths. With
those files available, the historical matrix can be rerun into a new directory:

```powershell
node scripts/parity-corpus.mjs --url http://127.0.0.1:PORT/ --html path\to\release\index.html --manifest testdata/fresh-corpus.json --output bin/corpus/fresh20-rerun --require-readable
```

Playwright is a development dependency; the historical run used installed Edge
headlessly. Its reports contained thumbnails, default swatches, full palettes,
per-case metrics, input hashes, and exact JSON differences. These paths identify
local evidence retained in the original development checkout. They are not files
or downloadable links in this public repository:

- `bin/corpus/fresh20-readability-v4/report.html` — historical 20-image report.
- `bin/corpus/regression-readability-v4/report.html` — historical private-image report.
- `bin/readability-v4-validation.json` — historical Windows build and test evidence.

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
repair directions, the second repair could undo the first. Hades forced Light
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
Migrant Mother photograph produces four invented hues; Earthrise, aurora, and the
snowy Spring concept each need three. The labels and origin metadata disclose this,
but the resulting interface can look much more colourful than its reference.
Kaguya's near-white drawing also gains strong teal/brown accents. This is a product
tradeoff between four semantic roles and fidelity to a sparse source.

**Next decision:** offer a source-faithful mode with fewer accent roles, or stronger
control over invented colours. Do not imply those colours were sampled from the image.

### P2: derived accents can be too similar to each other

Three default plates fall below the selector's 0.07 OKLab distance threshold:
Nausicaä (observed/rupture, 0.0668), Spirited Away (attention/gold, 0.0679), and
Spring (attention/gold, 0.0675). The derived top-up path does not enforce the same
final distinctness test used for real four-colour sets. Similar roles are especially
weak in status-heavy UI. These are diagnostics in this colour space, not a claim
that a single distance threshold proves accessibility.

**Next fix:** test final repaired/derived accents jointly, including role distinction.

### P2: readability repair can strongly change pale source colours

Eleven default plates contain a real signal accent shifted by more than 0.10 OKLab
distance. In the lunar interior, pale yellow `#FBF598` becomes olive `#686200`
(0.469). Kaguya's pale green `#CDECA4` becomes `#68833F` (0.331). The output is
readable, but the luminous or delicate mood is lost. This is distinguishable from
invention: the colour has a source, followed by a large recorded repair.

**Next improvement:** show source and repaired paint together and help the user choose
between fidelity and contrast. The existing pixel-pick/lock workflow can preserve an
exact colour, but the user must then assess its readability.

## Historical default plate observations

All rows below passed the measured contrast targets in Auto during the historical
public-image evaluation. Distances are minimum accent-to-accent Euclidean OKLab
distances. The historical visual report marked derived accents with stars;
diagnostic flags were not counted as parity failures. These source attributions
and observations do not describe the generated patterns in this repository.

| Source | Auto | Variants | Derived / 4 | Min distance | Visual observation |
|---|---|---:|---:|---:|---|
| [Princess Kaguya](https://www.ghibli.jp/works/kaguyahime/) | Light | 1 | 2 | .080 | Pale paper retained; strong added accents and darker green lose delicacy. |
| [Ponyo](https://www.ghibli.jp/works/ponyo/) | Dark | 1 | 2 | .081 | Blue/pink survive; invented cyan/yellow broaden the scene. |
| [Nausicaä](https://www.ghibli.jp/works/nausicaa/) | Dark | 1 | 2 | .067 | Dark blue atmosphere survives; two cool roles are close. |
| [Howl's Moving Castle](https://www.ghibli.jp/works/howl/) | Light | 5 | 0 | .103 | Varied skin/clothing scene supplies several usable sets; yellow is darkened. |
| [Spirited Away](https://www.ghibli.jp/works/chihiro/) | Dark | 1 | 2 | .068 | Train's dark mood survives; attention/gold are close. |
| [Celeste](https://store.steampowered.com/app/504230/) | Dark | 6 | 0 | .181 | Snow, dialogue UI, and small saturated details provide distinct accents. |
| [Stardew Valley](https://store.steampowered.com/app/413150/) | Dark | 6 | 0 | .148 | Dense warm pixel art yields variety without invention. |
| [Hades](https://store.steampowered.com/app/1145360/) | Dark | 6 | 0 | .197 | Neon effects and contrasting environment colours separate well. |
| [Factorio](https://store.steampowered.com/app/427520/) | Dark | 4 | 0 | .077 | Earth/fire palette stays warm; separation is comparatively narrow. |
| [Hollow Knight](https://store.steampowered.com/app/367520/) | Dark | 1 | 1 | .075 | Soft glowing fight scene gives fewer distinct accents and one invention. |
| [Earthrise](https://science.nasa.gov/resource/apollo-8s-iconic-earthrise/) | Dark | 1 | 3 | .079 | Sparse blue subject on black cannot supply four source hues. |
| [Migrant Mother](https://commons.wikimedia.org/wiki/File:Lange-MigrantMother02.jpg) | Dark | 1 | 4 | .082 | Grayscale structure is faithful; all four accents are invented. |
| [Iceland aurora](https://commons.wikimedia.org/wiki/File:Aurora_Borealis-Northern_Lights_Iceland.jpg) | Dark | 1 | 3 | .089 | Faint sky colour mostly falls below the default signal floor. |
| [Tokyo neon](https://commons.wikimedia.org/wiki/File:Neon-street-in-Tokyo-Japan.jpg) | Dark | 6 | 0 | .234 | Many small bright signs produce the strongest default role separation. |
| [Clownfish](https://commons.wikimedia.org/wiki/File:Clown_fish_with_Anemone.jpg) | Dark | 6 | 0 | .125 | Saturated violet/orange survive; deep blue is lifted substantially. |
| [Sintel city](https://www.davidrevoy.com/article45/sintel-art-direction) | Light | 1 | 1 | .103 | Sepia mood retained, with a conspicuous added teal role. |
| [Spring environment](https://studio.blender.org/projects/spring/?asset=837) | Light | 1 | 3 | .068 | Snow/stone structure works; added saturated accents weaken the subdued mood. |
| [Toroidal colony](https://www.nasa.gov/image-article/retrofuturistic-nasa-space-art/) | Light | 6 | 0 | .200 | Dense multicolour painting supports distinct roles; pale signals darken. |
| [GJ 486 b](https://science.nasa.gov/asset/webb/exoplanet-gj-486-b-artist-concept/) | Dark | 4 | 0 | .075 | Tiny star and rim supply warm variants, with limited role separation. |
| [Lunar interior](https://www.esa.int/ESA_Multimedia/Images/2020/11/Habitat_workstations) | Light | 5 | 0 | .095 | Pale architecture yields heavily darkened accents, changing its mood. |

The readability repair addresses the forced-polarity failures. Invented hues, similar
derived accents, and large source-colour adjustments remain separate quality limits.

# Scene Palette

Drop a still (Edgerunners shot, concept art, tool screenshot). Get a **named plate**:

ground · panels · ink · muted · observed · attention · rupture · gold

Each slot has a name, a job, and a hex. Edit the words. **Arm a slot and click a pixel on the still
to force that exact artist colour into that role** — it locks, and it survives every re-forge after.

In the Windows portable edition, extract the ZIP and double-click `scene-palette.exe` or
`Run Scene Palette.cmd`. It opens the local app in your default browser; no Go installation is needed.
You can also open the included `index.html` directly for standalone mode. The frontend has no build
step or internet dependency.

Export: CSS tokens, Godot 4 colour pack, JSON plate, GPL, HEX.

Build UI themes and Godot 2D / 2.5D colour packs from your own images.

The standalone HTML needs only a current browser. Building or running from source requires
Go 1.23 or newer. Node.js 20 or newer is needed only for development checks; see
[CONTRIBUTING.md](CONTRIBUTING.md) for optional image and browser test dependencies.

---

## Local Go engine and standalone frontend

The launcher serves `index.html` with a local Go engine. Forge, Remix, polarity changes, and derived
counterparts use the Go extraction pipeline; Bundle, Export vault, and Rebuild index use the Go
archive and contract code. The browser owns editing, shelf storage, review decisions, and previews.
Images and palettes stay on this device. Opening `index.html` directly retains the complete standalone
JavaScript workflow, with the same extraction results and durable file format.

```powershell
go run . serve -open
go run . forge -out palette.json -name "Night bay" C:\path\to\still.png
go run . forge -bundle night-bay.zip -name "Night bay" C:\path\to\still.png
go run . bundle -out approved.zip palette.json C:\path\to\still.png
go run . check -json palette.json
go run . rebuild-index C:\path\to\palette-vault
go test ./...
node scripts/verify-js-index.mjs
node scripts/color-math-fixture.mjs --check
```

The migration boundary and the logic deliberately left in JavaScript are recorded in
[docs/GO_MIGRATION.md](docs/GO_MIGRATION.md). Contract fixtures live under `testdata/vault`;
deterministic generated test patterns and their extraction baselines live under
`testdata/reference-images` and `testdata/reference-palettes`. No private source images or
old Git history are included. Generate or verify the patterns with
`node scripts/generate-fixtures.mjs --write` or `--check`.
The optional image-oracle check, `node scripts/reference-extract.mjs --check`, runs the
actual functions from `index.html`, verifies `testdata/histogram-signal.golden.json`, and
requires the development-only `sharp` package. `internal/colorx` and `internal/extract` are parity-tested
Go libraries through plate assembly — names, origins, variants, notes, and found inks included.
`internal/live` preserves locked roles and pinned inks while rebuilding from working pixels or a cached
histogram. `internal/palette` serializes the durable browser contract, including facets and signature.
Human-review metadata is still created by the browser; Go independently verifies its declared
fingerprint scope.
A CLI-forged plate without review metadata is intentionally unsealed.

Browser end-to-end checks run with `node scripts/browser-e2e.mjs --url http://127.0.0.1:8765/`
against a running server. They require the development-only `playwright` package and a browser; on
Windows the script uses installed Edge. `SCENE_PALETTE_SHARP` and `SCENE_PALETTE_PLAYWRIGHT` can point
at existing package directories, or expose those packages through `NODE_PATH`. Test downloads,
screenshots, and the report stay under ignored `bin/e2e/`.

The broader corpus check compares every generated variant with the original standalone JavaScript
engine, using fresh browser profiles and an offline HTML context:

```powershell
node scripts/parity-corpus.mjs --url http://127.0.0.1:8765/ --output bin/corpus/regression-run --require-readable
node scripts/parity-corpus.mjs --url http://127.0.0.1:8765/ --manifest path\to\manifest.json --output bin/corpus/fresh-run --require-readable
```

By default it uses the ten generated regression patterns. A custom manifest supplies an `images` array
with `id`, `file` (relative to the manifest), `category`, and source metadata. `--html path\to\index.html`
selects the exact shipped HTML for comparison. Outputs include exact JSON differences, per-image
palettes, readability and colour diagnostics, and an offline HTML inspection report. Use a new output
folder for each run. `--require-readable` also fails the run when a generated, unlocked core ink
misses its target on either surface in either engine. Locked/manual exclusions are reported;
raw found inks and aesthetic diagnostics remain outside that gate.
The dated [quality report](docs/QUALITY_REPORT.md) records a historical local evaluation of
20 public images and its measured weaknesses. Those image files and reports are not included.
`testdata/fresh-corpus.json` retains source references and hashes; using it requires obtaining
the images separately under their owners' terms. Generated-pattern parity tests do not replace
that real-image review.

`forge` accepts one to eight PNG, JPEG, or GIF files and pools them with equal weight. Browser defaults
are preserved: a 140,000-pixel budget per still, `.085` signal floor, six found inks, auto polarity,
and the first variant. Use `-polarity dark|light`, `-variant 2`, `-found`, `-chroma-min`, or `-budget`
to override them. JSON goes to stdout by default; `-out palette.json` writes a file. Use
`-bundle plate.zip` instead to include the original source bytes, generated preview, CSS, Godot,
GPL, and notes in a deterministic ZIP. `bundle` also packages an existing palette while preserving
its human seal and all recorded metadata. Optional source files are matched by their recorded content
hash, regardless of argument order; unavailable sources are explicitly marked metadata-only. A single
bundle never contains `index.json`.

The CLI decodes originals with Go's PNG/JPEG/GIF readers. The browser first decodes and caps a working
canvas at 1400 pixels on its long edge, then sends lossless PNG working pixels to the loopback engine.
This preserves browser orientation, colour management, alpha handling, and the recorded transform chain.
CLI and browser extraction are identical for identical working pixels; large originals or different
image decoders can produce different working pixels.

`check` is the safe agent entry point. It validates the durable contract, recomputes the fingerprint
declared by the browser seal, and exits with an error unless the plate has a complete match. New seals
bind the semantic palette content **and** the normalized source filename/hash identities. Legacy
`palette-content` seals remain readable and are explicitly reported as content-only:

```powershell
go run . check palette.json
go run . check -json palette.json
go run . check -allow-unsealed palette.json  # inspection only; never grants eligibility
```

`-json` returns the plate name, polarity, role colors, contract state, verified seal `scope`, recorded
and computed fingerprints, the recorded proposal-decision counts when present, an independently
recomputed four-check review assessment, and a single `agentEligible` decision. Missing review metadata,
unsupported review versions or scopes, invalid timestamps, changed palette content, and changed
source identities under the new scope all fail closed. `-allow-unsealed` changes only the process exit
result so a person can inspect a valid draft; the report still says `agentEligible: false`.

On Windows, `Run Scene Palette.cmd` first uses an adjacent `scene-palette.exe`. In a source checkout
it otherwise finds Go from `PATH` or the standard Program Files install and runs `serve -open`.
If neither is available, it opens the standalone JavaScript app. Keep the console window open while
using the Go app; close it to stop the server. Running the executable without arguments opens the browser;
explicit `serve` stays headless unless `-open` is supplied.
If another local tool already owns port `8765`, the launcher automatically chooses an available
loopback port and opens that address. An explicit `serve -addr ...` remains strict.

Build a local portable Windows x64 release with `powershell -NoProfile -File scripts/package-windows.ps1`
(Go and Git are needed only on the build machine).
Each invocation creates a fresh folder and ZIP under ignored `bin/releases/`, containing the executable,
the exact standalone `index.html`, launcher, usage notes, licenses, build provenance, and SHA-256 checksums.
The build uses `CGO_ENABLED=0` and records the source commit and any uncommitted source changes. It is
unsigned and is not published automatically. Image test corpora and test reports are not included.

## How it picks

**Structure** (ground, panel, ink, muted) comes from luminance quantiles over the whole frame, so the
plate keeps the still's own cast — an Edgerunners alley grounds out warm brown, not neutral grey.
Initial chroma is capped per slot, keeping structural colours subdued before readability repair.

**Polarity is read from the frame.** A night bay grounds out on its darks; a hazy daytime facade
grounds out on its brights and gets a light plate — dark ink on pale ground. Forcing the opposite
polarity can require stronger surface and ink adjustments. The **Auto / Dark / Light** control
overrides the guess: use it when a dusk or interior
still sits near the threshold, or when you want a dark UI from a daytime reference. Flipping rebuilds
from the cached histogram, so it is instant and never re-reads the image.

**Signal** (observed, attention, rupture, gold) is binned by hue in OKLCh and weighted
`sqrt(area) · chroma²`. That is what keeps an 8-pixel lime cuff — about 0.2% of the frame — from being
averaged away by a quarter-frame of dull orange. Candidates are repaired to 3:1 on both surfaces
before they compete, and real four-colour sets are checked for distinctness. Roles are matched by
brute-forcing all 24 permutations against each role's ideal hue.

**Readability repair checks both surfaces together.** Generated `ink` targets 4.5:1 and `muted` plus
the four accents target 3:1. Passing repairs are retained exactly. A failing repair searches OKLCh
lightness, then neutral grays, then feasible 8-bit RGB colours. Generated surfaces adjust only when
they cannot support any shared body ink. Surface and ink changes are recorded in their origins.
The repair can still move a pale or muted source colour substantially; inspect the Readability matrix
and provenance, especially when using locked paint.

Real paint always wins. A slot is only **derived** when the still cannot offer a fourth hue you could
actually tell apart from the other three — two role slots reading as the same red is worse than one
honest invention. Derived slots are badged on the swatch and called out in the notes.

## Variants and remix

A busy frame supports more than one honest plate. Forge ranks every four-colour set the still can
carry and shows them as a **Variants** strip — click one to apply it, or press **Remix** to cycle.
Structure is shared across variants; only the four signal slots swap, and locked slots ignore the
switch entirely. Each plate is named after its rarest colours so the words tell them apart. A
two-colour still honestly reports one plate and disables Remix.

## Readability

The **Readability** matrix checks every token against *both* surfaces, not just ground, plus the
label colour each filled accent would paint with. `AA` is 4.5:1 (body copy); `pass` is 3:1, the floor
for large text and UI components. A `low` can identify a pinned colour or a generated repair that
failed on one surface. Pinned colours are never auto-corrected.

## Preview surfaces

The preview has four tabs, and **Compare follows whichever one is selected** — so you can put two
plates side by side on the surface you actually care about.

| Surface | What it stresses |
| --- | --- |
| **Agent harness** | The original: trace colours, status dots, filled buttons on ground. |
| **Forms** | Panel-vs-ground separation, borders, muted labels, the `observed` focus ring, the `rupture` error state, disabled text. |
| **Table** | Zebra rows, header rules, right-aligned numerics, and all three status accents at once. |
| **Approval** | The only surface that exercises `gold` — the reserved "human acts only" slot has no other preview. |

Every surface is markup on the `--p-*` vars alone; none of them names a colour. Label colours on filled
accents are computed per plate (black or white, whichever reads), for all four accents.

## Shelf, import, compare

**Save** puts the current plate on the shelf; loading that card makes it the current shelf plate and
changes the action to **Update**. Update revises that entry in place, so its identity and A/B pins
survive, and it records review or seal changes even when the colours did not move. An Undo action is
offered after each update. If a theme pair uses that revision, the pair is cleared rather than leaving
a stale export behind.

The shelf survives a reload via browser storage, and survives everything else via **Export shelf** /
**Import shelf** JSON. Storage on `file://` is unreliable — Chrome usually allows it but shares one
bucket across all local files, and Safari blocks it outright — so the tool warns once if a save is
refused and the export file is the durable path.

Shelf import is additive and offers Undo. It skips only exact saved revisions, so a human-sealed or
source-changed revision is not discarded merely because its colours match an unsealed plate. Imported
IDs that collide are remapped locally, keeping **Update** and A/B attached to the right card. At the
40-plate limit, existing local plates are never evicted; extra imported entries are reported as held
back and remain in the import file.

Loading a JSON or shelf plate keeps its recorded source set separate from whatever image canvases are
currently open. Re-exporting therefore preserves the imported provenance instead of relabelling the
plate with unrelated workspace stills. Picking from an open still adds that still to the plate record;
forging again deliberately resets the record to the stills used for that extraction.

A standalone palette round-trip also retains its creation time, original generator/extraction identity,
tags/family, extraction parameters, median luminance, and notes. Token IDs, jobs, derived flags, locks,
and origins survive as well, so merely opening and re-saving a plate cannot break its matching human
seal. An actual **Forge** resets the generator to the current algorithm. Colour-derived facets and
signatures are rebuilt from the normalized eight-role plate rather than carrying stale computed values.

**Import** loads a JSON plate back in, and a `.json` dropped on the still zone does the same. Import is
tolerant on input and strict on output: keyed or positional, `hex` or `rgb`, missing slots filled with
defaults and counted in the notes. It never guesses positionally when keys are present.

**Compare** puts two saved plates side by side as full harness previews with a role-by-role diff and
ΔE per changed slot. Both previews and the live one come from one template, so they cannot drift.

## Themes — two plates, one file

A day facade and a night bay should not be merged into one plate: pooling them puts median luminance
in the middle, which lands `ground` in the muddy mid-tone the polarity logic exists to avoid. The
honest shape is two plates that ship together.

Forge each still on its own, **Save** both, pick them as **A** and **B**, then **Pair A + B**. Sides
are ordered by their own polarity, not by which you picked first. If you only have one still,
**Derive counterpart** builds the opposite polarity from the cached histogram — and the report will
tell you when that produced a mid-toned "dark" side rather than a real one.

The report checks each side independently, flags an accent whose hue shifts more than 40 degrees
between sides (it will read as a different colour), and trims found inks to the count both sides
share — a var that exists on one side only would vanish on switch.

CSS comes out as the full cascade:

```css
:root { color-scheme: dark light; --ground: #070505; ... }
:root[data-theme="light"] { --ground: #E7E5CC; ... }
@media (prefers-color-scheme: light) {
  :root:not([data-theme="dark"]) { --ground: #E7E5CC; ... }
}
```

Explicit choice wins, system preference applies otherwise, and asking for dark by name still gets
dark. Godot exports both sides as dictionaries behind `plate(light: bool)`. Two plates of the same
polarity pair as base/alt instead, and say so.

## The vault bundle

**The folder is the authority. `index.json` is a cache.** Every field in the index is copied from, or
recomputed from, a `palette.json`, so deleting the index loses nothing — `buildIndex()` reconstructs it
from the folders alone. Nothing in the bundle layer reads application state, which is what makes that
true: a bundle written today can be re-indexed by something that has only the folder and none of this
code.

**Bundle** writes one plate as a zip:

```
night-bay/
  palette.json     the contract — schema 1
  preview.png      swatch sheet
  tokens.css       CSS custom properties
  plate.gd         Godot 4 const pack
  palette.gpl      GIMP/Aseprite/Krita palette
  notes.md         human-readable, with provenance
  sources/
    01-night-bay.png   the original bytes, unmodified
index.json         the cache, rebuildable from the above
```

### What a plate records about itself

| Field | Holds |
| --- | --- |
| `schema` | The contract version. Bump = a breaking file-shape change; optional metadata remains backward-compatible. |
| `generator.version` | The application that wrote it. |
| `generator.extraction` | The **algorithm** version. Moves only when extraction output changes, so you can tell a plate re-forged by new code from one merely re-saved. |
| `sources[]` | Per still: original filename, byte count, `hash` (sha256, or a labelled `fnv1a64` fallback where `crypto.subtle` is unavailable), `natural` and `working` dimensions, and the `transforms` chain between them. |
| `params` | Sample budget, signal floor, found-ink count, polarity mode, variant index. |
| `facets` | Derived, never typed — polarity, dominant hue, accent hues, hue spread, chroma range, temperature, derived/picked slot counts, min contrast, panel separation. |
| `signature` | Similarity vector: per-role OKLab, a 12-bin area-weighted hue histogram, per-role lightness. |
| `review` | Optional versioned human-review state, including the proposal delta, advisory assessment, and up to 20 superseded seals as recorded history. New seals use scope `palette-and-source-identities`; legacy `palette-content` seals remain verifiable but are labeled content-only. Only a complete, matching supported seal authorizes agent use. |

The hash is always **labelled with its algorithm**. A fallback is never silently substituted for sha256.

### Per-colour origin

Every token says where it came from, and no colour is unattributed:

- **`structure`** — the luminance quantile it came from, whether chroma was capped, and every repair
  applied (`{op, against, target, from, to}`), so a lifted `ink` shows both its original and final hex.
- **`signal`** — hue bin, band, frame area, salience, and **which stills** contributed to that bin.
- **`derived`** — the role's ideal hue and the reason the still could not supply a real one.
- **`picked`** — the still's filename *and content hash*, the pixel in **both** coordinate spaces
  (`working` as clicked, `natural` mapped back to the original image), the 3×3 sampling method, and a
  timestamp. Its `repairs` array is always empty: a hand-picked colour is law and is never
  auto-corrected.
- **`imported`** — carried through verbatim when present in the source file; otherwise marked as having
  no recorded origin rather than being invented.

Origins survive a JSON round trip, so re-importing a plate does not launder its provenance.

### Human review boundary

Forge and Remix always create a **proposal** and snapshot its compact decision baseline: plate identity,
role values and wording, locks, found inks, and source identities. Renaming, repurposing, locking,
picking, removing an ink, or changing the source set marks it **human-edited**. The review control shows
the exact proposal-to-current before/after delta. Either state is useful for exploration and export, but
neither is agent-eligible. **Seal for agents** records that human decision, its delta, a timestamp, and a
deterministic fingerprint over the palette's semantic content plus normalized source filename/hash
identities. Any later content or source-identity edit invalidates it and moves the superseded decision
into a bounded history of the 20 most recent seals, with the invalidation time and reason.

**Review focus** puts four deterministic checks beside that decision: readability on both surfaces,
native token origins, source hashes, and any derived or fallback core roles. An attention result is not
a confidence score or an automatic veto; it tells the person what needs judgment, and the seal remains
the person's decision. The browser records the assessment in `review.assessment` while `check -json`
recomputes it from the current file.

A plate with no `review` object—including output from `forge` and older shelves—is unsealed by
definition. Agents should use `check -json` and continue only when its independently computed result says
`agentEligible: true`. Exported `review.proposal` and `review.decision` preserve what the person accepted
and the recorded change counts; `check -json` exposes the summary as `recordedDecision`. Exported
`review.history` preserves superseded seals, and `check -json` exposes them as `recordedHistory`. Those
entries are recorded audit provenance only: the earlier palette content is not retained, so prior
fingerprints cannot be independently reverified. Only the current fingerprint is checked. New seals
detect changed source identities as well as changed palette content; the deterministic checksum is not
a cryptographic identity signature.

### Where the vault lives — settled

`probe-file-access.html` answered this: **`showDirectoryPicker` is not exposed to `file://`.** Chrome
gates the File System Access API to non-opaque origins, and a `file://` page has an opaque one. So there
is no directory backend here, and **ZIP is the backend, not a fallback.**

That has one consequence worth knowing, because it is not obvious: a zip cannot *update* a folder, so a
one-plate **Bundle** deliberately contains **no `index.json`**. Unzipping it into a vault root would
otherwise overwrite a real index with a single-entry one. The index is only ever written over a whole
vault:

- **Export vault** — every plate on the shelf plus the live one, as one tree with a correct index.
- **Rebuild index** — point at a vault folder on disk; it reads the `palette.json` in every subfolder
  and writes a fresh `index.json`. This is the reconstructibility claim made good: no app state, no
  stored handle, just the files. Unzip bundles from anywhere into one root, rebuild, and the index is
  correct.

Shelf and imported entries may not have their original source bytes — localStorage could not hold them
and a standalone JSON file does not carry them. Bundle and vault exports keep those source records but
mark them `"retained": false`, with `path` set to `null` rather than pointing at a file that is not in
the zip. Any matching source still that is actually open can be reattached by its content hash.

If `crypto.subtle` is unavailable the bundle falls back to a **labelled** `fnv1a64`. The sidebar
diagnostics show which algorithm your sources actually got, under `source hash`.

## Controls

| Control | What it does |
| --- | --- |
| **Auto / Dark / Light** | Polarity of the plate. Auto reads it from the still; the other two override. Rebuilds instantly. |
| **Signal floor** | Minimum chroma to count as signal. Drop it to catch fainter neon. |
| **Found inks** | How many non-role colours to keep alongside the core eight. |
| **Quality** | Pixels sampled per still (45k / 140k / 400k). Sampling is strided at full resolution — never downscaled, which would blur thin neon away. |
| **Remix** | Cycles to the next plate this still supports. |
| **Bundle** | One plate as a vault bundle — palette.json, sources, preview, tokens. No index by design. |
| **Export vault** | Every plate on the shelf plus the live one, as one tree with a real index.json. |
| **Rebuild index** | Point at a vault folder; regenerates index.json from the palette.json in each subfolder. |
| **Save / Update / Import** | Save creates a shelf entry; Update revises the loaded shelf plate without changing its identity or A/B pins; Import loads a JSON plate (or drop the `.json` on the still zone). |
| **A / B** | On a shelf card, pins that plate to one side of Compare — and to one side of a theme pair. |
| **Pair A + B** | Makes the two compared plates a light/dark theme. |
| **Derive counterpart** | Builds the opposite polarity of the current plate from the cached histogram. |
| **pick** | Arms that slot. The next click on the still writes that pixel into the role and locks it. |
| **lock** | Holds a slot through a re-forge, a remix, and a polarity flip. Lock one or two, then Forge again to re-roll only the rest. |
| **Try the plate on this app** | Repaints this tool in the extracted plate. Reversible — the checkbox puts it back. |

Click the still with nothing armed to pin a **found ink**. Use its remove button to drop it.
`Esc` cancels an armed slot.

A hand-pinned colour is law: it is never auto-corrected for contrast, even below target. The slot
badge turns red and the notes say so, but the pixel you chose is the pixel you keep.

Extraction version 4 repairs generated inks against ground and panel together: body ink targets
4.5:1 and the other core inks target 3:1 on both. Passing version-3 repairs are preserved. When
generated surfaces cannot support any shared body ink, they are adjusted in the selected polarity,
with the changes recorded in their origins. Unlocked inks also adapt to locked surfaces when a
shared readable colour exists. Incompatible locked surfaces produce a warning and keep your paint.
Existing saved palettes and review seals remain compatible; re-forging records version 4.

For keyboard picking, focus the still, move the sample with the arrow keys (`Shift` moves ten pixels),
and press `Enter` to apply it. Clear, still removal, found-ink removal, and shelf removal offer a
single-step **Undo** action.

## Files

- `index.html` — the whole tool.
- Saved plates live in browser storage under `scene-palette.shelf.v1`, plus whatever you export.
- `probe-file-access.html` — one-off diagnostic for the directory backend. Safe to delete once you have the answer.
- `index.legacy.html` — the previous ColorThief-based version, kept for reference.
- `vendor/color-thief.umd.js` — used only by the legacy file; the current tool has no dependencies.

## License and contributions

Scene Palette code, documentation, and generated test patterns are licensed under the
[MIT License](LICENSE), copyright 2026 Scene Palette contributors. Bundled/adapted components
retain their own notices in [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md). External images
referenced by the optional evaluation manifest remain the property of their owners and are
not included or relicensed.

See [CONTRIBUTING.md](CONTRIBUTING.md) for development and test instructions, and
[SECURITY.md](SECURITY.md) for privacy, vulnerability reporting, and local secret scanning.

# Scene Palette user guide

On an empty workspace, **Try a sample** generates a palette from the supplied blue landscape.
The three sample thumbnails stay available beside image upload so you can switch to violet
or amber, or choose your own images. Samples replace the current workspace and run the same
image-analysis and export workflow as uploads; Clear/Undo can restore prior work.
The artwork, including its original text, is bundled inside the HTML and works offline.
The blue example starts in Light polarity; violet and amber start in Dark. These are
normal generation settings, and you can change polarity or colour policy afterward.

Start with the [README](../README.md#quick-start--no-installation) for setup and your first palette.
This guide explains the controls, file formats, and advanced workflows.

## How it picks

**Structure** (ground, panel, ink, muted) comes from luminance quantiles over the whole frame, so the
plate keeps the still's own cast, such as the warm brown surfaces of a softly lit alley.
Initial chroma is capped per slot, keeping structural colours subdued before readability repair.

**Polarity is read from the frame.** A night bay grounds out on its darks; a hazy daytime facade
grounds out on its brights and gets a light plate — dark ink on pale ground. Forcing the opposite
polarity can require stronger surface and ink adjustments. The **Auto / Dark / Light** control
overrides the guess: use it when a dusk or interior
still sits near the threshold, or when you want a dark UI from a daytime reference. Flipping rebuilds
from the cached histogram, so it is instant and never re-reads the image.

**Signal** (observed, attention, rupture, gold) is binned by hue in OKLCh and weighted
`sqrt(area) · chroma²`. This helps a small bright detail survive beside a large dull area.
Candidates are repaired to 3:1 on both surfaces before selection. Role assignment evaluates all
24 permutations against the role hues; the selected colour policy controls how missing paint is supplied.

**Readability repair checks both surfaces together.** Generated `ink` targets 4.5:1 and `muted` plus
the four accents target 3:1. A failing repair searches OKLCh lightness, neutral grays, then feasible
8-bit RGB colours. Generated surfaces adjust when they cannot support shared body ink. Locked paint
stays exact, and incompatible locked surfaces produce a warning. Repairs are recorded in origins.

### Colour policy

**Readable UI** is the default. It favours distinct semantic accents and can derive missing hues.
After contrast repairs and locks are applied, it checks the four final accent colours again. It tries
available observed replacements before bounded generated adjustments, keeping contrast checks and
locks intact. If the search cannot separate a pair, it reports that limitation; it does not claim
that every possible palette was searched. The final distinction floor is 0.07 OKLab distance.

**Source hues** keeps all eight roles while favouring the source's observed hue families. When an
image has too few signal colours, it reuses source hues or neutral structural paint with tonal
variations instead of completing the palette with arbitrary role hues. Generated tonal reuse is
still marked **derived**. Similar accents are expected for monochrome and sparse inputs; use labels,
shapes, and position as well as colour to distinguish meanings.

Both policies preserve readability targets and exact locks. Source hues is a preference, not a
promise that every final colour matches a source pixel: forced polarity or a narrow contrast range
between locked surfaces can require a hue shift or added chroma. The notes explicitly identify
roles whose source hues shifted for readability. Neutral and grayscale inputs otherwise retain
neutral paint rather than receiving a minimum-chroma accent fill.

For observed accents with a large source adjustment, both engines also try a bounded lightness/chroma
search to reduce distance from the recorded source. It retains the current readable result as a
candidate, preserves contrast, and in Readable UI preserves the existing distinction floor. This is
a local search over a fixed candidate set, not a global nearest-colour guarantee. A later distinction
repair may still move an accent farther away. Found inks stay raw source paint.

### Inspect and restore source paint

The colour diagnostics list derived accents, adjustments above 0.10 OKLab distance, and final accent
pairs below 0.07. Open a role's **why** details to see **Before adjustment** and **Current** swatches,
their distance, and the recorded repairs. These are available when the token has a valid recorded
source colour; imported files without that provenance cannot provide a source comparison.

**Use source colour** restores that recorded paint to a core role and locks it. This is a deliberate
human edit: it can fail contrast or reduce accent distinction, changes the human-review state, and
invalidates a current seal. The toast's **Undo** restores the prior workspace, including its review
state. Unlock the role to let future extraction adapt it again. Loading an older plate alone does
not apply these changes or run the new colour policy.

## Variants and remix

A busy frame can support several four-colour sets. The **Variants** strip applies a ranked set;
**Remix** cycles through those available. Structure is shared, and locked roles keep their paint.
Source hues may offer just one set when source colours must be reused. A plate with only one
available set disables Remix. Each generated plate is named from its colours.

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

The shelf uses browser storage across reloads. Use **Export shelf** / **Import shelf** JSON for
backups and transfers between browsers or machines. Storage behavior varies by browser, especially
for local `file://` pages, and clearing browser data can remove saved plates. If storage refuses a
save, the app warns you; export a copy of any work you want to retain.

Shelf import is additive and offers Undo. Valid exact saved revisions are deduplicated, so a human-sealed
or source-changed revision is not discarded merely because its colours match an unsealed plate. Imported
IDs that collide are remapped locally, keeping **Update** and A/B attached to the right card. At the
40-plate limit, existing local plates are never evicted; extra imported entries are reported as held
back and remain in the import file.

Shelf import and browser-storage restore share validation: each entry needs all eight unique roles,
valid colour channels, consistent hex/RGB values, and correctly typed, bounded metadata. Malformed
entries are skipped with a report, while valid entries remain usable. Shelf file imports are limited
to 8 MiB and at most 400 incoming entries; the saved shelf still holds at most 40 plates. Deeply nested or
oversized metadata is rejected. A failed restore does not automatically overwrite the stored data;
export or preserve a copy before replacing a damaged shelf.

Loading a JSON or shelf plate keeps its recorded source set separate from whatever image canvases are
currently open. Re-exporting therefore preserves the imported provenance instead of relabelling the
plate with unrelated workspace stills. Picking from an open still adds that still to the plate record;
forging again deliberately resets the record to the stills used for that extraction.

A standalone palette round-trip also retains its creation time, original generator/extraction identity,
tags/family, extraction parameters, median luminance, and notes. Token IDs, jobs, derived flags, locks,
and origins survive as well, so merely opening and re-saving a plate cannot break its matching human
seal. Missing older colour-policy metadata is not inserted simply by opening or re-saving the file.
An actual **Forge** resets the generator to the current algorithm. Colour-derived facets and
signatures are rebuilt from the normalized eight-role plate rather than carrying stale computed values.

**Import** loads a JSON plate back in, and a `.json` dropped on the still zone does the same. Import is
tolerant on input and strict on output: keyed or positional, `hex` or `rgb`, missing slots filled with
defaults and counted in the notes. It never guesses positionally when keys are present. This single-plate
compatibility path is separate from strict shelf validation; size, metadata type, and complexity limits
still apply. Files without any usable colours are rejected.

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
```

### What a plate records about itself

| Field | Holds |
| --- | --- |
| `schema` | The contract version. Bump = a breaking file-shape change; optional metadata remains backward-compatible. |
| `generator.version` | The application that wrote it. |
| `generator.extraction` | The **algorithm** version. Moves only when extraction output changes, so you can tell a plate re-forged by new code from one merely re-saved. |
| `sources[]` | Per still: original filename, byte count, `hash` (sha256, or a labelled `fnv1a64` fallback where `crypto.subtle` is unavailable), `natural` and `working` dimensions, and the `transforms` chain between them. |
| `params` | Sample budget, signal floor, found-ink count, polarity mode, colour policy (`readable-ui` or `source-hues`), variant index. Older records can omit colour policy. |
| `facets` | Derived, never typed — polarity, dominant hue, accent hues, hue spread, chroma range, temperature, derived/picked slot counts, min contrast, panel separation. |
| `signature` | Similarity vector: per-role OKLab, a 12-bin area-weighted hue histogram, per-role lightness. |
| `review` | Optional versioned human-review state, including the proposal delta, advisory assessment, and up to 20 superseded seals as recorded history. New seals use scope `palette-and-source-identities`; legacy `palette-content` seals remain verifiable but are labeled content-only. Only a complete, matching supported seal authorizes agent use. |

The hash is always **labelled with its algorithm**. A fallback is never silently substituted for sha256.

### Per-colour origin

Every token says where it came from, and no colour is unattributed:

- **`structure`** — the luminance quantile it came from, whether chroma was capped, and every repair
  applied (`{op, against, target, from, to}`), so a lifted `ink` shows both its original and final hex.
- **`signal`** — hue bin, band, frame area, salience, and **which stills** contributed to that bin.
- **`derived`** — the generation method and reason. Readable UI can use a role's ideal hue; Source hues
  uses `source-tonal-reuse` for generated tones from an observed hue or neutral source paint.
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
picking, using recorded source colour, removing an ink, or changing the source set marks it **human-edited**. The review control shows
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

### Saving a vault

Vault exports use ZIP files so they work in both the local Go app and standalone HTML mode.

A one-plate **Bundle** deliberately contains **no `index.json`**. Unzipping it into a vault root would
otherwise overwrite a real index with a single-entry one. The index is only ever written over a whole
vault:

- **Export vault** — every plate on the shelf plus the live one, as one tree with a correct index.
- **Rebuild index** — choose a vault folder; the app reads the `palette.json` files in its subfolders
  and downloads a fresh `index.json`. Move that download into the vault root to replace its old index.
  The CLI command `go run . rebuild-index <vault-folder>` writes the index directly into that folder.
  Both rebuild from the palette files without depending on saved browser state.

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
| **Colour policy** | Readable UI separates semantic accents; Source hues favours observed hue families and can reuse tonal paint. Both retain eight roles and check readability. |
| **Signal floor** | Minimum chroma to count as signal in Readable UI. Source hues also considers weaker observed colours. |
| **Found inks** | How many non-role colours to keep alongside the core eight. |
| **Quality** | Pixels sampled per still (45k / 140k / 400k). Samples are strided across the working image; the browser caps its long edge at 1400 pixels before extraction. |
| **Remix** | Cycles to the next plate this still supports. |
| **Bundle** | One plate as a vault bundle — palette.json, sources, preview, tokens. No index by design. |
| **Export vault** | Every plate on the shelf plus the live one, as one tree with a real index.json. |
| **Rebuild index** | Choose a vault folder; downloads a regenerated index.json. Move it into the vault root. |
| **Save / Update / Import** | Save creates a shelf entry; Update revises the loaded shelf plate without changing its identity or A/B pins; Import loads a JSON plate (or drop the `.json` on the still zone). |
| **A / B** | On a shelf card, pins that plate to one side of Compare — and to one side of a theme pair. |
| **Pair A + B** | Makes the two compared plates a light/dark theme. |
| **Derive counterpart** | Builds the opposite polarity of the current plate from the cached histogram. |
| **why / Use source colour** | Inspect before/current swatches and repairs; restore available recorded source paint to a core role, lock it, and mark a human edit. Undo is offered. |
| **pick** | Arms that slot. The next click on the still writes that pixel into the role and locks it. |
| **lock** | Holds a slot through a re-forge, a remix, and a polarity flip. Lock one or two, then Forge again to re-roll only the rest. |
| **Try the plate on this app** | Repaints this tool in the extracted plate. Reversible — the checkbox puts it back. |

Click the still with nothing armed to pin a **found ink**. Use its remove button to drop it.
`Esc` cancels an armed slot.

A hand-pinned colour is law: it is never auto-corrected for contrast, even below target. The slot
badge turns red and the notes say so, but the pixel you chose is the pixel you keep.

Scene Palette 2.1.0 records extraction version 5 for fresh output. It adds the colour policies,
bounded source-distance refinement, and final accent distinction checks to the joint ground/panel
readability repair. Schema 1 is unchanged. Valid older palettes retain their recorded extraction
version, metadata, and matching review seals until deliberately edited or re-forged.

For keyboard picking, focus the still, move the sample with the arrow keys (`Shift` moves ten pixels),
and press `Enter` to apply it. Clear, still removal, found-ink removal, and shelf removal offer a
single-step **Undo** action.

## Command-line reference

Run these commands from the repository root with Go installed. In a portable Windows package,
replace `go run .` with `.\scene-palette.exe`.

```powershell
go run . forge -out palette.json -name "Night bay" "C:\path\to\image.png"
go run . forge -bundle night-bay.zip -polarity dark "C:\path\to\image.png"
go run . forge -color-policy source-hues -out source-palette.json "C:\path\to\image.png"
go run . bundle -out approved.zip palette.json "C:\path\to\image.png"
go run . rebuild-index "C:\path\to\palette-vault"
```

`forge` accepts one to eight PNG, JPEG, or GIF files and pools them with equal weight. Browser defaults
are preserved: a 140,000-pixel budget per still, `.085` signal floor, six found inks, auto polarity,
Readable UI, and the first variant. Use `-color-policy readable-ui` (default) or
`-color-policy source-hues` to select the policy. Use `-polarity dark|light`, `-variant 2`, `-found`,
`-chroma-min`, or `-budget` for the other controls. JSON goes to stdout by default;
`-out palette.json` writes a file. Use `-bundle plate.zip` instead to include the original source bytes, generated preview, CSS, Godot,
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

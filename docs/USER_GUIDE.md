# Scene Palette user guide

On an empty workspace, **Try a sample** generates a palette from **Blue lake**.
The three sample thumbnails stay available beside image upload so you can switch to **Neon city**
or **Amber dusk**, or choose your own images. Samples replace the current workspace and run the same
image-analysis and export workflow as uploads; Clear/Undo can restore prior work.
The text-free sample artwork is bundled inside the HTML and works offline.
Blue lake starts in Light mode; Neon city and Amber dusk start in Dark. These are
normal generation settings, and you can change the background mode or colour style afterward.
Selecting a sample sets its palette name; you can rename it afterward. Ordinary edits preserve
the current name. Imported and reopened palettes keep their recorded names.
**Preview on this app** starts enabled, so generating a palette also previews it
on the app itself. Uncheck it to return to the standard app colours.

Start with the [README](../README.md#quick-start--no-installation) for setup and your first palette.
This guide explains the controls, file formats, and advanced workflows.

## Using a phone or narrow window

At widths up to 1000 CSS pixels, image selection, the three samples, and **Generate palette**
come first. Uploaded images wait for **Generate palette**; samples generate immediately.
After generation succeeds, keyboard focus and the view move to the palette. The eight
role cards keep their role labels, editable names and purposes, HEX copy buttons,
**Why**, **Pick**, and **Lock** controls.

The compact source thumbnails scroll horizontally when several images are loaded.
**Source image · color picker** opens the full image; **Pick** opens it automatically.
After a role pick or cancellation, focus returns to that role. **Why** brings the
origin explanation into view; closing it returns to the originating control.

**Checks & human approval** shows the palette's approval status and how many checks need attention.
Expand it for the complete checks, changes since generation,
approval history, and the explicit approval action. This advanced workflow records a review seal
for tools that require human approval; editing and exporting do not require it.

Immediately below the role cards, **Export** contains the existing formats, Copy,
and Download. **Palette tools** contains name, polarity, Save, Import, Bundle ZIP, Clear,
and Remix. **Extraction settings** contains Color style, Extracted colors, Color intensity,
Quality, app preview, and diagnostics. Before generation, these last two sections sit
below **Generate palette**. Opening sections or resizing the window does not edit the palette,
invalidate a seal, or change saved data. Desktop keeps the expanded workflow.

Copy reports success only when the browser accepts the operation. If both clipboard
methods are blocked, select the text in Export and use the browser's copy action.

## Colour roles and extraction

The eight roles give colours a purpose in a UI. Friendly labels explain those purposes;
the technical keys remain unchanged in saved files, CSS, and other exports:

| Visible role | Technical key | Purpose |
| --- | --- | --- |
| Background | `ground` | Main background. |
| Surface | `panel` | Raised or grouped surfaces, such as cards. |
| Primary text | `ink` | Main text. |
| Secondary text | `muted` | Quieter text and details. |
| Information | `observed` | Informational accent and focus states. |
| Attention | `attention` | Items needing attention. |
| Error | `rupture` | Errors and warnings. |
| Human action | `gold` | Human approval actions in the advanced review workflow. |

**Structure** (`ground`, `panel`, `ink`, `muted`) comes from luminance quantiles over the whole image, so the
palette keeps the image's own cast, such as the warm brown surfaces of a softly lit alley.
Initial chroma is capped per slot, keeping structural colours subdued before readability repair.

**Background mode (polarity) is read from the image.** A night scene usually produces a dark background;
a bright daytime scene produces a light palette with dark text. Forcing the opposite
mode can require stronger surface and text-colour adjustments. The **Auto / Dark / Light** control
overrides the guess: use it when a dusk or interior image sits near the threshold,
or when you want a dark UI from a daytime reference. Switching modes rebuilds
from the cached histogram, so it is instant and never re-reads the image.

**Accents** (`observed`, `attention`, `rupture`, `gold`) are binned by hue in OKLCh and weighted
`sqrt(area) · chroma²`. This helps a small bright detail survive beside a large dull area.
Candidates are repaired to 3:1 on both surfaces before selection. Role assignment evaluates all
24 permutations against the role hues; the selected colour policy controls how missing paint is supplied.

**Readability repair checks both surfaces together.** Generated `ink` targets 4.5:1 and `muted` plus
the four accents target 3:1. A failing repair searches OKLCh lightness, neutral grays, then feasible
8-bit RGB colours. Generated surfaces adjust when they cannot support shared primary text. Locked colours
stay exact, and incompatible locked surfaces produce a warning. Repairs are recorded in origins.

### Color style

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
repair may still move an accent farther away. Extracted colours retain the original image colours.

### Inspect and restore source colours

The colour diagnostics list derived accents, adjustments above 0.10 OKLab distance, and final accent
pairs below 0.07. Open a role's **Why** details to see **Before adjustment** and **Current** swatches,
their distance, and the recorded repairs. These are available when the token has a valid recorded
source colour; imported files without that provenance cannot provide a source comparison.

**Use source colour** restores that recorded paint to a core role and locks it. This is a deliberate
human edit: it can fail contrast or reduce accent distinction, changes the human-review state, and
invalidates a current seal. The toast's **Undo** restores the prior workspace, including its review
state. Unlock the role to let future extraction adapt it again. Loading an older palette alone does
not apply these changes or run the new colour policy.

## Variants and remix

A busy frame can support several four-colour sets. The **Variants** strip applies a ranked set;
**Remix** cycles through those available. Structure is shared, and locked roles keep their paint.
Source hues may offer just one set when source colours must be reused. A palette with only one
available set disables Remix. Individual colour names are generated from their colours;
the palette name remains editable and is preserved during normal editing.

## Readability

The **Readability** matrix checks every colour against *both* the background and surface, plus the
label colour each filled accent would paint with. `AA` is 4.5:1 (body copy); `pass` is 3:1, the floor
for large text and UI components. A `low` can identify a pinned colour or a generated repair that
failed on one surface. Pinned colours are never auto-corrected.

## Preview surfaces

The preview has four tabs, and **Compare follows whichever one is selected** — so you can put two
palettes side by side on the surface you actually care about.

| Surface | What it stresses |
| --- | --- |
| **UI preview** | Status colours, activity messages, and filled buttons on the background. |
| **Forms** | Surface separation, borders, secondary labels, the `observed` focus ring, the `rupture` error state, disabled text. |
| **Table** | Zebra rows, header rules, right-aligned numerics, and all three status accents at once. |
| **Approval** | Human approval status and the `gold` role reserved for actions a person must take. This preview cannot approve a palette. |

Every surface is markup on the `--p-*` vars alone; none of them names a colour. Label colours on filled
accents are computed per palette (black or white, whichever reads), for all four accents.

## Saved palettes, import, compare

**Save** adds the current palette to **Saved palettes**; loading that card makes it the active saved palette and
changes the action to **Update**. Update revises that entry in place, so its identity and A/B pins
survive, and it records review or seal changes even when the colours did not move. An Undo action is
offered after each update. If a theme pair uses that revision, the pair is cleared rather than leaving
a stale export behind.

Saved palettes use browser storage across reloads. Use **Export collection** / **Import collection** JSON for
backups and transfers between browsers or machines. Storage behavior varies by browser, especially
for local `file://` pages, and clearing browser data can remove saved palettes. If storage refuses a
save, the app warns you; export a copy of any work you want to retain.

Collection import is additive and offers Undo. Valid exact saved revisions are deduplicated, so a human-sealed
or source-changed revision is not discarded merely because its colours match an unsealed palette. Imported
IDs that collide are remapped locally, keeping **Update** and A/B attached to the right card. At the
40-palette limit, existing local palettes are never evicted; extra imported entries are reported as held
back and remain in the import file.

Collection import and browser-storage restore share validation: each entry needs all eight unique roles,
valid colour channels, consistent hex/RGB values, and correctly typed, bounded metadata. Malformed
entries are skipped with a report, while valid entries remain usable. Collection file imports are limited
to 8 MiB and at most 400 incoming entries; the saved collection still holds at most 40 palettes. Deeply nested or
oversized metadata is rejected. A failed restore does not automatically overwrite the stored data;
export or preserve a copy before replacing a damaged collection.

Loading a JSON or saved palette keeps its recorded source set separate from whatever image canvases are
currently open. Re-exporting therefore preserves the imported provenance instead of relabelling the
palette with unrelated workspace images. Picking from an open image adds that image to the palette record;
generating again deliberately resets the record to the images used for that extraction.

A standalone palette round-trip also retains its creation time, original generator/extraction identity,
tags/family, extraction parameters, median luminance, and notes. Token IDs, jobs, derived flags, locks,
and origins survive as well, so merely opening and re-saving a palette cannot break its matching human
seal. Missing older colour-policy metadata is not inserted simply by opening or re-saving the file.
Using **Generate palette** resets the generator to the current algorithm. Colour-derived facets and
signatures are rebuilt from the normalized eight-role palette rather than carrying stale computed values.

**Import** loads a JSON palette back in, and a `.json` dropped on the image upload area does the same. Import is
tolerant on input and strict on output: keyed or positional, `hex` or `rgb`, missing slots filled with
defaults and counted in the notes. It never guesses positionally when keys are present. This single-palette
compatibility path is separate from strict collection validation; size, metadata type, and complexity limits
still apply. Files without any usable colours are rejected.

**Compare** puts two saved palettes side by side as UI previews with a role-by-role diff and
ΔE per changed slot. Both previews and the live one come from one template, so they cannot drift.

## Themes — two palettes, one file

A bright daytime image and a dark night image work better as separate palettes: combining them
puts median luminance in the middle and can produce a muddy background. A theme pair keeps
both palettes in one export.

Generate a palette from each image on its own, **Save** both, pick them as **A** and **B**, then **Pair A + B**. Sides
are ordered by their own polarity, not by which you picked first. If you only have one image,
**Generate light/dark counterpart** builds the opposite polarity from the cached histogram — and the report will
tell you when that produced a mid-toned "dark" side rather than a real one.

The report checks each side independently, flags an accent whose hue shifts more than 40 degrees
between sides (it will read as a different colour), and trims extracted colours to the count both sides
share — a CSS variable that exists on one side only would vanish on switch.

CSS comes out as the full cascade:

```css
:root { color-scheme: dark light; --ground: #070505; ... }
:root[data-theme="light"] { --ground: #E7E5CC; ... }
@media (prefers-color-scheme: light) {
  :root:not([data-theme="dark"]) { --ground: #E7E5CC; ... }
}
```

Explicit choice wins, system preference applies otherwise, and asking for dark by name still gets
dark. Godot exports both sides as dictionaries behind `plate(light: bool)`. Two palettes of the same
polarity pair as base/alt instead, and say so.

## The vault bundle

A vault is a folder of palette bundles, with an `index.json` file for searching the collection.
Use **Bundle ZIP** for one palette or **Export vault ZIP** for the saved collection plus your current palette.

**The folder is the authority. `index.json` is a cache.** Every field in the index is copied from, or
recomputed from, a `palette.json`, so deleting the index loses nothing — `buildIndex()` reconstructs it
from the folders alone. Nothing in the bundle layer reads application state, which is what makes that
true: a bundle written today can be re-indexed by something that has only the folder and none of this
code.

**Bundle ZIP** writes one palette as a zip:

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

### What a palette records about itself

| Field | Holds |
| --- | --- |
| `schema` | The contract version. Bump = a breaking file-shape change; optional metadata remains backward-compatible. |
| `generator.version` | The application that wrote it. |
| `generator.extraction` | The **algorithm** version. Moves only when extraction output changes, so you can tell a palette regenerated by new code from one merely re-saved. |
| `sources[]` | Per image: original filename, byte count, `hash` (sha256, or a labelled `fnv1a64` fallback where `crypto.subtle` is unavailable), `natural` and `working` dimensions, and the `transforms` chain between them. |
| `params` | Sample budget, signal floor, extracted-colour count, polarity mode, colour policy (`readable-ui` or `source-hues`), variant index. Older records can omit colour policy. |
| `facets` | Derived, never typed — polarity, dominant hue, accent hues, hue spread, chroma range, temperature, derived/picked slot counts, min contrast, panel separation. |
| `signature` | Similarity vector: per-role OKLab, a 12-bin area-weighted hue histogram, per-role lightness. |
| `review` | Optional versioned human-review state, including the proposal delta, advisory assessment, and up to 20 superseded seals as recorded history. New seals use scope `palette-and-source-identities`; legacy `palette-content` seals remain verifiable but are labeled content-only. Only a complete, matching supported seal authorizes agent use. |

The hash is always **labelled with its algorithm**. A fallback is never silently substituted for sha256.

### Per-colour origin

Every token says where it came from, and no colour is unattributed:

- **`structure`** — the luminance quantile it came from, whether chroma was capped, and every repair
  applied (`{op, against, target, from, to}`), so a lifted `ink` shows both its original and final hex.
- **`signal`** — hue bin, band, frame area, salience, and **which images** contributed to that bin.
- **`derived`** — the generation method and reason. Readable UI can use a role's ideal hue; Source hues
  uses `source-tonal-reuse` for generated tones from an observed hue or neutral source paint.
- **`picked`** — the image's filename *and content hash*, the pixel in **both** coordinate spaces
  (`working` as clicked, `natural` mapped back to the original image), the 3×3 sampling method, and a
  timestamp. Its `repairs` array is always empty: a hand-picked colour stays exact and is never
  auto-corrected.
- **`imported`** — carried through verbatim when present in the source file; otherwise marked as having
  no recorded origin rather than being invented.

Origins survive a JSON round trip, so re-importing a palette preserves its source history.

### Human approval for agent use

Approval is optional for ordinary editing, saving, and exporting. It is for developer tools that
require a person to approve a palette before an automated agent can use it. The saved approval
record is called a **review seal**; it records a decision, not a security guarantee or proof of identity.

**Generate palette** and **Remix** always create a **proposal** and snapshot its compact decision baseline: palette identity,
role values and wording, locks, extracted colours, and source identities. Renaming, repurposing, locking,
picking, using recorded source colour, removing an extracted colour, or changing the source set marks it **human-edited**. The approval control shows
the exact proposal-to-current before/after delta. Either state is useful for exploration and export, but
neither is agent-eligible. **Approve for agent use** records that human decision, its delta, a timestamp, and a
deterministic fingerprint over the palette's semantic content plus normalized source filename/hash
identities. Any later content or source-identity edit invalidates it and moves the superseded decision
into a bounded history of the 20 most recent seals, with the invalidation time and reason.

**Checks & human approval** puts four deterministic checks beside that decision: readability on both surfaces,
native token origins, source hashes, and any derived or fallback core roles. An attention result is not
a confidence score or an automatic veto; it tells the person what needs judgment, and the seal remains
the person's decision. The browser records the assessment in `review.assessment` while `check -json`
recomputes it from the current file.

A palette with no `review` object—including output from `forge` and older saved collections—is unsealed by
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

A one-palette **Bundle ZIP** deliberately contains **no `index.json`**. Unzipping it into a vault root would
otherwise overwrite a real index with a single-entry one. The index is only ever written over a whole
vault:

- **Export vault ZIP** — every saved palette plus the current one, as one tree with a correct index.
- **Rebuild index** — choose a vault folder; the app reads the `palette.json` files in its subfolders
  and downloads a fresh `index.json`. Move that download into the vault root to replace its old index.
  The CLI command `go run . rebuild-index <vault-folder>` writes the index directly into that folder.
  Both rebuild from the palette files without depending on saved browser state.

Saved and imported entries may not have their original source bytes — browser storage could not hold them
and a standalone JSON file does not carry them. Bundle and vault exports keep those source records but
mark them `"retained": false`, with `path` set to `null` rather than pointing at a file that is not in
the zip. Any matching source image that is actually open can be reattached by its content hash.

If `crypto.subtle` is unavailable the bundle falls back to a **labelled** `fnv1a64`. The sidebar
diagnostics show which algorithm your sources actually got, under `source hash`.

## Controls

| Control | What it does |
| --- | --- |
| **Auto / Dark / Light** | Background mode (polarity). Auto reads it from the image; the other two override. Rebuilds instantly. |
| **Color style** | Readable UI separates semantic accents; Source hues favours observed hue families and can reuse tonal paint. Both retain eight roles and check readability. |
| **Color intensity** | Minimum vividness for accent candidates in Readable UI; lower values include less vivid colours. Source hues also considers weaker observed colours. This sets a chroma threshold, not the saturation of the finished palette. |
| **Extracted colors** | How many non-role colours to keep alongside the core eight. |
| **Quality** | Pixels sampled per image (45k / 140k / 400k). Samples are strided across the working image; the browser caps its long edge at 1400 pixels before extraction. |
| **Remix** | Cycles to the next accent combination this image supports. |
| **Bundle ZIP** | One palette as a vault bundle — palette.json, sources, preview, tokens. No index by design. |
| **Export vault ZIP** | Every saved palette plus the current one, as one tree with a real index.json. |
| **Rebuild index** | Choose a vault folder; downloads a regenerated index.json. Move it into the vault root. |
| **Save / Update / Import** | Save creates a saved palette; Update revises the loaded palette without changing its identity or A/B pins; Import loads a JSON palette (or drop the `.json` on the image upload area). |
| **A / B** | On a saved palette card, pins that palette to one side of Compare — and to one side of a theme pair. |
| **Pair A + B** | Makes the two compared palettes a light/dark theme. |
| **Generate light/dark counterpart** | Builds the opposite polarity of the current palette from the cached histogram. |
| **Why / Use source colour** | Inspect before/current swatches and repairs; restore available recorded source paint to a core role, lock it, and mark a human edit. Undo is offered. |
| **Pick** | Selects the role for image picking. The next click on the image writes that pixel colour into the role and locks it. |
| **Lock** | Keeps a role's colour through generation, remixing, and background-mode changes. Lock one or two, then generate again to change only the rest. |
| **Preview on this app** | Repaints this tool in the extracted palette. Reversible — the checkbox puts it back. |

Click the image without selecting a role to add an **extracted colour**. Use its remove button to drop it.
`Esc` cancels role picking.

A hand-picked colour stays exact: it is never auto-corrected for contrast, even below target. The role
badge turns red and the notes say so, but the pixel you chose is the pixel you keep.

Scene Palette 2.1.0 records extraction version 5 for fresh output. It adds the colour policies,
bounded source-distance refinement, and final accent distinction checks to the joint ground/panel
readability repair. Schema 1 is unchanged. Valid older palettes retain their recorded extraction
version, metadata, and matching review seals until deliberately edited or regenerated.

For keyboard picking, focus the image, move the sample with the arrow keys (`Shift` moves ten pixels),
and press `Enter` to apply it. Clear, image removal, extracted-colour removal, and saved-palette removal offer a
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
are preserved: a 140,000-pixel budget per image, `.085` signal floor, six extracted colours, auto polarity,
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
declared by the browser seal, and exits with an error unless the palette has a complete match. New seals
bind the semantic palette content **and** the normalized source filename/hash identities. Legacy
`palette-content` seals remain readable and are explicitly reported as content-only:

```powershell
go run . check palette.json
go run . check -json palette.json
go run . check -allow-unsealed palette.json  # inspection only; never grants eligibility
```

`-json` returns the palette name, polarity, role colors, contract state, verified seal `scope`, recorded
and computed fingerprints, the recorded proposal-decision counts when present, an independently
recomputed four-check review assessment, and a single `agentEligible` decision. Missing review metadata,
unsupported review versions or scopes, invalid timestamps, changed palette content, and changed
source identities under the new scope all fail closed. `-allow-unsealed` changes only the process exit
result so a person can inspect a valid draft; the report still says `agentEligible: false`.

# Go migration boundary

The local Go backend is integrated with the browser. Go owns extraction,
lock-aware variant assembly, ZIP bundles, vault export/indexing, and independent
seal verification. JavaScript owns the interactive editor, shelf, human review,
previews, and standalone `file://` fallback. This is the completed backend
boundary; the UI does not require a rewrite or a frontend build step.

## Frozen contract

- `palette.json` schema: `1`
- Scene Palette application version: `2.1.0`
- Extraction algorithm version: `5`
- Core role order: `ground`, `panel`, `ink`, `muted`, `observed`, `attention`, `rupture`, `gold`
- Every durable core token has a six-digit hex colour and an origin object.
- A durable palette includes `facets` and `signature`; browser imports may remain more tolerant.

Fixtures under `testdata/vault` freeze the JSON boundary and the rebuildable
`index.json` shape. The ten images under `testdata/reference-images` are deterministic,
programmatically generated patterns. Their palette baselines live under
`testdata/reference-palettes`; the manifest records their generation and hashes.
These patterns exercise extraction behavior without distributing private images.
Their familiar labels are fixture identifiers, not claims that they reproduce the
photographs or artwork used during earlier local development. The public repository
has new history and new image-derived baselines; private PNGs and their goldens are
not included.
`testdata/histogram-signal.golden.json` freezes the raw bins and ranked signal
candidates produced from those pixels, including one lower-budget stride probe.
It also freezes the repaired structural roles for auto, forced-dark, and
forced-light polarity on every reference image, plus filtered and repaired
accent candidates, ranked four-colour sets, derived top-ups, and semantic role
assignments. It also freezes the fully assembled unlocked core and found inks,
including names, jobs, origins, repairs, and notes for every returned fixture variant.
Synthetic fixture checks target deterministic regression and Go/HTML parity;
they do not establish representative image quality. See `QUALITY_REPORT.md` for the
separate historical evaluation and its limits.

## Implemented commands and verification

- `go run . serve` serves the existing embedded HTML and JavaScript at
  `http://127.0.0.1:8765/`; `-open` launches that URL in the default browser.
- `Run Scene Palette.cmd` prefers an adjacent Windows executable, then uses
  `go run . serve -open` in a source checkout, and otherwise opens `index.html`.
  Running the executable with no arguments opens the browser; explicit `serve`
  remains headless unless `-open` is supplied. The console owns the server lifetime.
- `scripts/package-windows.ps1` builds an unsigned, portable Windows x64 ZIP
  with no recipient runtime dependency. It retains the exact standalone HTML,
  records source revision/tree hashes and dirty state, checks for concurrent
  source changes, and emits package checksums under ignored `bin/releases/`.
  Optional `-RequireClean`, `-ExpectedGoVersion`, and `-ReleaseTag` checks validate
  a local release build without publishing or creating tags. See [Contributing](../CONTRIBUTING.md).
- `scripts/package-installer.ps1` validates a clean portable package and compiles
  its exact payload with NSIS. The per-user installer adds shortcuts and HKCU
  uninstall registration; upgrades and uninstall leave browser storage and user
  exports alone. It does not change the executable's browser/console lifecycle.
- `go run . rebuild-index <vault-folder>` validates every `palette.json` and
  writes a deterministic `index.json`.
- `go run . forge [options] <image> [image ...]` reads up to eight PNG, JPEG,
  or GIF files, records their SHA-256 hashes and decode metadata, pools them
  with the browser defaults, and emits one complete `palette.json`. Output is
  stdout unless `-out` is supplied; the CLI variant flag is one-based.
  `-color-policy readable-ui|source-hues` selects the extraction policy;
  omitting it keeps the Readable UI default.
- `go run . forge -bundle plate.zip <image> ...` writes the full portable
  bundle, including the unmodified originals and a labeled 960×250 preview.
- `go run . bundle [-out plate.zip] <palette.json> [source-image ...]` packages
  an existing document without losing human review, history, or extensions.
  Supplied source bytes must match a recorded SHA-256 or browser FNV hash.
- `go run . check [-json] <palette.json>` independently verifies the contract
  and current seal; a draft remains ineligible even with `-allow-unsealed`.
- Invalid palette folders are reported and skipped when at least one valid
  palette remains.
- `node scripts/verify-js-index.mjs` runs the actual browser `buildIndex()`
  against the same golden index used by the Go tests.
- `node scripts/reference-extract.mjs --check` extracts the current functions
  directly from `index.html`, feeds them the committed PNG pixels, and compares
  the ten durable palette baselines. It requires the development-only `sharp`
  package to decode PNGs; the application itself remains dependency-free. The
  harness verifies that extraction produces an unsealed proposal and compares
  extraction-owned fields with the versioned pre-review fixtures. Review
  metadata has a separate contract/round-trip verification gate.
- `node scripts/color-math-fixture.mjs --check` freezes representative outputs
  from the JavaScript colour functions. `internal/colorx` implements that pure
  math in Go, with focused parity tests against the frozen outputs.
- `internal/extract` starts at a decoded `image.Image` and matches the browser's
  full-resolution stride, luminance and hue-band histograms, equal still
  weighting, candidate merging, salience ordering, and distinct top pool. It
  also matches luminance quantiles, automatic polarity, chroma caps, panel
  separation, contrast repairs, and their provenance records. The accent stage
  matches candidate filtering and repair, deduplication, four-colour set
  scoring, derived top-ups, and semantic role assignment across every polarity.
  Fresh plate assembly then maps every set into the eight named semantic tokens,
  carries structure/signal/derived origins forward, selects found inks, and
  produces the same variant notes as the browser. Both engines also implement
  Source hues tonal reuse, bounded source-distance refinement, and final accent
  distinction checks. Observed replacements carry their own provenance.
- `internal/palette.BuildSnapshot` packs one fresh variant into the complete
  `palette.json` shape. It preserves caller-supplied timestamps and source
  metadata, derives bundle source paths, rounds token colour and contrast
  metadata, flattens provenance and repairs, and reconstructs the same facets
  and similarity signature as the browser. Tests compare all ten durable pattern
  palettes as complete JSON values and validate them with the strict contract reader.
- `internal/forge.Build` composes those stages from original image paths. With
  an explicit timestamp it is deterministic. Tests compare the ten single-pattern
  outputs with browser reference documents, allowing only the deliberately truthful
  decode-transform label (`go-image/png` instead of the JavaScript harness).
  A focused two-image case verifies equal-weight pooling and source ordering.
- `internal/live` extends the fresh path with exact browser lock/pin semantics,
  including reserved names, found-ink filtering against the final pinned core,
  and retained pins above the requested found count. Tests compare this with
  actual JavaScript functions as well as the ten complete fixture documents.
  Accent refinement runs after the effective locked surfaces and roles are
  assembled, so it evaluates the colours that the user receives.
- `internal/bundle` preserves the complete durable JSON, verifies attachments,
  renders previews, and emits deterministic ZIPs. Single bundles have no index;
  whole vaults have unique folders and an index equal to a fresh disk rebuild.
- `scripts/browser-e2e.mjs` drives an isolated headless browser against a running
  Go server and the standalone file. It compares extraction outputs and tests
  picking, locks, variants, polarity, counterparts, shelf persistence, review
  seals, validated shelf imports/restores, source-colour actions, policy changes,
  downloads, archives, and asynchronous failure/cancellation.
  It requires development-only Playwright; Windows uses installed Edge.
  Reports and screenshots are written only below ignored `bin/e2e/`.
- `scripts/parity-corpus.mjs` compares complete extraction snapshots with the
  standalone HTML in fresh browser profiles. Only timestamps and human-review
  state are excluded. It covers all variants, three polarity modes, sampling
  budgets, signal floors, found-ink counts, and pooled images. The standalone
  context is offline. Optional manifests and `--html` support new image corpora
  and the exact packaged HTML; reports include strict diffs and separate quality
  diagnostics based on unrounded RGB values. Run each policy separately with
  `--color-policy readable-ui` or `--color-policy source-hues`.
  `--require-readable` gates generated, unlocked core ink on both surfaces in both
  engines. `--require-distinct` additionally gates final generated, unlocked accent
  pairs in Readable UI; Source hues reports close pairs as a policy tradeoff.
  Manual/locked exclusions are reported. Outputs stay under ignored `bin/`.
- `npm ci` installs pinned development-only Playwright and sharp dependencies.
  `npm run check` runs the JavaScript/fixture checks; `npm run check:browser` builds
  an owned loopback server, runs browser workflows and both policy corpora, and
  cleans up on success or failure with bounded startup and child-process timeouts.
  CI installs Node before Go tests and runs Go test/vet, browser parity, and
  full-history plus source-only Gitleaks scans. See [Contributing](../CONTRIBUTING.md)
  for browser installation and the full local command sequence.

## Local API

The frontend discovers version 1 through `GET /api/info`. The remaining
endpoints accept POST requests, require a loopback Host and same-origin browser
requests, and keep at most two operations active. There is no CORS access,
server-side shelf/session, arbitrary filesystem path API, or internet service.

- `/api/extract`: multipart `request` JSON plus one to eight `image` PNGs for
  Forge; JSON with the returned histogram for rebuilds. Working PNGs are capped
  at 1400×1400 and the request at 96 MiB. The engine returns every variant for
  the requested polarity with the current locked roles and pins applied.
  `request.params.colorPolicy` accepts `readable-ui` or `source-hues`; an omitted
  value defaults to Readable UI, and unsupported values are rejected.
- `/api/bundle`: multipart `palette`, optional `preview`, and repeated `source`
  bytes. Returns a ZIP. Source paths are normalized within its palette folder.
- `/api/vault`: multipart `palettes` JSON array with `preview-N` and `source-N`
  attachments for up to 41 plates (40 shelf entries plus the current plate).
- `/api/index`: JSON array of `{folder,palette}`; returns `{index,skipped}` in
  canonical name/folder order. Valid folders remain usable when others fail.
- `/api/check`: a durable palette as JSON; returns `contractValid`,
  `agentEligible`, and the independently recomputed `review` verification.

Archive requests are capped at 256 MiB, individual binary parts at 64 MiB, and
JSON metadata at 8 MiB. Bundle output is also capped at 256 MiB including ZIP
overhead, with a shared budget across vault entries and at most 256 historical
source records per plate. Repeated references to the same source count toward
the output budget each time. Nothing uploaded to these endpoints is written to disk
by the server. The browser receives a download; CLI commands write explicit
local output files and protect their input files from replacement.

## Browser responsibilities and parity

Image decoding stays in the browser so EXIF orientation, colour profiles, alpha
and the 1400-pixel working-copy transform keep their existing behavior. Go reads
lossless PNGs of those canvases. The headless CLI decodes originals directly;
its different decode/downscale boundary is recorded in source transforms.

Editing, pixel picking, undo, shelf storage, compare, theme pairing, live export
text, canvas previews, and human seal creation remain JavaScript UI behavior.
Go rebuilds preserve human-owned tokens exactly. The standalone page retains
the JavaScript pipeline as the parity reference and offline implementation.
Changing source stills or importing a plate invalidates the old extraction
cache. Pending Go responses are discarded after newer requests or human edits,
imports, clear, undo, and seals. Server failures preserve the displayed plate
and report the error instead of changing engines mid-operation.

Schema 1 remains unchanged; application 2.1.0 uses extraction version 5. The
joint ground/panel contrast solver still checks exact, unrounded ratios. If no
shared body ink exists, generated surfaces move toward the requested polarity
and retain their separation, with recorded provenance. Locked paint stays exact;
unlocked inks adapt to the effective surfaces or report that the locked surfaces
cannot share readable ink. Raw found inks remain source paint.

Readable UI is the default. After contrast repair and lock application, final
accent refinement tries observed replacements and bounded generated candidates
to separate close pairs. It prioritizes changing unlocked derived paint and
preserves locked roles. Remaining close pairs are reported rather than presented
as proof that separation is impossible. Observed replacements replace the old
origin; repair history describes the paint actually delivered.

For large adjustments to observed accents, a bounded lightness/chroma search
tries to reduce recorded-source distance while retaining contrast and, in
Readable UI, the existing distinction floor. It includes the current valid
colour as a candidate. The search is deterministic and local, not a global
optimum claim; subsequent distinction work can still increase source distance.

Source hues retains the fixed eight-role contract. It uses observed hue families
and generated tonal reuse when the source is sparse, with no arbitrary hue
completion or minimum-chroma fill for neutral inputs. It does not force the
Readable UI accent-distance floor. Readability can still require a hue departure
under constrained surfaces, and those roles receive an explicit warning.

The browser compares recorded source/current paint without mutating provenance.
Using source colour is an explicit human edit that locks the role, records a
restore operation, and invalidates a current review seal; Undo restores the prior
workspace. Existing valid imports, shelf entries, and seals retain their recorded
extraction identity and parameters, including absent older colour-policy fields.
No automatic re-forge runs on load; a deliberate Forge writes extraction version 5.

Shelf file imports and local-storage restore share a bounded validator for eight
unique roles, strict colour representations, and metadata types/complexity.
Invalid entries are reported and skipped without discarding valid entries, and
failed restore does not automatically rewrite stored data. The single-palette
importer retains its tolerant legacy shape handling while enforcing size and
metadata bounds. Dynamic swatches use validated hex values and text is escaped.

The historical 2026-10-26 local corpus check found a one-ULP difference in newer
Edge's native `atan2` that swapped two roles in one private image's third variant.
JavaScript uses the same reference atan/atan2 arithmetic as Go for both histogram
hues and OKLCh conversion. The original fix preserved those private golden results
and strict role-assignment comparisons, without rounding or comparison tolerances.
The public suite retains focused mathematical regressions and generates its image
baselines from the new patterns. See [Quality report](QUALITY_REPORT.md) for
versioned validation evidence and remaining image-quality limitations.

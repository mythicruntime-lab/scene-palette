# Changelog

Notable changes to Scene Palette are recorded here, newest first. Version entries
describe the source history; they do not imply a published GitHub Release or a
hosted Windows download. Add future changes under Unreleased before the next version.

## Unreleased

- Enabled **Try the plate on this app** by default; the toggle still restores the
  standard app colours when switched off.

- Added a blue-first sample experience with three locally bundled images, selectable
  thumbnails, and the same upload, generation, preview, and export pipeline in the
  standalone browser app and Go-served interface.

## 2.1.0-beta.2 — 2026-10-27

[Windows beta downloads](https://github.com/mythicruntime-lab/scene-palette/releases/tag/v2.1.0-beta.2): rebuilt installer, portable ZIP, and SHA-256 checksums from exact source `0475fdb`. Application version 2.1.0 and extraction v5 are unchanged.

- Added a small BETA indicator and GitHub bug-report link to the shared app header.

- Published the repository and existing Windows beta downloads on 2026-10-27 after
  all six CI checks passed on `379ec89`. Updated the documentation and release wording.
- Added repository topics, enabled all six required CI checks on `main`, and enabled
  private vulnerability reporting. Branch protection applies to administrators, requires
  up-to-date pull requests and resolved conversations, and blocks force pushes and deletion.
- Verified anonymous access to all three release assets and their SHA-256 hashes.

### Verification

- Full Go tests/vet, JavaScript fixtures, and all six GitHub CI checks passed on the
  release source. Portable smoke tests covered hashes, runtime-free CLI/Forge, exact
  Go/HTML parity for both colour policies, save/reload/export, and the new header.
- Installer archive integrity passed, all eight payload files matched the portable
  package, and its extracted app passed 43 served/standalone browser workflows.
- Both packages passed desktop/narrow-header and keyboard-focus checks. Published
  asset downloads matched the local SHA-256 hashes.
- Installation-lifecycle tests were not rerun for beta 2; the existing user installation
  was preserved. Installer and packaging code is unchanged from lifecycle-tested beta 1.
  Packages remain unsigned.

## 2.1.0-beta.1 — 2026-10-27

[Windows beta downloads](https://github.com/mythicruntime-lab/scene-palette/releases/tag/v2.1.0-beta.1): installer, portable ZIP, and SHA-256 checksums.
Application 2.1.0 / extraction v5; tested artifact source `73161b1`.

### Added

- A per-user Windows installer built from the verified portable package, with Start
  Menu shortcuts, an optional desktop shortcut, upgrade handling, and an uninstaller
  that preserves user exports and browser storage.
- Installer packaging verifies payload hashes and clean source identity, records
  compiler provenance, and keeps generated artifacts in ignored local output.

### Changed

- Portable-package instructions explain closing other running copies to reuse the
  same local address and browser shelf, and backing up work with Export shelf.

### Fixed

- Installer preflight now resolves destination folders before they exist, while
  retaining drive-root and junction/symlink checks.
- Desktop shortcut ownership is updated only after creation or removal succeeds.
  Upgrades retain the registered installation path and optional shortcut preference.

### Verified — 2026-10-27

- Automated checks passed for fresh installation, an older-payload upgrade fixture,
  reinstall, running-app protection, downgrade/relocation rejection, and uninstall.
  Test exports and unrelated files survived upgrades and uninstall.
- The installed application passed 43 served/standalone browser checks. The rebuilt
  portable ZIP passed checksum, launch/Forge without Go or Node, save/reload, and
  download checks. Both packages contain the same application executable.
- Five packaging checks rejected tampered files, unexpected payload entries, a wrong
  toolchain, an older source revision, and dirty source. Secret scans found no secrets.
- The maintainer manually confirmed that the installer and installed app work.
  Windows packages remain unsigned, with generated binaries kept outside Git history.

See the [Windows packaging validation](docs/QUALITY_REPORT.md#windows-packaging-validation--2026-10-27)
for the automated test scope and remaining coverage limits.

## 2.1.0 — 2026-10-26

Application 2.1.0 uses extraction v5. Schema 1 and the eight named roles remain unchanged.

### Added

- Optional **Source hues** policy in the browser, Go CLI, and local API. It reuses
  observed hue families and neutral/tonal paint while retaining all eight roles.
- Source/current swatches, colour-change measurements, and close-accent diagnostics
  in the why panel. **Use source colour** restores recorded paint, locks it as a
  human choice, and supports Undo.
- Pinned GitHub Actions workflows, reproducible development dependencies, hostile-input
  browser checks, and separate parity/readability runs for both colour policies.
- README screenshots, a generated before/after comparison, and detailed release notes.

### Fixed

- HTML injection through imported and restored shelves. Entries now receive complete,
  bounded validation, with independent safe rendering of colours and metadata.
- Hostile, missing, or duplicate found-ink IDs breaking editor controls. Imported
  found inks receive fresh internal IDs without changing saved content or seals.
- Final accent collisions after contrast repairs and lock application in Readable UI.
- Browser cube-root differences that caused Go/HTML provenance and role-assignment
  mismatches. Both engines now use matching reference arithmetic without tolerances.
- Delayed colour-policy responses overwriting a newer user choice.
- Fresh empty shelves lacking an import control.

### Changed

- A bounded source-distance search reduces avoidable colour changes while preserving
  readability. Exact manual locks remain unchanged and can produce warnings.
- Clean Windows packaging verifies the toolchain and source state, isolates external
  Go build overrides, and records safe provenance and checksums.
- Existing valid palettes retain their recorded origins, generation identity, and
  seals when opened. Explicit generation uses extraction v5.

### Verification and limits

- Local validation passed 1,488 exact Go/HTML corpus comparisons, 43 browser workflow
  checks, the full Go suite and vet, and JavaScript/fixture checks. Corpus runs had
  zero generated-readability failures; all three previous default accent collisions
  were resolved.
- Source and history secret scans found no secrets. The clean Windows review ZIP,
  built with Go 1.27.3, passed extracted-package checks without Go or Node on its PATH.
- Source hues can repeat colours, and pale source paint can still need substantial
  contrast adjustments. Automated checks do not establish complete accessibility.

See the [2.1.0 release notes](docs/RELEASE_NOTES_2.1.0.md) and
[quality report](docs/QUALITY_REPORT.md) for evidence, compatibility, and limitations.

## 2.0.0 — 2026-10-26

Initial repository baseline, using extraction v4 and schema 1.

- Standalone offline HTML app and matching Go engine, local browser server, and CLI.
- Eight-role palettes, raw found inks, multiple variants, polarity controls, exact
  picks/locks, saved shelves, comparison previews, and human-review seals.
- CSS, Godot, JSON, GPL, HEX, bundle, and vault export workflows.
- Joint ground/panel contrast repair for generated palettes.
- Portable Windows packaging with build information and checksums.
- MIT license, third-party notices, user/contributor/security documentation, and
  deterministic generated regression images in a fresh source history.

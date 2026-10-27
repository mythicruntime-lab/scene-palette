# Changelog

Notable changes to Scene Palette are recorded here, newest first. Version entries
describe the source history; they do not imply a published GitHub Release or a
hosted Windows download. Add future changes under Unreleased before the next version.

## Unreleased

### Added

- A per-user Windows installer built from the verified portable package, with Start
  Menu shortcuts, an optional desktop shortcut, upgrade handling, and an uninstaller
  that preserves user exports and browser storage.
- Installer packaging verifies payload hashes and clean source identity, records
  compiler provenance, and keeps generated artifacts in ignored local output.

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

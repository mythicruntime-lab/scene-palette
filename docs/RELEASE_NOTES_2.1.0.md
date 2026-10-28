# Scene Palette 2.1.0 Beta 2

Application 2.1.0 uses extraction v5, schema 1, and the same eight palette roles.
The [Windows beta](https://github.com/mythicruntime-lab/scene-palette/releases/tag/v2.1.0-beta.2) provides the tested installer,
portable ZIP, and `SHA256SUMS.txt`. Tag `v2.1.0-beta.2` identifies exact artifact
source commit `0475fdb`; later main-branch commits record documentation and results.
See the [changelog](../CHANGELOG.md) for version history.

## Changes

Beta 2 rebuilds the installer and portable ZIP with a small **BETA** indicator and a
**Report a bug** link in the shared app header. The application version, extraction
algorithm, saved data, and installer behavior are unchanged from beta 1.

- Shelf imports and stored shelves now pass complete validation before use. Rendering
  also escapes metadata and accepts only valid colour values. Hostile shelf colours,
  counts, and found-ink IDs cannot execute markup or break the editor. Valid old
  palettes, origins, and human seals remain compatible.
- Readable UI checks final accent distinction after contrast repairs and locks. A
  bounded search reduces avoidable movement from source colours while retaining the
  existing strict readability targets.
- The optional Source hues policy reuses source hue families and neutral/tonal paint.
  It keeps all eight roles, can produce similar accents, and reports that tradeoff.
- The why panel compares source and current paint. Use source colour restores the
  recorded RGB, locks it as a human choice, and supports Undo. Merely viewing the
  diagnostics leaves saved content and seals unchanged.
- Go and standalone HTML use matching reference colour arithmetic. Delayed policy
  responses cannot replace a newer choice. Policy settings survive JSON, shelves,
  counterpart generation, Remix, and rebuilds.
- Added pinned CI, reproducible development dependencies, README screenshots, and
  clean-build/toolchain checks for Windows packaging.

## Verification

Beta 2 passed full Go tests/vet, JavaScript fixtures, all six GitHub CI checks, and
portable smoke tests on exact source `0475fdb`. The new installer passed archive
integrity testing; its eight payload files matched the portable package, and its
extracted app passed 43 served/standalone browser workflows plus the header checks.
Desktop/narrow layouts, keyboard focus, and public download hashes were verified.
The existing installed copy was preserved: beta 2 did not rerun installer lifecycle
tests. The installer recipe and packaging scripts are unchanged from beta 1.

The earlier colour/security evaluation remains applicable to the unchanged engine:

Local checks passed: 1,488 exact Go/HTML corpus comparisons, 43 browser workflows,
the full Go suite and vet, actual-HTML goldens, math/contracts, and generated-image
reproducibility. All corpus runs have zero generated-readability failures. The
three previous default accent collisions are resolved. See the [quality report](QUALITY_REPORT.md)
for policy-specific counts, source-distance measurements, and coverage limits.

The Windows review ZIP is built from a clean commit with Go 1.27.3. Its accompanying
checksum and build-info file identify the exact artifact. The package is unsigned;
Windows may show an unrecognized-app warning. Extract it before launching the app.
It includes the zero-install HTML version and needs no installed Go or Node runtime.
GitHub's source ZIP is a separate download and does not contain the executable.

## Remaining tradeoffs and publication steps

Pale source colours can still require large adjustments. Source hues can repeat
colours between roles; exact locks can fail contrast. Neither a contrast ratio nor
an OKLab distance is a complete accessibility assessment.

The published beta includes the Windows installer, portable ZIP, and their checksums.
Local verification includes source/history secret scans, extracted-package smoke tests,
and installer lifecycle checks. GitHub Actions runs on source pushes; consult its run
results for remote verification. Repository About text and topics are updated.
See the [publication checklist](PRE_PUBLICATION_PLAN.md#github-publication-checklist)
for the verified public launch, enabled CI protection and vulnerability reporting, and download checks.

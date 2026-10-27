# Scene Palette 2.1.0 Beta 1

Application 2.1.0 uses extraction v5, schema 1, and the same eight palette roles.
The [private Windows beta](https://github.com/mythicruntime-lab/scene-palette/releases/tag/v2.1.0-beta.1) provides the tested installer,
portable ZIP, and `SHA256SUMS.txt`. Tag `v2.1.0-beta.1` identifies exact artifact
source commit `73161b1`; later main-branch commits record documentation and results.
See the [changelog](../CHANGELOG.md) for version history.

## Changes

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
results for remote verification. Repository About/topics and protection/reporting
settings remain separate follow-up work. Keep the repository private unless the
owner explicitly authorizes a visibility change.

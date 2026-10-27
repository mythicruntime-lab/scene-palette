# Contributing

Scene Palette has one standalone HTML frontend and a Go engine with the same
extraction behavior. Keep the standalone file usable without installation,
bundling, or a network connection. There are no Go module dependencies beyond
the standard library.

## Development setup

The source requires Go 1.23 or newer. Use the latest patched supported Go release
for regular development and distributable builds. The minimum-version CI job is
a compatibility check; it is not the release toolchain.

Use Node.js 20.9 or newer for verification; CI uses Node 24. The image oracle uses
`sharp`, and browser checks use `playwright`. Install the pinned development-only
packages from the committed lockfile:

```sh
npm ci
npx --no-install playwright install chromium
```

On Linux, use `npx --no-install playwright install --with-deps chromium` to install
the required browser system libraries as well. Windows browser tests use installed
Microsoft Edge when present and Playwright Chromium otherwise. The app itself does
not depend on Node, npm, Playwright, or sharp.

`SCENE_PALETTE_SHARP` and `SCENE_PALETTE_PLAYWRIGHT` can point to existing package
directories for local development. CI deliberately uses the lockfile instead.

## Verification

From the repository root, with Node installed before running Go tests:

```sh
go test -count=1 ./...
go vet ./...
npm run check
npm run check:browser
```

`npm run check` verifies generated fixture bytes, colour math, JavaScript contract
and index behavior, and the actual HTML extraction oracle. The Go suite also invokes
the actual JavaScript implementation when Node is available. A missing Node executable
can skip those checks, so a Go-only run is not complete parity validation.

`npm run check:browser` builds its own test executable, starts it on an available
loopback port, waits for readiness, and stops only the processes it started. It runs
the browser workflow suite, then separate generated-fixture corpus runs for Readable
UI and Source hues. Both require exact Go/HTML parity and readable generated core inks;
Readable UI also requires final accent distinction. This longer check belongs in
`jobgate start` on the shared development workstation.

For manual investigation, run `go run . serve` in one terminal and these in another:

```sh
node scripts/browser-e2e.mjs --url http://127.0.0.1:8765/
node scripts/parity-corpus.mjs --url http://127.0.0.1:8765/ --output bin/corpus/check-ui --color-policy readable-ui --require-readable --require-distinct
node scripts/parity-corpus.mjs --url http://127.0.0.1:8765/ --output bin/corpus/check-source --color-policy source-hues --require-readable
```

Use a new output directory for each corpus run. Reports, screenshots, downloads,
and binaries stay in ignored `bin/`. Browser tests use separate profiles. Generated
fixtures exercise algorithm behavior; they are not real-world usability evidence.
See [the quality report](docs/QUALITY_REPORT.md).

Keep tests strict. Fix the implementation instead of increasing tolerances or
silently replacing expected output. When deliberately changing extraction,
update the matching Go and HTML paths, record the algorithm version change,
review the fixture diff, and rerun both parity and readability checks. The
image oracle can regenerate snapshots with `node scripts/reference-extract.mjs --write`;
regeneration is not proof that a behavior change is correct.

## Continuous integration

`.github/workflows/ci.yml` runs on pushes and pull requests with read-only repository
permissions. It checks contracts on supported Go release families across Windows and
Linux, plus the advertised minimum Go version. Browser jobs run on both platforms.
The secret job scans complete reachable history and the exact committed source tree
using checksum-pinned Gitleaks. No project secrets or private image corpora are required.

Action revisions and npm dependencies are pinned. Update pins deliberately and rerun
the relevant checks. Failure artifacts contain generated-fixture browser diagnostics;
do not add private screenshots or source media to them. A local workflow edit is not
evidence of a successful GitHub Actions run; confirm the real run after an authorized push.

### Contribution checks

Submit changes to `main` through a pull request and wait for all six checks in the
`Verify` workflow to pass on the current revision:

- `Contracts (ubuntu-latest, Go 1.26.x)`
- `Contracts (windows-latest, Go 1.27.x)`
- `Contracts (ubuntu-latest, Go 1.23.x)`
- `Browsers (ubuntu-latest)`
- `Browsers (windows-latest)`
- `Secrets (history and source)`

Keep the branch up to date with `main` and resolve review conversations before merging.
Protection is enabled on `main` and applies to administrators too. It prohibits force
pushes and branch deletion and requires no second-person approval, so a solo maintainer can
merge their own passing PR. These are enforced merge gates, verified on 2026-10-27.
See the [publication checklist](docs/PRE_PUBLICATION_PLAN.md#github-publication-checklist)
for verification details. If CI job names change, update the required checks in GitHub
as well as this list.

## Windows packages

Ordinary local packaging remains available:

```powershell
powershell -NoProfile -File scripts/package-windows.ps1
```

For a review package, commit the intended source first and require a clean tree and
an exact patched Go toolchain. This example uses the patch checked on 2026-10-26;
consult the [Go release history](https://go.dev/doc/devel/release#policy) for later builds:

```powershell
powershell -NoProfile -File scripts/package-windows.ps1 -RequireClean -ExpectedGoVersion go1.27.3
```

When preparing a tagged release, also pass `-ReleaseTag` with the existing local
`vX.Y.Z` tag matching the application's version. The tag must point at HEAD, the tree
must be clean, and `-ExpectedGoVersion` is mandatory. The script verifies the executable's
embedded Go version and records all these facts in `build-info.json`.

Clean and tagged builds disable saved Go configuration (`GOENV=off`), workspaces,
inherited build flags/overlays/alternate modfiles, toolchain auto-switching, and automatic
PGO profiles. They use the selected local Go executable, the default Windows x64 instruction
set, and read-only module resolution. The effective Go environment and build flags are
recorded from a compiler-only allowlist in package metadata. Developer override values and
configuration paths are omitted; network settings are never collected. Every process environment
change is restored afterward.
Ordinary development packages retain inherited Go settings and are marked
`inherited-development`; use the clean path for a distributable package.

The script neither creates tags nor publishes releases. Verify the ZIP/checksums and smoke-test
the extracted app without Go or Node on PATH before distributing it. All packages remain unsigned.

### Build the installer

Use the portable Windows compiler from the [official NSIS download](https://nsis.sourceforge.io/Download).
No global compiler installation is required. NSIS 3.13 was used for the local Windows
beta packages. Verify its archive against the upstream checksum before building.

First build the clean portable package as above. Then pass its extracted package
directory (not the ZIP) to the installer wrapper:

```powershell
powershell -NoProfile -File scripts/package-installer.ps1 -PortableDirectory "bin/releases/scene-palette-VERSION-windows-amd64-BUILD-COMMIT" -ExpectedGoVersion go1.27.3 -NSISCompiler "path/to/nsis/makensis.exe"
```

The wrapper validates the portable file allowlist, checksums, version, clean source
identity, and source-tree hashes before compiling. It packages those same bytes;
the application is not recompiled. The Setup executable, checksum, and installer
build information remain under ignored `bin/releases/`. Neither packaging script
publishes, pushes, tags, or changes repository visibility.

The installer uses the current user's LocalAppData Programs folder, HKCU uninstall
registration, Start Menu shortcuts, and an optional desktop shortcut. It requests
no administrator rights. A stable application identity supports upgrades. Running
application files must be closed before replacement or removal. The uninstaller
removes only known installed files and its own registration/shortcuts; it does not
clear browser storage or delete user exports.

Before distribution, test first install, a previous-version upgrade, same-version
reinstall, rejection while the app is running, and uninstall. Check installed file
hashes and shortcut targets, and run the packaged app without Go or Node on PATH.
Use an isolated writable test directory and check for an existing install before
touching per-user registration. Preserve all unrelated files and processes. Both
installer and portable builds are unsigned beta artifacts until separately signed.

`scripts/verify-windows-installer.ps1` automates these lifecycle checks with
`-SetupFile`, `-PortableDirectory`, and `-PreviousSetupFile`. Add `-RunBrowserChecks`
to run the existing served/standalone workflow suite against the installed files.
It refuses existing product registration or shortcuts, uses an isolated `bin/checks/`
application directory, and cleans up its own registration and shortcuts afterward.
The previous-version input may be a test-only installer built from a known older
portable payload; record it as a fixture, not as a previously published release.

## Changes and reports

Keep changes focused and explain the observed problem, resulting behavior, and
validation. Bug reports should include OS/browser, Go or standalone mode, settings,
steps, and expected versus actual results. Attach only images and palettes you
are allowed to share. Use generated examples where possible; private screenshot
contents and exported source metadata can be sensitive.

Before committing, inspect `git diff --cached` and run a local secret scan as
described in [SECURITY.md](SECURITY.md). Do not commit credentials, local vaults,
generated reports, downloaded corpora, or release ZIPs. The MIT license covers
project contributions; preserve all third-party notices and source credits.

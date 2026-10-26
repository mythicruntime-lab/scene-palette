# Contributing

Scene Palette has one standalone HTML frontend and a Go engine with the same
extraction behavior. Keep the standalone file usable without installation,
bundling, or a network connection. There are no Go module dependencies beyond
the standard library.

## Development setup

Use Go 1.23 or newer. Node.js 20 or newer is needed for JavaScript verification.
The core Go and JavaScript contract checks need no npm packages. The image oracle
uses `sharp`; browser checks use `playwright`. Install those only for development:

```sh
npm install --no-save --package-lock=false sharp playwright
npx playwright install chromium
```

Windows browser tests use installed Microsoft Edge. On other platforms they use
Playwright's Chromium. `SCENE_PALETTE_SHARP` and `SCENE_PALETTE_PLAYWRIGHT` can
point to existing package directories instead of installing them here. The
distributed app does not depend on Node, npm, Playwright, or sharp.

## Verification

From the repository root:

```sh
go test -count=1 ./...
go vet ./...
node scripts/color-math-fixture.mjs --check
node scripts/verify-js-index.mjs
node scripts/reference-extract.mjs --check
```

The Go suite includes checks against the actual JavaScript implementation when
Node is available. A missing Node executable can skip those checks, so install
Node when validating parity.

For browser workflows, run `go run . serve` in one terminal and run these in another:

```sh
node scripts/browser-e2e.mjs --url http://127.0.0.1:8765/
node scripts/parity-corpus.mjs --url http://127.0.0.1:8765/ --output bin/corpus/check --require-readable
```

Use a new output directory for each corpus run. Reports, screenshots, downloaded
media, and binaries stay in ignored `bin/`. Browser tests use separate profiles.
The generated fixtures exercise algorithm behavior; they are not real-world
usability evidence. See [the quality report](docs/QUALITY_REPORT.md).

Keep tests strict. Fix the implementation instead of increasing tolerances or
silently replacing expected output. When deliberately changing extraction,
update the matching Go and HTML paths, record the algorithm version change,
review the fixture diff, and rerun both parity and readability checks. The
image oracle can regenerate snapshots with `node scripts/reference-extract.mjs --write`;
regeneration is not proof that a behavior change is correct.

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

# Pre-publication audit and colour improvement plan

Reviewed 2026-10-26 against `d14165b53af1272690618a19781d4cd5cadd973b`.
Status: **Windows beta published on 2026-10-27**. [Release `v2.1.0-beta.1`](https://github.com/mythicruntime-lab/scene-palette/releases/tag/v2.1.0-beta.1) includes the tested Windows installer, portable ZIP, and checksums from exact source `73161b1`. Presentation is updated; protection/reporting setup is blocked by GitHub plan and visibility restrictions. See the [current publication checklist](#github-publication-checklist).

## Implementation outcome

- Shelf imports/restores now share strict validation and independent safe rendering; imported found-ink IDs are transient editor state. Served and standalone hostile-input regressions pass.
- Added pinned, read-only CI and reproducible development dependencies. All six GitHub checks passed on `4c8bd11` in [Verify run 40593549616](https://github.com/mythicruntime-lab/scene-palette/actions/runs/40593549616), including Windows/Linux contracts and browser checks plus source/history secret scanning. Later commits require their own successful runs.
- Implemented app 2.1.0/extraction v5 locally with Readable UI final distinction, bounded source-distance refinement, optional Source hues, safe source/current diagnostics, and source-paint restore with lock/Undo.
- All twenty original public examples pass both policies: 503 and 509 exact comparisons, zero generated readability failures. The three prior default collisions are resolved. See [current results and limitations](QUALITY_REPORT.md).
- Exact reference cube-root arithmetic closes an Edge/Go parity gap without tolerance or score rounding. Legacy vault documents and frozen math fixtures remain unchanged.
- Added actual README screenshots and generated before/after examples. Clean review packaging verifies Go 1.27.3 and source provenance; ZIP checksums and packaged-app smoke checks accompany the local handoff.
- Final source/history secret scans and extracted-ZIP smoke verification are required before handoff; their reports accompany the local artifact. The owner subsequently approved the Windows packages, source push, and beta release. Tag `v2.1.0-beta.1` and its installer/ZIP/checksums are published. About/topics are updated; visibility is unchanged. The checklist below records the remaining settings.

The sections below retain the original review and approved acceptance criteria. Their old file line numbers and initial-state observations refer to the reviewed commit. The release patch was refreshed from 1.27.2 to 1.27.3 before building.

## GitHub publication checklist

Verified with repository-admin API access on **2026-10-27**:

- About now says **readable, contrast-aware UI color palettes**.
- Topics: `color-palette`, `godot`, `go`, `ui-design`, `color-extraction`,
  `contrast`, `offline`, and `windows`.
- README, release notes, changelog, and the hosted beta description use wording that
  remains accurate after publication. Beta assets and their tested source tag are unchanged.
- The repository is still private. No visibility change was requested as part of this cleanup.
- Reading and configuring `main` protection returned HTTP 403:
  **Upgrade to GitHub Pro or make this repository public to enable this feature.**
  Ruleset inspection returned the same restriction. Required checks are not configured.
- Reading and enabling private vulnerability reporting returned HTTP 404. GitHub documents
  the feature for public repositories. It is not enabled.

When the owner authorizes public visibility (or upgrades the plan for private branch protection):

1. Configure protection for `main` under **Settings → Branches**. Require pull requests,
   up-to-date branches, all six exact [contribution check names](../CONTRIBUTING.md#contribution-checks),
   and resolved conversations. Select **GitHub Actions** as each check's expected source
   (app ID `15368`). Apply the rule to administrators; prohibit force pushes and deletion.
   Keep required approving reviews at zero for solo-maintainer work.
2. After public visibility is enabled, enable **Settings → Advanced Security → Private
   vulnerability reporting**. Confirm **Report a vulnerability** appears under
   **Security → Advisories**, then update SECURITY.md to make that the primary reporting path.
3. Verify settings through the API: `GET /repos/mythicruntime-lab/scene-palette/branches/main/protection`
   must show all six checks with `strict: true` and administrator enforcement;
   `GET /repos/mythicruntime-lab/scene-palette/private-vulnerability-reporting`
   must return `enabled: true`. Confirm a normal contribution PR shows these checks as required.
4. Check the README, release notes, changelog, hosted release description, and this current
   status section again for stale access restrictions. Verify release downloads from a signed-out
   session after publication and record the date/settings evidence here. Do not retag or rebuild
   the tested beta merely to update its hosted description.

The passing CI run proves the jobs executed; it does not prove merge protection is enabled.
[GitHub branch-protection availability](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)
and [private-reporting setup](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/configure-vulnerability-reporting/configure-for-a-repository)
describe the current feature restrictions.

## Original assessment

The audit was substantially correct. The shelf-import security fix was a publication blocker.
The colour findings comprise one missing validation step and two design tradeoffs; treating
all three as contrast bugs would risk losing source identity or weakening readability.

| Finding | Verified evidence | Disposition |
| --- | --- | --- |
| Shelf import can execute injected HTML | `index.html:4402` accepts an eight-item core without validating its contents; `renderShelf()` at line 3376 inserts imported hex strings into markup. A harmless sentinel executed through the actual Import shelf control in an isolated browser. | Publication blocker. |
| The same input executes after reload | Shelf import persists it at `index.html:4417`; `loadShelf()` at line 3245 repeats the incomplete validation. The sentinel executed again after reload. | Fix import and restore together. |
| A colour-only fix is insufficient | `harnessHTML()` at `index.html:3050` interpolates the imported `stills` count into markup. A second harmless sentinel executed in Compare with otherwise valid colours. | Validate metadata and harden rendering too. |
| No CI, release, or tag | No `.github/workflows` files; GitHub returned no releases, and remote tag enumeration was empty. `main` is unprotected. | Add CI before relying on contributor changes; prepare a verified downloadable release. |
| Current Windows package needs a patched toolchain | Latest local package records Go 1.27.0 and clean source commit `1492623`. Official release history lists Go 1.27.2 with security fixes. | Rebuild final release using the latest supported patch, then verify the packaged executable. |
| Derived accents can collide | Native sets enforce minimum OKLab distance, but derived completion and later contrast/lock repairs lack a final joint distinction check. | First algorithm fix. |
| Sparse sources gain invented hues | Completion deliberately uses role-ideal hues and a chroma floor. | Add an optional source-hue policy; retain truthful origin labels. |
| Contrast repairs can substantially change paint | The solver preserves a passing sequential repair, then uses ordered fallbacks; it does not globally minimize perceptual change. | Expose the tradeoff and reduce avoidable changes without lowering contrast targets. |

The demonstrated security impact is script execution in the app page after importing a hostile
shelf, including persisted execution. It does not demonstrate arbitrary operating-system execution
or an unauthenticated remote-server attack. Reproduction used fresh headless Edge contexts,
standalone HTML, and harmless window sentinels; no network requests occurred. Local evidence is
in ignored `bin/audit-shelf/reproduction.json`. The Go app embeds the same frontend, but its served
path still needs explicit regression coverage.

The previous final publication scans already covered all three reachable commits and the intended
source files, with no secrets found. Those results are existing evidence, not a new scan during this
review. Repeat both scans on the final changed tree and history.

## 1. Close the shelf import and restore paths

Scope: `index.html`, existing pure JavaScript checks, and `scripts/browser-e2e.mjs`.

- Introduce one pure shelf-entry validation/normalization path used by file import and localStorage
  restore. Validate the complete core and found arrays, canonical role identities, strict colour
  values, consistent finite RGB values, numeric counts, and metadata types before rendering or saving.
- Reject malformed shelf entries with a clear imported/skipped summary. Keep valid entries and the
  existing shelf. Do not silently replace invalid saved colours with invented defaults, or overwrite
  stored data during startup merely because some entries were rejected.
- Reuse appropriate normalization helpers, but do not directly substitute `parsePlateJSON()`:
  it returns editor state, allocates found-ink UIDs, and does not retain the whole shelf record.
- Preserve legitimate IDs, creation times, generator identity, locks, origins, sources, tags/family,
  review history, and harmless extension fields. Preserve valid seal fingerprints; independently
  verify them, and never create a replacement seal for repaired or tampered input.
- Defend rendering independently. Only validated canonical colours may enter style values. Use
  `textContent` or context-appropriate escaping for metadata. Check shelf swatches, live chips,
  Compare, themes, and CSS/GDScript export identifiers and values, including the `stills` sink.
- Keep the existing tolerant single-palette importer compatible, while ensuring every value it
  eventually renders has a safe type and representation.

Acceptance:

- The confirmed colour and count payloads cannot create executable nodes, run a sentinel, or cause
  network activity through import, restore, Use, Compare, themes, or exports.
- Test hostile core/found colours, role IDs, names, counts, metadata, null entries, duplicate roles,
  colour/RGB contradictions, malformed shapes, and bounded oversized input in both served and
  standalone modes. Invalid input produces useful feedback without uncaught exceptions.
- Valid legacy and current shelves still round-trip; seals, deduplication, capacity behavior,
  identity, Update, A/B references, and Undo retain their intended behavior.
- Existing Go, actual-HTML contract/oracle, and browser regression checks pass. Security-only
  changes do not change extraction output or require an extraction-version bump.

## 2. Make the existing checks automatic

Scope: a small `.github/workflows/ci.yml`, reproducible development dependency declarations,
and corresponding contributor instructions. No runtime dependency is added to the app.

- Run on pushes and pull requests, using read-only repository permissions and no secrets for
  untrusted pull-request code. Pin action revisions and development packages; install with a lockfile.
- Install Node before Go tests so actual-JavaScript parity checks cannot silently skip. Run
  `go test -count=1 ./...`, `go vet ./...`, generated-fixture verification, colour-math checks,
  JavaScript index/contract checks, and the actual-HTML image oracle.
- Start a local Go server with bounded readiness checks and reliable process cleanup. Run the
  browser workflow suite and generated-image parity/readability corpus against that server and
  standalone HTML. Use installed Edge on Windows and installed Playwright Chromium on Linux.
- Exercise supported Go releases and Windows/Linux. Treat the `go 1.23` module directive as a
  language/compatibility declaration, not permission to distribute an obsolete runtime. Keep a
  separate minimum-version compatibility check if retaining the advertised minimum.
- Run Gitleaks against complete Git history and intended source files. Preserve the existing narrow
  allowlist for the browser storage namespace; do not broaden it to make a scan pass.
- Bound job time and concurrency, cancel superseded runs, and upload useful failure diagnostics.
  Use only generated/redistributable inputs in CI. Keep private and historical image artifacts local.

Acceptance: a clean checkout passes without workstation-specific paths or hidden npm packages;
the hostile-input tests fail on the old frontend and pass on the fix; pull-request checks actually
run on GitHub. Configure required checks only after their names and results are established.

## 3. Establish colour baselines and show the tradeoffs

Keep extraction v4 and its existing fixtures as the comparison baseline. Reuse the existing corpus
runner and its distinctness, invented-role, and source-adjustment measurements.

- Add small generated cases for grayscale, near-white pastel, one/two-hue scenes, derived-accent
  collisions, forced polarity, and collisions introduced by locked surfaces.
- Extend the existing **why** panel with validated source/repaired swatches side by side, repair
  reasons, and measured change. Add a concise palette-level summary of invented roles and close
  accent pairs. Existing `origin.raw` and `repairs` already record most of the necessary facts.
- Compute display diagnostics without mutating saved origins or review fingerprints. Opening an
  existing palette must not regenerate it, rewrite its extraction identity, or invalidate its seal.
- Make exact original paint an explicit Use original action following existing pick/lock and
  seal-invalidation behavior. Show the resulting contrast warnings; do not imply it is automatically
  suitable for text.

Acceptance: truthful diagnostics are visible in both modes, display safely even for imported
metadata, and leave a valid saved/sealed palette unchanged unless the user makes an edit.

## 4. Fix final accent distinction

Primary locations: `internal/extract/accents.go`, `internal/extract/plate.go`,
`internal/live/live.go`, and their corresponding functions in `index.html`.

- Evaluate derived alternatives after contrast repair against both effective surfaces. Check the
  final four accents jointly after all repairs and lock application, not only the initial real set.
- Preserve already valid sets. Prefer another observed candidate and changes to unlocked derived
  colours before modifying good source colours. Keep every locked colour exact.
- Use a bounded deterministic search with explicit candidate order and tie-breaking in Go and HTML.
  Retain the existing reference colour arithmetic and strict numeric comparisons.
- Add a mode-aware final-distinction check to the corpus runner. Record any unresolved collision
  and its reason; a search that found no solution must not claim mathematical impossibility.

Acceptance:

- Resolve the three documented historical default collisions and the new generated regressions.
- Every unconstrained Readable UI result in the release corpus meets the existing pairwise
  `0.07` OKLab distance on final RGB values and both-surface contrast targets.
- Deliberately conflicting locks remain unchanged and produce explicit warnings. Ordinary generated
  failures cannot be hidden by relabelling them as manual colours or by weakening thresholds.
- Distinction is a palette diagnostic, not proof of accessibility or colour-vision safety.

## 5. Reduce source drift and add an optional Source hues mode

Recommended behavior: **Readable UI remains the default; Source hues is opt-in.** Both keep the
eight named roles and the same export contract. Deleting roles would require a much larger schema,
preview, and export redesign.

First reduce avoidable repair distance:

- For substantial repairs, compare nearby deterministic lightness/chroma candidates that pass on
  both surfaces. Include the current valid result in the candidate set. For a fixed source and
  the same constraints, choose a result no farther from the source in OKLab.
- Recheck final accent distinction after any replacement. If improving one colour would cause a
  collision, retain the valid set and report the tradeoff. Keep surface hierarchy stable initially.
- Measure median/max source-to-final distance and the count above `0.10`, alongside visual review.
  Do not require every large shift to disappear: pale paint on a pale background cannot remain
  identical while satisfying the existing contrast target.

Then add Source hues:

- Prefer observed hue families, tonal variations, and neutral reuse. Remove the unrelated role-hue
  completion and arbitrary chroma floor from this mode; grayscale inputs must stay neutral.
- Keep all eight roles even if some share a hue. Clearly report reduced distinction when preserving
  the source hue range cannot provide four distinct readable accents. Do not secretly invent a new
  hue to satisfy a conflicting goal or silently lower contrast targets.
- Continue to label every generated/adjusted colour honestly. Source hues means preference for the
  source's hue range, not a promise of unchanged RGB values. Exact source paint remains an explicit
  manual choice.
- Carry the setting through standalone controls, Go CLI and API, cache/rebuild requests, snapshots,
  imports/shelves/exports, Remix, polarity changes, and counterpart generation. Missing settings in
  old files retain the legacy interpretation; merely opening them never re-forges their colours.

Acceptance: dedicated grayscale and limited-hue fixtures prove the policy, all generated readability
checks remain strict, reduced-distinction cases are reported separately, and both engines produce
identical snapshots for each setting. Review before/after images before choosing any further default
behavior change. Any adjustment to origins follows existing human-review invalidation rules.

## 6. Validate and version the complete result

- Increment extraction version when the algorithm changes (next available is v5); review golden
  diffs rather than accepting regenerated files as proof. If later changes ship separately, assign
  them their own extraction version. Security and display-only changes do not need that bump.
- Keep schema 1, fixed role order, and existing review fingerprint scopes. Make new generation
  parameters optional and backward-compatible. A deliberate re-forge creates a new proposal;
  old saved and sealed palettes retain their recorded identity and valid verification behavior.
- Run focused validator/selection/solver tests, the full Go suite and vet, JavaScript fixtures and
  actual-HTML oracle, then browser workflows and all returned variants in the corpus settings matrix.
- Rerun the ten generated fixtures and the same twenty historical public examples locally. All
  twenty files are currently available. Keep their sources and attribution in the development archive;
  public CI uses generated examples. Include a visual comparison of the challenging sparse/pastel cases.
- Require zero unexplained Go/HTML mismatches, zero generated readability regressions, no security
  sentinel execution, and explicit accounting for manual locks and mode-specific distinction limits.
  Do not require the old variant count: intentional changes may alter the number of valid variants.
- Retain the existing unrounded `4.5:1` body and `3:1` other-core targets on both surfaces. Raw found
  inks retain their documented meaning; they must not be confused with automatically repaired roles.
- Update the quality report with actual results, distributions, and remaining limitations. Automated
  fresh browser profiles and visual review are not a recruited-human usability study.

## 7. Prepare the downloadable release and repository presentation

- Build from a clean, tested release commit/tag using the latest patched supported Go toolchain.
  For this implementation, the refreshed official release check selected Go 1.27.3.
  Confirm the runtime embedded in the executable and its recorded build information agree.
- Keep the local packaging script useful for development; require clean source and the intended tag
  in the release path. The current script records a dirty tree but does not reject one.
- Rebuild the Windows x64 ZIP with standalone HTML, launcher, instructions, MIT license, third-party
  notices, provenance, and checksums. Revise the package wording that currently says publishing has
  not occurred before using that package in a published release; keep the unsigned warning explicit.
- Smoke-test the extracted ZIP on Windows without Go or Node on PATH: launch, image import, Forge,
  save/reload, exports, and shutdown. Verify the exact packaged HTML in standalone mode and compare
  its extraction results with the executable. Verify both package-file and ZIP checksums.
- Prepare release notes and attach the ZIP/checksum to a GitHub Release when release publication is
  authorized. Source publication and binary release are separate actions; neither changes visibility.
- Add one actual interface screenshot and one image-to-palette example using owned/generated assets.
  Use neutral repository description text and avoid a blanket accessibility claim; the current About
  description still contains the old image-category wording. Add relevant topics.
- Require passing CI on `main` after checks are established, with a workable solo-maintainer merge
  policy. Enable private vulnerability reporting when the repository becomes public; that GitHub
  feature is documented for public repositories. The owner controls the visibility change.

## Implementation order and publication gates

1. Security regression tests, shared shelf validation, and renderer fixes.
2. Reproducible CI and a passing baseline on GitHub.
3. Colour diagnostics and final-distinction fix.
4. Source-distance improvement and optional Source hues policy.
5. Full local image comparison, compatibility review, final secret scans, and updated quality report.
6. Tagged clean build, Windows package smoke test, README visuals, and release preparation.

The minimum source-publication gate is the security fix plus passing full regression and final secret
scans; CI is strongly recommended before accepting contributions. The colour work is a separate
quality milestone and is not a reason to weaken that security gate. A downloadable release also
requires the patched-toolchain build and packaged-artifact checks. The implementation changes product code locally. Repository settings, tags, releases,
and visibility remain unchanged until separately authorized.

## Reference checks

- [Local release notes](RELEASE_NOTES_2.1.0.md), [current quality report](QUALITY_REPORT.md), [Go architecture](GO_MIGRATION.md), and
  [contributor test instructions](../CONTRIBUTING.md).
- [Go release history and support policy](https://go.dev/doc/devel/release#policy): supported release
  families and the 1.27.2 patch/security fixes.
- [Playwright CI documentation](https://playwright.dev/docs/ci-intro): browser installation and
  runner setup. Adapt it to this repository's existing Node scripts; no test-framework rewrite is needed.
- [GitHub private vulnerability reporting](https://docs.github.com/en/code-security/how-tos/report-and-fix-vulnerabilities/configure-vulnerability-reporting/configure-for-a-repository): availability and repository configuration.

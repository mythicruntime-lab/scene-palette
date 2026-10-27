# Security and privacy

Scene Palette processes images locally. Standalone `index.html` runs in your
browser; the Go executable serves a local frontend and API. The API checks
loopback hosts and same-origin requests. Do not expose it through a public proxy
or use it as a multi-user service. There is no account system or authentication.

The browser shelf uses local storage. Exports can contain original images,
filenames, hashes, provenance, and review history. Inspect these before sharing.
Unsigned Windows builds and the project's review seals serve different purposes:
a palette seal records a review decision and content fingerprint; it is not a
digital signature, identity proof, or security guarantee.

## Imported data and browser storage

Shelf file import and browser-storage restore validate entries before use. Each
plate must contain eight unique roles, valid 8-bit colour channels, consistent
hex/RGB representations, and correctly typed metadata. Malformed entries are
rejected and counted; valid entries can still be loaded. Invalid stored data is
not automatically overwritten by a failed restore.

Shelf file imports are limited to 8 MiB and 400 incoming entries. Metadata is bounded by
depth, node count, string length, and per-field list sizes; the saved shelf has a
40-plate limit. The single-palette importer retains compatible legacy input shapes
and applies size, complexity, and metadata-type checks. It can fill missing roles
and normalize usable colours, so it is not the same strict path as shelf import.
The interface validates swatch colour values and escapes imported text before
rendering it. These checks do not establish that imported provenance or a review
seal was authored by a particular person.

## Reporting a vulnerability

Use **Security → Report a vulnerability**, or open the
[private vulnerability report form](https://github.com/mythicruntime-lab/scene-palette/security/advisories/new).
Private vulnerability reporting is enabled. GitHub may ask you to sign in.
If the action is unavailable,
ask the maintainer for a private reporting channel without posting exploit
details, secrets, private images, or personal data in a public issue. No response
time or formal support window is promised. Include the affected version, impact,
and a minimal reproduction with non-sensitive data.

## Local secret scanning

Install [Gitleaks](https://github.com/gitleaks/gitleaks) separately; it is a
development tool, not an app dependency. Version 8.30.1 was used for the initial
publication check. From the repository root, after creating ignored `bin/`:

```sh
gitleaks git . --log-opts=--all --redact=100 --ignore-gitleaks-allow --report-format json --report-path bin/secrets-history.json
gitleaks dir . --redact=100 --ignore-gitleaks-allow --report-format json --report-path bin/secrets-working-tree.json
```

History and working files are separate scans; a clean current file does not
erase a historical secret. Directory scans can include ignored local artifacts,
so keep reports private. `.gitleaks.toml` extends all default rules and allows
only the exact public `scene-palette.shelf.v1` local-storage key in `index.html`.
It does not exempt credentials or whole source directories. CI repeats the scan
over full reachable Git history and a source-only archive; it uses read-only
permissions, pinned action revisions, and no repository secrets for these checks.

The ignore rules help prevent accidents but cannot remove already tracked data.
If a real credential is found, revoke or rotate it and assess history cleanup
before publishing. A clean pattern scan is useful evidence, not a guarantee that
every possible secret or vulnerability has been detected.

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

## Reporting a vulnerability

If this project is hosted on GitHub with private vulnerability reporting enabled,
use the repository's **Security → Report a vulnerability** action. Otherwise,
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
It does not exempt credentials or whole source directories.

The ignore rules help prevent accidents but cannot remove already tracked data.
If a real credential is found, revoke or rotate it and assess history cleanup
before publishing. A clean pattern scan is useful evidence, not a guarantee that
every possible secret or vulnerability has been detected.

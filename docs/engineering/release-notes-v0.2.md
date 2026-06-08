# Draft v0.2 Release Notes

GOWDK v0.2 is an experimental 0.x compiler/runtime release.

Not production-ready. Public syntax, generated output, runtime packages, and
tooling contracts may change before a stable release.

## Implemented

- Open-ended 0.x hardening checklist with per-version planning buckets.
- v0.2 release checklist for Public Truth and Release Trust work.
- Release note template requiring experimental, not-production-ready, known
  gaps, checksum, and attestation sections.
- README experimental status, project laws, and current support matrix.
- Getting started release-install path with Linux, macOS, Windows, checksum,
  attestation, and VS Code `.vsix` verification notes.
- Root `SECURITY.md` aligned with the deeper repository security baseline.
- Public issue templates for compiler, generated output, runtime, docs,
  examples, language, addon, and non-sensitive security hardening reports.
- Release workflow support for version-specific release notes files.

## Partial

- GitHub-side release metadata still needs manual verification before
  publication. `v0.1.5` was observed as `isPrerelease: false` on 2026-06-08 and
  should be corrected or made explicit in the release body.
- Public `0.x Hardening` project board, labels, and issue backlog creation are
  manual GitHub-side follow-up work.
- `gowdk doctor` is referenced as planned install verification but is not
  implemented yet.

## Planned

- Public issue backlog for every current `Partial` requirement and selected
  `Planned` roadmap item.
- Release body validation for experimental warning, not-production-ready
  warning, known gaps, checksum instructions, and attestation instructions.
- Broader release smoke tests for generated app HTTP behavior.
- Automated docs link checking and Markdown lint.

## Intentionally Out Of Scope

- Production-readiness claims.
- Migration guides.
- Framework comparison docs as core positioning.
- Mandatory npm, Tailwind, Gin, Echo, Fiber, Redis, NATS, or WebSocket
  dependencies.
- New compiler syntax or runtime feature expansion.

## Known Gaps

- v0.2 is primarily a trust/docs/release hygiene slice, not a compiler feature
  release.
- Release metadata and project board changes require GitHub-side actions.
- Generated output remains pre-1.0 and unstable unless a reference doc marks a
  surface stable.

## Required Release Verification

Run the full checklist in `docs/engineering/v0.2-release-checklist.md` before
publishing.

Required local gates:

```sh
git diff --check
go test ./...
go run golang.org/x/vuln/cmd/govulncheck@latest ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
```

## Artifact Verification

Download the CLI artifact for your platform and `checksums.txt` from the GitHub
release.

```sh
grep ' <artifact>$' checksums.txt | sha256sum -c -
```

On macOS, use:

```sh
shasum -a 256 <artifact>
```

Verify GitHub artifact attestations:

```sh
gh attestation verify <artifact> -R cssbruno/GOWDK
```

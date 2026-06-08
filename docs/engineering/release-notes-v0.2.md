# Experimental 0.x release: GOWDK v0.2

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
- Automated release-note validation for the release template, v0.2 draft notes,
  and the selected release body in the release workflow.
- Published artifact smoke workflow for Linux, macOS Intel, macOS ARM, and
  Windows CLI artifacts.
- Release policy guard script for no production-ready claim, no hidden mandatory
  npm install in CI/release packaging, and draft/pre-release release metadata.
- `gowdk version --json` for release workflow verification.
- `v0.1.5` GitHub release metadata corrected to pre-release with an
  experimental/not-production-ready warning at the top of the release body.
- Public hardening labels from `docs/engineering/release-plan.md`.
- Public backlog issues for current `Partial` PRDs:
  https://github.com/cssbruno/GoWDK/issues/1 through
  https://github.com/cssbruno/GoWDK/issues/13.
- Public backlog issues for selected `Planned` roadmap items:
  https://github.com/cssbruno/GoWDK/issues/15 through
  https://github.com/cssbruno/GoWDK/issues/35.
- Public release-plan bucket and detailed backlog issues:
  https://github.com/cssbruno/GoWDK/issues/36 through
  https://github.com/cssbruno/GoWDK/issues/70.
- Focused follow-up issues for optional dependencies, release trust, compiler
  spine, diagnostics, Go interop, endpoint/security hardening, components,
  VS Code, contracts, dev overlay, and the flagship example:
  https://github.com/cssbruno/GoWDK/issues/71 through
  https://github.com/cssbruno/GoWDK/issues/114.
- Public `0.x Hardening` project board:
  https://github.com/users/cssbruno/projects/2.

## Partial

- `gowdk doctor` is referenced as planned install verification but is not
  implemented yet.

## Planned

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
- Generated output remains pre-1.0 and unstable unless a reference doc marks a
  surface stable.

## Breaking Or Unstable Generated Output

Generated output is pre-1.0. Treat generated Go, generated JavaScript,
manifests, build reports, route reports, and runtime package contracts as
unstable unless a reference doc explicitly marks a surface stable.

## Required Release Verification

Run the full checklist before publishing:

- `docs/engineering/v0.2-release-checklist.md`

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

## VS Code Extension

Install the packaged `.vsix` manually when the release includes one:

```sh
code --install-extension gowdk-vscode-<version>.vsix
```

## Tool Versions

- Go: `1.26.4`
- Node.js for extension checks: `24`

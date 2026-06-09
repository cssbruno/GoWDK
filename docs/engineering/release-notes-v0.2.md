# Experimental 0.x release: GOWDK v0.2

GOWDK v0.2 is an experimental 0.x compiler/runtime release.

Not production-ready. Public syntax, generated output, runtime packages, and
tooling contracts may change before a stable release.

## Implemented

- `BuildConfig.Scripts` for global script tags in generated build-time and
  request-time HTML documents. Use `Type: "module"` for ES module bundles.
- Page and component `js "./file.js"`, `js "./file.ts"`, and inline `js {}`
  declarations for scoped browser module inclusion without loading those
  modules on unrelated pages.
- Open-ended 0.x hardening checklist with per-version planning buckets.
- v0.2 release checklist for Public Truth and Release Trust work.
- Release note template requiring experimental, not-production-ready, known
  gaps, checksum, and attestation sections.
- README experimental status, project shape, and current support matrix.
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
  npm install in CI/release packaging, and visible pre-release metadata.
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
- `v0.2.3` release metadata: CLI/editor versions, optional module root-version
  requirements, root changelog, release-doc current-version examples, and
  GitHub milestone policy.
- Parser-style regular-expression cleanup across compiler, LSP, CSS/glob
  rewriting, runtime form scanning, and generated action validation paths.
- Optional framework/context bridge and nested optional adapter modules for
  Echo, Fiber, Gin, Redis Streams, NATS, and WebSocket fanout.
- `v0.2.5` release metadata: CLI/editor versions, optional module root-version
  requirements, root changelog, and release-doc current-version examples.
- Explicit page access metadata: real page sources must declare
  `@guard public` for intentionally public pages or protected guard IDs for
  guarded pages.
- Thin native RBAC guard IDs through `role:<name>` and `permission:<name>`,
  backed by application-owned `runtime/auth.Provider` implementations.
- Generated guarded apps fail Go compilation when required backing hooks are
  missing: `GOWDKGuardRegistry` for custom guards and `GOWDKAuthProvider` for
  native RBAC guards.
- `v0.2.6` release metadata: CLI/editor versions, optional module root-version
  requirements, root changelog, and release-doc current-version examples.
- Optional `@page` annotations with filename-derived page IDs, while keeping
  explicit `@route` and `@guard` metadata required.
- `gowdk init` now scaffolds the thinner route-first page shape and keeps
  public pages explicit through `@guard public`.
- Release packaging uploads `dist/*` as a GitHub Actions workflow artifact and
  verifies the selected tag release contains the expected download assets.
- `v0.2.7` release metadata: CLI/editor versions, optional module root-version
  requirements, root changelog, and release-doc current-version examples.
- `gowdk.Config.Env` declares normal env vars and secrets separately, validates
  empty names, duplicate names, secret-looking normal vars, and required names
  that are unset or blank.
- Generated embedded apps and backend-only apps repeat required env checks
  before serving requests.
- `gowdk inspect ir` prints the validated compiler IR for M2 debugging.
- `gowdk add` wires built-in addons into `gowdk.config.go`.
- Batteries-included auth and database addons provide common auth/session,
  password hashing, and SQLC-style database wiring helpers.
- Runtime request boundaries now include a default per-request deadline, API
  request-body caps, recovered panic logging, and secret redaction.
- `gowdk doctor` checks the local Go/GOWDK toolchain, project config, source
  discovery, language validation, route metadata, and relevant optional tools
  without writing generated output.

## Partial

- Generated app HTTP smoke coverage remains narrower than the full runtime
  surface.

## Planned

- Broader release smoke tests for generated app HTTP behavior.
- Automated docs link checking and Markdown lint.

## Intentionally Out Of Scope

- Production-readiness claims.
- Migration guides.
- Framework comparison docs as core positioning.
- Mandatory npm, Tailwind, Gin, Echo, Fiber, Redis, NATS, or WebSocket
  dependencies.
- Replacing backend authorization with generated page guards.

## Known Gaps

- v0.2 remains experimental and includes both release-trust work and early
  compiler/runtime feature slices.
- Generated output remains pre-1.0 and unstable unless a reference doc marks a
  surface stable.
- Guard metadata is a generated access redundancy layer and does not replace
  authorization in normal Go backend handlers and services.
- Env/secret metadata is a startup and config-load redundancy layer. Cloud
  platforms, containers, process managers, and secret managers still inject
  values, and backend authorization remains application-owned.

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
scripts/test-go-modules.sh
scripts/vulncheck-go-modules.sh
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

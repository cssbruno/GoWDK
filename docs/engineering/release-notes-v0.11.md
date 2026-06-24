# Experimental 0.x Release: GOWDK v0.11.0

GOWDK v0.11.0 is an experimental 0.x compiler/runtime release.

Not production-ready. Public syntax, generated output, runtime packages, and
tooling contracts may change before 1.0.

## Implemented

- CI-native audit output: versioned JSON schemas, SARIF output, stable finding
  fingerprints, report diffing, and documented audit exit codes.
- Explicit security waivers and scoped `gowdk build --allow-insecure=CODE`
  bypasses for audit findings.
- Evidence classification in security posture and audit reports, including
  app-owned obligations that static analysis cannot prove.
- Stale generated-audit-test detection with embedded posture, policy, schema,
  and compiler identity.
- Production-oriented trace sampling and OTLP export primitives in
  `runtime/trace` and `runtime/trace/otel`.
- Bounded build-time list/object iteration, comprehensions, reductions, and
  deterministic slice/struct-field build data.
- Bounded `{#await fetchJSON[T](...)}` client-island placeholders.

## Fixed

- Redirect responses validate unsafe local URLs before writing `Set-Cookie`
  headers.
- Enhanced partial forms include the clicked submit button and reject duplicate
  in-flight submissions.
- `gowdk:after-swap` dispatches from live DOM after fragment swaps.
- Store subscriber failures are isolated from later subscribers.
- `gowdk check` propagates server `g:if` scope into descendants.
- Request-time URL templates validate `srcset` candidates and encode SSR
  route/load/server-region substitutions in URL attributes.
- `gowdk dev` keeps generated app runtime ports reserved while handing the
  bound socket to child processes on supported platforms.

## Partial

- Audit, trace, build-data iteration, and client-island async helpers remain
  experimental 0.x contracts.
- OTLP export is available through the nested module; durable trace storage,
  hosted analysis, production metrics/log backends, and retention policy remain
  app-owned.
- Audit evidence can report app-owned state honestly, but authentication,
  tenant/resource authorization, and durable session policy remain application
  responsibilities.

## Planned

- Broader generated-output stability work before 1.0.
- Richer production audit fixtures and app-owned role/session evidence examples.
- More complete client interactivity primitives and generated-output tests.

## Intentionally Out Of Scope

- Production-readiness claims.
- Mandatory full-page SSR.
- Full-page hydration as the default browser model.
- User-written JavaScript as the normal app contract.
- Mandatory Tailwind, npm, Gin, Echo, Fiber, Redis, NATS, or another optional
  framework/tool dependency.
- Migration guides and framework comparison docs as core positioning.

## Known Gaps

- GOWDK remains experimental 0.x software.
- Public syntax, generated output, runtime packages, and tooling contracts may
  change before 1.0.
- Generated browser/runtime behavior is still compiler-owned enhancement, not a
  stable application framework contract.

## Breaking Or Unstable Generated Output

Generated output is pre-1.0. Treat generated Go, generated JavaScript, manifests,
and build reports as unstable unless a reference doc explicitly marks a surface
as stable.

The built-in audit baseline is now monotonic: a declared `*.audit.gwdk` policy
that reuses a built-in baseline policy name no longer replaces the built-in
policy. Rename custom policies and use `extends "baseline.<name>"` to tighten,
or add explicit waivers for specific findings.

## Required Verification

Release readiness and manual gates live in `docs/engineering/release.md`.

Required local gates:

```sh
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
code --install-extension gowdk-vscode-0.11.0.vsix
```

## Tool Versions

- Go: `1.26.4`
- Node.js for extension checks: `24`

## Release Metadata Checks

- [ ] Release is visible and has all expected download assets.
- [ ] Release is visible and is not marked as a draft.
- [ ] Release body starts with experimental and not-production-ready warnings.
- [ ] Release body includes known gaps.
- [ ] Release body includes checksum and attestation instructions.
- [ ] No production-readiness claim is made.

# Implementation Plan: Compiler And Extension Hardening

## Context

Implements `compiler-hardening-spec.md` and GitHub issues #664, #667, #669,
#670, #671, and #672.

## Assumptions

- Raw block text remains available for formatting, diagnostics, source maps,
  inspection, CSS emission, and inline Go extraction, but not as a semantic
  fallback after lowering.
- Generated output/app directories are GOWDK-owned generations.
- Legacy `Config.Addons` compatibility is required during 0.x.

## Proposed Changes

- Complete typed client/view/build/paths/server lowering and invariants; migrate
  downstream consumers away from raw-body reparsing.
- Strengthen opaque compiler snapshot and output-plan APIs.
- Add `internal/projectcompile` for canonical project compilation and structured
  diagnostics; migrate CLI/tooling consumers in bounded steps.
- Add `internal/publish` generation staging, atomic replacement, rollback, and
  recovery; route static and generated-app publication through it.
- Add typed built-in feature configuration plus a versioned executable
  extension contract and legacy addon adapter.
- Replace one-shot `go run` operations with a cached, context-bound addon host
  using versioned request/response envelopes and bounded I/O.
- Update architecture, compiler pipeline, config/addon/generated-output docs,
  requirements, and focused examples/tests.

## Files Expected To Change

- `gowdk.go`, `addons/*`, `internal/project/*`
- `internal/gwdkir`, `internal/gwdkanalysis`, `internal/compiler`
- `internal/projectcompile`, `internal/publish`
- `internal/buildgen`, `internal/appgen`, `internal/gowdkcmd`, `internal/lang`,
  `internal/lsp`
- compiler/reference/product documentation and focused fixtures

## Data And API Impact

- Adds typed feature and extension configuration while retaining deprecated
  addon compatibility.
- Adds an executable-host protocol version and structured error codes.
- Internal generator entry points increasingly require validated snapshots or
  opaque plans; public language syntax and manifest versions do not change.

## Tests

- Unit: IR invariants, phase rejection, feature provenance, host protocol,
  payload/path limits, transaction rollback/recovery.
- Integration: workspace validity agreement, repeated host reuse, static/app
  failed publication preservation.
- End-to-end: representative project check/build and generated app build.
- Manual: inspect generated output and extension timeout diagnostics.

## Verification Commands

```sh
go test ./internal/gwdkast ./internal/gwdkanalysis ./internal/gwdkir
go test ./internal/compiler ./internal/project ./internal/projectcompile ./internal/publish
go test ./internal/buildgen ./internal/appgen ./internal/lang ./internal/lsp ./internal/gowdkcmd
go test ./...
go build ./cmd/gowdk
scripts/test-go-modules.sh
scripts/check-docs-links.sh
scripts/check-docs-style.sh
scripts/check-removed-syntax.sh
scripts/check-doc-versions.sh
```

## Rollback Plan

- Revert consumers to the legacy addon adapter and direct phase entry points.
- Publication keeps the previous generation backup until the replacement is
  committed; recovery restores it if commit fails.
- No persisted user data or language syntax migration is involved.

## Risks

- Wide compiler-consumer migration can expose hidden raw-source dependencies.
- Cross-platform directory replacement differs on Windows; rollback tests must
  cover existing and absent destinations.
- Long-lived helper processes need deterministic cleanup after crash/timeout.

# Implementation Plan: go/packages Endpoint Binding Inspection

## Context

- Spec: `docs/compiler/endpoint-binding-inspection.md`
- Issue: `https://github.com/cssbruno/GoWDK/issues/257`

## Assumptions

- This is an internal compiler capability; it does not change public `.gwdk`
  syntax.
- Inline `go {}` blocks keep the existing AST inspection path until extraction
  materializes them as normal package files.

## Proposed Changes

- Replace real same-directory backend handler inspection with `packages.Load`.
- Classify supported handlers from `types.Signature` and Go type identity.
- Derive typed action input fields from `types.Struct` metadata.
- Keep current binding metadata fields stable in `gwdkir` and reports.
- Add focused tests for renamed imports/type aliases, build tags, non-exported
  handlers, package load errors, and existing typed inputs.

## Files Expected To Change

- `internal/compiler/backend_bindings.go`
- `internal/compiler/backend_bindings_test.go`
- `go.mod`
- `go.sum`
- Compiler/product docs that mention the old AST-based binding implementation.

## Data And API Impact

- Public binding report fields remain unchanged.
- The compiler root module now depends on `golang.org/x/tools/go/packages` for
  Go package loading.

## Tests

- Unit: endpoint binding signature classification and metadata tests in
  `internal/compiler`.
- Integration: generated app tests in `internal/appgen`.
- End-to-end: root module test suite.
- Manual: not required for this internal slice.

## Verification Commands

```sh
go test ./internal/compiler
go test ./internal/compiler ./internal/appgen
go test ./...
```

## Rollback Plan

- Revert the `go/packages` inspection path and dependency updates together.
- Keep binding metadata contracts stable so generated app consumers do not need
  migration.

## Risks

- `packages.Load` may surface package load errors that the previous AST scan
  ignored. The implementation reports those as missing-binding metadata so
  development builds can still emit 501 stubs while strict production builds
  fail through the existing policy.

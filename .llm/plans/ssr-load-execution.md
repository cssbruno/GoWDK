# Implementation Plan: SSR Load Execution

## Context

Spec: `.llm/features/ssr-load-execution.md`

## Assumptions

- The first implementation supports literal load key declarations only:
  `load { => { user, title } }`.
- User code owns actual request logic through a same-package exported Go
  function named from the page ID.

## Proposed Changes

- Parse load key declarations in buildgen and emit placeholders.
- Extend backend binding discovery with `load` function signatures.
- Carry load binding and replacement metadata into `SSRRoute`.
- Generate AST-based SSR handler code that calls the bound load function,
  replaces placeholders, and handles missing/unsupported/error cases.

## Files Expected To Change

- `internal/buildgen/ssr.go`
- `internal/compiler/backend_bindings.go`
- `internal/manifest/manifest.go`
- `internal/gwdkanalysis/analyzer.go`
- `internal/gwdkir/ir.go`
- `internal/appgen/*`
- SSR language and reference docs

## Tests

- Unit: buildgen load placeholder parsing.
- Unit/integration: appgen generated source and runtime handler behavior.
- Regression: missing/unsupported load functions return explicit 501.

## Verification Commands

```sh
go test ./internal/buildgen ./internal/compiler ./internal/appgen
go test ./...
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Revert load binding metadata, buildgen placeholders, and appgen load calls.
- Restore the prior SSR load rejection test if the slice needs to be backed out.

## Risks

- Placeholder rendering can hide unsupported data shapes. Keep the first slice
  scalar-only and escape all replacements.

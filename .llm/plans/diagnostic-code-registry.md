# Implementation Plan: Diagnostic Code Registry

## Context

Spec: `.llm/features/diagnostic-code-registry.md`

Issue: https://github.com/cssbruno/GoWDK/issues/75

Milestone: M2 - Compiler + Language Contract

## Assumptions

- The registry can land before constants are threaded through compiler packages.
- Scanning non-test Go source is enough to catch accidental public-code churn in
  this slice.
- Addon-provided custom codes stay outside the core registry unless they are
  emitted by repository code.

## Proposed Changes

- Add `internal/diagnostics` with a public registry of emitted codes.
- Add registry tests for sorting, uniqueness, metadata completeness, stability
  values, and source coverage.
- Update language and reference diagnostics docs.

## Files Expected To Change

- `.llm/features/diagnostic-code-registry.md`
- `.llm/plans/diagnostic-code-registry.md`
- `internal/diagnostics/registry.go`
- `internal/diagnostics/registry_test.go`
- `docs/language/diagnostics.md`
- `docs/reference/diagnostics.md`

## Data And API Impact

- No diagnostic JSON changes.
- No emitted code changes.
- New internal package only.

## Tests

- Unit: `go test ./internal/diagnostics`
- Integration: root Go tests for unchanged behavior.
- End-to-end: all nested module tests.
- Manual: inspect docs for stale code names.

## Verification Commands

```sh
go test ./internal/diagnostics
go test ./...
scripts/test-go-modules.sh
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Remove `internal/diagnostics` and docs references if the registry structure
  needs to move.

## Risks

- The source scanner intentionally catches common emitted-code patterns rather
  than parsing every possible dynamic code path. Future code-generation or addon
  paths may need additional scanner patterns.

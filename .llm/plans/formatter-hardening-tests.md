# Implementation Plan: Formatter Hardening Tests

## Context

Spec: `.llm/features/formatter-hardening-tests.md`

Issue: https://github.com/cssbruno/GoWDK/issues/79

Milestone: M2 - Compiler + Language Contract

## Assumptions

- The current formatter behavior stays unchanged in this slice.
- Parser-backed formatting is a later feature.
- Unsupported formatter cases should be documented instead of silently implied.

## Proposed Changes

- Add table-driven idempotence tests for page, component, and endpoint source.
- Add a test that formats old action block syntax and then confirms parser
  diagnostics still report the migration error.
- Update formatting docs with hardening coverage and unsupported cases.

## Files Expected To Change

- `.llm/features/formatter-hardening-tests.md`
- `.llm/plans/formatter-hardening-tests.md`
- `internal/lang/format_test.go`
- `docs/language/formatting.md`

## Data And API Impact

- No formatter output or API behavior changes.

## Tests

- Unit: `go test ./internal/lang`
- Integration: root module tests.
- End-to-end: nested module tests.
- Manual: inspect formatting docs for unsupported cases.

## Verification Commands

```sh
go test ./internal/lang
go test ./...
scripts/test-go-modules.sh
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Remove the added tests and docs note if the formatter-hardening scope changes.

## Risks

- The tests intentionally protect current line-oriented behavior without making
  unsupported formatting behavior appear stable.

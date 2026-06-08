# Implementation Plan: Diagnostic Explain Command

## Context

Spec: `.llm/features/diagnostic-explain-command.md`

Issue: https://github.com/cssbruno/GoWDK/issues/77

Milestone: M2 - Compiler + Language Contract

## Assumptions

- The registry remains the source of truth for code metadata.
- The first explain slice can provide detailed examples for common stable codes
  and fallback summaries for the rest.
- `gowdk explain` should not require project config.

## Proposed Changes

- Add explanation payload and lookup helpers to `internal/diagnostics`.
- Add close-code suggestions for unknown codes.
- Add `gowdk explain [--json] <diagnostic-code>`.
- Update CLI and diagnostics docs.
- Add CLI tests.

## Files Expected To Change

- `.llm/features/diagnostic-explain-command.md`
- `.llm/plans/diagnostic-explain-command.md`
- `internal/diagnostics/explain.go`
- `internal/diagnostics/explain_test.go`
- `cmd/gowdk/explain.go`
- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `docs/reference/cli.md`
- `docs/reference/diagnostics.md`

## Data And API Impact

- Adds CLI command `gowdk explain`.
- Adds internal JSON payload shape for `gowdk explain --json`.
- Does not change diagnostic JSON emitted by `gowdk check --json`.

## Tests

- Unit: `go test ./internal/diagnostics`
- CLI: `go test ./cmd/gowdk`
- Integration: root module tests.
- End-to-end: nested module tests.

## Verification Commands

```sh
go test ./internal/diagnostics
go test ./cmd/gowdk
go test ./...
scripts/test-go-modules.sh
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Remove `cmd/gowdk/explain.go`, explanation helpers, tests, and docs references.

## Risks

- Fallback explanations may be too generic for less common codes. Future slices
  should add more code-specific details without changing the command shape.

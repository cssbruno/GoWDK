# Implementation Plan: Diagnostic Registry Severity And Fixes

## Context

Spec: `.llm/features/diagnostic-code-registry.md`

Issue: https://github.com/cssbruno/GoWDK/issues/178

Milestone: M2 - Compiler + Language Contract

## Assumptions

- The registry can land before constants are threaded through compiler packages.
- Scanning non-test Go source is enough to catch accidental public-code churn in
  this slice.
- Addon-provided custom codes stay outside the core registry unless they are
  emitted by repository code.
- Single-file fixes are safe only when a named rewriter can compute bounded,
  non-overlapping text edits.

## Proposed Changes

- Extend `internal/diagnostics` with severity and optional fix metadata.
- Add `internal/diagnosticfix` for named source rewriters shared by CLI and
  LSP.
- Add `gowdk fix [--dry-run] [--code <code>]`.
- Add `--warnings-as-errors` to `gowdk check`.
- Route LSP code actions through registry fix metadata.
- Include registry fix metadata in diagnostic JSON serialization and explain
  output.
- Add registry tests for sorting, uniqueness, metadata completeness, stability
  values, severity values, fix metadata, and source coverage.
- Update language, CLI, and reference diagnostics docs.

## Files Expected To Change

- `.llm/features/diagnostic-code-registry.md`
- `.llm/plans/diagnostic-code-registry.md`
- `cmd/gowdk/fix.go`
- `cmd/gowdk/lang.go`
- `cmd/gowdk/main.go`
- `internal/diagnostics/registry.go`
- `internal/diagnostics/registry_test.go`
- `internal/diagnosticfix/fix.go`
- `internal/lang/diagnostic.go`
- `internal/lsp/features.go`
- `docs/language/diagnostics.md`
- `docs/reference/diagnostics.md`

## Data And API Impact

- `check --json` emits optional `fix` metadata for registered fixable
  diagnostics.
- No emitted code changes.
- New `gowdk fix` CLI command.

## Tests

- Unit: `go test ./internal/diagnostics ./internal/diagnosticfix`
- Integration: root Go tests for CLI, lang, LSP, compiler, and buildgen.
- End-to-end: nested module tests for contract adapters.
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

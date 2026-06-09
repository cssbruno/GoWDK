# Implementation Plan: gowdk doctor

## Context

Spec: `.llm/features/gowdk-doctor.md`

## Assumptions

- Branch: `feature/gowdk-doctor`.
- v1 is read-only and does not build or generate artifacts.
- JSON is versioned but remains experimental during 0.x.

## Proposed Changes

- Add `gowdk doctor` CLI dispatch, usage, and implementation.
- Build a doctor report with environment metadata, summary counts, and check
  records.
- Reuse existing config loading, discovery, language validation, IR building,
  route metadata, and contract reference linking.
- Treat optional tool failures as warnings only when relevant.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/doctor.go`
- `cmd/gowdk/main_test.go`
- `docs/reference/cli.md`
- `docs/engineering/release-plan.md`

## Data And API Impact

- Adds a public CLI command and JSON report shape:
  `version`, `status`, `summary`, `environment`, and `checks`.
- Existing CLI commands remain unchanged.

## Tests

- Unit: command JSON/text output, missing config, valid project, language error,
  optional-tool warning.
- Integration: package-level CLI tests.
- End-to-end: full repository test suite.
- Manual: run `go run ./cmd/gowdk doctor --json`.

## Verification Commands

```sh
go test ./cmd/gowdk ./internal/project ./internal/lang
go test ./...
```

## Rollback Plan

- Remove the command dispatch, implementation file, tests, and docs.

## Risks

- Over-warning on optional tools. Keep warnings relevant to detected config or
  project files only.

# Implementation Plan: Inspect Compiler IR Command

## Context

Relevant spec, issue, ADR, or discussion:

- `.llm/features/inspect-ir-command.md`
- `docs/engineering/release-plan.md` M2 compiler-spine inspect/golden-test
  checklist.

## Assumptions

- Raw `internal/gwdkir.Program` JSON is acceptable for the first M2 inspection
  slice because it is an internal debugging surface, not a stable public schema.
- `gowdk inspect` will grow more subcommands later, so `ir` should be an
  explicit target.

## Proposed Changes

- Add `gowdk inspect ir [--config <file>] [--module <name>] [--ssr] [files...]`.
- Reuse project input parsing through `loadCommandInputs`.
- Reuse `lang.CheckFiles`, lower with `gwdkanalysis.BuildIR`, link contract
  references, and print JSON.
- Add CLI coverage for explicit files, selected modules, and unknown inspect
  targets.
- Update CLI/config docs and README tooling wording.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/inspect.go`
- `cmd/gowdk/main_test.go`
- `README.md`
- `docs/reference/cli.md`
- `docs/reference/config.md`
- `.llm/features/inspect-ir-command.md`
- `.llm/plans/inspect-ir-command.md`

## Data And API Impact

- Adds a CLI command. No generated output, runtime API, persisted data, or
  public manifest schema changes.

## Tests

- Unit: CLI command tests under `cmd/gowdk`.
- Integration: selected-module discovery test.
- End-to-end: not needed for this first inspection slice.
- Manual: inspect JSON shape with an example page if needed.

## Verification Commands

```sh
go test ./cmd/gowdk -run 'Inspect|Routes' -count=1
go test ./cmd/gowdk -count=1
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Revert the inspect command, tests, and docs. Existing commands are unchanged.

## Risks

- Raw IR JSON may be treated as stable too early. Docs should call it an
  internal compiler inspection surface.

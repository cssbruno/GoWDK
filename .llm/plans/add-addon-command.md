# Implementation Plan: Add Addon Command

## Context

Relevant spec, issue, ADR, or discussion:

- `.llm/features/add-addon-command.md`
- `docs/engineering/release-plan.md` CLI/DX hardening and addon documentation
  work.

## Assumptions

- Built-in addon wiring is useful before third-party addon discovery exists.
- The command should edit only `gowdk.config.go`; dependency installation stays
  with normal Go tooling.

## Proposed Changes

- Add `gowdk add` dispatch and usage text.
- Implement a built-in addon registry for canonical addon import paths.
- Parse, mutate, and format config files through Go AST APIs.
- Add CLI tests for listing, adding, idempotency, and non-literal `Addons`.
- Document the command in CLI, config, and addons references.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/add.go`
- `cmd/gowdk/main_test.go`
- `docs/reference/cli.md`
- `docs/reference/config.md`
- `docs/reference/addons.md`
- `.llm/features/add-addon-command.md`
- `.llm/plans/add-addon-command.md`

## Data And API Impact

- Adds a CLI command that rewrites a local config file. No generated output,
  runtime API, or manifest schema changes.

## Tests

- Unit: CLI command tests in `cmd/gowdk`.
- Integration: config rewrite verified by reading the formatted file.
- End-to-end: not required for the first built-in addon wiring slice.
- Manual: run `gowdk add --list` and `gowdk add ssr` in a temp project if
  needed.

## Verification Commands

```sh
go test ./cmd/gowdk -run 'AddCommand' -count=1
go test ./cmd/gowdk -count=1
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Revert the command, tests, and docs. Users can still edit config manually.

## Risks

- AST rewriting may not preserve comments exactly. This is acceptable for the
  first slice because the command targets small scaffolded config files.

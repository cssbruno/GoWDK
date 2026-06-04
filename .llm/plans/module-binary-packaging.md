# Implementation Plan: Module Binary Packaging

## Context

Relevant spec: `.llm/features/module-binary-packaging.md`

## Assumptions

- The first implementation uses static `Build.Targets` for repeatable
  module-to-binary composition and keeps ad hoc CLI module selection.
- A generated app or binary contains the selected static output directory
  exactly as produced by the same `gowdk build` invocation.

## Proposed Changes

- Add public `BuildTargetConfig` and `Build.Targets`.
- Parse literal build targets from `gowdk.config.go`.
- Run configured targets through the existing build/app/bin path.
- Add CLI tests for all configured targets and selected `--target` builds.
- Update product and reference docs with static target examples.

## Files Expected To Change

- `README.md`
- `gowdk.go`
- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `docs/compiler/pipeline.md`
- `docs/compiler/project-structure.md`
- `docs/engineering/architecture.md`
- `docs/engineering/operations.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/reference/cli.md`
- `docs/reference/config.md`
- `internal/project/config.go`
- `internal/project/config_test.go`

## Data And API Impact

- Adds public `gowdk.BuildTargetConfig` and `gowdk.BuildConfig.Targets`.
- CLI behavior supports static `Build.Targets`, selected `--target` builds, and
  existing ad hoc `--module` selection.
- Documentation clarifies that explicit file paths bypass configured target
  execution and use the ad hoc build path.

## Tests

- Unit: none.
- Integration: CLI tests that build all configured targets and selected targets.
- End-to-end: generated binary HTTP checks for selected and unselected module
  routes.
- Manual: documented config and commands can be run from project roots with
  configured modules.

## Verification Commands

```sh
go test ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Revert the tests and documentation updates. No persisted data or runtime
  migration is introduced.

## Risks

- Separate module-target builds can reuse stale output if users point multiple
  targets at the same `Output` or `App` directory. Docs should instruct separate
  paths per binary target.

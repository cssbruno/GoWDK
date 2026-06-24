# Implementation Plan: Contract Role Binaries

## Context

Spec: `docs/product/contract-role-binaries-spec.md`.
Issue: https://github.com/cssbruno/GoWDK/issues/634.

## Assumptions

- The root module must stay free of concrete broker and scheduler dependencies.
- The first cron schedule slice only needs deterministic `@once` and
  `@every <duration>` behavior.
- Adapter startup is app-owned and provided through ordinary Go functions.

## Proposed Changes

- Extend `BuildTargetConfig` and the config parser with worker and cron role
  targets.
- Add role app generation in `internal/appgen` for `cmd/worker` and `cmd/cron`.
- Add dependency-free scheduled job validation/execution helpers to
  `runtime/contracts`.
- Add CLI flags, target selection, clean support, deploy-recipe binary
  selection, and build-report events.
- Add generated-source smoke tests that compile and run worker and cron
  binaries from a fixture module.

## Files Expected To Change

- `gowdk.go`
- `cmd/gowdk/*`
- `internal/appgen/*`
- `internal/project/*`
- `runtime/contracts/*`
- `docs/product/*`, `docs/reference/*`, `docs/compiler/*`,
  `docs/engineering/*`, `README.md`

## Data And API Impact

- New public config fields:
  `Build.Worker`, `Build.Cron`, `BuildTargetConfig.WorkerApp`,
  `WorkerBinary`, `Worker`, `CronApp`, `CronBinary`, and `Cron`.
- New runtime helpers:
  `contracts.ScheduledJob`, `ValidateScheduledJobs`, and `RunScheduledJobs`.
- No new persisted data contract and no new root production dependency.

## Tests

- Unit: config parser, scheduler validation, target selection.
- Integration: generated worker/cron app build and binary smoke tests.
- End-to-end: focused CLI configured-target test coverage.
- Manual: inspect generated app paths and build-report event output when needed.

## Verification Commands

```sh
go test ./internal/appgen -run 'TestGenerateContract|TestBuildBinaryCompilesGeneratedApp'
go test ./runtime/contracts
go test ./internal/project
go test ./cmd/gowdk -run 'TestBuildRequestHasAdHocArgs|TestBuildOptionsShouldBuildConfiguredTargets|TestSelectBuildTargetsAppliesBuildOnlyDefaultsAndValidation|TestBuildCommandRunsConfiguredBuildTargets|TestCleanTargets'
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove the new build target fields and CLI flags.
- Delete role app generation and scheduler helpers.
- Revert docs to user-owned worker/cron command guidance.

## Risks

- Schedule semantics may need a richer optional scheduler later.
- Provider signatures are intentionally narrow and may need adapter-friendly
  options once real deployments exercise more topologies.

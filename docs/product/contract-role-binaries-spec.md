# Feature Spec: Contract Role Binaries

## Problem

Contract apps can register worker-only events and cron-only jobs, but deployers
need generated executables for those roles without importing broker, queue, or
scheduler clients into the GOWDK root module.

## Goals

- Generate standalone worker and cron Go apps from scanned contract metadata.
- Compile those apps to local binaries through CLI flags and `Build.Targets`.
- Keep adapter construction app-owned through configured provider functions.
- Validate selected cron jobs and schedules before generated code is written.
- Record role app and binary generation in build reports.

## Non-Goals

- Own broker, queue, scheduler, dead-letter, rollout, or production supervision
  infrastructure.
- Add mandatory Redis, NATS, cron parser, or scheduler dependencies to the root
  module.
- Generate every future schedule policy in the first slice.

## Users And Permissions

- Primary users: Go developers deploying contract workers and scheduled jobs.
- Roles or permissions: generated worker binaries execute `worker` role event
  subscribers; generated cron binaries execute selected `cron` role jobs.
- Data visibility rules: role binaries include only the selected role's
  contract registrations and app-owned provider imports.

## User Flow

1. Register event subscribers or jobs in ordinary Go with `runtime/contracts`.
2. Configure `Build.Worker` / `Build.Cron` defaults or a worker/cron build
   target in `gowdk.config.go`.
3. Run `gowdk build --target <name>` or the matching ad hoc CLI flags.
4. Deploy the compiled worker or cron binary with app-owned infrastructure.

## Requirements

### Functional

- Worker targets require an `EventSource` provider returning
  `(contracts.EventSource, error)`.
- Worker targets may configure `SeenStore` and `Backoff` providers.
- Cron targets require explicit jobs with type, schedule, overlap policy, and
  missed-run policy.
- Cron schedules support `@once` and `@every <duration>` in the dependency-free
  runtime slice; overlap and missed-run policies support `skip`.
- Generated role apps expose a fresh registry and role-specific command under
  `cmd/worker` or `cmd/cron`.
- Role generation fails clearly when no matching role contract, provider, or
  schedule exists.

### Non-Functional

- Performance: generated binaries build from minimal role-specific Go modules.
- Reliability: event replay keeps ack/nack, deduplication, and backoff behavior
  in `runtime/contracts`; production dead-letter and durable retry policy stay
  adapter-owned.
- Accessibility: not applicable.
- Security/privacy: role binaries do not broaden web contract access; web
  adapters still execute only `web` role contracts.
- Observability: build reports include role generation and binary build events.

## Acceptance Criteria

- [x] Worker-only app and binary generation works from a configured provider.
- [x] Cron-only app and binary generation works for an `@once` job.
- [x] Unknown cron jobs and invalid schedules fail before generated output.
- [x] Root `runtime/contracts` keeps broker and scheduler dependencies optional.
- [x] `go test ./internal/appgen -run 'TestGenerateContract|TestBuildBinaryCompilesGeneratedApp'`
- [x] `go test ./runtime/contracts`
- [x] `go test ./cmd/gowdk -run 'TestBuildRequestHasAdHocArgs|TestBuildOptionsShouldBuildConfiguredTargets|TestSelectBuildTargetsAppliesBuildOnlyDefaultsAndValidation|TestBuildCommandRunsConfiguredBuildTargets|TestCleanTargets'`

## Edge Cases

- `--worker-bin` without `--worker-app` and `--cron-bin` without `--cron-app`
  fail during target selection.
- A cron job type that matches multiple scanned jobs must be qualified with the
  full import path.
- Drained event sources can exit workers cleanly by returning
  `contracts.ErrEventSourceClosed`.
- Context cancellation during worker or cron shutdown is not treated as a fatal
  startup error.

## Dependencies

- Internal: `cmd/gowdk`, `internal/appgen`, `internal/contractscan`,
  `internal/project`, `runtime/contracts`.
- External: app-owned provider packages only.

## Open Questions

- Whether future cron schedules should use a nested optional parser module or
  remain app-owned scheduler integration.
- Whether deployment recipe generation should grow role-specific Docker and
  supervisor templates beyond the current systemd starter path.

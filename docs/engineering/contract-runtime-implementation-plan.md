# Implementation Plan: Contract Runtime

## Context

Spec: `docs/product/contract-runtime-spec.md`

Roadmap step 14 is complete when typed Go contract registrations can be
inspected, executed through generated web adapters, replayed by worker/cron
roles from the same registrations, and documented with clear runtime limits.

## Assumptions

- User Go owns domain behavior, persistence, authorization, idempotency,
  transactions, scheduling, and deployment supervision.
- Generated apps expose helper APIs, but separate worker and cron binaries are
  future deployment tooling rather than a runtime contract requirement.
- Optional brokers and realtime transports stay in nested modules or
  dependency-free root runtime packages.

## Proposed Changes

- Add explicit event-worker option APIs for nacked-batch retry backoff.
- Pass event-worker options through generated `gowdkapp` worker helpers.
- Keep existing worker function behavior unchanged when no options are passed.
- Add explicit `contracts.RegisterInvalidation[event, query]` metadata and scan
  validation for domain-event to query refresh.
- Lower bound invalidation edges to IR, build reports, graph output, generated
  HTML markers, generated command event sinks, and generated client refresh.
- Update product, architecture, reference, addon, and README docs to mark the
  milestone-14 runtime boundary complete.

## Files Expected To Change

- `runtime/contracts/worker.go`
- `runtime/contracts/worker_test.go`
- `internal/appgen/source_contracts.go`
- `internal/appgen/source_realtime.go`
- `internal/appgen/appgen_test.go`
- `internal/appgen/testdata/generated_go_golden/app.go.golden`
- `internal/buildgen`
- `internal/clientrt/assets/gowdk.js`
- `internal/contractscan`
- `internal/gwdkir`
- `runtime/contracts`
- `examples/contracts`
- `docs/product/contract-runtime-spec.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/engineering/architecture.md`
- `docs/reference/contracts.md`
- `docs/reference/addons.md`
- `README.md`

## Data And API Impact

- Adds `EventWorkerRetry`, `EventWorkerBackoff`, `EventWorkerOption`,
  `WithEventWorkerBackoff`, and `ConstantEventWorkerBackoff` to
  `runtime/contracts`.
- Adds `RunEventWorker*WithOptions` runtime helpers.
- Generated apps with executable contract registrations now expose
  `RunContractEventWorkerWithOptions` and
  `RunContractEventWorkerWithSeenStoreAndOptions`.
- Adds `QueryInvalidation`, `QueryInvalidationNotice`,
  `RegisterInvalidation`, and `QueryInvalidationCommandEventSink` to
  `runtime/contracts`.
- Adds `Program.QueryInvalidations` to compiler IR and
  `query_invalidation` build-report events.
- No existing public function signature changes.

## Tests

- Unit: runtime worker backoff and context cancellation tests.
- Unit: runtime invalidation registry/event-sink tests.
- Compiler: contract scan, IR invariant, and validation tests for invalidation
  edges.
- Integration: appgen source and golden tests for generated helper emission.
- Generated output: build report, rendered marker, and generated client DOM
  refresh tests.
- End-to-end: contracts example build with generated app and binary.
- Manual: contract CLI list/graph/trace commands as needed for docs checks.

## Verification Commands

```sh
go test ./runtime/contracts
go test ./internal/contractscan ./internal/compiler ./internal/gwdkir ./internal/buildgen ./internal/appgen ./internal/clientrt
go run ./cmd/gowdk build --config examples/contracts/gowdk.config.go --out /tmp/gowdk-contracts-build --app /tmp/gowdk-contracts-app --bin /tmp/gowdk-contracts-site examples/contracts/patients.page.gwdk
go build ./cmd/gowdk
```

## Rollback Plan

- Revert the additive worker option helpers and generated helper pass-through.
- Keep existing `RunEventWorker`, `RunEventWorkerForRole`,
  `RunEventWorkerWithSeenStore`, and generated worker helper behavior.
- Remove invalidation registrations from examples; generated apps continue to
  stream explicit presentation patches for `g:subscribe` regions.

## Risks

- Worker retries can still redeliver events outside a deduplication window;
  subscribers must remain idempotent.
- Backoff policy is process-local; durable adapters still own persistent retry
  metadata and dead-letter behavior.
- Generated invalidation refresh currently refetches the current document and
  swaps matching non-subscribed regions; fragment/API-specific query execution
  remains future hardening work.

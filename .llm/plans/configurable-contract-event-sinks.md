# Implementation Plan: Configurable Contract Event Sinks

## Context

Feature spec: `.llm/features/configurable-contract-event-sinks.md`

## Assumptions

- In-process dispatch stays the default for generated command routes.
- Durable delivery policy remains adapter-owned.

## Proposed Changes

- Add `contracts.CommandEventSink` with a dispatch helper that treats `nil` as
  the default in-process sink and skips empty event batches.
- Add sink constructors for in-process, outbox, broker, and presentation fanout
  delivery.
- Add `CompositeCommandEventSink`.
- Add `runtime/contracts/membroker`, `runtime/contracts/redisstream`,
  `runtime/contracts/natsbroker`, `runtime/contracts/sse`, and
  `runtime/contracts/websocketfanout` adapters.
- Generate `RegisterContractEventSink` when executable command contract routes
  exist.
- Generate thread-safe sink registration plus `NewContractRegistry` and
  `RunContractEventWorker` when executable contracts exist.
- Change generated command contract adapters to call
  `CaptureCommandEventsForRole`, dispatch captured events through the configured
  sink, and then return JSON.
- Keep generated query adapters on `ExecuteQueryForRole`.
- Update contracts docs for generated sink registration.

## Files Expected To Change

- `runtime/contracts/contracts.go`
- `runtime/contracts/contracts_test.go`
- `runtime/contracts/membroker`
- `runtime/contracts/redisstream`
- `runtime/contracts/natsbroker`
- `runtime/contracts/sse`
- `runtime/contracts/websocketfanout`
- `internal/appgen/source_contracts.go`
- `internal/appgen/source_backend.go`
- `internal/appgen/source.go`
- `internal/appgen/appgen_test.go`
- `docs/reference/contracts.md`

## Data And API Impact

- Public runtime API adds `CommandEventSink` and sink constructor/helper
  functions.
- Generated app API adds `RegisterContractEventSink` when command contract
  routes are generated.
- Generated app API adds `NewContractRegistry` and `RunContractEventWorker`
  when executable contract registrations are generated.

## Tests

- Unit: runtime sink adapter behavior.
- Integration: generated app source and generated binary custom sink.
- End-to-end: existing generated command/query route behavior.
- Manual: none.

## Verification Commands

```sh
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Revert generated command adapters to `ExecuteCommandForRole`.
- Remove `RegisterContractEventSink` and runtime sink API.

## Risks

- Generated source ordering could accidentally dispatch events before command
  success; tests must assert capture happens after input decoding and before JSON
  response writing.

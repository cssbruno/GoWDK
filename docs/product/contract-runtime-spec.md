# Feature Spec: Contract Runtime

## Problem

Go applications need a typed backend contract model that can be inspected,
executed locally, exposed through generated web adapters, and replayed from
worker processes without turning browser UI events into trusted backend facts.

## Goals

- Model queries, commands, events, and jobs as typed Go registrations.
- Keep command ownership singular and reject duplicate command owners.
- Capture domain, integration, and presentation events only after command
  success.
- Let generated `g:command` and `g:query` adapters execute web-role contracts.
- Let worker and cron roles run the same registrations from user-owned Go
  commands or generated helper APIs.
- Provide CLI list, trace, and graph views over scanned contract metadata.

## Non-Goals

- Generate separate worker or cron binaries as part of the runtime contract.
- Treat browser UI events as backend events.
- Add a mandatory broker, queue, database, or realtime dependency to the root
  module.
- Replace app-owned authorization, transactions, idempotency, or persistence.

## Users And Permissions

- Primary users: Go developers building GOWDK apps with typed backend behavior.
- Roles or permissions: `web`, `worker`, `cron`, `api`, and `admin` runtime
  role filtering.
- Data visibility rules: generated web adapters execute only contracts
  registered for the `web` role or no explicit role; non-web references are
  compiler diagnostics.

## User Flow

1. Register contracts in normal Go with `runtime/contracts`.
2. Reference routable commands or queries from `.gwdk` with `g:command` or
   `g:query`.
3. Build a generated app and optionally register a command event sink.
4. Run subscribers locally, through an outbox/broker worker, or through cron
   role job execution from user-owned Go.
5. Inspect registrations with `gowdk contracts`, `gowdk list`, `gowdk graph`,
   and `gowdk trace`.

## Requirements

### Functional

- Typed registrations exist for queries, commands, events, and jobs.
- Command/query web references are represented in IR, build reports, routes,
  and generated adapters.
- Command execution captures emitted events and dispatches them only after
  success.
- Event replay supports role filtering, ack/nack, seen-store deduplication, and
  configurable nacked-batch backoff.
- Generated app packages expose `RegisterContractEventSink`,
  `NewContractRegistry`, and worker replay helpers for event sources.
- CLI contract reports scan registrations, roles, command emissions, event
  subscribers, jobs, diagnostics, graph, and trace output.

### Non-Functional

- Performance: local dispatch is dependency-free and does not require a broker.
- Reliability: durable adapters remain at-least-once and require idempotent
  subscribers.
- Accessibility: presentation events are output-only notifications and do not
  define browser input semantics.
- Security/privacy: browser UI events cannot declare backend facts; generated
  web adapters preserve guard, rate-limit, and CSRF ordering.
- Observability: metadata exposes stable operation names and labels.

## Acceptance Criteria

- [x] `go test ./runtime/contracts`
- [x] `go test ./internal/appgen`
- [x] `go run ./cmd/gowdk build --config examples/contracts/gowdk.config.go --out /tmp/gowdk-contracts-build --app /tmp/gowdk-contracts-app --bin /tmp/gowdk-contracts-site examples/contracts/patients.page.gwdk`

## Edge Cases

- Duplicate command owners fail scanning/check/build.
- Non-web contract references fail before generated routes run.
- Subscriber failures nack batches when the source supports nack.
- Duplicate event IDs can be skipped only after dispatch and ack succeed.
- Nil sinks default to in-process dispatch.

## Dependencies

- Internal: `runtime/contracts`, `internal/contractscan`, `internal/gwdkir`,
  `internal/appgen`, `cmd/gowdk`.
- External: optional nested Redis, NATS, and WebSocket adapter modules only when
  applications import them.

## Open Questions

- Which editor views should surface contract graph and route binding status
  first?

# Feature Spec: Configurable Contract Event Sinks

## Problem

Generated `g:command` routes currently execute commands through the local
contract registry and immediately dispatch emitted backend events in-process.
Apps that need outbox, broker, or realtime presentation delivery must hand-write
event plumbing outside the generated command adapter.

## Goals

- Let generated command adapters capture command-emitted events and hand them to
  one configurable sink.
- Preserve current in-process event dispatch as the default behavior.
- Keep UI events as command/query triggers only; templates still cannot declare
  trusted backend facts.

## Non-Goals

- Add database-backed outbox, external retry policy, or generated worker binary
  orchestration.
- Change query execution.
- Allow browser UI events to publish domain or integration events directly.

## Users And Permissions

- Primary users: Go app developers using `g:command` with contract runtime
  events.
- Roles or permissions: generated web command routes continue to execute with
  `contracts.RoleWeb`.
- Data visibility rules: presentation fanout receives only presentation events.

## User Flow

1. The user registers command handlers and event subscribers in normal Go.
2. A generated `g:command` route decodes and validates submitted input.
3. The generated adapter captures emitted command events after command success.
4. The generated adapter sends the events to the registered sink or the default
   in-process sink.

## Requirements

### Functional

- Runtime exposes `CommandEventSink`.
- Runtime exposes adapters for in-process dispatch, outbox, broker, and
  presentation fanout.
- Runtime exposes a composite sink helper for multiple destinations.
- Runtime includes dependency-free in-memory broker, Redis Streams, NATS, SSE,
  and WebSocket adapters.
- Generated apps expose `RegisterContractEventSink`.
- Passing `nil` to `RegisterContractEventSink` restores default in-process
  dispatch.
- Generated command contract routes use `CaptureCommandEventsForRole`.
- Generated query contract routes still use `ExecuteQueryForRole`.
- Generated apps expose `NewContractRegistry` and `RunContractEventWorker` for
  split worker entrypoints.

### Non-Functional

- Performance: empty event batches must be skipped.
- Reliability: sink errors must fail the generated command response before JSON
  success is written.
- Accessibility: no UI behavior changes.
- Security/privacy: `g:event` remains rejected and presentation fanout filters
  non-presentation events.
- Observability: existing contract observation names remain stable.

## Acceptance Criteria

- [x] Runtime sink adapters are covered by unit tests.
- [x] Generated command adapter source captures and dispatches events through the
  sink.
- [x] A generated app can register a custom sink and receive emitted envelopes.
- [x] `go test ./...` and `go build ./cmd/gowdk` pass.

## Edge Cases

- Empty event batch.
- Nil registered sink.
- Sink dependency errors.
- Subscriber failure through the default in-process sink.
- Concurrent sink registration while command requests are running.

## Dependencies

- Internal: `runtime/contracts`, `internal/appgen`.
- External: optional runtime adapters use `github.com/redis/go-redis/v9`,
  `github.com/nats-io/nats.go`, and `github.com/coder/websocket`.

## Open Questions

- None for this slice.

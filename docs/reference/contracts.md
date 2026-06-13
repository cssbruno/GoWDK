# Contracts

`runtime/contracts` is the first runtime slice of the GOWDK Runtime
contract-driven backend model. It is usable from normal Go today. `.gwdk`
command and query references are discoverable in compiler IR, build reports,
and generated web adapters when they have a routable method/path.

## Trust Boundary

```text
frontend UI event -> command/query -> backend handler -> backend-owned event
frontend <- result or presentation event
```

- UI events are browser-local clicks, submits, inputs, and changes.
- Commands are backend intent and have one owner handler.
- Queries read state and should not change state.
- Domain events are backend facts emitted after command success.
- Integration events are backend facts intended for durable delivery later.
- Presentation events are browser-facing notifications; they are not trusted
  input.
- Contract scanning rejects first browser-UI and vague event-name anti-patterns
  such as `ButtonClicked`, `FormSubmitted`, and `PatientChanged`.

## Runtime API

Enable future compiler integration with:

```go
Addons: []gowdk.Addon{
    contractsaddon.Addon(),
}
```

The implemented runtime registry is currently independent from compiler
integration.

Go does not support generic methods, so the API uses generic functions over a
registry. Keep this shape while the repository targets Go 1.26; revisit it when
the project upgrades to Go 1.27 and the language supports generic methods:

```go
r := contracts.NewRegistry()

contracts.RegisterQuery[GetPatientPage, PatientPageData](r, LoadPatientPage)
contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient)
contracts.RegisterDomainEvent[PatientCreated](r, SendWelcomeEmail)
contracts.RegisterDomainEvent[PatientCreated](r, WriteAuditLog)
contracts.RegisterJob[SyncPatients](r, RunPatientSync)
```

Run contracts:

```go
page, err := contracts.ExecuteQuery[GetPatientPage, PatientPageData](ctx, r, query)
result, err := contracts.ExecuteCommand[CreatePatient, CreatePatientResult](ctx, r, command)
err := contracts.ExecuteJob(ctx, r, job)
```

Run a role-filtered runtime:

```go
result, err := contracts.ExecuteCommandForRole[CreatePatient, CreatePatientResult](
    ctx,
    r,
    contracts.RoleWeb,
    command,
)

err := contracts.PublishDomainForRole(ctx, r, contracts.RoleWorker, PatientCreated{ID: id})
err := contracts.ExecuteJobForRole(ctx, r, contracts.RoleCron, SyncPatients{})
metadata := r.ContractsForRole(contracts.RoleWeb)
```

Default execution helpers run the whole in-process registry for small
single-binary apps. Role-specific helpers run handlers with no explicit roles
and handlers registered for the selected role. They skip handlers registered
only for another role and return `role_not_allowed` when the selected role
tries to execute a command, query, or job that is not available to that role.

Generated web adapters always execute command/query references with
`contracts.RoleWeb`. A `.gwdk` `g:command` or `g:query` reference to a
worker/cron/admin/API-only contract is a compiler diagnostic, not a generated
route that fails later.

## Local Single-Binary App Path

The supported M6 path is local-first: one generated binary can serve the page,
execute `g:command` and `g:query` web adapters through the web role, and replay
captured backend events through local runtime helpers. Split worker binaries
and cron generation remain planned, not production-ready behavior.

Minimal page:

```gwdk
package contracts

import patients "github.com/acme/clinic/patients"

page patients
route "/patients"
guard public

view {
  <main>
    <form method="post" action="/patients" g:command="patients.CreatePatient">
      <input name="name" />
      <button>Create patient</button>
    </form>
    <section g:query="patients.GetPatientPage"></section>
  </main>
}
```

Normal Go owns the contracts and handlers:

```go
package patients

import (
    "context"

    "github.com/cssbruno/gowdk/runtime/contracts"
)

type GetPatientPage struct{ Filter string }
type PatientPageData struct{ Source string `json:"source"` }
type CreatePatient struct{ Name string }
type CreatePatientResult struct{ ID string `json:"id"` }
type PatientCreated struct{ ID string }

func Register(registry *contracts.Registry) {
    contracts.RegisterQuery[GetPatientPage, PatientPageData](registry, LoadPatientPage, contracts.RoleWeb)
    contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
    contracts.RegisterDomainEvent[PatientCreated](registry, SendWelcomeEmail, contracts.RoleWorker)
}

func LoadPatientPage(ctx context.Context, query GetPatientPage) (PatientPageData, error) {
    return PatientPageData{Source: "db"}, nil
}

func HandleCreatePatient(ctx context.Context, command CreatePatient) (CreatePatientResult, error) {
    if err := contracts.EmitDomain(ctx, PatientCreated{ID: "patient-1"}); err != nil {
        return CreatePatientResult{}, err
    }
    return CreatePatientResult{ID: "patient-1"}, nil
}

func SendWelcomeEmail(ctx context.Context, event PatientCreated) error { return nil }
```

The generated command adapter captures events emitted by `HandleCreatePatient`
only after the command succeeds. Browser events remain untrusted UI input;
backend facts must be emitted from Go handlers with `EmitDomain`,
`EmitIntegration`, or `EmitPresentation`.

The repository example is `examples/contracts/`:

```sh
go run ./cmd/gowdk build --config examples/contracts/gowdk.config.go \
  --out /tmp/gowdk-contracts-build \
  --app /tmp/gowdk-contracts-app \
  --bin /tmp/gowdk-contracts-site \
  examples/contracts/patients.page.gwdk
```

Verify adapter metadata through the build report:

```sh
grep -F '"name": "patients.CreatePatient"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F '"kind": "command"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F '"path": "/contracts/patients"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F '"guards": "public"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F '"name": "patients.GetPatientPage"' /tmp/gowdk-contracts-build/gowdk-build-report.json
grep -F '"kind": "query"' /tmp/gowdk-contracts-build/gowdk-build-report.json
```

## Observability

`runtime/contracts` exposes stable operation names and labels for logs,
metrics, and traces. These names are API values, not CLI display text.

```go
metadata := r.ContractsForRole(contracts.RoleWeb)
observation := metadata[0].ObservationForRole(
    contracts.ObservationExecuteCommand,
    contracts.RoleWeb,
)
```

The stable operation names include:

| Operation | Name |
| --- | --- |
| Register query | `gowdk.contract.register.query` |
| Register command | `gowdk.contract.register.command` |
| Register event | `gowdk.contract.register.event` |
| Register job | `gowdk.contract.register.job` |
| Execute query | `gowdk.contract.execute.query` |
| Execute command | `gowdk.contract.execute.command` |
| Capture command events | `gowdk.contract.capture.command` |
| Execute job | `gowdk.contract.execute.job` |
| Publish event | `gowdk.contract.publish.event` |
| Store command events in outbox | `gowdk.contract.outbox.store` |
| Publish broker events | `gowdk.contract.broker.publish` |
| Send presentation events | `gowdk.contract.presentation.send` |
| Worker receive batch | `gowdk.contract.worker.receive` |
| Worker ack batch | `gowdk.contract.worker.ack` |
| Worker nack batch | `gowdk.contract.worker.nack` |
| Worker dedup skip | `gowdk.contract.worker.dedup_skip` |

`Metadata.ObservationLabels()` returns the stable contract labels: kind, event
category, contract type name, result type name, role, roles, and handler count
when known. `EventEnvelope.ObservationLabels()` returns the event kind,
category, stable event ID, and captured event contract type.
`ContractName[T]()` returns the same Go contract type name used by metadata and
event envelopes.
`ObservationForRole` records the runtime role that performed the operation.

Inside a command handler, emit backend-owned events through the command context:

```go
func HandleCreatePatient(ctx context.Context, cmd CreatePatient) (CreatePatientResult, error) {
    id := "patient-1"
    if err := contracts.EmitDomain(ctx, PatientCreated{ID: id}); err != nil {
        return CreatePatientResult{}, err
    }
    return CreatePatientResult{ID: id}, nil
}
```

Emitted events are dispatched only after the command handler returns
successfully. If the command returns an error, recorded events are discarded.

Capture events instead of dispatching subscribers when a command needs an
outbox boundary:

```go
result, events, err := contracts.CaptureCommandEvents[CreatePatient, CreatePatientResult](
    ctx,
    r,
    command,
)
```

Each captured `EventEnvelope` contains a stable event ID, event category, Go
type name, and typed value. Capturing does not run event subscribers.

For dependency-free outbox integration, implement the small `Outbox` interface:

```go
type PatientOutbox struct{}

func (PatientOutbox) StoreEvents(ctx context.Context, events []contracts.EventEnvelope) error {
    return nil
}

result, err := contracts.ExecuteCommandToOutbox[CreatePatient, CreatePatientResult](
    ctx,
    r,
    PatientOutbox{},
    command,
)
```

`ExecuteCommandToOutbox` stores events only after the command handler succeeds.
It does not dispatch subscribers. Database transactions, outbox tables, retry
policy, idempotency, broker publication, and worker delivery remain adapter
responsibilities outside the core package.

For durable domain events, adapter code should preserve this order:

```text
start transaction
apply state change
store domain event in outbox
commit transaction
publish from worker
```

Use plain `ExecuteCommand` for small single-binary apps where in-process
subscriber dispatch is enough. Use `CaptureCommandEvents` or
`ExecuteCommandToOutbox` when subscribers should run from a later worker or
broker delivery path.

GOWDK Runtime also includes a dependency-free file outbox adapter for local
durable JSON Lines storage:

```go
import "github.com/cssbruno/gowdk/runtime/contracts/fileoutbox"

outbox := fileoutbox.New(
    "var/gowdk-outbox.jsonl",
    fileoutbox.WithJSONTypeDecoder[PatientCreated](),
    fileoutbox.WithDeadLetter("var/gowdk-outbox.dead.jsonl", 5),
)

_, err := contracts.ExecuteCommandToOutbox[CreatePatient, CreatePatientResult](
    ctx,
    r,
    outbox,
    command,
)

err = contracts.RunEventWorker(ctx, r, outbox)
```

The file outbox implements both `contracts.Outbox` and
`contracts.EventSource`. It appends captured envelopes as JSON Lines records,
decodes records through explicitly registered decoders, removes records only
after worker `Ack`, and keeps records after `Nack` for retry. Nack records the
attempt count, last attempt time, and last error in the durable record. It is
useful for local development, small single-host deployments, and tests.
When `WithDeadLetter(path, maxAttempts)` is configured, records move to the
dead-letter JSON Lines file after the configured failed delivery count.
Applications that need database transactions, cross-process locking, retry
backoff, broker delivery, or operational dead-letter processing should use a
database-backed or broker-backed adapter.

Delivery guarantees:

- Local in-process dispatch is process-local exactly once for that command
  execution because subscribers run before the command response is written.
- Outbox and broker delivery is at-least-once. Use `RunEventWorkerWithSeenStore`
  or `RunEventWorkerForRoleWithSeenStore` with a `contracts.SeenStore` to skip
  duplicate event IDs inside a configured deduplication window. Duplicate
  batches are acknowledged without invoking subscribers.
- A deduplication window is not an exactly-once guarantee. Subscribers must
  still tolerate redelivery outside the window, after store loss, after seen
  store write failures, or across concurrent workers. Event IDs are marked seen
  only after worker dispatch and source `Ack` both succeed.

GOWDK Runtime provides three seen-store adapters:

- `contracts.NewMemorySeenStore(limit)` keeps a bounded process-local LRU
  window for local single-binary apps and tests.
- `fileoutbox.NewSeenStore(path, fileoutbox.WithSeenLimit(limit))` keeps a
  dependency-free JSON Lines window next to the file outbox.
- `redisstream.NewSeenStore(client, prefix, ttl)` checks IDs with Redis
  `EXISTS`, records IDs with `SET`, and applies an expiration TTL for Redis
  Streams worker deployments.

Subscriber handlers must still be idempotent for any durable delivery adapter.
Use a stable domain key, event ID, outbox record ID, or application-level
dedupe table to make repeated deliveries safe. GOWDK Runtime does not hide
retries behind generated JavaScript or browser state.

External broker adapters can implement the dependency-free `Broker` interface:

```go
type PatientBroker struct{}

func (PatientBroker) PublishEvents(ctx context.Context, events []contracts.EventEnvelope) error {
    return nil
}

result, err := contracts.ExecuteCommandToBroker[CreatePatient, CreatePatientResult](
    ctx,
    r,
    PatientBroker{},
    command,
)
```

`ExecuteCommandToBroker` publishes captured events only after the command
handler succeeds. It does not dispatch local subscribers. Broker adapters own
serialization, acknowledgements, retries, dead-letter behavior, and delivery
guarantees.

Realtime adapters can implement `PresentationFanout` for browser-facing output:

```go
type PatientFanout struct{}

func (PatientFanout) SendPresentationEvents(ctx context.Context, events []contracts.EventEnvelope) error {
    return nil
}

result, err := contracts.ExecuteCommandToPresentationFanout[CreatePatient, CreatePatientResult](
    ctx,
    r,
    PatientFanout{},
    command,
)
```

Only presentation events are sent to fanout. Domain and integration events are
filtered out. Fanout adapters own SSE/WebSocket sessions, serialization, client
targeting, buffering, and disconnect behavior.

Worker or broker adapter code can replay captured events through the same typed
subscriber registry:

```go
err := contracts.PublishEnvelopesForRole(ctx, r, contracts.RoleWorker, events)
```

Envelope replay keeps the original event category and type. Subscribers still
run through role filtering. If a subscriber returns an error, replay stops and
returns `subscriber_failed`; adapter retry and idempotency policy stays outside
the core runtime.

Queue or outbox adapters can drive worker subscribers with `EventSource`:

```go
type PatientEventSource struct{}

func (PatientEventSource) ReceiveEventBatch(ctx context.Context) (contracts.EventBatch, error) {
    return contracts.EventBatch{}, contracts.ErrEventSourceClosed
}

err := contracts.RunEventWorker(ctx, r, PatientEventSource{})
```

`RunEventWorker` dispatches batches with `RoleWorker`, calls `Ack` after
successful subscriber replay, calls `Nack` when subscriber replay fails, stops
cleanly when the source returns `ErrEventSourceClosed`, and returns the context
error when `ctx` is canceled. `RunEventWorkerForRole` can be used for another
runtime role. `ErrEventSourceClosed` means a finite source drained cleanly, as
with the file outbox or in-memory broker; long-lived brokers such as Redis
Streams and NATS should keep blocking until the worker context is canceled or
the adapter is genuinely closed.

Generated command routes use the same event-plumbing boundary through one
configurable sink:

```go
gowdkapp.RegisterContractEventSink(contracts.OutboxCommandEventSink(outbox))
```

The generated app API exists when routable command contract adapters are
generated. Passing `nil` restores the default in-process sink:

```go
gowdkapp.RegisterContractEventSink(nil)
```

Available sink helpers:

- `InProcessCommandEventSink()` dispatches captured events through the local
  registry with role filtering.
- `OutboxCommandEventSink(outbox)` stores captured events without local
  subscriber dispatch.
- `BrokerCommandEventSink(broker)` publishes captured events to a broker.
- `PresentationFanoutCommandEventSink(fanout)` sends only presentation events.
- `CompositeCommandEventSink(...)` sends the same captured event batch to
  multiple sinks in order.

Apps that need more than one destination can implement
`contracts.CommandEventSink` directly or use `CompositeCommandEventSink`.

Choose the sink based on where subscribers should run:

| Need | Sink |
| --- | --- |
| Small single-binary app | `InProcessCommandEventSink()` |
| Local durable queue or test fixture | `OutboxCommandEventSink(fileoutbox.New(...))` |
| Local in-memory queue | `BrokerCommandEventSink(membroker.New())` |
| Redis Streams queue | `BrokerCommandEventSink(redisstream.New(...))` |
| Core NATS live pub/sub | `BrokerCommandEventSink(natsbroker.New(...))` |
| Browser notifications over SSE or WebSocket | `PresentationFanoutCommandEventSink(hub)` |
| More than one destination | `CompositeCommandEventSink(...)` |

`CompositeCommandEventSink` sends the same captured batch to each sink in
order. A later sink is not called after an earlier sink returns an error.
Presentation fanout sinks filter non-presentation events themselves. Broker and
outbox sinks receive the full event batch.

Generated packages with executable contract registrations also expose:

```go
registry := gowdkapp.NewContractRegistry()
err := gowdkapp.RunContractEventWorker(ctx, source)
err = gowdkapp.RunContractEventWorkerWithSeenStore(ctx, source, seen)
```

`NewContractRegistry` creates a fresh registry using the scanned registration
functions. `RunContractEventWorker` replays an `EventSource` through the same
registrations with the worker role. `RunContractEventWorkerWithSeenStore` uses
the same worker role and skips duplicate event IDs through the provided
`contracts.SeenStore`.

These helpers are deliberately local process APIs. Generated apps do not yet
emit separate worker or cron binaries, supervisor configs, queue topology, or
managed deployment recipes. Use them from the generated binary, a user-owned
command, or a test fixture until split worker generation is designed.

Dependency-free adapters:

- `runtime/contracts/fileoutbox` stores JSON Lines records on disk and
  implements both `Outbox` and `EventSource`.
- `runtime/contracts/membroker` provides an in-memory `Broker` and
  `EventSource` for tests, local development, and single-process apps.
- `runtime/contracts/sse` provides an `http.Handler` and
  `PresentationFanout` for server-sent browser presentation events.

Optional broker and realtime adapters:

- `runtime/contracts/redisstream` uses Redis Streams as a `Broker` and
  `EventSource`.
- `runtime/contracts/natsbroker` uses core NATS publish/subscribe as a
  `Broker` and `EventSource`.
- `runtime/contracts/websocketfanout` provides an `http.Handler` and
  `PresentationFanout` for browser WebSocket clients.

These concrete optional adapters are nested Go modules. Add only the adapter an
application uses:

```sh
go get github.com/cssbruno/gowdk/runtime/contracts/redisstream
go get github.com/cssbruno/gowdk/runtime/contracts/natsbroker
go get github.com/cssbruno/gowdk/runtime/contracts/websocketfanout
```

## Sink Recipes

### Redis Streams

Use Redis Streams when command routes should append events to a queue and a
worker should replay subscribers later:

```go
import (
    "time"

    "github.com/cssbruno/gowdk/runtime/contracts"
    "github.com/cssbruno/gowdk/runtime/contracts/redisstream"
    redis "github.com/redis/go-redis/v9"
)

client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})

events := redisstream.New(
    client,
    "gowdk:events",
    "gowdk-workers",
    "worker-1",
    redisstream.WithBlock(5*time.Second),
    redisstream.WithJSONDecoder[PatientCreated]("patients.PatientCreated"),
)

if err := events.EnsureGroup(ctx); err != nil {
    return err
}

gowdkapp.RegisterContractEventSink(contracts.BrokerCommandEventSink(events))
```

A worker can use the same adapter as an `EventSource`:

```go
if err := gowdkapp.RunContractEventWorker(ctx, events); err != nil {
    return err
}
```

`Ack` calls `XACK` and then `XDEL`. Subscriber failures leave messages pending
for the Redis consumer group to handle according to app-owned retry policy.
Register JSON decoders for event types that need typed Go values when replayed
through subscribers.

### NATS

Use the NATS adapter for live event distribution where subscribers are expected
to be online:

```go
import (
    "time"

    "github.com/cssbruno/gowdk/runtime/contracts"
    "github.com/cssbruno/gowdk/runtime/contracts/natsbroker"
    nats "github.com/nats-io/nats.go"
)

conn, err := nats.Connect(nats.DefaultURL)
if err != nil {
    return err
}

events := natsbroker.New(
    conn,
    "gowdk.events",
    natsbroker.WithQueue("gowdk-workers"),
    natsbroker.WithTimeout(5*time.Second),
    natsbroker.WithJSONDecoder[PatientCreated]("patients.PatientCreated"),
)
defer events.Close()

gowdkapp.RegisterContractEventSink(contracts.BrokerCommandEventSink(events))
```

Worker replay uses the same adapter:

```go
if err := gowdkapp.RunContractEventWorker(ctx, events); err != nil {
    return err
}
```

This adapter uses core NATS publish/subscribe. It does not provide durable
replay for offline subscribers. Use Redis Streams, the file outbox, or a
custom JetStream adapter when events must survive worker downtime.

### SSE Presentation Fanout

Use SSE when the app needs one-way browser presentation events:

```go
import (
    "net/http"

    "github.com/cssbruno/gowdk/runtime/contracts"
    "github.com/cssbruno/gowdk/runtime/contracts/sse"
)

hub := sse.New()
http.Handle("/gowdk/events", hub)

gowdkapp.RegisterContractEventSink(
    contracts.PresentationFanoutCommandEventSink(hub),
)
```

The browser receives `event: gowdk-presentation` messages whose `data` value is
the JSON `contracts.EventEnvelope`. Domain and integration events are ignored.

### WebSocket Presentation Fanout

Use WebSocket fanout when clients need a persistent bidirectional transport for
presentation events:

```go
import (
    "net/http"

    "github.com/coder/websocket"
    "github.com/cssbruno/gowdk/runtime/contracts"
    "github.com/cssbruno/gowdk/runtime/contracts/websocketfanout"
)

hub := websocketfanout.New(websocketfanout.WithAcceptOptions(websocket.AcceptOptions{
    OriginPatterns: []string{"https://example.com"},
}))
http.Handle("/gowdk/events/ws", hub)

gowdkapp.RegisterContractEventSink(
    contracts.PresentationFanoutCommandEventSink(hub),
)
```

Each presentation event is written as one text JSON `contracts.EventEnvelope`.
Slow clients can drop queued messages when their buffer fills; tune
`WithBufferSize` for the app's realtime behavior.

### Fanout Plus Queue

Apps can send browser presentation events immediately and still queue the full
event batch for backend workers:

```go
sink := contracts.CompositeCommandEventSink(
    contracts.PresentationFanoutCommandEventSink(hub),
    contracts.BrokerCommandEventSink(events),
)

gowdkapp.RegisterContractEventSink(sink)
```

The generated command route fails before writing JSON success if any sink
returns an error.

## `.gwdk` Command References

Use `g:command` on forms to declare backend command intent:

```html
<form method="post" action="/patients" g:command="patients.CreatePatient">
  <input name="name">
  <button>Create patient</button>
</form>
```

Current behavior:

- Renders `data-gowdk-command="patients.CreatePatient"`.
- Adds a command reference to `internal/gwdkir.Program.ContractRefs`.
- Records the reference alias, imported package path when declared with a
  `.gwdk import`, local command type, bound result type, binding status, and
  handler/register function names and runtime roles in IR/build-report
  metadata when known.
- Records literal form `method` and `action` as command adapter IR
  method/path. If a page-owned `g:command` form omits `action`, the command
  adapter path is the owning page route, matching the browser's default form
  submission target.
- `gowdk build` links command references to scanned Go command registrations
  and adds `contract_reference` events with status and source line/column to
  `gowdk-build-report.json`. Command events include method/path when present.
- Generated apps register the scanned package registration function once in a
  local `runtime/contracts.Registry`, route the form method/action through the
  backend router, capture emitted backend events with
  `CaptureCommandEventsForRole(..., contracts.RoleWeb, input)`, send captured
  events to the configured command event sink, and return the command result as
  no-store JSON.
- Success responses are `200 application/json` with the command result encoded
  directly as JSON. Error responses are `application/json` with
  `{"error":"..."}` and `Cache-Control: no-store`; ordinary 5xx errors use the
  generic HTTP status text, while `response.NewHandlerError(status, message,
  cause)` can opt into an explicit client-safe status and message.
- When the scanner can see the exported command input struct fields, generated
  adapters parse submitted form values, allow only the scanned fields, decode
  supported scalar fields, and pass the typed command input to the registry.
- When generated CSRF is enabled, command contract forms receive the same
  hidden token injection as POST action forms, and generated command adapters
  validate the submitted token before dispatch.
- Command references on guarded pages inherit the page guards. When rate
  limiting is enabled, generated command adapters run rate limiting first,
  guards second, then form parsing, CSRF validation, typed input decoding, and
  command execution.
- `gowdk check` and CLI `gowdk build` fail when a command reference is missing,
  linked to an invalid Go handler signature, or bound only to non-web runtime
  roles.
- Requires a package-qualified Go reference such as `patients.CreatePatient`.
- Must not be combined with `g:post`.

If the scanner cannot see the command input fields yet, generated command
adapters construct a zero-value command input before dispatch.

## `.gwdk` Query References

Use `g:query` on HTML elements to declare readonly backend query intent:

```html
<section g:query="patients.GetPatientPage">
  <h1>Patients</h1>
</section>
```

Current behavior:

- Renders `data-gowdk-query="patients.GetPatientPage"`.
- Adds a query reference to `internal/gwdkir.Program.ContractRefs`.
- Records the reference alias, imported package path when declared with a
  `.gwdk import`, local query type, bound result type, binding status, and
  handler/register function names and runtime roles in IR/build-report
  metadata when known.
- `gowdk build` links query references to scanned Go query registrations and
  adds `contract_reference` events with status and source line/column to
  `gowdk-build-report.json`.
- Page-owned query references record `GET` plus the page route as first
  request-time source metadata.
- Generated apps register the scanned package registration function once in a
  local `runtime/contracts.Registry`, route page-owned query references through
  the backend router, execute the query with `ExecuteQueryForRole(...,
  contracts.RoleWeb, input)`, and return the query result as no-store JSON.
- Success responses are `200 application/json` with the query result encoded
  directly as JSON. Error responses are `application/json` with
  `{"error":"..."}` and `Cache-Control: no-store`; ordinary 5xx errors use the
  generic HTTP status text, while `response.NewHandlerError(status, message,
  cause)` can opt into an explicit client-safe status and message.
- Page-owned query routes share the page path, so generated apps dispatch them
  only for explicit query requests: `Accept: application/json`, another
  `+json` media type, or `X-GOWDK-Query: true`. Normal document requests keep
  serving the page HTML at the same route.
- When the scanner can see the exported query input struct fields, generated
  adapters decode supported URL query parameters into the typed query input.
- Query references on guarded pages inherit the page guards. When rate limiting
  is enabled, generated query adapters run rate limiting first, guards second,
  then typed URL query decoding and query execution.
- `gowdk check` and CLI `gowdk build` fail when a query reference is missing,
  linked to an invalid Go handler signature, or bound only to non-web runtime
  roles.
- Requires a package-qualified Go reference such as `patients.GetPatientPage`.
- Must not be combined with `g:post` or `g:command` on the same form.

If the scanner cannot see the query input fields yet, generated query adapters
construct a zero-value query input before dispatch.

Templates must not declare backend facts:

```html
<!-- rejected -->
<form g:event="PatientCreated">
```

Use `g:on:*` for local UI/component events and `g:command` for backend intent.

## Current Limits

- Generated command/query adapters execute bound references through
  `runtime/contracts` when the `.gwdk` reference has a routable method/path,
  an import path, a local contract type, a result type, and a scanned package
  registration function.
- `.gwdk` command/query reference linking matches the full reference name, the
  captured local contract type, and the scanned Go contract type import path
  when the `.gwdk` import alias differs from the Go package name.
- Form-local `g:command` references and element-local `g:query` references
  include exact source line and column in IR and build reports.
- Missing, invalid, or non-web-only command/query references produce
  `contract_reference_*` diagnostics in `gowdk check` and stop CLI builds.
- Generated fallback contract routes that remain in appgen for allowed
  non-bound modes return explicit HTTP 501 no-store responses.
- Other contract diagnostics do not all have exact source spans yet.
- `gowdk contracts`, `gowdk list commands|queries|events|jobs`, `gowdk graph`,
  and `gowdk trace <contract>` can scan Go AST registration calls today.
- Contract scan reports include `go/types` diagnostics for command, query,
  event, and job handler signatures across local package files and imported
  handler symbols when the standard Go importer can resolve them.
- Contract scanning caches package import/export inspection by package directory
  and import set inside each scan.
- Contract scanning rejects feature packages that import generated app output
  such as `gowdk-generated-app/gowdkapp`, because that dependency direction
  creates generated app import cycles.
- Contract scan reports include the top-level package registration function
  that accepts `*contracts.Registry`, when the registration call is inside one.
- Contract scan roles are propagated into linked IR, app adapter IR, and
  build-report metadata.
- Page guards are propagated into linked IR and app adapter IR for generated
  command/query routes.
- Contract scan reports include same-package exported command/query input struct
  fields for generated form/query decoders.
- Contract scan reports validate local and imported contract/result types
  resolved by `go/types` as exported struct symbols where the scanner can
  resolve them.
- Contract scan reports duplicate command owner registrations.
- `gowdk check` and CLI `gowdk build` fail on contract scan diagnostics such
  as invalid handler signatures and duplicate command owners.
- `gowdk graph` detects command-emitted events when command handlers call
  `contracts.EmitDomain`, `contracts.EmitIntegration`, or
  `contracts.EmitPresentation` with a visible event type.
- Contract scanning reports `contract_event_category_invalid` when a command
  emits a visible event type under one category but the scanner only sees
  registrations for that event type under another category.
- `gowdk trace <contract>` reports a single command/query/event/job, command
  emitted events, event subscribers, source locations, handlers, and roles.
- `runtime/contracts` can capture command-emitted events as `EventEnvelope`
  values and pass them to a dependency-free `Outbox` interface without
  dispatching subscribers.
- `EventEnvelope` carries a stable event ID for outbox/broker replay and worker
  deduplication.
- Captured event envelopes can be replayed later with
  `PublishEnvelope`, `PublishEnvelopes`, and role-filtered variants.
- `runtime/contracts/fileoutbox` provides a dependency-free JSON Lines adapter
  that implements `contracts.Outbox` and `contracts.EventSource`, including
  nack retry metadata and an opt-in dead-letter file.
- `contracts.NewMemorySeenStore`, `fileoutbox.NewSeenStore`, and
  `redisstream.NewSeenStore` provide deduplication windows for event workers.
- External broker adapters can implement the dependency-free `Broker`
  interface and receive captured envelopes through `ExecuteCommandToBroker` or
  `PublishEventsToBroker`.
- Realtime adapters can implement the dependency-free `PresentationFanout`
  interface and receive only presentation envelopes through
  `ExecuteCommandToPresentationFanout` or `SendPresentationEventsToFanout`.
- Generated command adapters expose `RegisterContractEventSink`; a registered
  `CommandEventSink` receives captured command events before the generated
  adapter writes the JSON command result.
- Generated contract packages expose `NewContractRegistry` and
  `RunContractEventWorker` / `RunContractEventWorkerWithSeenStore` when
  executable contract registrations are present.
- Queue/outbox adapters can implement the dependency-free `EventSource`
  interface and drive worker-role subscribers through `RunEventWorker`.
- `internal/appgen` records command/query contract exposure metadata in backend
  adapter IR, including reference name, alias, import path, local contract
  type, result type, runtime roles, decoded input fields, binding status,
  handler, register function, owner, and source.
- Generated app adapter source is assembled from Go AST nodes, printed with
  `go/printer`, and normalized with `go/format`; contract adapter emitters do
  not use string line writing.
- `gowdk routes` includes routable `g:command` and `g:query` references as
  backend endpoint metadata with contract binding details.
- `gowdk build` writes `openapi.json` for the routable web surface and
  `asyncapi.json` for contract integration events. The AsyncAPI report excludes
  domain and presentation events by default. Its CloudEvents mapping is:
  `type` = contract event type, `source` = the application/module boundary,
  `id` = event envelope identifier supplied by the transport, `time` = event
  envelope time supplied by the transport, and `datacontenttype` =
  `application/json`. Imported event payload structs currently emit shallow
  named schemas; #315 tracks imported payload field expansion.
- Command contract adapter IR includes the form method and either the literal
  form action or, for page-owned forms that omit `action`, the page route.
- Page-owned query contract adapter IR includes `GET` plus the page route.
- Page-owned generated query routes use JSON/query request negotiation so they
  do not replace normal static, SPA, or SSR page responses.
- Cross-package contract input field discovery remains planned.
- Retry backoff policy, split web/worker/cron binaries, and managed deployment
  recipes remain planned. Split worker generation is blocked on stable local
  command/query adapters, generated registry/replay helper usage, durable
  outbox/broker policy, retry/backoff semantics, and deployment supervision
  docs. Cron generation is blocked on the same runtime role policy plus
  schedule ownership, overlap prevention, failure reporting, and restart
  behavior. M6 does not make a production-readiness claim for either path.

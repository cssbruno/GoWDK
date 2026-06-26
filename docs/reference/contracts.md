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

Enable compiler integration with:

```go
Addons: []gowdk.Addon{
    contractsaddon.Addon(),
}
```

Enable `addons/realtime` alongside `addons/contracts` when the app wants an
explicit config feature for browser presentation-event fanout.

The runtime registry, generated `g:command` / `g:query` adapters, and generated
worker helper APIs are implemented for the current contract-driven runtime
slice.

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

The supported milestone-14 path is local-first: one generated binary can serve
the page, execute `g:command` and `g:query` web adapters through the web role,
and replay captured backend events through local runtime helpers. Worker and
cron roles can run the same generated registry helpers from user-owned
commands or generated standalone role binaries.

## Standalone Worker And Cron Binaries

`Build.Targets` can generate role-only Go apps and compile them to binaries
without embedding the web output:

```go
Build: gowdk.BuildConfig{
	Targets: []gowdk.BuildTargetConfig{
		{
			Name:         "contracts-worker",
			WorkerApp:    ".gowdk/worker",
			WorkerBinary: "bin/worker",
			Worker: gowdk.ContractWorkerConfig{
				EventSource: gowdk.ServiceRef{
					ImportPath: "github.com/acme/clinic/workers",
					Function:   "EventSource",
				},
			},
		},
		{
			Name:       "contracts-cron",
			CronApp:    ".gowdk/cron",
			CronBinary: "bin/cron",
			Cron: gowdk.ContractCronConfig{
				Jobs: []gowdk.ContractCronJobConfig{{
					Type:            "patients.SyncPatients",
					Schedule:        "@every 15m",
					OverlapPolicy:   "skip",
					MissedRunPolicy: "skip",
				}},
			},
		},
	},
}
```

`Worker.EventSource` is required and must name a provider function with
signature `func() (contracts.EventSource, error)`. `Worker.SeenStore` and
`Worker.Backoff` are optional providers for `contracts.SeenStore` and
`contracts.EventWorkerBackoff`. Generated workers install SIGINT/SIGTERM
context cancellation and run event subscribers with the `worker` role.

Cron targets require explicit jobs. `Type` can be a scanned type name,
`package.Type`, or full `import/path.Type` when needed to disambiguate.
Schedules currently support `@once` and `@every <duration>`, and the supported
overlap and missed-run policy is `skip`. Generated cron binaries run selected
jobs with the `cron` role and zero-value job input.

Ad hoc builds can use the same surface:

```sh
gowdk build --worker-app .gowdk/worker --worker-bin bin/worker
gowdk build --cron-app .gowdk/cron --cron-bin bin/cron
```

Ad hoc worker builds still require `Build.Worker` provider configuration from
`gowdk.config.go`, and ad hoc cron builds require configured
`Build.Cron.Jobs`; the flags choose output locations only. `--worker-bin`
requires `--worker-app`, and `--cron-bin` requires `--cron-app`.

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

`g:command` is the reactive, contract-governed write. Any page that declares one
— build-time or request-time (`server {}`, SSR, hybrid) — ships the small client
runtime, which intercepts the submit, posts the command in the background, and
applies a single-flight region refresh: the generated adapter computes which
`g:query` regions the command's domain events invalidate and names them in the
`X-GOWDK-Queries` response header, so the submitter's regions update immediately
without waiting for the realtime fanout that refreshes every other connected
client.

For a parameterless region whose data comes from the page's `server {}`, the
adapter goes one step further and renders the invalidated region inline — true
single-flight. It returns a `{ result, patches: [{ query, html }] }` envelope
(signalled by the `X-GOWDK-Patches` response header) and the client swaps the
region HTML directly, with no second page fetch. Regions that need route context
the command request lacks (a dynamic route param) stay in the `X-GOWDK-Queries`
header only and the client refetches them — the same path used when JavaScript
re-runs the page render. The raw command result body is preserved whenever no
region renders, so non-browser callers are unaffected.

The typed result rides on the `gowdk:command-success` event for optional
optimistic UI. With realtime configured, the same invalidation also fans out over
SSE to other clients (`g:subscribe` / invalidated `g:query` regions), reusing the
client's region-swap routine so the embedded and fanned-out paths converge.

Two caveats the `ssr_command_no_client` warning surfaces. First, with client
JavaScript disabled a bare submit still navigates to the adapter's JSON — use a
`g:post` action handler returning a `response.Response` (for example
`response.RedirectTo`) when a no-JavaScript write path matters. Second, a
`g:command` with no `g:query` region for it to refresh is a non-reactive write:
it only fires `gowdk:command-success`. The warning fires only in that second
case (a request-time command with no read region); add a bound `g:query` region
or switch to `g:post`.

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

Each captured `EventEnvelope` contains a stable event ID, optional
`traceparent`, optional normalized `Audience` labels, event category, Go type
name, and typed value. Capturing does not run event subscribers.

For browser-facing presentation events that should only reach a tenant, user,
session, or other server-owned audience, use:

```go
err := contracts.EmitPresentationForAudience(
    ctx,
    PatientNotice{ID: id},
    "tenant:"+tenantID,
    "user:"+userID,
)
```

Audience labels are delivered through the same event envelope and preserved by
the file outbox. Dependency-free SSE fanout uses those labels only when the app
installs an audience-aware hub; see `docs/reference/realtime.md`.

For tests, `runtime/testkit` wraps this path with an in-memory registry helper
and typed event assertions. See `docs/reference/testing.md` and
`examples/contracts/patients/contracts_test.go`.

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
`contracts.EventSource`. It stores captured envelopes as JSON Lines records,
rewrites pending and dead-letter files through temp-file replacement, decodes
records through explicitly registered decoders, removes records only after
worker `Ack`, and keeps records after `Nack` for retry. Nack records the attempt
count, last attempt time, and last error in the durable record. It is useful for
local development, small single-host deployments, and tests.
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
shared := gowdkapp.ContractRegistry()
fresh := gowdkapp.NewContractRegistry()
err := gowdkapp.RunContractEventWorker(ctx, source)
err = gowdkapp.RunContractEventWorkerWithOptions(
    ctx,
    source,
    contracts.WithEventWorkerBackoff(backoff),
)
err = gowdkapp.RunContractEventWorkerWithSeenStore(ctx, source, seen)
err = gowdkapp.RunContractEventWorkerWithSeenStoreAndOptions(
    ctx,
    source,
    seen,
    contracts.WithEventWorkerBackoff(backoff),
)
```

`ContractRegistry` returns the generated app's shared in-process registry.
Generated web command/query routes use the same registry, and lifecycle
services can read it from `runtime/app.ServiceContext.Values` with key
`runtime/app.ServiceValueContractRegistry`. `NewContractRegistry` creates a
fresh registry using the scanned registration functions. `RunContractEventWorker`
replays an `EventSource` through a fresh registry with the worker role.
`RunContractEventWorkerWithSeenStore` uses the same worker role and skips
duplicate event IDs through the provided `contracts.SeenStore`. The
`WithOptions` variants pass runtime worker options, including nacked-batch
backoff, through to `runtime/contracts`.

These helpers are deliberately local process APIs. Use them from the generated
binary, a generated role binary, a user-owned worker or cron command, or a test
fixture. Supervisor configs, queue topology, and deployment recipe starters are
platform tooling, not part of the milestone-14 runtime contract.

## Worker Backoff

By default, an event worker immediately asks the source for another batch after
the source accepts `Nack`. Pass `contracts.WithEventWorkerBackoff` when a
worker should wait after nacked subscriber delivery:

```go
backoff := func(retry contracts.EventWorkerRetry) time.Duration {
    delay := 250 * time.Millisecond
    for i := 1; i < retry.Attempt && delay < 5*time.Second; i++ {
        delay *= 2
    }
    if delay > 5*time.Second {
        return 5 * time.Second
    }
    return delay
}

err := gowdkapp.RunContractEventWorkerWithSeenStoreAndOptions(
    ctx,
    source,
    seen,
    contracts.WithEventWorkerBackoff(backoff),
)
```

Use `contracts.ConstantEventWorkerBackoff(duration)` for a fixed delay.
Backoff runs only after subscriber replay fails and the `EventSource` accepts
`Nack`; ack failures, receive failures, missing `Nack`, and context
cancellation still return errors. Durable adapters still own their persistent
attempt counters, dead-letter files, pending-message behavior, and operational
retry policy.

Dependency-free adapters:

- `runtime/contracts/fileoutbox` stores JSON Lines records on disk and
  implements both `Outbox` and `EventSource`. Each record has its own durable
  record ID plus the event envelope ID used by worker deduplication.
- `runtime/contracts/membroker` provides an in-memory `Broker` and
  `EventSource` for tests, local development, and single-process apps.
- `runtime/contracts/sse` provides an `http.Handler` and
  `PresentationFanout` for server-sent browser presentation events.
  `addons/realtime` re-exports this dependency-free SSE hub as `NewSSE`, with
  buffer, retry, replay, audience, and audience-revocation support.

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

See `docs/reference/realtime.md` for the transport choice, config setup, and
deployment caveats.

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
custom JetStream adapter when events must survive worker downtime. When a batch
drain encounters a later malformed message after already decoding earlier
messages, the adapter returns the decoded events so they can still be
dispatched.

### SSE Presentation Fanout

Use SSE when the app needs one-way browser presentation events:

```go
import (
    "net/http"

    "github.com/cssbruno/gowdk/runtime/contracts"
    "github.com/cssbruno/gowdk/runtime/contracts/sse"
)

hub := sse.New(
    sse.WithRetryMillis(2000),
    sse.WithReplayLimit(128),
)
http.Handle("/gowdk/events", hub)

gowdkapp.RegisterContractEventSink(
    contracts.PresentationFanoutCommandEventSink(hub),
)
```

The browser receives `event: gowdk-presentation` messages whose `data` value is
the JSON `contracts.EventEnvelope`. The hub emits SSE `id:` lines from
`EventEnvelope.ID`, advertises a browser reconnect delay with `retry:`, and can
replay a bounded in-memory window after `Last-Event-ID`. Domain and integration
events are ignored.

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
  cause)` can opt into an explicit client-safe status and message. Form parse,
  oversized body, CSRF, and typed input decode failures use the same JSON error
  shape.
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

## `.gwdk` Realtime Subscriptions

Use `g:subscribe` beside `g:query` to bind a query-owned region to a
browser-facing presentation event:

```html
<section g:query="patients.GetPatientPage" g:subscribe="patients.PatientNotice">
  <h1>Patients</h1>
</section>
```

Current behavior:

- Renders `data-gowdk-subscribe="patients.PatientNotice"` and a validated
  `data-gowdk-subscribe-type` marker beside the `data-gowdk-query` marker.
- Adds a subscription record to
  `internal/gwdkir.Program.RealtimeSubscriptions`.
- Records query and event aliases, imported package paths when declared with
  `.gwdk import`, local query/event types, owner metadata, guards, and exact
  source spans.
- Requires `realtime.Addon()` in project config.
- `gowdk check` and CLI `gowdk build` fail when the event reference is missing,
  linked to an invalid Go handler signature, registered as a domain or
  integration event, or bound only to non-web runtime roles.
- `gowdk build` adds `realtime_subscription` events with status and source
  line/column to `gowdk-build-report.json`.
- Generated apps with bound subscriptions expose `RealtimeEventsPath`, mount
  `/_gowdk/realtime/events`, and stream only subscribed presentation event
  types through the dependency-free SSE fanout by default.
- Generated realtime streams inherit subscribed page guards. The generated
  handler chooses page guards from `?path=...` or the same-origin referer path
  when available, otherwise it fails closed by requiring the union of guarded
  subscriptions before opening the SSE response.
- `RegisterRealtimeFanout(realtime.PresentationFanout)` can replace the
  generated fanout for app-owned transport setup.
- Generated `gowdk.js` connects subscribed pages to the SSE stream and applies
  explicit `replaceHTML` patches from presentation event payloads to matching
  query-owned regions.
- Requires package-qualified Go references such as
  `patients.PatientNotice`.
- Must be on the same element as `g:query`; unbounded subscriptions are
  rejected.

Only explicit version-1 `replaceHTML` client patches are supported today. The
dependency-free SSE adapter sends a `retry: 1000` directive for browser
EventSource reconnects by default, supports `WithRetryMillis`,
`WithReplayLimit`, bounded per-client buffers, and `RevokeAudience` for active
session stream revocation. Events are dropped for clients whose buffers are
full rather than blocking command execution. Richer patch shapes remain a
separate follow-up piece.

## Query Invalidations

Use `contracts.RegisterInvalidation[event, query]` in Go when a domain event
should refresh query-owned regions:

```go
func Register(registry *contracts.Registry) {
	contracts.RegisterQuery[GetPatientPage, PatientPageData](registry, LoadPatientPage, contracts.RoleWeb)
	contracts.RegisterCommand[CreatePatient, CreatePatientResult](registry, HandleCreatePatient, contracts.RoleWeb)
	contracts.RegisterDomainEvent[PatientCreated](registry, SendWelcomeEmail, contracts.RoleWorker)
	contracts.RegisterInvalidation[PatientCreated, GetPatientPage](registry)
}
```

Current behavior:

- Scans invalidation edges beside normal contract registrations.
- Rejects edges that name an unknown query, an unknown domain event, or a
  domain event no scanned command emits.
- Joins validated edges with bound `g:query` references into
  `internal/gwdkir.Program.QueryInvalidations`.
- Requires `realtime.Addon()` when a bound query uses generated invalidation
  refresh.
- `gowdk build` adds `query_invalidation` events with status and source
  line/column to `gowdk-build-report.json`.
- `gowdk graph` prints `invalidates` edges from domain events to queries.
- Generated HTML renders validated `data-gowdk-query-type` markers for
  invalidated query regions.
- Generated command adapters emit a `gowdk.query.invalidate` presentation event
  after successful command event dispatch when captured domain events
  invalidate bound queries.
- Generated apps with eligible standalone public SSR/hybrid query regions mount
  `/_gowdk/realtime/query-refresh`; generated `gowdk.js` asks that endpoint for
  `{query, html}` patches first, then refetches the current document for any
  remaining non-subscribed query regions. Regions with `g:subscribe` are left
  to explicit presentation patches.

Invalidations are explicit Go metadata, not compiler inference from handler
bodies. Fragment/API-specific query execution and richer refresh policies remain
future work.

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
  `contract_reference_*` diagnostics in `gowdk check`, in LSP dirty-buffer
  diagnostics, and stop CLI builds. Diagnostic suggestions point to
  `gowdk contracts list` and `gowdk contracts graph` when scanned registration
  state needs inspection.
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
- Contract scan reports include exported JSON fields for supported local and
  imported command/query result structs and integration-event payload structs
  when those structs can be resolved. Unsupported imported payload/result
  shapes stay as shallow named schemas with the Go type marker.
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
  atomic file replacement, nack retry metadata, and an opt-in dead-letter file.
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
- Generated contract packages expose `ContractRegistry`, `NewContractRegistry`, and
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
  `application/json`. Resolvable local and imported integration-event payload
  structs contribute JSON-field schemas; unsupported or unresolvable imported
  payloads emit shallow named schemas with the Go type marker.
- Command contract adapter IR includes the form method and either the literal
  form action or, for page-owned forms that omit `action`, the page route.
- Page-owned query contract adapter IR includes `GET` plus the page route.
- Page-owned generated query routes use JSON/query request negotiation so they
  do not replace normal static, SPA, or SSR page responses.
- Cross-package contract input field discovery remains planned.
- Standalone worker and cron binary generators cover the first platform tooling
  slice. Schedule ownership beyond `@once` / `@every`, overlap prevention
  beyond `skip`, durable retry operations, failure reporting, restart behavior,
  and production supervision stay app-owned.

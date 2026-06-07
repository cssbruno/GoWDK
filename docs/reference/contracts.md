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
registry:

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

`Metadata.ObservationLabels()` returns the stable contract labels: kind, event
category, contract type name, result type name, role, roles, and handler count
when known. `EventEnvelope.ObservationLabels()` returns the event kind,
category, and captured event contract type. `ContractName[T]()` returns the same
Go contract type name used by metadata and event envelopes.
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

Each captured `EventEnvelope` contains the event category, Go type name, and
typed value. Capturing does not run event subscribers.

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

Subscriber handlers must be idempotent for any durable delivery adapter. A
worker can crash after a subscriber side effect but before `Ack`, or an adapter
can retry after `Nack`. Use a stable domain key, event id, outbox record id, or
application-level dedupe table to make repeated deliveries safe. GOWDK Runtime
does not hide retries behind generated JavaScript or browser state.

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
runtime role.

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
- Records literal form `method` and `action` as command adapter IR method/path.
- `gowdk build` links command references to scanned Go command registrations
  and adds `contract_reference` events with status and source line/column to
  `gowdk-build-report.json`. Command events include method/path when present.
- Generated apps register the scanned package registration function once in a
  local `runtime/contracts.Registry`, route the form method/action through the
  backend router, execute the command with `ExecuteCommandForRole(...,
  contracts.RoleWeb, input)`, dispatch emitted backend events after command
  success, and return the command result as no-store JSON.
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
- Captured event envelopes can be replayed later with
  `PublishEnvelope`, `PublishEnvelopes`, and role-filtered variants.
- `runtime/contracts/fileoutbox` provides a dependency-free JSON Lines adapter
  that implements `contracts.Outbox` and `contracts.EventSource`, including
  nack retry metadata and an opt-in dead-letter file.
- External broker adapters can implement the dependency-free `Broker`
  interface and receive captured envelopes through `ExecuteCommandToBroker` or
  `PublishEventsToBroker`.
- Realtime adapters can implement the dependency-free `PresentationFanout`
  interface and receive only presentation envelopes through
  `ExecuteCommandToPresentationFanout` or `SendPresentationEventsToFanout`.
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
- Command contract adapter IR includes literal form method/path.
- Page-owned query contract adapter IR includes `GET` plus the page route.
- Page-owned generated query routes use JSON/query request negotiation so they
  do not replace normal static, SPA, or SSR page responses.
- Cross-package contract input field discovery remains planned.
- Database-backed outbox implementations, concrete broker adapters, retry
  backoff policy, split web/worker/cron binaries, and concrete SSE/WebSocket
  adapters are planned.

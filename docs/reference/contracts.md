# Contracts

`runtime/contracts` is the first runtime slice of GOWDK's contract-driven
backend model. It is usable from normal Go today. `.gwdk` command references
are discoverable in compiler IR and build reports; generated HTTP adapters are
still planned.

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
- `gowdk build` links command references to scanned Go command registrations
  and adds `contract_reference` events with status and source line/column to
  `gowdk-build-report.json`.
- `gowdk check` and CLI `gowdk build` fail when a command reference is missing
  or linked to an invalid Go handler signature.
- Requires a package-qualified Go reference such as `patients.CreatePatient`.
- Must not be combined with `g:post`.

This is metadata and validation only. Generated command adapters, typed form
decoding, and CSRF wiring are still planned.

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
- `gowdk build` links query references to scanned Go query registrations and
  adds `contract_reference` events with status and source line/column to
  `gowdk-build-report.json`.
- `gowdk check` and CLI `gowdk build` fail when a query reference is missing or
  linked to an invalid Go handler signature.
- Requires a package-qualified Go reference such as `patients.GetPatientPage`.
- Must not be combined with `g:post` or `g:command` on the same form.

This is metadata and validation only. Generated query adapters and request-time
query execution are still planned.

Templates must not declare backend facts:

```html
<!-- rejected -->
<form g:event="PatientCreated">
```

Use `g:on:*` for local UI/component events and `g:command` for backend intent.

## Current Limits

- Generated adapters do not execute command/query contracts yet.
- `.gwdk` command/query reference linking is first-slice only: it matches
  package name plus local contract type name, such as
  `patients.CreatePatient` or `patients.GetPatientPage`.
- Form-local `g:command` references and element-local `g:query` references
  include exact source line and column in IR and build reports.
- Missing or invalid command/query references produce `contract_reference_*`
  diagnostics in `gowdk check` and stop CLI builds.
- Other contract diagnostics do not all have exact source spans yet.
- `gowdk contracts`, `gowdk list commands|queries|events|jobs`, `gowdk graph`,
  and `gowdk trace <contract>` can scan Go AST registration calls today.
- Contract scan reports include first same-file `go/types` diagnostics for
  command, query, event, and job handler signatures.
- Contract scan reports duplicate command owner registrations.
- `gowdk check` and CLI `gowdk build` fail on contract scan diagnostics such
  as invalid handler signatures and duplicate command owners.
- `gowdk graph` detects command-emitted events when command handlers call
  `contracts.EmitDomain`, `contracts.EmitIntegration`, or
  `contracts.EmitPresentation` with a visible event type.
- `gowdk trace <contract>` reports a single command/query/event/job, command
  emitted events, event subscribers, source locations, handlers, and roles.
- `runtime/contracts` can capture command-emitted events as `EventEnvelope`
  values and pass them to a dependency-free `Outbox` interface without
  dispatching subscribers.
- Full package graph validation and imported handler validation are planned.
- Durable outbox implementations, broker adapters, split web/worker/cron
  binaries, and realtime SSE/WebSocket fanout are planned.

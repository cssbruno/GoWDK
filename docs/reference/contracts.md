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
- Other contract diagnostics do not all have exact source spans yet.
- `gowdk contracts`, `gowdk list commands|queries|events|jobs`, and
  `gowdk graph` can scan Go AST registration calls today.
- Contract scan reports include first same-file `go/types` diagnostics for
  command, query, event, and job handler signatures.
- Contract scan reports duplicate command owner registrations.
- `gowdk graph` detects command-emitted events when command handlers call
  `contracts.EmitDomain`, `contracts.EmitIntegration`, or
  `contracts.EmitPresentation` with a visible event type.
- Full package graph validation and imported handler validation are planned.
- `gowdk trace` is planned.
- Durable outbox, broker adapters, worker roles, cron roles, and realtime
  SSE/WebSocket fanout are planned.

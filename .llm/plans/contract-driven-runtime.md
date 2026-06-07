# Implementation Plan: Contract-Driven Runtime

## Context

Spec: `.llm/features/contract-driven-runtime.md`

Related plans:

- `.llm/plans/compiler-contract-completion.md`
- `.llm/plans/deep-go-package-integration.md`
- `.llm/plans/go-native-adapter-boundary.md`
- `.llm/plans/gowdk-world-roadmap.md`

The implementation must fit the current GOWDK direction:

```text
.gwdk file
  -> GOWDK parser
  -> GOWDK AST
  -> GOWDK analyzer
  -> generated normal Go code
  -> go/format
  -> go build
```

```text
.go files
  -> standard go/parser
  -> standard go/ast
  -> standard go/types
  -> validate exported handlers/types
```

The contract runtime is not a replacement for static, SPA, SSR, or hybrid page
rendering. It is the backend contract model that pages, endpoints, workers, and
cron jobs can use.

The trust model is:

```text
UI event
  -> command/query
  -> backend validation, authorization, persistence
  -> backend-owned domain event
  -> subscribers, workers, integration events, presentation events
```

The browser can request work with commands and read state with queries. It
cannot create backend facts. Domain events are emitted only after the backend
has made the state change real.

## Assumptions

- GOWDK stays compile-first and Go-first.
- User application behavior stays in normal Go packages.
- Generated Go stays adapter glue emitted through Go AST/printer/format.
- Existing `act` and `api` declarations remain supported while the contract
  layer is designed.
- Runtime route kinds stay page/render concepts: static/build-time SPA, SSR,
  and hybrid.
- Commands, queries, events, and jobs are backend contracts, not page route
  kinds.
- Browser UI events are local interactions, not backend domain events.
- Domain and integration events are backend-owned facts.
- In-process dispatch is the default; durable queues and brokers are optional
  adapters.

## Proposed Changes

### Add A Public Contract Registry

Add a small runtime package for typed registration:

```go
func RegisterPatients(r gowdk.Registry) {
    contracts.RegisterQuery[PatientPageQuery, PatientPageData](r, LoadPatientPage)
    contracts.RegisterCommand[CreatePatient, CreatePatientResult](r, HandleCreatePatient)
    contracts.RegisterDomainEvent[PatientCreated](r, SendWelcomeEmail)
    contracts.RegisterDomainEvent[PatientCreated](r, WriteAuditLog)
    contracts.RegisterJob[SyncPatients](r, RunPatientSync)
}
```

Go does not support generic methods, so the first implemented API uses generic
functions over `*contracts.Registry` instead of `r.Command[T]` method syntax.

The first implementation can keep registration explicit and local:

- no package-wide reflection scanning;
- no function-name auto-discovery;
- no broker dependency;
- no global mutable bus hidden behind imports.

Names should be checked by convention:

- commands use imperative names, such as `CreatePatient`;
- queries use read-oriented names, such as `GetPatientPage`;
- domain and integration events use past-tense names, such as
  `PatientCreated`;
- vague contracts such as `ButtonClicked`, `FormSubmitted`, `PatientChanged`,
  `ThingUpdated`, and `DoStuff` should be rejected by docs first; the scanner
  now rejects the first browser-UI and vague `Changed` event-name anti-patterns
  as hard diagnostics because check/build currently fail on all scan
  diagnostics.

### Add Contract Metadata To Compiler IR

Extend analyzer output with typed contract metadata:

- package path and package name;
- contract kind: query, command, event, job;
- event category when applicable: domain, integration, or presentation;
- contract type name;
- result type name when applicable;
- handler symbol;
- source span for `.gwdk` references;
- Go source location for registered handlers when available;
- binding status: bound, missing, duplicate owner, unsupported signature;
- runtime roles that can execute the contract.

This must consume existing package and endpoint metadata instead of creating a
parallel compiler pipeline.

### Add Command/Query Binding From `.gwdk`

Design the syntax after endpoint IR is stable. Candidate syntax:

```gwdk
command patients.CreatePatient POST "/patients"
query patients.PatientPageQuery
```

or form-local:

```html
<form g:command="patients.CreatePatient">
  <input name="name">
  <button>Create patient</button>
</form>
```

The implementation must decide one syntax and document rejected alternatives.
Until then, existing `act` and `api` remain the stable endpoint declarations.

Templates must not expose a `g:event="PatientCreated"` shape for domain events.
The browser may use local UI events and realtime presentation events, but
domain and integration events remain backend-owned.

### Add Local Runtime Dispatch

Implement the default runtime as direct in-process dispatch:

- command owner lookup;
- query handler lookup;
- event subscriber list;
- job handler lookup;
- ordered middleware/hooks where needed;
- context propagation;
- panic boundaries and no-store request-time responses for HTTP-exposed
  contracts.

Events must not be required for command success. A command can succeed and then
dispatch local events. Durable/eventual delivery belongs to optional outbox
work.

For durable delivery, the later outbox path must preserve this order:

```text
start transaction
apply state change
store domain event in outbox
commit transaction
publish event from worker
```

This avoids publishing `PatientCreated` before the patient record exists.

### Add Optional Runtime Roles

Allow one codebase to build multiple runtime roles:

```text
gowdk-web    -> pages, forms, SSR, commands exposed over HTTP
gowdk-worker -> event subscribers and async jobs
gowdk-cron   -> scheduled jobs
gowdk-api    -> external API over selected contracts
gowdk-admin  -> internal/admin tools over selected contracts
```

Small apps must still run as one binary.

### Add CLI Visibility

Add contract-oriented commands:

```sh
gowdk list commands
gowdk list queries
gowdk list events
gowdk contracts
gowdk contracts --json
gowdk graph
gowdk trace
gowdk check
```

Example report:

```text
COMMAND patients.CreatePatient
  owner: patients.HandleCreatePatient
  used by: /patients/new form
  emits: patients.PatientCreated
  roles: web

EVENT patients.PatientCreated
  subscribers:
    - patients.SendWelcomeEmail
    - audit.WritePatientCreatedLog
    - search.IndexPatient
  delivery:
    - in-process dev
    - outbox worker prod
  roles: worker
```

### Keep SPA Static-First

SPA enhancement can submit commands or load fragments, but generated JavaScript
must not own:

- app routing;
- auth or authorization;
- business rules;
- database access;
- server validation;
- command/action behavior;
- global app state;
- page loading policy;
- cache and revalidation policy.

The no-JavaScript baseline should remain normal form POSTs or direct URLs
wherever the contract is HTTP-exposed.

## What Changes

- Add contract metadata to IR and build reports.
- Add a public registry API for query, command, event, and job handlers.
- Add compiler validation for contract references and ownership.
- Add generated adapters that call registered command/query handlers from
  `.gwdk` forms, pages, fragments, endpoints, or SSR loads.
- Add local event dispatch for domain events.
- Add presentation-event metadata for realtime browser updates.
- Add optional worker/cron runtime role wiring.
- Add contract graph CLI output.
- Add docs that show domain-level commands/events and reject UI-event noise.

## What Does Not Change

- Static/build-time pages remain the default output.
- SSR remains explicit and non-default.
- Hybrid remains SPA output until an explicit request-time branch is declared.
- Existing `act` and `api` endpoint declarations are not removed by this plan.
- Route metadata does not become command/event/job metadata.
- Frontend code does not emit domain or integration events.
- Generated Go does not contain user business logic.
- Generated JavaScript does not own trusted app behavior.
- Core does not require npm, a JavaScript framework, Redis, Kafka, NATS,
  RabbitMQ, Temporal, or an ORM.
- GOWDK does not invent a custom Go compiler for this feature.

## Files Expected To Change

- Public API:
  - `registry.go` or a new `contracts` package.
  - `runtime/app`
  - optional `runtime/contracts`
- Compiler:
  - `internal/gwdkast`
  - `internal/gwdkanalysis`
  - `internal/gwdkir`
  - `internal/compiler`
  - `internal/gotypes`
  - `internal/appgen`
  - `internal/buildgen` where reports/manifests need contract metadata.
- CLI:
  - `cmd/gowdk`
  - route/build report output packages.
- Docs:
  - `README.md`
  - `docs/product/vision.md`
  - `docs/product/requirements.md`
  - `docs/product/roadmap.md`
  - `docs/engineering/architecture.md`
  - `docs/language/`
  - `docs/reference/`
- Examples:
  - `examples/login`
  - a new small domain example such as `examples/contracts` after the runtime
    API exists.

## Data And API Impact

- New public typed registration API.
- New manifest/build-report fields for contract metadata.
- New CLI output for contract graph inspection.
- New generated adapter code paths for command/query HTTP exposure.
- New optional metadata for presentation events sent to the browser.
- Existing endpoint and route JSON should remain backward compatible unless a
  version bump is explicitly documented.

## Implementation Checklist

### Phase 0: Design Lock

- [x] Decide first API shape: `runtime/contracts.Registry` plus generic
      registration and execution functions.
- [x] Decide whether commands return events directly, through a result field, or
      through a dispatcher on context: first slice uses `Emit*` on the command
      context, then dispatches or captures recorded events after success.
- [x] Decide first event representation: domain, integration, and presentation
      categories in registry metadata.
- [x] Decide first `.gwdk` syntax for command binding:
      form-local `g:command="pkg.Command"`.
- [x] Decide first `.gwdk` syntax for query binding:
      element-local `g:query="pkg.Query"`.
- [x] Explicitly reject frontend-originated domain event syntax.
- [x] Decide first runtime role identifiers: `web`, `worker`, `cron`, `api`,
      and `admin`; split-binary config shape remains planned.
- [ ] Add an ADR if the syntax or runtime role model is hard to reverse.

### Phase 1: Runtime Registry

- [x] Add typed query registration.
- [x] Add typed command registration with one-owner enforcement.
- [x] Add typed event subscriber registration.
- [x] Separate domain events, integration events, and presentation events in
      registry metadata.
- [x] Add typed job registration.
- [x] Add in-process dispatch helpers.
- [x] Ensure local domain events are emitted only after command success.
- [x] Add context propagation.
- [x] Add duplicate command owner errors.
- [x] Add unsupported signature errors.
- [x] Add unit tests for registration and dispatch.

### Phase 2: Go Contract Discovery And Validation

- [x] Add first `go/parser`, `go/ast`, and `go/types` scan pass for local
      registration calls and same-file handler signatures.
- [x] Expand Go validation across full local package files.
- [x] Expand Go validation across full local package files and imported
      handler symbols resolved by `go/types`.
- [x] Validate local exported contract structs and handler functions.
- [x] Detect duplicate command owner registrations in scanned Go files.
- [x] Surface same-file handler signature and duplicate command owner scan
      diagnostics through `gowdk check` and CLI `gowdk build`.
- [x] Validate local command input and result types.
- [x] Validate local query input and result types.
- [x] Validate first same-file command, query, event subscriber, and job handler
      signatures.
- [x] Validate event subscriber signatures across full local packages and
      imported handler symbols resolved by `go/types`.
- [x] Validate first browser-UI and vague `Changed` event-name anti-patterns.
- [ ] Expand event category and naming convention validation.
- [x] Validate job handler signatures across full local packages and imported
      handler symbols resolved by `go/types`.
- [ ] Detect import cycles caused by feature packages importing generated app
      output.
- [ ] Cache package inspection by package path/directory.

### Phase 3: Compiler IR Integration

- [x] Add first command-reference source metadata through `internal/gwdkast`
      view body starts and view attribute offsets.
- [x] Lower first form-local command references through
      `internal/gwdkanalysis`.
- [x] Add first command-reference metadata to `internal/gwdkir`.
- [x] Link first `.gwdk` command references to scanned Go command metadata.
- [x] Add first element-local query references through `internal/gwdkanalysis`.
- [x] Add first query-reference metadata to `internal/gwdkir`.
- [x] Link first `.gwdk` query references to scanned Go query metadata.
- [ ] Expand command linking across import aliases and full package paths.
- [x] Reject `.gwdk` domain-event emission from templates through `g:event`.
- [ ] Add presentation-event references for realtime UI notifications only
      after backend event metadata exists.
- [ ] Preserve existing endpoint metadata.
- [x] Add first missing/invalid command/query reference status in build
      reports.
- [x] Add diagnostics for unresolved or invalid command/query references.
- [x] Add first duplicate command owner diagnostics from Go contract scans.
- [x] Add exact `.gwdk` source spans for form-local command references and
      element-local query references in IR and build reports.
- [ ] Add source spans for all contract diagnostics.

### Phase 4: Generated Adapter Integration

- [x] Add first adapter IR metadata for command/query contract exposure.
- [x] Extend command adapter IR with HTTP method/path from form template
      directives.
- [x] Extend query adapter IR with first page-route request-time source
      metadata.
- [ ] Generate Go adapter code with Go AST/printer/format only.
- [ ] Wire command form submission to generated request decoding.
- [ ] Wire query execution for request-time page/fragment/API needs.
- [ ] Keep CSRF validation before command decoding for form POSTs.
- [ ] Keep guards/rate limits before user command/query handlers when exposed
      through HTTP.
- [ ] Keep missing/unsupported contract responses explicit and no-store.
- [ ] Ensure command results and presentation events are browser-facing outputs,
      not backend truth inputs.

### Phase 5: Event And Job Runtime Roles

- [x] Add single-binary default role behavior.
- [x] Add runtime role filtering helpers for commands, queries, events, jobs,
      and metadata.
- [ ] Add worker role binary wiring for event subscribers and queued jobs.
- [ ] Add cron role binary wiring for scheduled jobs.
- [x] Add presentation-event fanout hook for SSE/WebSocket adapters.
- [x] Add role filtering so web binaries do not accidentally run worker-only
      subscribers unless configured.
- [x] Add event worker source loop with ack/nack and context cancellation.
- [ ] Add graceful shutdown for generated background handlers and cron jobs.
- [x] Add event-envelope replay helpers for worker/outbox delivery code.
- [x] Document failure behavior for subscriber errors.

### Phase 6: Optional Reliability Adapters

- [x] Define broker adapter interface.
- [x] Define optional outbox interface without choosing an ORM or database.
- [x] Define transaction/outbox ordering rules for domain events.
- [x] Add a dependency-free file outbox adapter for local durable JSON Lines
      storage and worker replay.
- [x] Add first explicit dead-letter policy for the file outbox.
- [x] Define retry and idempotency guidance for subscribers.
- [x] Add docs for when in-process dispatch is enough.
- [x] Add docs for when outbox/queue/broker support is needed.
- [x] Keep all external adapters outside core dependencies.

### Phase 7: CLI And Observability

- [x] Add `gowdk contracts`.
- [x] Add `gowdk list commands`.
- [x] Add `gowdk list queries`.
- [x] Add `gowdk list events`.
- [x] Add `gowdk list jobs`.
- [x] Add `gowdk contracts --json`.
- [x] Add `gowdk graph`.
- [x] Add first static `gowdk trace` for scanned command/query/event/job
      contracts.
- [x] Link `gowdk routes` endpoints to contracts where applicable.
- [x] Include first command/query-reference metadata in build reports.
- [x] Add stable names for logs/metrics/traces.
- [x] Add tests for text and JSON output.

### Phase 8: Examples And Docs

- [ ] Document good domain commands, queries, events, and jobs.
- [ ] Document bad UI-event/noisy bus examples.
- [ ] Document that frontend UI events trigger commands and never emit domain
      events.
- [ ] Document domain, integration, and presentation event differences.
- [ ] Document the outbox ordering rule for durable domain events.
- [ ] Document SPA behavior and no-JavaScript fallback.
- [ ] Add a small contract example.
- [ ] Update login example only if it makes the flow clearer.
- [ ] Document migration from `act`/`api` to contract-backed endpoints only
      after the contract API is implemented.

## Tests

- Unit:
  - registry duplicate command owner detection;
  - typed dispatch success and failure paths;
  - event fan-out order and error behavior;
  - file outbox store, decode, ack, nack, and worker replay behavior;
  - domain event category and naming metadata;
  - presentation event metadata does not register as trusted input;
  - job registration and role filtering;
  - Go type validation for supported and unsupported signatures.
- Integration:
  - `.gwdk` command form calls a normal Go command handler;
  - command handler emits an event consumed by two subscribers;
  - command cannot publish a domain event before the state change succeeds;
  - frontend domain-event syntax is rejected;
  - presentation event can notify browser-facing realtime adapter;
  - same registrations run as one binary and split web/worker binaries;
  - missing command owner produces a diagnostic or explicit runtime response
    according to build mode;
  - generated adapters run guards, rate limits, CSRF, decoding, handler calls,
    and response writing in the correct order.
- End-to-end:
  - example app can create a domain record through a command;
  - direct no-JavaScript POST still works;
  - SPA-enhanced submit uses the same command semantics;
  - realtime presentation notification updates UI after backend domain event;
  - worker role can process a domain event without running web routes.
- Manual:
  - inspect `gowdk contracts` and `gowdk graph` output;
  - inspect generated Go and confirm it is adapter glue only;
  - confirm no broker service is required for the default example.

## Verification Commands

```sh
gofmt -w <changed-go-files>
go test ./...
go build ./cmd/gowdk
go run ./cmd/gowdk contracts --json
go run ./cmd/gowdk list commands
go run ./cmd/gowdk list events
go run ./cmd/gowdk graph
cd examples/contracts && go run ../../cmd/gowdk build
cd examples/contracts && go build .
```

## Rollback Plan

- Keep the runtime registry package isolated until `.gwdk` syntax and adapter
  generation are stable.
- If command/query syntax conflicts with existing endpoints, roll back only the
  syntax parser and keep runtime registration experiments behind docs or tests.
- If worker/cron roles complicate one-binary generation, keep single-binary
  dispatch and defer split roles.
- If broker/outbox adapter design expands core dependencies, remove adapters
  from core and keep only interfaces plus docs.
- If presentation events blur into trusted input, remove realtime fanout from
  the first slice and keep command/query flows only.

## Risks

- The bus can become noisy if UI events are allowed into backend contracts.
  Mitigation: document and diagnose domain-level contract naming and usage.
- A generic registry can hide control flow. Mitigation: require explicit
  registration and strong CLI graph output.
- Split binaries can create configuration drift. Mitigation: generate role
  metadata from one compiler IR.
- Event reliability expectations can exceed in-process dispatch. Mitigation:
  document default semantics and keep durable delivery as optional outbox work.
- Realtime notifications can be confused with domain events. Mitigation:
  separate presentation events in metadata, docs, and diagnostics.
- More public API too early can freeze weak names. Mitigation: ship the first
  slice as experimental until examples and CLI output prove the model.

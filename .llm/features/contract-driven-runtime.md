# Feature Spec: Contract-Driven Runtime

## Problem

GOWDK needs a larger backend architecture than page actions and APIs, but it
must not become a noisy event machine where every click, field change, or modal
state becomes backend behavior.

The durable model should be contract-driven first and event-driven second:

```text
Query   = read data or render a page
Command = ask the system to do something
Event   = something already happened
Job     = background or scheduled work
```

This keeps GOWDK Go-first. Normal Go owns business behavior. `.gwdk` declares
web usage. Generated code remains adapter glue.

The trust boundary is:

```text
Frontend -> commands and queries
Backend  -> domain and integration events
Frontend <- results and presentation events
```

The frontend produces UI events such as clicks, submits, inputs, and changes.
Those UI events can trigger commands or queries. They must not produce domain
events. Domain events are backend-owned facts emitted only after validation,
authorization, persistence, and transaction success.

## Goals

- Define typed Go contracts for queries, commands, events, and jobs.
- Keep commands single-owner and events multi-subscriber.
- Keep domain events backend-owned.
- Keep the default runtime local and in-process.
- Allow larger apps to split web, worker, cron, API, and admin binaries without
  rewriting domain code.
- Make contract usage visible through CLI reports and generated metadata.
- Let SPA/static pages invoke server commands and queries without making
  generated JavaScript own routing, auth, validation, business rules, global
  state, or page loading policy.
- Layer this model on top of the existing AST, analyzer, IR, endpoint metadata,
  generated adapter IR, and runtime-kit work.

## Non-Goals

- Branding GOWDK as an event-driven framework.
- Treating UI events as backend bus messages.
- Letting the frontend emit domain or integration events.
- Replacing current page render modes with command/query/event/job route kinds.
- Requiring Redis, Kafka, NATS, RabbitMQ, Temporal, or any external broker.
- Generating user domain services, repositories, auth, validation, or business
  rules.
- Making generated JavaScript the owner of app routing, page loading policy, or
  trusted application state.
- Replacing existing `act`/`api` endpoint declarations in this slice.
- Adding custom Go compiler behavior before normal Go AST/type analysis is
  exhausted.

## Users And Permissions

- Primary users: Go developers building small apps that can later split into
  larger deployments.
- Roles or permissions: no new permission model in core; user Go still owns
  auth, guards, roles, tenancy, and policy.
- Data visibility rules: contract reports may show symbols, paths, binaries,
  and binding status, but must not expose secrets, request bodies, or private
  runtime data.

## User Flow

1. A developer defines typed Go contracts and handlers in normal Go packages.
2. A package registration function registers queries, commands, events, and
   jobs with a GOWDK registry.
3. `.gwdk` pages bind forms, page loads, fragments, or endpoints to typed
   contracts through explicit declarations.
4. Frontend UI events trigger commands or queries; they do not emit domain
   events.
5. Backend command handlers validate, authorize, persist, and then emit domain
   events only after the state change is real.
6. Workers, subscribers, realtime channels, and presentation updates react to
   backend-owned events.
7. The compiler uses standard Go parser, AST, and types packages to validate
   exported contract types and handlers.
8. GOWDK generates normal Go adapter code from typed IR and formats it.
9. A single binary can run everything locally.
10. Larger deployments can run selected runtime roles: web, worker, cron, API,
   admin, or combinations of those roles.
11. CLI tooling explains which contracts exist, where they are used, which
   handlers own them, which events are emitted, and which binaries run them.

## Requirements

### Functional

- Contract terminology:
  - UI event: local browser interaction such as click, submit, input, or change.
  - Command: user or system intent crossing into backend trust.
  - Query: readonly request for data, page state, or render data.
  - Domain event: backend fact produced after business logic succeeds.
  - Integration event: durable backend fact shared between binaries or services.
  - Presentation event: backend-derived notification sent to the browser for
    realtime UI updates.
- Commands:
  - A command has one owner handler.
  - Command names use imperative tense, such as `CreatePatient` or
    `ScheduleAppointment`.
  - Command names resolve to exported Go contract types.
  - Command handlers can return typed results and optional domain events.
  - Duplicate command handlers are hard diagnostics.
- Queries:
  - A query reads data for page rendering, fragments, API responses, or command
    result refreshes.
  - Queries must have no state-changing side effects.
  - Query handlers must not be treated as background subscribers.
  - Query result contracts are typed and discoverable.
- Events:
  - Domain and integration events represent something that already happened.
  - Domain and integration event names use past tense, such as
    `PatientCreated` or `InvoicePaid`.
  - The frontend cannot emit domain or integration events.
  - Domain events are emitted only after backend validation, authorization,
    persistence, and transaction success.
  - Domain events can have zero, one, or many subscribers.
  - Integration events are durable backend events shared between binaries or
    services.
  - Presentation events are realtime browser notifications derived from backend
    facts; they are not accepted as trusted domain input.
  - Event subscribers cannot be the owner of the command that caused the event
    unless the user explicitly registers that behavior.
  - Event dispatch is in-process by default and broker-backed only through
    optional adapters.
- Jobs:
  - A job represents background or scheduled work.
  - Jobs can run in the web binary for small apps or in a worker/cron binary for
    larger apps.
  - Job registration is explicit; no function-name auto-discovery.
- Runtime roles:
  - `RunWeb` runs page routes, endpoints, commands exposed over HTTP, SSR, and
    static assets.
  - `RunWorker` runs event subscribers and queued jobs.
  - `RunCron` runs scheduled jobs.
  - `RunAPI` is an optional external API role over the same contracts.
  - `RunAdmin` is optional tooling/admin surface over selected contracts.
- Tooling:
  - `gowdk list commands`, `gowdk list queries`, `gowdk list events`, and
    `gowdk contracts` list contracts, owners, subscribers, usage, and binaries.
  - `gowdk graph` can output a contract graph.
  - `gowdk trace` can explain a command/event path once runtime trace metadata
    exists.
  - `gowdk check` validates missing owners, duplicate command handlers,
    unsupported signatures, unresolved contract references, and unsafe
    generated-JS ownership.
  - Existing `gowdk routes` keeps route/page/endpoint reporting and can link to
    contracts without becoming the contract graph.

### Non-Functional

- Performance: local in-process dispatch must add minimal overhead over direct
  handler calls.
- Reliability: broker adapters and outbox support are optional and must not
  affect small-app defaults.
- Accessibility: SPA command enhancement must preserve no-JavaScript form
  fallback where possible.
- Security/privacy: generated adapters enforce request limits, CSRF for form
  commands, no-store request-time responses, and user-owned auth/guards.
- Observability: contract execution should expose stable names for logs,
  metrics, traces, build reports, and CLI output.

## Acceptance Criteria

- [ ] A command can be registered once, called from a `.gwdk` form, and handled
      by a normal Go function.
- [ ] Duplicate command owners produce a hard diagnostic with source locations.
- [ ] An event emitted by a command can call multiple local subscribers.
- [ ] Domain events are emitted only after the backend state change succeeds.
- [ ] A `.gwdk` template cannot declare that the frontend emits a domain event.
- [ ] Presentation events can notify the browser without becoming trusted input.
- [ ] The same package registrations can run in a single web binary or split
      web/worker binaries.
- [ ] `gowdk contracts` reports command owner, `.gwdk` usage, emitted events,
      subscribers, and selected runtime roles.
- [ ] Generated JavaScript never owns route truth, auth, trusted validation,
      action behavior, global state, or load/cache policy.
- [ ] Broker/outbox adapters are optional extension points, not core
      dependencies.
- [ ] Docs show good domain contracts and reject noisy UI-event contracts.

## Edge Cases

- A `.gwdk` form references a command that is not registered.
- Two packages register the same command owner.
- A command emits an event with no subscribers.
- An event subscriber fails after the command succeeds.
- A command persists state but event dispatch fails.
- A command tries to emit a domain event before transaction success.
- A browser sends a forged presentation event payload back to the backend.
- A worker binary starts without a broker adapter.
- A SPA page submits a command while JavaScript is disabled.
- A command handler needs request context, route params, CSRF, or guard data.
- A feature package imports generated app output and creates an import cycle.

## Dependencies

- Internal:
  - GOWDK AST and analyzer.
  - Stable internal IR.
  - Unified endpoint metadata.
  - Generated adapter IR.
  - Go AST generated code emission.
  - Runtime app/router, form, response, validation, CSRF, fragments, SSR, and
    route metadata.
- External:
  - Go toolchain only for core.
  - Optional future adapters for queues, brokers, persistent outbox, and cron.

## Open Questions

- What exact `.gwdk` syntax should bind a form to a command without conflicting
  with existing `act` declarations?
- Should command handlers return events directly, append them to a result, or
  publish through a context-bound dispatcher?
- Should the first durable outbox contract be tied to explicit transactions, or
  should it start as a persistence-agnostic interface only?
- What is the smallest useful outbox API that does not force a database layer
  into core?
- Should runtime roles be config-driven, CLI-driven, or both?
- How much contract graph detail belongs in `gowdk contracts` versus
  `gowdk graph`?

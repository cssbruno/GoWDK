# ADR 0012: Realtime Subscribe Surface

Date: 2026-06-15

Status: Accepted

## Context

M14 adds realtime reactivity on top of the M13 presentation-event fanout
boundary. The source contract needs to say which browser DOM regions can listen
for presentation events before generated fanout codegen, client patching,
reconnect behavior, or examples are implemented.

Existing constraints:

- Presentation events are the only browser-facing event category.
- Domain and integration events stay backend-owned facts.
- Generated JavaScript can enhance static-first pages, but it must not own
  routes, auth, trusted validation, business rules, page loading policy, or
  cache policy.
- `client {}` is local UI behavior, not a general app data runtime.
- Query references already define bounded, compiler-owned data regions through
  `g:query`.

Issue #147 proposed compiler-derived event-to-query invalidation. M14 keeps the
template contract explicit and adds a Go-owned invalidation graph instead of
letting templates name domain events directly.

## Decision

GOWDK accepts explicit view-level subscriptions through:

```gwdk
<section g:query="patients.GetPatientPage" g:subscribe="patients.PatientNotice">
  ...
</section>
```

Rules:

- `g:subscribe` names a package-qualified Go presentation-event contract.
- `g:subscribe` is valid only on an element that also declares `g:query`.
- The compiler lowers each subscription into IR with owner, query, event, import
  alias/path, event type, source file, and source span.
- The compiler validates subscriptions against scanned runtime contract
  registrations.
- The referenced contract must be a `runtime/contracts` event registration with
  `PresentationEvent` category.
- The registration must be available to the generated web role, or unrestricted.
- The project must enable `realtime.Addon()` before subscriptions are accepted.

GOWDK also accepts explicit Go invalidation registrations:

```go
contracts.RegisterInvalidation[PatientCreated, GetPatientPage](registry)
```

Rules:

- `RegisterInvalidation[event, query]` says a domain event type invalidates a
  query type.
- The compiler scans invalidation registrations beside normal contract
  registrations.
- The scanner rejects edges that name an unknown query, an unknown domain event,
  or a domain event no scanned command emits.
- The compiler joins invalidation edges to `g:query` references and records the
  joined graph in `Program.QueryInvalidations`, `gowdk-build-report.json`, and
  `gowdk graph`.
- Generated apps send a `gowdk.query.invalidate` presentation event after
  command event dispatch for affected domain events.
- Generated `gowdk.js` refetches the current document and swaps only matching
  non-subscribed `data-gowdk-query-type` regions. Regions with `g:subscribe`
  remain owned by explicit presentation patches.

This ADR does not make `client {}` a subscription language and does not let
templates reference domain or integration events directly.

## Consequences

### Positive

- The source contract is explicit and inspectable.
- The target DOM region is bounded by an existing query-owned element.
- Diagnostics can point at the exact subscription attribute.
- Server fanout and client patch-loop work can consume one IR shape.

### Negative

- Authors must name each presentation event or invalidation edge explicitly.
- Query invalidation performs a document refetch instead of payload diffing.
- Subscribed regions and invalidated regions have separate update ownership.

### Neutral

- Generated HTML may expose compiler-owned `data-gowdk-*` markers for later
  runtime binding.
- Generated HTML may expose validated `data-gowdk-query-type` markers for exact
  invalidation matching.

## Alternatives Considered

- Extend `client {}` with subscription statements. Rejected for M14 because it
  would mix server-state invalidation with local UI behavior and expand the
  bounded client language before the server/client ownership contract is ready.
- Infer invalidation from handler bodies. Rejected because it would make
  backend data policy implicit and brittle.
- Allow templates to subscribe to domain or integration events directly.
  Rejected because those categories are backend-owned facts and must not become
  browser-facing input.

## Follow-Up

- #130: lower `g:subscribe` to IR and validate presentation-event bindings.
  Implemented with exact-span diagnostics for missing, invalid, non-web-role,
  non-presentation, and missing-addon references.
- #131: generate server fanout registration using subscription IR. Implemented
  as generated subscription-filtered SSE fanout.
- #132: implement compiler-owned client patch/refresh loop. Implemented for
  explicit `replaceHTML` patches on subscribed query regions.
- #133: define reconnect, backpressure, and guard-gated stream behavior.
  One-second SSE retry hints, drop-on-full client buffers, and guard rejection
  before stream open are implemented; active session-change stream revocation
  remains follow-up work.
- #134: add live-updating examples and docs. Implemented in
  `examples/contracts`.
- #147: compiler-derived event-to-query invalidation. Implemented through
  explicit `RegisterInvalidation[event, query]` scan metadata, build-report and
  graph output, generated `gowdk.query.invalidate` presentation events, and
  query-region document refetch.
- #538: user/session/audience scoping. Implemented for dependency-free SSE
  through optional `EventEnvelope.Audience` labels,
  `contracts.EmitPresentationForAudience`, and
  `realtime.WithSSEAudienceFromRequest`. The labels are server-owned; generated
  streams remain guard-checked and subscription/type filtered, and applications
  install an audience-aware fanout when they send user- or tenant-specific
  presentation payloads.

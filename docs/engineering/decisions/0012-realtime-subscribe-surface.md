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

Issue #147 proposed compiler-derived event-to-query invalidation. That remains
useful future design input, but it requires a broader invalidation graph and
server/client refresh policy. M14 needs an explicit first contract that can
validate source spans and event categories before derived refresh behavior
exists.

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

This ADR does not make `client {}` a subscription language and does not let
templates reference domain or integration events directly.

## Consequences

### Positive

- The source contract is explicit and inspectable.
- The target DOM region is bounded by an existing query-owned element.
- Diagnostics can point at the exact subscription attribute.
- Server fanout and client patch-loop work can consume one IR shape.

### Negative

- Authors must name each presentation event explicitly in the first slice.
- Derived event-to-query invalidation is deferred.
- Live updates require query-region wiring even when a future patch could be
  smaller than a query refresh.

### Neutral

- Generated HTML may expose compiler-owned `data-gowdk-*` markers for later
  runtime binding.
- #147 remains a possible future layer after explicit subscriptions and
  generated refresh semantics are proven.

## Alternatives Considered

- Extend `client {}` with subscription statements. Rejected for M14 because it
  would mix server-state invalidation with local UI behavior and expand the
  bounded client language before the server/client ownership contract is ready.
- Derive invalidation from domain events and query bindings first. Deferred
  because it requires explicit invalidation declarations, generated fanout, and
  fragment refresh policy that are larger than #130.
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

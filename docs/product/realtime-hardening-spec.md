# Feature Spec: Realtime Hardening

## Problem

Realtime subscriptions already stream presentation events to query-owned browser
regions, but the transport and refresh behavior must stay resilient and
authorization-safe across disconnects, stale browser state, and session changes.
Issue #635 tracks the hardening needed after the first realtime slice.

## Goals

- Keep reconnect behavior bounded and server-advertised.
- Keep replay optional, bounded, and tied to stable event IDs.
- Let applications revoke active streams through server-owned audience labels.
- Refresh invalidated query regions through route/query-scoped patches before a
  full-document fallback.
- Reject unsupported patch envelope versions and operations without changing the
  DOM.
- Document operational limits and adapter ownership.

## Non-Goals

- Exactly-once browser delivery.
- Durable cross-process replay in the root runtime.
- Browser-owned authorization, route policy, or cache policy.
- New `.gwdk` syntax.
- Fragment/API-specific standalone renderers in this slice. Their current
  result-to-patch behavior is defined as "no generated patch"; the browser uses
  the current-document fallback unless the region also has an eligible
  route-matched SSR/hybrid renderer.

## Users And Permissions

- Primary users: GOWDK application authors using `addons/realtime`,
  `g:subscribe`, or contract query invalidations.
- Roles or permissions: generated stream and refresh behavior must keep guards
  and audience labels server-owned.
- Data visibility rules: scoped presentation events require an audience-aware
  fanout; route refresh patches are generated only for eligible public
  parameterless request-time regions and are scoped by the browser's current
  route path.

## User Flow

1. A page renders query-owned regions with `data-gowdk-query-type` markers.
2. Generated `gowdk.js` opens `/_gowdk/realtime/events?path=<current route>`.
3. Temporary disconnects use the server's SSE `retry:` hint and optional
   `Last-Event-ID` replay.
4. A domain event emits `gowdk.query.invalidate`.
5. The browser asks `/_gowdk/realtime/query-refresh?path=<current route>` for
   route/query patches, applies returned region HTML, then refetches the current
   document only for regions not patched.
6. Session or authorization changes can call `RevokeAudience` so connected
   clients reconnect and re-run generated guards/audience assignment.

## Requirements

### Functional

- SSE streams advertise a configurable reconnect delay.
- SSE replay keeps only a configured in-memory event window and records replay
  misses.
- Active streams can be disconnected by server-owned audience label.
- Generated route refresh must consider both the invalidated query type and the
  current route path.
- Command single-flight refresh may continue to use query-only rendering only
  when the query has one unambiguous eligible renderer.
- Patch envelopes must be versioned and reject unknown patch operations.
- Fragment/API-specific query execution has an explicit fallback policy:
  generated route/query refresh does not execute fragment or API handlers, does
  not synthesize patches from arbitrary JSON/fragment responses, and falls back
  to the current-document refresh path for unsupported regions.

### Non-Functional

- Performance: slow clients must not block command execution or other clients.
- Reliability: replay misses and unsupported patches leave the DOM unchanged.
- Accessibility: refresh keeps existing focus restoration behavior.
- Security/privacy: guards and audience labels remain server-owned; stream
  responses run guards before opening, and route refresh renders only eligible
  public route-matched regions with `no-store` responses.
- Observability: runtime counters and browser events expose drops, replay,
  revocations, refresh, and patch errors for app-owned metrics/tracing.

## Acceptance Criteria

- [x] Temporary disconnects recover according to a documented bounded policy.
- [x] A revoked or changed session cannot continue receiving events under stale
  authorization once the app revokes the matching audience label.
- [x] Query invalidation can refresh only the affected route/query region.
- [x] Unsupported or stale patch envelopes fail safely and observably.
- [x] Replay remains optional and never implies exactly-once delivery.

## Edge Cases

- Replay disabled or expired: the browser reconnects without replay and waits
  for future events or command/document refresh.
- Multiple eligible routes for the same query: route-scoped refresh can patch
  the matching route; query-only command single-flight skips ambiguous queries.
- Protected or dynamic route regions: no standalone patch is registered, so the
  browser falls back to the route's normal guarded page request.
- Unsupported patch shape: dispatch `gowdk:realtime-error` and leave the region
  unchanged.

## Dependencies

- Internal: `runtime/contracts/sse`, `runtime/ssr`, `runtime/realtime`,
  `internal/appgen`, `internal/clientrt`.
- External: none.

## Open Questions

## Follow-Up Questions

- Which generated adapter metadata should a future fragment/API-specific
  renderer use for safe region rendering beyond the current fallback policy?
- Should production deployments standardize metrics labels for SSE stats, or
  keep export fully app-owned?

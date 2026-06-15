# Implementation Plan: Realtime Reactivity

## Context

Relevant sources:

- Issue #130: ADR plus bounded `.gwdk` subscribe surface, IR, and validation.
- Issue #131: server-side fanout codegen in `internal/appgen`.
- Issue #147: explicit Go invalidation registration plus generated query
  refresh wiring.
- ADR 0012: explicit `g:subscribe` on query-owned elements.
- `docs/product/realtime-reactivity-spec.md`.

## Assumptions

- The first slices implement validation, metadata, generated SSE fanout,
  explicit `replaceHTML` client patches, explicit domain-event to query
  invalidation registrations, one-second SSE retry hints, drop-on-full
  per-client SSE buffers, and guard checks before generated streams open.
  Custom backoff/replay, active session-change stream revocation, richer patch
  shapes, and broader examples are still follow-up work.
- `g:subscribe` is only valid on an element that also declares `g:query`.
- Subscriptions target presentation events registered through
  `runtime/contracts`.
- `realtime.Addon()` is required for any subscription.
- `realtime.Addon()` is required for generated invalidation refresh wiring.
- `RegisterInvalidation[event, query]` uses domain events only in this slice.

## Proposed Changes

- Extend the view directive contract with `g:subscribe`.
- Collect subscription references with exact source offsets.
- Add `gwdkir.RealtimeSubscription` and `Program.RealtimeSubscriptions`.
- Lower view subscription refs during template assembly.
- Link subscription event refs with `contractscan`.
- Validate missing, invalid, non-presentation, non-web-role, and missing-addon
  subscription cases through compiler diagnostics.
- Emit query-region data attributes for later client runtime wiring.
- Record subscription metadata in build reports.
- Generate a dependency-free SSE route for generated apps with bound
  subscriptions.
- Compose command event dispatch with a subscription-filtered presentation
  fanout without removing app-registered event sinks.
- Emit `gowdk.js` for SPA pages with subscribed query regions.
- Emit validated `data-gowdk-subscribe-type` markers for exact event matching.
- Connect subscribed pages to `/_gowdk/realtime/events` through generated
  client runtime.
- Apply explicit `replaceHTML` patches from presentation event payloads to the
  matching query-owned region, and emit `gowdk:realtime-error` for unsupported
  patch payloads without mutating the DOM.
- Declare SSE retry timing and pin the drop-on-full slow-client policy.
- Run inherited subscribed-page guards before generated SSE stream responses
  open, using the requested page path or referer path when available and a
  fail-closed guard union otherwise.
- Add `contracts.RegisterInvalidation[event, query]` runtime metadata.
- Scan invalidation registrations and reject unknown queries, unknown domain
  events, or domain events no command emits.
- Join invalidation edges with bound `g:query` references into
  `Program.QueryInvalidations`.
- Add build-report and `gowdk graph` invalidation metadata.
- Generate `gowdk.query.invalidate` presentation events after command event
  dispatch for matching domain events.
- Emit validated `data-gowdk-query-type` markers and load `gowdk.js` for
  invalidated query regions.
- Refetch the current document and replace only matching non-subscribed query
  regions on invalidation.
- Update language/reference docs and focused tests.

## Files Expected To Change

- `docs/engineering/decisions/0012-realtime-subscribe-surface.md`
- `docs/product/realtime-reactivity-spec.md`
- `docs/engineering/realtime-reactivity-implementation-plan.md`
- `internal/view/*`
- `internal/gwdkir/*`
- `internal/gwdkanalysis/*`
- `internal/contractscan/*`
- `internal/compiler/*`
- `cmd/gowdk/*`
- `internal/lang/tools.go`
- `internal/buildgen/*`
- `internal/appgen/*`
- `internal/diagnostics/*`
- `runtime/contracts/*`
- `docs/language/*`
- `docs/reference/*`

## Data And API Impact

- Internal IR gains a `RealtimeSubscriptions` slice.
- Internal IR gains a `QueryInvalidations` slice.
- Generated HTML can include `data-gowdk-subscribe`.
- Generated HTML can include `data-gowdk-query-type`.
- Build reports can include `realtime_subscription` events.
- Build reports can include `query_invalidation` events.
- Generated apps can mount `/_gowdk/realtime/events`, expose
  `RealtimeEventsPath`, and expose
  `RegisterRealtimeFanout(realtime.PresentationFanout)`.
- Generated apps can synthesize `gowdk.query.invalidate` presentation events.
- Public diagnostic registry gains realtime subscription codes.
- Public diagnostic registry gains contract invalidation codes.
- No new production dependency is introduced.

## Tests

- Unit: `internal/view`, `internal/gwdkir`, `internal/gwdkanalysis`,
  `internal/contractscan`, `internal/compiler`, `internal/diagnostics`,
  `internal/appgen`.
- Integration: `cmd/gowdk` check/build diagnostics for registered and missing
  presentation events.
- End-to-end: generated binary coverage opens the generated SSE stream, executes
  a command, and verifies only subscribed presentation events are delivered.
- End-to-end: generated binary coverage rejects guard-denied realtime streams
  before opening SSE responses.
- Runtime: SSE coverage pins the retry directive and bounded drop-on-full
  backpressure behavior.
- Runtime: invalidation registration and best-effort invalidation fanout sink.
- Client runtime: Node DOM harness covers EventSource mount, deterministic
  patching, unsupported patch rejection, invalidation document refetch, island
  cleanup/remount, and stream cleanup when subscribed/invalidated regions leave
  the page.
- Manual: inspect generated HTML/build report from a small realtime fixture.

## Verification Commands

```sh
go test ./internal/view ./internal/gwdkir ./internal/gwdkanalysis ./internal/contractscan ./internal/compiler ./internal/diagnostics ./internal/buildgen ./internal/lang ./cmd/gowdk
go test ./runtime/contracts/sse
go test ./runtime/contracts
go test ./internal/appgen -run 'TestGenerateWritesRealtimeFanoutForSubscriptions|TestGenerateGuardsRealtimeStreamForSubscribedPages|TestGeneratedBinaryRealtimeFanoutStreamsSubscribedPresentationEvents|TestGeneratedBinaryRealtimeStreamGuardDenialClosesStream'
go build ./cmd/gowdk
```

## Rollback Plan

- Remove `g:subscribe` from the supported directive set.
- Remove `RealtimeSubscriptions` from IR and downstream validators/reporters.
- Remove generated realtime declarations and mux registration from appgen.
- Remove invalidation scan/link/report metadata and the
  `gowdk.query.invalidate` client path.
- Keep ADR 0012 updated with the replacement decision if the surface changes.

## Risks

- Subscription metadata could look implemented before fanout/client update
  slices exist; docs must call this out.
- Component-owned subscriptions may need extra client/runtime ownership
  decisions in #132.
- Over-broad event matching could expose backend-owned events; validation must
  require `PresentationEvent`, and generated fanout must filter to subscribed
  event types.

# Feature Spec: Realtime Reactivity

## Problem

GOWDK can register presentation events and fan them out over SSE or WebSocket,
but `.gwdk` pages cannot yet declare which UI regions should react to those
events. Authors need a Go-owned, compiler-validated way to connect browser
updates to presentation events without making user JavaScript or `client {}`
own server data policy.

## Goals

- Add an explicit `.gwdk` subscription surface for presentation-event driven UI.
- Lower subscriptions into compiler IR with source spans and owner metadata.
- Validate subscription events against scanned Go contract registrations.
- Keep live regions bounded to query-owned elements for the first slice.
- Preserve static-first pages and no-JavaScript fallbacks.

## Non-Goals

- No implicit domain-event to query invalidation in the first slice.
- No payload diffing or arbitrary DOM patch language in `.gwdk`.
- No user-written JavaScript ownership of trusted app behavior.
- No direct template subscription to domain or integration events.
- No global app store or cross-route data cache contract.

## Users And Permissions

- Primary users: Go developers building GOWDK apps with contract-driven backend
  behavior and optional realtime browser updates.
- Roles or permissions: subscriptions execute through generated web-facing
  runtime behavior and may only target unrestricted or `web` role
  presentation-event registrations.
- Data visibility rules: event payload authorization stays in user Go and
  runtime guard/session policy. The compiler only allows browser-facing
  presentation events into subscription metadata.

## User Flow

1. The app enables `realtime.Addon()` in `gowdk.config.go`.
2. Go code registers a presentation event through `runtime/contracts`.
3. A `.gwdk` view declares a query region with `g:query`.
4. The same element declares `g:subscribe` for the presentation event.
5. The compiler validates the event reference and emits subscription IR.
6. Generated apps stream subscribed presentation events to the browser.
7. Generated `gowdk.js` applies explicit bounded patches to the subscribed
   query region.

## Requirements

### Functional

- `g:subscribe="pkg.Event"` is accepted only beside `g:query`.
- The referenced event must resolve to a scanned presentation-event contract.
- Unknown, invalid, non-presentation, or non-web-role event references produce
  diagnostics at the `g:subscribe` source span.
- Subscriptions are represented in `gwdkir.Program`.
- `gowdk check`, `gowdk build`, LSP diagnostics, and `gowdk inspect ir` all see
  the same subscription metadata.
- Generated HTML marks subscribed query regions with compiler-owned data
  attributes for later client runtime hookup.
- Generated client runtime connects subscribed pages to generated SSE streams.
- Generated client runtime applies explicit `replaceHTML` patches from
  presentation event payloads to the matching query region.
- Unsupported realtime patch payloads fail safely without mutating the DOM.
- Generated SSE streams run inherited page guards before opening the response.
- SSE reconnect timing and slow-client drop behavior are explicit.

### Non-Functional

- Performance: validation reuses the existing contract scan report.
- Reliability: unsupported source fails before generated fanout/client runtime
  assumptions are made.
- Accessibility: no-JavaScript output remains the normal rendered query region.
- Security/privacy: only presentation events can cross the browser boundary.
- Observability: build reports should include discovered realtime
  subscriptions when present.

## Acceptance Criteria

- [x] ADR 0012 records the source contract and the #147 direction chosen for
  the first slice.
- [x] IR models `g:subscribe` with owner, query, event, import, status, and
  source span metadata.
- [x] Unknown event references produce diagnostics with source spans.
- [x] Domain and integration events are rejected for `g:subscribe`.
- [x] Subscriptions fail without `realtime.Addon()`.
- [x] Focused tests cover view parsing, IR lowering, linking, validation, and
  generated HTML markers.
- [x] Generated apps with bound subscriptions mount a dependency-free SSE stream
  at `/_gowdk/realtime/events`.
- [x] Generated command adapters send command-emitted presentation events through
  a subscription-filtered fanout.
- [x] Generated fanout skips unsubscribed presentation events and all
  non-presentation event categories.
- [x] Generated SPA output emits `gowdk.js` for subscribed regions.
- [x] Generated HTML emits validated `data-gowdk-subscribe-type` markers.
- [x] Client runtime applies explicit `replaceHTML` patches deterministically.
- [x] Unsupported patch shapes fail safely and emit `gowdk:realtime-error`.
- [x] SSE streams declare a one-second EventSource retry directive.
- [x] Slow SSE clients use bounded per-client buffers and drop events when full.
- [x] Guard denial rejects generated realtime streams safely before SSE opens.
- [x] `examples/contracts` builds a live-updating `g:subscribe` flow and
  documents setup, expected behavior, no-JavaScript fallback, and known limits.

## Edge Cases

- `g:subscribe` without `g:query` is rejected as unbounded.
- Multiple `g:subscribe` directives on one element are invalid.
- Dynamic page routes do not create implicit subscription routes.
- Component-owned subscriptions are legal only when they remain query-bounded.
- Import aliases resolve the same way as `g:query` and `g:command`.

## Dependencies

- Internal: ADR 0012, `runtime/contracts` presentation events,
  `addons/realtime`, `internal/view`, `internal/gwdkir`,
  `internal/gwdkanalysis`, `internal/contractscan`, `internal/compiler`.
- External: none for the root module first slice.

## Open Questions

- Whether #147's derived invalidation graph should layer on top of explicit
  subscriptions or become a separate source declaration.
- Whether later client update shapes should re-fetch query JSON or request a
  server fragment; the first supported shape is an explicit event-payload
  `replaceHTML` patch.
- How server-side session changes should actively revoke already-open streams.

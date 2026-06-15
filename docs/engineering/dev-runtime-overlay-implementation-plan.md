# Implementation Plan: Dev Runtime Overlay Bridge

## Context

Relevant issue: [#424](https://github.com/cssbruno/GoWDK/issues/424).

Relevant product rule: generated JavaScript can enhance local development, but
must not become the source of truth for routing, auth, business rules, trusted
validation, request-time behavior, or cache policy.

## Assumptions

- The current live-reload script and rebuild overlay payload are already
  redacted for compiler/build diagnostics.
- Runtime panic details are not safe to send to browsers without a separate
  generated-app development payload contract.
- The existing incremental SPA dependency graph is stable enough to scope a
  conservative JS-island-only HMR event.
- Static component markup and generated-app runtime mode still need full reload.

## Proposed Changes

- Start generated apps on an internal loopback address in `gowdk dev` runtime
  mode.
- Start a CLI-owned reverse proxy at the requested `--addr`.
- Serve `/__gowdk/reload` from the CLI-owned proxy.
- Strip `Accept-Encoding` before proxying so HTML can be rewritten locally.
- Inject the existing live-reload script into successful HTML `GET` responses.
- Send reload events after successful generated-app rebuild/restart cycles.
- Emit `component-hmr` SSE events for component-only SPA/static changes when the
  dependency graph maps the changed component to affected pages.
- In the injected browser bridge, fetch the current document, replace matching
  changed `<gowdk-island>` roots, remount islands, and reload when no matching
  island boundary exists.

## Files Expected To Change

- `cmd/gowdk/dev.go`
- `cmd/gowdk/serve.go`
- `cmd/gowdk/main_test.go`
- `docs/reference/dev.md`
- `docs/product/requirements.md`
- `docs/product/roadmap.md`
- `docs/product/dev-runtime-overlay-spec.md`
- `docs/engineering/dev-runtime-overlay-implementation-plan.md`

## Data And API Impact

- No generated app source changes.
- No production runtime API changes.
- Runtime-mode `gowdk dev` now uses an internal app address behind the public
  dev proxy.
- Plain SPA/static `gowdk dev` can emit a dev-only `component-hmr` SSE event.

## Tests

- Unit: proxy HTML injection, runtime process address wiring, HMR payload
  planning, HMR script injection, and reload fallback planning.
- Integration: focused `cmd/gowdk` tests for dev serving behavior.
- End-to-end: `go run ./cmd/gowdk dev --app <dir>` manual smoke can confirm
  browser reload behavior; SPA component island edits can confirm root swaps.
- Manual: not required for this slice.

## Verification Commands

```sh
go test ./cmd/gowdk
go build ./cmd/gowdk
```

## Rollback Plan

- Revert runtime-mode dev serving to passing the requested `--addr` directly to
  the generated app and remove the proxy handler.

## Risks

- A generated app that binds only to a special external interface will not see
  that exact address in dev mode; the proxy keeps the public address stable for
  browsers.
- Immediate reload after process restart can race with generated app startup on
  very slow machines; browsers can refresh again and the proxy remains stable.
- Component HMR intentionally remounts changed roots and does not preserve local
  island state yet.

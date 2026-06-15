# Feature Spec: Dev Runtime Overlay Bridge

## Problem

Developers using `gowdk dev --app` or a build target with `App` run the
generated app binary for backend routes, actions, APIs, fragments, and SSR. The
generated binary owns the browser port, so the existing dev server live-reload
script and browser build-error overlay are unavailable in that mode.

## Goals

- Keep generated-app development behind the same requested `gowdk dev --addr`.
- Deliver rebuild failure overlay events to generated-app pages without changing
  production generated output.
- Reload the browser after successful generated-app rebuilds and restarts.
- Hot-swap changed JavaScript island component roots in plain SPA/static dev
  serving when the component dependency graph proves the current page is
  affected.
- Keep runtime logs and request-time errors terminal-first until there is a safe
  browser payload contract.

## Non-Goals

- State-preserving HMR.
- HMR for generated-app runtime mode, WASM islands, static component markup
  without island boundaries, page changes, layout changes, and source-set
  changes.
- Preserving island or component state across generated-app reloads or HMR root
  swaps.
- Browser surfacing for runtime panics, cookies, submitted form values, request
  bodies, or raw runtime logs.
- Any generated app source or production binary change.

## Users And Permissions

- Primary users: local GOWDK app developers.
- Roles or permissions: local development only.
- Data visibility rules: browser overlay payloads use the existing redacted
  rebuild error event shape and must not include request data, cookies, or form
  bodies.

## User Flow

1. A developer starts `gowdk dev --app .gowdk/app`.
2. The CLI serves the requested address through a dev-only proxy and starts the
   generated app on an internal loopback address.
3. Browser HTML responses receive the live-reload script.
4. Failed rebuilds show the browser overlay while the last successful app keeps
   serving when possible.
5. Successful rebuilds restart the generated app and reload the browser.
6. In plain SPA/static serving, component-only changes emit a component HMR
   event when the dependency graph maps the change to affected routes.
7. The browser swaps matching `<gowdk-island>` roots from a fresh document and
   remounts islands, or reloads the page when no safe island boundary exists.

## Requirements

### Functional

- Runtime-mode dev serving must keep the public `--addr` stable.
- The generated app process must receive `GOWDK_ADDR` for an internal loopback
  address, not the public dev address.
- The proxy must serve `/__gowdk/reload` itself.
- The proxy must inject the existing live-reload script into successful HTML
  `GET` responses from the generated app.
- Non-HTML responses must proxy unchanged except for standard reverse-proxy
  behavior.
- Component HMR events must include changed component identity and affected
  routes derived from the same dependency graph used by incremental SPA builds.
- The browser must fall back to full reload when the current page is affected
  but no matching island root can be hot-swapped.

### Non-Functional

- Performance: proxy overhead is local-development only.
- Reliability: rebuild errors still reach the terminal if no browser is
  connected.
- Accessibility: overlay uses the existing `role="alert"` script.
- Security/privacy: production generated output is unchanged; runtime request
  data is not sent to the browser overlay.
- Observability: runtime stdout/stderr remain attached to the terminal.

## Acceptance Criteria

- [x] Runtime-mode generated app pages receive the live-reload script through
  the dev-only proxy.
- [x] Rebuild errors can be sent to generated-app pages over `/__gowdk/reload`.
- [x] Generated app binaries run on an internal address while the browser keeps
  using the requested `--addr`.
- [x] JS island component HMR has dependency-graph coverage.
- [x] Page-route changes fall back to full reload while stale output cleanup
  remains verified.
- [x] Docs state that runtime panic surfacing and broader/state-preserving HMR
  remain deferred.

## Edge Cases

- If the generated app is not ready yet, the proxy may temporarily return the
  standard reverse-proxy error.
- HTML responses with explicit content encodings are not rewritten.
- `HEAD` responses are not rewritten.
- Static component markup without a `<gowdk-island>` root cannot be swapped
  safely and reloads instead.
- Dynamic routes are matched by the browser bridge against GOWDK route patterns
  where possible.

## Dependencies

- Internal: `cmd/gowdk` dev loop, live reload broker, generated app process
  runner.
- External: Go standard library only.

## Open Questions

- What safe, structured runtime error payload should generated apps expose in
  development for panic overlays?
- What state-preservation contract should future HMR support use for local
  stores, effects, refs, and parent/child exports?

# Feature Spec: Interactive Runtime

## Problem

GOWDK currently proves static rendering, components, generated assets, one-binary
serving, and first-slice action redirects. That is not enough for product apps
that users expect to feel reactive: forms should update parts of the page,
loading and error states should be visible, common UI controls should respond
without hand-written JavaScript, and selected UI regions should support local
state.

The product cannot become a React or Svelte clone with an npm dependency graph.
The fix is a compile-first interactivity model with server fragments for
action-driven updates, generated JavaScript by default for typed local state
islands, and explicit WASM only when a component instance requests it.

## Goals

- Make action-driven partial updates work end to end without enabling full-page SSR.
- Add a small generated client runtime only when a page uses interactive features.
- Provide declarative interaction syntax for common product UI workflows.
- Keep static HTML as the initial output for every interactive page.
- Preserve one-binary deploys with embedded static assets and generated handlers.
- Make compiler diagnostics explain which runtime capability a page requires.

## Non-Goals

- Full React compatibility.
- Full Svelte syntax compatibility.
- A virtual DOM as the framework identity.
- Mandatory client-side routing.
- Mandatory npm, bundlers, or user-written JavaScript for ordinary forms and partial updates.
- Replacing SSR addon semantics with a client app model.

## Users And Permissions

- Primary users: Go developers building product apps with `.gwdk` pages.
- Roles or permissions: project authors control build config, addons, generated binaries, and runtime features.
- Data visibility rules: server fragments and generated handlers must not log sensitive form values or leak private response data into static artifacts.

## User Flow

1. A developer writes a static/action page with `g:post`, `g:target`, and a matching `fragment "#id" {}` block.
2. `gowdk build --out dist --app .gowdk/app --bin bin/site` emits static HTML, the partial runtime asset, and generated POST handlers.
3. A browser submits the form. The generated runtime sends a partial request, receives a server fragment, swaps the target, and restores focus.
4. For local-only UI state, a developer declares `state ui.Type = ui.Init()` on
   a component. GOWDK renders the init state into static HTML and emits a
   generated JavaScript island for component calls by default.
5. If a component instance needs the WASM path, the developer writes
   `g:island="wasm"` on that component call. No WASM is emitted otherwise.

## Requirements

### Functional

- `g:post` forms with `g:target` must submit with `X-GOWDK-Partial`.
- Generated action handlers must return a fragment response when a matching partial request reaches an action with `fragment "#id" {}`.
- Static output must include `gowdk.js` only for pages that need partial runtime behavior.
- Fragment responses must set `Content-Type: text/html; charset=utf-8`, `Cache-Control: no-store`, and fragment target metadata.
- Client runtime must support `innerHTML` and `outerHTML` swaps, loading state, request lifecycle events, and focus restoration.
- Compiler validation must reject partial targets that do not point to a static `id` in the rendered page.
- The next client-state slice must support a narrow declarative model before general expressions:
  - local scalar state,
  - event-triggered assignment,
  - increment/decrement,
  - boolean toggle assignment,
  - scalar field reads.
- Stateful components use generated JavaScript islands by default.
- `g:island="wasm"` must be explicit per component instance, and unknown
  `g:island` values must be compiler errors.
- Component contracts must support Go module import paths for props and state
  structs.
- Duplicate component names and redundant component implementations must be
  compile errors.

### Non-Functional

- Performance: static HTML must remain the first paint; the base partial runtime should stay small and dependency-free.
- Reliability: failed partial requests must not corrupt the DOM; forms must fall back to normal POST behavior when JavaScript is unavailable.
- Accessibility: focus restoration, `aria-busy`, and semantic form behavior are required for partial updates.
- Security/privacy: generated action handlers need body limits, unexpected-field rejection, CSRF before production status, and no sensitive logs.
- Observability: generated binaries should expose route/action/partial failure counts once runtime metrics exist.

## Acceptance Criteria

- [ ] A page using `g:post`, `g:target`, and `fragment "#id" {}` builds into static HTML plus `assets/gowdk/gowdk.js`.
- [ ] The generated binary returns a 200 HTML fragment for partial POST requests and a redirect or no-content response for normal POST requests.
- [ ] The client runtime swaps the returned fragment using the requested swap mode and restores focus.
- [ ] Pages without partial or island features do not emit `gowdk.js` or island
  assets.
- [ ] `go test ./internal/staticgen ./internal/appgen ./internal/clientrt` covers the first partial-update path.
- [ ] Docs show the current interactive feature set and clearly separate planned islands from implemented partials.
- [ ] A local-state island example can update a counter without user-written
  JavaScript.

## Edge Cases

- Partial request targets an action with no fragment.
- Action declares multiple fragments and the client asks for one target.
- JavaScript disabled.
- Fragment target is missing from the current DOM.
- Validation failure during a partial request.
- Oversized form body.
- Duplicate action routes.
- Multiple pages require the same runtime asset.
- Static asset route overlaps a dynamic SSR route.

## Dependencies

- Internal:
  - `internal/parser` fragment and directive parsing.
  - `internal/view` static markup rendering and `g:post` lowering.
  - `internal/staticgen` page output and asset manifest generation.
  - `internal/appgen` embedded binary generation.
  - `internal/clientrt` partial client runtime.
  - `runtime/response` fragment response contracts.
- External:
  - None for partial runtime.
- Generated JavaScript is the default local island runtime.
- Browser WASM islands are explicit with `g:island="wasm"`.

## Open Questions

- What exact syntax should represent local state without feeling like a second programming language?
- Should fragments be allowed to use components in the first production slice?
- How should partial validation errors map to target fragments?
- Should generated metrics be included in the first interactive binary slice or added after handler contracts settle?

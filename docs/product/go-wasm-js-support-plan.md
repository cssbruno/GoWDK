# Go WASM And JavaScript Support Plan

Date: 2026-06-04

## Summary

GOWDK should support browser interactivity without changing its core identity.
The default product remains static/action-first: pages render at build time,
forms and mutations use typed server actions, partial updates use server
fragments, and full-page SSR stays optional.

This document covers future WASM islands and the generated browser runtime they
share with partial updates. The generated JavaScript runtime is not WASM-specific
and should be specified separately before implementation if partial updates land
before islands.

JavaScript support should have two layers:

1. A small generated JavaScript runtime owned by GOWDK.
2. Optional Go-authored browser islands compiled to WebAssembly.

Users should not need to write JavaScript for normal application flows. When
they need local browser interactivity, they should write Go island code and let
GOWDK compile, load, and mount it.

## Goals

- Keep normal pages usable without user-authored JavaScript.
- Use generated JavaScript only for browser integration that cannot be done in
  HTML alone.
- Let users author advanced client behavior in Go through optional WASM islands.
- Load WASM only for components that opt in.
- Keep WASM islands separate from server actions, partial updates, and SSR.
- Preserve static-first output and one-binary deploy.
- Make the client model explicit enough for compiler diagnostics and editor
  tooling.

## Non-Goals

- Do not turn GOWDK into a SPA framework.
- Do not require user-authored JavaScript for forms, navigation, or normal
  server interactions.
- Do not replace server actions with client-side mutations.
- Do not use WASM for simple DOM swaps where small generated JavaScript is
  cheaper and clearer.
- Do not make WASM part of v0.1 through v0.5. It belongs after static output,
  actions, partials, SSR, and hybrid behavior are stable.

## Core Model

```text
Static/action-first GOWDK:
  .gwdk source
  -> compiler
  -> static HTML
  -> generated Go handlers
  -> generated small JS runtime
  -> optional embedded assets

Go WASM islands:
  selected component opt-in
  -> Go browser package compiled to .wasm
  -> generated island manifest
  -> generated JS loader mounts island on the matching DOM node
```

Normal app flows:

```text
GET /patients
  -> serve static or embedded prerendered HTML

POST /patients?/create
  -> generated server action
  -> optional server fragment response
  -> generated JS applies fragment swap
```

Island flow:

```text
GET /counter
  -> serve static HTML with island marker
  -> generated gowdk.js scans island markers
  -> loads counter.wasm
  -> passes serialized props
  -> Go WASM attaches local browser behavior
```

## Generated JavaScript Runtime

GOWDK should emit a small `gowdk.js` runtime when a page uses partial updates or
WASM islands.

Responsibilities:

- Enhance `g:post` form submissions.
- Read `g:target` and `g:swap` attributes.
- Send action requests through `fetch`.
- Apply server fragment responses.
- Manage loading states.
- Restore focus after partial swaps.
- Dispatch lifecycle events.
- Load WASM island assets.
- Pass serialized island props.
- Mount and unmount island instances.

Non-responsibilities:

- Own application state globally.
- Render full pages on the client.
- Implement a client router by default.
- Replace server actions.
- Run user-authored JavaScript modules.

## WASM Island Addon

Add a future optional addon:

```go
package islands

func Addon() gowdk.Addon
```

Feature ID:

```go
gowdk.FeatureIslands
```

Possible package layout:

```text
addons/islands/
  islands.go
  manifest.go
  loader.go

runtime/browser/
  context.go
  dom.go
  event.go
  lifecycle.go

internal/clientrt/
  gowdk_js.go
  wasm_loader.go

internal/codegen/
  islands.go
```

The addon enables compilation and loading of Go browser islands. It should not
be required for partial updates. Partial updates need only the generated JS
runtime.

## Source Syntax

The language should mark client islands explicitly. Candidate syntax:

```gwdk
@page counter
@route "/counter"
@layout root

view {
  <Page title="Counter">
    <Counter initial={10} g:island />
  </Page>
}
```

For named island entrypoints:

```gwdk
view {
  <Counter initial={10} g:island="counter" />
}
```

For island-only components:

```gwdk
@component Counter
@island counter

props {
  initial int
}

view {
  <button>{initial}</button>
}
```

These examples are not final grammar. The language spec must decide whether
`g:island` is a directive attribute, an annotation, or both.

## Go Island Authoring

User browser-side code should be Go, compiled to WASM.

Conceptual API:

```go
package counter

import "github.com/gowdk/gowdk/runtime/browser"

type Props struct {
	Initial int `json:"initial"`
}

func Mount(ctx browser.Context, props Props) error {
	button := ctx.Root().Query("button")
	count := props.Initial

	button.On("click", func(event browser.Event) {
		count++
		button.SetText(strconv.Itoa(count))
	})

	return nil
}
```

GOWDK should generate the bridge between the `.gwdk` component and this Go WASM
entrypoint.

## Runtime Browser API

`runtime/browser` should be intentionally small. It should wrap only browser
APIs needed by islands.

Initial API surface:

- `Context.Root()` returns the island root element.
- `Context.Fetch()` wraps browser fetch if needed.
- `Element.Query(selector string)` finds descendants.
- `Element.SetText(value string)` updates text content.
- `Element.SetHTML(value SafeHTML)` only accepts explicit safe HTML.
- `Element.Attr(name string)` reads attributes.
- `Element.SetAttr(name, value string)` writes attributes.
- `Element.On(event string, handler func(Event))` registers event handlers.
- `Event.PreventDefault()` prevents default behavior.
- `Event.Target()` exposes the event target.

Rules:

- Avoid exposing low-level JS interop as the primary API.
- Allow an explicit escape hatch only for advanced cases.
- Keep DOM writes local to the island root unless an API explicitly says
  otherwise.

## Generated Assets

For a page using an island:

```text
dist/
  pages/counter/index.html
  assets/gowdk.js
  assets/islands/counter.wasm
  assets/islands/counter.json
  assets/island-manifest.json
```

Example island manifest:

```json
{
  "islands": {
    "counter": {
      "wasm": "/assets/islands/counter.wasm",
      "entry": "counter.Mount",
      "props": "CounterProps"
    }
  }
}
```

The exact JSON shape should be decided when the compiler owns island codegen.

## Generated HTML Contract

The compiler should mark island roots with stable attributes:

```html
<div
  data-gowdk-island="counter"
  data-gowdk-props="..."
>
  <button>10</button>
</div>
```

Rules:

- Props must be serialized safely.
- The marker must survive static rendering, embedding, and SSR.
- The island root must be narrow: only the component subtree should be owned by
  the island.
- Generated attributes must be namespaced with `data-gowdk-*`.

## Build Pipeline

WASM island build steps:

1. Discover `.gwdk` files and island declarations.
2. Parse component and island directives.
3. Resolve island entrypoints to Go packages/functions.
4. Generate prop structs or bridge code if needed.
5. Compile island packages with:

   ```sh
   GOOS=js GOARCH=wasm go build -o dist/assets/islands/<name>.wasm <pkg>
   ```

6. Copy or generate the JavaScript WASM loader.
7. Emit island manifest JSON.
8. Inject `gowdk.js` into pages that require partial updates or islands.
9. Embed island assets when `gowdk.Embed` is enabled.

## SSR And Hybrid Interaction

Static pages:

- HTML renders at build time.
- Island marker and serialized props are emitted at build time.
- WASM mounts after the browser loads the page.

Action pages:

- Same as static pages.
- Server actions remain backend-owned.
- Island code may enhance local component behavior, but server mutations still go
  through actions.

Partial pages:

- Fragment responses may contain island markers.
- Generated JS must mount islands added by a fragment swap.
- Generated JS must unmount islands removed by a fragment swap.

SSR pages:

- HTML renders at request time.
- Island props may be serialized from request-time `load {}` data.
- SSR must still use the same marker and manifest contract.

Hybrid pages:

- Follow the final hybrid policy once it is defined.
- The island loader must not force a page into full client-side routing.

## Performance Rules

WASM is not automatically faster than JavaScript.

Use generated JavaScript for:

- DOM updates.
- Form enhancement.
- Partial swaps.
- Loading states.
- Focus restoration.
- Island loading.

Use Go WASM for:

- Local interactive widgets.
- CPU-heavy browser-side logic.
- Reusing Go domain logic in the browser.
- Complex stateful components that need Go authoring.

Avoid WASM for:

- Simple click handlers.
- Simple DOM toggles.
- Basic form submission.
- Navigation.
- Small UI behavior that generated JS can own.

## Security And Privacy

- Escape serialized props.
- Do not serialize secrets into island props.
- Do not expose server-only data to browser code.
- Keep raw HTML APIs explicit and opt-in.
- Keep island DOM ownership local to the mounted subtree.
- Validate all action/API requests server-side even if an island performed local
  validation.
- Ensure embedded WASM assets do not include private source paths or debug data by
  default.
- Decide source map policy before release.

## Accessibility

- Server-rendered HTML should remain meaningful before WASM loads.
- Controls should work as much as possible without island code.
- Generated partial update runtime must preserve focus.
- Islands should expose lifecycle hooks for focus management when needed.
- Compiler diagnostics should warn when an island removes required semantic
  markup or depends on empty placeholder content.

## Diagnostics

Add diagnostics for:

- `g:island` used without the islands addon.
- Island directive references missing entrypoint.
- Island entrypoint has invalid signature.
- Props cannot be serialized.
- Island package fails to compile for `GOOS=js GOARCH=wasm`.
- Multiple islands try to own the same root.
- Partial fragment inserts island markup but client runtime is disabled.
- SSR page serializes non-browser-safe data into props.

## Testing Plan

Unit tests:

- Island directive parsing.
- Island manifest generation.
- Prop serialization.
- Browser runtime helper behavior where testable.
- Diagnostic codes and messages.

Integration tests:

- Compile a static page with one island.
- Compile an action page with generated JS and no island.
- Compile a partial fragment that inserts an island marker.
- Compile an SSR page with request-time island props.
- Verify embedded assets include `.wasm`, `gowdk.js`, and island manifest.

End-to-end tests:

- Serve generated app.
- Load static island page in a browser.
- Confirm WASM loads and mounts.
- Submit an enhanced form through generated JS.
- Swap a fragment that contains a new island.
- Confirm removed island instances are unmounted.

Manual checks:

- Browser devtools show no unnecessary island downloads on pages without islands.
- Basic pages still work if WASM fails to load.
- Generated JS payload remains small.

## Documentation Plan

Add docs:

- `docs/language/islands.md`
- `docs/guides/wasm-islands.md`
- `docs/reference/generated-js-runtime.md`
- `docs/reference/island-manifest.md`
- `docs/reference/runtime-browser.md`

Update docs:

- `docs/product/roadmap.md`
- `docs/product/requirements.md`
- `docs/engineering/architecture.md`
- `docs/engineering/security.md`
- `docs/engineering/testing.md`
- `docs/product/missing-implementation-checklist.md`

Add examples:

- Static counter island.
- Partial update that inserts an island.
- Server action plus local WASM validation.
- SSR page with request-time island props.

## Implementation Phases

### Phase 1: Generated JavaScript Runtime

- Emit `gowdk.js`.
- Enhance `g:post` and support `g:target` and `g:swap`.
- Support loading states and focus restoration.
- Keep runtime independent from WASM islands.
- Test server fragment swaps.

### Phase 2: Island Language Contract

- Specify `g:island` or equivalent syntax.
- Parse island directives.
- Add manifest fields for island usage.
- Add diagnostics for unsupported island usage.
- Add docs and examples.

### Phase 3: WASM Build Pipeline

- Add islands addon.
- Resolve Go island entrypoints.
- Compile Go packages to WASM.
- Emit island assets and manifest.
- Inject loader only on pages that need it.

### Phase 4: Runtime Browser API

- Add `runtime/browser`.
- Wrap minimal DOM/event APIs.
- Add lifecycle APIs.
- Add explicit escape hatches.
- Add security review for browser data exposure.

### Phase 5: Partial, SSR, And Embed Integration

- Mount islands inserted by partial swaps.
- Unmount islands removed by partial swaps.
- Serialize request-time SSR props safely.
- Embed WASM assets in one-binary output.
- Add generated binary smoke tests.

## Open Questions

- Should island syntax be `g:island`, `@island`, or both?
- Should island Go entrypoints be inferred from component names or declared
  explicitly?
- Should props be declared in `.gwdk`, Go, or both?
- Should GOWDK support TinyGo as an optional optimization later?
- Should source maps for WASM be disabled by default?
- Should the generated JS runtime be emitted always, or only when partials or
  islands are used?

## Recommended Defaults

- Use `g:island` as the first island marker because it matches existing `g:`
  directives.
- Emit generated JavaScript only when the page needs partial updates or islands.
- Compile islands with the standard Go WASM toolchain first.
- Keep TinyGo out of scope until the standard Go path works.
- Keep all server mutations in actions/APIs.
- Treat WASM as an optional island layer, not the default UI model.

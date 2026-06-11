# Feature Spec: Golangish Reactive Islands

## Problem

The current island slice proves that GOWDK can render stateful components and
emit generated JavaScript by default, but it only supports a tiny expression
subset. That is not enough for product UI work. At the same time, raw Go is not
a good fit for browser-local reactivity: Go's runtime model, pointers,
goroutines, package imports, and WASM output are too heavy for common UI state
such as toggles, forms, tabs, filters, and small derived values.

GOWDK needs a deliberately small, Go-like client language that feels familiar to
Go developers, type-checks at compile time, and compiles to generated JavaScript
without npm. WASM stays explicit for cases that truly need browser-side Go.

This is a GOWDK compiler feature, not a fork of Go. The `client {}` language is
a constrained `.gwdk` subset that can read Go-derived component contracts, but
it is not arbitrary Go source and does not require a custom Go compiler.

## Goals

- Define a Go-ish `.gwdk` client language for local component interactivity.
- Keep JavaScript generation as the default island runtime.
- Keep `g:island="wasm"` explicit and separate from the default path.
- Support common reactive UI features without exposing arbitrary JavaScript.
- Preserve SPA HTML as the first output.
- Keep the language small enough for compiler diagnostics, editor tooling, and
  generated code review.

## Non-Goals

- Full JavaScript syntax.
- Full Go syntax in the browser.
- React, Svelte, JSX, virtual DOM, or npm as required runtime pieces.
- Arbitrary package imports inside client logic.
- Implicit WASM for stateful components.
- General-purpose async application framework in the first slices.
- Forking or replacing the Go compiler.

## Users And Permissions

- Primary users: Go developers building GOWDK product UIs.
- Roles or permissions: project authors define components and decide whether an
  instance uses default generated JS or explicit WASM.
- Data visibility rules: client state is visible in the page. Secrets,
  credentials, server-only data, and private action data must not be serialized
  into island bootstrap data.

## User Flow

1. A developer declares props and state with imported Go structs.
2. The developer writes local UI logic in a `client {}` block using a Go-like
   subset.
3. The component `view {}` uses bindings such as `{Count}`,
   `g:on:click={Increment()}`, `g:if={Open}`, `g:bind:value={Query}`, and
   `g:for={item in Items}`.
4. `gowdk build` type-checks the component contract, client block, and view
   bindings, then emits SPA HTML and generated JS assets.
5. If a component instance explicitly sets `g:island="wasm"`, GOWDK emits WASM
   island assets instead of the default generated-JS island for that instance.

## Proposed Language Shape

The language is Go-like, but intentionally not Go:

```gwdk
package ui

component Counter

import ui "github.com/acme/app/ui"

props ui.CounterProps
state ui.CounterState = ui.NewCounterState()

client {
  fn Increment() {
    Count++
  }

  fn Toggle() {
    Open = !Open
  }

  computed Label string {
    return fmt("%d clicks", Count)
  }
}

view {
  <button g:on:click={Increment()}>{Label}</button>
}
```

Rules:

- `client {}` can read and write declared state fields.
- `client {}` can read props but cannot mutate them.
- `fn` declares event-callable component-local functions.
- `computed` declares derived values.
- Built-ins are compiler-owned, such as `fmt`, `len`, `lower`, `upper`, and
  `contains`; they are not Go imports.
- The compiler lowers the subset to generated JavaScript.
- Unsupported syntax is a compile error, not a runtime fallback.
- The source stays inside `.gwdk`; normal Go packages still own server logic and
  component contract types.

## Requirements

### Functional

- Parse `client {}` blocks in `.cmp.gwdk` files.
- Type-check client reads/writes against resolved props and state contracts.
- Compile supported client functions, computed values, effects, bindings, and
  directives to generated JavaScript.
- Reject unsupported expressions and unknown fields before codegen.
- Emit deterministic runtime assets under `assets/gowdk/islands/`.
- Record all emitted island assets in `gowdk-assets.json`.
- Preserve explicit `g:island="wasm"` as an opt-in path.

### Non-Functional

- Performance: generated JS should be per-component and tree-shaped, not a full
  framework runtime.
- Reliability: unsupported syntax must fail compilation with source spans.
- Accessibility: bindings must preserve semantic HTML and support ARIA
  attributes.
- Security/privacy: only state explicitly declared for the component may be
  serialized to the browser.
- Observability: generated JS should have readable component/function names and
  optional source-map support in a later slice.

## Acceptance Criteria

- [x] A counter component can use `fn Increment() { Count++ }` and
  `g:on:click={Increment()}`.
- [x] A disclosure component can use `computed` values and `g:if`.
- [x] A filter component can bind an input with `g:bind:value={Query}` and
  render a filtered list.
- [x] Attribute, class, style, and ARIA bindings update without full-page
  hydration.
- [x] Unsupported client syntax fails with a clear diagnostic.
- [x] A stateful component without `g:island` emits generated JS by default.
- [x] A component call with `g:island="wasm"` emits only explicit WASM island
  assets for that instance.

## Edge Cases

- Component prop and state fields share a name.
- Computed values depend on other computed values.
- Event handlers mutate a field used by a list, conditional, and attribute
  binding at the same time.
- A list key is missing or duplicated.
- Form binding tries to write into a prop instead of state.
- Async calls race with newer user input.
- Server fragment replacement removes an active island.
- JavaScript is disabled.

## Dependencies

- Internal:
  - `internal/parser` for `client {}` parsing.
  - `internal/gotypes` for props/state field contracts.
  - `internal/compiler` for type checking and diagnostics.
  - `internal/view` for binding/directive parsing.
  - `internal/buildgen` for asset emission and script injection.
  - `runtime/*` only for public contracts that generated apps need.
- External:
  - None for generated JavaScript.
  - Go toolchain only for resolving imported props/state contracts.

## Open Questions

- Should the client language be named in user docs, or described simply as the
  `client {}` subset?
- Should string formatting use `fmt(...)`, `sprintf(...)`, or a template
  interpolation form?
- Should async client code be allowed before typed server actions are complete?
- Should stores be component-local first, or allow page/module scope from the
  beginning?

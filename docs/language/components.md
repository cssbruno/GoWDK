# Components

The first component slice is implemented for SPA build output.

Implemented today:

- Explicit or discovered `.cmp.gwdk` build inputs with `@component Name`.
- Optional `props { name string }` declarations.
- Component-local Go imports using normal module import paths, such as
  `import ui "github.com/acme/app/ui"`.
- Typed props contracts that reference imported Go structs, such as
  `props ui.CounterProps`.
- Typed state contracts that reference imported Go structs and a no-argument
  init function, such as `state ui.CounterState = ui.NewCounterState()`.
- One `view {}` block per component.
- Self-closing component calls such as `<Hero title="GOWDK" />`.
- Wrapper component calls such as `<Panel>...</Panel>` with child content
  rendered into `<slot />` in the component view.
- Page-level cross-package component imports with
  `use ui "components"` and qualified calls such as `<ui.Hero />`.
- Component-scoped cross-package component imports with local aliases such as
  `use icons "icons"` and `<icons.Badge />` inside the declaring component.
- Escaped `{prop}` text and attribute interpolation inside component views.
- Component prop values can interpolate page build data, such as a route param
  from literal `paths {}`.
- Slot children render in the caller scope, so page build data and route params
  used inside the child content are resolved before being inserted into the
  component slot.
- A component with `state ...` renders initial state at build time and emits a
  generated JavaScript island by default when called without `g:island`.
- Component-local `client { fn Name(...) { ... } }` handlers can group the
  current safe typed expression subset and be called from `g:on:*` with scalar
  expressions, such as `Add(Count + 1)`.
- Component-local `computed Name Type { return expr }` values can derive
  read-only browser state from props, state, and other computed values. The
  compiler orders computed values by dependency and rejects cycles.
- Pages can declare first-slice page-scoped stores with
  `store cart ui.CartState = ui.NewCartState()`. Component `client {}` blocks
  can declare explicit dependencies with `use cart`; the compiler validates
  store type/init contracts and rejects unknown store uses. Runtime shared-state
  subscriptions are still planned.
- A component can declare `@wasm ./browser/counter` to compile a browser-side
  Go package for explicit WASM island calls. The package is built with
  `GOOS=js GOARCH=wasm`, must produce a real browser WASM module, and must not
  import server/process/network packages such as `net/http`, `os/exec`,
  `database/sql`, raw `syscall`, `plugin`, or `unsafe`.
- A component call can explicitly request the WASM island artifact with
  `<Counter g:island="wasm" />`. If the component has no `@wasm` package,
  GOWDK still emits the first-slice placeholder module plus loader shape.
- Duplicate component names are rejected during manifest validation.
- Redundant component implementations are rejected during manifest validation
  with `redundant_component_implementation`, even when their names differ.
- Component files are compiler inputs, not Go imports. A page can call a
  same-package component by name, such as `<Hero />`, when that component file
  is part of the same build/module input set. Cross-package page calls must use
  a GOWDK source import and qualified component tag:

```gwdk
package pages

use ui "components"

view {
  <main><ui.Hero title="GOWDK" /></main>
}
```

  The quoted `use` target is a discovered `.gwdk` package name, not a Go import
  path. Imported components can call sibling components in their own package by
  bare name inside the component body. Components can also declare their own
  scoped `use` aliases for qualified child component calls:

```gwdk
package marketing

use icons "icons"

@component Hero

view {
  <section><icons.Badge /></section>
}
```

## Component Contract

Component files are GOWDK compiler inputs. They are not imported by Go code and
must not import generated app output. Go `import` declarations inside `.gwdk`
files import normal Go packages for typed contracts and build-time helpers.
GOWDK `use` declarations import discovered `.gwdk` source packages; today that
contract is implemented for qualified component calls.

Props are caller-provided inputs. Inline `props {}` declarations are string-only
in the current slice, while imported Go struct contracts can provide typed
props metadata. Parent calls can pass literal strings and the implemented
build-data interpolation subset. Props are read-only to `client {}` code; mutable
browser state belongs in `state` or in an explicit page store.

State is component-local UI state. A `state Type = Init()` declaration runs the
no-argument Go init function at build time for SPA/static output and serializes
the JSON-compatible initial value into the component island. State is visible to
the browser and must not carry secrets, trusted authorization state, database
state, or server validation results that the server still needs to enforce.

Computed values are read-only derived state. They can depend on props, state,
and other computed values. The compiler builds a dependency graph for declared
computed values, emits them in dependency order, and rejects cycles before
generated JavaScript is written.

Stores are page-scoped UI state. A page `store` declaration names the store,
the Go type, and the build-time init function. A component `client { use name }`
declares that the island may use that store. Generated browser store state is a
page-local enhancement contract; it is not global application authority and it
does not replace server-side routing, auth, validation, persistence, or action
behavior.

`client {}` is a compiler-owned UI language, not arbitrary JavaScript. The
supported handlers, helpers, lifecycle blocks, effects, refs, list built-ins,
bindings, conditionals, computed values, and scalar expressions are defined in
[syntax.md](syntax.md). Generated island JavaScript interprets that bounded
subset instead of evaluating arbitrary user JavaScript source.

Generated browser runtime behavior is scoped to the island or page enhancement
that requested it. JavaScript may update text, attributes, classes, styles,
form bindings, list rows, local state, page stores, partial responses, and SPA
link transitions. It must not own route existence, auth, business rules,
database access, trusted server validation, action behavior, or cache/loading
policy. Real routes, direct browser refresh, and server behavior stay owned by
the compiler manifest, generated Go, and user Go handlers.

Explicit WASM islands require `@wasm` and `g:island="wasm"`. The referenced
package is browser-side Go compiled for `GOOS=js GOARCH=wasm` with server and
process packages rejected. The production ABI, required entrypoints, and export
validation are still planned; WASM islands are not a replacement for backend
handlers.

Not implemented yet:

- Non-string props in inline `props {}` blocks.
- Parent-to-child expression props beyond the implemented literal and build-data
  interpolation slice.
- Bindable child state.
- Full runtime validation for user browser logic in WASM islands, including
  required Go/JS entrypoint registration and export checks.
- Named slots or scoped slots.
- Wiring generated Go component packages into the generated app layout.
- Cross-package store and asset use syntax.
- Component-scoped CSS and component-level assets.

Component design rules:

- Components must stay portable and must not derive route identity from folder location.
- Component resolution should be explicit enough for diagnostics, editor navigation, and generated code.
- Generated component output must escape untrusted text by default.
- Public generated-runtime contracts belong under `runtime/`.

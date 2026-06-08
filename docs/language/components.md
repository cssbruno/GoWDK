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
- Typed exports metadata blocks such as `exports { selectedID string }`.
- Typed emits metadata blocks such as `emits { select(id string) }`.
- One `view {}` block per component.
- Self-closing component calls such as `<Hero title="GOWDK" />`.
- Wrapper component calls such as `<Panel>...</Panel>` with child content
  rendered into `<slot />` in the component view.
- Named slots with caller-side `<template g:slot="name">...</template>` and
  component-side `<slot name="name">fallback</slot>`.
- Scalar scoped slots with component-side slot props and caller-side `let:`
  bindings, for example `<slot name="item" label={Title} />` consumed by
  `<template g:slot="item" let:label>...</template>`.
- Page-level cross-package component imports with
  `use ui "components"` and qualified calls such as `<ui.Hero />`.
- Component-scoped cross-package component imports with local aliases such as
  `use icons "icons"` and `<icons.Badge />` inside the declaring component.
- Escaped `{prop}` text and attribute interpolation inside component views.
- Component prop values can interpolate page build data, such as a route param
  from literal `paths {}`.
- Slot children render in the caller scope, so page build data and route params
  used inside the child content are resolved before being inserted into the
  component slot. Scoped slot values are injected into that caller scope for
  the slot body.
- A component with `state ...` renders initial state at build time and emits a
  generated JavaScript island by default when called without `g:island`.
- Component-local `client { func Name(...) { ... } }` handlers can group the
  current safe typed expression subset and be called from `g:on:*` with scalar
  expressions, such as `Add(Count + 1)`. The older `fn Name(...)` spelling
  remains accepted.
- Components can dispatch declared events from `client {}` handlers with
  `emit name(Field)`. Parent components can listen on component calls with
  `g:on:name={...}` and receive typed event fields through the compiler-owned
  `event` scope.
- Component-local `computed Name Type { return expr }` values can derive
  read-only browser state from props, state, and other computed values.
  Computed values may also use one Go-style `if` return followed by a fallback
  return. The compiler orders computed values by dependency and rejects cycles.
- Pages can declare first-slice page-scoped stores with
  `store cart ui.CartState = ui.NewCartState()`. Component `client {}` blocks
  can declare explicit dependencies with `use cart`; the compiler validates
  store type/init contracts and rejects unknown store uses. Runtime shared-state
  subscriptions are still planned.
- A component can declare `@wasm ./browser/counter` to compile a browser-side
  Go package and make normal calls to that component use the WASM island
  runtime by default. The package is built with
  `GOOS=js GOARCH=wasm`, must produce a real browser WASM module, and must not
  import server/process/network packages such as `net/http`, `os/exec`,
  `database/sql`, raw `syscall`, `plugin`, or `unsafe`.
- A component call can explicitly request the WASM island artifact with
  `<Counter g:island="wasm" />` when a call-site override is needed. If the
  component has no `@wasm` package, GOWDK still emits the first-slice
  placeholder module plus loader shape for that explicit call.
- Duplicate component names are rejected during manifest validation.
- Redundant component implementations are rejected during manifest validation
  with `redundant_component_implementation`, even when their names differ.
- Component `@css` declarations are parsed, lowered into compiler metadata,
  emitted as scoped component CSS, linked from generated pages, content-hashed,
  manifest-mapped, and served with immutable cache headers. Component `@asset`
  declarations are emitted as content-hashed files under
  `assets/gowdk/components/<package>/<component>/`, manifest-mapped, and served
  with immutable cache headers.
- Component `js "./file.js"` and `js "./file.ts"` declarations emit scoped
  browser module files and link them only from pages that call that component.
  Inline `js {}` blocks are supported for small cases, but path-based modules
  are preferred. GOWDK transforms TypeScript without type-checking and does not
  yet bundle or follow JavaScript imports.
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

## Examples

Default slot:

```gwdk
@component Card

view {
  <section><slot /></section>
}
```

Named slot:

```gwdk
@component Panel

view {
  <section>
    <header><slot name="actions"><span>No actions</span></slot></header>
    <slot />
  </section>
}
```

Scoped slot:

```gwdk
@component Row

props {
  label string
}

view {
  <slot name="item" value={label} />
}
```

```gwdk
view {
  <Row label="Ada">
    <template g:slot="item" let:value>
      <strong>{value}</strong>
    </template>
  </Row>
}
```

Typed emits:

```gwdk
@component Option

props {
  ID string
}

emits {
  select(id string)
}

client {
  fn Pick() {
    emit select(ID)
  }
}
```

Bindable child state is not a stable public contract. Use typed emits plus
parent-owned state instead:

```gwdk
view {
  <Option g:on:select={SelectedID = event.id} />
}
```

Typed exports are metadata today:

```gwdk
exports {
  selectedID string
}
```

Stores are explicit page-scoped UI state:

```gwdk
@route "/cart"
@guard public

store cart ui.CartState = ui.NewCartState()
```

```gwdk
@component CartButton

client {
  use cart
}
```

WASM islands are declared on the component:

```gwdk
@component Counter
@wasm ./browser/counter

view {
  <button>Count</button>
}
```

```gwdk
view {
  <Counter />
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

Imported Go structs are the stable typed prop path today. Non-string inline
props are planned, but inline `props {}` blocks currently accept only `string`.
Defaults should be expressed in normal Go init/build data or by rendering a
fallback in the component `view {}`. There is no rest/spread prop syntax, prop
renaming syntax, or implicit global prop lookup in the current contract.

State is component-local UI state. A `state Type = Init()` declaration runs the
no-argument Go init function at build time for SPA/static output and serializes
the JSON-compatible initial value into the component island. State is visible to
the browser and must not carry secrets, trusted authorization state, database
state, or server validation results that the server still needs to enforce.

Bindable child state is not stable as a parent/child contract. Parent-child
coordination should use typed emits plus parent-owned state, or server actions
for trusted behavior.

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

Store use is explicit. Same-page stores use `client { use cart }`; stores from
another discovered `.gwdk` package require a GOWDK `use` alias and a qualified
client store reference such as `client { use stores.cart }`. Cross-package
stores are validated by alias and store name, not discovered globally.
App-global stores and cross-route persistence are deferred.

Exports are typed component metadata today. They document values a component
intends to expose, but parent pages/components do not yet have a stable runtime
API for consuming exported component values. Until that contract is generated
and documented, use props, typed emits, stores, actions, or build/load data for
actual data flow.

Slots are the reusable-markup primitive. A default slot uses `<slot />`, named
slots use `<slot name="name">`, and scoped slots pass scalar values through
slot props plus caller-side `let:` bindings. GOWDK does not currently have a
separate snippet/render value model.

Recursive component rendering is rejected to prevent unbounded build-time
rendering. Dynamic component selection is deferred; component calls must name a
known component directly or through an explicit `use` alias.

`client {}` is a compiler-owned UI language, not arbitrary JavaScript. The
supported handlers, helpers, lifecycle blocks, effects, refs, list built-ins,
bindings, conditionals, computed values, and scalar expressions are defined in
[syntax.md](syntax.md). Generated island JavaScript interprets that bounded
subset instead of evaluating arbitrary user JavaScript source.

Client handlers run in source order. The generated runtime batches state
updates, recomputes computed values in dependency order, runs cleanup before
effects rerun or unload, and then updates DOM bindings. Async client handlers
are limited to compiler-owned async helpers such as validated
`await fetchJSON[T](urlExpr)` assignments; they cannot return values and do not
change the ownership boundary.

Generated browser runtime behavior is scoped to the island or page enhancement
that requested it. JavaScript may update text, attributes, classes, styles,
form bindings, list rows, local state, page stores, partial responses, and SPA
link transitions. It must not own route existence, auth, business rules,
database access, trusted server validation, action behavior, or cache/loading
policy. Real routes, direct browser refresh, and server behavior stay owned by
the compiler manifest, generated Go, and user Go handlers.

Production WASM islands are declared with component-level `@wasm`. Normal calls
to that component use the WASM island runtime by default. The referenced package
is browser-side Go compiled for `GOOS=js GOARCH=wasm` with server and process
packages rejected. GOWDK validates the required component-scoped ABI entrypoints,
ships Go's browser `wasm_exec.js` runtime asset for declared Go WASM packages,
and keeps DOM mutation in the generated host loader. WASM islands are not a
replacement for backend handlers.

Not implemented yet:

- Non-string props in inline `props {}` blocks.
- Stable parent consumption of typed `exports {}` values.
- Rest/spread props, prop renaming, recursive component rendering, dynamic
  component selection, and bindable child state.
- Full runtime validation for user browser logic in WASM islands, including
  required Go/JS entrypoint registration and export checks.
- Wiring generated Go component packages into the generated app layout.
- Cross-package store and asset use syntax.
- Emitting and rewriting component-scoped CSS and component-level assets from
  the existing component `@css` and `@asset` metadata.

Component design rules:

- Components must stay portable and must not derive route identity from folder location.
- Component resolution should be explicit enough for diagnostics, editor navigation, and generated code.
- Generated component output must escape untrusted text by default.
- Public generated-runtime contracts belong under `runtime/`.

# Components

The first component slice is implemented for SPA build output.

Implemented today:

- Explicit or discovered `.cmp.gwdk` build inputs with `component Name`.
- Optional `props { name string }` declarations, including scalar default
  literals such as `props { count int = 0 }`.
- Inline scalar props with `string`, `int`, `float`, and `bool` types.
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
- A component can declare `wasm ./browser/counter` to compile a browser-side
  Go package and make normal calls to that component use the WASM island
  runtime by default. The package is built with
  `GOOS=js GOARCH=wasm`, must produce a real browser WASM module, and must not
  import server/process/network packages such as `net/http`, `os/exec`,
  `database/sql`, raw `syscall`, `plugin`, or `unsafe`.
- A component call can explicitly request the WASM island artifact with
  `<Counter g:island="wasm" />` when a call-site override is needed. If the
  component has no `wasm` package, GOWDK still emits the first-slice
  placeholder module plus loader shape for that explicit call.
- Duplicate component names are rejected within the same GOWDK package during
  compiler validation. Components in different packages may share a name and
  must be referenced through the calling package's `use` alias.
- Redundant component implementations are rejected during compiler validation
  with `redundant_component_implementation`, even when their names differ.
- Component `css` declarations are parsed, lowered into compiler metadata,
  emitted as scoped component CSS, linked from generated pages, content-hashed,
  manifest-mapped, and served with immutable cache headers. Component `asset`
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

component Hero

view {
  <section><icons.Badge /></section>
}
```

## Examples

Default slot:

```gwdk
component Card

view {
  <section><slot /></section>
}
```

Named slot:

```gwdk
component Panel

view {
  <section>
    <header><slot name="actions"><span>No actions</span></slot></header>
    <slot />
  </section>
}
```

Scoped slot:

```gwdk
component Row

props {
  label string
  count int = 0
  active bool = false
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
component Option

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

Typed exports declare local component values that a parent can observe through
the generated `exports` event:

```gwdk
exports {
  SelectedID string
}

view {
  <button g:on:click={SelectedID = "first"}>{SelectedID}</button>
}
```

Parent components can listen with `g:on:exports`:

```gwdk
view {
  <Picker g:on:exports={CurrentID = event.SelectedID} />
}
```

Stores are explicit page-scoped UI state:

```gwdk
route "/cart"
guard public

store cart ui.CartState = ui.NewCartState()
```

```gwdk
component CartButton

client {
  use cart
}
```

### WASM Islands

WASM islands are declared on the component:

```gwdk
component Counter
wasm ./browser/counter

view {
  <button>Count</button>
}
```

See `examples/components/wasm/` for a runnable component-level WASM island ABI
example with the required browser Go exports.

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

Props are caller-provided inputs. Inline `props {}` declarations support scalar
`string`, `int`, `float`, and `bool` types. Parent calls pass quoted string
props, scalar literal expressions for numbers and booleans, or expression
values from the implemented build-data subset. Props are read-only to
`client {}` code; mutable browser state belongs in `state` or in an explicit
page store.

Imported Go structs are the stable typed prop path for richer contracts.
Inline props can declare static scalar defaults with `name type = literal`.
Defaults are used when a caller omits the prop and are overridden by explicit
caller values.

Advanced prop forwarding stays inside the typed compiler contract:

- `{...props}` may be used inside a component that declares props. It forwards
  only same-named props that the child component also declares; it does not
  expose an arbitrary prop bag or global lookup.
- `target:source` maps a differently named caller prop into a declared child
  prop. Without a value, `target:source` forwards `{source}`. With a value, such
  as `target:source={Expr}` or `target:source="literal"`, the value is used for
  `target` while `source` names the caller-side source for diagnostics.
- Explicit props, spreads, and renames cannot provide the same target prop more
  than once. Unknown target props and unsupported spread sources fail before
  output is written.

State is component-local UI state. A `state Type = Init()` declaration runs the
no-argument Go init function at build time for SPA/static output and serializes
the JSON-compatible initial value into the component island. State is visible to
the browser and must not carry secrets, trusted authorization state, database
state, or server validation results that the server still needs to enforce.

Bindable child state is supported on component calls with
`g:bind:<ExportedState>={ParentState}`. The target must be a child state field,
must be declared in `exports`, and must have a scalar type compatible with the
parent state field. Generated JavaScript sends the parent value down through
reactive props and listens for the child's typed `exports` event to write the
new child value back to parent state. A single component call may bind several
exported fields at once; the generated assignments share the one `exports`
listener and run as ordered statements. A component may not export a field named
`active`, which the exports payload reserves for the mount-state flag. Bound
state is still local UI state: it is not trusted input, server state, auth state,
validation, business logic, route truth, or cache policy.

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
App-global stores are deferred.

A `use` can carry the store's Go type so the component can reference the store's
fields directly, without redeclaring a matching `state` shape:

```gwdk
component CartBadge

client {
  use cart ui.CartState

  computed Label string { return string(Count) }
}

view {
  <span>{Label}</span>
}
```

`use cart ui.CartState` binds `CartState`'s fields (here `Count`) into the
component's client scope. The type is resolved against the component's own
imports, so a reusable component stays self-describing even when different pages
declare a same-named store; the annotated type is the component's contract for
the store's shape, exactly as a local `state` declaration was. The island seeds
those fields with the type's zero values for SSR and adopts the store's real
(init or persisted) value on mount; keep a local `state <Type> = <init>` if you
need the store's init value reflected in the server-rendered HTML.

A page store can opt into browser persistence with a `persist` modifier:

```gwdk
store cart  ui.CartState = ui.NewCartState() persist "local"
store prefs ui.UIPrefs   = ui.DefaultPrefs() persist "session"
```

`persist "local"` keeps the store in `localStorage` (survives a browser
restart); `persist "session"` keeps it in `sessionStorage` (survives reload and
SPA navigation, cleared when the tab closes). Persistence is keyed by store name
(`gowdk:store:<name>`), so the same persisted store keeps its value across routes
on the origin. Only the store's own declared fields are persisted — never
component state, props, or computed values. The compiler embeds a hash of the
store's struct shape; when the shape changes, stale persisted state is discarded
rather than restored, so a struct change never crashes on old data. Because
browser storage is readable by any script on the origin, persisting a field whose
name resembles a secret (`token`, `password`, `secret`, `auth`, …) is a warning —
including a nested field such as `Profile.Token`, because persistence writes the
whole value of each top-level field: keep credentials and trusted authorization
state server-side. An unknown scope is rejected — see
`gowdk explain page_store_persist_scope_invalid`.

`persist "local"` stores also sync across tabs: when one tab writes, other tabs
on the origin mirror the value through the browser `storage` event. `persist
"session"` stores are deliberately tab-local — `sessionStorage` is partitioned
per top-level tab, so session-scoped stores do not (and cannot) sync across tabs.
To drop a persisted store (for example after checkout or logout), use the
bounded `clear <store>` statement inside a client function, mount, destroy, or
effect block:

```gwdk
client {
  use cart

  func Checkout() {
    clear cart
  }
}
```

`clear cart` lowers to `window.__gowdkStores.clear("cart")`, which removes the
stored copy and resets the store to its build-time init value, notifying every
island that uses it. A component may only clear a store it `use`s; clearing an
unused store is a compile error. If two pages persist a store with the
same name but different shapes, they share one storage key and discard each
other's data on navigation; the compiler warns with
`page_store_persist_key_conflict`. If they share the same shape but declare
different `local`/`session` scopes, the runtime keeps whichever scope initialized
first and the compiler warns with `page_store_persist_scope_conflict`. A store
first reached on a route that does not persist it still adopts persistence when a
later route declares it, restoring the saved value regardless of navigation order.

Persistence survives SPA navigation: when the client runtime swaps page content
it re-scans store seeds, so a store first declared on a later client-side route
hydrates without a full page load, and a store already in memory keeps its value.

WASM islands participate in page stores too. The host loader merges every used
store's current (and persisted) value into the mount/handle/destroy payload's
`state`, writes back any store values an export returns in the extended
`{ patches, stores }` result shape, and re-invokes the island when another island
changes a used store. Go WASM exports can use `runtime/wasm` to decode the
current payload and return either a patch array or `{ patches, stores }` through
the required `func() uint32` ABI; see `examples/components/wasm/README.md`.

Current limits. Invalid persist scopes are reported but not auto-fixed, because
choosing `local` vs `session` is a deliberate decision.

Exports must reference a declared prop, state field, or computed value and the
declared type must match that local symbol. Generated JavaScript islands emit
an `exports` event with `event.active == true` after mount and updates, plus a
`gowdk:exports` DOM event for direct integrations. Before unmount, the runtime
emits the same events with `event.active == false` and exported values set to
`null`, so parent code can clear local handles. Exports are local UI handles;
they are not server state, trusted input, or a replacement for backend actions.

Slots are the reusable-markup primitive. A default slot uses `<slot />`, named
slots use `<slot name="name">`, and scoped slots pass scalar values through
slot props plus caller-side `let:` bindings. GOWDK does not currently have a
separate snippet/render value model.

Recursive component rendering is rejected to prevent unbounded build-time
rendering; direct and transitive cycles fail before output is written. Dynamic
component selection is rejected; component calls must name a known component
directly or through an explicit `use` alias.

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

For loading/error UI that does not need a hand-written `Loading` or `Error`
state field, component views can use `{#await fetchJSON[T](urlExpr)}` blocks.
The block is local to the JS island, renders pending/then/catch branches, and
does not expose arbitrary JavaScript promises.

Client `g:if` branches and keyed client `g:for` rows can opt into CSS-driven
motion with `g:transition="name"`; keyed client `g:for` rows can opt into
reorder hooks with `g:animate="name"`. The generated runtime toggles
`gowdk-transition-*` and `gowdk-animate-*` classes only. User or addon CSS owns
all durations, easing, transforms, and `prefers-reduced-motion` behavior.

Generated browser runtime behavior is scoped to the island or page enhancement
that requested it. JavaScript may update text, attributes, classes, styles,
form bindings, list rows, local state, page stores, partial responses, and SPA
link transitions. It must not own route existence, auth, business rules,
database access, trusted server validation, action behavior, or cache/loading
policy. Real routes, direct browser refresh, and server behavior stay owned by
the compiler manifest, generated Go, and user Go handlers.

Production WASM islands are declared with component-level `wasm`. Normal calls
to that component use the WASM island runtime by default. The referenced package
is browser-side Go compiled for `GOOS=js GOARCH=wasm` with server and process
packages rejected. GOWDK validates the required component-scoped ABI entrypoints,
ships Go's browser `wasm_exec.js` runtime asset for declared Go WASM packages,
passes `gowdk-wasm-island-v1` payloads to component WASM exports, and keeps DOM
mutation in the generated host loader. WASM islands are not a replacement for
backend handlers.

The runnable ABI example in `examples/components/wasm/` builds a component
WASM asset, per-component host loader, and `wasm_exec.js` from a browser Go
package with the required `GOWDKMount<Component>`,
`GOWDKHandle<Component>`, and `GOWDKDestroy<Component>` exports.

Not implemented yet:

- Supported recursive component rendering and supported dynamic component
  selection.
- Full runtime validation for user browser logic in WASM islands beyond
  required export, browser import, and patch-operation checks.
- Wiring generated Go component packages into the generated app layout.
- Cross-package store and asset use syntax.
- Emitting and rewriting component-scoped CSS and component-level assets from
  the existing component `css` and `asset` metadata.

Component design rules:

- Components must stay portable and must not derive route identity from folder location.
- Component resolution should be explicit enough for diagnostics, editor navigation, and generated code.
- Generated component output must escape untrusted text by default.
- Public generated-runtime contracts belong under `runtime/`.

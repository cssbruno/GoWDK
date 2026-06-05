# Components

The first component slice is implemented for static build output.

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

Not implemented yet:

- Non-string props in legacy `props {}` blocks.
- Expression props.
- Client function locals, return values, loops, or arbitrary browser logic.
- Full runtime validation for user browser logic in WASM islands, including
  required Go/JS entrypoint registration and export checks.
- Named slots or scoped slots.
- Wiring generated Go component packages into the generated app layout.

Component design rules:

- Components must stay portable and must not derive route identity from folder location.
- Component resolution should be explicit enough for diagnostics, editor navigation, and generated code.
- Generated component output must escape untrusted text by default.
- Public generated-runtime contracts belong under `runtime/`.

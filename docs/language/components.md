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
- A component call can explicitly request the first WASM island artifact slice
  with `<Counter g:island="wasm" />`.
- Duplicate component names are rejected during manifest validation.
- Redundant component implementations are rejected during manifest validation
  with `redundant_component_implementation`, even when their names differ.

Not implemented yet:

- Non-string props in legacy `props {}` blocks.
- Expression props.
- Client function locals, return values, loops, or arbitrary browser logic.
- Real user browser logic in WASM islands; the current WASM path emits a
  minimal valid module plus loader only for explicit `g:island="wasm"` calls.
- Named slots or scoped slots.
- Wiring generated Go component packages into the generated app layout.

Component design rules:

- Components must stay portable and must not derive route identity from folder location.
- Component resolution should be explicit enough for diagnostics, editor navigation, and generated code.
- Generated component output must escape untrusted text by default.
- Public generated-runtime contracts belong under `runtime/`.

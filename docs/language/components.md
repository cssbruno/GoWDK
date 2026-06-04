# Components

The first component slice is implemented for static build output.

Implemented today:

- Explicit or discovered `.cmp.gwdk` build inputs with `@component Name`.
- Optional `props { name string }` declarations.
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
- Duplicate component names are rejected during manifest validation.

Not implemented yet:

- Non-string props.
- Expression props.
- Named slots or scoped slots.
- Wiring generated Go component packages into the generated app layout.

Component design rules:

- Components must stay portable and must not derive route identity from folder location.
- Component resolution should be explicit enough for diagnostics, editor navigation, and generated code.
- Generated component output must escape untrusted text by default.
- Public generated-runtime contracts belong under `runtime/`.

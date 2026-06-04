# Components

The first component slice is implemented for static build output.

Implemented today:

- Explicit or discovered `.cmp.gwdk` build inputs with `@component Name`.
- Optional `props { name string }` declarations.
- One `view {}` block per component.
- Self-closing component calls such as `<Hero title="GOWDK" />`.
- Escaped `{prop}` text and attribute interpolation inside component views.
- Component prop values can interpolate page build data, such as a route param
  from literal `paths {}`.
- Duplicate component names are rejected during manifest validation.

Not implemented yet:

- Component children or slots.
- Non-string props.
- Expression props.
- Component-to-component calls as a documented contract.
- Generated Go component source.

Component design rules:

- Components must stay portable and must not derive route identity from folder location.
- Component resolution should be explicit enough for diagnostics, editor navigation, and generated code.
- Generated component output must escape untrusted text by default.
- Public generated-runtime contracts belong under `runtime/`.

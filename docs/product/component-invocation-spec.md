# Feature Spec: Minimal Component Invocation

## Problem

GOWDK can now emit static HTML for one simple page, but v0.1 needs reusable `.cmp.gwdk` components. The first useful component slice is a page that invokes a component file and passes static string props into it.

## Goals

- Parse `.cmp.gwdk` files with `@component`, optional `props {}`, and `view {}`.
- Resolve one capitalized component invocation from a page during `gowdk build`.
- Pass static quoted string props into the component.
- Render `{prop}` text interpolation inside component markup with escaping.
- Add a buildable component example and tests.

## Non-Goals

- Component children or slots.
- Expression props, boolean props, spread props, or non-string prop types.
- Component imports, namespaces, or package-level visibility.
- Component-to-component invocation.
- Generated Go component source.

## Users And Permissions

- Primary users: early GOWDK contributors building the first component compiler slice.
- Roles or permissions: local CLI user with read access to input files and write access to the output directory.
- Data visibility rules: component props and generated HTML stay local to the requested build.

## User Flow

1. User writes `hero.cmp.gwdk` with `@component Hero`, `props { title string }`, and `view {}`.
2. User writes a page that calls `<Hero title="GOWDK" />`.
3. User runs `gowdk build --out dist page.gwdk hero.cmp.gwdk`, or runs
   `gowdk build --out dist` in a directory where both files are discoverable.
4. GOWDK emits static HTML with the component expanded.

## Requirements

### Functional

- Component files must be passed explicitly to `gowdk build` or discovered by
  default build discovery.
- Component names must be capitalized.
- Prop declarations support `name string`.
- Component calls support self-closing capitalized tags with quoted string attributes.
- Missing components and missing required props must fail before writing output.
- `{prop}` in component text must render the escaped prop value.

### Non-Functional

- Performance: keep the first resolver in-memory and dependency-free.
- Reliability: fail the whole build before writing files when any component cannot render.
- Accessibility: preserve semantic HTML written inside component views.
- Security/privacy: escape prop values before they enter HTML output.
- Observability: build errors must name the missing component or prop.

## Acceptance Criteria

- [x] `gowdk build --out <dir> examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk` writes expanded component HTML.
- [x] Missing component references fail with an actionable error.
- [x] Missing required string props fail with an actionable error.
- [x] Prop interpolation is escaped.
- [x] `go test ./...` passes.
- [x] `go build ./cmd/gowdk` passes.

## Edge Cases

- Duplicate component names fail validation.
- Component files passed to `gowdk check` are parsed and validated for manifest
  identity, but full component semantic analysis remains out of scope for this
  slice.
- Component children should fail until slot semantics are defined.

## Dependencies

- Internal: `internal/parser`, `internal/manifest`, `internal/view`, `internal/staticgen`, `internal/lang`.
- External: none.

## Open Questions

- Whether component props eventually belong in `.gwdk`, Go types, or both.
- How project config loading should customize component discovery.

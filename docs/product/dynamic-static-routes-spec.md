# Feature Spec: Dynamic Static Routes

## Problem

Dynamic static routes are part of the GOWDK product model. The compiler now
records `paths {}` body text, but static builds still need a first executable
subset so pages like `/blog/{slug}` can prerender concrete files without SSR.

## Goals

- Preserve `paths {}` body text in the internal manifest.
- Add a buildable-language example for a dynamic static route that declares
  route params through `paths {}`.
- Keep `gowdk check`, `gowdk manifest`, and `gowdk sitemap` working for dynamic
  static routes with `paths {}`.
- Let `gowdk build` expand the first literal `paths {}` subset into concrete
  static files.
- Bind literal route params into the current static `view {}` rendering context.
- Report invalid, missing, unused, or duplicate generated path declarations
  before writing output.

## Non-Goals

- Evaluating Go code, loading data sources, or running user build functions.
- Binding route params into `build {}` data.
- Evaluating non-route data expressions inside `view {}`.
- Defining the final full `paths {}` statement grammar.

## Users And Permissions

- Primary users: early GOWDK contributors and Go developers validating static
  route declarations.
- Roles or permissions: local read access to `.gwdk` files.
- Data visibility rules: captured `paths {}` bodies remain internal compiler
  metadata and are not emitted in public manifest JSON.

## User Flow

1. A user writes `@route "/blog/{slug}"` on a static page.
2. The user adds a `paths {}` block with literal parameter sets.
3. `gowdk check` accepts the page because dynamic static routes have a
   `paths {}` block.
4. The page `view {}` can render `{slug}` in text, attribute values, or
   component prop values.
5. `gowdk build` emits one HTML file for each declared parameter set.

## Requirements

### Functional

- The parser must capture source text between `paths {` and the matching
  top-level `}` line.
- Missing closing braces for captured `paths {}` blocks must produce a parser
  error.
- The internal manifest must retain the captured body text.
- Public manifest JSON continues to expose only the `paths` boolean.
- `gowdk build` must parse the first supported literal path declaration syntax:
  `=> { slug: "hello-gowdk" }`.
- One non-empty literal string value must be provided for every route parameter.
- Unused params in `paths {}` must fail the build.
- Duplicate params in one `paths {}` declaration must fail the build.
- Duplicate generated output paths must fail before any files are written.
- Example files must include one buildable dynamic static route with a non-empty
  `paths {}` body.
- Static build output includes one HTML file for each declared path.
- Route params are available to page `view {}` interpolation.
- Route params can be used in static HTML attribute values.
- Route params can be used as component prop values.
- Missing route-param interpolation in a page or component prop must fail before
  writing output.

### Non-Functional

- Performance: keep line-oriented parsing dependency-free.
- Reliability: fail before writing partial output when path expansion is invalid.
- Accessibility: no UI impact.
- Security/privacy: do not expose `paths {}` body text in generated public JSON.
- Observability: route manifest output records every generated concrete route.

## Acceptance Criteria

- [x] Parser tests prove `paths {}` body capture.
- [x] Parser tests prove unclosed `paths {}` fails.
- [x] A dynamic static route example passes `gowdk check`.
- [x] `gowdk manifest --ssr examples/basic/*.gwdk` shows the dynamic route with
  `"paths": true`.
- [x] `gowdk build` emits concrete files for the supported literal `paths {}`
  subset.
- [x] `gowdk-routes.json` records every generated concrete dynamic route.
- [x] Invalid `paths {}` declarations fail before partial output is written.
- [x] Route params render in generated dynamic page HTML.
- [x] Route params can be passed to component props during static generation.

## Edge Cases

- Empty `paths {}` is invalid for dynamic static builds because there is no
  route to generate.
- Lines inside `paths {}` that do not match the literal subset are rejected at
  build time.
- Nested braces inside a single body line are preserved, but a line containing
  only `}` closes the captured block.
- Route parameter values must not produce unsafe path segments.
- Component internals still render only declared component props; route params
  must be passed explicitly as component prop values.

## Dependencies

- Internal: `internal/parser`, `internal/manifest`, `internal/staticgen`,
  examples, docs.
- External: none.

## Open Questions

- Should route expansion execute embedded Go-like source, call generated Go
  functions, or consume static manifest data?
- How should route params become available to `build {}`?

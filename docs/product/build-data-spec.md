# Feature Spec: Literal Build Data

## Problem

Static pages can now render literal route params from `paths {}`, but non-dynamic
static pages still cannot feed simple build-time data into `view {}`. GOWDK
needs a first `build {}` execution subset so compile-first pages can render
literal content without SSR or request-time logic.

## Goals

- Preserve `build {}` body text in the internal manifest.
- Support a first literal build data declaration syntax:
  `=> { title: "Hello" }`.
- Make literal build data available to the current static `view {}`
  interpolation context.
- Merge literal build data with literal route params for dynamic static pages.
- Fail before writing output when build data is malformed or conflicts with
  route params.

## Non-Goals

- Execute Go code, expressions, loops, conditionals, imports, or data-source
  calls.
- Bind route params into `build {}` expressions.
- Support non-string build data.
- Support multiple build data return statements.

## Users And Permissions

- Primary users: Go developers building static pages with compile-time content.
- Roles or permissions: local CLI user with write access to the selected output
  directory.
- Data visibility rules: build data is rendered into generated HTML only when
  referenced by `view {}`.

## User Flow

1. A user writes a static page with a literal `build {}` block.
2. The user references returned build data with `{title}` in `view {}`.
3. `gowdk build` renders the page with escaped build-time values.

## Requirements

### Functional

- The parser captures source text between `build {` and the matching top-level
  `}` line.
- `gowdk build` parses at most one literal build data declaration:
  `=> { key: "value" }`.
- Build data names must be valid identifiers.
- Build data values must be non-empty string literals.
- Duplicate build data names must fail.
- Build data names that collide with route params must fail.
- Build data is available to page text, page attributes, and component prop
  values through the existing static interpolation path.

### Non-Functional

- Performance: line-oriented parsing with no new dependencies.
- Reliability: invalid build data fails before writing partial output.
- Accessibility: no direct impact.
- Security/privacy: interpolated build data remains HTML-escaped.
- Observability: generated HTML and build errors show the effective behavior.

## Acceptance Criteria

- [x] Parser tests prove `build {}` body capture.
- [x] Static build renders literal build data in page text and attributes.
- [x] Static build can pass literal build data into component string props.
- [x] Static build rejects malformed build data before writing output.
- [x] Static build rejects build data names that collide with route params.

## Edge Cases

- Empty `build {}` remains valid and provides no data.
- Multiple literal build data declarations are rejected for now.
- Comments and blank lines in `build {}` are ignored.

## Dependencies

- Internal: `internal/parser`, `internal/manifest`, `internal/staticgen`,
  `internal/view`.
- External: none.

## Open Questions

- What exact Go-like expression grammar should replace the literal subset?
- How should future route params be exposed inside `build {}`?

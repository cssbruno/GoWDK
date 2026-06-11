# Feature Spec: Plain Keyword Metadata

## Problem

GOWDK metadata previously used an annotation-style sigil for declarations such
as `route`, `guard`, and `component`. That visual style reads like decorators
or runtime annotations to Go developers, which conflicts with GOWDK's
compile-first, Go-first language positioning.

## Goals

- Make bare metadata keywords the only `.gwdk` syntax.
- Use `route`, `guard`, `component`, `wasm`, `css`, `asset`, and related
  metadata without `@`.
- Replace endpoint-local `error` suffixes with `error`.
- Update docs, examples, snippets, tests, and editor support to the new syntax.

## Non-Goals

- Add a migration compatibility layer for old `@` syntax.
- Change block semantics, route behavior, guards, CSS selection, or generated
  output contracts.

## Users And Permissions

- Primary users: Go developers authoring `.gwdk` pages, components, layouts,
  examples, and generated app inputs.
- Roles or permissions: unchanged.
- Data visibility rules: unchanged.

## User Flow

1. A user writes `.gwdk` metadata with plain keywords.
2. The compiler parses the file and emits the same IR and generated output as
   the former annotation syntax.
3. Old sigil syntax fails fast with a parser diagnostic.

## Requirements

### Functional

- Top-level metadata declarations must parse without `@`.
- Legacy sigil metadata declarations must be rejected.
- Action and API error-page suffixes must use `error "<path.html>"`.
- Formatter, LSP, VS Code snippets, docs, and examples must show bare keywords.

### Non-Functional

- Performance: no meaningful parser performance change.
- Reliability: existing source spans and validation paths remain stable.
- Accessibility: no UI behavior impact.
- Security/privacy: no generated runtime behavior impact.
- Observability: diagnostics remain explicit for malformed old syntax.

## Acceptance Criteria

- [ ] Parser tests cover bare page, route, guard, layout, component, wasm, css,
  and asset metadata.
- [ ] Parser tests reject representative old `@` metadata.
- [ ] CLI/lang/editor tests and golden fixtures use bare syntax.
- [ ] Documentation and examples no longer teach top-level `@` metadata.
- [ ] Relevant Go tests pass.

## Edge Cases

- Endpoint-local error pages use `error`, not `error`.
- CSS at-rules inside `style {}` bodies are unaffected.
- Mentions of `@` in changelogs or historical text can remain only when
  describing removed legacy syntax.

## Dependencies

- Internal: parser, lang tools, LSP/editor helpers, examples, docs.
- External: none.

## Open Questions

- None for this change.

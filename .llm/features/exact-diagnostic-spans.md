# Feature Spec: Exact Diagnostic Spans

## Problem

M2 diagnostics should point at the smallest useful source range so CLI output,
editor integrations, and `gowdk explain` navigation land on the token that needs
attention. Route validation already carries source spans, but duplicate route
parameters used name-only lookup and could point at the first parameter instead
of the repeated one.

## Goals

- Keep route, endpoint, fragment, component, view, build, and load diagnostics
  anchored to precise spans.
- Lock endpoint route validation to route literal or parameter spans.
- Point duplicate route parameter diagnostics at the duplicate occurrence.

## Non-Goals

- Redesign diagnostic JSON.
- Replace parser recovery or formatter behavior.
- Add LSP protocol support in this slice.

## Users And Permissions

- Primary users: CLI users, editor integrations, and contributors.
- Roles or permissions: none.
- Data visibility rules: no user data impact.

## User Flow

1. A source file contains a malformed endpoint path or repeated route parameter.
2. The compiler emits a diagnostic with a source range.
3. CLI/editor tooling highlights the exact route literal or parameter segment.

## Requirements

### Functional

- Route diagnostics with parameter-specific failures use the matching parameter
  span when available.
- Duplicate route parameter diagnostics use the duplicate occurrence span.
- Fragment dynamic-route diagnostics use the dynamic parameter span.
- Tests cover action, API, fragment, and page route span behavior.

### Non-Functional

- Performance: no additional source parsing in validation.
- Reliability: behavior is deterministic from manifest spans.
- Accessibility: precise spans improve editor and terminal navigation.
- Security/privacy: no runtime data impact.
- Observability: no new runtime telemetry.

## Acceptance Criteria

- [x] Malformed action/API endpoint parameter types point at parameter spans.
- [x] Fragment dynamic-route diagnostics point at the dynamic segment.
- [x] Duplicate page route parameters point at the duplicate occurrence.
- [x] Focused compiler tests pass.

## Edge Cases

- If a parser or Go endpoint scanner cannot provide parameter spans, diagnostics
  fall back to the route declaration span.

## Dependencies

- Internal: manifest source spans, compiler route validation.
- External: GitHub issue #78 in milestone M2.

## Open Questions

- Should a later slice expose source span examples in `gowdk explain --json`?

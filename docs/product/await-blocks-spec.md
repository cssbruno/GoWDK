# Feature Spec: Bounded Await Blocks

## Problem

Client islands that fetch JSON need local loading and error UI without hand
maintaining `Loading` and `Error` state fields around every async handler.
Before this feature, the markup parser rejected `{#await}` entirely, so island
authors had to spread async control flow across client state, handler code, and
conditional markup.

## Goals

- Support a bounded `{#await}` markup block inside JS client islands.
- Render pending, resolved, and error branches around compiler-owned async
  fetches.
- Keep async execution deterministic and local to the island runtime.
- Preserve the bounded client language: no arbitrary Promise or raw JavaScript
  access from markup.

## Non-Goals

- Supporting `g:await` or `g:async` directives.
- Supporting arbitrary JavaScript promises.
- Supporting value-returning async helper functions before the broader client
  return-type work lands.

## Users And Permissions

- Primary users: GOWDK component authors building browser-enhanced islands.
- Roles or permissions: no special roles.
- Data visibility rules: fetched data is visible only to the client island that
  requested it.

## User Flow

1. The author writes `{#await fetchJSON[T](urlExpr)}` in a component view.
2. The pending branch renders immediately.
3. The runtime fetches JSON, swaps in the `{:then name}` branch on success, or
   swaps in the `{:catch err}` branch on failure.

## Requirements

### Functional

- Parse `{#await expr}`, `{:then name}`, optional `{:catch err}`, and `{/await}`.
- Validate that `expr` is `fetchJSON[T](urlExpr)` and that `urlExpr` is a
  bounded client expression.
- Expose the resolved value to the `then` branch and an error object with
  `message` to the `catch` branch.
- Re-run nested client conditionals, loops, bindings, and event handlers after
  a branch swap.
- Abort stale fetches when an await expression is replaced or the island is
  destroyed.

### Non-Functional

- Performance: fetch and render only within the owning island.
- Reliability: stale async results must not replace newer branch state.
- Accessibility: authors control accessible pending/error markup.
- Security/privacy: only root-relative or otherwise compiler-validated URL
  expressions already allowed by `fetchJSON` may be used.
- Observability: fetches keep the existing island trace lane.

## Acceptance Criteria

- [x] Parser/model/render tests cover valid and invalid await blocks.
- [x] Browser/runtime tests cover pending, success, error, and nested branch
      bindings.
- [x] Docs describe the supported syntax and exclusions.
- [x] Existing async handler behavior remains unchanged.

## Edge Cases

- Missing `{:then}` is rejected.
- Duplicate `{:then}` or `{:catch}` is rejected.
- Await blocks outside component islands are rejected.
- Branch variables must be valid identifiers and must not collide with each
  other.

## Dependencies

- Internal: view parser/model, view renderer, JS island runtime, client
  expression validator.
- External: browser `fetch` and `AbortController` when available.

## Open Questions

- Value-returning async helper functions should be handled with #501 rather
  than this bounded markup slice.

# ADR 0008: Bounded Client Language

## Status

Accepted

## Context

GOWDK needs browser-side behavior for local UI state, progressive form
enhancement, partial updates, and explicit islands. The risk is letting that
browser behavior become a second application runtime that owns routes, auth,
business rules, data loading, or trusted validation.

External JavaScript framework reactivity models are useful comparison points,
but GOWDK's product direction is Go-first and compile-first. The client
language must stay small enough for the compiler to parse, type-check, format,
generate, and explain.

## Decision

`client {}` is a bounded GOWDK language for local component and page-enhancement
behavior. It is not arbitrary JavaScript and it is not a route, auth, database,
business-rule, or server-validation layer.

Supported client behavior should grow through explicit compiler-owned syntax:

- typed handlers and helper functions;
- scalar expression evaluation;
- computed values with dependency ordering and cycle diagnostics;
- lifecycle hooks and effect cleanup;
- DOM refs with a small safe method set;
- local bindings, class/style toggles, conditionals, and keyed lists;
- compiler-owned async helpers when their ordering and failure behavior is
  documented.

Page stores are page-scoped UI state. They can be initialized from normal Go
types/functions and used by components that explicitly declare `client { use
storeName }` or a qualified GOWDK source alias. They are browser enhancement
state, not global application authority.

Generated JavaScript may update local island state, page stores, text,
attributes, classes, styles, list rows, partial responses, and SPA navigation
enhancements. It must not own route existence, auth, business rules, database
access, trusted server validation, action behavior, global application state,
loading policy, or cache policy.

Cross-package stores are allowed only through explicit GOWDK `use` aliases and
validated store names. App-global stores, implicit store lookup, and
cross-route persistence are deferred.

Async client handlers are allowed only through compiler-owned async functions
and helpers, such as validated `await fetchJSON[...]`. Async functions cannot
return values. Await is rejected outside async handlers. The generated runtime
must preserve deterministic update ordering: handler statements run in source
order, awaited assignments resume in source order for that handler, computed
values update after state changes, and DOM bindings update after computed
values settle.

## Consequences

- GOWDK can add richer browser behavior without making JavaScript the app
  contract.
- Client-language syntax needs explicit parser, type-checker, formatter, LSP,
  and generated-runtime support before it is documented as stable.
- Stores can support shared island UI state while preserving Go/server
  authority for trusted behavior.
- Future broad reactivity features need ADR-level scrutiny if they would change
  these ownership boundaries.

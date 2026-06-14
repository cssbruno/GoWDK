# ADR 0003: JS Default, Component-Declared WASM Islands

Date: 2026-06-04

Status: Accepted

## Context

GOWDK needs local component interactivity without becoming a React, Svelte, or
npm-centered runtime. App-shell HTML remains the first output, server fragments
handle action-driven updates, and local state needs a small browser runtime.

WASM is still important for richer browser-side Go logic, but making it the
implicit runtime for every stateful component would force simple counters,
toggles, and disclosure widgets through a heavier runtime and make them harder
to inspect.

## Decision

Stateful components use generated JavaScript islands by default. A component
with `state <alias>.<Type> = <alias>.<Init>()` renders initial state at build
time and emits `assets/gowdk/islands/<package>/<Component>.js` when a page
calls it without an island override.

WASM is declared on the component with `wasm <package>`. Normal calls to that
component emit `assets/gowdk/islands/<package>/Counter.wasm` and
`assets/gowdk/islands/<package>/Counter.wasm.js`. `g:island="wasm"` remains
available as a call-site override for compatibility and targeted experiments. Unknown
`g:island` values are compiler errors.

## Consequences

### Positive

- The default interactive path stays dependency-free, inspectable, and small.
- App-shell HTML remains the initial output for stateful components.
- WASM work has an explicit component-level opt-in boundary instead of becoming
  a hidden default for every stateful component.

### Negative

- GOWDK now has two island asset paths to keep documented and tested.
- The generated JavaScript expression subset must stay intentionally narrow
  until a broader client model is designed.

### Neutral

- ADR 0004 defines the production WASM island ABI. Broader user-code ergonomics
  can evolve without making WASM the default component runtime.

## Alternatives Considered

- Make WASM the default island runtime. Rejected because it is too heavy for
  common scalar UI state and weakens build-time inspectability.
- Wait for a full WASM design before adding local state. Rejected because
  generated JavaScript can handle the first useful stateful component slice
  without adding npm or full-page hydration.

## Follow-Up

- Keep ADR 0004 as the production WASM island ABI source of truth.
- Expand generated JavaScript only through explicit, tested expression slices.

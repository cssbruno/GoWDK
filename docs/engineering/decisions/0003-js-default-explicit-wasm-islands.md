# ADR 0003: JS Default, Explicit WASM Islands

Date: 2026-06-04

Status: Accepted

## Context

GOWDK needs local component interactivity without becoming a React, Svelte, or
npm-centered runtime. App-shell HTML remains the first output, server fragments
handle action-driven updates, and local state needs a small browser runtime.

WASM is still important for future richer browser-side Go logic, but making it
implicit would force every stateful component through a heavier runtime and make
simple counters, toggles, and disclosure widgets harder to inspect.

## Decision

Stateful components use generated JavaScript islands by default. A component
with `state <alias>.<Type> = <alias>.<Init>()` renders initial state at build
time and emits `assets/gowdk/islands/<Component>.js` when a page calls it
without an island override.

WASM is explicit per component instance. `<Counter g:island="wasm" />` emits
`assets/gowdk/islands/Counter.wasm` and
`assets/gowdk/islands/Counter.wasm.js`. Unknown `g:island` values are compiler
errors.

## Consequences

### Positive

- The default interactive path stays dependency-free, inspectable, and small.
- App-shell HTML remains the initial output for stateful components.
- Future WASM work has an explicit opt-in boundary instead of becoming a hidden
  default.

### Negative

- GOWDK now has two island asset paths to keep documented and tested.
- The generated JavaScript expression subset must stay intentionally narrow
  until a broader client model is designed.

### Neutral

- The first WASM artifact slice only proves asset emission and loading shape; a
  production WASM island ABI is still future work.

## Alternatives Considered

- Make WASM the default island runtime. Rejected because it is too heavy for
  common scalar UI state and weakens build-time inspectability.
- Wait for a full WASM design before adding local state. Rejected because
  generated JavaScript can handle the first useful stateful component slice
  without adding npm or full-page hydration.

## Follow-Up

- Add an ADR for the production WASM island ABI before arbitrary browser-side Go
  logic is supported.
- Expand generated JavaScript only through explicit, tested expression slices.

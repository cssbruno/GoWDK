# Feature Spec: Bounded Markup Transitions

## Problem

Stateful components can mount and unmount `g:if` branches and keyed `g:for`
rows, but authors have no compiler-owned way to attach CSS-driven enter, leave,
or reorder states to those lifecycle points. Using unsupported
`g:transition`/`g:animate` currently fails, so authors must duplicate lifecycle
state by hand.

## Goals

- Support a bounded `g:transition` directive for client `g:if` branches and
  keyed client `g:for` rows.
- Support a bounded `g:animate` directive for keyed client `g:for` row reorder
  animation hooks.
- Keep animation values in user or addon CSS; the runtime only toggles stable
  classes and data attributes.
- Preserve deterministic island updates and enhancement-only behavior.

## Non-Goals

- No JavaScript animation engine or built-in animation durations/easings.
- No dynamic transition-name expressions in this slice.
- No server-lane `server {}` transition behavior.
- No component-call lifecycle directives in this slice; the existing client
  lifecycle is attached to HTML elements.

## Users And Permissions

- Primary users: GOWDK component authors.
- Roles or permissions: none.
- Data visibility rules: no new data is exposed; directive values are literal
  CSS hook names.

## User Flow

1. Author declares CSS for the generated transition or animation classes.
2. Author adds `g:transition="fade"` to a client `g:if` branch or keyed
   `g:for` row.
3. Author optionally adds `g:animate="reorder"` to a keyed `g:for` row.
4. The generated island runtime toggles classes when rows or branches enter,
   leave, or move.

## Requirements

### Functional

- `g:transition` accepts one literal motion name on the same element as a
  client `g:if`, `g:else-if`, `g:else`, or keyed client `g:for`.
- `g:animate` accepts one literal motion name on the same element as keyed
  client `g:for`.
- Generated HTML emits `data-gowdk-transition` and `data-gowdk-animate`.
- Runtime transition classes are:
  `gowdk-transition`, `gowdk-transition-<name>`,
  `gowdk-transition-enter`, `gowdk-transition-enter-from`,
  `gowdk-transition-enter-to`, `gowdk-transition-leave`,
  `gowdk-transition-leave-from`, and `gowdk-transition-leave-to`.
- Runtime animation classes are:
  `gowdk-animate`, `gowdk-animate-<name>`, and `gowdk-animate-move`.
- Misuse fails at build/render time with a targeted error.

### Non-Functional

- Performance: list diffing remains keyed and local to the island root.
- Reliability: interrupted leave transitions can be reversed without removing
  the node.
- Accessibility: docs direct authors to honor `prefers-reduced-motion`.
- Security/privacy: literal names are validated as CSS-safe identifiers.
- Observability: no new runtime telemetry.

## Acceptance Criteria

- [x] Parser accepts `g:transition` and `g:animate`.
- [x] Build output emits the expected data attributes for client lifecycle
      elements.
- [x] JS island runtime toggles enter, leave, interrupted-leave, and move
      classes.
- [x] Misuse diagnostics cover static elements, invalid names, and `g:animate`
      outside keyed lists.
- [x] Language docs and stability tables describe the supported slice.

## Edge Cases

- A hidden initial branch should not animate until it is mounted by the client.
- A leave transition interrupted by a remount cancels the pending removal.
- A removed keyed row with no CSS transition still falls back to deterministic
  removal.
- A moved keyed row only receives move classes when its keyed position changes.

## Dependencies

- Internal: `internal/viewparse`, `internal/viewrender`,
  `internal/clientrt/assets/island.js`, `internal/buildgen` browser tests.
- External: browser `transitionend` / `animationend` events; user-authored CSS.

## Open Questions

- Whether a later slice should add `g:transition:in` / `g:transition:out`.
- Whether `g:animate` should eventually apply true FLIP transforms or remain a
  class/state contract.

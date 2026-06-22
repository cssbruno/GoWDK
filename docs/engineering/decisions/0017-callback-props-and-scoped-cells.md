# ADR 0017: Parent–Child Communication = Callback Props + Scoped Cells

Date: 2026-06-21

Status: Accepted

## Context

Parent↔child communication in components has **three overlapping mechanisms**:

- `emits { ... }` + `emit name(...)` in the child, observed by the parent with
  `g:on:<event>` — discrete events.
- `exports { ... }` in the child, observed by the parent with `g:on:exports` —
  continuous state observation.
- `g:bind:<ExportedState>={ParentField}` — two-way binding (already a
  desugaring: reactive prop down + the child's `exports` event up).

All three already lower to the **same wire**: a `data-gowdk-parent-on-<event>`
attribute whose expression the parent runs when the child dispatches a bubbling
`CustomEvent` (see `internal/viewrender/component.go` and
`internal/clientrt/assets/island.js`). So the divergence is in the *authoring
surface*, not the transport: three concepts for two underlying needs.

That is a learnability tax and more surface than ADR 0008's "explicit, minimal"
principle wants. The ecosystem precedent is clear: Svelte 5 collapsed events into
callback props; Vue 3 removed its event bus. GOWDK already rejected an event bus
(#514, closed).

This ADR is part of the interactivity-unification epic (#520) and decides the
parent–child axis of it.

## Decision

Split parent–child communication by the **principled axis: action vs state**, two
mechanisms instead of three.

### Actions (discrete) → callback props

A callback prop is an `on<Event>` prop whose value is a parent expression. The
child declares it as part of its prop/emit contract and "calls" it; the parent
passes behavior:

```gwdk
<TodoItem onDone={Count++} />
```

This replaces both `emit` + `g:on:<event>` and exports-used-as-events.

**Lowering (implementation honesty).** A callback prop compiles to the *existing*
`data-gowdk-parent-on-<event>` transport — the same bubbling-`CustomEvent` wire
`emit`/`exports`/`g:bind` use today. The child invoking a callback prop compiles
to the same dispatch `emit` compiles to now. We are unifying the **authoring
surface**, not inventing a new wire and not promising "just pass a function"
across the island boundary. The compiler validates the callback-prop name against
the child's declared callback contract and type-checks the parent expression in
the parent's client scope, exactly as `g:on:<event>` is validated today.

### Shared / observed state (continuous) → a writable scoped cell

Continuous state sharing and two-way binding use a **writable scoped cell** — a
reactive state with an explicit scope axis (`state @island|@page|@app`, ADR
direction #517). `bind:` is sugar over passing the cell as a value prop plus a
callback prop that writes it back:

```gwdk
<SearchBox bind:value={@page Query} />
// sugar over: value={@page Query} + onValueChange={@page Query = event.value}
```

This replaces store-sharing, `exports`-observation, and `g:bind`'s ad-hoc
two-way path with one model: **state lives in a scoped cell; sharing is reading
that cell; two-way is reading it plus a write-back callback.** It depends on #517
landing the scoped-cell primitive.

### Surface details this ADR reserves

Two grammar/encoding details the implementing change must honor, surfaced in
review:

- **Callback event names are lower-cased on the wire.** HTML attribute names are
  lower-cased by the DOM, while `CustomEvent` names are case-sensitive, and the
  runtime derives the event to listen for by slicing the
  `data-gowdk-parent-on-<event>` attribute. So a callback prop `onDone` maps to
  the parent-on attribute `data-gowdk-parent-on-done` and the child dispatches the
  lower-cased event `done` (the canonical encoding is: drop the `on` prefix,
  lower-case the remainder). Authoring is `onCamelCase`; the wire/event is the
  lower-cased name. The compiler validates the child declares the matching
  callback so the parent listener and child dispatch cannot drift.
- **`bind:` is a reserved directive prefix.** Non-`g:` attributes that contain a
  colon are currently parsed as `target:source` component prop renames, so
  `bind:value={…}` would otherwise be read as a prop named `bind` sourced from
  `value`. This ADR reserves `bind:*` as a binding directive that is matched
  *before* the prop-rename rule; `bind` is not usable as a rename target. (If a
  future need for a literal `bind` prop arises, it uses the rename's escape form,
  not the bare `bind:`.)

### Net model

**Events are callbacks, state is scoped cells.** Two principled mechanisms
replace three fuzzy ones, with no new transport and no event bus.

### Migration

`emit`/`emits`, `exports`, and `g:on:exports`, and `g:bind:` are deprecated. In
0.x, breaking is acceptable: each deprecated form gets a diagnostic that names
the new form (callback prop, or `bind:`/scoped cell), in the same "no silent
alias, precise migration nudge" style used for the v0.6.0 lane rename. Migration
notes ship with the implementing change.

This stays consistent with #384: the parent expression and the child callback use
the one bounded-client IR/evaluator; nothing here adds a second semantics.

## Consequences

### Positive

- Three concepts → two, along a principled action/state axis: less to learn,
  smaller compiler surface (ADR 0008).
- No wire change: callback props reuse the proven `data-gowdk-parent-on-*`
  transport, so the risk is in parsing/validation, not the runtime.
- `bind:` becomes a transparent two-line desugaring instead of a bespoke
  two-way path, which is easier to explain and to type-check.
- Aligns with current framework consensus (callback props; no event bus).

### Negative

- A breaking surface change: existing `emit`/`exports`/`g:bind` sources must
  migrate. Mitigated by precise migration diagnostics and 0.x status.
- The state half is blocked on #517 (scoped cells); until then only the
  callback-prop (action) half is implementable.

### Neutral

- The transport, island ABI, and `g:on:*` DOM-event surface are unchanged.
- This is the decided direction; implementation is phased (see Follow-Up) and
  the existing mechanisms keep working until their replacement lands with
  migration diagnostics.

## Alternatives Considered

- **Keep all three mechanisms.** Rejected: it is the status quo learnability tax
  this ADR removes; three surfaces for a two-axis need.
- **A component event bus.** Rejected and already closed (#514): implicit
  coupling over explicit ownership.
- **Callback props as real passed functions across the island boundary.**
  Rejected as over-promising: islands are independent runtimes; the honest model
  is that callback props *compile to* the bubbling-event transport, like `g:bind`
  desugars today.
- **A single two-way `bind:` without scoped cells.** Rejected: that is today's
  `g:bind` ad-hoc path; the scope axis (#517) is what makes shared state
  ownership explicit instead of implicit.

## Follow-Up

- Depends on **#384** (single-source client semantics) and **#517** (scoped
  cells) for the state half.
- **Phase 1 (action half, #517-independent):** callback-prop syntax + lowering
  to the existing `data-gowdk-parent-on-<event>` transport; validation and
  type-checking; deprecation diagnostics for `emit`/`g:on:<event>`.
- **Phase 2 (state half, gated on #517):** `bind:` desugars to value-prop +
  callback-prop over a scoped cell; deprecate `exports`/`g:on:exports`/`g:bind`.
- Each phase ships diagnostics, migration notes, an example, and tests, per the
  #518 checklist. Tracked under the interactivity-unification epic #520.

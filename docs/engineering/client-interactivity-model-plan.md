# Implementation Plan: Unify the Client Interactivity & Communication Model

## Context

Epic: `cssbruno/GoWDK#520`

The browser-side surface has grown to roughly ten loosely-related concepts. This
plan is the architectural decomposition for the epic: reduce that surface to a
small set of orthogonal primitives, close the determinism-integrity risk at its
root, and turn the bounded→WASM cliff into a ladder — without weakening the
"islands not hydration / bounded / Go-owned / deterministic" principles
(ADR 0008, ADR 0003).

It does not introduce new behavior on its own; it sequences and ties together the
workstream issues and their ADRs so each can land independently in the right
order.

## Today's surface (~10)

`state`, `computed`, `effect`, `props`, scoped slots, `emit`, `exports`,
`g:bind`, page stores, app-global stores, instance/context state, realtime.

The redundancy is concentrated in two places:

- **State sharing** is expressed three+ ways (page stores, app-global stores,
  instance/context state, plus `exports` observation).
- **Parent↔child communication** is expressed three ways (`emit` + `g:on`,
  `exports` + `g:on:exports`, `g:bind`) that already lower to one transport.

## Target (~5 orthogonal primitives)

1. **Reactive state with a scope axis** — `state @island | @page | @app | @instance`,
   one lifecycle and one mental model instead of separate store kinds. → #517
   (supersedes #508, #515).
2. **`computed` / `effect`** — unchanged.
3. **Communication = callback props (actions) + scoped cells (state)**; `bind:`
   is sugar. → #518 / ADR 0017.
4. **Scoped slots** — unchanged.
5. **No event bus** — #514 (closed); explicit ownership over implicit coupling.

Pure Go helpers (#519 / ADR 0016) are the escape hatch that keeps the bounded
language small while removing the cliff — not a sixth primitive, but the ladder
rung between bounded expressions and a full WASM island.

## Foundational gate: single-source semantics (#384)

Every client-language expansion in this epic is **gated on #384** — one IR and one
evaluator that generate *both* the validator and the runtime, so there is a single
source of truth for client semantics. The pattern already exists in the
`RuntimeExpressionSpec` that drives operator/builtin parity (and is cross-checked
by the expression conformance test). #384 generalizes that to the whole bounded
language. Widening the surface (iteration #501, switch/match #504, Go helpers
#519, scoped cells #517) before #384 would multiply the Go/JS divergence surface,
so #384 lands first.

## Workstreams and sequencing

1. **#384 — single-source client semantics.** Foundational; gates everything
   below. One IR + one evaluator; generate validator AND runtime.
2. **#517 — unify reactive state under `state @scope`.** Collapses page/app/
   instance stores into one scoped primitive (supersedes #508, #515). Unblocks
   the state half of #518.
3. **#518 — callback props + scoped cells** (ADR 0017). Action half is
   #517-independent and can land right after #384; state half (`bind:` over a
   scoped cell) follows #517.
4. **#519 — pure Go helpers from the bounded client** (ADR 0016). Follows #384;
   independent of #517/#518.

Gated on #384 but outside this epic's primitive set: **#501** (iteration) and
**#504** (switch/match) — don't widen the divergence surface before #384.

## Decisions captured

- ADR 0008 — bounded client language (the boundary this epic preserves).
- ADR 0016 — pure Go helpers compile to WASM from their Go source (#519).
- ADR 0017 — callback props (actions) + scoped cells (state); `bind:` as sugar;
  deprecate `emit`/`exports`/`g:bind` (#518).
- #514 (closed) — no event bus.

## Migration posture

Breaking changes are acceptable in 0.x and follow the v0.6.0 lane-rename
precedent: every removed/renamed form gets a precise migration diagnostic, never
a silent alias. Migration notes ship with each workstream's implementing change,
not ahead of it.

## Out of scope (unaffected by this epic)

`#502`, `#503`, `#505`, `#506`, `#507`, `#509`, `#510`, `#511`, `#512`.

## Considered & rejected

Event bus (#514, closed); full-page hydration; resumability (Qwik-style).

## Status

This document is the epic's architectural plan. The epic issue (#520) remains the
open tracker; it closes when its workstreams (#384, #517, #518, #519) land. This
plan and ADRs 0016/0017 capture the decided direction so the workstreams can
proceed in order.

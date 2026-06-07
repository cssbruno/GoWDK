# ADR 0009: Optional Inline Go Authoring

Date: 2026-06-07

Status: Accepted

## Context

GOWDK currently keeps real application behavior in normal Go packages while
`.gwdk` files own routes, rendering declarations, markup, CSS, endpoint
metadata, and bounded compiler-owned client behavior.

This boundary keeps the Go toolchain intact, but it also makes GOWDK feel less
integrated than component frameworks where the author can keep related page
logic and markup in one file. Users should be able to choose that colocated
authoring style without turning `.gwdk` into a second non-Go backend language.

## Decision

GOWDK will support optional inline Go authoring in `.gwdk` files as planned
language surface.

Inline Go must remain real Go:

- it is optional; separate `.go` files remain the default and supported path;
- it must be parsed and type-checked with the standard Go toolchain after
  extraction or lowering;
- extracted code must become normal importable package code in the declaring
  `.gwdk` package;
- generated adapter code still calls exported Go symbols and must not own user
  domain logic;
- inline Go must not create a custom Go dialect, relaxed Go syntax, or hidden
  runtime-only behavior;
- the emitted or extracted Go must be inspectable, formatted, and testable.

The first stable design should prefer a small explicit block form rather than
implicit script sections. Exact syntax remains planned and must be specified
before implementation.

## Consequences

### Positive

- Users can colocate page/component-specific Go code with `.gwdk` markup when
  that improves ergonomics.
- GOWDK keeps Go as the programming language instead of inventing a backend
  scripting language.
- Inline-authored logic can still participate in normal package imports,
  handler binding, build-time data calls, tests, and generated adapters.

### Negative

- The compiler must own extraction, source mapping, diagnostics, formatting,
  and conflict handling for inline Go.
- Inline Go can make `.gwdk` files larger, so docs and tooling need clear
  guidance on when separate Go files are better.
- Generated or extracted files must avoid confusing users about what source of
  truth to edit.

### Neutral

- This changes the authoring surface, not the runtime ownership model.
- The generated adapter boundary from ADR 0005 still applies.
- The compiler/runtime boundary from ADR 0006 is amended: behavior stays in
  normal Go code, but that Go code may be authored inline in `.gwdk` once the
  planned extraction pipeline exists.

## Alternatives Considered

- Keep all application logic in separate `.go` files forever. Rejected because
  it blocks an ergonomic colocated authoring option.
- Allow arbitrary non-Go script logic in `.gwdk`. Rejected because it creates a
  second application language and weakens Go toolchain compatibility.
- Generate user behavior from declarations. Rejected because generated adapters
  should remain glue and user behavior should stay user-owned.

## Follow-Up

- Write a feature spec for inline Go block syntax, extraction, source maps,
  package conflicts, formatting, tests, and generated output layout.
- Update language docs when syntax exists.
- Add diagnostics that explain whether a symbol came from a `.go` file,
  extracted inline Go, or generated adapter code.

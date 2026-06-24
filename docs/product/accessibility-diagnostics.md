# Feature Spec: Accessibility Diagnostics

## Problem

GOWDK can already warn about a small set of literal markup accessibility issues,
but authors still need earlier feedback for ARIA references, custom interactive
semantics, unnamed controls, and statically knowable focus mistakes. The
compiler should catch only low-noise problems it can prove from supported
`view {}` markup.

## Goals

- Expand compiler warnings for literal ARIA, labels, IDs, interactive handlers,
  accessible names, landmarks, and focusability.
- Keep every new rule deterministic and source-spanned.
- Keep warnings scoped to resolvable markup inside the current page, component,
  or layout view tree.

## Non-Goals

- Full WCAG certification.
- Browser accessibility-tree emulation.
- Cross-file component implementation analysis.
- Runtime or Playwright accessibility scans.

## Users And Permissions

- Primary users: GOWDK app authors and teams running `gowdk check` in CI.
- Roles or permissions: no new auth roles.
- Data visibility rules: diagnostics reference source files and markup spans
  only.

## User Flow

1. Run `gowdk check` or use the LSP on `.gwdk` sources.
2. See warning diagnostics with stable codes and exact markup spans.
3. Fix the literal markup or keep complementary browser audits for dynamic cases.

## Requirements

### Functional

- Warn on duplicate literal IDs in one resolvable view tree.
- Warn on literal ARIA and `<label for>` references that do not resolve in that
  tree.
- Warn on unsupported literal ARIA roles and attributes.
- Warn on a bounded set of ARIA role/attribute combinations that are clearly
  invalid.
- Warn when custom click interactions lack semantic role, focusability, or
  keyboard event handling.
- Warn when controls or explicit landmarks have no accessible name.
- Warn on positive `tabindex` and focusable elements hidden with
  `aria-hidden="true"`.

### Non-Functional

- Performance: walk each view tree linearly plus small literal-reference maps.
- Reliability: skip dynamic expressions instead of reporting speculative
  failures.
- Accessibility: diagnostics are a first pass and must not replace browser
  checks.
- Security/privacy: do not emit user secrets or dynamic values.
- Observability: every rule must have a stable diagnostic code.

## Acceptance Criteria

- [x] New warning codes are registered and listed in diagnostic docs.
- [x] Positive and negative fixtures cover pages/components/layouts where the
      rule can be resolved.
- [x] Dynamic references are ignored unless the compiler can prove a problem.
- [x] Documentation says compiler lint is not accessibility certification.

## Edge Cases

- `aria-labelledby` can reference multiple IDs; warn only on literal missing
  tokens.
- Component children are inspected, but imported component internals are not.
- Native controls and elements with `aria-hidden="true"` can still be focusable.

## Dependencies

- Internal: `internal/lang` diagnostics, `internal/viewmodel`,
  `internal/diagnostics`.
- External: none.

## Open Questions

- Which additional ARIA role/attribute pairs should graduate from browser audits
  into compiler warnings?

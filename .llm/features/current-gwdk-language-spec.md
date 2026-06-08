# Feature Spec: Current .gwdk Language Spec

## Problem

M2 needs a stable language contract before parser recovery, formatter
hardening, source spans, and IR migration can be reviewed cleanly. The current
language behavior is documented across several pages, but there is no compact
source of truth that names the supported syntax, planned syntax, and rejected
syntax policy in one place.

## Goals

- Document the current implemented `.gwdk` language surface in one spec page.
- Separate implemented, partial, planned, and intentionally unsupported syntax.
- Link detailed docs instead of duplicating every reference page.
- Make parser, formatter, diagnostics, and IR follow-up work easier to scope.

## Non-Goals

- Change compiler behavior.
- Declare planned syntax as implemented.
- Replace detailed language reference pages.
- Promise production readiness or a stable 1.0 language contract.

## Users And Permissions

- Primary users: GOWDK contributors, addon authors, and early users reading
  experimental language docs.
- Roles or permissions: none.
- Data visibility rules: no runtime data or user data impact.

## User Flow

1. A contributor opens the language docs before changing parser or diagnostics.
2. They read the current spec to identify whether a syntax surface is
   implemented, partial, planned, or rejected.
3. They update the spec and linked detailed docs when implementation reality
   changes.

## Requirements

### Functional

- List file kinds and package rules.
- List annotations, imports, use declarations, endpoints, blocks, Go blocks,
  scoped JavaScript, routes, view markup, components, slots, and directives.
- State raw HTML and unsupported syntax policy.
- State diagnostics and compatibility policy.
- Link to detailed docs for behavior-specific rules.

### Non-Functional

- Performance: documentation-only, no runtime impact.
- Reliability: wording must not overstate implemented behavior.
- Accessibility: docs use plain headings and compact lists.
- Security/privacy: raw HTML policy must stay explicit.
- Observability: not applicable.

## Acceptance Criteria

- [ ] `docs/language/spec.md` exists and is linked from `docs/language/README.md`.
- [ ] The spec covers the issue #80 checklist.
- [ ] Existing detailed language docs remain the source for deep behavior.
- [ ] Verification confirms docs links and repository tests still pass where relevant.

## Edge Cases

- If current docs disagree, the spec should follow implemented behavior from
  README, requirements, roadmap, and architecture, then leave detailed docs to
  be corrected in follow-up commits.

## Dependencies

- Internal: `docs/language/*`, `docs/product/requirements.md`,
  `docs/product/roadmap.md`, `docs/engineering/architecture.md`.
- External: GitHub issue #80 in milestone M2.

## Open Questions

- Which planned syntax surfaces should become hard diagnostics first instead of
  only being documented as unsupported?

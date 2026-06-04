# Feature Spec: Action Required Validation Slice

## Problem

Before this slice, `valid(input)?` was metadata only. Generated action handlers
could decode allowed form fields, but still redirected when a
statically-required field was omitted or empty.

## Goals

- Treat direct static `required` controls inside same-page `g:post` forms as
  first-slice validation rules.
- Run those required-field checks only when the action declares `valid(input)?`.
- Return HTTP 422 for required-field validation failures before redirecting.
- Keep validation failures free of submitted values.
- Preserve the current dependency-free generated app output.

## Non-Goals

- Resolve user-defined Go validation functions.
- Resolve real Go struct tags.
- Validate types such as `email`, `number`, dates, files, or ranges.
- Render field-level validation HTML.
- Implement CSRF.

## Users And Permissions

- Primary users: Go developers building static-first action forms.
- Roles or permissions: local compile and generated app build access.
- Data visibility rules: validation responses and logs must not expose submitted
  field values.

## User Flow

1. User declares `valid(input)?` in an action.
2. User marks a direct form control with `required`.
3. Generated binary returns HTTP 422 when that field is missing or empty.
4. Generated binary redirects normally when required fields are present.

## Requirements

### Functional

- Infer direct static `required` fields from `input`, `textarea`, and `select`
  controls inside same-page `g:post` forms.
- Missing or empty required fields fail validation.
- Repeated values pass when at least one submitted value is non-empty after
  trimming spaces.
- Actions without `valid(input)?` keep redirecting after successful decoding.
- Validation failures return HTTP 422.

### Non-Functional

- Performance: validation remains linear in required field count.
- Reliability: validation runs after decoding and before redirects.
- Accessibility: emitted static HTML remains unchanged.
- Security/privacy: responses and logs do not include submitted values.
- Observability: generated handlers return a stable status for validation
  failures.

## Acceptance Criteria

- [x] View tests cover required-field inference.
- [x] Generated app tests prove missing required fields return HTTP 422.
- [x] Generated app tests prove actions without `valid(input)?` do not enforce
  required-field validation.
- [x] CLI action binary smoke covers valid submissions still redirecting.
- [x] Docs/checklist describe current validation behavior and limitations.

## Edge Cases

- Duplicate field names across forms are one decoded field; required wins if any
  matching direct control is required.
- Whitespace-only values fail required validation.
- Missing expected fields are still allowed when no `required` direct control is
  inferred or no `valid(input)?` is declared.

## Dependencies

- Internal: `internal/view`, `internal/appgen`.
- External: Go standard library only.

## Open Questions

- How should future user-defined validators map errors back into static or
  partial form responses?

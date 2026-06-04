# Feature Spec: Typed Form Decoder Slice

## Problem

The first action redirect slice parses submitted forms but discards the parsed
values before redirecting. GOWDK needs the generated action path to start using
typed form decoder shape so action input names become real generated values and
unexpected submitted fields are not silently mass-assigned.

## Goals

- Add stable runtime helpers for generated form decoders.
- Infer first-slice expected fields from direct static HTML controls inside
  same-page `<form g:post={action}>` forms.
- Generate a named input type for each declared `input := form TypeName`.
- Generate per-action decoder functions that preserve repeated values.
- Reject submitted fields outside the inferred allowlist with HTTP 400.

## Non-Goals

- Resolve user-defined Go structs from application source.
- Generate field-specific struct members.
- Execute user action logic.
- Enforce required fields or validation messages.
- Implement CSRF.
- Infer fields hidden inside component calls.
- Support file upload inputs.

## Users And Permissions

- Primary users: Go developers building static-first action forms.
- Roles or permissions: local compile and generated app build access.
- Data visibility rules: decoder errors and generated logs must not include
  submitted field values.

## User Flow

1. User declares `act submit { input := form SignupInput ... }`.
2. User writes `<form g:post={submit}>` with direct static controls such as
   `<input name="email" />`.
3. `gowdk build --app --bin` emits a generated `SignupInput` type and decoder.
4. Generated binary accepts expected fields, preserves repeated values, and
   rejects unexpected fields with HTTP 400 before redirecting.

## Requirements

### Functional

- Runtime form helpers copy submitted values without aliasing request storage.
- Runtime form helpers preserve repeated values.
- Runtime form helpers reject duplicate decoder schema fields.
- Runtime form helpers reject unknown submitted fields by field name only.
- Generated app source includes named input types for declared form input types.
- Generated action handlers call per-action decoders before redirecting.
- First-slice field inference reads direct `input`, `textarea`, and `select`
  controls with static `name` attributes inside `g:post` forms.

### Non-Functional

- Performance: expected-field checks remain linear in submitted and expected
  field counts.
- Reliability: invalid generated decoder schemas fail before redirecting.
- Accessibility: emitted form HTML remains unchanged.
- Security/privacy: unexpected-field errors do not expose submitted values.
- Observability: invalid submitted form payloads return HTTP 400 without logging
  field values.

## Acceptance Criteria

- [x] Runtime form tests cover repeated values, unknown fields, and schema
  duplicate rejection.
- [x] View tests cover first-slice action form field inference.
- [x] Generated app tests prove decoder source is emitted and unexpected fields
  return HTTP 400.
- [x] CLI action binary smoke covers expected fields still redirecting.
- [x] Docs/checklist describe current decoder behavior and limitations.

## Edge Cases

- Multiple forms for one action union their static direct field names.
- Missing expected fields are allowed in this slice and represented as absent
  values until validation contracts exist.
- Duplicate field names in markup are one expected field; repeated submitted
  values are preserved.
- Dynamic `name` attributes in generated action forms are unsupported.

## Dependencies

- Internal: `internal/view`, `internal/appgen`, `runtime/form`.
- External: Go standard library only.

## Open Questions

- How should generated decoders resolve real user Go structs and field tags?
- Should missing expected fields become validation errors or decoder errors?

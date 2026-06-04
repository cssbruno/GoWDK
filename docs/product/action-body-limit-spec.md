# Feature Spec: Action Request Body Limit

## Problem

Generated action handlers parse request bodies before any application logic runs.
Without an explicit body limit, generated apps can spend memory and CPU parsing
oversized form submissions.

## Goals

- Cap generated action request bodies before `ParseForm`.
- Return HTTP 413 for oversized action submissions.
- Keep the generated app dependency-free.
- Preserve existing valid action form behavior.

## Non-Goals

- Add configurable limits.
- Implement upload handling.
- Add API or SSR body limits.
- Change local static serving.

## Users And Permissions

- Primary users: Go developers deploying generated static/action binaries.
- Roles or permissions: local compile and generated app build access.
- Data visibility rules: oversized body responses and logs must not include
  submitted values.

## User Flow

1. User builds an app with `gowdk build --app --bin`.
2. A client submits an oversized action form.
3. Generated binary returns HTTP 413 before parsing form values.

## Requirements

### Functional

- Generated action handlers wrap `request.Body` with `http.MaxBytesReader`.
- Oversized forms return HTTP 413.
- Malformed non-oversized forms still return HTTP 400.
- Existing valid form submissions still redirect.

### Non-Functional

- Performance: the body cap must run before `ParseForm`.
- Reliability: generated handlers keep deterministic status codes.
- Accessibility: no static HTML changes.
- Security/privacy: no submitted values are logged.
- Observability: the body limit is documented in generated-output docs.

## Acceptance Criteria

- [x] Generated source tests assert `http.MaxBytesReader` is emitted.
- [x] Generated binary tests prove oversized forms return HTTP 413.
- [x] Existing generated binary redirect and validation tests still pass.
- [x] Docs/checklist describe the body limit.

## Edge Cases

- The limit applies to generated action POST routes only.
- Future file uploads need separate upload-specific size rules.

## Dependencies

- Internal: `internal/appgen`.
- External: Go standard library only.

## Open Questions

- Should the body limit become configurable through `gowdk.Config`?

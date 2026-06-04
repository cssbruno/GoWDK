# Feature Spec: Action Form Directive

## Problem

The first action redirect slice can generate POST handlers, but authors still
need to hand-write `method="post"` and `action="/route"`. GOWDK should let a
portable page bind a form to a declared action by name without duplicating the
route.

## Goals

- Support `g:post={actionName}` on lowercase `<form>` elements in the static
  `view {}` subset.
- Validate that the action exists and is available for generated redirect
  handling.
- Lower the directive to `method="post"` and `action="<current-route>"` during
  static generation.
- Work for explicit static/action pages and literal dynamic static routes.

## Non-Goals

- Support `g:target`, `g:swap`, loading states, or client runtime behavior.
- Support `g:post` on non-form elements.
- Support JavaScript-enhanced submissions.
- Infer action names from routes or component names.

## Users And Permissions

- Primary users: Go developers authoring static-first forms.
- Roles or permissions: local compile and generated output write access.
- Data visibility rules: no submitted form values are logged by this directive.

## User Flow

1. User declares `act submit { ... }` on a page.
2. User writes `<form g:post={submit}>`.
3. `gowdk build` emits `<form method="post" action="/page-route">`.
4. `gowdk build --app --bin` generates a POST handler for the action redirect.

## Requirements

### Functional

- `g:post={name}` parses as a directive attribute.
- `g:post` only works on `<form>`.
- `g:post` fails if the form already declares `method` or `action`.
- Unknown actions fail before static output is written.
- Unsupported `g:` directives still fail clearly.

### Non-Functional

- Performance: directive lowering remains part of static rendering.
- Reliability: failures happen before partial static output is written.
- Accessibility: emitted HTML is a normal form.
- Security/privacy: no form values are logged.
- Observability: compiler errors name the missing/unsupported action.

## Acceptance Criteria

- [x] View renderer tests cover `g:post` lowering.
- [x] View renderer tests reject unknown actions and invalid directive placement.
- [x] Static build tests cover emitted form attributes.
- [x] Static build tests reject unknown actions before writing output.
- [x] A buildable example uses `g:post`.

## Edge Cases

- Multiple `g:post` directives on one form fail.
- `g:post` with no value fails.
- A form with both `g:post` and `method` or `action` fails.

## Dependencies

- Internal: `internal/view`, `internal/staticgen`, first action redirect slice.
- External: none.

## Open Questions

- How should `g:target` and `g:swap` compose with `g:post` once partial
  fragments exist?

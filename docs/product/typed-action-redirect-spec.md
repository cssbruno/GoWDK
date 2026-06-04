# Feature Spec: Typed Action Redirect Slice

## Problem

Static pages can declare `act` blocks today, but the compiler only records the
action name. GOWDK needs the first executable action path so static/action-first
apps can submit a form without enabling full-page SSR.

## Goals

- Parse the first supported action body subset:
  - `input := form TypeName`
  - `valid(input)?`
  - `-> "/safe-local-path"`
- Preserve parsed action input and redirect metadata in the compiler manifest.
- Generate embedded static app POST handlers for non-dynamic pages with
  supported redirect actions.
- Keep actions available on static pages without SSR.
- Reject unsupported action body lines with compiler diagnostics.

## Non-Goals

- Generate user-defined Go action logic.
- Generate real typed structs from application code.
- Implement CSRF.
- Implement full user validation beyond the first static required-field slice.
- Generate partial fragments, JSON responses, file uploads, or API handlers.
- Support dynamic action routes in the generated app.

## Users And Permissions

- Primary users: Go developers building static-first forms.
- Roles or permissions: local compile and Go build access.
- Data visibility rules: generated action handlers must not log submitted form
  values.

## User Flow

1. User writes a static or action page with `act subscribe { ... }`.
2. User runs `gowdk check` and sees action body diagnostics if syntax is
   unsupported.
3. User runs `gowdk build --out dist --app .gowdk/app --bin dist/site`.
4. Generated binary accepts `POST /newsletter` and redirects to the declared
   local target.

## Requirements

### Functional

- `act name {}` captures body text.
- `input := form TypeName` records input variable and type names.
- `valid(input)?` records that the action expects validation.
- `-> "/path"` records a redirect target.
- Redirect targets must be local absolute paths and must not start with `//`.
- Unknown action body lines produce diagnostics.
- Generated static app handles `POST <page route>` for redirect actions.
- Generated static app calls `ParseForm` before redirecting.
- Generated static app returns 405 for unsupported methods on static routes.

### Non-Functional

- Performance: action route checks remain simple generated switch cases.
- Reliability: unsupported action body syntax fails before generated app output.
- Accessibility: no client runtime or markup changes in this slice.
- Security/privacy: redirects are restricted to local absolute paths; form values
  are parsed but not logged.
- Observability: generated app logs startup and fatal server errors only.

## Acceptance Criteria

- [x] Parser tests cover supported action body parsing.
- [x] Parser tests reject unsupported action body lines.
- [x] Manifest output includes parsed action input and redirect metadata.
- [x] Generated app tests prove a POST action route redirects with 303.
- [x] Generated app tests reject unsafe redirect targets before writing app
  source.
- [x] Docs describe the supported action subset and current limitations.

## Edge Cases

- `valid(other)?` fails when the action input variable is `input`.
- `-> "https://example.com"` fails because external redirects are unsupported.
- Multiple redirects in one action fail.
- Dynamic action routes remain unsupported in generated app output.

## Dependencies

- Internal: `internal/parser`, `internal/manifest`, `internal/appgen`.
- External: Go standard library only.

## Open Questions

- How should GOWDK bind real user Go types into generated action decoders?
- What CSRF storage strategy should be used for static/action pages?

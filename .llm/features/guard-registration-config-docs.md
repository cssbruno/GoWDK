# Feature Spec: Guard Registration Configuration Docs

## Problem

Generated SSR, action, and API routes can declare `@guard` IDs and fail closed
when no matching guard is registered, but app owners need a documented
success-path configuration pattern that keeps authorization logic in normal Go.

## Goals

- Document how generated apps expose `RegisterGuards`.
- Show where app-owned guard functions are registered for generated binaries.
- Prove registered guards allow SSR, action, and API request-time routes to
  continue to their normal handlers.

## Non-Goals

- Define custom guard error response syntax.
- Add a new generated context type.
- Move auth, session, or business authorization rules into `.gwdk`.

## Users And Permissions

- Primary users: GOWDK app developers deploying generated app binaries.
- Roles or permissions: application owners provide guard functions.
- Data visibility rules: guard functions receive request-time context and must
  avoid exposing sensitive rejection details.

## User Flow

1. A page declares `@guard auth.required`.
2. The generated app exposes `RegisterGuards`.
3. App-owned Go code registers `auth.required` with an `ssr.GuardRegistry`.
4. Generated SSR, action, and API handlers run the guard before user logic.

## Requirements

### Functional

- Generated apps with guarded request-time routes must support same-package
  guard registration.
- Missing guards must continue to fail closed.
- Registered guards must allow successful request-time route execution.

### Non-Functional

- Performance: guard dispatch remains a direct map lookup.
- Reliability: registration examples must use compile-time Go imports.
- Accessibility: not applicable.
- Security/privacy: guard errors are server policy decisions; examples must not
  encourage client-owned auth decisions.
- Observability: docs explain the fail-closed behavior.

## Acceptance Criteria

- [x] Docs include a `RegisterGuards` example.
- [x] Docs clarify that SSR, action, and API handlers share the same registry.
- [x] Integration coverage proves registered guards allow generated SSR, action,
  and API routes.

## Edge Cases

- Missing registry or missing guard ID returns HTTP 403.
- A registered guard that returns an error returns HTTP 403.

## Dependencies

- Internal: `addons/ssr.GuardRegistry`, generated `RegisterGuards`.
- External: none.

## Open Questions

- Custom guard rejection pages and JSON error shape remain part of custom
  SSR/action/API error-boundary syntax.

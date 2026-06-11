# Feature Spec: Thin Native RBAC

## Problem

Generated guards already protect request-time routes, actions, APIs, fragments,
and contract web adapters, but every page/access check must be registered as an
application-specific guard. GOWDK needs a thin native RBAC path that keeps
identity external while making role and permission guard metadata portable as
defense-in-depth redundancy.

## Goals

- Reuse `guard` metadata for native role and permission checks.
- Require every real page source to declare access intent with `guard`; public
  pages use `guard public`.
- Keep users, sessions, OAuth, tenant storage, and persistence in application Go.
- Expose a small runtime auth contract for a current principal.
- Require generated guarded apps to provide backing hooks without importing
  feature packages back into generated output.
- Treat native RBAC guards as generated route/page access redundancy, never as
  the source of truth for backend/resource authorization.

## Non-Goals

- No user management, password handling, OAuth, sessions, JWT parsing, or tenant
  model in GOWDK.
- No policy language beyond role and permission membership.
- No route-local redirect or custom response guard result in this slice.
- No replacement for Go handler/service authorization.

## Users And Permissions

- Primary users: Go developers generating request-time GOWDK handlers.
- Roles or permissions: application-defined strings.
- Data visibility rules: generated guards can reject the route before user
  handler logic, but domain-level data filtering and protected-resource
  authorization remain application code.

## User Flow

1. A page declares `guard role:admin` or `guard permission:posts.write`.
2. App startup defines `GOWDKAuthProvider` or `GOWDKGuardRegistry`.
3. Generated handlers execute rate limiting, then native RBAC/custom guards, then
   request decoding and user logic.

## Requirements

### Functional

- Native guard IDs with `role:` require the principal to have that role.
- Native guard IDs with `permission:` require the principal to have that
  permission.
- Missing `guard` on a real page source fails validation.
- `guard public` marks intentional public access, requires no runtime backing
  hook, and cannot be combined with protected guard IDs.
- Non-public page guards require request-time page rendering so frontend page
  access can be checked before HTML is returned.
- Missing required backing hooks fail Go compilation.
- A missing principal or missing role/permission fails closed through the
  existing guard failure path.
- Existing custom `guard` registrations continue to work.

### Non-Functional

- Performance: role and permission checks are linear over short application-owned
  slices.
- Reliability: generated guarded apps fail Go compilation when required backing
  hooks are missing.
- Accessibility: no UI impact.
- Security/privacy: GOWDK stores no credentials or session data.
- Observability: existing guard error text identifies the failed guard ID.

## Acceptance Criteria

- [x] `runtime/auth` exposes principal, provider, and native RBAC guard helpers.
- [x] Real page sources fail validation without explicit `guard` metadata.
- [x] `guard public` marks intentional public access and requires no backing
  hook.
- [x] Protected page guards fail validation on build-time SPA/action page
  routes.
- [x] Generated guarded apps require `GOWDKAuthProvider` for native RBAC guards.
- [x] Generated guarded apps require `GOWDKGuardRegistry` for custom guards.
- [x] Generated guarded apps execute native RBAC guards without custom guard
  registry entries.
- [x] Existing custom guard tests continue to pass.
- [x] Docs show the thin RBAC contract and non-goals.

## Edge Cases

- Empty `role:` or `permission:` guard IDs fail.
- `public` mixed with any other guard ID fails validation.
- Protected guard IDs on build-time page routes fail validation.
- Native RBAC guard with no auth provider hook fails Go compilation.
- Auth provider error fails closed.
- A nil principal fails as unauthenticated.

## Dependencies

- Internal: existing `guard` parser and generated `RegisterGuards` hook.
- External: none.

## Open Questions

- Whether a future syntax should add explicit `require role` declarations once
  the AST/IR migration is complete.

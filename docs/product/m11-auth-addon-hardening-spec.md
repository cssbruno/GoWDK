# Feature Spec: M11 Auth Addon Hardening

## Problem

Go teams trying the experimental auth addon need explicit contracts for password
hashing, session secrets, generated guard wiring, and CSRF-protected actions.
The current addon has useful pieces, but the crypto stance, replacement points,
runtime secret setup, and example flow need to be clear before users copy the
pattern.

## Goals

- Record the auth addon cryptography and dependency stance in an ADR.
- Let applications provide a custom password hasher while keeping PBKDF2 as the
  default.
- Make session signing secrets fail closed with errors that name only the
  missing or unsafe setting.
- Document how session-backed guards interact with generated CSRF validation.
- Add a small runnable guard example with one public route and one protected
  route.

## Non-Goals

- Make the auth addon production-ready.
- Own user stores, OAuth, MFA, account recovery, durable session storage,
  tenants, or backend resource authorization.
- Add a production dependency for password hashing in the root module.

## Users And Permissions

- Primary users: Go developers evaluating GOWDK generated action, SSR, and
  guard flows.
- Roles or permissions: native RBAC guard IDs such as `role:user`.
- Data visibility rules: generated guards protect page and endpoint access;
  application handlers still authorize protected resources.

## User Flow

1. Configure `auth.Addon()` and request-time rendering for a guarded page.
2. Provide runtime session and CSRF secrets through environment variables.
3. Register generated app guard hooks from app-owned Go.
4. Log in through a generated action, receive a signed session cookie, access a
   guarded SSR page, and log out through a protected action.

## Requirements

### Functional

- `addons/auth` exposes `PasswordHasher` and keeps `PBKDF2Hasher` as the
  default implementation.
- `addons/auth.New` accepts either a direct secret or `SecretEnv` and rejects
  missing or short session secrets.
- The docs link to the auth cryptography ADR and explain replacement points.
- The example builds through a documented command and includes public/protected
  routes plus login/logout boundaries.

### Non-Functional

- Performance: password hashing uses the encoded iteration count and keeps
  defaults explicit.
- Reliability: missing session secrets fail before serving auth flows.
- Accessibility: the example uses semantic labels and status-safe form markup.
- Security/privacy: errors mention env var names only and never secret values.
- Observability: generated route reports show public and protected route
  metadata.

## Acceptance Criteria

- [x] ADR exists and explains the selected crypto/dependency stance.
- [x] Auth users can provide a custom password hasher.
- [x] Default PBKDF2 behavior remains covered by tests.
- [x] Missing or unsafe session secrets produce clear errors without leaking
  values.
- [x] Docs explain generated guard/session and CSRF ordering.
- [x] Example includes at least one protected route and one public route.

## Edge Cases

- A protected action with no valid session fails at the guard step before CSRF
  validation.
- A public login action has no guard and therefore validates CSRF before calling
  the login handler.
- A short session secret loaded from env reports the env var name, not the env
  value.

## Dependencies

- Internal: `addons/auth`, `runtime/auth`, `runtime/guard`, generated action and
  SSR route support.
- External: none.

## Open Questions

- Should future bcrypt or Argon2 helpers live in a nested optional module or
  stay entirely app-owned?

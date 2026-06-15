# ADR 0011: Auth Addon Cryptography Stance

Date: 2026-06-15

## Status

Accepted

## Context

`addons/auth` is an experimental 0.x convenience addon for common authentication
plumbing: password hashing, signed session cookies, and native RBAC guard
providers. The repository dependency policy says to prefer maintained libraries
for cryptography while keeping production dependencies minimal and optional
integrations isolated.

The root module already targets Go 1.26.4. That standard library includes
`crypto/pbkdf2`, so the addon can use a maintained PBKDF2 implementation without
adding `golang.org/x/crypto` to the root module graph.

The addon must also avoid implying that GOWDK owns a complete authentication
system. Applications still own user lookup, credential lifecycle, MFA, OAuth,
account recovery, durable session storage, tenant policy, and backend resource
authorization.

## Decision

Keep the auth addon default dependency-free in the root module and implement the
default password hasher with Go's standard-library `crypto/pbkdf2` using
HMAC-SHA256.

Expose a small `PasswordHasher` interface and `PBKDF2Hasher` implementation so
applications can replace the default with bcrypt, Argon2, a password-hashing
service, or another app-owned policy without changing generated GOWDK route
contracts.

Do not add `golang.org/x/crypto` to the root module for this default. If a
future addon needs Argon2 or bcrypt helpers, package that as an optional or
nested dependency boundary and document the tradeoff separately.

Session signing secrets must fail closed: the auth addon requires at least
32 bytes of secret material, can read that secret from a named environment
variable, and reports only the variable name or structural requirement in
errors.

## Consequences

### Positive

- The default password hashing path uses a maintained standard-library PBKDF2
  implementation and adds no production dependency.
- Root module dependency surface stays small.
- Applications with stronger or organization-specific password policy have an
  explicit replacement point.
- Session secret failures are clear without exposing secret values.

### Negative

- PBKDF2-HMAC-SHA256 is a conservative default, not a modern memory-hard
  password hashing recommendation.
- Applications that require bcrypt or Argon2 must provide or import their own
  hasher for now.

### Neutral

- The auth addon remains experimental 0.x behavior.
- GOWDK still does not own user stores, OAuth, MFA, durable sessions, or
  resource authorization.

## Alternatives Considered

- Add `golang.org/x/crypto` to the root module and switch the default to an
  x/crypto implementation. Rejected for the current default because Go 1.26.4
  already provides maintained PBKDF2 in the standard library and the root module
  should avoid unnecessary production dependencies.
- Keep the hand-rolled PBKDF2 loop. Rejected because the standard library now
  provides this primitive.
- Make Argon2 or bcrypt the built-in default. Deferred because those choices
  need a dependency and parameter policy that is better isolated as an optional
  package or app-owned hasher.

## Follow-Up

- Keep PBKDF2 default tests pinned with a known vector.
- Document the `PasswordHasher` replacement point in the addons reference.
- If GOWDK ships bcrypt or Argon2 helpers later, place them behind an optional
  dependency boundary and add a new ADR or update this one.

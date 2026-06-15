# Implementation Plan: M11 Auth Addon Hardening

## Context

Spec: `docs/product/m11-auth-addon-hardening-spec.md`

Issues: #117, #118, #119, #120, #121

ADR: `docs/engineering/decisions/0011-auth-addon-cryptography.md`

## Assumptions

- The root module remains dependency-light.
- Go 1.26.4 standard-library `crypto/pbkdf2` is available.
- Auth remains experimental 0.x behavior and must not be documented as a full
  production authentication system.

## Proposed Changes

- Replace the hand-rolled PBKDF2 loop with standard-library `crypto/pbkdf2`.
- Add `PasswordHasher` and `PBKDF2Hasher`.
- Add env-backed session secret loading with a 32-byte minimum.
- Add session cookie helper methods for generated action responses.
- Add focused auth addon docs, CSRF/session ordering notes, and ADR links.
- Add `examples/auth-guard` with generated app hooks, public login, protected
  dashboard, and logout action.

## Files Expected To Change

- `addons/auth/*`
- `docs/reference/addons.md`
- `docs/reference/hooks.md`
- `docs/engineering/decisions/*`
- `docs/product/m11-auth-addon-hardening-spec.md`
- `docs/engineering/auth-addon-hardening-implementation-plan.md`
- `examples/auth-guard/*`
- `examples/README.md`

## Data And API Impact

- New public API: `PasswordHasher`, `PBKDF2Hasher`, `Options.SecretEnv`,
  `DefaultSessionSecretEnv`, `MinSessionSecretBytes`, `Sessions.Cookie`, and
  `Sessions.ClearCookie`.
- Existing `HashPassword`, `HashPasswordWithIterations`, `VerifyPassword`,
  `Sessions.Issue`, and `Sessions.Clear` remain.
- Direct session secrets shorter than 32 bytes now fail.

## Tests

- Unit: auth addon password hasher, PBKDF2 vector, custom hasher, session secret
  env failures, session cookie helpers.
- Integration: example package tests and `make build`.
- End-to-end: generated `examples/auth-guard` binary build.
- Manual: run `examples/auth-guard/bin/auth-guard` with documented env vars.

## Verification Commands

```sh
go test ./addons/auth
cd examples/auth-guard && make check && make build
go test ./...
go build ./cmd/gowdk
scripts/test-go-modules.sh
```

## Rollback Plan

- Revert the auth addon API additions and docs together.
- Remove `examples/auth-guard` from `examples/README.md`.
- Keep the ADR as superseded if the cryptography stance changes after review.

## Risks

- Enforcing a 32-byte direct session secret can break experimental callers that
  used short local secrets.
- Generated app hooks are still copied into generated app output before build,
  which is documented but not yet first-class source-adjacent extraction.

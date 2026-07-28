# Feature Spec: CSRF Secret Rotation

## Problem

Generated apps currently sign and validate CSRF tokens with one runtime secret.
Changing that secret invalidates open forms and makes a rolling deployment
unsafe: old instances reject tokens minted by new instances, and new instances
reject tokens minted by old instances.

## Goals

- Let generated apps sign with one primary secret and validate with additional
  verification-only secrets.
- Keep the existing single-secret configuration fully compatible.
- Refresh a valid token signed by a verification key to the primary key when a
  generated page next injects a CSRF token.
- Fail generated-app startup when any configured secret is absent or shorter
  than 32 bytes.
- Document a rollback-safe staged rotation procedure.

## Non-Goals

- Owning deployment-platform secret storage or distribution.
- Replacing app-owned authentication, sessions, or resource authorization.
- Adding key identifiers or changing the existing token wire format.
- Adding token expiration or cross-instance server-side CSRF state.

## Users And Permissions

- Primary users: operators of generated GOWDK applications with actions,
  commands, or state-changing APIs.
- Roles or permissions: deployment operators control secret environment
  variables and generated app rollout order.
- Data visibility rules: config stores environment-variable names only; secret
  values remain runtime-only and must not appear in generated source, reports,
  diagnostics, or logs.

## User Flow

1. Deploy every instance with the current key as primary and the next key as a
   verification key.
2. Deploy every instance with the next key as primary and the current key as a
   verification key.
3. After the overlap window, deploy without the retired key.

## Requirements

### Functional

- `gowdk.CSRFConfig` exposes `VerificationSecretEnvs []string`.
- `actions.CSRFOptions` exposes `VerificationSecrets [][]byte`.
- `CSRF.Token` signs new tokens only with `Secret`.
- `CSRF.Validate` accepts signatures from the primary or any verification key.
- `CSRF.Token` replaces a request cookie signed only by a verification key with
  a newly generated primary-key token.
- Config validation rejects blank or duplicate verification environment names,
  including duplication of the primary environment name.
- Generated frontend, backend-only, and split app outputs read every configured
  CSRF secret and fail closed when a value is missing.

### Non-Functional

- Performance: verification is linear in the small operator-configured key
  list; all configured keys are checked without an early return.
- Reliability: primary-only behavior and the existing token format remain
  unchanged.
- Accessibility: no user-interface contract changes.
- Security/privacy: secret bytes are copied into runtime-owned memory and never
  emitted into generated source or reports.
- Observability: startup errors identify the missing environment-variable name
  without printing a value.

## Acceptance Criteria

- [x] A token minted with the old primary validates after that key moves to the
  verification list.
- [x] A token minted with the new primary validates on an instance that
  pre-staged the new key as verification-only.
- [x] Rendering a form with an old valid cookie emits a new primary-key cookie.
- [x] Removing a verification key makes its tokens fail validation.
- [x] Generated source reads the configured primary and verification
  environment variables and compiles.
- [x] A generated binary accepts an overlapping-key token end to end.
- [x] Existing primary-only runtime and generated-app tests remain green.

## Edge Cases

- Missing, blank, short, or duplicate configured keys.
- Principal-bound CSRF tokens during rotation.
- Repeated verification keys supplied directly through the runtime API.
- A request containing a cookie and submitted token that match each other but
  were signed by a retired key.

## Dependencies

- Internal: `gowdk.CSRFConfig`, structural/native config loading,
  `runtime/actions`, and generated app source under `internal/appgen`.
- External: none.

## Open Questions

- None for this slice. Key identifiers can be reconsidered only if measured
  verification cost justifies a token-format migration.

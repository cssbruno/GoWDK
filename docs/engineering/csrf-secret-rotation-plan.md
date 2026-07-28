# Implementation Plan: CSRF Secret Rotation

## Context

The feature contract is
[CSRF Secret Rotation](csrf-secret-rotation-spec.md). It closes the multi-key
rotation gap recorded in `SECURITY.md` and the deployment reference.

## Assumptions

- The existing HMAC token format remains secure and does not need a key ID.
- Secret environment names are compile-time config; values are runtime-only.
- A three-phase deployment is acceptable because it permits rolling deploys
  and rollback without invalidating tokens.

## Proposed Changes

- Add verification-secret environment names to `gowdk.CSRFConfig`, its
  structural parser, and config validation.
- Extend `runtime/actions.CSRF` with a primary signer and multiple verification
  keys.
- Generate AST-backed startup code that loads all configured keys and passes
  them to `actions.NewCSRF`.
- Seed configured verification secrets in generated audit tests.
- Update security, config, action, deployment, operations, generated-output,
  and product-status documentation.

## Files Expected To Change

- `gowdk.go`
- `internal/project/config.go` and config validation/tests
- `runtime/actions/csrf.go` and tests
- `internal/appgen/source.go`, audit test generation, and generator tests
- `SECURITY.md`
- `docs/compiler/generated-output.md`
- `docs/reference/config.md` and `docs/reference/deployment.md`
- `docs/language/actions.md`
- `docs/engineering/security*.md` and `operations.md`
- `docs/product/requirements.md`

## Data And API Impact

- Additive public Go config field:
  `CSRFConfig.VerificationSecretEnvs []string`.
- Additive public runtime option:
  `CSRFOptions.VerificationSecrets [][]byte`.
- No `.gwdk` syntax, manifest schema, token format, or persisted-data change.
- Existing binaries/configs with only `SecretEnv` behave as before.

## Tests

- Unit: config validation; primary/verification signing, validation, refresh,
  retirement, short-key rejection, and principal binding.
- Integration: generated source contains all environment reads and compiles.
- End-to-end: generated binary accepts a token minted by an overlapping key.
- Manual: follow the documented three-phase rollout against a generated app.

## Verification Commands

```sh
gofmt -w gowdk.go internal/project/config.go internal/project/config_validation.go internal/project/config_test.go internal/gowdkcmd/project_inputs.go runtime/actions/csrf.go runtime/actions/actions_test.go internal/appgen/source.go internal/appgen/audit_tests.go internal/appgen/appgen_test.go
go test ./runtime/actions ./internal/project ./internal/appgen
go build ./cmd/gowdk
scripts/check-docs-links.sh
scripts/check-docs-style.sh
scripts/test-go-modules.sh
```

## Rollback Plan

- Remove verification keys from config and redeploy with the current primary.
- Because the token format is unchanged, reverting the code keeps primary-key
  tokens valid.
- During phase two, rollback to phase one; phase-one instances already verify
  the promoted key.

## Risks

- Operators can remove an old key before the overlap window ends, invalidating
  open forms. The deployment guide makes the sequencing explicit.
- Large key lists add HMAC work per validation. The list is trusted config and
  expected to contain only the staged/current/retiring keys.
- Generator changes overlap active app-generation work; edits must remain
  confined to the CSRF AST builder and focused tests.

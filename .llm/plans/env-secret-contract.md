# Implementation Plan: Env And Secret Contract

## Context

Relevant spec, issue, ADR, or discussion:

- `.llm/features/env-secret-contract.md`
- User request to separate secrets from variables and fail clearly when required
  env names are missing.

## Assumptions

- `gowdk.config.go` should own the environment contract only.
- Secret values must stay in deployment/runtime environment sources.
- The first slice can define types, parsing, validation, docs, and diagnostics
  before implementing full `gowdk doctor` or `gowdk env`.

## Proposed Changes

- Add `Env gowdk.EnvConfig` to `gowdk.Config`.
- Add `EnvConfig{Vars []EnvVar, Secrets []SecretEnv}` with required/default
  metadata.
- Parse literal env config in `internal/project`.
- Validate duplicate names, empty names, defaults on secrets, and secret-looking
  names in normal vars.
- Add missing-env diagnostics where the selected validation command opts in.
- Update config docs with contract examples and deployment responsibility.

## Files Expected To Change

- `gowdk.go`
- `internal/project/config.go`
- `internal/project/config_test.go`
- `internal/compiler` or a new env validation package
- `internal/diagnostics` registry, if stable codes are added
- `docs/reference/config.md`
- `docs/reference/deployment.md`
- `.llm/features/env-secret-contract.md`
- `.llm/plans/env-secret-contract.md`

## Data And API Impact

- Adds public config fields.
- No secret values are added to generated artifacts or persisted metadata.
- Generated app startup checks may change behavior in a later slice if required
  runtime envs are missing.

## Tests

- Unit: config literal parsing, duplicate detection, missing-env diagnostics,
  secret default rejection, and secret-looking var rejection.
- Integration: CLI validation command with controlled environment.
- End-to-end: generated app startup check once runtime enforcement is added.
- Manual: inspect diagnostics and docs for redaction.

## Verification Commands

```sh
go test ./internal/project ./internal/compiler ./cmd/gowdk -count=1
go test ./...
scripts/test-go-modules.sh
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Revert config fields, parser support, diagnostics, and docs. Existing runtime
  env reads remain unchanged.

## Risks

- Running missing-env validation too early in normal builds could break local
  builds that only need generated output. Keep validation opt-in or profile
  aware until the deployment model is explicit.
- Secret-name heuristics may produce false positives. Start with conservative
  hard errors for obvious suffixes.

# Feature Spec: Env And Secret Contract

## Problem

Generated apps already depend on environment variables such as `GOWDK_ADDR`,
`GOWDK_CSRF_SECRET`, `GOWDK_BACKEND_ORIGIN`, `GOWDK_APP_ID`,
`GOWDK_MODULE_NAME`, and `GOWDK_INSTANCE_ID`. Those expectations are scattered
across runtime and generated code. Users need one config-owned contract that
states which runtime values exist, which are required, and which are secrets,
without ever storing secret values in `gowdk.config.go`.

## Goals

- Add a typed `gowdk.EnvConfig` to `gowdk.Config`.
- Forcefully separate normal environment variables from secrets.
- Emit clear diagnostics when required env names are missing from the local
  environment during checks that opt into env validation.
- Keep secret values out of config, manifests, build reports, generated output,
  and diagnostics.
- Provide a foundation for future `gowdk doctor` and `gowdk env` commands.

## Non-Goals

- Store secret values in config.
- Load `.env` files in compiler core.
- Replace deployment systems, CI secrets, systemd env files, Docker env, or
  Kubernetes secrets.
- Add a general secret manager.

## Users And Permissions

- Primary users: GOWDK app developers and operators.
- Roles or permissions: local config authoring and deployment setup.
- Data visibility rules: env names and non-secret defaults are visible; secret
  values are never printed, persisted, or embedded.

## User Flow

1. Declare normal runtime variables and secrets separately in `gowdk.config.go`.
2. Run a validation command that checks the env contract.
3. Missing required names fail with a direct diagnostic such as
   `DATABASE_URL is required but is not set`.
4. Generated docs and future doctor/env commands can list required variables
   and secrets without leaking values.

## Requirements

### Functional

- Config shape separates `Vars` and `Secrets`.
- `Vars` may have safe defaults.
- `Secrets` must not have defaults or inline values.
- The same name cannot appear in both `Vars` and `Secrets`.
- Secret-looking names in `Vars`, such as names ending in `_SECRET`,
  `_TOKEN`, `_PASSWORD`, or `_KEY`, should be rejected or warned before
  generated output.
- Required entries produce missing-env diagnostics when validation is enabled.
- Diagnostics and reports print names only, never values.

### Non-Functional

- Performance: env validation is O(number of declared env names).
- Reliability: generated app startup checks can reuse the same contract.
- Accessibility: diagnostics use direct, actionable language.
- Security/privacy: secret values are never represented in config or logs.
- Observability: future env reports can show present/missing/redacted status.

## Acceptance Criteria

- [ ] `gowdk.Config` has an env contract with separate vars and secrets.
- [ ] Duplicate names across vars/secrets fail validation.
- [ ] Secret-looking names in vars fail validation or emit stable diagnostics.
- [ ] Required missing vars/secrets produce clear missing-env diagnostics.
- [ ] Docs show that config owns the contract, while deployment owns values.

## Edge Cases

- Empty names are rejected.
- Defaults are rejected for secrets.
- Required variables with defaults are treated as satisfiable by default only
  when the variable is non-secret.
- Case sensitivity follows the host environment.

## Dependencies

- Internal: config parser, diagnostics registry, future doctor/env commands,
  generated app startup checks.
- External: host process environment only.

## Open Questions

- Should env validation run on every `check/build`, or only through
  `gowdk doctor` / `gowdk env` until deployment profiles exist?
- Which exact secret-name patterns should be hard errors in `Vars`?

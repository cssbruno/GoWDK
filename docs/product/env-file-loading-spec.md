# Feature Spec: Env File Loading

## Problem

Projects can declare required runtime secrets in `EnvConfig`, but CLI commands
and generated binaries only checked the process environment. Local development
required shell-side exports or third-party tools before `gowdk check`, `build`,
`dev`, `doctor`, or a generated binary could pass the same env contract.

## Goals

- Let project-aware CLI commands load a local env file before env validation.
- Preserve process environment precedence so CI and production behavior stays
  unchanged.
- Let generated binaries load an explicit env file for direct local runs.
- Keep diagnostics and doctor output value-free.

## Non-Goals

- Managing production secrets.
- Expanding dotenv syntax beyond simple `NAME=value` files.
- Making `gowdk serve` load project config; it remains static output serving.

## Users And Permissions

- Primary users: local developers and operators testing generated binaries.
- Roles or permissions: no new app permissions.
- Data visibility rules: commands may report env variable names and file paths,
  never values.

## User Flow

1. Developer writes `.env` or `.env.dev` in the project root.
2. Developer runs `gowdk check --env-file .env.dev` or sets `GOWDK_ENV=dev`.
3. GOWDK loads missing process values from the file and validates `EnvConfig`.
4. For generated binaries, the developer sets `GOWDK_ENV_FILE=/path/to/.env`
   or starts the binary from a directory containing `.env`.

## Requirements

### Functional

- `--env-file <path>` loads before `gowdk.config.go` env validation.
- Without `--env-file`, `.env.<GOWDK_ENV>` is discovered before `.env`.
- Process env values override file values.
- Generated binaries load `GOWDK_ENV_FILE` or discovered `.env` before startup
  env and CSRF secret checks.
- `doctor --json` reports the loaded env-file path and counts, not values.

### Non-Functional

- Performance: parse small env files once per command/build cycle.
- Reliability: malformed env files fail the command before validation proceeds.
- Accessibility: not applicable.
- Security/privacy: never print secret values; keep production env precedence.
- Observability: `doctor` exposes whether a file was loaded.

## Acceptance Criteria

- [x] `gowdk check --env-file <file>` satisfies a required secret from the file.
- [x] Process env wins when the same name appears in the file.
- [x] Auto-discovered `.env` satisfies the same validation path.
- [x] Generated binaries can load an explicit env file with `GOWDK_ENV_FILE`.
- [x] `doctor --json` reports the env file without leaking values.

## Edge Cases

- Explicit missing or malformed env files fail fast.
- Blank required values still fail after file loading.
- Empty optional values remain valid unless a secret `MinBytes` check applies.

## Dependencies

- Internal: `runtime/envfile`, CLI project config loading, generated app output.
- External: none.

## Open Questions

- Should a future production deploy recipe emit examples for platform-specific
  secret managers instead of env files?

# Implementation Plan: Env File Loading

## Context

Spec: `docs/product/env-file-loading-spec.md`
Issue: https://github.com/cssbruno/GoWDK/issues/467

## Assumptions

- Local env files are a developer convenience, not a production secret manager.
- Process env precedence is required for CI and deployment safety.
- `serve` stays outside scope because it does not load project config.

## Proposed Changes

- Add dependency-free `runtime/envfile` parsing and loading helpers.
- Load env files from shared CLI project config loading before
  `project.LoadConfig`.
- Add `--env-file` parsing to project-aware commands and build flags.
- Generate env-file loading into generated apps before env/CSRF validation.
- Add doctor metadata for loaded env files without values.
- Update CLI/config/deployment docs.

## Files Expected To Change

- `runtime/envfile/`
- `cmd/gowdk/project_inputs.go`, `build.go`, `doctor.go`, `dev.go`,
  `dev_loop.go`, `main.go`
- `internal/appgen/source_env.go`, `source.go`, `source_backend_app.go`
- CLI/generated app tests and docs

## Data And API Impact

- New public runtime package: `github.com/cssbruno/gowdk/runtime/envfile`.
- New CLI flag: `--env-file <path>` for project-aware commands and build flags.
- New generated-binary env var: `GOWDK_ENV_FILE`.

## Tests

- Unit: dotenv parser behavior and process-env precedence.
- Integration: `gowdk check --env-file`, auto `.env`, doctor JSON metadata.
- End-to-end: generated binary loads `GOWDK_ENV_FILE`.
- Manual: run `gowdk check --env-file .env.dev` in a project with `EnvConfig`.

## Verification Commands

```sh
gofmt -w runtime/envfile/*.go cmd/gowdk/*.go internal/appgen/*.go
go test ./runtime/envfile ./cmd/gowdk ./internal/appgen
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove env-file loading calls and the generated helper.
- Remove `--env-file` parsing and docs.
- Keep existing process-env validation behavior unchanged.

## Risks

- Accidentally overriding CI env values; mitigated by process-env precedence.
- Leaking values in diagnostics; mitigated by name/count-only reporting.
- Generated binaries loading unintended `.env` from the current working
  directory; explicit process env values still win and production should use
  platform env variables.

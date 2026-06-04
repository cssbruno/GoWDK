# Implementation Plan: Embedded Static App

## Context

Relevant spec: `docs/product/embedded-static-app-spec.md`.

Roadmap phase: embedded assets and one-binary static server.

## Assumptions

- The first generated app can be self-contained and dependency-free.
- The generated app embeds the already generated static output directory instead
  of re-running compiler logic.
- Future generated handlers can replace or extend this generated command layout.

## Proposed Changes

- Add an `internal/appgen` package that writes a generated Go module with:
  - `go.mod`.
  - `main.go`.
  - copied static output under `static/`.
- Add `--app <dir>` to `gowdk build`.
- Add `--bin <file>` to `gowdk build`; it requires `--app` and runs `go build`.
- Keep generated static serving behavior aligned with `gowdk serve`: GET/HEAD
  only, root index, extensionless nested index, no directory listing.
- Update CLI, generated output, operations, testing, README, requirements, and
  the missing implementation checklist.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `internal/appgen/`
- `docs/product/missing-implementation-checklist.md`
- `docs/product/requirements.md`
- `docs/engineering/architecture.md`
- `docs/engineering/operations.md`
- `docs/engineering/testing.md`
- `docs/reference/cli.md`
- `docs/compiler/generated-output.md`
- `README.md`
- `examples/README.md`

## Data And API Impact

- Adds CLI flags:
  - `gowdk build --app <dir>`
  - `gowdk build --bin <file>`
- Adds generated app layout:
  - `<app-dir>/go.mod`
  - `<app-dir>/main.go`
  - `<app-dir>/static/**`

## Tests

- Unit: generated app source and copied static files.
- Unit: app directory cannot be inside static output.
- Unit: `--bin` without `--app` fails.
- Integration: `gowdk build --app --bin` builds a binary and a live HTTP request
  returns generated static HTML.

## Verification Commands

```sh
go test ./cmd/gowdk ./internal/appgen
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk build --out /tmp/gowdk-build --app /tmp/gowdk-app --bin /tmp/gowdk-site examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
```

## Rollback Plan

- Remove the `internal/appgen` package, CLI flags, tests, and docs references.
- Keep `gowdk build --out` and `gowdk serve` unchanged.

## Risks

- Users may point `--app` at an existing directory; generation overwrites
  `go.mod`, `main.go`, and `<app-dir>/static`.
- The generated app serves only static files until actions, APIs, fragments, and
  SSR handlers exist.

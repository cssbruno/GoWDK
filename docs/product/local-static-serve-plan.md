# Implementation Plan: Local Static Serve

## Context

Relevant spec: `docs/product/local-static-serve-spec.md`.

Roadmap phase: embedded assets and one-binary static server.

## Assumptions

- This command is local development tooling and does not replace generated
  production binary work.
- The selected directory already contains output from `gowdk build`.
- The server should bind to `127.0.0.1:8080` by default.

## Proposed Changes

- Add `serve` to `cmd/gowdk`.
- Parse `--dir`, `--addr`, `--dir=<dir>`, and `--addr=<addr>`.
- Add an internal handler that serves GET/HEAD from the selected directory and
  falls back extensionless paths to `index.html`.
- Add CLI tests with `httptest`.
- Update docs, examples, and the missing implementation checklist.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `README.md`
- `docs/product/missing-implementation-checklist.md`
- `docs/reference/cli.md`
- `docs/engineering/operations.md`
- `docs/engineering/testing.md`
- `examples/README.md`

## Data And API Impact

- Adds CLI command: `gowdk serve --dir <dir> [--addr <addr>]`.
- No generated-output schema changes.
- No public Go API changes.

## Tests

- Unit: handler serves root index.
- Unit: handler serves extensionless nested route index.
- Unit: handler rejects unsupported methods.
- Unit: command rejects missing directories.
- Integration/manual: build examples and start local server.

## Verification Commands

```sh
go test ./cmd/gowdk
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
```

## Rollback Plan

- Remove the `serve` command, handler, tests, and docs references.
- Keep build output unchanged.

## Risks

- Users may confuse local `gowdk serve` with production generated binaries.
  Documentation should keep that boundary explicit.

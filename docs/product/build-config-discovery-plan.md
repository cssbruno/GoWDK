# Implementation Plan: Build Config Discovery

## Context

Relevant spec: `docs/product/build-config-discovery-spec.md`

Related backlog items:

- Add config-file-aware discovery for `gowdk build`.
- Add `internal/project` for config loading, source roots, and discovery.

## Assumptions

- `gowdk.config.go` is the project-root config filename for this slice.
- Static parsing is safer than compiling or executing user config.
- Only `gowdk build` consumes config for now.

## Proposed Changes

- Add `internal/project` with a static `gowdk.config.go` loader.
- Parse literal `Source.Include`, `Source.Exclude`, and `Build.Output` fields
  from `gowdk.Config{...}` assigned to `Config`.
- Add `--config <path>` and `--config=<path>` to `gowdk build`.
- Make `Build.Output` satisfy the output directory when `--out` is omitted.
- Keep explicit CLI `--out` and explicit file lists highest precedence.
- Add unit and CLI tests.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `internal/project/config.go`
- `internal/project/config_test.go`
- `README.md`
- `docs/reference/cli.md`
- `docs/reference/config.md`
- `docs/compiler/project-structure.md`
- `docs/product/missing-implementation-checklist.md`

## Data And API Impact

- Internal: new `internal/project` package.
- CLI: `gowdk build [--config <path>] [--ssr] [--out <dir>] [files...]`.
- Public root package API: unchanged.

## Tests

- Unit: parse literal source include/exclude and build output.
- Unit: invalid config syntax fails.
- CLI: build discovers configured source files and uses configured output.
- CLI: `--out` overrides config output.
- CLI: `--config` loads a custom path.

## Verification Commands

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
rm -rf /tmp/gowdk-build && go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
```

## Rollback Plan

- Remove `internal/project` and `--config` parsing.
- Return `gowdk build` to requiring `--out` unless explicit CLI output exists.
- Keep default discovery behavior unchanged.

## Risks

- Static config parsing may reject or ignore valid Go expressions users expect to
  work. Documentation must call this an initial literal subset.
- Future executable config loading may need a different package boundary.

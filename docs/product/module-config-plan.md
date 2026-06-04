# Implementation Plan: Module Config

## Context

Relevant spec: `docs/product/module-config-spec.md`

## Assumptions

- A module is a named source group in this slice, not a separate deployment
  artifact.
- The project can still have root `Source` for simple apps.
- Static parsing should continue to support only literal config values.

## Proposed Changes

- Add `ModuleConfig` to the root package config API with a user-defined
  `Type string`.
- Add `Modules []ModuleConfig` to `gowdk.Config`.
- Parse literal `Modules` entries from `gowdk.config.go`.
- Use module source includes and excludes during `gowdk build` discovery.
- Default a name-only module to `<name>/**/*.gwdk`.
- Add `gowdk build --module <name>` to select configured modules on demand.
- Update config, CLI, compiler, architecture, and README docs.

## Files Expected To Change

- `config.go`
- `internal/project/config.go`
- `internal/project/config_test.go`
- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `README.md`
- `docs/reference/config.md`
- `docs/reference/cli.md`
- `docs/compiler/project-structure.md`
- `docs/compiler/pipeline.md`
- `docs/engineering/architecture.md`
- `docs/product/missing-implementation-checklist.md`

## Data And API Impact

- Public root package API gains module config types.
- CLI discovery can be limited with `--module` when no explicit build file list
  is passed.
- Static build output remains one output directory.
- Deployment remains user-owned code; this slice does not generate Kubernetes
  manifests or deployment configuration.

## Tests

- Unit: parse literal module names, types, includes, and excludes.
- Integration: `gowdk build` discovers name-default and explicit module sources.
- Integration: `gowdk build --module <name>` discovers only selected modules.
- End-to-end: covered by `go test ./...` and `go build ./cmd/gowdk`.
- Manual: optional sample config can be run through `gowdk build`.

## Verification Commands

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
```

## Rollback Plan

- Remove `Modules` from `gowdk.Config`.
- Remove module parsing from `internal/project`.
- Return `gowdk build` discovery to root `Source` only.

## Risks

- Users may expect module type to alter generated backend/frontend behavior
  immediately. Documentation must call it metadata-only in this slice.
- Name-based default source roots may surprise users with unconventional folder
  names; explicit `Source.Include` remains the escape hatch.

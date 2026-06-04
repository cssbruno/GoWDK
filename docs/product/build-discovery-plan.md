# Implementation Plan: Build Discovery And Static Route Manifest

## Context

Relevant spec: `docs/product/build-discovery-spec.md`

Relevant roadmap items:

- Phase 1 portable file manifest.
- Phase 3 static/prerender output.
- Recommended next vertical slice in
  `docs/product/missing-implementation-checklist.md`.

## Assumptions

- Until project config loading exists, default discovery starts from the current
  working directory.
- Explicit CLI paths remain the most precise way to build a subset of files.
- Component identity means `@component` name.

## Proposed Changes

- Add default build source discovery to `cmd/gowdk`.
- Keep `--out` required, but make file arguments optional for `build`.
- Add duplicate page ID and component name validation to `internal/compiler`.
- Add static route manifest generation to `internal/staticgen`.
- Update docs and tests for the new behavior.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `cmd/gowdk/main_test.go`
- `internal/compiler/validate.go`
- `internal/compiler/validate_test.go`
- `internal/staticgen/staticgen.go`
- `internal/staticgen/staticgen_test.go`
- `README.md`
- `docs/reference/cli.md`
- `docs/reference/config.md`
- `docs/compiler/pipeline.md`
- `docs/compiler/manifest.md`
- `docs/product/requirements.md`
- `docs/product/missing-implementation-checklist.md`
- `examples/README.md`

## Data And API Impact

- CLI: `gowdk build --out <dir>` can now infer source files.
- Static output: `gowdk-routes.json` is written at the output root.
- Internal result: static build result records the route manifest path.
- No public Go package API changes.

## Tests

- Unit: compiler duplicate identity validation.
- Unit: static route manifest output.
- Integration: CLI build discovery from a temporary project.
- End-to-end: existing example build smoke command.
- Manual: inspect generated `gowdk-routes.json`.

## Verification Commands

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
rm -rf /tmp/gowdk-build && go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
test -f /tmp/gowdk-build/gowdk-routes.json
```

## Rollback Plan

- Revert the CLI discovery helper and return `build` to requiring explicit file
  paths.
- Remove duplicate component validation from `internal/compiler` if component
  semantics need a different namespace later.
- Remove `gowdk-routes.json` generation without changing generated HTML output.

## Risks

- Default discovery from a broad repository root may include examples or future
  non-build fixtures. Users can pass explicit paths until config loading exists.
- Route manifest schema may need to evolve once dynamic static routes and assets
  are generated.

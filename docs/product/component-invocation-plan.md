# Implementation Plan: Minimal Component Invocation

## Context

Relevant spec: `docs/product/component-invocation-spec.md`

Build slice already exists through `gowdk build --out`.

## Assumptions

- Component files can be explicit build inputs; default build discovery now also
  finds `.cmp.gwdk` files when no explicit paths are passed.
- `@component` identifies component files; `.cmp.gwdk` is the documented convention.
- The first prop type is `string` only.

## Proposed Changes

- Add `manifest.Component` and `manifest.Prop`.
- Add `parser.ParseComponent`.
- Add build-file parsing in `internal/lang` that accepts pages and components.
- Extend `internal/view` with component call nodes and prop interpolation.
- Extend `internal/staticgen` to render pages with component registry data.
- Add `examples/basic/hero.cmp.gwdk` and update `home.page.gwdk` to invoke it.
- Update docs and checklist.

## Files Expected To Change

- `internal/manifest/manifest.go`
- `internal/parser/*`
- `internal/lang/tools.go`
- `internal/view/*`
- `internal/staticgen/*`
- `cmd/gowdk/*`
- `examples/basic/*`
- `README.md`
- `docs/product/missing-implementation-checklist.md`
- `docs/language/*`
- `docs/compiler/*`
- `docs/reference/cli.md`
- `examples/README.md`

## Data And API Impact

- Adds internal manifest component data.
- `gowdk build` can accept page files and component files together.
- Public manifest JSON remains page-only in this slice.
- No public Go package API changes.

## Tests

- Unit: parse component name, props, and view body.
- Unit: render component invocation with escaped prop interpolation.
- Unit: fail on missing component and missing required prop.
- Integration: CLI build page plus component into `index.html`.
- End-to-end: not added in this slice.
- Manual: run `go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk`.

## Verification Commands

```sh
gofmt -w cmd/gowdk/*.go internal/manifest/*.go internal/parser/*.go internal/lang/*.go internal/view/*.go internal/staticgen/*.go
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
```

## Rollback Plan

- Remove component manifest/parser/build support and restore `home.page.gwdk` to lowercase-only markup.

## Risks

- This is not a full component compiler; keeping the syntax constrained avoids locking in unreviewed grammar.
- Component files can be explicit build inputs, discovered by default, or
  discovered through build config source settings.

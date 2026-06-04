# Implementation Plan: GWDK Go Build Imports

## Context

Relevant spec: `.llm/features/gwdk-go-build-import.md`.

## Assumptions

- The first implementation only needs build-time data imports.
- Imported functions are no-argument functions that return a JSON-encodable
  object.
- Go module resolution is delegated to `go run`.

## Proposed Changes

- Add parsed import metadata to page manifests.
- Parse top-level import lines in `.gwdk` page files.
- Extend static build data parsing to execute `=> alias.Func()`.
- Update the Go interop example to use the import as the primary path.
- Update language and examples documentation.

## Files Expected To Change

- `internal/manifest/manifest.go`
- `internal/manifest/json.go`
- `internal/parser/page.go`
- `internal/parser/syntax.go`
- `internal/staticgen/staticgen.go`
- `examples/go-interop/*`
- `examples/README.md`
- `docs/language/*`

## Data And API Impact

- Public manifest JSON gains optional page import metadata.
- `.gwdk` syntax gains top-level `import` declarations.

## Tests

- Unit: parser and staticgen tests for imports and Go build data calls.
- Integration: build the `examples/go-interop` page.
- End-to-end: `go test ./...`.
- Manual: inspect generated HTML for imported data.

## Verification Commands

```sh
go run ./cmd/gowdk build --out /tmp/gowdk-go-interop examples/go-interop/imported-build.page.gwdk
test -f /tmp/gowdk-go-interop/go-imported/index.html
go test ./examples/go-interop
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk examples/go-interop/*.gwdk
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Revert parser/manifest/staticgen import handling.
- Restore the Go interop example to literal `build {}` data.

## Risks

- Build-time Go execution can run arbitrary local code.
- Repeated Go invocations may be slow until broader compiler caching exists.

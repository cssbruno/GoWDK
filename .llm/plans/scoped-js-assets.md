# Implementation Plan: Scoped JavaScript Assets

## Context

Spec: `.llm/features/scoped-js-assets.md`

## Assumptions

- `js` is a source declaration, not an `@annotation`.
- The first slice copies declared files and emits module script tags.
- Bundling and dependency graph handling are separate future work.

## Proposed Changes

- Add JS references to AST, manifest, IR, and public manifest JSON.
- Parse top-level `js "./file.js"` in pages and components.
- Add an `AssetJS` IR asset kind.
- Copy scoped JS files into generated output.
- Add page/component script tags only to pages that declare or use them.
- Document scoped JS in language/generated-output docs.

## Files Expected To Change

- `internal/parser/`
- `internal/gwdkast/`
- `internal/manifest/`
- `internal/gwdkir/`
- `internal/gwdkanalysis/`
- `internal/buildgen/`
- `internal/view/`
- `docs/`

## Data And API Impact

- Manifest/IR records gain `JS` metadata.
- Generated output gains scoped JS asset files and module script tags.

## Tests

- Unit: parser/lowering tests for page and component `js`.
- Integration: buildgen test for page/component scoped JS emission.

## Verification Commands

```sh
gofmt -w <changed-go-files>
go test ./internal/parser ./internal/gwdkanalysis ./internal/buildgen ./internal/manifest
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove the `js` parser branch, `AssetJS`, scoped asset planner, and docs.

## Risks

- This does not yet bundle or copy JS import dependencies; imported files must
  be available by another mechanism until a bundler/import graph slice exists.

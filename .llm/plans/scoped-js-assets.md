# Implementation Plan: Scoped JavaScript Assets

## Context

Spec: `.llm/features/scoped-js-assets.md`

## Assumptions

- `js` is a source declaration, not an `@annotation`.
- Path-based declarations are preferred over inline `js {}` blocks.
- TypeScript support means transform-only output, not type checking.
- Bundling and dependency graph handling are separate future work.

## Proposed Changes

- Add JS references to AST, manifest, IR, and public manifest JSON.
- Parse top-level `js "./file.js"`, `js "./file.ts"`, and inline `js {}` in
  pages and components.
- Add an `AssetJS` IR asset kind.
- Copy scoped JS files into generated output.
- Transform scoped TS files into generated JS output through esbuild's Go API.
- Emit inline `js {}` blocks as deterministic generated JS module files.
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

- Manifest/IR records gain `JS` and inline JS metadata.
- Generated output gains scoped JS asset files and module script tags.
- `github.com/evanw/esbuild` becomes a production compiler dependency so GOWDK
  can transform TypeScript without requiring Vite or npm.

## Tests

- Unit: parser/lowering tests for page and component `js`.
- Integration: buildgen test for page/component scoped JS, TS transform, and
  inline JS emission.

## Verification Commands

```sh
gofmt -w <changed-go-files>
go test ./internal/parser ./internal/gwdkanalysis ./internal/buildgen ./internal/manifest
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove the `js` parser branch, `AssetJS`, scoped asset planner, esbuild
  dependency, and docs.

## Risks

- This does not yet bundle or copy JS import dependencies; imported files must
  be available by another mechanism until a bundler/import graph slice exists.
- esbuild transform does not type-check TypeScript; teams should still run
  `tsc --noEmit` or equivalent if they need type-check gates.

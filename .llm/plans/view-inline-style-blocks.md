# Implementation Plan: Style Blocks

## Context

Relevant spec: `.llm/features/view-inline-style-blocks.md`

## Assumptions

- `style {}` is only supported as a sibling block outside `view {}`.
- Generated CSS assets are preferred over inline `<style>` tags.

## Proposed Changes

- Extend block metadata to carry style block CSS.
- Update parser entrypoints to capture top-level `style {}` blocks.
- Lower style block CSS through manifest and IR compatibility paths.
- Emit page, component, and layout style block CSS through buildgen.
- Document the syntax and add focused tests.

## Files Expected To Change

- `internal/manifest/manifest.go`
- `internal/gwdkast/ast.go`
- `internal/gwdkir/ir.go`
- `internal/parser/page.go`
- `internal/parser/syntax.go`
- `internal/gwdkanalysis/analyzer.go`
- `internal/buildgen/ir.go`
- `internal/buildgen/components.go`
- `internal/buildgen/css.go`
- Parser/buildgen/docs tests and references.

## Data And API Impact

- Internal manifest and IR block structs gain `Style` and `StyleBody`.
- No public Go API changes.
- Generated CSS asset manifests include inline style assets.

## Tests

- Unit: parser extraction, syntax AST extraction, buildgen page/component/layout
  CSS emission.
- Integration: targeted `go test` packages.
- End-to-end: `go test ./...`.
- Manual: inspect generated HTML/CSS for a page with sibling `style {}`.

## Verification Commands

```sh
go test ./internal/parser ./internal/gwdkanalysis ./internal/buildgen
go test ./...
go build ./cmd/gowdk
```

## Rollback Plan

- Remove style block capture and the new block fields; existing external CSS
  behavior remains unchanged.

## Risks

- Line-based block parsing could mishandle unusual CSS formatting. Tests cover
  common multi-line rules and nested at-rules.

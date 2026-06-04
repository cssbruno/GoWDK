# Implementation Plan: Literal Build Data

## Context

Relevant spec: `docs/product/build-data-spec.md`.

Roadmap phase: static/prerender output.

## Assumptions

- The first `build {}` subset uses the same literal return shape as the current
  `paths {}` subset: `=> { key: "value" }`.
- Build data is page-level and shared by every generated route for that page.
- Route params and build data share one interpolation namespace and name
  collisions should fail.

## Proposed Changes

- Add `BuildBody` to `manifest.Blocks`.
- Capture `build {}` body text in the page parser.
- Add staticgen parsing for zero or one literal build data declaration.
- Merge build data with each page output's route params before view rendering.
- Add tests for parser capture, build data rendering, component prop rendering,
  malformed build data, and route-param name collisions.
- Update docs, examples, and checklist.

## Files Expected To Change

- `internal/manifest/manifest.go`
- `internal/parser/page.go`
- `internal/parser/page_test.go`
- `internal/staticgen/staticgen.go`
- `internal/staticgen/staticgen_test.go`
- `examples/basic/home.page.gwdk`
- `README.md`
- `docs/product/requirements.md`
- `docs/product/missing-implementation-checklist.md`
- `docs/language/blocks.md`
- `docs/language/grammar.md`
- `docs/language/semantics.md`
- `docs/language/markup.md`
- `docs/compiler/generated-output.md`
- `docs/engineering/architecture.md`
- `examples/README.md`

## Data And API Impact

- Internal manifest data gains `Blocks.BuildBody`.
- Public manifest JSON remains unchanged.
- Generated HTML can contain escaped literal build data referenced by `view {}`.

## Tests

- Unit: parser captures `build {}` body and rejects unclosed body capture.
- Unit: staticgen renders literal build data into page text/attributes.
- Unit: staticgen passes literal build data to component props.
- Unit: staticgen rejects malformed build data before writing output.
- Integration: CLI build smoke checks generated HTML contains build data.
- End-to-end: none in this slice.

## Verification Commands

```sh
go test ./internal/parser
go test ./internal/staticgen
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
grep -q "Portable Go web compiler" /tmp/gowdk-build/index.html
```

## Rollback Plan

- Remove `BuildBody` capture.
- Remove staticgen build data parsing and merge behavior.
- Restore examples/docs to route-param-only interpolation.

## Risks

- The literal syntax may differ from the eventual full build-time language.
  Keeping this as a narrow subset makes migration straightforward.

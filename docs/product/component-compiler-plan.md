# Implementation Plan: Minimal Component Compiler And Static Build Slice

Status: Completed for the first static build slice. The current home example now
uses the follow-up component invocation slice, so build commands include
`examples/basic/hero.cmp.gwdk`.

## Context

Relevant spec: `docs/product/component-compiler-spec.md`

Roadmap phases: component compiler, then static/prerender output.

## Assumptions

- The first build slice can support a deliberately small static markup subset.
- Page routes remain declared in source and derive output paths.
- `view {}` body capture can be line-oriented until the canonical grammar replaces it.

## Proposed Changes

- Extend parsed page metadata with the raw `view {}` body.
- Add a minimal markup AST/parser/renderer for lowercase HTML tags, quoted attributes, boolean attributes, self-closing tags, and text nodes.
- Add a static build package that validates build-time pages, renders HTML documents, and writes route-derived files.
- Add `gowdk build --out <dir> <files>` to the CLI.
- Update docs and examples to show the first buildable page.

## Files Expected To Change

- `cmd/gowdk/main.go`
- `internal/parser/page.go`
- `internal/manifest/manifest.go`
- `internal/view/*`
- `internal/staticgen/*`
- `examples/basic/home.page.gwdk`
- `README.md`
- `docs/product/requirements.md`
- `docs/product/missing-implementation-checklist.md`
- `docs/engineering/architecture.md`
- `docs/engineering/testing.md`
- `docs/compiler/*`
- `docs/reference/cli.md`
- `examples/README.md`

## Data And API Impact

- Adds CLI command: `gowdk build --out <dir> <files>`.
- Adds internal manifest data for `view {}` body content.
- No public Go package API changes.
- No persisted data changes beyond generated HTML files in the requested output directory.

## Tests

- Unit: parser captures `view {}` body.
- Unit: markup parser escapes text and attributes and rejects unsupported tags.
- Unit: static generator maps routes and writes HTML.
- Integration: CLI build command writes `index.html` for a fixture page.
- End-to-end: not added in this slice.
- Manual: run `go run ./cmd/gowdk build --out <tmpdir> examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk`.

## Verification Commands

```sh
gofmt -w cmd/gowdk/main.go internal/parser/page.go internal/manifest/manifest.go internal/view/*.go internal/staticgen/*.go
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
```

## Rollback Plan

- Remove `gowdk build`, `internal/view`, `internal/staticgen`, and the manifest `ViewBody` field.
- Restore `examples/basic/home.page.gwdk` to validation-only markup.

## Risks

- The line-oriented `view {}` capture will not handle interpolation or braces in text.
- The minimal HTML parser is temporary and must not expand into a full grammar by accident.
- Component invocation is handled by the follow-up spec in `docs/product/component-invocation-spec.md`.

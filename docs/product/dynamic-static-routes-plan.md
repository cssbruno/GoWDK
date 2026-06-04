# Implementation Plan: Dynamic Static Routes

## Context

Relevant spec: `docs/product/dynamic-static-routes-spec.md`

Relevant decision: ADR 0002 keeps `paths {}` as build-time route declaration
and requires it for dynamic static/action routes.

## Assumptions

- Captured source body text is the input for the first executable subset.
- Public manifest JSON should stay compact and avoid exposing block bodies.
- The first supported syntax is one literal parameter set per line:
  `=> { slug: "hello-gowdk" }`.
- Route params are used for path expansion and the current static `view {}`
  interpolation context. They are not available to `build {}` expressions yet.

## Proposed Changes

- Parse literal path declarations in `internal/staticgen`.
- Expand dynamic route templates such as `/blog/{slug}` into concrete route
  strings.
- Pass each path declaration's route params into static view rendering.
- Generalize `internal/view` interpolation so route params can render in page
  text, static HTML attributes, and component prop values.
- Reuse existing route-to-output-path rules for generated dynamic routes.
- Reject missing, unused, duplicate, empty, malformed, and duplicate-output path
  declarations before writing output.
- Update the dynamic static route example so it can be built.
- Update docs and the missing-implementation checklist.

## Files Expected To Change

- `internal/staticgen/staticgen.go`
- `internal/view/view.go`
- `internal/view/view_test.go`
- `internal/staticgen/staticgen_test.go`
- `examples/basic/blog-post.page.gwdk`
- `README.md`
- `docs/language/blocks.md`
- `docs/language/grammar.md`
- `docs/language/semantics.md`
- `docs/compiler/generated-output.md`
- `docs/compiler/manifest.md`
- `docs/product/requirements.md`
- `docs/product/missing-implementation-checklist.md`
- `examples/README.md`

## Data And API Impact

- Internal Go API: no new public type is required for this slice.
- Public manifest JSON: unchanged.
- CLI behavior: `build` emits concrete files for dynamic static pages that use
  the supported literal `paths {}` subset and renders route params into page
  HTML.

## Tests

- Unit: staticgen expands literal dynamic paths into concrete HTML files.
- Unit: view renders data interpolation and escapes values.
- Unit: staticgen binds dynamic route params into page HTML and component props.
- Unit: staticgen route manifest includes concrete dynamic routes.
- Unit: staticgen rejects missing, unused, malformed, and duplicate path output.
- Integration: example `check` and `manifest` smoke commands.
- Manual: build the dynamic route example and inspect generated files.

## Verification Commands

```sh
go test ./internal/staticgen
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
go run ./cmd/gowdk sitemap --ssr examples/basic/*.gwdk
rm -rf /tmp/gowdk-build && go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
rm -rf /tmp/gowdk-dynamic-build && go run ./cmd/gowdk build --out /tmp/gowdk-dynamic-build examples/basic/blog-post.page.gwdk
test -f /tmp/gowdk-dynamic-build/blog/hello-gowdk/index.html
test -f /tmp/gowdk-dynamic-build/blog/static-first/index.html
grep -q "hello-gowdk" /tmp/gowdk-dynamic-build/blog/hello-gowdk/index.html
```

## Rollback Plan

- Restore staticgen dynamic route rejection.
- Keep existing `paths {}` body capture unchanged.

## Risks

- Future `paths {}` grammar may differ from the literal subset. The subset is
  intentionally narrow and can be migrated behind the same build-time boundary.
- This slice supports string route params only; richer typed build execution
  remains future work.

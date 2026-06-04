# Implementation Plan: CSS Plugin Extension Point

## Context

Relevant spec: `docs/product/css-plugin-extension-spec.md`

Roadmap scope: Phase 4 CSS/plugin extension points.

## Assumptions

- Tailwind remains a future plugin.
- The first useful plugin contract can work with source metadata and generated
  output paths before full AST/class extraction exists.
- Static builds are the first compiler surface that should consume CSS results.

## Proposed Changes

- Add `FeatureCSS`, `Stylesheet`, `CSSProcessor`, `CSSContext`, `CSSResult`, and
  `CSSAsset` to the root package.
- Add `addons/css`.
- Add `BuildConfig.Stylesheets`.
- Extend static config parsing for literal `Build.Stylesheets`.
- Invoke CSS processors in `internal/staticgen`, write CSS assets, and inject
  stylesheet links in generated documents.
- Add tests and docs.

## Files Expected To Change

- `addon.go`
- `config.go`
- `css.go`
- `addons/css/css.go`
- `internal/project/config.go`
- `internal/project/config_test.go`
- `internal/staticgen/staticgen.go`
- `internal/staticgen/staticgen_test.go`
- `cmd/gowdk/main.go`
- Reference and product docs.

## Data And API Impact

- Public root API gains CSS config and processor types.
- Static build result gains CSS artifacts.
- CLI prints CSS artifact paths returned by static builds.

## Tests

- Unit: CSS addon registers `FeatureCSS`.
- Unit: config loader parses literal stylesheet links.
- Unit: staticgen emits stylesheet links.
- Unit: staticgen invokes a CSS processor and writes CSS output.
- Unit: staticgen rejects unsafe CSS asset paths before writing output.

## Verification Commands

```sh
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
go run ./cmd/gowdk check --ssr examples/basic/*.gwdk
go run ./cmd/gowdk manifest --ssr examples/basic/*.gwdk
rm -rf /tmp/gowdk-build && go run ./cmd/gowdk build --out /tmp/gowdk-build examples/basic/home.page.gwdk examples/basic/hero.cmp.gwdk
```

## Rollback Plan

- Remove CSS processor invocation from `internal/staticgen`.
- Remove CSS public types and `addons/css`.
- Keep existing static HTML output behavior unchanged.

## Risks

- Processor context may need to evolve once the compiler has a real AST. Keep the
  first contract small and metadata-oriented.
- Asset path rules may need hashing/fingerprinting later.

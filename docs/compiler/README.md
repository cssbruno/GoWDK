# Compiler

This directory documents GOWDK compiler behavior and generated-output contracts.

## Current Status

Implemented today:

- Recursive `.gwdk` discovery through `internal/discover`.
- Page and component metadata parsing through `internal/parser`.
- Manifest and site-map models through `internal/manifest` and `internal/lang`.
- Render-rule, duplicate identity, redundant component, and component contract
  validation through `internal/compiler`.
- SPA `view {}` markup and component invocation parsing through `internal/view`.
- Imported Go props/state contract resolution through `internal/gotypes`.
- Route-binding metadata for `gowdk routes` through `internal/compiler`.
- App-shell HTML, route manifest, and asset manifest emission for simple build-time
  pages, literal build data, imported Go build data functions, literal dynamic
  paths, components, partial runtime assets, and island runtime assets through
  `internal/buildgen`.
- Mandatory SPA build reports through `internal/buildgen`, written as
  `gowdk-build-report.json` for disk builds and returned on build errors.
- Generated embedded app source and optional binary compilation through
  `internal/appgen`, including supported action/API/fragment handlers, form
  input decoder and required-field validation wrappers, CSRF wiring, guards,
  and concrete or dynamic SSR routes with declared `load {}` fields.
- SPA `gowdk.config.go` loading for build source discovery, build targets,
  and output through `internal/project`.
- CLI tools for `tokens`, `fmt`, `check`, `manifest`, `sitemap`, `build`, and `lsp`.

Not implemented yet:

- Full AST/semantic/type analysis beyond the current component contract slice.
- Full component compilation, arbitrary `build {}` statements beyond expression
  records, and full `paths {}` execution.
- Broader generated-client reactivity, component non-CSS asset emission, and
  load/action invalidation.
- Broader hybrid request-time behavior beyond explicit `load {}` branches.

## Documents

- `pipeline.md`: current and target compile pipeline.
- `project-structure.md`: current source inputs and planned project layout.
- `generated-output.md`: planned generated artifacts and current limitations.
- `browser-compiler.md`: browser-facing partial runtime, JavaScript islands, and
  explicit WASM island behavior.
- `SPA-build-report.md`: generated build report schema and CLI debug output.
- `manifest.md`: manifest and site-map JSON contracts.

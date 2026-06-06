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
  `internal/appgen`, including first-slice action redirect handlers and form
  input decoder and required-field validation wrappers, partial action fragment
  responses, and first-slice SSR routes without `load {}`.
- SPA `gowdk.config.go` loading for build source discovery, build targets,
  and output through `internal/project`.
- CLI tools for `tokens`, `fmt`, `check`, `manifest`, `sitemap`, `build`, and `lsp`.

Not implemented yet:

- Full project config loading for every compiler command.
- Full AST/semantic/type analysis beyond the current component contract slice.
- Full component compilation, general interpolation, arbitrary `build {}`
  execution, and full `paths {}` execution.
- Real user Go type resolution for typed action decoders, user action logic,
  API/fragment/SSR handlers.
- Real user action/API/fragment execution.
- Generated `load {}` execution and hybrid request-time
  handlers.

## Documents

- `pipeline.md`: current and target compile pipeline.
- `project-structure.md`: current source inputs and planned project layout.
- `generated-output.md`: planned generated artifacts and current limitations.
- `browser-compiler.md`: browser-facing partial runtime, JavaScript islands, and
  explicit WASM island behavior.
- `SPA-build-report.md`: generated build report schema and CLI debug output.
- `manifest.md`: manifest and site-map JSON contracts.

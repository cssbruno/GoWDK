# Compiler

This directory documents GOWDK compiler behavior and generated-output contracts.

## Current Status

Implemented today:

- Recursive `.gwdk` discovery through `internal/discover`.
- Page and component metadata parsing through `internal/parser`.
- Manifest and site-map models through `internal/manifest` and `internal/lang`.
- Render-rule and duplicate identity validation through `internal/compiler`.
- Static `view {}` markup and component invocation parsing through `internal/view`.
- Route-binding planning through `internal/codegen`.
- Static HTML, route manifest, and asset manifest emission for simple build-time pages, literal build data, literal dynamic paths, and components through `internal/staticgen`.
- Generated embedded static app source and optional binary compilation through
  `internal/appgen`, including first-slice action redirect handlers and form
  input decoder and required-field validation wrappers.
- Static `gowdk.config.go` loading for build source discovery and output through `internal/project`.
- CLI tools for `tokens`, `fmt`, `check`, `manifest`, `sitemap`, `build`, and `lsp`.

Not implemented yet:

- Full project config loading for every compiler command.
- Full AST/semantic/type analysis.
- Full component compilation, general interpolation, arbitrary `build {}` execution, and full `paths {}` execution.
- Real user Go type resolution for typed action decoders, user action logic,
  API/fragment/SSR handlers.
- Generated action/API/fragment execution.
- Route-aware generated app registration beyond static file serving.

## Documents

- `pipeline.md`: current and target compile pipeline.
- `project-structure.md`: current source inputs and planned project layout.
- `generated-output.md`: planned generated artifacts and current limitations.
- `manifest.md`: manifest and site-map JSON contracts.
- `codegen.md`: route-binding planning and future emitters.

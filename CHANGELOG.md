# Changelog

GOWDK is experimental 0.x software. Public syntax, generated output, runtime
packages, and tooling contracts may change before a stable release.

## v0.2.3 - 2026-06-08

### Implemented

- Added TypeScript and JavaScript page/component script support with scoped
  page loading.
- Added inline `js {}` support for small browser snippets, while keeping file
  imports as the recommended path.
- Replaced parser-style regular-expression handling with lexer/scanner parsers
  across `.gwdk`, client-language, view/island, route, build-data, CSS, glob,
  LSP, runtime form, and generated action validation paths.
- Added `runtime/validation.MatchPattern` for generated form `pattern`
  validation without importing `regexp` into generated apps.
- Added optional framework/context bridge support and isolated optional adapter
  modules for Echo, Fiber, Gin, Redis Streams, NATS, and WebSocket fanout.
- Added all-module test scripts for root and nested optional Go modules.

### Changed

- Project positioning now uses the concrete "Project shape" instead of
  slogan-style project laws.
- `gowdk version` and the VS Code extension metadata now report `0.2.3`.
- Optional contract adapter modules require `github.com/cssbruno/gowdk v0.2.3`.

### Known Gaps

- GOWDK remains not production-ready.
- Generated output remains pre-1.0 and unstable unless a reference document
  explicitly marks a surface stable.
- GitHub milestones track capability buckets, not patch-release changelogs.

## v0.2.2 - 2026-06-08

### Implemented

- Added the first TypeScript scoped-script slice.
- Added lexer-backed parsing for the core `.gwdk` parser pattern layer.

### Known Gaps

- Superseded by `v0.2.3` for release metadata, changelog, and non-regexp
  scanner cleanup.

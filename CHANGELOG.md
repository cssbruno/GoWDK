# Changelog

GOWDK is experimental 0.x software. Public syntax, generated output, runtime
packages, and tooling contracts may change before a stable release.

## v0.2.7 - 2026-06-09

### Implemented

- Added `gowdk.Config.Env` with separate `Vars` and `Secrets` runtime
  environment contract declarations.
- Required env vars and secrets now fail config loading when unset or blank,
  while secret values stay out of config, diagnostics, generated code, and
  build artifacts.
- Generated embedded apps and backend-only apps repeat required env checks at
  startup before serving requests.
- Added stable env contract diagnostics for empty names, duplicate names,
  missing required names, and secret-looking names declared as normal vars.
- Added `gowdk inspect ir` as an M2 compiler IR inspection command.
- Added `gowdk add` for wiring built-in addons into `gowdk.config.go`.
- Added batteries-included `addons/auth` and `addons/db` packages for common
  auth/session/password and SQLC-style database wiring.

### Changed

- `gowdk version` and the VS Code extension metadata now report `0.2.7`.
- Optional contract adapter modules require `github.com/cssbruno/gowdk v0.2.7`.
- Generated request boundaries now apply the default per-request deadline, cap
  API request bodies, log recovered panics, and redact secret-looking values in
  diagnostics and runtime panic logs.
- CLI command code was split into per-command files without changing the public
  command surface.
- Release CI prunes high-churn CodeQL caches and keeps visible release asset
  checks aligned with the artifact list.

### Known Gaps

- GOWDK remains not production-ready.
- The env/secret contract is a fail-fast redundancy layer. Cloud providers,
  containers, process managers, or secret managers still own value injection.
- Env checks do not replace backend authorization, handler validation,
  database permissions, deployment secrets, CSRF, or guard backing code.

## v0.2.6 - 2026-06-08

### Changed

- `@page` is now optional for page files. When omitted, GOWDK derives the page
  ID from the source filename, such as `blog-post.page.gwdk` -> `blog-post`.
- `@route` and `@guard` remain explicit page metadata. Public pages must still
  declare `@guard public`.
- `gowdk init` now generates the thinner route-first page shape without an
  explicit `@page` annotation.
- Release packaging now uploads `dist/*` as a GitHub Actions workflow artifact
  and verifies the tag release has every expected download asset after upload.
- `gowdk version` and the VS Code extension metadata now report `0.2.6`.
- Optional contract adapter modules require `github.com/cssbruno/gowdk v0.2.6`.

### Known Gaps

- GOWDK remains not production-ready.
- Explicit `@page` is still supported when a stable page ID should not follow
  the filename.

## v0.2.5 - 2026-06-08

### Implemented

- Added explicit page access validation: real page sources must declare
  `@guard public` for public pages or protected guard IDs for guarded pages.
- Added thin native RBAC guard IDs with `role:<name>` and
  `permission:<name>` backed by application-owned `runtime/auth.Provider`
  implementations.
- Generated guarded apps now fail Go compilation when required backing hooks
  are missing: `GOWDKGuardRegistry` for custom guards and `GOWDKAuthProvider`
  for native RBAC guards.
- Protected page guards now require request-time page rendering so frontend
  page access can be checked before HTML is returned.

### Changed

- `gowdk version` and the VS Code extension metadata now report `0.2.5`.
- Optional contract adapter modules require `github.com/cssbruno/gowdk v0.2.5`.
- Examples, scaffolds, and language/reference docs now use explicit
  `@guard public` on intentionally public pages.

### Known Gaps

- GOWDK remains not production-ready.
- Guard metadata is a generated access redundancy layer and does not replace
  authorization in normal Go backend handlers and services.

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

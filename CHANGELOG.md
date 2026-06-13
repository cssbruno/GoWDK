# Changelog

GOWDK is experimental 0.x software. Public syntax, generated output, runtime
packages, and tooling contracts may change before a stable release.

## Unreleased

### Implemented

- M4 Go interop is complete for the current 0.x surface: a user can see why a Go
  function or type did or did not bind. `gowdk inspect go-bindings` emits a
  versioned JSON report (schema version 1) covering actions, APIs, fragments,
  SSR load functions, build-time Go calls, and web command/query references,
  each with source, source span, package, expected symbol, signature, input
  metadata, binding status, reason, and a next-step suggestion.
- `gowdk generate stubs` writes conservative missing action/API handler stubs to
  a `gowdk_stubs.go` file beside the owning source package, formats them with
  gofmt, and refuses to overwrite an existing stub file. Action stubs use
  `func(context.Context) (response.Response, error)` and API stubs use
  `func(context.Context, *http.Request) (response.Response, error)`.
- Build-time Go helpers may now return `T` or `(T, error)`; the runner tries the
  error-returning shape before falling back to the legacy single-return shape.
- Build-helper execution parses stdout as the JSON payload and preserves stderr
  only for failure messages, so successful helper logging no longer corrupts
  build data.
- `docs/reference/go-interop.md` is a first-class page documenting build data,
  supported action/API/load signatures, typed route-param access through
  `app.Params`/`app.TypedParams`, and `net/http` middleware wrapping.
- `gowdk check` and `gowdk build` now surface Go binding near-misses as
  non-fatal warnings, so a user sees why a handler did not bind without reading
  the JSON report or running a strict production build: a same-named function
  with an unsupported signature emits `unsupported_backend_signature`, and a
  same-named but unexported function (for example `func submit` where the block
  expects `Submit`) emits `unexported_backend_handler`. A handler with no
  candidate function stays silent so the 501-stub workflow is unaffected; strict
  production builds still fail closed via `backend_binding_required`. This is the
  first slice of #328.
- Backend handler binding no longer hides failures behind silent fallbacks: a
  handler declared in both same-package Go and an inline `go {}` block is
  reported as `ambiguous_backend_handler` instead of silently preferring one
  source; a sibling Go package that fails to compile keeps a "could not be
  inspected" binding instead of falling back to an inline block and reporting a
  misleading bound handler (the compile error is reported by `go_package_error`);
  and a failing `go list` for a same-package build function now surfaces its real
  cause (for example a missing `go.mod`) rather than a generic "requires a
  buildable Go package" message. A component-script resolution error during
  build now fails the build instead of silently omitting the page's component
  scripts.
- Generated `g:command` and `g:query` contract web adapters now use one JSON
  response contract: success writes the command/query result as no-store JSON,
  and failures write `{"error":"..."}` as no-store JSON with ordinary 5xx
  details redacted unless the handler returns an explicit
  `response.HandlerError`; command form parse, oversized body, CSRF, and input
  decode failures use the same JSON error shape.
- Contract event envelopes now carry stable IDs for durable delivery. Workers
  can opt into deduplication with `RunEventWorkerWithSeenStore` or
  `RunEventWorkerForRoleWithSeenStore`; duplicate IDs are acked without
  subscriber dispatch inside the configured window, and fresh IDs are marked
  seen only after dispatch and source ack succeed. Runtime includes bounded
  in-memory, file-backed, and Redis TTL seen-store adapters. File outbox records
  keep unique row IDs separate from event IDs, file-backed outbox/dead-letter
  and seen-store updates use temp-file replacement, and NATS batch drains
  preserve already-decoded events if a later drained message cannot be decoded.

### Known Gaps

- GOWDK remains not production-ready.
- Passing route params into Go build functions is deferred (#327).
- Generated per-route param structs and typed load/action result accessors are
  deferred (#23).
- Broader Go binding diagnostics for unsupported signatures, build-tag-hidden
  symbols, and unsupported return/parameter types are deferred (#328).
- Broader Go-package interop examples (`database/sql`, `pgx`, `sqlc`, `slog`,
  and similar) are deferred (#329).

## v0.3.0 - 2026-06-12

### Changed

- `gowdk version` and the VS Code extension metadata now report `0.3.0`.
- Optional contract adapter modules require `github.com/cssbruno/gowdk v0.3.0`.
- Conflict diagnostics (`duplicate_route`, `route_method_conflict`, including
  contract-route conflicts) now carry a related source location pointing at the
  first declaration. `gowdk check --json` gains an additive `related` array per
  diagnostic, and the language server reports it as `relatedInformation`.
- The formatter now tracks brace depth with the parser's string- and
  comment-aware scanner, so braces inside string literals, comments, and
  template literals (for example `title "a { b"`) no longer skew indentation.
- A page that declares no `guard` is no longer a build error. `guard` is now
  optional, but a page is not public by default: `missing_page_guard` is now a
  **warning** and the page's route is denied (403) at request time until the
  author adds `guard public` or a protective guard. Static pages are denied
  through the generated app's deny registry; request-time SSR pages are denied
  in their own handler. Access is never granted by omission.
- Compiler validation diagnostics now carry a severity. Warning-severity
  diagnostics surface to authors and editors but do not fail the build.

### Implemented

- M3 route, endpoint, and contract reporting is complete for the current 0.x
  surface: `gowdk routes`, `gowdk endpoints`, `gowdk inspect tree`, and
  `gowdk inspect endpoint-graph` expose source-linked route and backend
  metadata without requiring users to read generated source.
- `gowdk build` writes `openapi.json` for the routable web surface and
  `asyncapi.json` for contract integration-event metadata.
- Route and endpoint reports include versioned JSON, source spans, route
  params, render/cache metadata, guards, planned handlers, backend binding
  state, contract binding state, and non-fatal route-mode notes.
- A machine-checked `.gwdk` conformance corpus
  (`internal/lang/testdata/conformance/`) pins the language contract: `accept/`
  cases must check clean and `reject/` cases must produce their declared stable
  diagnostic codes. See `docs/language/conformance.md`.
- A per-construct stability and deprecation table
  (`docs/language/stability.md`) documents which blocks, metadata keywords, and
  `g:` directives are stable, partial, planned, or deprecated, guarded against
  drift from the code registries by a test.
- `source.SourcePosition` carries a byte `Offset`, with `source.PositionAt` and
  `source.OffsetOf` conversion helpers, as the exact substrate for future
  AST-backed formatting and precise editor edits.
- ADR 0010 records the decision to replace the line-oriented parser with a
  shared tokenizer and a recursive-descent parser with error recovery, migrated
  behind the stable `gwdkast` AST boundary.
- The default-deny contract now covers every way a guardless page could leak:
  dynamic build-time pages (`paths {}`) are denied by route pattern so each
  concrete artifact returns 403, direct index artifact paths
  (`/dashboard/index.html`) are normalized to their route before the deny check,
  and a guardless page that declares `act`/`api`/`fragment` endpoints is a build
  **error** (`missing_page_guard`) because those endpoints would otherwise be
  publicly callable.

### Known Gaps

- GOWDK remains not production-ready.
- Generated output and tooling reports remain pre-1.0 and may change unless a
  reference document marks a surface stable.
- M3 completes the current reportability milestone; it does not complete secure
  runtime, SSR/hybrid, component/client, or production operations hardening.

## v0.2.8 - 2026-06-10

### Changed

- Layout identity is now derived from the `.layout.gwdk` file name. An
  `layout` metadata declaration inside a layout file no longer declares identity; it
  declares the parent layout(s) the layout inherits from and is optional.
- `gowdk version` and the VS Code extension metadata now report `0.2.8`.
- Optional contract adapter modules require `github.com/cssbruno/gowdk v0.2.8`.

### Implemented

- A layout that references itself through `layout` is now a compile error
  (`layout_self_reference`), as is a cyclic layout inheritance chain
  (`cyclic_layout_reference`).
- A layout whose `layout` parent does not resolve to a declared layout now
  reports `unknown_layout_id` at validation time.
- A layout must contain exactly one `<slot />` placeholder. Layouts with zero
  or multiple slots now hard-error at validation time (`layout_slot_count`)
  instead of failing later during composition.

### Known Gaps

- GOWDK remains not production-ready.

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

- `page` is now optional for page files. When omitted, GOWDK derives the page
  ID from the source filename, such as `blog-post.page.gwdk` -> `blog-post`.
- `route` and `guard` remain explicit page metadata. Public pages must still
  declare `guard public`.
- `gowdk init` now generates the thinner route-first page shape without an
  explicit `page` metadata declaration.
- Release packaging now uploads `dist/*` as a GitHub Actions workflow artifact
  and verifies the tag release has every expected download asset after upload.
- `gowdk version` and the VS Code extension metadata now report `0.2.6`.
- Optional contract adapter modules require `github.com/cssbruno/gowdk v0.2.6`.

### Known Gaps

- GOWDK remains not production-ready.
- Explicit `page` is still supported when a stable page ID should not follow
  the filename.

## v0.2.5 - 2026-06-08

### Implemented

- Added explicit page access validation: real page sources must declare
  `guard public` for public pages or protected guard IDs for guarded pages.
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
  `guard public` on intentionally public pages.

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

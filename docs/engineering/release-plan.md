# 0.x Improvement Checklist

This document is the 0.x hardening backlog for GOWDK after the v0.1 release
line. It is intentionally not a finish-line roadmap. It records what to improve
while preserving the product rules already defined in
`docs/product/roadmap.md`, `docs/product/requirements.md`, and
`docs/engineering/architecture.md`.

Minor versions such as `v0.2.0`, `v0.3.0`, `v0.4.0`, or later `0.x` tags are
release vehicles. They may group work from the waves below, but they must not
promise production readiness or imply that the project has reached a final
framework shape.

## Product Rules To Protect

- [ ] Keep GOWDK framed as an experimental 0.x Go-first compiler/runtime.
- [ ] Do not add production-ready claims.
- [ ] Do not add a finish-line version promise.
- [ ] Do not add migration-guide docs.
- [ ] Do not add framework comparison docs as core docs.
- [ ] Keep `.gwdk` as the declaration layer.
- [ ] Keep normal Go as the behavior layer.
- [ ] Keep generated Go as adapter glue.
- [ ] Keep static/build-time pages as the default.
- [ ] Keep request-time behavior explicit.
- [ ] Keep SSR opt-in.
- [ ] Keep hybrid behavior explicit.
- [ ] Keep generated JavaScript as enhancement only.
- [ ] Do not let generated JavaScript own auth, routing truth, validation truth,
  business logic, server state, or cache policy.
- [ ] Keep `net/http` as the runtime boundary.
- [ ] Keep Chi, Gin, Echo, and Fiber as optional adapters.
- [ ] Keep Redis, NATS, SSE, and WebSocket adapters optional.
- [ ] Keep Tailwind optional.
- [ ] Keep npm optional.
- [ ] Do not download optional tools during normal builds.
- [ ] Make unsupported behavior fail loudly with diagnostics.
- [ ] Make partial behavior visible in docs, examples, CLI output, and release
  notes.

## Release Wave Index

These waves are ordered by dependency, not by promised version number.

| Wave | Theme | Outcome |
| --- | --- | --- |
| Public Truth | Release metadata, README status, known gaps, and repo issue hygiene. | Users can tell what works, what is partial, and what is unsafe to rely on. |
| Release Trust | Release workflow, notes, checksums, attestations, docs checks, and smoke tests. | Releases are reproducible and clearly experimental. |
| Compiler Spine | AST, analyzer, IR, diagnostics, generated output, and deterministic generation. | Compiler phases are explicit and boring. |
| Go Interop | Go binding inspection, stubs, typed params, build/load contracts, and package resolution. | Go code is easy to connect and debug. |
| Endpoint Adapters | Unified endpoint metadata, adapter IR, strict binding mode, and reports. | Generated backend glue is strict, readable, and framework-neutral. |
| Secure Runtime | Actions, APIs, fragments, CSRF, redirects, guards, timeouts, limits, errors, and logs. | Runtime behavior is safer without claiming production readiness. |
| SSR And Hybrid | Request-time page contracts, load, guards, route params, errors, cache, and hybrid behavior. | Request-time lanes are explicit and documented. |
| Components And Islands | Component contracts, client language, reactivity, SPA navigation, and WASM islands. | Browser behavior stays bounded to enhancement and local UI. |
| CSS And Assets | Optional CSS processors, Tailwind, scoped CSS, asset manifests, and packaging. | Assets are deterministic and optional tools stay optional. |
| DX And Examples | CLI inspection, doctor/explain, dev server, LSP, native docs, and examples. | Users can learn GOWDK through native examples, not migration positioning. |
| Ops And CI | Dependency policy, security policy, production-safety gates, CI, performance, and operations docs. | Hardening is measurable and visible. |

## GitHub Milestone Buckets

GitHub milestones are the planning source of truth. Release tags are separate
release vehicles: a `v0.x.y` tag may ship completed work from one or more
milestones, but a tag must not rename, skip, or imply completion of a GitHub
milestone unless the related issues and docs agree.

Last synced with GitHub milestones on 2026-06-09.

| Milestone | Focus | Release gate |
| --- | --- | --- |
| M2 - Compiler + Language Contract | Compiler spine, diagnostics, source spans, parser/formatter hardening, and the current `.gwdk` language contract. | Build output, app generation, CLI reports, and language diagnostics use typed compiler records for supported paths, or remaining compatibility paths are explicitly listed. |
| M3 - Route / Endpoint / Contract Reports | Routes, endpoints, contracts, diagnostics, source maps, and editor-navigable reports. | A user can inspect route, endpoint, and contract metadata without reading generated source. |
| M4 - Go Interop | Go binding inspection, stubs, typed params, build/load contracts, and package resolution. | A user can see why a Go function or type did or did not bind. |
| M5 - Secure Endpoint Runtime | Strict endpoint adapters, body/header/time limits, CSRF response contract, panic boundaries, and safe redirects. | Actions, APIs, fragments, and generated request-time handlers have tested failure paths for invalid input, missing handlers, CSRF failure, guard failure, limits, redirects, and panic recovery. |
| M6 - Contracts Web Adapter | Stable `g:command`/`g:query` web adapters, contract role diagnostics, local outbox docs, and worker replay docs. | Web command/query behavior and worker replay behavior are explicit, optional, and do not replace action/API declarations prematurely. |
| M7 - SSR / Hybrid | Request-time page lifecycle, load contracts, hybrid behavior, cache, guards, and route-local errors. | Concrete and dynamic request-time pages can be built into generated binaries with tested load, guard, error, cache, and direct-refresh behavior. |
| M8 - Components / Client Language | Component props, slots, events, client reactivity, SPA navigation, lifecycle, and WASM islands. | Component and client behavior remains local enhancement and cannot own routing, auth, server validation, business logic, server state, or cache policy. |
| M9 - Assets / WASM / Packaging | Optional adapters, CSS/assets, WASM ABI, cache manifests, generated app packaging, and dependency surface policy. | Optional browser WASM, Tailwind, CSS, assets, and packaging behavior are testable without making npm, Tailwind, or WASM mandatory. |
| M10 - DX / Product Experience | VS Code cockpit, examples, cookbook, flagship app, docs, operations, website, playground, CI, and learning path. | A new user can install, inspect, debug, learn, and run native GOWDK examples through documented commands. |
| M11 - Auth Addon Hardening | Auth crypto stance, swappable password hashing, fail-closed session secret contract, CSRF/session interplay, guard docs, and addon docs. | Auth addon defaults and examples are explicit about crypto, secrets, sessions, CSRF, and guard boundaries. |
| M12 - DB Addon Hardening | `database/sql` plumbing maturity: migrations apply, transaction/health helpers, sqlc walkthrough docs, and real-driver nested tests. | DB addon behavior stays plumbing-only and does not own domain code or schema DSL semantics. |
| M13 - WebSocket / Realtime Addon | Presentation-event fanout packaged as an opt-in WebSocket addon with config and `gowdk add` wiring. | Realtime transport packaging is optional and dependency-isolated; SSE remains the dependency-free core default. |
| M14 - Realtime Reactivity Wiring | ADR-gated server presentation events drive bounded live DOM updates without user JavaScript. | Realtime UI updates have explicit `.gwdk` syntax, IR, validation, guard gating, backpressure, reconnect, and client patch-loop contracts. |

Milestone issue counts and titles live in GitHub. If this table drifts, update
it from GitHub before adding new release planning text.

## Standard Release Gates

Every 0.x minor release must have:

- [ ] Release notes that begin with "Experimental 0.x release" and "Not
  production-ready."
- [ ] Release notes split into implemented, partial, planned, intentionally out
  of scope, required verification, and known gaps.
- [ ] Current requirement statuses in `docs/product/requirements.md`.
- [ ] Current architecture notes in `docs/engineering/architecture.md`.
- [ ] Current CLI, config, generated-output, routing, deployment, and examples
  docs for changed behavior.
- [ ] Passing CI and current manual release gates from
  `docs/engineering/release.md`.
- [ ] Checksums and artifact attestation instructions in release notes.
- [ ] A release checklist link in release notes.
- [ ] A "no production claim" check before publishing.
- [ ] Draft and pre-release GitHub release settings unless the release policy is
  deliberately changed.

## Public Truth

- [ ] Verify `v0.1.5` is marked as a pre-release on GitHub.
- [ ] If GitHub displays it only as "Latest," update release settings or release
  wording.
- [ ] Add a release note template in `.github/`.
- [ ] Add "known limitations" to every release.
- [ ] Add "breaking/unstable generated output" to every release.
- [ ] Add CLI artifact verification instructions to release notes.
- [ ] Add checksum verification instructions to release notes.
- [ ] Add attestation verification instructions to release notes.
- [ ] Add VS Code `.vsix` install instructions to release notes.
- [ ] Add exact Go version requirement to release notes.
- [ ] Add exact Node version requirement for extension build/test to release
  notes.
- [ ] Create a public project board named `0.x Hardening`.
- [ ] Add waves to the board: Public truth, Release trust, Compiler spine, Go
  interop, Endpoint adapters, Secure runtime, SSR/hybrid, Components/islands,
  CSS/assets, DX/examples, and Ops/docs.
- [ ] Convert every `Partial` requirement into an issue.
- [ ] Convert every `Planned` roadmap item into an issue unless the item is
  intentionally tracked only in docs.
- [ ] Add labels for compiler, parser, IR, diagnostics, generated Go, runtime,
  actions, API, fragments, SSR, hybrid, components, client, WASM, CSS, assets,
  security, ops, docs, examples, LSP, dev server, release blocker, breaking
  change, good first issue, safe to try today, blocked by compiler IR, blocked
  by security hardening, and blocked by generated app runtime.
- [ ] Add issue templates for compiler bugs, generated output bugs, runtime
  bugs, security concerns, docs gaps, example requests, language proposals, and
  addon proposals.
- [ ] Add README links to known gaps, the release checklist, and the public
  hardening board.

## README And Getting Started

- [ ] Keep the pre-release warning near the top.
- [ ] Add "experimental 0.x" wording near the top.
- [ ] Add "public contracts may change" near the install section.
- [ ] Move "What works today," "What is partial," and "What is planned" higher.
- [ ] Add a compact support matrix for static build output, dynamic SPA paths,
  build-time Go data, actions, APIs, fragments, SSR, hybrid, components, WASM
  islands, CSS/assets, one-binary output, contracts, dev server, and LSP.
- [ ] Include matrix columns for stable enough to demo, not production
  security, docs available, example available, and tests available.
- [ ] Add direct links from matrix rows to docs, examples, or issues.
- [x] Replace slogan-style project laws with the concrete project shape:
  `.gwdk` source declarations, normal Go behavior, compiler IR/output,
  generated runtime wiring, explicit request-time lanes, bounded browser
  enhancement, optional integrations, and diagnostics for unsupported source.
- [ ] Replace vague "full-stack" wording with concrete supported lanes.
- [ ] Keep install from GitHub release asset as the first install path.
- [ ] Keep build from source as the contributor/development path.
- [ ] Add Linux, macOS Intel, macOS ARM, and Windows install examples.
- [ ] Add checksum and attestation verification examples.
- [ ] Add `gowdk version` verification.
- [x] Add `gowdk doctor` verification once implemented.
- [ ] Add troubleshooting for missing `gowdk.config.go`, missing Tailwind
  binary, unsupported Go handler signatures, missing SSR feature, and generated
  binary build failures.
- [ ] Warn that `gowdk serve` is static-output only and generated binaries are
  needed for actions, APIs, fragments, SSR, and hybrid runtime behavior.

## Release Trust

- [x] Keep `scripts/test-go-modules.sh` in the release gate.
- [x] Keep `scripts/vulncheck-go-modules.sh` in the release gate.
- [x] Keep `go build ./cmd/gowdk` in the release gate.
- [x] Keep Node syntax checks and VS Code Node tests.
- [x] Keep example `check`, `manifest`, `sitemap`, and `routes` gates.
- [x] Add `gowdk version --json` check after building artifacts.
- [x] Add smoke execution for each OS/arch artifact where possible.
- [x] Add generated CLI artifact checksum verification after checksum file
  generation.
- [ ] Add generated binary HTTP smoke tests for static, SSR, action POST, API
  GET, fragment, hybrid, and WASM build paths.
- [x] Add VS Code `.vsix` package existence check.
- [x] Add release body validation for experimental warning, not-production-ready
  warning, known gaps, and checksum instructions.
- [ ] Add docs link checker.
- [ ] Add Markdown lint.
- [ ] Add generated docs sync check.
- [ ] Add generated output determinism check.
- [ ] Add `gofmt` check.
- [ ] Add `go vet ./...`.
- [ ] Add dependency, license, module graph, and dependency-size reports.
- [x] Add security policy consistency check.
- [ ] Add examples README command consistency check.
- [x] Add a "no migration docs" check if this becomes a hard policy.

## Toolchain And Dependency Policy

- [ ] Add `toolchain go1.26.4` if stronger local toolchain behavior is desired.
- [x] Add `gowdk doctor` checks for Go version and required local tools.
- [x] Add CI and release checks that print `go version` and `go env GOVERSION`.
- [x] Document exact Go version requirements and future patch compatibility.
- [x] Explain `govulncheck` in release docs.
- [x] Keep `docs/engineering/dependency-policy.md` current.
- [x] Classify dependencies as compiler core, runtime core, optional HTTP
  adapters, optional broker adapters, optional realtime adapters, optional
  CSS/tool adapters, or test/dev only.
- [x] Explain why Chi, Gin, Echo, Fiber, Redis, NATS, and WebSocket packages are
  direct dependencies, or move them to optional submodules.
- [ ] Add CI checks for new direct dependencies.
- [ ] Add dependency diff, license report, vulnerability report, and module
  graph report to releases.
- [x] Enforce no mandatory npm and no build-time downloads.
- [ ] Test that generated code does not import Chi, Gin, Echo, Fiber, Redis, or NATS
  by default.

## Security And Production-Safety Gates

- [x] Update root `SECURITY.md` to match `docs/engineering/security.md`.
- [x] Keep the production warning.
- [x] Replace outdated "planned but not complete" wording with precise
  "first slice exists, not production enforcement" wording.
- [x] List implemented first slices: generated action decoding, unexpected
  field rejection, direct literal request-shape validation, opt-in CSRF,
  configurable action/API body caps, generated `http.Server` timeout defaults,
  `MaxHeaderBytes`, safe local redirect slice, guard execution slice, SSR panic
  boundaries, and no-store request-time responses.
- [x] List incomplete production areas: auth/session policy, full guard
  contract, CSRF secret rotation, full redirect policy, per-route body/header
  limit policy beyond current defaults, file upload policy, public API
  hardening, realtime security policy, and admin tooling policy.
- [x] Enable GitHub private vulnerability reporting if available.
- [x] Add a vulnerability report contact path.
- [x] Add threat models for compiler diagnostics, generated logs, actions,
  APIs, fragments, SSR load, guards, generated assets, VS Code extension, WASM
  islands, and contracts/realtime.
- [x] Add security checklist items to the PR template.
- [x] Add security review trigger labels.
- [x] Add generated `http.Server` timeout configuration: read, write, idle, and
  read-header timeouts.
- [x] Add `MaxHeaderBytes`.
- [x] Keep action request body caps and add API/fragment body caps where
  relevant. Current action and API lanes use configurable caps that default to
  1 MiB; generated standalone fragments are GET-only and action fragments share
  the action cap.
- [x] Add configurable body limits.
- [x] Add explicit 405 responses.
- [x] Ensure panic recovery wraps all generated request-time user Go.
- [x] Ensure production-safe error pages and no stack traces in production mode.
- [x] Prevent secret values in diagnostics and logs.
- [x] Add log redaction for cookies, auth headers, CSRF tokens, passwords,
  secrets, sensitive form fields, and sensitive query params.
- [x] Add secure headers middleware or docs for `X-Content-Type-Options`,
  `Referrer-Policy`, `Content-Security-Policy`, frame policy, and optional
  HSTS.
- [x] Add cookie helper docs for `HttpOnly`, `Secure`, `SameSite`, path, and
  domain policy.
- [x] Add safe redirect allowlists, open redirect tests, and unsafe external
  redirect diagnostics.
- [x] Add embedded secret exclusion tests for `.env`, source maps with secrets,
  private files, and temporary artifacts.
- [x] Add reverse proxy, TLS termination, request ID, health endpoint, and
  metrics endpoint security policy docs.

## Compiler Spine

- [x] Keep lex -> parse -> AST -> analyze -> IR -> validate -> generate as
  strict phases. See `docs/compiler/pipeline.md` and
  `docs/engineering/architecture.md`.
- [x] Finish downstream migration to `internal/gwdkir`.
- [x] Make build output generation, app generation, CLI reports, and LSP
  metadata consume typed IR.
- [x] Remove compatibility structs from long-term generation paths.
- [x] Add golden coverage for implemented compiler-spine handoffs: AST, IR,
  generated Go, generated HTML/CSS, manifest, route report, endpoint report,
  build report, and route/asset output manifests. Component graph and asset
  graph commands are deferred to #235.
- [x] Add source spans to current AST and IR records where possible. Remaining
  exact-span improvements for diagnostics, reports, and LSP metadata are
  deferred to #235.
- [x] Add compiler invariant checks.
- [x] Gate invalid IR before generated-output planning. Test-only invalid-IR
  panic helper policy is deferred to #235.
- [x] Add deterministic output, stale output cleanup, and unchanged-output
  preservation tests.
- [x] Ban stringy generated Go except temporary documented exceptions. See
  `docs/engineering/generated-code-policy.md`.
- [x] Move generated Go to `go/ast`, `go/printer`, and `go/format` for current
  generated app and adapter surfaces.
- [x] Add internal architecture docs for compiler passes.
- [x] Add contributor guidance for new syntax requiring parser, formatter,
  diagnostic, IR, generation, docs, and example/fixture coverage. See
  `docs/compiler/syntax-contributors.md`.

## Parser, Formatter, Diagnostics, And Language Spec

- [x] Add stable diagnostic codes. See
  `internal/diagnostics/registry.go` and
  `docs/reference/diagnostic-codes.md`.
- [x] Add `gowdk explain <diagnostic-code>`.
- [x] Make `gowdk check --json` a stable tooling contract. See
  `docs/reference/diagnostics.md` and `docs/language/diagnostics.md`.
- [x] Add parser recovery so one syntax error does not hide the rest of the
  file. The shared parser now accumulates declaration/block diagnostics while
  preserving broad `parse_error` fallback diagnostics where no stable code
  exists.
- [x] Add exact spans and suggestions for package declarations, imports, `use`,
  metadata declarations, routes, layouts, render modes, `paths`, `build`, `load`, `view`,
  `style`, `client`, `go`, `go ssr`, `go client`, `go addon.*`, actions, APIs,
  fragments, component props, component state, and WASM declarations. Current
  metadata/block/view/client surfaces have span and suggestion coverage where
  parser records exist; remaining broader exact-span work is deferred to #250.
- [x] Add suggestions for missing config, missing SSR feature, duplicate routes,
  unsupported handler signatures, missing exported Go symbols, invalid route
  params, unsupported build functions, unsupported component props, and missing
  Tailwind command. Implemented suggestion surfaces are documented in
  `docs/reference/diagnostics.md`; remaining suggestion expansion is deferred to
  #250.
- [x] Add formatter idempotence and comment preservation tests. Parser-backed
  formatting beyond the current line-oriented formatter is deferred to #250.
- [x] Add malformed syntax tests. Current parser/check fixtures cover malformed
  metadata, imports, `use`, endpoint migration, and unsupported build syntax;
  broader recovery-driven malformed syntax coverage is deferred to #250.
- [x] Add parser, route matcher, view parser, and form decoder fuzz tests.
  Deferred to #250 until the invariants and seed corpus for those fuzz targets
  are documented.
- [x] Write a formal `.gwdk` language spec covering file kinds, package rules,
  Go imports, component `use`, layout references, asset references, addon
  references, metadata declarations, blocks, expressions, view markup, component calls,
  slots, event bindings, class/style directives, `g:` directives, comments,
  reserved words, Go identifier mapping, route params, dynamic paths, raw HTML
  policy, unsupported syntax behavior, deprecation policy, and 0.x
  compatibility. See `docs/language/spec.md`; remaining expansion is deferred to
  #250.
- [x] Add grammar examples, invalid syntax examples, diagnostics examples, and a
  GOWDK-native mental model guide. Current grammar, diagnostics, and language
  spec docs cover the baseline; broader examples and mental-model expansion are
  deferred to #250.

## Go Interop

- [x] Make Go interop a first-class docs page.
- [x] Add `gowdk inspect go-bindings`.
- [x] Add `gowdk generate stubs`.
- [x] Support build functions returning `(T, error)`.
- [x] Support same-package build functions consistently.
- [x] Support imported package aliases consistently.
- [ ] Support route params into build functions. Deferred to #327.
- [x] Support `context.Context` for request-time functions.
- [ ] Support typed route params in `load`, APIs, actions, and fragments where
  relevant. The current generated route context exposes `app.Params(ctx)` and
  `app.TypedParams(ctx)` for SSR/load; per-route structs and broader
  action/API/fragment typed accessors are deferred to #23.
- [x] Add Go symbol discovery reports.
- [ ] Add diagnostics for unsupported signatures, hidden-by-build-tags symbols,
  non-exported symbols, wrong packages, ambiguous imports, unsupported return
  types, unsupported parameter types, and JSON encoding failures for build data.
  First slice landed: `gowdk check`/`build` now emit `unsupported_backend_signature`
  and `unexported_backend_handler` warnings for backend handler near-misses.
  Build-tag-hidden symbols, wrong packages, ambiguous imports, detailed
  return/parameter type diagnostics, and build-data JSON encoding diagnostics
  remain deferred to #328.
- [ ] Add examples using normal Go packages such as `database/sql`, `pgx`,
  `sqlc`, `slog`, session packages, validator packages, email packages,
  markdown packages, image processing packages, and queue packages. Deferred to
  #329.
- [x] Keep serious app behavior in `.go` files.
- [x] Keep inline `go {}` extractable and testable.
- [x] Document that `.gwdk` calls supported Go contracts and is not arbitrary Go
  everywhere.
- [x] Add tests for package path resolution, aliased imports, build tags,
  generated `gowdk_go/` packages, same-package handler discovery, and imported
  build-data errors.

## Routes, Layouts, View Engine, And HTML Safety

- [x] Formalize the current route pattern grammar, trailing slash policy,
  encoded path handling, route params, typed param helpers, and final-segment
  rest params. Route-priority/report hardening beyond the current generated
  server behavior is deferred to #237.
- [x] Add route conflict diagnostics for page, API, action, fragment, and
  contract endpoint combinations in the current route model.
- [x] Add versioned route reports for current route and endpoint metadata.
  Route records include source spans, route params, guards, layouts, cache, and
  planned handlers. Generated output paths remain in `gowdk-routes.json`
  because dynamic SPA routes can expand to multiple files.
- [x] Add a source-linked inspect tree for current package, page, component,
  layout, route, endpoint, contract-reference, and view markup nodes. See
  [#317](https://github.com/cssbruno/GoWDK/issues/317).
- [x] Add current direct refresh, 404, encoded param, static SPA, dynamic SPA
  `paths`, SSR, hybrid, API, action, fragment, and trailing-slash tests.
  Remaining path-traversal and expanded matrix coverage is deferred to #237.
- [x] Define current layout composition, nested layout behavior, ordering, slot
  rules, request-aware runtime layout contracts, package-scoped layout imports,
  and qualified layout references/diagnostics. Generated app request-aware and
  hybrid layout wiring follow-ups are deferred to #237.
- [x] Add current head and metadata support for `title`, route metadata,
  description, canonical URL, Open Graph, Twitter card, app `Head`, and
  sitemap/manifest metadata. Robots/noindex, preload, and prefetch are deferred
  to #237.
- [x] Document supported HTML subset.
- [x] Escape text and attributes by default.
- [x] Define URL escaping, boolean attributes, class binding, style binding,
  event binding, form binding, and raw HTML policy.
- [x] Add unsafe raw HTML diagnostics for the explicit `g:html` escape hatch.
- [x] Add the first practical accessibility warning, `missing_img_alt`.
  Missing labels, empty links, button type, and heading order are deferred to
  #237.
- [x] Add unsafe `href`, `src`, and `action` tests plus script and attribute
  injection tests for the current view renderer.

## Endpoint Adapters, Actions, APIs, And Fragments

- [x] Normalize actions, APIs, fragments, SSR loads, commands, and queries into
  one endpoint/contract metadata model for the current report and adapter IR
  surface.
- [x] Include source file, source span, kind, package path, package name, symbol,
  method, path, signature kind, input type, guards, CSRF policy, cache policy,
  and binding status in the current endpoint report.
- [ ] Add endpoint report output type and rate limit policy once the adapter IR
  carries stable metadata for both. Deferred to
  [#57](https://github.com/cssbruno/GoWDK/issues/57).
- [x] Add strict binding mode for production-shaped builds. See
  [#83](https://github.com/cssbruno/GoWDK/issues/83).
- [x] Allow 501 stubs only behind an explicit flag. See
  [#87](https://github.com/cssbruno/GoWDK/issues/87).
- [x] Keep development/default builds compatible and make production migration
  stubs explicit through `Build.AllowMissingBackend` /
  `--allow-missing-backend`.
- [x] Add endpoint conflict diagnostics and a versioned endpoint report command.
- [x] Emit OpenAPI and AsyncAPI inspection artifacts from `gowdk build` for the
  routable web surface and integration-event contract surface.
- [x] Add endpoint graph output. See
  [#319](https://github.com/cssbruno/GoWDK/issues/319).
- [x] Generate adapters from typed IR with deterministic imports, route
  registration, request decoding, and response writing for the current
  action/API/fragment/SSR/contract surface. See
  [#86](https://github.com/cssbruno/GoWDK/issues/86).
- [x] Test generated adapters for success, validation error, missing handler,
  unsupported handler, redirect, guard failure, CSRF failure, panic, no-store
  response, and method not allowed.
- [x] Fully document action syntax, methods, form encoding, JSON support,
  direct file input rejection, multipart rejection, user-owned uploads, typed
  input decoding, scalar decoding, unknown field policy, missing/repeated field
  policy, checkbox/radio/select policy, submit intent, request-shape
  validation, domain validation handoff, validation error shape, partial
  validation fragments, redirects, reload outcomes, CSRF token placement,
  invalid CSRF response, and body limits.
- [ ] Add broader action examples for contact, settings, validation fragments,
  redirects, and partial fragment responses beyond the current
  newsletter/signup/login slices. Deferred to
  [#337](https://github.com/cssbruno/GoWDK/issues/337).
- [x] Document and add broader API helpers for strict JSON bodies, typed query
  params, JSON/error/no-content responses, content type handling, and
  method-not-allowed behavior. Generated typed route params, `(T, error)`
  signatures, CORS, and richer examples remain planned.
- [x] Document supported API signatures, context/request support,
  `response.Response`, generated error handling, guards, no-store responses,
  missing/unsupported handler behavior, and unsupported methods for the current
  first slice.
- [ ] Add broader API examples for session, search, JSON CRUD, and webhooks with
  user-owned validation. Deferred to
  [#337](https://github.com/cssbruno/GoWDK/issues/337).
- [x] Document fragments, standalone fragment routes, action-returned
  fragments, validation fragments, `g:target`, `g:swap`, swap modes, partial
  request headers, no-JS fallback, errors, focus restoration, island remounts,
  and no-store behavior.
- [ ] Add fragment examples for inline validation, table row update, list
  refresh, modal body update, and dashboard card refresh. Deferred to
  [#337](https://github.com/cssbruno/GoWDK/issues/337).

## SSR, Hybrid, Cache, Guards, And Auth Hooks

- [x] Document SSR lifecycle, render mode, feature requirement, `load {}`
  grammar, declared load paths, typed route params, `(T, error)` load functions,
  `context.Context` load functions, redirects, not found, custom errors,
  route-local error pages, endpoint-local error pages, panic boundaries,
  guard-before-load ordering, layout-data merge, and cache policy.
- [ ] Add SSR examples for simple pages, dashboards, guarded account pages,
  dynamic detail pages, and route-local error pages. Deferred to
  [#102](https://github.com/cssbruno/GoWDK/issues/102).
- [x] Document hybrid lifecycle, bare hybrid behavior, hybrid with and without
  `load`, SSR feature requirement, cache, revalidation, action invalidation,
  fragment refresh, and data refresh.
- [x] Defer hybrid streaming until simpler behavior is stable.
- [x] Add route/build report output that shows hybrid clearly.
- [x] Document static asset, SPA HTML, SSR HTML, API, action, fragment, and
  hybrid cache policy.
- [x] Document `cache` and `revalidate`.
- [x] Add route report cache column and build report cache section.
- [x] Test immutable asset cache, SPA `no-cache`, request-time `no-store`,
  `cache`, `revalidate`, and invalid `revalidate`.
- [x] Document guard syntax, required backing hooks, guard failure behavior, and
  support matrix for SSR, actions, APIs, fragments, and hybrid.
- [x] Document request context helpers for request, params, CSRF, session, and
  app context.
- [ ] Add user-owned session, cookie session, bearer token, admin role,
  guest-only page, JSON auth failure, redirect auth failure, and partial auth
  failure examples. Deferred to
  [#102](https://github.com/cssbruno/GoWDK/issues/102) and
  [#337](https://github.com/cssbruno/GoWDK/issues/337).

## Components, Client Language, SPA Navigation, And WASM

- [ ] Document component contracts, file structure, import/use rules, props,
  slots, events, state, lifecycle, CSS/assets, and unsupported behavior.
- [ ] Add required/default/boolean/numeric/string/object/array/imported Go
  struct prop support as contracts become stable.
- [ ] Add prop validation diagnostics.
- [ ] Add named slots and scoped slots only when syntax is stable.
- [x] Add child-to-parent events, typed event payloads, bindable state, mount,
  update, cleanup, real `g:if`, `g:for`, keyed `g:for`, keyed DOM updates,
  recursion policy, and dynamic component policy.
- [ ] Add component snapshot and browser behavior tests.
- [ ] Add native component examples for buttons, text fields, cards, counters,
  tabs, modals, dropdowns, tables, pagination, toasts, form fields, and nav
  menus.
- [ ] Document `client {}` state, computed values, handlers, allowed/rejected
  expressions, dependency graph, cycles, batching, update order, cleanup, async
  policy, event policy, DOM patch policy, browser diagnostics, and production
  minification later.
- [ ] Test computed updates, class toggles, conditional DOM, event handlers,
  repeated state updates, cycles, cleanup, partial swap remounts, and SPA
  navigation remounts.
- [ ] Document static-first SPA navigation, link interception, external links,
  downloads, hash links, targets/new tabs, prefetch, route asset prefetch,
  scroll restoration, focus restoration, loading UI, error UI, and optional
  enhancement behavior.
- [ ] Add no-JS, direct refresh, browser back/forward, route swap, island
  remount, fragment remount, and generated JS size tests/reports.
- [ ] Document and version the WASM island ABI, required exports, optional
  cleanup, mount/remount, multiple instances, event bridge, DOM patch bridge,
  browser-unsafe imports, diagnostics, size reporting, asset manifest reporting,
  and `wasm_exec.js` version.
- [ ] Add WASM tests for compile success, missing export, bad export signature,
  unsafe imports, mount, event, patch, emit, cleanup, remount after fragment,
  and remount after SPA navigation.

## CSS, Assets, Packaging, Runtime, And Contracts

- [ ] Keep Tailwind optional, outside compiler/runtime core, and never
  downloaded during builds.
- [ ] Add tests proving no Tailwind download and clear missing Tailwind
  diagnostics.
- [ ] Document Tailwind installation through user-owned toolchains and
  `tailwind.Options.Command`.
- [ ] Document CSS processor API, page-aware processors, scoped component CSS,
  component `css`, component `style {}`, layout `style {}`, component
  `asset`, non-CSS assets, image/font/icon assets, asset manifest helpers,
  content hashing, immutable cache, CSS ordering, duplicate CSS warnings,
  unused CSS warnings, missing asset diagnostics, asset graph command, and
  `gowdk inspect assets`.
- [ ] Document generated app directory layout, binary layout, embedded output,
  module selection, target selection, split frontend/backend builds,
  backend-only builds, and deploy WASM versus browser island WASM.
- [ ] Add generated output ownership, file cleanup, stale cleanup,
  deterministic output, unchanged file preservation, binary size, generated
  source size, asset size, selected module, and embedded asset reports.
- [ ] Add one-binary, split binary, backend-only, and WASM artifact smoke tests.
- [ ] Keep generated apps `net/http` first.
- [ ] Document middleware registration, graceful shutdown, health/readiness,
  `/_gowdk/health`, metrics collectors, request counters, request IDs,
  structured logging hooks, future OpenTelemetry hooks, route logging, panic
  logging, static asset serving, 404/500 handling, compression, optional ETags,
  cache-control helpers, reverse proxies, trusted proxy/header policy, Caddy,
  Nginx, Docker, systemd, environment variables, secrets, and binary rollback.
- [ ] Document contract model, command/query/event/job signatures, one command
  owner, backend-owned domain events, presentation events as untrusted UI
  notifications, idempotency, retry, backoff, dead-letter, replay, runtime role
  filtering, and contract CLI output.
- [ ] Add worker binary generation and cron binary generation when the runtime
  role contract is ready.
- [ ] Add examples for signup email jobs, checkout commands, domain events,
  admin notifications, realtime dashboard updates, and background sync.

## CLI, Dev Server, LSP, Docs, And Examples

- [x] Add `gowdk doctor`.
- [ ] Add `gowdk explain <diagnostic-code>`.
- [ ] Add `gowdk inspect` targets. `ir`, `tree`, `endpoint-graph`, and
  `go-bindings` are implemented; `assets`, `generated`, and `deps` remain
  planned.
- [x] Add `gowdk generate stubs`.
- [ ] Add `gowdk clean`, `gowdk env`, `gowdk version --json`, and
  `gowdk benchmark`.
- [ ] Improve JSON output for `check`, `routes`, `manifest`, and `sitemap`.
- [ ] Add build timing, binary size, generated file, stale cleanup, strict mode,
  stub mode, debug mode, and machine-readable build report schema docs.
- [ ] Add browser error overlay to `gowdk dev`.
- [ ] Show compiler errors, generated Go build errors, and dev-only runtime
  panics in the browser.
- [ ] Keep last successful build clearly visible.
- [ ] Log restart reason, changed files, rebuild timing, generated files
  changed, and generated binary rebuilds.
- [ ] Document backend proxy mode, `--app`, `preview`, and `--hot`.
- [ ] Add dev tests for no-op rebuilds, component changes, layout changes, CSS
  changes, config changes, backend process restarts, failed rebuild recovery,
  and generated app dev flow.
- [ ] Add LSP exact source-range diagnostics once spans are complete.
- [ ] Add go-to-definition for components, layouts, Go handlers, Go build
  functions, CSS inputs, and assets.
- [ ] Add hover docs, completions, quick fixes, tree views, graph views, build
  report viewer, generated output viewer, workspace health view, and extension
  compatibility docs.
- [ ] Add native docs for building static sites, full GOWDK apps, Go package
  interop, forms/actions, typed APIs, fragments, SSR pages, hybrid pages,
  guarded routes, components, WASM islands, Tailwind, one binary, deployment,
  generated Go, security, known gaps, when not to use GOWDK, troubleshooting,
  cookbook, language reference, CLI reference, config reference, runtime
  reference, addon reference, dependency policy, release process, and testing
  strategy.
- [ ] Do not add migration guides.
- [ ] Do not add "versus framework X" docs as core positioning.
- [ ] Keep capability examples and add larger native examples for static sites,
  build data, layouts, actions, session guards, dashboards, APIs, fragments,
  database usage, components, WASM islands, Tailwind, one-binary deploys,
  Docker, systemd, Caddy, contracts workers, SSE, and WebSocket.
- [ ] Add one flagship full-stack native GOWDK example with home page, login,
  cookie session, protected dashboard, SSR load, action submit, API route,
  fragment update, CSRF, guard, database package in normal Go, one-binary
  deploy, tests, and README.
- [ ] Require every example to include purpose, commands, expected output,
  feature status, what GOWDK owns, what Go owns, tests or smoke checks,
  generated artifact paths, and known limitations.

## Testing, CI, Operations, Performance, Playground, And Addons

- [ ] Keep `scripts/test-go-modules.sh`, CLI build tests, and VS Code tests.
- [ ] Add parser, route, form decoder, and URL escaping fuzzing.
- [ ] Add generated Go, HTML, CSS, manifest, sitemap, route report, and build
  report schema tests.
- [ ] Add action, API, fragment, SSR, hybrid, guard, CSRF, generated binary,
  generated WASM, browser client runtime, fragment, SPA navigation, and WASM
  island integration tests.
- [ ] Add performance, memory, binary size, generated output determinism, docs
  command, examples command, release checklist, and regression tests.
- [ ] Keep baseline CI fast and split jobs for Go unit tests, compiler tests,
  runtime tests, appgen tests, CLI tests, examples smoke, docs checks, VS Code
  tests, security scan, and dependency/license scan.
- [ ] Add OS matrix for Linux, macOS, and Windows.
- [ ] Add architecture matrix where useful for amd64 and arm64.
- [ ] Cache Go and Node dependencies properly.
- [ ] Add docs link check, Markdown lint, generated output determinism check,
  release dry run, release artifact smoke workflow, nightly extended examples,
  nightly fuzz/benchmark where practical, and branch protection once stable.
- [ ] Expand operations docs for static-only deploy, one-binary deploy, split
  frontend/backend deploy, backend-only deploy, Docker, systemd, Caddy, Nginx,
  environment variables, secrets, CSRF secrets, logs, metrics, health,
  readiness, graceful shutdown, cache/CDN, binary rollback, artifact layout,
  backup/restore as user responsibility, incident response as user
  responsibility, dependency update policy, and observability TODOs.
- [ ] Add performance benchmarks for cold build, incremental build, dev rebuild,
  generated binary startup, static route latency, SSR latency, action latency,
  API latency, fragment latency, memory, binary size, generated JS size,
  generated CSS size, and WASM size.
- [ ] Add compiler phase timing for discovery, parse, analyze, IR, validate,
  generate, write, and `go build`.
- [ ] Add build timing to build reports and `gowdk benchmark`.
- [ ] Update website install docs to match release assets and sync website docs
  from the repo automatically.
- [ ] Add website current release badge, experimental warning, what works today,
  known gaps, cookbook, examples index, runnable snippets, generated output
  preview, route manifest preview, build report preview, and website link
  checker.
- [ ] Document addon lifecycle, registration, config, compiler hooks, runtime
  hooks, generated file ownership, version compatibility, security
  restrictions, CSS processor addon, Tailwind addon, rate-limit addon, embed
  addon, SSR addon, partial addon, contracts addon, addon test harness, example
  addon, incompatible/missing addon diagnostics, version handshake, and addon
  docs examples.

## Native Learning Path

- [ ] Lesson 1: install GOWDK.
- [ ] Lesson 2: create a page.
- [ ] Lesson 3: add build-time Go data.
- [ ] Lesson 4: add a component.
- [ ] Lesson 5: add CSS/assets.
- [ ] Lesson 6: add an action.
- [ ] Lesson 7: add validation.
- [ ] Lesson 8: add CSRF.
- [ ] Lesson 9: add an API.
- [ ] Lesson 10: add a fragment.
- [ ] Lesson 11: add SSR.
- [ ] Lesson 12: add a guard.
- [ ] Lesson 13: use a database from Go.
- [ ] Lesson 14: build one binary.
- [ ] Lesson 15: deploy behind Caddy.
- [ ] Lesson 16: inspect generated Go.
- [ ] Lesson 17: troubleshoot diagnostics.
- [ ] Lesson 18: add tests.
- [ ] Lesson 19: add optional Tailwind.
- [ ] Lesson 20: add optional WASM island.

## Priority Queue

Start with these in order:

- [x] Verify release metadata shows experimental/pre-release correctly.
- [x] Open public issue backlog.
- [x] Add `0.x Hardening` project board.
- [x] Update website install docs for release binaries.
- [x] Sync root `SECURITY.md` with deeper security baseline.
- [x] Keep dependency policy current and add missing enforcement.
- [ ] Add license/dependency scan to CI.
- [x] Add release note template.
- [ ] Add `gowdk doctor`.
- [ ] Add `gowdk explain`.
- [x] Add `gowdk inspect go-bindings`.
- [x] Add `gowdk generate stubs`.
- [ ] Stabilize `gowdk check --json`.
- [ ] Add diagnostic codes.
- [ ] Add exact source spans where missing.
- [ ] Finish downstream `gwdkir` migration.
- [ ] Add generated Go golden tests.
- [ ] Add endpoint IR report.
- [ ] Add strict production-shaped binding mode.
- [ ] Add generated app HTTP smoke tests.
- [ ] Add CSRF secret docs.
- [ ] Add safe redirect tests.
- [ ] Add guard contract docs and tests.
- [ ] Add request timeout, header limit, and body limit support.
- [ ] Build flagship full-stack native GOWDK example.
- [ ] Build deployment-shaped example.
- [ ] Add native cookbook.
- [ ] Add browser dev error overlay.
- [ ] Add VS Code quick fix for creating a missing Go handler.
- [ ] Add performance/build timing report.

## Do Not Add For Now

- [x] Do not add migration guides.
- [x] Do not add "GOWDK vs X" docs as core positioning.
- [x] Do not make SSR default.
- [x] Do not make full-page hydration default.
- [x] Do not make browser JavaScript the app contract.
- [x] Do not generate user domain logic.
- [x] Do not generate auth or business validation logic.
- [x] Do not auto-discover endpoints by function name.
- [x] Do not scan Chi/Gin/Echo/Fiber route registrations as route truth.
- [x] Do not require npm.
- [x] Do not require Tailwind.
- [x] Do not require Redis.
- [x] Do not require NATS.
- [x] Do not require Chi, Gin, Echo, or Fiber.
- [x] Do not download optional tools during builds.
- [x] Do not hide partial features behind confident wording.
- [ ] Do not add more syntax without diagnostics, tests, docs, and examples.

## Direction

```text
Make the current 0.x surface trustworthy.
Make the compiler spine boring.
Make Go interop excellent.
Make generated adapters strict and readable.
Make security warnings precise.
Make examples native to GOWDK.
Do not expand into comparison or migration positioning.
```

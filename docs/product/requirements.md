# Product Requirements

## Current Status

The product direction is Go-first full web app compilation: GOWDK is the
`.gwdk` component/page compiler, and GOWDK Runtime is the app/runtime layer.
Build-time page output is the default, actions/APIs/fragments are first-class
request-time endpoint behavior, and `@render ssr` is an integrated non-default
request-time page-rendering lane selected per page.

Current user-facing documentation now separates implemented behavior from
planned behavior across the README, CLI/config/routing/deployment references,
language references, compiler docs, and examples.

## Status Legend

- Implemented: available in the current codebase, documented, and covered by
  tests or an explicit verification command.
- Partial: available for a narrower slice than the full requirement, with
  remaining limits called out in the notes.
- Experimental: available to try, but the public contract may still change.
- Planned: accepted product direction with no stable implementation yet.
- Intentionally out of scope: rejected for the current product direction.

## Requirements

| ID | Requirement | Priority | Status | Notes |
| --- | --- | --- | --- | --- |
| PRD-001 | Compile portable package-peer `.gwdk` files that declare `package`, `@page`, `@route`, `@layout`, and optional `@render`. | High | Partial | Discovery, package parsing, metadata parsing, parser syntax validation, default build discovery, route shape/conflict validation, required page-view validation, explicit component-file build input, typed GOWDK AST, AST analyzer, versioned compiler IR, endpoint comment discovery, and endpoint conflict diagnostics are implemented; full downstream migration to the IR remains planned. |
| PRD-002 | Default render mode must be `spa`. | High | Implemented | Root `RenderConfig.DefaultMode()` defaults to `gowdk.SPA`. |
| PRD-003 | Support render modes `spa`, `action`, `hybrid`, and `ssr`. | High | Implemented | Root `RenderMode` constants exist. |
| PRD-004 | Reject `@render ssr` unless the SSR feature is enabled in config or CLI options. | High | Implemented | `internal/compiler.ValidatePage` emits `missing_ssr_addon`; the diagnostic name is historical. |
| PRD-005 | Require `paths {}` for dynamic SPA routes. | High | Implemented | Dynamic SPA routes without paths are rejected; action endpoints on those pages inherit generated concrete page paths. Malformed routes, duplicate route params, duplicate page route patterns, and route-method conflicts are rejected; the first literal string `paths {}` subset can prerender dynamic SPA routes. |
| PRD-006 | Keep typed actions available without SSR. | High | Partial | SPA pages with exported `act Name POST "/path"` endpoint declarations validate without SSR. Generated apps can serve POST action handlers with generated typed decoders, unexpected-field rejection, generated validation for direct literal `required`, `minlength`, `maxlength`, and Go-regexp-compatible `pattern` form controls, generated validation fragments for partial requests, partial fragment responses, same-package action handlers using no-input, typed value, typed pointer, or `form.Values` signatures returning `response.Response`, and opt-in generated CSRF token injection/validation through `Build.CSRF.Enabled`. Direct file inputs and multipart generated action forms are rejected because uploads belong in user-owned API/server handlers. User-defined domain validation patterns remain in normal Go handlers. |
| PRD-007 | Treat `load {}` as request-time behavior requiring SSR or hybrid rendering. | High | Implemented | SPA pages with `load` are rejected. |
| PRD-008 | Keep runtime render core reusable across build-time pages, backend fragments, and request-time pages. | High | Implemented | `runtime/render` exists independently from `addons/ssr`; SSR is integrated through compiler/runtime hooks and enabled by feature registration. |
| PRD-009 | Generate build-output/prerender output for v0.1. | High | Partial | `gowdk build --out` emits app-shell HTML, `gowdk-routes.json`, and `gowdk-assets.json` for simple build-time pages, the first literal dynamic path subset, literal build data, imported and same-package no-argument Go build data functions, scalar build fields, earlier-field references, string concatenation, numeric arithmetic, boolean logic, comparisons, and explicit or discovered components; generated handlers, arbitrary build-time statements beyond expression records, and full component semantics remain planned. |
| PRD-010 | Provide CSS/plugin extension points without adding Tailwind to the compiler core or runtime core. | High | Partial | `FeatureCSS`, `addons/css`, configured stylesheet links, compile-time CSS processors, discovered CSS inputs, extracted literal classes, `@css` page selection, generated page CSS output, CSS asset manifest entries, page-aware processor stylesheet selections, component CSS AST/IR scope and hash metadata, emitted scoped component CSS, scoped selector/keyframe rewriting, AST-only config loading for built-in addons, executable config loading for external importable addons, an experimental Tailwind v4 standalone-CLI wrapper, and generated CSS minification/content-hashed emitted filenames are implemented; richer CSS plugin capabilities and non-CSS component asset output remain planned. |
| PRD-011 | Support embedded assets and one-binary serving. | High | Partial | `addons/embed` and `runtime/asset` boundaries exist; `gowdk serve` can serve generated build output locally; `gowdk build --app` can generate an embedded app, `--bin` can compile it into one binary, and `--wasm` can compile a Go `js/wasm` artifact for SPA pages, feature-bound action/API handlers, action redirects, action fragments, standalone fragments, and concrete or dynamic SSR pages with declared `load {}` identifier or dotted paths. Broader hybrid request-time behavior remains planned. |
| PRD-012 | Support server fragments for partial updates without full-page SSR. | Medium | Partial | `addons/partial`, `runtime/response.FragmentFor`, generated client runtime emission, generated action fragment responses for partial POSTs, standalone fragment routes, generated required-field validation fragments for partial POSTs, generated CSRF validation when enabled, and first-slice generated JavaScript islands for local component state are implemented. Richer fragment rendering and broader local client-side reactivity remain planned. |
| PRD-013 | Complete request-time page rendering with `load {}`, guards, layouts, and error handling. | Medium | Partial | `addons/ssr` registers the SSR feature and provides load context, guard execution, route registration, request-aware layout composition, safe local redirect errors, default error-handler contracts, and declared load path resolution. Generated embedded apps can serve concrete and dynamic `@render ssr` pages rendered from `view {}` and literal or imported `build {}` data, generated SSR/action/API routes expose `RegisterGuards`, run declared guards with fail-closed missing-guard behavior, and have generated-binary coverage for registered guard success paths, `load { => { field, user.name } }` execution calls same-package Go load functions through `ssr.LoadContext`, optional generated `404.html`/`500.html` pages are used by runtime app error responses, SSR routes can declare `@error "/errors/page.html"` for route-local generated load/render failure and route panic pages, action/API declarations can declare endpoint-local `@error` pages for generated panic boundaries, and generated SSR/action/API lanes have no-store panic boundaries. |
| PRD-014 | Add optional WASM islands after the core compiler and action flow are stable. | Low | Partial | Explicit `g:island="wasm"` component calls emit WASM and loader assets under `assets/gowdk/islands/`. Declared `@wasm` browser-side Go packages are compiled with `GOOS=js GOARCH=wasm`, checked for browser-unsafe imports, ship the Go `wasm_exec.js` runtime asset, instantiate through Go runtime imports when needed, and validate required GOWDK ABI exports. Browser-runtime integration coverage exercises the generated host loader mount, event, patch, emit, and cleanup contract; fuller user-code runtime validation remains planned. |
| PRD-015 | Provide language tools for `.gwdk` token inspection, formatting, validation, manifest output, and LSP editor integration. | High | Implemented | `internal/lang`, `internal/lsp`, and CLI commands exist. |
| PRD-016 | Keep hybrid pages SPA by default and require explicit request-time capabilities. | High | Implemented | Bare `@render hybrid` pages are emitted as build-time SPA output; hybrid pages require the SSR addon only when explicit request-time behavior such as `load {}` is declared. |
| PRD-017 | Define cache and revalidation behavior for static files, SPA routes, backend endpoints, partial responses, SSR routes, and hybrid pages. | Medium | Partial | Generated binaries apply asset-manifest cache policies for generated assets, default SPA HTML to `no-cache`, default request-time handlers to `no-store`, and apply explicit page `@cache` policies to successful generated static SPA HTML and SSR HTML responses. `@revalidate` accepts positive second or duration values, requires `@cache`, and compiles into a `stale-while-revalidate=<seconds>` Cache-Control directive for generated static SPA HTML and SSR HTML. Richer hybrid cache policy syntax remains planned. |
| PRD-018 | Escape generated HTML by default and require any raw HTML escape hatch to be explicit. | High | Partial | Current SPA rendering escapes text and attributes. |
| PRD-019 | Provide optional rate limiting for request-time handlers without making it core. | Medium | Implemented | `FeatureRateLimit` and `addons/ratelimit` expose HTTP middleware, fixed-window decisions, an in-memory store, and a Redis-backed store adapter. Generated action, API, fragment, SSR, and split-backend proxy handlers expose `RegisterRateLimiter(*ratelimit.Limiter)` when the addon is enabled and call the registered limiter before guards and user logic. Docs include an in-memory registration example and a concrete go-redis adapter. |
| PRD-020 | Allow generated apps and binaries to package selected configured modules. | High | Implemented | `Build.Targets` SPAally declares module sets, output dirs, generated app dirs, and binaries. `gowdk build` runs all configured targets, `--target` selects named targets, and ad hoc repeated or comma-separated `--module` flags remain supported. |
| PRD-021 | Provide a dependency-free fast local development loop. | High | Partial | `gowdk dev` polls discovered inputs, compares content hashes, rebuilds only on real input changes, can incrementally render changed page sources for plain build output, serves the generated output, and live reloads browsers after successful rebuilds. SPA/app generation skips identical file writes. |
| PRD-022 | Allow generated app output to compile to a WASM deploy artifact. | Medium | Partial | `gowdk build --wasm <file>` and `Build.Targets[].WASM` compile the generated app with `GOOS=js GOARCH=wasm`. This remains separate from explicit browser island assets emitted by `g:island="wasm"`. |
| PRD-023 | Keep current documentation aligned with implemented CLI, config, compiler, language, routing, deployment, and examples. | High | Implemented | `README.md`, `docs/getting-started.md`, reference docs, language docs, compiler docs, and `examples/README.md` describe current support and call out planned behavior. |
| PRD-024 | Require project config before compiling or validating `.gwdk` code. | High | Implemented | `check`, `manifest`, `sitemap`, `routes`, `build`, and `dev` require `gowdk.config.go` in the current directory or an explicit `--config <file>`, even when explicit `.gwdk` file paths are provided. |
| PRD-025 | Keep framework integrations optional and outside compiler/runtime core. | Medium | Implemented | Generated apps expose standard `net/http` handlers and framework-neutral code by default. Optional `runtime/adapters/echo`, `runtime/adapters/gin`, and `runtime/adapters/fiber` packages wrap the same generated `http.Handler`; docs cover Echo v5, Gin, Fiber, and Fiber adaptor caveats. |

## P0/P1/P2 Decision Backlog

This backlog records product decisions without treating deferred work as
implemented.

| Area | Requirement Direction | Status |
| --- | --- | --- |
| Markup language | Expand `view {}` only through GOWDK-owned AST nodes and directives; defer raw HTML, async placeholders, transitions, DOM/document targets, and DOM actions until separate contracts exist. | Planned |
| Snippets and slots | Keep slots as the stable reusable markup primitive; defer first-class snippet/render values. | Planned |
| Component props | Keep imported Go structs as the primary typed prop path; add non-string literal props and defaults before considering rest/spread, renaming, recursion, dynamic components, or bindable child state. | Planned |
| Client reactivity | Keep bounded compiler-owned `client {}`; generated JS must not own routing, auth, business rules, database access, server validation, action behavior, global app state, or page loading policy. | Planned |
| Shared state | Keep stores page/island scoped until cross-package or app-global stores have explicit ownership, serialization, subscription, and teardown contracts. | Planned |
| Load/data lifecycle | Keep `build {}` build-time, `load {}` request-time, and actions/APIs/fragments as endpoint lanes; defer universal/browser-owned load policy. | Planned |
| Hybrid | Keep bare hybrid as SPA output and `load {}` as the explicit request-time branch; defer streaming, data refresh, and non-HTTP revalidation. | Partial |
| Hooks | Compose app-wide hooks as `net/http` middleware plus explicit generated registration points; defer route rewriting and fetch interception. | Planned |
| Errors | Keep `@error` for route-local SSR and action/API boundaries; define expected error types and layout boundaries later. | Partial |
| Dev server | Keep dependency-free live reload as baseline; add browser error overlay before component-aware HMR. | Planned |
| Routing | Add rest params and trailing-slash policy first while keeping explicit route declarations; defer optional params, route groups, and same-path page/API negotiation. | Planned |
| Typed generated APIs | Generate typed route-param accessors first; defer typed load/action data accessors until result contracts are stable. | Planned |
| Forms | Keep progressive-enhancement-first form behavior; full POST and enhanced POST share action result semantics; domain validation stays in user Go. | Planned |
| APIs | Broaden APIs through public request/response helpers and typed body/query helpers, not framework-specific adapters. | Planned |
| Contract runtime | Add typed Go queries, commands, backend-owned domain/integration events, presentation events, and jobs after endpoint/adapter IR is stable. Frontend UI events trigger commands or queries, commands have one owner, domain events are emitted after backend state changes succeed, local in-process dispatch is default, and broker/outbox/worker roles are optional. First runtime registry, runtime role filtering helpers, event-envelope capture/replay, stable observation names and labels for logs/metrics/traces, dependency-free outbox, broker, presentation-fanout, and event-source interfaces, event worker loop with ack/nack and context cancellation, dependency-free file outbox adapter with retry metadata and opt-in dead-letter storage, Go AST scanner, scan-local package inspection cache, local-package and imported-handler `go/types` diagnostics, local exported struct/function contract diagnostics, duplicate command-owner scan diagnostics, first browser-UI and vague event-name diagnostics, contract/list/graph/trace CLI, form-local `g:command` metadata with literal form method/action, element-local `g:query` metadata with page-route source metadata, `g:event` rejection, IR command/query references with exact source locations, command/query reference binding status, appgen adapter IR exposure metadata, command method/path adapter metadata, query page-route adapter metadata, generated web command/query adapters, routes-report contract endpoint metadata, missing/invalid/non-web-role contract-reference diagnostics, enforced Go contract scan diagnostics in check/build, and build-report contract-reference events with role metadata are implemented; imported contract type binding, all diagnostic spans, split-binary worker/cron wiring, database-backed outbox implementations, concrete broker adapters, retry backoff policies, and concrete realtime adapters remain planned. | Partial |
| Cache | Keep `@cache` and `@revalidate` as HTTP cache policy until load/action invalidation contracts exist. | Partial |
| Guards | Extend guards with safe local redirects and response helpers before richer request-local state. | Planned |
| Component CSS | Make component CSS explicit, compiler-scoped, and documented; Tailwind and processors remain optional. | Partial |
| Accessibility | Add accessibility diagnostics as compiler warnings with stable codes and spans. | Planned |
| Diagnostics and LSP | Expand diagnostic catalogue before broad parser recovery; prioritize hover, semantic tokens, go-to-definition, and route/type navigation. | Planned |
| Testing and scaffolding | Add optional Go handler tests, generated app smoke tests, template/addon selection, and editable generated examples. | Partial |
| Deployment and operations | Prefer docs and optional generators for static hosts, Docker, systemd, reverse proxies, CDN policy, health checks, metrics, logging, binary deploy, rollback, and CSRF secret rotation. | Planned |
| Full-page hydration | Keep full-page hydration out of the repository core; use static pages, progressive enhancement, server fragments, and explicit islands. | Intentionally out of scope |
| Island ergonomics | Improve compiler-owned island syntax, lifecycle cleanup, focus helpers, local batching, and diagnostics without exposing arbitrary JavaScript as the app contract. | Planned |
| Client builtins | Add deterministic formatting, collection, async-safe UI, focus, and selection helpers only with generated-output tests. | Planned |
| WASM islands | Keep browser-side Go explicit and separate from backend handlers; improve ABI docs, validation, and examples. | Planned |
| PWA/offline | Keep service workers and PWA behavior optional and documentation-first; no hidden offline/cache defaults. | Planned |
| Images | Document image optimization patterns first; optional integrations may emit assets or metadata without turning core into an image pipeline. | Planned |
| Addon discovery | Start with repository/website docs or registry metadata; add CLI discovery only after addon versioning, trust, and compatibility rules exist. | Planned |
| Playground | Own playground onboarding in website/docs first, with optional CLI export later; hosted execution must remain sandboxed and optional. | Planned |
| Performance profiling | Document measurement for build time, output size, generated JS size, SSR/action latency, binary size, and cache behavior before adding automation. | Partial |
| Migration guides | Publish docs-first guides for Go templates, htmx-style apps, JavaScript-framework concepts, and static Go sites while preserving GOWDK-native terminology. | Partial |

## Non-Functional Requirements

- Performance: SPA pages should be generated at build time and served directly from disk or embedded assets.
- Reliability: compiler diagnostics must fail fast for invalid render modes, SSR used without the feature enabled, and dynamic SPA routes without paths.
- Security: actions need CSRF, typed form decoding, validation, and safe redirects before production use.
- Privacy: generated logs and diagnostics must not expose secrets or sensitive form data.
- Packaging: generated binaries and WASM artifacts must embed only the selected module output for that build.
- Developer loop: failed rebuilds must not stop the last successful served output, no-op generated writes should not retrigger dev loops, and page-local build-output edits should not force full output rendering.
- Accessibility: generated components should preserve semantic HTML and support focus restoration for partial updates.
- Localization: route and content generation should not assume one locale.
- Supportability: manifest output should include route, render mode, layouts, paths presence, and guards for debugging.
- Project shape: project-level compiler commands must fail fast when no config file is loaded.

## Out Of Scope

- Full SPA runtime as the default experience.
- Mandatory full-page SSR.
- User-written JavaScript for normal forms, actions, and partial update flows.
- WASM islands as the default component runtime.

## Open Questions

- Which downstream compiler passes should migrate from manifest compatibility
  structs to `internal/gwdkir.Program` first?
- Should hybrid pages get additional cache policy syntax beyond page-level
  `@cache` and `@revalidate`?
- Should processor-emitted CSS become selectable named `@css` inputs through a
  future page-aware processor contract?
- Should build targets eventually support per-target addon and render-mode
  overrides?
- What generated adapter shape should execute `g:command` and `g:query`
  contracts without replacing existing endpoint declarations prematurely?

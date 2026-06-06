# Product Requirements

## Current Status

The product direction is Go-first full web app compilation: a component/page
compiler plus an app/runtime kit. Build-time page output is the default,
actions/APIs/fragments are core request-time endpoint behavior, and `@render
ssr` is an integrated non-default request-time page-rendering lane selected per
page.

Current user-facing documentation now separates implemented behavior from
planned behavior across the README, CLI/config/routing/deployment references,
language references, compiler docs, examples, and the documentation checklist in
`docs/product/documentation-checklist.md`.

## Requirements

| ID | Requirement | Priority | Status | Notes |
| --- | --- | --- | --- | --- |
| PRD-001 | Compile portable package-peer `.gwdk` files that declare `package`, `@page`, `@route`, `@layout`, and optional `@render`. | High | Partial | Discovery, package parsing, metadata parsing, parser syntax validation, default build discovery, route shape/conflict validation, required page-view validation, explicit component-file build input, typed GOWDK AST, AST analyzer, versioned compiler IR, endpoint comment discovery, and endpoint conflict diagnostics are implemented; full downstream migration to the IR remains planned. |
| PRD-002 | Default render mode must be `spa`. | High | Implemented | Root `RenderConfig.DefaultMode()` defaults to `gowdk.SPA`. |
| PRD-003 | Support render modes `spa`, `action`, `hybrid`, and `ssr`. | High | Implemented | Root `RenderMode` constants exist. |
| PRD-004 | Reject `@render ssr` unless the SSR feature is enabled in config or CLI options. | High | Implemented | `internal/compiler.ValidatePage` emits `missing_ssr_addon`; the diagnostic name is historical. |
| PRD-005 | Require `paths {}` for dynamic SPA routes. | High | Implemented | Dynamic SPA routes without paths are rejected; action endpoints on those pages inherit generated concrete page paths. Malformed routes, duplicate route params, duplicate page route patterns, and route-method conflicts are rejected; the first literal string `paths {}` subset can prerender dynamic SPA routes. |
| PRD-006 | Keep typed actions available without SSR. | High | Partial | SPA pages with exported `act Name POST "/path"` endpoint declarations validate without SSR. Generated apps can serve POST action handlers with generated typed decoders, unexpected-field rejection, generated validation for direct literal `required`, `minlength`, `maxlength`, and Go-regexp-compatible `pattern` form controls, generated validation fragments for partial requests, first-slice partial fragment responses, same-package action handlers using no-input, typed value, typed pointer, or `form.Values` signatures returning `response.Response`, and opt-in generated CSRF token injection/validation through `Build.CSRF.Enabled`. Direct file inputs and multipart generated action forms are rejected because uploads belong in user-owned API/server handlers. User-defined domain validation patterns remain in normal Go handlers. |
| PRD-007 | Treat `load {}` as request-time behavior requiring SSR or hybrid rendering. | High | Implemented | SPA pages with `load` are rejected. |
| PRD-008 | Keep runtime render core reusable across build-time pages, backend fragments, and request-time pages. | High | Implemented | `runtime/render` exists independently from `addons/ssr`; SSR is integrated through compiler/runtime hooks and enabled by feature registration. |
| PRD-009 | Generate build-output/prerender output for v0.1. | High | Partial | `gowdk build --out` emits app-shell HTML, `gowdk-routes.json`, and `gowdk-assets.json` for simple build-time pages, the first literal dynamic path subset, literal build data, imported and same-package no-argument Go build data functions, scalar build fields, earlier-field references, string concatenation, numeric arithmetic, boolean logic, comparisons, and explicit or discovered components; generated handlers, arbitrary build-time statements beyond expression records, and full component semantics remain planned. |
| PRD-010 | Provide CSS/plugin extension points without adding Tailwind to initial core. | High | Partial | `FeatureCSS`, `addons/css`, configured stylesheet links, compile-time CSS processors, discovered CSS inputs, extracted literal classes, `@css` page selection, generated page CSS output, CSS asset manifest entries, page-aware processor stylesheet selections, component CSS AST/IR scope and hash metadata, AST-only config loading for built-in addons, executable config loading for external importable addons, an experimental Tailwind v4 standalone-CLI wrapper, and generated CSS minification/content-hashed emitted filenames are implemented; richer CSS plugin capabilities remain planned. |
| PRD-011 | Support embedded assets and one-binary serving. | High | Partial | `addons/embed` and `runtime/asset` boundaries exist; `gowdk serve` can serve generated build output locally; `gowdk build --app` can generate an embedded app, `--bin` can compile it into one binary, and `--wasm` can compile a Go `js/wasm` artifact for SPA pages, feature-bound action/API handlers, first-slice action redirects, first-slice action fragments, and first-slice concrete or dynamic SSR pages with declared `load {}` identifier or dotted paths. General fragment routing remains planned. |
| PRD-012 | Support server fragments for partial updates without full-page SSR. | Medium | Partial | `addons/partial`, `runtime/response.FragmentFor`, generated client runtime emission, first-slice generated action fragment responses for partial POSTs, generated required-field validation fragments for partial POSTs, generated CSRF validation when enabled, and first-slice generated JavaScript islands for local component state are implemented. Richer fragment rendering and broader local client-side reactivity remain planned. |
| PRD-013 | Complete request-time page rendering with `load {}`, guards, layouts, and error handling. | Medium | Partial | `addons/ssr` registers the SSR feature and provides load context, guard execution, route registration, request-aware layout composition, safe local redirect errors, default error-handler contracts, and declared load path resolution. Generated embedded apps can serve concrete and dynamic `@render ssr` pages rendered from `view {}` and literal or imported `build {}` data, generated SSR/action/API routes expose `RegisterGuards`, run declared guards with fail-closed missing-guard behavior, and have generated-binary coverage for registered guard success paths, `load { => { field, user.name } }` execution calls same-package Go load functions through `ssr.LoadContext`, optional generated `404.html`/`500.html` pages are used by runtime app error responses, and generated SSR/action/API lanes have no-store panic boundaries; custom error-boundary syntax remains planned. |
| PRD-014 | Add optional WASM islands after the core compiler and action flow are stable. | Low | Partial | Explicit `g:island="wasm"` component calls emit WASM and loader assets under `assets/gowdk/islands/`. Declared `@wasm` browser-side Go packages are compiled with `GOOS=js GOARCH=wasm`, checked for browser-unsafe imports, ship the Go `wasm_exec.js` runtime asset, instantiate through Go runtime imports when needed, and validate required GOWDK ABI exports. Browser-runtime integration coverage exercises the generated host loader mount, event, patch, emit, and cleanup contract; fuller user-code runtime validation remains planned. |
| PRD-015 | Provide language tools for `.gwdk` token inspection, formatting, validation, manifest output, and LSP editor integration. | High | Implemented | `internal/lang`, `internal/lsp`, and CLI commands exist. |
| PRD-016 | Keep hybrid pages SPA by default and require explicit request-time capabilities. | High | Implemented | Bare `@render hybrid` pages are emitted as build-time SPA output; hybrid pages require the SSR addon only when explicit request-time behavior such as `load {}` is declared. |
| PRD-017 | Define cache and revalidation behavior for static files, SPA routes, backend endpoints, partial responses, SSR routes, and hybrid pages. | Medium | Partial | Generated binaries apply asset-manifest cache policies for generated assets, default SPA HTML to `no-cache`, default request-time handlers to `no-store`, and apply explicit page `@cache` policies to successful generated static SPA HTML and SSR HTML responses. `@revalidate` accepts positive second or duration values, requires `@cache`, and compiles into a `stale-while-revalidate=<seconds>` Cache-Control directive for generated static SPA HTML and SSR HTML. Richer hybrid cache policy syntax remains planned. |
| PRD-018 | Escape generated HTML by default and require any raw HTML escape hatch to be explicit. | High | Partial | Current SPA rendering escapes text and attributes. |
| PRD-019 | Provide optional rate limiting for request-time handlers without making it core. | Medium | Implemented | `FeatureRateLimit` and `addons/ratelimit` expose HTTP middleware, fixed-window decisions, an in-memory store, and a Redis-backed store adapter. Generated action, API, SSR, and split-backend proxy handlers expose `RegisterRateLimiter(*ratelimit.Limiter)` when the addon is enabled and call the registered limiter before guards and user logic. Docs include an in-memory registration example and a concrete go-redis adapter. |
| PRD-020 | Allow generated apps and binaries to package selected configured modules. | High | Implemented | `Build.Targets` SPAally declares module sets, output dirs, generated app dirs, and binaries. `gowdk build` runs all configured targets, `--target` selects named targets, and ad hoc repeated or comma-separated `--module` flags remain supported. |
| PRD-021 | Provide a dependency-free fast local development loop. | High | Partial | `gowdk dev` polls discovered inputs, compares content hashes, rebuilds only on real input changes, can incrementally render changed page sources for plain build output, serves the generated output, and live reloads browsers after successful rebuilds. SPA/app generation skips identical file writes. |
| PRD-022 | Allow generated app output to compile to a WASM deploy artifact. | Medium | Partial | `gowdk build --wasm <file>` and `Build.Targets[].WASM` compile the generated app with `GOOS=js GOARCH=wasm`. This remains separate from explicit browser island assets emitted by `g:island="wasm"`. |
| PRD-023 | Keep current documentation aligned with implemented CLI, config, compiler, language, routing, deployment, and examples. | High | Implemented | `README.md`, `docs/getting-started.md`, reference docs, language docs, compiler docs, `examples/README.md`, and `docs/product/documentation-checklist.md` describe current support and call out planned behavior. |
| PRD-024 | Require project config before compiling or validating `.gwdk` code. | High | Implemented | `check`, `manifest`, `sitemap`, `routes`, `build`, and `dev` require `gowdk.config.go` in the current directory or an explicit `--config <file>`, even when explicit `.gwdk` file paths are provided. |
| PRD-025 | Keep framework integrations optional and outside compiler/runtime core. | Medium | Implemented | Generated apps expose standard `net/http` handlers and framework-neutral code by default. Optional `runtime/adapters/echo`, `runtime/adapters/gin`, and `runtime/adapters/fiber` packages wrap the same generated `http.Handler`; docs cover Echo v5, Gin, Fiber, and Fiber adaptor caveats. |

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

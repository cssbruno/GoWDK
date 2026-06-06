# GOWDK Missing Checklist

This checklist tracks what is still missing before GOWDK can be presented as a
Go-first full web app platform.

Current product shape:

```text
GOWDK component/page compiler
        +
GOWDK app/runtime kit
        =
Go-first full web app
```

The compiler owns `.gwdk` package-peer files, pages, layouts, components,
build-time output, CSS, islands, manifests, diagnostics, and generated adapter
source. The app/runtime kit owns routing, form decoding, response envelopes,
actions, APIs, CSRF, partial fragments, SSR contracts, embedded assets, and
one-binary serving.

Do not use this file to preserve old plan names. Removed first-slice planning
docs have been folded into `.llm/plans/gowdk-world-roadmap.md`.

Active planning sources:

- `.llm/plans/gowdk-world-roadmap.md`
- `.llm/features/deep-go-package-integration.md`
- `.llm/plans/deep-go-package-integration.md`
- `.llm/features/go-native-adapter-boundary.md`
- `.llm/plans/go-native-adapter-boundary.md`
- `.llm/features/golangish-reactive-islands.md`
- `docs/engineering/decisions/0006-gowdk-compiler-and-kit-boundary.md`

Compiler lanes:

```text
.gwdk file
  -> GOWDK parser
  -> GOWDK AST
  -> GOWDK analyzer
  -> generated normal Go code
  -> go/format
  -> go build
```

```text
.go files
  -> standard go/parser
  -> standard go/ast
  -> standard go/types
  -> validate exported handlers/types
```

## Current Baseline

- [x] Default render mode is `spa`.
- [x] `@render ssr` is non-default request-time page rendering and is rejected
      unless the SSR feature is enabled.
- [x] `load {}` is rejected on SPA pages.
- [x] Dynamic SPA routes require `paths {}`; action endpoints inherit generated
      concrete page paths.
- [x] First literal dynamic `paths {}` subset can prerender SPA output.
- [x] Project compiler commands require `gowdk.config.go` or `--config`.
- [x] Build output emits SPA HTML, route manifest, asset manifest, and build report.
- [x] Generated app source, binary, and generated app WASM deploy artifacts exist.
- [x] Configured build targets select modules, output dirs, app dirs, binaries, and WASM artifacts.
- [x] Dev loop does content-hash rebuilds, live reload, no-op write skipping, and page-local incremental SPA output.
- [x] CSS addon boundary, stylesheet links, discovered CSS inputs, page CSS output, `@css`, and Tailwind wrapper exist.
- [x] Component discovery, imported props/state contracts, slots, generated JS islands, and explicit WASM island assets exist.
- [x] First-slice client language supports local variables, helpers, computed values, events, refs, lists, and selected built-ins.
- [x] First-slice action/API backend binding can call same-directory Go handlers with narrow signatures.
- [x] Missing or unsupported first-slice action/API bindings generate clear
      `501` responses in development or explicit stub mode.
- [x] Generated app Go source is formatted before write.
- [x] CLI tooling includes `tokens`, `fmt`, `check`, `manifest`, `sitemap`, `routes`, and `lsp`.

## GOWDK Compiler

- [x] Define the GOWDK AST for package declarations, annotations, routes,
      imports, stores, blocks, component contracts, and source spans.
- [x] Add a GOWDK analyzer that lowers the GOWDK AST into normalized route,
      component, package, type, asset, and generated adapter metadata.
- [x] Define the GOWDK source import model:
  - [x] Go `import` inside `.gwdk` imports normal Go packages only.
  - [x] Same-package `.gwdk` and `.go` files are peers and need no import.
  - [x] Page-level cross-package component calls use explicit GOWDK
        `use alias "package"` declarations and qualified tags such as
        `<ui.Hero />`.
  - [x] Imported components resolve their own same-package child components by
        bare name without making those names page-global.
  - [x] Component-scoped cross-package `use` declarations need explicit
        semantics and renderer/build-asset support.
  - [x] Cross-package layouts use explicit GOWDK `use alias "package"`
        declarations and qualified `@layout alias.id` references.
  - [x] Cross-package stores and assets need explicit GOWDK import/use
        semantics, not accidental global lookup.
  - [x] Decide syntax for qualified layout references before broad
        multi-package layout reuse.
- [x] Require `package <name>` as the first non-comment declaration in every real page, layout, and component `.gwdk` file.
- [x] Store package name and package source span in manifest records.
- [x] Validate `.gwdk` package names against sibling `.go` files.
- [x] Fail check/build on Go package parse/package errors with `go_package_error`.
- [x] Fail check/build on Go package type-check errors with `go_package_error`.
- [x] Replace old action/API blocks with exact exported endpoint declarations:

```gwdk
package auth

act Login POST "/"
api Session GET "/api/session"
```

- [x] Reject old action/API block syntax with direct migration diagnostics.
- [x] Reject non-exported handler names in `act`, `api`, and `g:post`.
- [x] Reject non-POST action methods.
- [x] Keep route declarations in `.gwdk`; keep behavior in normal Go.
- [x] Add package, handler symbol, endpoint path, method, binding status, and binding message to manifest, routes output, and build report metadata.
- [x] Add a stable internal IR for templates, client behavior, routes, assets, and generated output.
  - [x] Route/sitemap metadata reports consume `internal/gwdkir.Program`.
  - [x] Build output planning, memory output planning, incremental SPA output,
        and SSR artifact planning consume `internal/gwdkir.Program`.
  - [x] Generated app planning consumes `internal/gwdkir.Program`.
- [x] Expand source spans and suggestions across parser, route, view, component, client, package, endpoint, and build errors.
- [x] Support same-package build functions or document why explicit imports remain required.
- [x] Support broader build-time data beyond the first literal/imported no-argument subset.
  - [x] Merge multiple literal `=> { ... }` declarations in one `build {}`
        block with duplicate-field diagnostics.
  - [x] Support same-package no-argument build data functions with standard Go
        package import resolution.
  - [x] Support scalar literal build values and references to earlier build
        fields or route params.
- [x] Attach generated endpoint metadata to bound action/API handler contexts
      instead of only embedding it in route/build output.
- [x] Attach generated page route metadata and dynamic params to generated SSR
      handler contexts.
- [x] Reuse the generated page route context for future request-time
      `load {}` and guard user logic.

## App Runtime Kit

- [x] Add shared backend routing primitives in `runtime/app`:
  - [x] `BackendHandler`
  - [x] `BackendRouter`
  - [x] `NewBackendRouter`
  - [x] action and API endpoint registration
  - [x] method/path dispatch with normalized paths
- [x] Add runtime adapter helpers:
  - [x] `Action0`
  - [x] `ActionForm[T]`
  - [x] `ActionFormPtr[T]`
  - [x] `ActionValues`
  - [x] `APIHandler`
  - [x] `NotImplemented`
- [x] Update generated apps to use one backend hook instead of separate action/API hook shapes.
- [x] Preserve no-store defaults for request-time action/API/fragment responses.
- [x] Keep request body size limits in generated action adapters.
- [x] Wire generated guards for SSR/action/API paths.
- [x] Add runtime metrics only after handler contracts settle.

## SPA Navigation And Generated JS Guardrails

- [x] Define SPA as static-first with optional client navigation, not a
      client-owned application shell.
- [x] Ensure every generated SPA route remains a real URL that works on direct
      open and browser refresh.
- [x] Allow generated JS to enhance navigation by intercepting internal links,
      fetching real built page shells, replacing the current document, and
      preserving browser history, scroll, and focus without owning route
      existence.
- [x] Forbid generated JS from owning the app contract:
  - [x] route existence
  - [x] auth or authorization decisions
  - [x] business rules
  - [x] database access
  - [x] trusted server validation
  - [x] action behavior
  - [x] global app state
  - [x] page loading policy
  - [x] cache/revalidation policy
- [x] Keep the compiler manifest, generated Go runtime, and user Go code as the
      source of truth for routes, backend behavior, request-time data, and
      security policy.
- [x] Keep forms progressively enhanced: generated JS may improve submission and
      partial swaps, but supported action forms should degrade to normal HTTP
      POST behavior where possible.
- [x] Keep `client {}` limited to local component/UI behavior such as toggles,
      tabs, counters, focus, small filters, and visual state.

## Go Handler Binding

- [x] Resolve same-package handler ownership through `go list`.
- [x] Validate exported handlers and input types through standard `go/parser`
      and `go/ast`.
- [x] Add sibling package type-check validation through standard `go/types`.
- [x] Cache package inspection by source directory or import path.
- [x] Bind exact exported handler symbols. Do not map lowercase names to exported names.
- [x] Keep missing handler symbols non-fatal in development/stub mode and
      generate `501`.
- [x] Keep unsupported signatures non-fatal in development/stub mode and
      generate `501`.
- [x] Support action signatures:
  - [x] `func Name(context.Context) (response.Response, error)`
  - [x] `func Name(context.Context, Input) (response.Response, error)`
  - [x] `func Name(context.Context, *Input) (response.Response, error)`
  - [x] `func Name(context.Context, form.Values) (response.Response, error)`
- [x] Support API signature:
  - [x] `func Name(context.Context, *http.Request) (response.Response, error)`
- [x] Record binding signature kind, input type, pointer mode, package, and import requirements.
- [x] Handle generated import alias collisions deterministically.
- [x] Document that feature packages must not import generated app output.
- [x] Define the request-scoped context contract before broad action/API support:
  - [x] decide whether handlers receive `context.Context` plus runtime helpers
        such as `app.Request(ctx)`, `app.Params(ctx)`, `app.CSRF(ctx)`, and
        `app.Session(ctx)`.
  - [x] decide not to use an explicit `app.Context`.
- [x] Define binding severity policy:
  - [x] dev/migration mode may report missing or unsupported handlers and
        generate `501`.
  - [x] strict/production mode should fail build for missing or unsupported
        explicitly declared handlers.
  - [x] optional stub mode, if kept, should require an explicit flag such as
        `--allow-missing-backend`.

## Endpoint Metadata And Discovery

- [x] Normalize all endpoints into one framework-neutral endpoint metadata model.
- [x] Include endpoint source, kind, package path, package name, symbol, method,
      path, signature kind, input type, source span, and binding status.
- [x] Merge endpoint metadata from `.gwdk` endpoint declarations and explicit Go
      endpoint comments.
- [x] Support explicit Go endpoint comments as an optional discovery source:
  - [x] `//gowdk:act POST /login`
  - [x] `//gowdk:api GET /api/session`
- [x] Do not auto-discover endpoints from function names alone.
- [x] Do not scan Gin/Echo/Fiber route registration code in the first endpoint
      discovery model.
- [x] Make route conflicts between `.gwdk` declarations and Go endpoint comments
      hard diagnostics; never silently pick a winner.
- [x] Keep adapter IR independent of whether an endpoint came from `.gwdk` or a
      Go comment.
- [x] Keep route metadata limited to `static`, `spa`, `ssr`, and `hybrid`;
      actions and APIs are endpoint metadata, not route kinds.
- [x] Surface route-mode disabled lanes as `info` metadata in `gowdk routes`
      output and as `info:` console lines on stderr.

## Forms, Actions, API, And Fragments

- [x] Generate typed action struct decoders from same-package Go AST metadata;
      do not use runtime reflection for struct shape decoding.
- [x] Keep `runtime/form` limited to value normalization, allowlist checks, and
      scalar parse helpers used by generated decoder code.
- [x] Decode typed action structs from `form:"name"` tags first, then exported Go field names.
- [x] Ignore `form:"-"` fields.
- [x] Reject unknown submitted fields.
- [x] Support `string`, `[]string`, `bool`, signed integers, and unsigned integers.
- [x] Define empty-value behavior for numeric and boolean fields.
- [x] Strip or reserve runtime fields before user input decoding, including
      `_csrf`, `_gwdk`, `_method`, and any generated runtime metadata fields.
- [x] Decide submit button intent handling before unknown-field rejection.
- [x] Decide checkbox absence behavior.
- [x] Decide repeated scalar behavior: reject, first value, or last value.
- [x] Keep nested structs, maps, and slices other than `[]string` unsupported in
      v1 unless explicitly designed.
- [x] Return structured decode errors without exposing submitted values.
- [x] Wire CSRF token generation and validation into generated action adapters.
- [x] Define generated form token exposure for SPA/action pages.
- [x] Define invalid-CSRF response status and body shape.
- [x] Keep `NoopCSRF` test-only.
- [x] Keep redirects, JSON, HTML, fragments, validation, auth, and storage in user Go handlers returning `runtime/response.Response`.
- [x] Add structured form error and validation fragment patterns.
- [x] Keep file uploads out of generated actions; uploads belong in user-owned
      API/server handlers with explicit body limits, storage, validation,
      cleanup, and security rules.
- [x] Improve `select`, radio, and checkbox group handling for server forms.
- [x] Add production-safe action/API docs covering CSRF, redirects, validation, fragments, cache/no-store, and error handling.

## Framework Adapters

- [x] Keep GOWDK core `net/http` compatible.
- [x] Ensure generated apps expose or mount as `http.Handler`.
- [x] Keep Gin, Echo, and Fiber out of compiler/runtime core dependencies.
- [x] Add optional adapter packages only after the core handler contract is
      stable:
  - [x] `runtime/adapters/gin`
  - [x] `runtime/adapters/echo`
  - [x] `runtime/adapters/fiber`
- [x] Make adapters wrap the same `http.Handler` produced by GOWDK; do not emit
      framework-specific code by default.
- [x] Document Fiber's `net/http` adaptor overhead and semantic differences if a
      Fiber adapter is added.
- [x] Keep endpoint metadata framework-neutral so framework adapters consume the
      same route/action/API model as the standard generated app.

## Generated Adapter Source

- [x] Define a typed backend adapter IR for imports, endpoint registrations, decoding, handler calls, response writing, and `501` fallbacks.
- [x] Generate backend endpoint registration from the IR through Go AST.
- [x] Replace broad action/API string builders with full Go AST emission.
  - [x] Action handler source is emitted through `go/ast` and guarded against
        `WriteString`/`strings.Builder` regression.
  - [x] API handler source is emitted through `go/ast` and guarded against
        `WriteString`/`strings.Builder` regression.
  - [x] Backend dispatch/proxy source is emitted through `go/ast` and guarded
        against `WriteString`/`strings.Builder` regression.
  - [x] SSR exact/dynamic handler source is emitted through `go/ast` and
        guarded against `WriteString`/`strings.Builder` regression.
  - [x] Generated CSRF helper declarations are emitted through `go/ast`.
  - [x] Generated app and backend app shell packages are emitted through
        `go/ast` and guarded against raw template regression.
- [x] Generate all generated Go with `go/ast` and `go/printer`, then run `go/format`.
- [x] Ban hardcoded line writing, `WriteString` chains, token concatenation, and source snippets for generated Go unless a documented temporary exception is strictly necessary.
- [x] Replace generated app shell Go templates with AST emission.
- [x] Preserve `//go:embed app` comments.
- [x] Sort imports and routes deterministically.
- [x] Never generate user handler functions, input structs, auth logic, validation policy, storage code, or service code.
- [x] Do not import missing or unsupported handler packages solely for `501` routes.
- [x] Drive one-binary, split frontend proxy, and backend-only app generation from the same route metadata.
- [x] Add structural tests for generated imports, route dispatch, and missing-handler output.

## Routing, Rendering, SSR, Hybrid, And Cache

- [x] Add runtime typed route param decoding helpers.
- [x] Add generated typed route param bindings once route-param type syntax
      exists.
- [x] Add route-level metadata.
- [x] Generate sitemap output from the full route graph.
- [x] Add redirects and error pages.
- [x] Execute request-time `load {}` in generated SSR handlers.
- [x] Wire generated SSR guards.
- [x] Add full request-time user logic for SSR pages through generated SSR
      handlers.
- [x] Add SSR/action/API error boundaries.
- [x] Define hybrid pages as SPA by default with explicit request-time capabilities.
- [x] Define cache and revalidation behavior for static files, SPA routes,
      backend endpoints, partial responses, SSR routes, and hybrid pages.
- [x] Add syntax for route/cache policy once generated route metadata stabilizes.
- [x] Ensure generated binaries apply cache policy consistently.
- [x] Keep SSR optional instead of making it the framework identity.

## Components, Client Language, And Islands

- [x] Add real branch mount/unmount for `g:if`.
- [x] Remove hidden-toggle `g:if` as the JS island update behavior; static
      first render may still include `hidden` until the island mounts.
- [x] Add parent-to-child expression props beyond current string/build-data interpolation.
- [x] Add child-to-parent events.
- [x] Add bindable child state.
- [x] Add typed component exports.
- [x] Add named slots and scalar scoped slots.
- [x] Add scoped component CSS and component-level assets.
- [x] Add a documented contract for component state, props, stores, client code, and generated runtime behavior.
- [x] Add a proper reactive dependency graph.
- [x] Track dependencies for computed values automatically across the supported
      client language.
- [x] Allow richer computed bodies, loops inside `client {}`, DOM/component
      event object access, and broader compiler-owned built-ins.
- [x] Batch updates predictably and detect reactive cycles with useful diagnostics.
- [x] Keep browser behavior typed and compiler-owned instead of turning `client {}` into unbounded JavaScript.
- [x] Define and implement the production WASM island ABI from ADR 0004.
- [x] Add browser-side Go logic contracts for explicit WASM islands.
- [x] Validate required WASM island entrypoints and exports.

## CSS, Plugins, Assets, And Packaging

- [x] Add full addon/plugin loading for built-in and external importable Go addons.
- [x] Add component ASTs for CSS scoping and hashing.
- [x] Add page-aware CSS processor selections.
- [x] Add Tailwind and CSS deployment docs with real commands.
- [x] Keep module selection as artifact packaging, not runtime module orchestration.
- [x] Keep generated app WASM deploy artifacts separate from explicit browser WASM islands.
- [x] Add asset cache policy and hashing rules for generated binaries.

## Dev, Playground, And Tooling

- [x] Extend `gowdk dev` to run generated app/runtime-kit flows when backend routes or SSR are selected.
- [x] Decide whether generated backend processes are restarted by `gowdk dev` or proxied to a user-owned backend command.
- [x] Add fast rebuild caching beyond current changed-page incremental output.
- [x] Add local deploy preview support.
- [x] Add local hot deploy pipeline for generated apps through `gowdk preview --hot`
      and generated app restarts in `gowdk dev --app`.
- [x] Add browser playground UI.
- [x] Load the GOWDK compiler as WebAssembly in the browser UI.
- [x] Add editable project tree, full generated HTML/CSS/JS viewers, starter
      templates, shareable links, and export/download.
- [x] Improve baseline LSP completions for components, routes, props, state,
      stores, client constructs, and directives.
- [x] Add project-aware LSP completions for concrete component names, route
      IDs, prop names, state fields, store names, and directives.
- [x] Add editor navigation for component calls, route IDs, guards, stores, and imported Go contracts.
- [x] Add editor release workflow coverage.

## Documentation

- [x] Update README examples to package-first action/API syntax.
- [x] Update language docs for required package declarations.
- [x] Update action/API docs around exact exported Go handlers.
- [x] Update routing docs for package-integrated endpoint declarations.
- [x] Update deployment docs for one-binary and split runtime-kit flow.
- [x] Update architecture docs for GOWDK compiler plus app/runtime kit.
- [x] Update requirements statuses after package integration lands.
- [x] Update login example to package-first syntax and typed action input.
- [x] Keep README, requirements, architecture, roadmap, and this checklist in sync.

## Verification Commands

Keep these current as the quality gates:

```sh
gofmt -w <changed-go-files>
go test ./...
go build ./cmd/gowdk
node --check editors/vscode/extension.js
node --check editors/vscode/extension-core.js
node --test editors/vscode/*.test.js
go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk
cd examples/login && make check
cd examples/login && make build
cd examples/login && make split-build
```

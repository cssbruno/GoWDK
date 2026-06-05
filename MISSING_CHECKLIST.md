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
actions, APIs, CSRF, partial fragments, SSR addon contracts, embedded assets,
and one-binary serving.

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
- [x] SSR is optional and rejected without the SSR addon.
- [x] `load {}` is rejected on SPA pages.
- [x] Dynamic SPA/action routes require `paths {}`.
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
- [x] Missing or unsupported first-slice action/API bindings generate clear `501` responses.
- [x] Generated app Go source is formatted before write.
- [x] CLI tooling includes `tokens`, `fmt`, `check`, `manifest`, `sitemap`, `routes`, and `lsp`.

## GOWDK Compiler

- [ ] Define the GOWDK AST for package declarations, annotations, routes,
      imports, stores, blocks, component contracts, and source spans.
- [ ] Add a GOWDK analyzer that lowers the GOWDK AST into normalized route,
      component, package, type, asset, and generated adapter metadata.
- [ ] Require `package <name>` as the first non-comment declaration in every page, layout, and component `.gwdk` file.
- [ ] Store package name and package source span in manifest records.
- [ ] Validate `.gwdk` package names against sibling `.go` files.
- [ ] Fail check/build on Go package type-check errors with `go_package_error`.
- [ ] Replace legacy action/API blocks with exact exported route declarations:

```gwdk
package auth

act Login POST "/"
api Session GET "/api/session"
```

- [ ] Reject legacy action/API block syntax with migration diagnostics.
- [ ] Reject non-exported handler names in `act`, `api`, and `g:post`.
- [ ] Reject non-POST action methods.
- [ ] Keep route declarations in `.gwdk`; keep behavior in normal Go.
- [ ] Add package, handler symbol, route, method, binding status, and binding message to manifest, routes output, and build report metadata.
- [ ] Add a stable internal IR for templates, client behavior, routes, assets, and generated output.
- [ ] Expand source spans and suggestions across parser, route, view, component, client, package, and build errors.
- [ ] Support same-package build functions or document why explicit imports remain required.
- [ ] Support broader build-time data beyond the first literal/imported no-argument subset.
- [ ] Use generated route metadata in request-time handlers instead of only embedding it with build output.

## App Runtime Kit

- [ ] Add shared backend routing primitives in `runtime/app`:
  - [ ] `BackendHandler`
  - [ ] `BackendRouter`
  - [ ] `NewBackendRouter`
  - [ ] action and API route registration
  - [ ] method/path dispatch with normalized paths
- [ ] Add runtime adapter helpers:
  - [ ] `Action0`
  - [ ] `ActionForm[T]`
  - [ ] `ActionFormPtr[T]`
  - [ ] `ActionValues`
  - [ ] `APIHandler`
  - [ ] `NotImplemented`
- [ ] Update generated apps to use one backend hook instead of separate action/API hook shapes.
- [ ] Preserve no-store defaults for request-time action/API/fragment responses.
- [ ] Keep request body size limits in generated action adapters.
- [ ] Wire generated guards for SSR/action/API paths.
- [ ] Add runtime metrics only after handler contracts settle.

## Go Handler Binding

- [ ] Resolve same-package handler ownership through `go list`.
- [ ] Validate exported handlers and input types through standard `go/parser`,
      `go/ast`, and `go/types`.
- [ ] Cache package inspection by source directory or import path.
- [ ] Bind exact exported handler symbols. Do not map lowercase names to exported names.
- [ ] Keep missing handler symbols non-fatal and generate `501`.
- [ ] Keep unsupported signatures non-fatal and generate `501`.
- [ ] Support action signatures:
  - [ ] `func Name(context.Context) (response.Response, error)`
  - [ ] `func Name(context.Context, Input) (response.Response, error)`
  - [ ] `func Name(context.Context, *Input) (response.Response, error)`
  - [ ] `func Name(context.Context, form.Values) (response.Response, error)`
- [ ] Support API signature:
  - [ ] `func Name(context.Context, *http.Request) (response.Response, error)`
- [ ] Record binding signature kind, input type, pointer mode, package, and import requirements.
- [ ] Handle generated import alias collisions deterministically.
- [ ] Document that feature packages must not import generated app output.

## Forms, Actions, API, And Fragments

- [ ] Add `runtime/form.DecodeStruct[T any](Values) (T, error)`.
- [ ] Decode typed action structs from `form:"name"` tags first, then exported Go field names.
- [ ] Ignore `form:"-"` fields.
- [ ] Reject unknown submitted fields.
- [ ] Support `string`, `[]string`, `bool`, signed integers, and unsigned integers.
- [ ] Define empty-value behavior for numeric and boolean fields.
- [ ] Return structured decode errors without exposing submitted values.
- [ ] Wire CSRF token generation and validation into generated action adapters.
- [ ] Define generated form token exposure for SPA/action pages.
- [ ] Define invalid-CSRF response status and body shape.
- [ ] Keep `NoopCSRF` test-only.
- [ ] Keep redirects, JSON, HTML, fragments, validation, auth, and storage in user Go handlers returning `runtime/response.Response`.
- [ ] Add structured form error and validation fragment patterns.
- [ ] Add file upload support only after body limits, storage, validation, cleanup, and security rules are defined.
- [ ] Improve `select`, radio, and checkbox group handling for server forms.
- [ ] Add production-safe action/API docs covering CSRF, redirects, validation, fragments, cache/no-store, and error handling.

## Generated Adapter Source

- [ ] Define a typed backend adapter IR for imports, route registrations, decoding, handler calls, response writing, and `501` fallbacks.
- [ ] Generate backend route registration from the IR through Go AST.
- [ ] Replace broad action/API string builders with full Go AST emission.
- [ ] Generate all generated Go with `go/ast` and `go/printer`, then run `go/format`.
- [ ] Ban hardcoded line writing, `WriteString` chains, token concatenation, and source snippets for generated Go unless a documented temporary exception is strictly necessary.
- [ ] Replace generated app shell Go templates with AST emission.
- [ ] Preserve `//go:embed app` comments.
- [ ] Sort imports and routes deterministically.
- [ ] Never generate user handler functions, input structs, auth logic, validation policy, storage code, or service code.
- [ ] Do not import missing or unsupported handler packages solely for `501` routes.
- [ ] Drive one-binary, split frontend proxy, and backend-only app generation from the same route metadata.
- [ ] Add structural tests for generated imports, route dispatch, and missing-handler output.

## Routing, Rendering, SSR, Hybrid, And Cache

- [ ] Add typed route param decoding.
- [ ] Add route-level metadata.
- [ ] Generate sitemap output from the full route graph.
- [ ] Add redirects and error pages.
- [ ] Execute request-time `load {}` in generated SSR handlers.
- [ ] Wire generated SSR guards.
- [ ] Add full request-time user logic for SSR pages through the SSR addon.
- [ ] Add SSR/action/API error boundaries.
- [ ] Define hybrid pages as SPA by default with explicit request-time capabilities.
- [ ] Define cache and revalidation behavior for SPA, action, API, partial, SSR, and hybrid routes.
- [ ] Add syntax for route/cache policy once generated route metadata stabilizes.
- [ ] Ensure generated binaries apply cache policy consistently.
- [ ] Keep SSR optional instead of making it the framework identity.

## Components, Client Language, And Islands

- [ ] Add real branch mount/unmount for `g:if`.
- [ ] Keep hidden-toggle `g:if` only as an explicit mode if useful.
- [ ] Add parent-to-child expression props beyond current string/build-data interpolation.
- [ ] Add child-to-parent events.
- [ ] Add bindable child state.
- [ ] Add typed component exports.
- [ ] Add named slots and scoped slots.
- [ ] Add scoped component CSS and component-level assets.
- [ ] Add a documented contract for component state, props, stores, client code, and generated runtime behavior.
- [ ] Add a proper reactive dependency graph.
- [ ] Track dependencies for computed values automatically across the full client language.
- [ ] Allow richer computed bodies, loops inside `client {}`, event object access, and broader compiler-owned built-ins.
- [ ] Batch updates predictably and detect reactive cycles with useful diagnostics.
- [ ] Keep browser behavior typed and compiler-owned instead of turning `client {}` into unbounded JavaScript.
- [ ] Define and implement the production WASM island ABI from ADR 0004.
- [ ] Add browser-side Go logic contracts for explicit WASM islands.
- [ ] Validate required WASM island entrypoints and exports.

## CSS, Plugins, Assets, And Packaging

- [ ] Add full addon/plugin loading.
- [ ] Add component ASTs for CSS scoping and hashing.
- [ ] Add page-aware CSS processor selections.
- [ ] Add Tailwind and CSS deployment docs with real commands.
- [ ] Keep module selection as artifact packaging, not runtime module orchestration.
- [ ] Keep generated app WASM deploy artifacts separate from explicit browser WASM islands.
- [ ] Add asset cache policy and hashing rules for generated binaries.

## Dev, Playground, And Tooling

- [ ] Extend `gowdk dev` to run generated app/runtime-kit flows when backend routes or SSR are selected.
- [ ] Decide whether generated backend processes are restarted by `gowdk dev` or proxied to a user-owned backend command.
- [ ] Add fast rebuild caching beyond current changed-page incremental output.
- [ ] Add deploy preview support.
- [ ] Add hot deploy pipeline for generated apps.
- [ ] Add browser playground UI.
- [ ] Load the GOWDK compiler as WebAssembly in the browser UI.
- [ ] Add editable project tree, live preview, generated HTML/CSS/JS viewers, diagnostics panel, starter templates, shareable links, and export/download.
- [ ] Improve LSP completions for components, routes, props, state, stores, and directives.
- [ ] Add editor navigation for component calls, route IDs, guards, stores, and imported Go contracts.
- [ ] Add editor release workflow coverage.

## Documentation

- [ ] Update README examples to package-first action/API syntax.
- [ ] Update language docs for required package declarations.
- [ ] Update action/API docs around exact exported Go handlers.
- [ ] Update routing docs for package-integrated route declarations.
- [ ] Update deployment docs for one-binary and split runtime-kit flow.
- [ ] Update architecture docs for GOWDK compiler plus app/runtime kit.
- [ ] Update requirements statuses after package integration lands.
- [ ] Update login example to package-first syntax and typed action input.
- [ ] Keep README, requirements, architecture, roadmap, and this checklist in sync.

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

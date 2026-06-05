# GOWDK Missing Checklist

This checklist tracks what GOWDK still needs before it can be presented as a
complete Go-native web app platform.

The goal is not to copy Svelte, React, or any other frontend framework. The
goal is to build a strong GOWDK architecture:

- Go owns the app contract.
- `.gwdk` files stay portable.
- Static output is the default.
- Client interactivity is explicit and compiler-generated.
- Hot deploy and fast local iteration are first-class features.
- Browser compilation lets people try the platform without installing a toolchain.

This file should track missing work, not old names. Current implementation
status is summarized in `README.md`, `docs/product/requirements.md`, and
`docs/engineering/architecture.md`.

## Core Compiler

- [x] Add first-slice `g:for` list rendering.
- [x] Require `g:key` for mutable list rendering.
- [x] Add first-slice keyed row reuse and reorder for generated islands.
- [x] Add source maps for generated JavaScript islands in development mode.
- [x] Add structured diagnostics with file, line, column, ranges, and suggestions for common parser, route, client, view, event, and list errors.
- [ ] Add real branch mount/unmount for `g:if`.
- [ ] Keep the current hidden-toggle `g:if` behavior only as an explicit mode if useful.
- [ ] Add a stable internal IR for templates, client behavior, routes, assets, and generated output.
- [ ] Expand diagnostics so every compiler/build failure has precise source ranges and actionable suggestions.
- [ ] Add generated-output inspection in dev mode.
- [ ] Make invalid client/template code fail with clear compiler errors across the full supported syntax.
- [ ] Keep generated output deterministic so deploy diffs stay small.
- [ ] Support arbitrary build-time statements beyond the first literal/imported build-data subset.
- [ ] Use generated route metadata in request-time handlers instead of only embedding it with static output.

## Client Reactivity

- [x] Add effect cleanup support.
- [x] Add first-slice computed values.
- [x] Detect computed dependency cycles with compiler diagnostics.
- [x] Support nested field and indexed state reads in island expressions.
- [x] Support first-slice array updates with compiler-owned `append`, `remove`, and `move` built-ins.
- [ ] Add a proper reactive dependency graph.
- [ ] Track dependencies for computed values automatically across the full client language.
- [ ] Allow computed values to use richer bodies, not only one return expression.
- [ ] Allow effects to depend on multiple values.
- [ ] Generate explicit state setters.
- [ ] Batch updates predictably.
- [ ] Detect reactive cycles outside computed values with useful diagnostics.
- [ ] Support nested object mutation/update operations.
- [ ] Support broader array updates without requiring manual full replacement or first-slice helper built-ins.
- [x] Wire page-scoped stores to runtime shared-state subscriptions.
- [ ] Keep browser behavior typed and compiler-owned instead of turning `client {}` into unbounded JavaScript.

## Client Language

- [x] Add local variables inside `client {}` functions.
- [x] Add first-slice helper functions.
- [x] Add return values for internal helper functions.
- [x] Add helper function calls inside client expressions.
- [x] Add first-slice typed built-ins: `len`, `string`, `int`, and `float`.
- [x] Add first-slice string built-ins: `lower`, `upper`, and `contains`.
- [x] Add Go-ish conditional expressions.
- [x] Add event modifiers: `.prevent`, `.stop`, `.once`, `.capture`, `.debounce`, and `.throttle`.
- [x] Add DOM refs with first-slice `Focus`, `Blur`, and `ScrollIntoView`.
- [ ] Add loops inside `client {}` blocks.
- [ ] Add event object access.
- [ ] Add date/time helpers and broader compiler-owned built-ins.
- [ ] Improve type inference.
- [ ] Improve validation between Go state, props, stores, and client expressions.
- [ ] Add clearer errors for unsupported syntax.
- [ ] Add full runtime validation for user browser logic in WASM islands.

## Component Model

- [x] Add explicit/discovered `.cmp.gwdk` inputs.
- [x] Add typed imported Go props contracts.
- [x] Add typed imported Go state contracts and init functions.
- [x] Add self-closing component calls.
- [x] Add wrapper component calls with default `<slot />` content projection.
- [x] Add generated JavaScript islands for stateful component calls.
- [x] Add explicit `g:island="wasm"` component calls.
- [x] Reject duplicate and redundant component implementations.
- [ ] Add parent-to-child expression props beyond current string/build-data interpolation.
- [ ] Add non-string props in legacy `props {}` blocks.
- [ ] Add child-to-parent events.
- [ ] Add bindable child state.
- [ ] Add named slots and scoped slots.
- [ ] Add typed component exports.
- [ ] Add component composition documentation.
- [ ] Add scoped CSS rules per component.
- [ ] Add component-level assets.
- [ ] Wire generated Go component packages into generated app layouts.
- [ ] Add a documented contract for component state, props, stores, client code, and generated runtime behavior.

## Forms And Actions

- [x] Parse first-slice `act {}` input, validation intent, redirect, and fragment bodies.
- [x] Lower `g:post` forms to same-page POST routes for static/action pages.
- [x] Generate first-slice form input decoders.
- [x] Reject unexpected fields and enforce required fields in generated first-slice decoders.
- [x] Generate first-slice POST redirect handlers.
- [x] Generate first-slice action fragment responses.
- [x] Add partial-update client runtime with target/swap headers, swaps, loading metadata, and focus restoration.
- [x] Add signed CSRF validator in `addons/actions`.
- [ ] Wire CSRF generation and validation into generated action handlers.
- [ ] Resolve real user Go action input types.
- [ ] Execute user action logic in generated handlers.
- [ ] Add typed form model generation.
- [ ] Add validation rules beyond the first required-field slice.
- [ ] Pair client validation with server validation.
- [ ] Add structured form errors and validation fragments.
- [ ] Add optimistic submit support.
- [ ] Improve progressive enhancement for `g:post`.
- [ ] Add file uploads.
- [ ] Improve `select`, radio, checkbox group handling for server forms.
- [ ] Add explicit loading states.
- [ ] Add disabled states during submission.
- [ ] Add production-safe action docs covering CSRF, redirects, validation, and fragments.

## API Handlers

- [x] Parse first-slice API route metadata.
- [x] Add `addons/api` capability boundary.
- [x] Add first-slice codegen stubs for API handlers.
- [ ] Generate API handlers into embedded apps.
- [ ] Define API-only file or route semantics.
- [ ] Execute user-owned API handler logic.
- [ ] Add typed request decoding and response writing contracts.
- [ ] Add API error handling and no-store/cache behavior.
- [ ] Add API docs and examples.

## Routing, Rendering, And SSR

- [x] Default render mode to `static`.
- [x] Reject `@render ssr` without the SSR addon.
- [x] Reject `load {}` on static pages.
- [x] Require `paths {}` for dynamic static/action routes.
- [x] Prerender the first literal dynamic `paths {}` subset.
- [x] Generate simple concrete `@render ssr` pages in embedded apps when they do not use `load {}`.
- [x] Add route shape, duplicate param, duplicate route pattern, and route-method conflict validation.
- [x] Add manifest and sitemap CLI output.
- [ ] Add nested routes.
- [ ] Add layouts with complete inheritance/composition rules.
- [ ] Add typed route param decoding.
- [ ] Add route-level loaders.
- [ ] Execute request-time `load {}` in generated SSR handlers.
- [ ] Wire generated guard enforcement for SSR/action/API paths.
- [ ] Add dynamic SSR route generation.
- [ ] Add full request-time user logic for SSR pages.
- [ ] Add route-level metadata.
- [ ] Add redirects.
- [ ] Add error pages.
- [ ] Add error boundaries for SSR/action/API paths.
- [ ] Generate sitemap output from the full route graph.
- [ ] Add optional route prefetching.
- [ ] Keep SSR optional instead of making it the default rendering model.

## Hybrid, Cache, And Request-Time Policy

- [ ] Define hybrid pages as static by default with explicit request-time capabilities.
- [ ] Define cache and revalidation behavior for static routes.
- [ ] Define cache and revalidation behavior for action, API, partial, SSR, and hybrid routes.
- [ ] Add syntax for route/cache policy once generated route metadata stabilizes.
- [ ] Document partial update caching and no-store defaults.
- [ ] Ensure generated binaries apply cache policy consistently.

## CSS, Plugins, And Assets

- [x] Add CSS addon boundary and compile-time processor contract.
- [x] Add configured stylesheet links.
- [x] Add discovered CSS inputs and generated page CSS output.
- [x] Add static class extraction.
- [x] Add `@css` page selection.
- [x] Add experimental Tailwind v4 standalone-CLI processor wrapper.
- [ ] Add full addon/plugin loading.
- [ ] Add component ASTs for CSS scoping and hashing.
- [ ] Add page-aware CSS processor selections.
- [ ] Add scoped component CSS.
- [ ] Add component-level asset handling.
- [ ] Add Tailwind and CSS deployment docs with real commands.

## WASM And Browser Compiler

- [x] Add `gowdk build --wasm` for generated app WASM deploy artifacts.
- [x] Add `Build.Targets[].WASM`.
- [x] Add `playground` in-memory compiler API.
- [x] Add `cmd/gowdk-wasm` browser/WASM compiler wrapper.
- [x] Add explicit first-slice WASM island asset emission.
- [x] Compile declared component WASM island packages with `GOOS=js GOARCH=wasm`.
- [x] Reject unsupported server/process/network imports in WASM island packages.
- [ ] Add a browser playground UI.
- [ ] Load the GOWDK compiler as WebAssembly in the browser UI.
- [ ] Add an editable project tree.
- [ ] Add live preview.
- [ ] Add generated HTML viewer.
- [ ] Add generated CSS viewer.
- [ ] Add generated JavaScript viewer.
- [ ] Add diagnostics panel.
- [ ] Add starter templates.
- [ ] Add shareable playground links.
- [ ] Add project export/download.
- [ ] Add examples that show the browser compiler producing real GOWDK output.
- [ ] Define and implement a production WASM island ABI.
- [ ] Add browser-side Go logic contracts for WASM islands.
- [ ] Validate required WASM island entrypoint registration and exports.

## Hot Deploy And Dev Loop

- [x] Add compiler watch mode that explains what changed and what rebuilt.
- [x] Add content-hash based rebuild detection.
- [x] Add page-local incremental static output for plain static watch output.
- [x] Skip identical generated static/app writes.
- [x] Make failed rebuilds keep the last good generated binary running.
- [x] Add generated-binary restart with `watch --restart`.
- [x] Add browser live reload in `gowdk dev`.
- [x] Add one-command local build/run workflows.
- [ ] Add a hot deploy pipeline for generated apps.
- [ ] Add fast rebuild caching beyond current changed-page incremental output.
- [ ] Add deploy preview support.

## Compiler-Owned Safety

- [x] Add HTML escaping for current static text and attributes.
- [x] Add signed CSRF validator support in `addons/actions`.
- [x] Add rate-limit addon boundary, in-memory store, and Redis adapter.
- [x] Add forbidden-import checks for WASM island packages.
- [ ] Wire CSRF into generated action handlers.
- [ ] Ensure generated logs and diagnostics do not expose secrets or sensitive form data.
- [ ] Document source-map handling so production builds do not leak sensitive source.

## Editor And Tooling

- [x] Add `tokens`, `fmt`, `check`, `manifest`, `sitemap`, `routes`, and `lsp` CLI commands.
- [x] Add dependency-free LSP server with diagnostics, formatting, and keyword completions.
- [x] Add VS Code extension files, syntax highlighting, snippets, and route visualizer.
- [ ] Improve LSP completions for components, routes, props, state, stores, and directives.
- [ ] Improve editor diagnostics for full component and client language coverage.
- [ ] Add editor navigation for component calls, route IDs, guards, stores, and imported Go contracts.
- [ ] Add editor docs and release workflow coverage.

## Documentation

- [ ] Rewrite getting started around the current reality: clone, build, run.
- [ ] Write real CLI docs with commands that work today.
- [ ] Write config docs with real examples.
- [ ] Write architecture docs that explain the compiler pipeline.
- [ ] Write client language docs with supported and unsupported syntax.
- [ ] Write component docs with state, props, slots, stores, and client behavior.
- [ ] Write routing docs.
- [ ] Write actions/forms docs.
- [ ] Write API docs.
- [ ] Write deployment docs.
- [ ] Write browser compiler docs.
- [ ] Add examples that match the actual compiler, not planned features.
- [ ] Clearly separate implemented features from planned features.
- [ ] Keep README, requirements, architecture, and this checklist in sync.

## Product Positioning

- [ ] Stop implying release/version features that do not exist.
- [ ] Do not claim Svelte, React, or Solid parity.
- [ ] Describe GOWDK as Go-native reactive islands with compiler-generated browser behavior.
- [ ] Show real GOWDK code and real generated output.
- [ ] Show the one-binary deploy story.
- [ ] Show the hot deploy story once it is implemented.
- [ ] Show the browser compiler once it is implemented.
- [x] Document release/version readiness honestly.

## Highest Priority

These are the most important missing pieces:

- [ ] Real user Go action execution
- [ ] Generated CSRF-wired action handlers
- [ ] Generated API handlers
- [ ] Request-time `load {}` execution
- [ ] Generated guard enforcement
- [ ] Hybrid/cache/revalidation policy
- [ ] Full reactive dependency graph
- [ ] Richer `client {}` language
- [ ] Component composition beyond default slots
- [ ] Hot deploy pipeline
- [ ] Browser playground UI
- [ ] Production WASM island ABI
- [ ] Accurate documentation

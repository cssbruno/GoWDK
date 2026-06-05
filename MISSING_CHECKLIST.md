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

## Core Compiler

- [ ] Add `g:each` list rendering.
- [ ] Add keyed list rendering for stable DOM updates.
- [ ] Add real branch mount/unmount for `g:if`.
- [ ] Keep the current hidden-toggle `g:if` behavior only as an explicit mode if useful.
- [ ] Add a stable internal IR for templates, client behavior, routes, assets, and generated output.
- [ ] Add better diagnostics with file, line, column, source ranges, and useful suggestions.
- [ ] Add source maps for generated JavaScript.
- [ ] Add generated-output inspection in dev mode.
- [ ] Make invalid client/template code fail with clear compiler errors.
- [ ] Keep generated output deterministic so deploy diffs stay small.

## Client Reactivity

- [ ] Add a proper reactive dependency graph.
- [ ] Track dependencies for computed values automatically.
- [ ] Allow computed values to use richer bodies, not only one return expression.
- [ ] Allow effects to depend on multiple values.
- [x] Add effect cleanup support.
- [ ] Generate explicit state setters.
- [ ] Batch updates predictably.
- [ ] Detect reactive cycles with useful diagnostics.
- [ ] Support nested object updates.
- [ ] Support array updates without requiring manual full replacement for every case.
- [ ] Keep browser behavior typed and compiler-owned instead of turning `client {}` into unbounded JavaScript.

## Client Language

- [x] Add local variables inside `client {}` functions.
- [x] Add first-slice helper functions.
- [x] Add return values for internal helper functions.
- [x] Add helper function calls inside client expressions.
- [x] Add first-slice typed built-ins: `len`, `string`, `int`, and `float`.
- [ ] Add loops.
- [ ] Add event object access.
- [ ] Add date/time helpers and broader compiler-owned built-ins.
- [ ] Improve type inference.
- [ ] Improve validation between Go state and client expressions.
- [ ] Add clearer errors for unsupported syntax.

## Component Model

- [ ] Add parent-to-child props for nested components.
- [ ] Add child-to-parent events.
- [ ] Add bindable child state.
- [ ] Add slots, children, or content projection.
- [ ] Add typed component exports.
- [ ] Add component composition documentation.
- [ ] Add scoped CSS rules per component.
- [ ] Add component-level assets.
- [ ] Add a documented contract for component state, props, client code, and generated runtime behavior.

## Forms And Actions

- [ ] Add typed form model generation.
- [ ] Add validation rules.
- [ ] Pair client validation with server validation.
- [ ] Add structured form errors.
- [ ] Add optimistic submit support.
- [ ] Improve progressive enhancement for `g:post`.
- [ ] Add file uploads.
- [ ] Improve `select`, radio, checkbox group handling.
- [ ] Add loading states.
- [ ] Add disabled states during submission.
- [ ] Add focus restoration after partial updates.

## Routing And Rendering

- [ ] Add nested routes.
- [ ] Add layouts with clear inheritance rules.
- [ ] Add typed route param decoding.
- [ ] Add route-level loaders.
- [ ] Add route-level metadata.
- [ ] Add redirects.
- [ ] Add error pages.
- [ ] Add error boundaries for SSR/action paths.
- [ ] Add sitemap generation from the route graph.
- [ ] Add optional route prefetching.
- [ ] Keep SSR optional instead of making it the default rendering model.

## Hot Deploy And Dev Loop

- [x] Add browser live reload.
- [x] Add hot reload for static/template edits.
- [ ] Add hot deploy pipeline for generated apps.
- [x] Add compiler watch mode that explains what changed and what rebuilt.
- [ ] Add fast rebuild caching.
- [ ] Add deploy preview support.
- [ ] Add Render deployment docs.
- [ ] Add Cloudflare deployment docs.
- [ ] Add VPS/systemd deployment docs.
- [x] Add one-command local build/run/deploy workflows.
- [x] Make failed rebuilds keep the last good app running.

## Browser Compiler

- [ ] Add a browser playground UI.
- [ ] Load the GOWDK compiler as WebAssembly in the browser.
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

## Documentation

- [ ] Rewrite getting started around the current reality: clone, build, run.
- [ ] Write real CLI docs with commands that work today.
- [ ] Write config docs with real examples.
- [ ] Write architecture docs that explain the compiler pipeline.
- [ ] Write client language docs with supported and unsupported syntax.
- [ ] Write component docs with state, props, slots, and client behavior.
- [ ] Write routing docs.
- [ ] Write deployment docs.
- [ ] Write browser compiler docs.
- [ ] Add examples that match the actual compiler, not planned features.
- [ ] Clearly separate implemented features from planned features.

## Product Positioning

- [ ] Stop implying release/version features that do not exist.
- [ ] Do not claim Svelte, React, or Solid parity.
- [ ] Describe GOWDK as Go-native reactive islands with compiler-generated browser behavior.
- [ ] Show real GOWDK code and real generated output.
- [ ] Show the one-binary deploy story.
- [ ] Show the hot deploy story once it is implemented.
- [ ] Show the browser compiler once it is implemented.

## Highest Priority

These are the most important missing pieces:

- [ ] `g:each`
- [ ] Keyed DOM updates
- [ ] Real reactive dependency graph
- [ ] Richer `client {}` language
- [ ] Component composition
- [ ] Hot reload and hot deploy
- [ ] Browser compiler UI
- [ ] Accurate documentation

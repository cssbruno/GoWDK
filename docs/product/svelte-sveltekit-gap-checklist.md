# GOWDK Gap Checklist From Svelte/SvelteKit Comparison

This checklist tracks what is missing or weaker in GOWDK when compared with
Svelte and SvelteKit. It is not a request to clone their product model or
implement their full feature set. Use it to decide which comparison points need
a GOWDK-native answer, which should be deferred, and which should be rejected as
intentional non-goals.

Comparison anchors, not implementation requirements:

- Svelte 5 component/compiler surface: runes, broad template syntax, scoped
  styles, stores/context, compiler diagnostics, and generated component output.
- SvelteKit app framework surface: routing, load/data lifecycle, form actions,
  hooks, errors, page options, adapters, tooling, and deployment ecosystem.
- GOWDK product rules still win where they intentionally differ: explicit
  routes, build-time pages by default, Go-owned behavior, generated Go adapter
  glue, optional SSR, and one-binary deploys.

## How To Use This Checklist

- Keep checked items limited to behavior that is implemented, documented, and
  covered by tests or an explicit verification command.
- Keep intentional differences visible. If GOWDK deliberately chooses not to
  provide an equivalent for a Svelte/SvelteKit feature, move the item to the
  intentional non-goals section with the reason.
- Prefer completing contract work before feature breadth. A narrow, stable Go
  contract is better than a wide feature surface that cannot be explained or
  generated safely.
- Update `docs/product/requirements.md`, `docs/product/roadmap.md`, and
  `MISSING_CHECKLIST.md` when an item changes status.

Status legend:

- [x] Implemented and documented.
- [ ] Missing, incomplete, or not yet product-stable.
- Experimental: implemented enough to try, but not yet stable API.
- Intentional non-goal: not planned because it conflicts with GOWDK direction.

## Already Strong In GOWDK

- [x] Explicit portable `.gwdk` route declarations instead of folder-derived
      route truth.
- [x] Build-time SPA/static page output as the default page lane.
- [x] Backend action and API endpoint declarations that bind to same-package Go
      handlers.
- [x] Generated Go app output and local single-binary builds.
- [x] Split frontend/backend generation support.
- [x] Optional SSR lane selected with `@render ssr`.
- [x] Dynamic SPA route generation through literal `paths {}`.
- [x] Typed route param decoding for request-time routes.
- [x] CSRF-wired generated action adapters when enabled.
- [x] Typed generated action decoding for supported Go input structs.
- [x] Required/minlength/maxlength/pattern request-shape validation for direct
      literal action form controls.
- [x] Partial form enhancement and fragment response support.
- [x] Guards for generated SSR, action, and API handlers.
- [x] Optional request-time rate limiting hook.
- [x] Optional Echo, Gin, and Fiber adapters around the same `net/http`
      handler contract.
- [x] Generated route, asset, manifest, sitemap, and build-report metadata.
- [x] CLI commands for build, dev, preview, serve, check, format, manifest,
      sitemap, routes, tokens, and LSP.
- [x] VS Code/editor integration and language-server baseline.

## P0 Gaps

These are the highest-impact gaps surfaced by the Svelte/SvelteKit comparison.
Each item should result in a GOWDK-native contract decision, not a copy of the
Svelte/SvelteKit implementation.

- [ ] Decide the next GOWDK-native `view {}` language expansion.
      Comparison pressure comes from broad template features such as spread
      attributes, keyed blocks, async placeholders, snippets/renderable markup,
      raw HTML escape hatches, local constants, debug helpers, transitions,
      animations, DOM actions, and document/window/body/head targets. Do not
      implement these one-for-one unless they fit GOWDK's compiler-owned model.
- [ ] Define whether GOWDK wants a first-class snippet/render equivalent.
      Current default, named, and scalar scoped slots cover component insertion
      but not reusable typed markup values with parameters and lexical scope.
- [ ] Broaden the component props contract.
      Missing or weaker areas include non-string inline props, defaults,
      rest/spread props, prop renaming, recursive component ergonomics, dynamic
      component selection, and a clear bindable-prop equivalent.
- [ ] Decide the long-term GOWDK reactivity model.
      The comparison point is Svelte 5 runes, but GOWDK should preserve a
      bounded compiler-owned `client {}` language unless a stronger Go-first
      model is explicitly designed.
- [ ] Define a stronger shared-state/store runtime.
      Page stores exist, but they still need a stable GOWDK contract for shared
      island state, subscriptions, isolation, serialization, and teardown.
- [ ] Complete the GOWDK load/data lifecycle decision.
      Comparison pressure comes from server versus universal load, enhanced
      fetch, parent data, dependency tracking, client invalidation,
      post-action load reruns, serialized fetch results, and streaming data.
      Implement only the parts that serve GOWDK's build-time, endpoint, and SSR
      lanes.
- [ ] Complete hybrid request-time behavior.
      GOWDK currently treats bare hybrid pages as SPA output, while generated
      binary docs still list hybrid request-time behavior as unsupported.
- [ ] Add a general app-wide hook/middleware contract.
      Current guards and rate-limit registration are useful but narrower than
      app-wide request hooks, response transforms, fetch interception, error
      hooks, rerouting, transport hooks, and init hooks.
- [ ] Add route-scoped error-boundary syntax and behavior.
      Current panic boundaries plus optional `404.html`/`500.html` are weaker
      than route/page/layout error boundaries and expected versus unexpected
      error handling.
- [ ] Upgrade the dev server from live reload toward component-aware hot
      updates where appropriate.
      Missing or weaker areas include state-preserving component updates,
      browser error overlay, module/dependency graph, and precise update
      invalidation.
- [ ] Fix documentation consistency before public positioning.
      Several docs still describe first-slice or planned limitations that newer
      checklists mark complete. Align README, requirements, roadmap,
      deployment, CLI, language, release, and missing-work docs.
- [ ] Publish a release-readiness statement grounded in current behavior.
      README still says no release binary and not production-ready; release
      docs and current implementation status need a single consistent story.

## P0 Acceptance Criteria

- [ ] Every P0 item has a product decision: implement, defer, or intentional
      non-goal.
- [ ] Every implemented P0 item has language docs, reference docs, examples,
      and tests.
- [ ] Every deferred P0 item has a diagnostic or explicit documentation entry
      that tells users what is unsupported.
- [ ] The public README, requirements, roadmap, and missing checklist use the
      same status language.
- [ ] `go test ./...` and `go build ./cmd/gowdk` pass after each implementation
      slice.

## P1 Gaps

These are important product gaps once the P0 contracts are clear.

- [ ] Expand routing expressiveness where it fits GOWDK's explicit-route model.
      Candidate gaps: rest params, optional params, route groups, configurable
      trailing slash policy, and page/API same-path content negotiation.
- [ ] Generate stronger route-specific typed APIs for user Go.
      Current typed params are runtime-map based. Consider generated typed
      structs/accessors for params, load data, action data, endpoint metadata,
      and page contracts.
- [ ] Add a GOWDK page form lifecycle where it fits the product.
      Missing or weaker areas include page-level form state, status exposure,
      automatic data invalidation after actions, redirect/error behavior for
      enhanced forms, and nearest error-boundary handling.
- [ ] Broaden API endpoint support.
      Candidate gaps: richer body/query helpers, method fallback policy,
      response error shape, Accept/content negotiation, and route-page sharing.
- [ ] Make cache and revalidation more than response headers if needed.
      `@cache` and `@revalidate` currently map to HTTP cache headers. Missing
      or weaker areas include data dependency invalidation, prerender crawling,
      dynamic entry generation, and runtime reload policy.
- [ ] Make guards more expressive.
      Current guards are nil/error and generally map to HTTP 403. Consider
      guard-level redirect/response contracts and richer request-local state.
- [ ] Create a clear component CSS authoring model.
      GOWDK has CSS processors, page CSS, hashing, and scope metadata, but does
      not yet have an obvious component-local authoring model with default
      scoping, specificity rules, keyframes behavior, and docs.
- [ ] Add accessibility diagnostics.
      Candidate warnings: missing `alt`, invalid ARIA, clickable elements
      without keyboard support, label/control mismatch, empty headings, empty
      links, invalid autofocus patterns, and role misuse.
- [ ] Improve compiler diagnostics and parser recovery.
      GOWDK has useful diagnostics, spans, suggestions, formatting, and LSP,
      but the comparison shows a need for a larger warning/error catalogue and
      broader recovery behavior.
- [ ] Improve LSP/editor features.
      Candidate gaps: hover, go-to-definition, references, semantic tokens,
      code actions, workspace-wide component/type intelligence, exact source
      ranges, and generated route/type navigation.
- [ ] Add app testing scaffolds.
      Candidate outputs: Go handler tests, generated app smoke tests,
      Playwright/browser E2E, accessibility checks, and component/island test
      harnesses for scaffolded apps.
- [ ] Improve scaffolding beyond `gowdk init`.
      Candidate gaps: interactive templates, add-on selection, auth/db/test
      setup, migration commands, generated examples, and playground-to-project
      export.
- [ ] Add platform deployment adapters or generators where they do not fight
      the Go-first model.
      Candidate targets: static hosts, Docker, Fly.io, Render, Kubernetes,
      Cloudflare, Vercel, Netlify, systemd, and CDN cache examples.
- [ ] Document production operations as first-class guidance.
      Include secrets, CSRF secret rotation, reverse proxies, cache/CDN policy,
      health checks, metrics, logging, binary deployment, and rollback.

## P1 Acceptance Criteria

- [ ] Routing, form, API, cache, guard, CSS, LSP, testing, scaffold, and
      deployment decisions are each represented in requirements and roadmap.
- [ ] User-facing gaps have examples or clear unsupported diagnostics.
- [ ] New generated contracts are stable enough for application code to import
      only public `runtime/` or addon packages, never generated output.
- [ ] Docs include migration guidance for apps built against the previous
      first-slice behavior.

## P2 Gaps

These are polish, ecosystem, or expansion gaps.

- [ ] Decide whether full-page hydration is a goal.
      GOWDK currently favors static pages, partial updates, and islands rather
      than full component JavaScript output and hydration.
- [ ] Expand generated JavaScript island ergonomics while preserving the bounded
      compiler-owned client language.
- [ ] Expand client built-ins where they matter for real apps.
      Candidate areas: date/time formatting, collection helpers, string/number
      formatting, async-safe patterns, and focus/selection helpers.
- [ ] Improve WASM island validation and examples.
      Keep browser-side Go logic explicit and separate from backend handlers.
- [ ] Add service worker and offline/PWA guidance if it fits the product.
- [ ] Add image optimization guidance or integrations.
- [ ] Add package/addon discovery.
      This does not need to mirror any JavaScript package ecosystem, but users
      need a way to find official and community GOWDK addons.
- [ ] Add public playground onboarding if the playground is part of product
      strategy.
      Candidate gaps: hosted playground, tutorial integration, shareable
      examples, export to project, and import from repository.
- [ ] Add performance profiling documentation.
      Include build time, generated app size, generated JS size, SSR latency,
      action latency, binary size, and cache behavior.
- [ ] Add migration/onboarding guides for likely audiences.
      Candidate guides: Go templates to GOWDK, htmx-style apps to GOWDK,
      JavaScript-framework concepts to GOWDK, and static Go sites to GOWDK.

## P2 Acceptance Criteria

- [ ] Each P2 item has a product rationale before implementation starts.
- [ ] Optional ecosystem features remain optional and do not add mandatory npm,
      framework, or platform dependencies to core.
- [ ] Playground, package discovery, and migration work has a documented owner
      surface: CLI, docs, website, or repository examples.

## Detailed Workstreams

Use these workstreams to turn the high-level gaps into reviewable slices.

### 1. Documentation Truth Pass

- [ ] Choose one source-of-truth status model for all product docs.
- [ ] Add a status legend to `docs/product/requirements.md`.
- [ ] Update `MISSING_CHECKLIST.md` so checked items match implemented,
      documented, and verified behavior.
- [ ] Update `docs/product/roadmap.md` so completed slices are not described as
      future work.
- [ ] Update `docs/engineering/architecture.md` so current baseline and target
      architecture are separated clearly.
- [ ] Update `docs/reference/cli.md` for actual generated-binary behavior.
- [ ] Update `docs/reference/deployment.md` for actual SSR, action, API,
      fragment, and hybrid limits.
- [ ] Update `docs/language/ssr.md` for exact `load {}` support and limits.
- [ ] Update `docs/language/components.md` for actual store, CSS, scoped slot,
      export, and WASM island status.
- [ ] Update `docs/compiler/generated-output.md` for actual generated app,
      island, partial, SSR, and source-map behavior.
- [ ] Remove stale "first-slice" wording where behavior is now stable.
- [ ] Keep "first-slice" wording where the supported language or runtime
      surface is intentionally narrow.
- [x] Add a short comparison note that explains intentional product differences
      without turning external frameworks into the roadmap.

Done when:

- [ ] A reader can tell what works today without checking source code.
- [ ] No major feature is simultaneously marked implemented and planned in
      different docs.
- [ ] The README, requirements, roadmap, and missing checklist agree.

### 2. Template Language Contract

- [ ] Inventory all currently parsed markup syntax from `internal/view`.
- [ ] Document every supported `g:` directive and every unsupported directive.
- [ ] Decide whether to add raw HTML escape hatches. If yes, require explicit
      syntax and document security rules.
- [ ] Decide whether to add snippet/render syntax or keep slots only.
- [ ] Decide whether `head` management is part of `view {}` or page metadata.
- [ ] Decide whether document/window/body event targets belong in core.
- [ ] Decide whether transitions and animations are core, addon, or non-goal.
- [ ] Decide whether DOM actions/attachments are core, addon, or non-goal.
- [x] Add parser diagnostics for unsupported external-framework syntax that
      users are likely to try.
- [ ] Add golden tests for each supported markup construct.
- [x] Add negative tests for unsupported constructs with useful diagnostics.

Done when:

- [ ] `docs/language/markup.md` is an exact contract, not a direction document.
- [x] Unsupported familiar template syntax fails clearly.
- [ ] `gowdk fmt` preserves or normalizes the supported syntax predictably.

### 3. Component Contract

- [ ] Define inline prop types beyond `string`, or document why imported Go
      structs are the only typed prop path.
- [ ] Add prop defaults or document how users should express defaults in Go.
- [ ] Decide whether rest/spread props are allowed.
- [ ] Decide whether bindable child state is stable or experimental.
- [ ] Decide whether recursive components are supported, rejected, or deferred.
- [ ] Decide whether dynamic component selection is allowed.
- [ ] Define exported component values and how parent pages/components consume
      them.
- [ ] Define slot versus snippet terminology for GOWDK.
- [ ] Add examples for default slots, named slots, scoped slots, emits,
      bindable state, typed exports, stores, and WASM islands.
- [ ] Add component contract tests for each supported public feature.

Done when:

- [ ] A component author can predict prop, state, slot, event, store, CSS, and
      island behavior from docs alone.
- [ ] Component features have diagnostics that prevent accidental global lookup
      or ambiguous cross-package resolution.

### 4. Client Reactivity And Stores

- [ ] Write a short ADR explaining why GOWDK uses bounded `client {}` instead of
      arbitrary JavaScript or external framework reactivity.
- [ ] Document the supported client expression grammar in one place.
- [ ] Document computed dependency tracking and cycle diagnostics.
- [ ] Document update batching behavior.
- [ ] Document lifecycle and effect cleanup behavior.
- [ ] Define page store runtime semantics: initialization, subscription,
      mutation, isolation, serialization, and teardown.
- [ ] Decide whether stores can cross package boundaries.
- [ ] Decide whether client handlers can be async and what async means for
      update ordering.
- [ ] Add integration tests for multi-island store sharing.
- [ ] Add tests for effect ordering, cleanup, batching, and cycles.

Done when:

- [ ] The client language is intentionally bounded but coherent.
- [ ] Store behavior is stable enough to write examples without caveats.

### 5. CSS And Assets

- [ ] Decide the authoring syntax for component-scoped CSS.
- [ ] Decide how scoped selectors are rewritten and how specificity is handled.
- [ ] Decide how keyframes are scoped.
- [ ] Decide how global CSS escapes work.
- [ ] Decide how component-level assets are declared and emitted.
- [ ] Document the relationship between page CSS, component CSS, CSS processors,
      Tailwind, hashing, and asset manifests.
- [ ] Add tests for scoped CSS output, hashed filenames, manifest mappings, and
      generated binary cache headers.
- [ ] Add examples for plain CSS, scoped component CSS, Tailwind, and external
      CSS processors.

Done when:

- [ ] GOWDK has a clear CSS story that is not just "processor hooks exist".
- [ ] The docs no longer conflict on component CSS/assets status.

### 6. Load, Data, And Invalidation

- [ ] Define the difference between `build {}` and `load {}` in terms of cache,
      timing, request data, and generated output.
- [ ] Decide whether GOWDK needs a browser-side or shared load equivalent.
- [ ] Decide whether SSR `load {}` can call multiple functions or only the
      generated `Load<PageID>` convention.
- [ ] Define how layout load data composes with page load data.
- [ ] Define how action results can invalidate or refresh page data.
- [ ] Define whether partial fragments can declare data dependencies.
- [ ] Define whether generated client navigation can prefetch or reuse load
      data.
- [ ] Define redirect, not-found, forbidden, and validation behavior from load
      functions.
- [ ] Add generated typed data contracts or document the map-based contract.
- [ ] Add tests for parent/page load composition, redirects, errors, and action
      invalidation if implemented.

Done when:

- [ ] Users can choose `paths {}`, `build {}`, `load {}`, actions, APIs, and
      fragments without guessing when each runs.
- [ ] Post-action behavior is deterministic for full POSTs and enhanced POSTs.

### 7. Hybrid Rendering

- [ ] Define hybrid page modes precisely.
- [ ] Decide what request-time capabilities a hybrid page can use.
- [ ] Decide whether hybrid can prerender and later revalidate.
- [ ] Decide whether hybrid can stream or partially refresh server data.
- [ ] Define generated binary behavior for hybrid pages.
- [ ] Define dev-server behavior for hybrid pages.
- [ ] Add diagnostics for unsupported hybrid combinations.
- [ ] Add docs and examples for bare hybrid SPA output and request-time hybrid
      behavior.

Done when:

- [ ] `@render hybrid` is either product-stable or clearly experimental.
- [ ] Deployment docs do not say hybrid request-time behavior is both planned
      and implemented.

### 8. Hooks, Guards, And Middleware

- [ ] Define whether GOWDK has app-wide hooks or only generated registration
      points.
- [ ] Decide if hooks operate on `http.Handler`, `context.Context`, or a
      GOWDK-specific public contract.
- [ ] Define hook order relative to static serving, backend routing, guards,
      rate limiting, CSRF, action decoding, SSR load, and panic boundaries.
- [ ] Decide if hooks can rewrite routes.
- [ ] Decide if hooks can transform responses.
- [ ] Decide if hooks can intercept generated fetch/navigation requests.
- [ ] Expand guard result shape beyond error/nil if redirects or custom
      responses are needed.
- [ ] Add tests for ordering and failure behavior.

Done when:

- [ ] Auth/session/cookie examples can be written without importing generated
      app output from feature packages.
- [ ] Hook behavior is clear enough for production middleware.

### 9. Errors And Boundaries

- [ ] Define expected versus unexpected errors.
- [ ] Define user error types or response helpers for not found, forbidden,
      invalid request, conflict, validation, and redirect.
- [ ] Define page/layout-level error boundary syntax if supported.
- [ ] Define action/API/fragment error boundary syntax if supported.
- [ ] Define error response cache policy.
- [ ] Define what error details are logged versus rendered.
- [ ] Add generated binary tests for SSR load errors, action errors, API
      errors, guard errors, panic boundaries, and missing error documents.
- [ ] Add docs and examples for custom `404`, `500`, and route-level errors.

Done when:

- [ ] Production apps have a safe, documented error customization path.
- [ ] Panic values and submitted form values are not exposed in generated
      responses.

### 10. Forms, Actions, And Progressive Enhancement

- [ ] Define page-level form state if GOWDK needs it.
- [ ] Define action result shape for full-page POST, enhanced POST, JSON, and
      fragment responses.
- [ ] Define automatic invalidation after successful actions.
- [ ] Define redirect handling for enhanced actions.
- [ ] Define nearest error-boundary behavior for enhanced actions.
- [ ] Decide whether component-hidden fields can be inferred.
- [ ] Expand generated validation only where it remains request-shape
      validation, not domain validation.
- [ ] Keep file uploads intentionally user-owned unless a separate design is
      written.
- [ ] Add tests for full POST fallback, partial swaps, validation fragments,
      redirects, CSRF failures, oversized bodies, and handler errors.

Done when:

- [ ] A form works without JavaScript and upgrades predictably with generated
      JavaScript.
- [ ] Domain validation remains clearly owned by user Go handlers.

### 11. Routing And Typed Access

- [ ] Decide whether to support rest params.
- [ ] Decide whether to support optional params.
- [ ] Decide whether to support route groups without folder-derived route
      truth.
- [ ] Decide whether page routes and API endpoints can share method/path through
      content negotiation.
- [ ] Decide trailing slash policy and redirects.
- [ ] Generate typed route param structs or accessors for request-time routes.
- [ ] Generate typed endpoint metadata accessors if useful.
- [ ] Generate typed load/action data accessors if useful.
- [ ] Add tests for route conflicts, typed param decoding, invalid params, and
      generated accessor compatibility.

Done when:

- [ ] Typed route access is easier than reading `map[string]any` while keeping
      route declarations explicit.

### 12. Dev Experience

- [ ] Add browser error overlay for compiler/build failures.
- [ ] Add precise changed-file diagnostics in dev output.
- [ ] Decide whether component-level HMR is worth the complexity.
- [ ] Preserve local island state across updates if HMR is implemented.
- [ ] Add faster dependency tracking beyond polling where portable.
- [ ] Add generated app restart diagnostics and last-good-build behavior docs.
- [ ] Add `gowdk doctor` or equivalent environment validation if recurring
      setup failures appear.
- [ ] Improve scaffolded examples for common workflows.

Done when:

- [ ] `gowdk dev` feels reliable for real apps, even if it intentionally does
      not provide JavaScript bundler-style HMR feature-for-feature.

### 13. LSP And Editor Support

- [ ] Add hover support for annotations, directives, stores, props, routes, and
      handlers.
- [ ] Add go-to-definition for component calls.
- [ ] Add go-to-definition for Go handler symbols where possible.
- [ ] Add references for page IDs, routes, components, stores, and guards.
- [ ] Add semantic tokens for `.gwdk`.
- [ ] Add code actions for common migrations and missing imports/uses.
- [ ] Add project-wide completions for discovered components, layouts, guards,
      routes, stores, props, and state fields.
- [ ] Add tests for LSP requests beyond diagnostics/formatting/completion.

Done when:

- [ ] Editor support is useful without requiring the VS Code extension to patch
      over missing LSP behavior.

### 14. Scaffolding, Testing, And Examples

- [ ] Add template selection to `gowdk init`.
- [ ] Add example apps for static site, actions, login/auth, dashboard SSR,
      API-only backend, partial updates, Tailwind, and split frontend/backend.
- [ ] Add optional generated test scaffold.
- [ ] Add Playwright or browser smoke-test guidance for generated apps.
- [ ] Add accessibility test guidance.
- [ ] Add performance smoke-test guidance.
- [ ] Add migration examples for existing Go applications.
- [ ] Add "web framework concepts for GOWDK users" documentation.

Done when:

- [ ] A new user can scaffold, build, test, and deploy a small app without
      reading source code.

### 15. Deployment And Ecosystem

- [ ] Keep the single-binary deploy as the primary differentiator.
- [ ] Add Docker deployment guidance.
- [ ] Add systemd deployment guidance.
- [ ] Add reverse proxy examples for Caddy and nginx.
- [ ] Add CDN/static hosting guidance for `gowdk build --out`.
- [ ] Add Cloudflare deployment guidance if compatible with the binary/WASM
      story.
- [ ] Add Vercel/Netlify static deployment guidance if useful for build-output
      files.
- [ ] Add Kubernetes guidance only if real users need it.
- [ ] Define addon discovery and versioning expectations.
- [ ] Define whether third-party addon packages are loaded through Go imports,
      CLI discovery, or project config only.

Done when:

- [ ] Deployment docs cover the common paths without forcing platform-specific
      code into compiler/runtime core.

## Intentional Differences To Preserve

- [ ] Do not make filesystem route placement the source of truth.
- [ ] Do not make full-page SSR the default rendering model.
- [ ] Do not require npm, Vite, Tailwind, Svelte, or a JavaScript framework for
      normal app flows.
- [ ] Do not move domain logic, persistence, auth, or business validation into
      generated code.
- [ ] Do not make generated JavaScript the authority for routes, auth, server
      validation, or cache policy.
- [ ] Do not make WASM islands the default component runtime.
- [ ] Do not add platform adapters to core if normal Go deployment or docs can
      solve the need.

## Verification Matrix

- [ ] General repository verification: `go test ./...`.
- [ ] CLI build verification: `go build ./cmd/gowdk`.
- [ ] Formatting verification for changed Go files: `gofmt -w <files>`.
- [ ] Example syntax verification:
      `go run ./cmd/gowdk check --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk`.
- [ ] Login example verification: `cd examples/login && make check`.
- [ ] Login build verification: `cd examples/login && make build`.
- [ ] Split build verification: `cd examples/login && make split-build`.
- [ ] VS Code extension syntax verification:
      `node --check editors/vscode/extension.js`.
- [ ] VS Code extension core syntax verification:
      `node --check editors/vscode/extension-core.js`.
- [ ] VS Code extension tests: `node --test editors/vscode/*.test.js`.

## Documentation Cleanup Checklist

- [ ] Reconcile `MISSING_CHECKLIST.md` with `docs/product/requirements.md`.
- [ ] Reconcile `MISSING_CHECKLIST.md` with `docs/product/roadmap.md`.
- [ ] Reconcile `MISSING_CHECKLIST.md` with `docs/engineering/architecture.md`.
- [ ] Reconcile generated binary support descriptions across
      `docs/reference/cli.md`, `docs/reference/deployment.md`,
      `docs/getting-started.md`, and `README.md`.
- [ ] Reconcile SSR `load {}` support descriptions across language,
      deployment, CLI, architecture, and release docs.
- [ ] Reconcile component CSS/assets status across `docs/language/components.md`,
      `docs/reference/css.md`, requirements, and the missing checklist.
- [ ] Reconcile WASM island validation status across component docs,
      requirements, ADR 0004, generated-output docs, and examples.
- [ ] Replace stale "first-slice" wording where behavior is now product-stable.
- [ ] Keep "first-slice" wording where behavior is genuinely narrow.
- [ ] Add a single status legend used consistently across product docs:
      implemented, partial, experimental, planned, and intentionally out of
      scope.

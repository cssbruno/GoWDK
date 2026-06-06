# GOWDK Gap Checklist

This checklist tracks gaps that still need product, compiler, runtime, tooling,
documentation, or ecosystem decisions before GOWDK can be presented as a
polished Go-first full web app platform. Some gaps were found by comparing
GOWDK with Svelte and SvelteKit, but this document is not a request to clone
their product model or implement their full feature set. Use it to decide which
comparison points need a GOWDK-native answer, which should be deferred, and
which should be rejected as intentional non-goals.

Comparison anchors, not implementation requirements:

- Svelte 5 component/compiler surface: runes, broad template syntax, scoped
  styles, stores/context, compiler diagnostics, and generated component output.
- SvelteKit app framework surface: routing, load/data lifecycle, form actions,
  hooks, errors, page options, adapters, tooling, and deployment ecosystem.
- GOWDK product rules still win where they intentionally differ: explicit
  routes, build-time pages by default, Go-owned behavior, generated Go adapter
  glue, optional SSR, and one-binary deploys.

## How To Use This Checklist

- Keep checked implementation items limited to behavior that is implemented,
  documented, and covered by tests or an explicit verification command. For
  decision gaps, a checked item means the product decision is recorded and any
  remaining implementation work is still tracked by the relevant acceptance
  criteria or detailed workstream.
- Keep intentional differences visible. If GOWDK deliberately chooses not to
  provide an equivalent for a Svelte/SvelteKit feature, move the item to the
  intentional non-goals section with the reason.
- Prefer completing contract work before feature breadth. A narrow, stable Go
  contract is better than a wide feature surface that cannot be explained or
  generated safely.
- Update `docs/product/requirements.md`, `docs/product/roadmap.md`,
  `MISSING_CHECKLIST.md`, and this checklist when an item changes status.

Status legend:

- [x] Implemented and documented, or product decision recorded for decision
      gaps.
- [ ] Missing, incomplete, or not yet product-stable.
- Implemented: available in the current codebase, documented, and covered by
      tests or an explicit verification command.
- Partial: available for a narrower slice than the full requirement, with
      remaining limits called out near the checklist item or requirement.
- Experimental: implemented enough to try, but not yet stable API.
- Planned: accepted product direction with no stable implementation yet.
- Intentionally out of scope: rejected for the current product direction.

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

- [x] Decide the next GOWDK-native `view {}` language expansion.
      Comparison pressure comes from broad template features such as spread
      attributes, keyed blocks, async placeholders, snippets/renderable markup,
      raw HTML escape hatches, local constants, debug helpers, transitions,
      animations, DOM actions, and document/window/body/head targets. Do not
      implement these one-for-one unless they fit GOWDK's compiler-owned model.
      Decision: expand `view {}` only through GOWDK-owned directives and AST
      nodes. Prioritize exact markup contract docs, keyed `g:for`, explicit
      head metadata, and accessible diagnostics. Defer raw HTML, async
      placeholders, transitions, document/window/body targets, and DOM actions
      until each has a written security/runtime contract.
- [x] Define whether GOWDK wants a first-class snippet/render equivalent.
      Current default, named, and scalar scoped slots cover component insertion
      but not reusable typed markup values with parameters and lexical scope.
      Decision: keep slots as the stable primitive for now. Do not add a
      first-class snippet/render value model in P0; revisit as a P2 language
      feature only if slots cannot express real component reuse.
- [x] Broaden the component props contract.
      Missing or weaker areas include non-string inline props, defaults,
      rest/spread props, prop renaming, recursive component ergonomics, dynamic
      component selection, and a clear bindable-prop equivalent.
      Decision: imported Go structs remain the primary typed prop path. Add
      non-string literal props and documented defaults next. Defer rest/spread,
      prop renaming, recursive components, dynamic component selection, and
      bindable child state until diagnostics can keep cross-package resolution
      explicit and predictable.
- [x] Decide the long-term GOWDK reactivity model.
      The comparison point is Svelte 5 runes, but GOWDK should preserve a
      bounded compiler-owned `client {}` language unless a stronger Go-first
      model is explicitly designed.
      Decision: keep a bounded compiler-owned `client {}` language. Do not
      allow arbitrary JavaScript, external framework reactivity, or generated JS
      ownership of app routing, auth, business rules, database access, server
      validation, action behavior, global app state, or page loading policy.
- [x] Define a stronger shared-state/store runtime.
      Page stores exist, but they still need a stable GOWDK contract for shared
      island state, subscriptions, isolation, serialization, and teardown.
      Decision: shared state is page/island scoped by default. Cross-package or
      app-global stores are deferred until there is an explicit ownership,
      serialization, subscription, and teardown contract. User Go remains the
      source of truth for trusted server state.
- [x] Complete the GOWDK load/data lifecycle decision.
      Comparison pressure comes from server versus universal load, enhanced
      fetch, parent data, dependency tracking, client invalidation,
      post-action load reruns, serialized fetch results, and streaming data.
      Implement only the parts that serve GOWDK's build-time, endpoint, and SSR
      lanes.
      Decision: `build {}` is build-time data, `load {}` is request-time data,
      and actions/APIs/fragments are endpoint lanes. Do not add universal load
      or browser-owned load policy in P0. Post-action invalidation and
      prefetch/reuse are planned as explicit generated-client features, not
      hidden framework behavior.
- [x] Complete hybrid request-time behavior.
      GOWDK currently treats bare hybrid pages as SPA output and supports the
      explicit `load {}` request-time branch, but broader hybrid behavior such
      as streaming, revalidation, and partial server data refresh still needs
      product decisions.
      Decision: hybrid means SPA output by default plus explicit request-time
      capabilities. The only product-stable request-time hybrid branch is
      `load {}` through the generated request-time page path. Streaming,
      revalidation beyond HTTP headers, and partial server data refresh are
      deferred.
- [x] Add a general app-wide hook/middleware contract.
      Current guards and rate-limit registration are useful but narrower than
      app-wide request hooks, response transforms, fetch interception, error
      hooks, rerouting, transport hooks, and init hooks.
      Decision: core hooks should compose as `net/http` middleware around the
      generated app handler plus explicit generated registration points for
      guards, rate limits, and future app init. Do not add route rewriting,
      fetch interception, or framework-specific hooks to core P0.
- [x] Add route-scoped error-boundary syntax and behavior.
      Current panic boundaries plus optional `404.html`/`500.html` are weaker
      than route/page/layout error boundaries and expected versus unexpected
      error handling.
      Decision: keep current `@error` syntax for route-local SSR and
      endpoint-local action/API boundaries. Expected error types and
      layout-level boundaries are planned P1/P2 work; panic details and form
      values must never be rendered.
- [x] Upgrade the dev server from live reload toward component-aware hot
      updates where appropriate.
      Missing or weaker areas include state-preserving component updates,
      browser error overlay, module/dependency graph, and precise update
      invalidation.
      Decision: keep dependency-free live reload as the P0 dev baseline.
      Component-aware HMR is deferred until the component/client runtime has a
      stable dependency graph. Add browser error overlay before state-preserving
      HMR.
- [x] Fix documentation consistency before public positioning.
      Several docs still describe first-slice or planned limitations that newer
      checklists mark complete. Align README, requirements, roadmap,
      deployment, CLI, language, release, and missing-work docs.
      Decision: product docs use the shared status legend from
      `docs/product/requirements.md`. Public positioning stays pre-release
      until production readiness and release binaries are true.
- [x] Publish a release-readiness statement grounded in current behavior.
      README still says no release binary and not production-ready; release
      docs and current implementation status need a single consistent story.
      Decision: current release-readiness statement is pre-release,
      source-built, and not production-ready. Do not publish a production
      readiness claim until release artifacts, operations guidance, and the
      remaining P0/P1 contracts are implemented and verified.

## P0 Acceptance Criteria

- [x] Every P0 item has a product decision: implement, defer, or intentional
      non-goal.
- [ ] Every implemented P0 item has language docs, reference docs, examples,
      and tests.
- [ ] Every deferred P0 item has a diagnostic or explicit documentation entry
      that tells users what is unsupported.
- [x] The public README, requirements, roadmap, and missing checklist use the
      same status language.
- [ ] `go test ./...` and `go build ./cmd/gowdk` pass after each implementation
      slice.

## P1 Gaps

These are important product gaps once the P0 contracts are clear.

- [x] Expand routing expressiveness where it fits GOWDK's explicit-route model.
      Candidate gaps: rest params, optional params, route groups, configurable
      trailing slash policy, and page/API same-path content negotiation.
      Decision: keep explicit `.gwdk` route declarations as source of truth.
      Add rest params and trailing-slash policy first. Defer optional params,
      route groups, and same-path page/API content negotiation until conflict
      diagnostics and generated route metadata can describe them exactly.
- [x] Generate stronger route-specific typed APIs for user Go.
      Current typed params are runtime-map based. Consider generated typed
      structs/accessors for params, load data, action data, endpoint metadata,
      and page contracts.
      Decision: generate typed route-param accessors next. Typed load/action
      data accessors are planned only after the load/action result contracts are
      stable. Application code should import public `runtime/` or addon
      packages, not generated app output.
- [x] Add a GOWDK page form lifecycle where it fits the product.
      Missing or weaker areas include page-level form state, status exposure,
      automatic data invalidation after actions, redirect/error behavior for
      enhanced forms, and nearest error-boundary handling.
      Decision: page form lifecycle stays progressive-enhancement-first. Full
      POST fallback and enhanced POST must share action result semantics.
      Automatic invalidation, enhanced redirects, and nearest boundary behavior
      are planned generated-client features; domain validation remains user Go.
- [x] Broaden API endpoint support.
      Candidate gaps: richer body/query helpers, method fallback policy,
      response error shape, Accept/content negotiation, and route-page sharing.
      Decision: broaden APIs through public response/request helpers and typed
      query/body helpers, not through framework-specific adapters. Method
      fallback and error response shape should be explicit. Accept/content
      negotiation and page/API same-path sharing are deferred.
- [x] Make cache and revalidation more than response headers if needed.
      `@cache` and `@revalidate` currently map to HTTP cache headers. Missing
      or weaker areas include data dependency invalidation, prerender crawling,
      dynamic entry generation, and runtime reload policy.
      Decision: keep `@cache` and `@revalidate` as HTTP cache policy in P1.
      Data dependency invalidation, prerender crawling, dynamic entry
      generation, and runtime reload policy are deferred until load/action
      invalidation contracts exist.
- [x] Make guards more expressive.
      Current guards are nil/error and generally map to HTTP 403. Consider
      guard-level redirect/response contracts and richer request-local state.
      Decision: extend guards to allow safe local redirects and explicit
      response helpers before adding richer request-local state. Guards must
      remain public-addon/runtime contracts and run before user endpoint or SSR
      logic.
- [x] Create a clear component CSS authoring model.
      GOWDK has CSS processors, page CSS, hashing, and scope metadata, but does
      not yet have an obvious component-local authoring model with default
      scoping, specificity rules, keyframes behavior, and docs.
      Decision: component-local CSS should be explicit, compiler-scoped, and
      documented around selector rewriting, keyframes, global escapes, asset
      emission, hashing, and binary cache headers. Tailwind and processors stay
      optional.
- [x] Add accessibility diagnostics.
      Candidate warnings: missing `alt`, invalid ARIA, clickable elements
      without keyboard support, label/control mismatch, empty headings, empty
      links, invalid autofocus patterns, and role misuse.
      Decision: add accessibility diagnostics as compiler warnings with stable
      codes and source spans. Start with missing `alt`, label/control mismatch,
      empty headings/links, clickable-without-keyboard, and invalid autofocus.
- [x] Improve compiler diagnostics and parser recovery.
      GOWDK has useful diagnostics, spans, suggestions, formatting, and LSP,
      but the comparison shows a need for a larger warning/error catalogue and
      broader recovery behavior.
      Decision: expand the diagnostic catalogue before broad parser recovery.
      Recovery should never hide invalid route, handler, security, or generated
      adapter contracts. LSP should consume the same diagnostics.
- [x] Improve LSP/editor features.
      Candidate gaps: hover, go-to-definition, references, semantic tokens,
      code actions, workspace-wide component/type intelligence, exact source
      ranges, and generated route/type navigation.
      Decision: prioritize hover, semantic tokens, go-to-definition for
      component/use/handler symbols, and route/type navigation. Code actions
      come after diagnostics have stable codes and suggestions.
- [x] Add app testing scaffolds.
      Candidate outputs: Go handler tests, generated app smoke tests,
      Playwright/browser E2E, accessibility checks, and component/island test
      harnesses for scaffolded apps.
      Decision: add optional scaffolded Go handler tests and generated app
      smoke tests first. Browser E2E, accessibility checks, and island harnesses
      should be docs/templates, not mandatory core dependencies.
- [x] Improve scaffolding beyond `gowdk init`.
      Candidate gaps: interactive templates, add-on selection, auth/db/test
      setup, migration commands, generated examples, and playground-to-project
      export.
      Decision: improve `gowdk init` with explicit template selection and addon
      selection. Auth/db/test setup should generate editable Go and `.gwdk`
      files. Migration commands and playground export are deferred.
- [x] Add platform deployment adapters or generators where they do not fight
      the Go-first model.
      Candidate targets: static hosts, Docker, Fly.io, Render, Kubernetes,
      Cloudflare, Vercel, Netlify, systemd, and CDN cache examples.
      Decision: prefer documentation and optional generators over runtime core
      adapters. Add static host, Docker, systemd, Caddy/nginx, and CDN guidance
      first. Kubernetes, Vercel/Netlify, and Cloudflare stay docs-only unless
      real deploy constraints require generated files.
- [x] Document production operations as first-class guidance.
      Include secrets, CSRF secret rotation, reverse proxies, cache/CDN policy,
      health checks, metrics, logging, binary deployment, and rollback.
      Decision: production operations guidance is required before any
      production-ready claim. Cover secrets, CSRF secret rotation, reverse
      proxies, cache/CDN policy, health checks, metrics, logging, binary
      deployment, and rollback as docs first.

## P1 Acceptance Criteria

- [x] Routing, form, API, cache, guard, CSS, LSP, testing, scaffold, and
      deployment decisions are each represented in requirements and roadmap.
- [ ] User-facing gaps have examples or clear unsupported diagnostics.
- [ ] New generated contracts are stable enough for application code to import
      only public `runtime/` or addon packages, never generated output.
- [ ] Docs include migration guidance for apps built against the previous
      first-slice behavior.

## P2 Gaps

These are polish, ecosystem, or expansion gaps.

- [x] Decide whether full-page hydration is a goal.
      GOWDK currently favors static pages, partial updates, and islands rather
      than full component JavaScript output and hydration.
      Decision: full-page hydration is not a core goal. GOWDK keeps static
      pages, progressive form enhancement, server fragments, and explicit
      islands as the browser model. Revisit full-page hydration only as an
      optional addon if it can avoid owning routing, auth, validation, app
      state, and loading policy.
- [x] Expand generated JavaScript island ergonomics while preserving the bounded
      compiler-owned client language.
      Decision: improve islands through compiler-owned ergonomics such as
      clearer event syntax, lifecycle cleanup, focus helpers, local state
      batching, and diagnostics. Do not expose arbitrary JavaScript as the
      public app contract.
- [x] Expand client built-ins where they matter for real apps.
      Candidate areas: date/time formatting, collection helpers, string/number
      formatting, async-safe patterns, and focus/selection helpers.
      Decision: add a small documented standard library for formatting,
      collection transforms, async-safe UI patterns, and focus/selection only
      when each builtin has deterministic generated output and tests.
- [x] Improve WASM island validation and examples.
      Keep browser-side Go logic explicit and separate from backend handlers.
      Decision: WASM islands remain explicit browser-side Go, never backend
      handler reuse. Improve validation, ABI docs, and examples before adding
      broader WASM ergonomics.
- [x] Add service worker and offline/PWA guidance if it fits the product.
      Decision: keep service worker and PWA support documentation-first and
      optional. Do not make offline behavior, cache policy, or service workers a
      hidden generated default.
- [x] Add image optimization guidance or integrations.
      Decision: document image optimization patterns first. Optional
      integrations may emit assets or metadata, but core should not become an
      image pipeline unless real projects need it.
- [x] Add package/addon discovery.
      This does not need to mirror any JavaScript package ecosystem, but users
      need a way to find official and community GOWDK addons.
      Decision: addon discovery should be docs or registry metadata owned by
      the repository/website first. Add CLI discovery only after addon
      versioning, trust, and compatibility rules are documented.
- [x] Add public playground onboarding if the playground is part of product
      strategy.
      Candidate gaps: hosted playground, tutorial integration, shareable
      examples, export to project, and import from repository.
      Decision: playground ownership is the website/docs surface first, with
      CLI project export later. Hosted execution must remain sandboxed and
      optional; local compiler/runtime behavior must not depend on it.
- [x] Add performance profiling documentation.
      Include build time, generated app size, generated JS size, SSR latency,
      action latency, binary size, and cache behavior.
      Decision: add profiling docs around build time, output size, generated JS
      size, SSR/action latency, binary size, cache behavior, and measurement
      commands before adding automated profiling features.
- [x] Add migration/onboarding guides for likely audiences.
      Candidate guides: Go templates to GOWDK, htmx-style apps to GOWDK,
      JavaScript-framework concepts to GOWDK, and static Go sites to GOWDK.
      Decision: publish docs-first migration guides for Go templates,
      htmx-style apps, JavaScript-framework concepts, and static Go sites.
      These guides must explain GOWDK differences instead of translating
      external framework concepts one-for-one.

## P2 Acceptance Criteria

- [x] Each P2 item has a product rationale before implementation starts.
- [x] Optional ecosystem features remain optional and do not add mandatory npm,
      framework, or platform dependencies to core.
- [x] Playground, package discovery, and migration work has a documented owner
      surface: CLI, docs, website, or repository examples.

## Detailed Workstreams

Use these workstreams to turn the high-level gaps into reviewable slices.

### 1. Documentation Truth Pass

- [x] Choose one source-of-truth status model for all product docs.
- [x] Add a status legend to `docs/product/requirements.md`.
- [x] Add this product gap checklist under `docs/product/` while preserving
      `MISSING_CHECKLIST.md` as the detailed implementation-status checklist.
- [x] Update `docs/product/roadmap.md` so completed slices are not described as
      future work.
- [x] Update `docs/engineering/architecture.md` so current baseline and target
      architecture are separated clearly.
- [x] Update `docs/reference/cli.md` for actual generated-binary behavior.
- [x] Update `docs/reference/deployment.md` for actual SSR, action, API,
      fragment, and hybrid limits.
- [x] Update `docs/language/ssr.md` for exact `load {}` support and limits.
- [x] Update `docs/language/components.md` for actual store, CSS, scoped slot,
      export, and WASM island status.
- [x] Update `docs/compiler/generated-output.md` for actual generated app,
      island, partial, SSR, and source-map behavior.
- [x] Remove stale "first-slice" wording where behavior is now stable.
- [x] Keep "first-slice" wording where the supported language or runtime
      surface is intentionally narrow.
- [x] Add a short comparison note that explains intentional product differences
      without turning external frameworks into the roadmap.

Done when:

- [x] A reader can tell what works today without checking source code.
- [x] No major feature is simultaneously marked implemented and planned in
      different docs.
- [x] The README, requirements, roadmap, and missing checklist agree.

### 2. Template Language Contract

- [x] Inventory all currently parsed markup syntax from `internal/view`.
- [x] Document every supported `g:` directive and every unsupported directive.
- [x] Decide whether to add raw HTML escape hatches. If yes, require explicit
      syntax and document security rules.
- [x] Decide whether to add snippet/render syntax or keep slots only.
- [x] Decide whether `head` management is part of `view {}` or page metadata.
- [x] Decide whether document/window/body event targets belong in core.
- [x] Decide whether transitions and animations are core, addon, or non-goal.
- [x] Decide whether DOM actions/attachments are core, addon, or non-goal.
- [x] Add parser diagnostics for unsupported external-framework syntax that
      users are likely to try.
- [ ] Add golden tests for each supported markup construct.
- [x] Add negative tests for unsupported constructs with useful diagnostics.

Done when:

- [x] `docs/language/markup.md` is an exact contract, not a direction document.
- [x] Unsupported familiar template syntax fails clearly.
- [ ] `gowdk fmt` preserves or normalizes the supported syntax predictably.

### 3. Component Contract

- [x] Define inline prop types beyond `string`, or document why imported Go
      structs are the only typed prop path.
- [x] Add prop defaults or document how users should express defaults in Go.
- [x] Decide whether rest/spread props are allowed.
- [x] Decide whether bindable child state is stable or experimental.
- [x] Decide whether recursive components are supported, rejected, or deferred.
- [x] Decide whether dynamic component selection is allowed.
- [x] Define exported component values and how parent pages/components consume
      them.
- [x] Define slot versus snippet terminology for GOWDK.
- [ ] Add examples for default slots, named slots, scoped slots, emits,
      bindable state, typed exports, stores, and WASM islands.
- [ ] Add component contract tests for each supported public feature.

Done when:

- [ ] A component author can predict prop, state, slot, event, store, CSS, and
      island behavior from docs alone.
- [ ] Component features have diagnostics that prevent accidental global lookup
      or ambiguous cross-package resolution.

### 4. Client Reactivity And Stores

- [x] Write a short ADR explaining why GOWDK uses bounded `client {}` instead of
      arbitrary JavaScript or external framework reactivity.
- [x] Document the supported client expression grammar in one place.
- [x] Document computed dependency tracking and cycle diagnostics.
- [x] Document update batching behavior.
- [x] Document lifecycle and effect cleanup behavior.
- [x] Define page store runtime semantics: initialization, subscription,
      mutation, isolation, serialization, and teardown.
- [x] Decide whether stores can cross package boundaries.
- [x] Decide whether client handlers can be async and what async means for
      update ordering.
- [ ] Add integration tests for multi-island store sharing.
- [ ] Add tests for effect ordering, cleanup, batching, and cycles.

Done when:

- [x] The client language is intentionally bounded but coherent.
- [x] Store behavior is stable enough to write examples without caveats.

### 5. CSS And Assets

- [x] Decide the authoring syntax for component-scoped CSS.
      Decision: component CSS uses explicit component-local
      `@css "<relative.css>"` annotations. The path is relative to the
      component source file. Inline style blocks are not the current contract.
- [x] Decide how scoped selectors are rewritten and how specificity is handled.
      Decision: emitted component CSS is scoped by default. The generated
      scope marker comes from the component CSS scope ID and is attached to
      compiler-owned component output. Selector rewriting should avoid
      specificity surprises and use `:where(...)` around the scope marker when
      browser support allows it.
- [x] Decide how keyframes are scoped.
      Decision: local `@keyframes` names are rewritten with the component scope
      ID, and local `animation` or `animation-name` references are rewritten to
      the scoped keyframe name.
- [x] Decide how global CSS escapes work.
      Decision: component CSS does not leak global selectors implicitly. Use
      page/global CSS for app-wide styles. A future explicit `:global(...)`
      escape may be added, but implicit global component selectors are not part
      of the contract.
- [x] Decide how component-level assets are declared and emitted.
      Decision: component assets use explicit component-local
      `@asset "<relative-path>"` annotations. The metadata path exists today;
      emitted assets should be content-hashed, manifest-mapped, and served with
      immutable generated binary cache headers when output support is added.
- [x] Document the relationship between page CSS, component CSS, CSS processors,
      Tailwind, hashing, and asset manifests.
- [ ] Add tests for scoped CSS output, hashed filenames, manifest mappings, and
      generated binary cache headers.
- [ ] Add examples for plain CSS, scoped component CSS, Tailwind, and external
      CSS processors.

Done when:

- [x] GOWDK has a clear CSS story that is not just "processor hooks exist".
- [x] The docs no longer conflict on component CSS/assets status.

### 6. Load, Data, And Invalidation

- [x] Define the difference between `build {}` and `load {}` in terms of cache,
      timing, request data, and generated output.
      Decision: `build {}` is build-time static page data and must not depend
      on an HTTP request. `load {}` is request-time page data and requires SSR
      or an explicit hybrid request-time branch. Request-time responses stay
      no-store unless a page cache policy explicitly applies to successful SSR
      HTML.
- [x] Decide whether GOWDK needs a browser-side or shared load equivalent.
      Decision: no browser-owned or universal load in the current contract.
      Future prefetch or reuse must be an explicit generated-client feature.
- [x] Decide whether SSR `load {}` can call multiple functions or only the
      generated `Load<PageID>` convention.
      Decision: one generated same-package `Load<PageID>` function returns the
      declared field map. Multiple load fields come from that single return
      value, including dotted paths.
- [x] Define how layout load data composes with page load data.
      Decision: layouts do not have independent `load {}` data today.
      Request-time layout data composition is planned work.
- [x] Define how action results can invalidate or refresh page data.
      Decision: actions own redirects, HTML, JSON, and fragments through the
      returned Go `response.Response`. GOWDK does not automatically rerun
      `load {}` after an action today.
- [x] Define whether partial fragments can declare data dependencies.
      Decision: no compiler-tracked fragment data dependencies today. Fragment
      Go hooks own request-time fragment data.
- [x] Define whether generated client navigation can prefetch or reuse load
      data.
      Decision: generated client navigation does not prefetch or reuse
      `load {}` data today.
- [x] Define redirect, not-found, forbidden, and validation behavior from load
      functions.
      Decision: load redirects use local `ssr.RedirectTo` or `ssr.Redirect`.
      Guards handle guarded access. Not-found, forbidden, validation, and typed
      expected-error helpers are planned; other load errors use generated SSR
      error-page handling.
- [x] Add generated typed data contracts or document the map-based contract.
      Decision: current load data is `map[string]any`; typed load accessors are
      planned after the load result contract is stable.
- [ ] Add tests for parent/page load composition, redirects, errors, and action
      invalidation if implemented.

Done when:

- [x] Users can choose `paths {}`, `build {}`, `load {}`, actions, APIs, and
      fragments without guessing when each runs.
- [x] Post-action behavior is deterministic for full POSTs and enhanced POSTs.

### 7. Hybrid Rendering

- [x] Define hybrid page modes precisely.
      Decision: bare `@render hybrid` is build-time SPA output. `@render
      hybrid` with `load {}` is the only stable request-time hybrid page mode.
- [x] Decide what request-time capabilities a hybrid page can use.
      Decision: the stable request-time capability is `load {}` through the
      generated load/render path with SSR enabled. Actions, APIs, and fragments
      remain endpoint lanes, not hybrid page route behavior.
- [x] Decide whether hybrid can prerender and later revalidate.
      Decision: bare hybrid pages prerender as SPA output. Hybrid pages with
      `load {}` are request-time routes. `@cache` and `@revalidate` remain HTTP
      response cache policy; they do not add background regeneration.
- [x] Decide whether hybrid can stream or partially refresh server data.
      Decision: no hybrid streaming or implicit server-data refresh today. Use
      explicit fragments for partial updates.
- [x] Define generated binary behavior for hybrid pages.
- [x] Define dev-server behavior for hybrid pages.
      Decision: plain `gowdk dev` serves bare hybrid SPA output. Hybrid pages
      with `load {}` need SSR enabled and generated app dev output, such as
      `gowdk dev --ssr --app <dir>`.
- [x] Add diagnostics for unsupported hybrid combinations.
      Decision: SPA `load {}` is rejected, hybrid `load {}` requires the SSR
      feature gate, and bare hybrid routes report request-time rendering as
      disabled in route info.
- [x] Add docs and examples for bare hybrid SPA output and request-time hybrid
      behavior.

Done when:

- [x] `@render hybrid` is either product-stable or clearly experimental.
- [x] Deployment docs do not say hybrid request-time behavior is both planned
      and implemented.

### 8. Hooks, Guards, And Middleware

- [x] Define whether GOWDK has app-wide hooks or only generated registration
      points.
      Decision: app-wide middleware is normal Go `http.Handler` wrapping around
      the generated app handler. Generated registration points exist for guards
      and rate limiting.
- [x] Decide if hooks operate on `http.Handler`, `context.Context`, or a
      GOWDK-specific public contract.
      Decision: middleware uses `http.Handler`; user handlers use
      `context.Context` plus public `runtime/app` helpers. Do not add a custom
      GOWDK context type.
- [x] Define hook order relative to static serving, backend routing, guards,
      rate limiting, CSRF, action decoding, SSR load, and panic boundaries.
      Decision: generated request-time routes attach metadata, install panic
      boundaries where supported, run rate limiting, run guards, run action
      CSRF, decode action input, then call user handlers or SSR load/render.
- [x] Decide if hooks can rewrite routes.
      Decision: no generated route rewriting hook in the current contract.
- [x] Decide if hooks can transform responses.
      Decision: no generated response transform hook in the current contract.
      Wrap the generated `http.Handler` with normal Go middleware when needed.
- [x] Decide if hooks can intercept generated fetch/navigation requests.
      Decision: no generated fetch/navigation interception hook in core.
- [x] Expand guard result shape beyond error/nil if redirects or custom
      responses are needed.
      Decision: guard results remain `nil` or `error` today. Redirect/custom
      response guard results are planned work.
- [ ] Add tests for ordering and failure behavior.

Done when:

- [x] Auth/session/cookie examples can be written without importing generated
      app output from feature packages.
- [x] Hook behavior is clear enough for production middleware.

### 9. Errors And Boundaries

- [x] Define expected versus unexpected errors.
      Decision: expected outcomes are user-owned `runtime/response.Response`
      values, `response.NewHandlerError` values, and SSR local redirect errors.
      Unexpected generated-lane failures are panic boundaries, generated
      request-shape failures, and missing generated resources.
- [x] Define user error types or response helpers for not found, forbidden,
      invalid request, conflict, validation, and redirect.
      Decision: use `response.Response` helpers for explicit status/body,
      fragment, JSON, validation, and redirect outcomes. Use
      `response.NewHandlerError` when an action/API handler must return an
      error with an explicit HTTP status. Dedicated named helpers remain planned
      only if repeated app code proves they are needed.
- [x] Define page/layout-level error boundary syntax if supported.
      Decision: SSR pages support route-local `@error`; layout-level and
      component-level boundaries are not supported today.
- [x] Define action/API/fragment error boundary syntax if supported.
      Decision: `act` and `api` support endpoint-local `@error` for generated
      panic boundaries. Fragment-specific boundary syntax is not supported.
- [x] Define error response cache policy.
      Decision: generated error responses use `Cache-Control: no-store`.
- [x] Define what error details are logged versus rendered.
      Decision: generated panic boundaries render safe fixed messages or
      generated HTML documents, not panic values. Generated form/request-shape
      errors do not echo submitted values. Returned handler error messages are
      user-owned and must stay safe.
- [ ] Add generated binary tests for SSR load errors, action errors, API
      errors, guard errors, panic boundaries, and missing error documents.
- [x] Add docs and examples for custom `404`, `500`, and route-level errors.
      See `docs/reference/errors.md`.

Done when:

- [x] Production apps have a safe, documented error customization path.
- [x] Panic values and submitted form values are not exposed in generated
      responses.

### 10. Forms, Actions, And Progressive Enhancement

- [x] Define page-level form state if GOWDK needs it.
      Decision: no generated page-level form state object today. Submitted
      data, handler responses, redirects, and fragments remain the source of
      truth.
- [x] Define action result shape for full-page POST, enhanced POST, JSON, and
      fragment responses.
      Decision: all action outcomes use `runtime/response.Response` helpers.
      Enhanced partial requests should return fragment responses for their
      target.
- [x] Define automatic invalidation after successful actions.
      Decision: no automatic invalidation today. Use full-page redirect,
      fragment response, JSON, or app-owned reload policy.
- [x] Define redirect handling for enhanced actions.
      Decision: full-page POST redirects are supported. Enhanced redirects are
      not a stable contract; enhanced requests should return fragments.
- [x] Define nearest error-boundary behavior for enhanced actions.
      Decision: no nearest error-boundary lookup today. Failed enhanced
      requests dispatch `gowdk:request-error`; validation fragments can target
      an explicit container.
- [x] Decide whether component-hidden fields can be inferred.
      Decision: not today. Generated inference reads direct literal controls in
      the page view.
- [x] Expand generated validation only where it remains request-shape
      validation, not domain validation.
      Decision: keep generated validation to literal request-shape constraints.
      Domain validation remains Go handler logic.
- [x] Keep file uploads intentionally user-owned unless a separate design is
      written.
      Decision: generated action extraction rejects file inputs and multipart
      forms. Uploads belong in user-owned Go handlers.
- [ ] Add tests for full POST fallback, partial swaps, validation fragments,
      redirects, CSRF failures, oversized bodies, and handler errors.

Done when:

- [x] A form works without JavaScript and upgrades predictably with generated
      JavaScript.
- [x] Domain validation remains clearly owned by user Go handlers.

### 11. Routing And Typed Access

- [x] Decide whether to support rest params.
      Decision: not supported today. Add only after exact, dynamic, static SPA,
      and SSR route precedence remains predictable.
- [x] Decide whether to support optional params.
      Decision: defer. Optional params create route-shape ambiguity and are not
      part of the current explicit route contract.
- [x] Decide whether to support route groups without folder-derived route
      truth.
      Decision: defer. `@route` remains the route source of truth; file/folder
      groups must not change URL shape implicitly.
- [x] Decide whether page routes and API endpoints can share method/path through
      content negotiation.
      Decision: no same-method page/API content negotiation today. Page routes
      and endpoints may share a path only when HTTP methods do not conflict.
- [x] Decide trailing slash policy and redirects.
      Decision: route declarations reject trailing slashes except `/`.
      Generated concrete action POST handlers tolerate a trailing slash fallback
      for compatibility.
- [x] Generate typed route param structs or accessors for request-time routes.
      Decision: current request-time access uses `runtime/app.Params(ctx)`,
      `runtime/app.TypedParams(ctx)`, and `runtime/route` typed helpers.
      Per-route structs are deferred.
- [x] Generate typed endpoint metadata accessors if useful.
      Decision: generated handlers attach endpoint metadata and user code reads
      it with `runtime/app.Endpoint(ctx)`.
- [x] Generate typed load/action data accessors if useful.
      Decision: defer until load and action result contracts are stable.
- [x] Add tests for route conflicts, typed param decoding, invalid params, and
      generated accessor compatibility.

Done when:

- [x] Typed route access is easier than reading `map[string]any` while keeping
      route declarations explicit.

### 12. Dev Experience

- [ ] Add browser error overlay for compiler/build failures.
- [x] Add precise changed-file diagnostics in dev output.
      Decision: `gowdk dev` now prints changed, added, and removed input paths
      before each rebuild.
- [x] Decide whether component-level HMR is worth the complexity.
      Decision: defer component-level HMR. Keep full-page live reload as the
      P0 baseline until the component/client dependency graph is stable.
- [x] Preserve local island state across updates if HMR is implemented.
      Decision: not applicable until HMR exists. State preservation is deferred
      with HMR.
- [x] Add faster dependency tracking beyond polling where portable.
      Decision: keep portable polling for now. Native watching is deferred
      until it can stay dependency-light and cross-platform.
- [x] Add generated app restart diagnostics and last-good-build behavior docs.
      See `docs/reference/dev.md`.
- [x] Add `gowdk doctor` or equivalent environment validation if recurring
      setup failures appear.
      Decision: no `gowdk doctor` yet. Add only after recurring setup failures
      expose a stable checklist worth automating.
- [ ] Improve scaffolded examples for common workflows.

Done when:

- [x] `gowdk dev` feels reliable for real apps, even if it intentionally does
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

- [x] Do not make filesystem route placement the source of truth.
- [x] Do not make full-page SSR the default rendering model.
- [x] Do not require npm, Vite, Tailwind, Svelte, or a JavaScript framework for
      normal app flows.
- [x] Do not move domain logic, persistence, auth, or business validation into
      generated code.
- [x] Do not make generated JavaScript the authority for routes, auth, server
      validation, or cache policy.
- [x] Do not make WASM islands the default component runtime.
- [x] Do not add platform adapters to core if normal Go deployment or docs can
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

- [x] Reconcile `MISSING_CHECKLIST.md` with `docs/product/gap-checklist.md`.
- [x] Reconcile `docs/product/gap-checklist.md` with
      `docs/product/requirements.md`.
- [x] Reconcile `docs/product/gap-checklist.md` with
      `docs/product/roadmap.md`.
- [x] Reconcile `docs/product/gap-checklist.md` with
      `docs/engineering/architecture.md`.
- [x] Reconcile generated binary support descriptions across
      `docs/reference/cli.md`, `docs/reference/deployment.md`,
      `docs/getting-started.md`, and `README.md`.
- [x] Reconcile SSR `load {}` support descriptions across language,
      deployment, CLI, architecture, and release docs.
- [x] Reconcile component CSS/assets status across `docs/language/components.md`,
      `docs/reference/css.md`, requirements, and the missing checklist.
- [x] Reconcile WASM island validation status across component docs,
      requirements, ADR 0004, generated-output docs, and examples.
- [x] Replace stale "first-slice" wording where behavior is now product-stable.
- [x] Keep "first-slice" wording where behavior is genuinely narrow.
- [x] Add a single status legend used consistently across all product docs:
      implemented, partial, experimental, planned, and intentionally out of
      scope.

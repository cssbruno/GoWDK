# Product Roadmap

## Product Shape

GOWDK is a Go-first full web app platform built from two coordinated parts:

```text
GOWDK Compiler
component/page compiler
        +
GOWDK Runtime
app/runtime layer
        =
Go-first full web app
```

GOWDK Compiler is the `.gwdk` language and component/page compiler. It turns
package-peer `.gwdk` files and normal Go packages into page output, CSS, client
assets, manifests, endpoint metadata, generated adapter Go, and deployable
artifacts. GOWDK Runtime is the app/runtime layer that serves those artifacts and
runs request-time behavior.

The public naming rule is deliberately narrow:

- `GOWDK` means the product and repository wordmark.
- `GOWDK Compiler` means compiler/language layer.
- `GOWDK Runtime` means app/runtime layer.
- `gowdk` means CLI, Go package/module spelling, and generated file prefixes.
- `addon` means optional feature-registration or integration package inside
  the compiler/runtime ecosystem, not a separate product.
- Never write `GOWDK Kit`; it is the redundant "kit kit" form because the `K`
  in `GOWDK` already carries that meaning.
- Avoid bare `core`; use `compiler core`, `runtime core`, or `repository core`.

GOWDK has three execution lanes:

- Build-time page lane: full pages default to static SPA/prerender output.
- Backend endpoint lane: actions, APIs, and fragments run at request time
  without making the page itself request-rendered.
- Request-time page lane: pages with `load {}` or `go ssr {}` are compiled into
  generated SSR handlers and run through GOWDK Runtime. This lane is integrated
  into the codebase; it is not a separate product layer. It is selected per
  page and currently enabled through the SSR feature gate in config or `--ssr`.

## Ownership Boundaries

Application Go code owns behavior: handlers, validation, auth, storage, typed
inputs, service calls, and business rules stay in Go. Separate `.go` files are
the default path today; optional inline Go authoring in `.gwdk` is planned only
when extracted code remains normal importable, testable package Go.

`.gwdk` files own web declarations: package, page identity, route, layouts,
component usage, build-time data, request-time data, dynamic paths, endpoint
declarations, stores, client blocks, and view markup.

GOWDK Compiler owns parsing, formatting, diagnostics, manifests, route
metadata, endpoint metadata, component/page compilation, CSS and asset planning,
build reports, and generated adapter source.

The generated adapter owns glue: request decoding, route and endpoint dispatch,
calling exported Go handlers, writing `runtime/response.Response`, and returning
clear `501` responses for missing or unsupported bindings where that mode is
allowed.

GOWDK Runtime owns `http.Handler` serving, embedded assets, backend
routing, request context helpers, form decoding primitives, response envelopes,
CSRF, partial fragments, SSR contracts, and one-binary or split-binary wiring.

Addons are package-level extension points around those layers. They can enable
or extend compiler behavior or runtime behavior, but they must not become a
third app framework model with different ownership rules.

Generated JavaScript may enhance navigation, forms, fragments, and local
component state, but it must not become the source of truth for routes, auth,
business rules, trusted validation, server state, or cache policy.

## Product Rules

These are the durable rules. Changing them should require an ADR.

1. Routes are declared inside `.gwdk` files; folder placement is not route truth.
2. `.gwdk` files are peers of Go files and declare `package <name>`.
3. Full pages default to build-time SPA output.
4. Dynamic SPA routes require `paths {}` unless the page uses request-time
   rendering.
5. `build {}` is build-time page data.
6. `load {}` is request-time page data and requires the SSR addon.
7. `act` and `api` declarations name exact exported Go symbols.
8. Actions and APIs are endpoint metadata, not page route kinds.
9. User behavior stays in Go code. Inline Go authoring is optional and must
   extract to normal package Go.
10. Generated Go is adapter glue, not generated application logic.
11. Actions, APIs, and fragments can work without full-page request rendering.
12. SSR is an integrated non-default request-time page-rendering lane.
13. One-binary deploy must work with and without request-time page rendering.
14. Core stays `net/http` compatible; Chi, Gin, Echo, Fiber, and similar frameworks
    are optional adapters, not core dependencies.
15. CSS and styling tooling are addon-driven; Tailwind is optional.
16. Normal app flows should not require user-written JavaScript.

## Current Baseline

`docs/product/requirements.md` records product status and broader product
decisions. Status terms must match `docs/product/requirements.md`: implemented,
partial, experimental, planned, and intentionally out of scope. At a high
level, the current baseline already includes:

- project config loading, source discovery, build targets, module selection,
  manifests, route validation, sitemap output, formatting, diagnostics, a
  shared tokenizer with parser recovery across declaration boundaries, CLI
  route/endpoint/inspect reports, and LSP support;
- package-first `.gwdk` files, package mismatch diagnostics, package source
  spans, package-scoped `use alias "package"` component imports, exact
  `act Name POST "/path"` and `api Name METHOD "/path"` declarations, optional
  `//gowdk:act` and `//gowdk:api` Go endpoint comments, and migration
  diagnostics for old action/API block syntax;
- build-time SPA output for simple pages, dynamic `paths {}` subsets, literal,
  imported, same-package, and default `go {}` build data subsets, layouts, components, CSS
  assets, route manifests, asset manifests, build reports, generated app
  output, non-served security posture reports, embedded assets, local binaries,
  generated Docker contexts for one-binary deploys, WASM deploy artifacts,
  optional sitemap/robots output, optional production generated-asset
  obfuscation, and a polling dev server with live reload;
- component discovery, imported props/state contracts, slots, generated
  JavaScript islands, component-level WASM island assets, and a first-slice client
  language for local component behavior;
- same-package Go handler binding through `go/packages` and `go/types`, exact
  exported handler matching, typed action input decoding,
  generated `501` responses for missing or unsupported bindings, deterministic
  import aliases, and formatted generated app source;
- shared backend routing primitives in `runtime/app`, runtime action/API
  adapter helpers, one generated backend hook, request body limits, and no-store
  defaults for request-time responses;
- typed backend adapter IR driving generated action/API/fragment/contract route
  registrations, backend imports, split frontend proxy route matching,
  backend-only route presence, guard/rate-limit/CSRF endpoint checks, and
  generated `501` fallback metadata;
- first-slice action/API execution, partial fragment responses, dynamic
  standalone fragment routes, and concrete or dynamic request-time SSR pages
  with declared `load {}` fields through buildgen, appgen, `runtime/app`, and
  `runtime/route`.
- compiler-validated query-bounded `g:subscribe` metadata for presentation
  events and explicit `RegisterInvalidation[event, query]` metadata for
  domain-event query refresh, including IR records, build-report events, exact
  event/query-type HTML markers, generated client `replaceHTML` patches,
  current-document refresh for non-subscribed invalidated query regions, and
  missing/invalid/non-web-role diagnostics.
- `gowdk audit` posture and policy evaluation for routes, backend endpoints,
  command/query contract web endpoints, and frontend audit surfaces, with
  declared `*.audit.gwdk` policies, generated runtime audit tests,
  registry-backed findings, and CI-friendly JSON output.
- dependency-free `runtime/trace` primitives for W3C trace IDs,
  `traceparent` propagation, context spans, sinks, sampling, local JSON/SSE
  trace collection, browser span ingest, a self-contained viewer, debug-gated
  generated app instrumentation through `addons/observability`, contract/event
  propagation, and nested-module OTLP export.

Do not roadmap those completed slices as future work. Future work should
stabilize their contracts, remove generation debt, and fill the missing
production pieces below.

`docs/product/requirements.md` records product decisions for comparison-driven
gaps. Treat those decisions as constraints on roadmap execution: planned work
may implement the GOWDK-native contract, defer it with clear docs or
diagnostics, or keep it intentionally out of scope. Do not turn deferred
comparison features into implicit commitments.

## Roadmap

This order follows dependencies. Some later areas already have first-slice code,
but they are not product-stable until the earlier metadata and adapter contracts
are stable.

| Step | Theme | Definition of Done |
| --- | --- | --- |
| 1 | GOWDK AST and analyzer | The compiler has explicit AST nodes for package declarations, metadata declarations, routes, imports, stores, blocks, component contracts, client blocks, and source spans. A real analyzer lowers that AST into normalized package, route, endpoint, component, type, asset, and generated-output metadata. |
| 2 | Stable internal IR | Templates, client behavior, routes, assets, CSS, endpoints, SSR pages, and generated output are represented by typed compiler IR instead of ad hoc parser/buildgen/appgen structs leaking across phases. |
| 3 | Source import semantics | Cross-package component calls have explicit page/component-scoped `use` semantics. Layouts, stores, and assets have explicit `use` semantics or are rejected with clear diagnostics. Qualified layout references are either implemented or intentionally deferred with documented diagnostics. |
| 4 | Build-time data and diagnostics | Build data moves beyond the first literal/imported no-argument subset. Same-package build functions are either supported or documented as intentionally unsupported. Parser, route, view, component, client, package, and build errors have useful spans and suggestions. |
| 5 | Unified endpoint metadata | Actions and APIs normalize into one framework-neutral endpoint model containing source, kind, package path, package name, symbol, method, path, signature kind, input type, source span, and binding status. Route metadata remains limited to static, SPA, SSR, and hybrid page routes. |
| 6 | Endpoint discovery policy | Optional Go endpoint comments such as `//gowdk:act POST /login` and `//gowdk:api GET /api/session` can feed the same endpoint model. The compiler never auto-discovers endpoints by function name and never scans Chi/Gin/Echo/Fiber route registration as a source of truth. Conflicts are hard diagnostics. |
| 7 | Binding severity policy | Missing or unsupported handlers can remain non-fatal in dev/migration mode, but strict production builds fail unless an explicit stub flag allows `501` output. Feature packages are documented as not importing generated app output. |
| 8 | Generated adapter IR | Implemented. Backend adapter generation is driven by typed IR for imports, endpoint registrations, request decoding, handler calls, response writing, and `501` fallbacks. One-binary, split frontend proxy, and backend-only app generation consume the same backend metadata. |
| 9 | Go AST generation cleanup | API handlers, backend route registration, app shells, embed wiring, split app code, and remaining generated Go move to `go/ast`/`go/printer` plus `go/format`. Hardcoded line writing and source snippets are banned except for documented temporary exceptions. |
| 10 | Secure actions and forms | Generated action adapters wire CSRF token generation and validation, define token exposure, invalid-CSRF status/body shape, submit-button intent handling, validation fragment patterns, and production-safe action/API docs. |
| 11 | Guards and runtime context | Generated guards work for SSR, actions, and APIs. The request context helper contract is documented around `context.Context`, `app.Request(ctx)`, `app.Params(ctx)`, `app.CSRF(ctx)`, and `app.Session(ctx)`, or the project deliberately switches to an explicit app context. |
| 12 | Request-time page rendering | Generated SSR handlers execute `load {}`, enforce guards, decode typed route params, expose route-level metadata, support redirects and error pages, and run full request-time user logic through the integrated request-time page lane. |
| 13 | Errors, cache, and hybrid | SSR/action/API error boundaries are defined. Static files, SPA routes, backend endpoints, partial responses, SSR routes, and hybrid pages get cache and revalidation policy. Hybrid pages use the explicit request-time lane while streaming, data refresh, and non-HTTP revalidation remain separate planned capabilities. |
| 14 | Contract-driven runtime | Implemented. Queries, commands, domain events, integration events, presentation events, and jobs are typed Go contracts. Frontend UI events trigger commands or queries. Commands have one owner. Domain and integration events are backend-owned facts emitted after backend success. Presentation events notify realtime UI through explicit, query-bounded subscription metadata, generated subscription-filtered SSE fanout, and generated bounded client patches. Domain events can explicitly invalidate bound queries through `RegisterInvalidation[event, query]`, generated `gowdk.query.invalidate` events, and generated current-document refresh for matching non-subscribed query regions. Local in-process dispatch is default, optional worker/cron roles can run the same registrations through runtime and generated helper APIs, worker replay supports ack/nack, seen-store deduplication, and explicit backoff hooks, and CLI tooling can list, trace, or graph contracts including `invalidates` edges. |
| 15 | Static-first SPA navigation | SPA routes remain real URLs that work on direct open and refresh. Generated JS may intercept internal links, fetch built page shells or fragments, swap page regions, preserve scroll/focus, prefetch static route assets, and show loading/error UI, but it must not own routing, auth, business rules, validation, backend behavior, global app state, loading policy, or cache policy. |
| 16 | Components and client language | Components gain real `g:if` mount/unmount, richer expression props, child-to-parent events, bindable state, typed exports, named/scoped slots, scoped CSS/assets, a documented component contract, a proper reactive dependency graph, predictable batching, and cycle diagnostics. |
| 17 | Islands and WASM | Generated JavaScript islands stay compiler-owned local UI behavior. Component-level WASM islands get a production ABI, browser-side Go logic contracts, and entrypoint/export validation. Deploy-target WASM artifacts remain separate from browser island WASM. |
| 18 | CSS, assets, and packaging | External addon loading is hardened, richer page-aware CSS processor contracts are stable, and Tailwind/CSS deployment docs stay explicit that external tooling is user-installed. Implemented CSS asset hashing, component CSS scope/hash metadata, component non-CSS asset emission, and binary cache policy remain stable. Module selection remains artifact packaging, not runtime module orchestration. |
| 19 | Framework adapters | GOWDK Runtime remains `net/http` first. Optional Chi, Echo, Gin, and Fiber adapters wrap the same generated `http.Handler`; Chi/Echo/Gin can mount routes from generated OpenAPI metadata, and generated code stays framework-neutral by default. |
| 20 | Dev and tooling | `gowdk dev` can run generated app/runtime flows for backend routes and SSR, skip unchanged rebuilds, cache watched input state, and show SPA/static browser rebuild failures with diagnostic codes, source ranges, last-good build time, and changed files. Backend process restart/proxy behavior is decided. Deploy previews, component-aware HMR, generated-app runtime browser overlay delivery, richer LSP completions, and editor navigation are added; runtime overlay delivery and component HMR are tracked in [#424](https://github.com/cssbruno/GoWDK/issues/424). |
| 21 | Observability | Partial. Generated apps can opt into GOWDK trace spans across route, guard, handler, SSR, action, API, fragment, contract, job, island, nav, and user lanes while keeping the root runtime dependency-free. `runtime/trace`, `addons/observability`, a local viewer, contract/event propagation, browser propagation, WASM island bridge reuse, and nested-module OTLP HTTP export are implemented. Durable trace storage, hosted analysis, and production sampling/access policy remain app-owned hardening work. |
| 22 | Documentation sync | README, requirements, architecture, deployment, roadmap, and examples stay synchronized with implemented behavior and commands. |

## Candidate Release Order

The exact version numbers can change, but the release order should not skip the
contract work that later features depend on.
`docs/engineering/release-plan.md` tracks the open-ended 0.x hardening backlog
without making any minor version a production-readiness target.

### Compiler Contract Release

- GOWDK AST and analyzer.
- Stable internal IR.
- Explicit source import semantics.
- Better spans and diagnostics.
- Broader build-time data contract.
- P0/P1 language decisions from `docs/product/requirements.md` remain enforced:
  no arbitrary JavaScript, no external template semantics, and no generated JS
  ownership of trusted app behavior.

### Endpoint And Adapter Release

- Unified endpoint metadata.
- Endpoint discovery policy.
- Binding severity policy.
- Generated adapter IR.
- Remaining generated Go emitted through AST/printer/format.

### Secure Backend Release

- CSRF-wired generated action adapters.
- Form token exposure and invalid-token response policy.
- Submit intent handling.
- Structured validation and fragment response patterns.
- Production-safe action/API docs.

### Request-Time Page Release

- Typed route params.
- Route-level metadata.
- Custom SSR/action/API error-boundary syntax and examples.
- Cache/no-store policy for request-time page rendering.

### SPA And Hybrid Release

- Static-first SPA navigation enhancements.
- Progressive form enhancement.
- Bare hybrid pages as generated request-time routes.
- Revalidation syntax and hybrid cache enforcement.

### Contract Runtime Release

- Typed query, command, event, and job registry.
- Command owner and event subscriber validation.
- Domain events are backend-owned facts emitted after state changes succeed.
- Presentation events can notify realtime UI without becoming trusted input.
- Local in-process dispatch as the default.
- Optional web, worker, cron, API, and admin runtime roles.
- CLI contract listing, trace, and graph output.
- No backend bus messages for low-level UI events.

### Component And Island Release

- Richer component contract.
- Proper client reactivity.
- Slots, component CSS/assets, and typed exports.
- Production WASM island ABI.
- Bounded `client {}` remains the reactivity model unless a later ADR replaces
  it with an equally Go-owned contract.
- Full-page hydration remains out of the repository core; browser behavior should stay static
  output, progressive enhancement, server fragments, and explicit islands.

### Platform Tooling Release

- Full CSS processor addon loading.
- Generated app dev loop.
- Stronger editor tooling.
- Production operations docs must cover secrets, CSRF rotation, reverse
  proxies, cache/CDN policy, health checks, metrics, logging, binary deploy,
  generated Docker contexts, richer platform manifests, and rollback before any
  production-ready claim.
- P2 ecosystem polish is owned by optional docs, examples, website pages, or
  CLI generators: playground onboarding, addon discovery, performance
  profiling, migration guides, image guidance, SEO metadata beyond
  sitemap/robots, and PWA/offline guidance must not add mandatory npm,
  framework, hosted execution, or platform dependencies to the repository core.

## Non-Goals For Repository Core

- Making full-page request rendering the default.
- Making browser JavaScript the app contract.
- Generating user domain logic, services, stores, auth, storage, or business
  validation.
- Requiring npm, Tailwind, Chi, Gin, Echo, Fiber, or another framework in the
  repository core.
- Making WASM islands the default component runtime.
- Treating folder placement as route truth.
- Auto-discovering backend endpoints from function names.

## Planning Sources

- `docs/product/requirements.md`: requirement status.
- `docs/product/playground.md`: docs-first playground and sandboxing contract.
- `docs/product/contract-runtime-spec.md`: milestone-14 contract runtime
  closure criteria.
- `docs/product/observability-tracing-spec.md`: runtime trace primitives and
  generated instrumentation direction.
- `docs/engineering/architecture.md`: architecture and implemented boundaries.
- `docs/engineering/release-plan.md`: open-ended 0.x hardening checklist.

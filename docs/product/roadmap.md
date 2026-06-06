# Product Roadmap

## Product Shape

GOWDK is a Go-first full web app platform built from two coordinated parts:

```text
GOWDK component/page compiler
        +
GOWDK app/runtime kit
        =
Go-first full web app
```

The compiler turns package-peer `.gwdk` files and normal Go packages into page
output, CSS, client assets, manifests, endpoint metadata, generated adapter Go,
and deployable artifacts. The app/runtime kit serves those artifacts and runs
request-time behavior.

GOWDK has three execution lanes:

- Build-time page lane: full pages default to static SPA/prerender output.
- Backend endpoint lane: actions, APIs, and fragments run at request time
  without making the page itself request-rendered.
- Request-time page lane: `@render ssr` pages are compiled into generated SSR
  handlers and run through the same app/runtime kit. This lane is integrated
  into the codebase; it is not a separate product layer. It is selected per
  page and currently enabled through the SSR feature gate in config or `--ssr`.

## Ownership Boundaries

Application Go packages own behavior: handlers, validation, auth, storage,
typed inputs, service calls, and business rules stay in normal Go.

`.gwdk` files own web declarations: package, page identity, route, render mode,
layouts, component usage, build-time data, dynamic paths, endpoint declarations,
stores, client blocks, and view markup.

The compiler owns parsing, formatting, diagnostics, manifests, route metadata,
endpoint metadata, component/page compilation, CSS and asset planning, build
reports, and generated adapter source.

The generated adapter owns glue: request decoding, route and endpoint dispatch,
calling exported Go handlers, writing `runtime/response.Response`, and returning
clear `501` responses for missing or unsupported bindings where that mode is
allowed.

The app/runtime kit owns `http.Handler` serving, embedded assets, backend
routing, request context helpers, form decoding primitives, response envelopes,
CSRF, partial fragments, SSR contracts, and one-binary or split-binary wiring.

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
6. `load {}` is request-time page data and requires `@render ssr` or an
   explicit hybrid request-time branch.
7. `act` and `api` declarations name exact exported Go symbols.
8. Actions and APIs are endpoint metadata, not page route kinds.
9. User behavior stays in normal Go packages.
10. Generated Go is adapter glue, not generated application logic.
11. Actions, APIs, and fragments can work without full-page request rendering.
12. SSR is an integrated non-default request-time page-rendering lane.
13. One-binary deploy must work with and without request-time page rendering.
14. Core stays `net/http` compatible; Gin, Echo, Fiber, and similar frameworks
    can be adapters later, not core dependencies.
15. CSS and styling tooling are plugin-driven; Tailwind is optional.
16. Normal app flows should not require user-written JavaScript.

## Current Baseline

`MISSING_CHECKLIST.md` is the detailed source of truth for what is done and what
is still missing. At a high level, the current baseline already includes:

- project config loading, source discovery, build targets, module selection,
  manifests, route validation, sitemap output, formatting, diagnostics, CLI
  route reports, and LSP support;
- package-first `.gwdk` files, package mismatch diagnostics, package source
  spans, package-scoped `use alias "package"` component imports, exact
  `act Name POST "/path"` and `api Name METHOD "/path"` declarations, optional
  `//gowdk:act` and `//gowdk:api` Go endpoint comments, and migration
  diagnostics for old action/API block syntax;
- build-time SPA output for simple pages, dynamic `paths {}` subsets, literal,
  imported, and same-package build data subsets, layouts, components, CSS
  assets, route manifests, asset manifests, build reports, generated app
  output, embedded assets, local binaries, WASM deploy artifacts, and a polling
  dev server with live reload;
- component discovery, imported props/state contracts, slots, generated
  JavaScript islands, explicit WASM island assets, and a first-slice client
  language for local component behavior;
- same-package Go handler binding through `go list`, `go/parser`, `go/ast`, and
  `go/types`, exact exported handler matching, typed action input decoding,
  generated `501` responses for missing or unsupported bindings, deterministic
  import aliases, and formatted generated app source;
- shared backend routing primitives in `runtime/app`, runtime action/API
  adapter helpers, one generated backend hook, request body limits, and no-store
  defaults for request-time responses;
- first-slice action/API execution, partial fragment responses, and concrete or
  dynamic `@render ssr` pages with declared `load {}` fields through buildgen,
  appgen, `runtime/app`, and `runtime/route`.

Do not roadmap those completed slices as future work. Future work should
stabilize their contracts, remove generation debt, and fill the missing
production pieces below.

## Roadmap

This order follows dependencies. Some later areas already have first-slice code,
but they are not product-stable until the earlier metadata and adapter contracts
are stable.

| Step | Theme | Definition of Done |
| --- | --- | --- |
| 1 | GOWDK AST and analyzer | The compiler has explicit AST nodes for package declarations, annotations, routes, imports, stores, blocks, component contracts, client blocks, and source spans. A real analyzer lowers that AST into normalized package, route, endpoint, component, type, asset, and generated-output metadata. |
| 2 | Stable internal IR | Templates, client behavior, routes, assets, CSS, endpoints, SSR pages, and generated output are represented by typed compiler IR instead of ad hoc parser/buildgen/appgen structs leaking across phases. |
| 3 | Source import semantics | Cross-package component calls have explicit page/component-scoped `use` semantics. Layouts, stores, and assets have explicit `use` semantics or are rejected with clear diagnostics. Qualified layout references are either implemented or intentionally deferred with documented diagnostics. |
| 4 | Build-time data and diagnostics | Build data moves beyond the first literal/imported no-argument subset. Same-package build functions are either supported or documented as intentionally unsupported. Parser, route, view, component, client, package, and build errors have useful spans and suggestions. |
| 5 | Unified endpoint metadata | Actions and APIs normalize into one framework-neutral endpoint model containing source, kind, package path, package name, symbol, method, path, signature kind, input type, source span, and binding status. Route metadata remains limited to static, SPA, SSR, and hybrid page routes. |
| 6 | Endpoint discovery policy | Optional Go endpoint comments such as `//gowdk:act POST /login` and `//gowdk:api GET /api/session` can feed the same endpoint model. The compiler never auto-discovers endpoints by function name and never scans Gin/Echo/Fiber route registration as a source of truth. Conflicts are hard diagnostics. |
| 7 | Binding severity policy | Missing or unsupported handlers can remain non-fatal in dev/migration mode, but strict production builds fail unless an explicit stub flag allows `501` output. Feature packages are documented as not importing generated app output. |
| 8 | Generated adapter IR | Backend adapter generation is driven by typed IR for imports, endpoint registrations, request decoding, handler calls, response writing, and `501` fallbacks. One-binary, split frontend proxy, and backend-only app generation consume the same metadata. |
| 9 | Go AST generation cleanup | API handlers, backend route registration, app shells, embed wiring, split app code, and remaining generated Go move to `go/ast`/`go/printer` plus `go/format`. Hardcoded line writing and source snippets are banned except for documented temporary exceptions. |
| 10 | Secure actions and forms | Generated action adapters wire CSRF token generation and validation, define token exposure, invalid-CSRF status/body shape, submit-button intent handling, validation fragment patterns, and production-safe action/API docs. |
| 11 | Guards and runtime context | Generated guards work for SSR, actions, and APIs. The request context helper contract is documented around `context.Context`, `app.Request(ctx)`, `app.Params(ctx)`, `app.CSRF(ctx)`, and `app.Session(ctx)`, or the project deliberately switches to an explicit app context. |
| 12 | Request-time page rendering | Generated SSR handlers execute `load {}`, enforce guards, decode typed route params, expose route-level metadata, support redirects and error pages, and run full request-time user logic through the integrated request-time page lane. |
| 13 | Errors, cache, and hybrid | SSR/action/API error boundaries are defined. Static files, SPA routes, backend endpoints, partial responses, SSR routes, and hybrid pages get cache and revalidation policy. Hybrid pages stay SPA by default and opt into request-time capabilities explicitly. |
| 14 | Static-first SPA navigation | SPA routes remain real URLs that work on direct open and refresh. Generated JS may intercept internal links, fetch built page shells or fragments, swap page regions, preserve scroll/focus, prefetch static route assets, and show loading/error UI, but it must not own routing, auth, business rules, validation, backend behavior, global app state, loading policy, or cache policy. |
| 15 | Components and client language | Components gain real `g:if` mount/unmount, richer expression props, child-to-parent events, bindable state, typed exports, named/scoped slots, scoped CSS/assets, a documented component contract, a proper reactive dependency graph, predictable batching, and cycle diagnostics. |
| 16 | Islands and WASM | Generated JavaScript islands stay compiler-owned local UI behavior. Explicit WASM islands get a production ABI, browser-side Go logic contracts, and entrypoint/export validation. Deploy-target WASM artifacts remain separate from browser island WASM. |
| 17 | CSS, assets, and packaging | Full plugin loading, page-aware CSS processor selection, component AST/IR scope and hash metadata, Tailwind/CSS deployment docs, asset hashing, and binary cache policy are implemented. Module selection remains artifact packaging, not runtime module orchestration. |
| 18 | Framework adapters | Core remains `net/http`. Optional Gin, Echo, and Fiber adapters wrap the same generated `http.Handler` after the handler contract is stable; generated code stays framework-neutral by default. |
| 19 | Dev, playground, and tooling | `gowdk dev` can run generated app/runtime-kit flows for backend routes and SSR. Backend process restart/proxy behavior is decided. Faster rebuild caching, deploy previews, browser playground, browser-compiled GOWDK, richer LSP completions, and editor navigation are added. |
| 20 | Documentation sync | README, requirements, architecture, deployment, roadmap, examples, and `MISSING_CHECKLIST.md` stay synchronized with implemented behavior and commands. |

## Candidate Release Order

The exact version numbers can change, but the release order should not skip the
contract work that later features depend on.

### Compiler Contract Release

- GOWDK AST and analyzer.
- Stable internal IR.
- Explicit source import semantics.
- Better spans and diagnostics.
- Broader build-time data contract.

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
- Production-safe action/API guard configuration docs.
- Production-safe action/API docs.

### Request-Time Page Release

- Broader `load {}` data shapes beyond declared scalar fields.
- SSR guard success-path registration examples.
- Typed route params.
- Route-level metadata.
- Custom SSR/action/API error-boundary syntax and examples.
- Cache/no-store policy for request-time page rendering.

### SPA And Hybrid Release

- Static-first SPA navigation enhancements.
- Progressive form enhancement.
- Bare hybrid pages as SPA output with explicit request-time capability gating.
- Cache and revalidation syntax and binary enforcement.

### Component And Island Release

- Richer component contract.
- Proper client reactivity.
- Slots, component CSS/assets, and typed exports.
- Production WASM island ABI.

### Platform Tooling Release

- Full CSS/plugin loading.
- Emitted CSS filename hashing and packaging docs.
- Optional framework adapters.
- Generated app dev loop.
- Browser playground and stronger editor tooling.

## Non-Goals For Core

- Making full-page request rendering the default.
- Making browser JavaScript the app contract.
- Generating user domain logic, services, stores, auth, storage, or business
  validation.
- Requiring npm, Tailwind, Gin, Echo, Fiber, or another framework in core.
- Making WASM islands the default component runtime.
- Treating folder placement as route truth.
- Auto-discovering backend endpoints from function names.

## Planning Sources

- `MISSING_CHECKLIST.md`: detailed missing-work checklist.
- `docs/product/requirements.md`: requirement status.
- `docs/engineering/architecture.md`: architecture and implemented boundaries.
- `.llm/plans/gowdk-world-roadmap.md`: active implementation planning index.
- `.llm/plans/deep-go-package-integration.md`: package-first language work.
- `.llm/plans/go-native-adapter-boundary.md`: generated adapter boundary work.
- `.llm/features/golangish-reactive-islands.md`: client language and island
  direction.

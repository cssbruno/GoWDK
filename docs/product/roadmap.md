# Product Roadmap

The roadmap orders product work by dependency. It does not replace
[Requirements](requirements.md), which owns current capability status.

## Product Shape

GOWDK is built from two coordinated parts:

```text
GOWDK Compiler + GOWDK Runtime = Go-first full web app output
```

GOWDK Compiler owns `.gwdk` parsing, analysis, diagnostics, IR, generated Go
adapter source, build output, manifests, metadata, assets, and tooling. GOWDK
Runtime owns generated app serving, request context, response envelopes, actions,
APIs, fragments, guards, contracts, tracing, embedded assets, and one-binary or
split-binary wiring.

Application behavior stays in Go. `.gwdk` files declare web contracts and normal
Go packages own handlers, validation, auth policy, storage, services, and
operations.

## Product Rules

- Routes are declared in `.gwdk` files, not inferred from folders.
- `.gwdk` files are peers of Go files and declare `package <name>`.
- Full pages default to build-time SPA/static output.
- Dynamic SPA routes require `paths {}` unless the page uses request-time
  rendering.
- `build {}` is build-time page data.
- `server {}` is request-time page data and requires the SSR addon.
- `act` and `api` declarations name exact exported Go symbols.
- Actions, APIs, and fragments are endpoint metadata, not page route kinds.
- Generated Go is adapter glue, not generated application logic.
- Actions, APIs, fragments, contracts, and realtime can work without full-page
  request-time rendering.
- SSR is integrated and selected per page.
- One-binary deploy must work with and without request-time page rendering.
- Core stays `net/http` compatible; framework adapters are optional.
- CSS and styling tooling are addon-driven; Tailwind is optional.
- Normal app flows should not require user-written JavaScript.

## Current Baseline

At a high level, the codebase already has:

- config loading, source discovery, build targets, module selection, manifests,
  route validation, formatting, diagnostics, route reports, endpoint reports,
  inspection reports, LSP support, and docs gates;
- typed `.gwdk` AST, analyzer, compiler IR, parser recovery, view model split,
  source import metadata, generated-output planning, and Go binding inspection;
- static/SPA output, literal dynamic paths, build-time data, layouts,
  components, CSS/assets, SEO output, generated app source, local binaries,
  Docker and deployment recipe starters, WASM artifacts, production asset
  obfuscation, and a polling dev loop;
- generated action, API, fragment, SSR, hybrid, guard, CSRF, rate-limit,
  contract, realtime subscription/invalidation, audit, trace, and lifecycle
  service slices;
- generated app startup, backend route registration, middleware hooks, worker
  and cron role outputs, and app test orchestration;
- root-module dependency boundaries with optional framework, telemetry, and
  broker adapters isolated in nested modules where required.

Do not roadmap completed slices as future work. Future work should stabilize
contracts, remove generation debt, and fill the remaining production gaps called
out in requirements.

## Roadmap

| Step | Theme | Definition of done |
| --- | --- | --- |
| 1 | GOWDK AST and analyzer | Source constructs have typed AST nodes, source spans, analyzer records, and tests. |
| 2 | Stable internal IR | Templates, client behavior, routes, assets, CSS, endpoints, SSR pages, and generated output use typed compiler IR. |
| 3 | Source import semantics | Cross-package components, layouts, stores, and assets have explicit `use` semantics or clear diagnostics. |
| 4 | Build-time data and diagnostics | Supported build data is typed, deterministic, well-diagnosed, and covered by fixtures. |
| 5 | Unified endpoint metadata | Actions, APIs, fragments, SSR loads, commands, and queries share framework-neutral metadata. |
| 6 | Endpoint discovery policy | Optional Go endpoint comments feed the same model; auto-discovery by framework route registration stays out of scope. |
| 7 | Binding severity policy | Missing and unsupported handlers are explicit, with strict production-shaped builds. |
| 8 | Generated adapter IR | Implemented. Generated backend adapters consume typed IR. |
| 9 | Go AST generation cleanup | Generated Go uses `go/ast`, `go/printer`, and `go/format`; string snippets are temporary exceptions only. |
| 10 | Secure actions and forms | CSRF, validation fragments, body limits, redirects, and production-safe action/API docs are wired and tested. |
| 11 | Guards and runtime context | Guards work across request-time lanes and request context helpers are documented. |
| 12 | Request-time page rendering | SSR handlers execute `server {}`, guards, route params, redirects, error pages, and user logic through the integrated lane. |
| 13 | Errors, cache, and hybrid | Error boundaries, cache policy, revalidation, and hybrid behavior are explicit and tested. |
| 14 | Contract-driven runtime | Implemented. Typed contracts, events, jobs, worker replay, realtime fanout, invalidation, and CLI graph/trace/list surfaces are available. |
| 15 | Static-first SPA navigation | SPA routes remain real URLs; generated JS can enhance navigation without owning routing, auth, validation, server state, or cache policy. |
| 16 | Components and client language | Components gain richer props, state, slots, scoped CSS/assets, transitions, batching, and cycle diagnostics. |
| 17 | Islands and WASM | JavaScript islands stay compiler-owned; component WASM islands get a production ABI and validation. |
| 18 | CSS, assets, and packaging | Optional CSS processors, scoped assets, content hashes, cache metadata, and packaging contracts are stable. |
| 19 | Framework adapters | Optional Chi, Echo, Gin, Fiber, and future adapters wrap generated `net/http` handlers without entering compiler/runtime core. |
| 20 | Dev and tooling | Formatter, diagnostics, LSP, VS Code, examples, docs-site, cookbook, `gowdk dev`, and `gowdk test` stay aligned with implemented behavior. |
| 21 | Observability | Trace, metrics, logs, browser spans, contract propagation, and production exporter boundaries stay opt-in and dependency-aware. |
| 22 | Production hardening | Security posture, auth boundaries, operations, release trust, and deployment guidance are measurable without claiming production readiness. |

## Related Documents

- [Requirements](requirements.md): current status matrix.
- [Vision](vision.md): product identity and constraints.
- [Contract Runtime](contract-runtime-spec.md): current contract-runtime product boundary.
- [Realtime Hardening](realtime-hardening-spec.md): realtime subscription,
  replay, revocation, and invalidation hardening.
- [Tracing ADR](../engineering/decisions/0013-built-in-tracing-observability.md):
  runtime trace primitives and generated instrumentation direction.
- [Architecture](../engineering/architecture.md): implemented system boundaries.
- [Release Process](../engineering/release.md): release gates, artifacts, and publication workflow.

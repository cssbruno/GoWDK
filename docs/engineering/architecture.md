# Architecture

GOWDK is a compile-first Go web compiler and runtime. `.gwdk` files declare web
surface contracts; normal Go packages own application behavior; generated Go is
inspectable adapter glue.

## System Shape

```text
.gwdk source + gowdk.config.go + Go packages
  -> parser and typed source AST
  -> analyzer and compiler IR
  -> validation and Go binding
  -> static output, generated app source, reports, manifests
  -> optional Go binary or Go js/wasm artifact
```

Full pages default to build-time SPA/static output. Actions, APIs, fragments,
contracts, guards, and request-time page loaders run through generated runtime
adapters only when declared and enabled. SSR remains an integrated non-default
request-time lane selected with `server {}` or `go server {}`.

## Ownership Boundaries

| Layer | Owns | Does not own |
| --- | --- | --- |
| GOWDK source | Page, component, layout, route, endpoint, asset, guard, cache, and bounded browser declarations | Business logic, storage, authorization policy, production operations |
| Compiler internals | Parsing, typed AST, analysis, IR, diagnostics, validation, build reports, manifests, and generated-output planning | Request serving and app-owned runtime state |
| Generated app | Adapter glue, route registration, decoding, response writing, guard/rate-limit/CSRF ordering, embedded assets, lifecycle hooks | Domain behavior and external infrastructure |
| Runtime packages | `net/http` helpers, request context, response envelopes, assets, guards, contracts, tracing, and addon helpers | Application schemas, migrations, secrets, auth policy, backups, incidents |
| Addons | Optional feature registration or integration packages | A third application framework model |

## Compatibility Records

`internal/gwdkir.Program` is the compiler handoff. New generated-output work
should consume that IR or add fields there first.

Current handoff path:

```text
source
  -> `internal/gwdkast`
  -> `internal/gwdkir` records
  -> `internal/gwdkanalysis.BuildProgram`
  -> `internal/compiler` validation and binding
  -> `internal/buildgen` / `internal/appgen`
```

Golden tests pin the major handoffs: parser AST, IR, routes, endpoints,
generated Go, generated HTML/CSS, route and asset manifests, build reports, and
public manifest output.

## Core Packages

| Package | Role |
| --- | --- |
| `cmd/gowdk` | CLI entrypoint for init, check, build, dev, test, inspect, routes, endpoints, audit, and LSP |
| `internal/discover` | Source discovery from config, module, and explicit path inputs |
| `internal/parser`, `internal/syntax`, `internal/gwdkast` | Tokenization, parsing, recovery, and source AST |
| `internal/gwdkanalysis`, `internal/gwdkir` | Program assembly and stable compiler IR |
| `internal/compiler` | Render-rule validation, endpoint discovery, Go binding, security posture, and diagnostics |
| `internal/viewmodel`, `internal/viewparse`, `internal/viewanalysis`, `internal/viewvalidation`, `internal/viewrender` | View source model, parsing, analysis, validation, and rendering |
| `internal/buildgen` | Static output, assets, manifests, reports, SEO output, and browser runtime assets |
| `internal/appgen` | Generated application source, backend adapters, embedded assets, split outputs, binaries, and role apps |
| `runtime/app`, `runtime/response`, `runtime/guard`, `runtime/contracts`, `runtime/trace` | Public runtime contracts consumed by generated applications |
| `addons/*` | Optional compiler/runtime feature gates and integration helpers |

## Runtime Rules

- Generated binaries use standard `net/http` handlers.
- TLS, public routing, storage, backups, and incident response are deployment
  concerns outside generated code.
- Generated JavaScript may enhance navigation, forms, fragments, local state,
  contracts, and realtime patches; it must not own routing truth, auth truth,
  validation truth, business logic, server state, or cache policy.
- Optional Chi, Echo, Gin, Fiber, Redis, NATS, WebSocket, Tailwind, and
  OpenTelemetry integrations stay isolated from the root runtime graph unless
  explicitly selected.

## Current Partial Areas

Use [Product Requirements](../product/requirements.md) for exact status. The
main partial surfaces are richer local client reactivity, hybrid streaming and
refresh, non-HTTP revalidation, richer worker/cron scheduling, and
platform-specific deployment adapters.

## Change Rules

- Add an ADR for hard-to-reverse architecture choices.
- Keep source model, analysis, rendering, and runtime packages separated.
- Add compiler IR fields before threading ad hoc structs across phases.
- Keep generated code deterministic, formatted, and inspectable.
- Update tests and the owning documentation in the same change.

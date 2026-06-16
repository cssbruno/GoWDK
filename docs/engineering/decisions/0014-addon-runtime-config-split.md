# ADR 0014: Addon Runtime Config Split

Date: 2026-06-16

Status: Proposed

## Context

Built-in addon packages used to combine build-time registration with
request-time runtime helpers. The registration face (`Addon()` and `ImportPath`)
imports the root `gowdk` config package. When generated apps imported helpers
from `addons/<name>`, the shipped app also linked the root build-time config
package through that addon.

The root package is small today, but this dependency direction violates the
compiler/runtime boundary and makes future config growth leak into generated
binaries.

## Decision

Request-time helper code belongs under `runtime/<name>`. The build-time addon
package remains the config-facing package.

| Addon package | Runtime package | Runtime role |
| --- | --- | --- |
| `addons/actions` | `runtime/actions` | CSRF, action registry, form decoding, required validation |
| `addons/api` | `runtime/api` | JSON request helpers, query helpers, API responses |
| `addons/partial` | `runtime/partial` | Fragment and swap helpers, client hook constants |
| `addons/ratelimit` | `runtime/ratelimit` | Limiter, stores, middleware, Redis-store adapter interface |
| `addons/realtime` | `runtime/realtime` | Presentation fanout aliases and dependency-free SSE helpers |
| `addons/ssr` | `runtime/ssr` | Load context, redirects, layouts, guards, region rendering |

Generated app source imports the runtime packages. Existing user imports of the
addon packages remain source-compatible through aliases while users migrate to
runtime helper imports.

## Consequences

### Positive

- Generated apps no longer import request-time helpers through addon packages
  that also import the root config package.
- New request-time code has a clear home under `runtime/`.
- The existing `runtime/auth` and `runtime/contracts` pattern becomes the
  repository rule for request-time helpers.

### Negative

- Preferred helper import paths changed, so docs and examples need migration
  notes even though compatibility re-exports remain.

### Neutral

- Config files still import `addons/<name>` and call `<name>.Addon()`.
- The compiler accepts both `addons/ssr.LoadContext` and
  `runtime/ssr.LoadContext` for load handler signatures during migration.

## Alternatives Considered

- Keep request-time helpers under `addons/<name>` and promise the root package
  stays light. Rejected because the boundary would remain unenforced.
- Move runtime helpers to `addons/<name>/runtime`. Rejected because it keeps
  runtime code under the addon tree and does not match `runtime/auth` and
  `runtime/contracts`.

## Follow-Up

- Prefer `runtime/<name>` imports in docs, examples, and generated-app extension
  snippets for request-time helpers.
- Keep `addons/<name>` re-exports during the 0.x migration window.

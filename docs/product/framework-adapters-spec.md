# Feature Spec: Framework Adapter Modules

## Problem

Go applications that already use Chi, Echo, or Gin need to mount a generated
GOWDK app without switching the host router or asking GOWDK to generate
framework-specific code.

## Goals

- Keep generated apps `net/http` first.
- Provide optional nested adapter modules for Chi, Echo, and Gin.
- Mount generated routes from GOWDK OpenAPI method/path metadata.
- Support host-app mount prefixes while dispatching the stripped path to the
  generated handler.
- Keep framework dependencies out of the root module.

## Non-Goals

- Generating Chi, Echo, or Gin code.
- Discovering routes from framework registration calls.
- Moving generated CSRF, body-limit, panic-recovery, guard, action, API,
  fragment, or SSR behavior into framework middleware.
- Fiber conformance through this slice.

## Users And Permissions

- Primary users: Go application developers embedding generated GOWDK routes in
  an existing framework app.
- Roles or permissions: framework adapters do not change app auth or generated
  guard semantics.
- Data visibility rules: adapters pass the request through to the generated
  handler and must not log request bodies or secret values.

## User Flow

1. Build a GOWDK app and get the generated `http.Handler`.
2. Read the generated `openapi.json` report.
3. Mount the generated routes into the host framework with an optional prefix.
4. Let the generated handler own dispatch, request decoding, guards, CSRF, and
   response writing.

## Requirements

### Functional

- `runtime/adapters` exposes dependency-free helpers for OpenAPI route
  extraction, mount-prefix handling, local URL rebasing, and GOWDK
  route-pattern translation.
- `runtime/adapters/chi`, `runtime/adapters/echo`, and `runtime/adapters/gin`
  expose `MountOpenAPI` and `MountRoutes`.
- `MountOpenAPI` keeps a fallback generated handler mount for page and asset
  routes not listed in OpenAPI.
- OpenAPI operations carry the original GOWDK route pattern in `x-gowdk.route`
  so final rest params survive host route registration.
- Existing catch-all `Mount` helpers remain available.
- Chi translates params to `{name}` and final rest params to `*`.
- Echo translates params to `:name`.
- Gin translates params to `:name` and reports ambiguous patterns as mount-time
  errors.
- Prefix mounting strips the host prefix before the generated handler sees the
  request path.
- Prefix mounting rebases same-origin root-relative `Location` headers and
  generated HTML URLs under the mount prefix.

### Non-Functional

- Performance: adapters perform registration-time translation and no route
  reflection at request time.
- Reliability: malformed route patterns fail during mounting.
- Accessibility: no UI impact.
- Security/privacy: generated protections remain in generated handlers; docs
  warn against duplicating body-limit, CSRF, and recovery policy blindly.
- Observability: framework context accessors remain integration escape hatches.

## Acceptance Criteria

- [x] Chi adapter module mounts and serves generated routes.
- [x] Echo adapter module mounts and serves generated routes.
- [x] Gin adapter module mounts and serves generated routes.
- [x] Shared conformance helper verifies served routes against OpenAPI
  paths/methods and mount-prefix stripping.
- [x] OpenAPI mounts keep page and asset fallback routing even when endpoint
  routes exist.
- [x] Rest params survive OpenAPI-based mounting through `x-gowdk.route`.
- [x] Gin ambiguous route conflicts return an error naming both routes.
- [x] Root `go.mod` remains unchanged.

## Edge Cases

- Root route mounted below a prefix dispatches to generated `/`.
- Dynamic params are translated for host framework registration, but generated
  route matching remains the source of truth.
- Rest params can be translated from GOWDK route metadata; OpenAPI path syntax
  represents dynamic params only as `{name}`.

## Dependencies

- Internal: `runtime/adapters`, generated `openapi.json`.
- External: nested optional modules only:
  `github.com/go-chi/chi/v5`, `github.com/labstack/echo/v5`, and
  `github.com/gin-gonic/gin`.

## Open Questions

- Should a future generated app expose route metadata directly in Go in
  addition to `openapi.json`?
- Should OpenAPI server URL configuration move into build config instead of
  adapter-side helper use?

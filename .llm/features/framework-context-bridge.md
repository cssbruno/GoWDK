# Feature Spec: Framework Context Bridge

## Problem

GOWDK generated apps expose `net/http` handlers. That keeps the runtime portable,
but applications mounted through optional Gin, Echo, or Fiber adapters sometimes
need framework-specific request data that middleware already computed.

## Goals

- Keep generated apps and GOWDK Runtime `net/http` first.
- Let code reached by a GOWDK request inspect the active Gin, Echo, or Fiber
  context when the optional adapter is used.
- Keep framework dependencies isolated in nested adapter modules.

## Non-Goals

- Make Gin, Echo, or Fiber route definitions a source of truth for GOWDK routes.
- Generate framework-specific app code by default.
- Replace GOWDK request context helpers such as `runtime/app.Request(ctx)`.

## Requirements

### Functional

- Each optional HTTP adapter attaches the current framework context to the
  request `context.Context` before calling the generated `http.Handler`.
- Each adapter exposes a package-specific accessor:
  - `runtime/adapters/gin.Context(ctx)`
  - `runtime/adapters/echo.Context(ctx)`
  - `runtime/adapters/fiber.Context(ctx)`
- Accessors return `false` when the GOWDK handler is not mounted through that
  framework adapter.

### Non-Functional

- Performance: context attachment must add only one context value per request.
- Reliability: Fiber must continue using Fiber's supported `net/http` adaptor.
- Security/privacy: framework contexts are available only to code already
  running for the same request.

## Acceptance Criteria

- [ ] Gin, Echo, and Fiber adapter tests prove the framework context is visible
      through `request.Context()` inside the wrapped handler.
- [ ] Framework integration docs show the bridge as an optional escape hatch.
- [ ] Root module tests still do not import Gin, Echo, or Fiber.

## Dependencies

- Internal: optional adapter modules under `runtime/adapters`.
- External: Gin, Echo, and Fiber dependencies remain nested module dependencies.

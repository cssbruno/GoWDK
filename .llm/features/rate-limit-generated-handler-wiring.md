# Feature Spec: Rate-Limit Generated Handler Wiring

## Problem

GOWDK already provides `addons/ratelimit` middleware, fixed-window decisions,
an in-memory store, and a Redis-backed store adapter. Generated action, API,
SSR, and split-backend proxy routes still need a generated registration hook so
apps can apply the addon to request-time handlers without making rate limiting
core behavior or importing a concrete Redis client.

## Goals

- Keep rate limiting optional and disabled unless the ratelimit addon is enabled
  and the generated app registers a limiter.
- Wire generated request-time lanes through a typed appgen hook.
- Keep generated code framework-neutral and emitted through Go AST.
- Let user-owned Go provide in-memory, Redis, or custom stores.

## Non-Goals

- Adding `.gwdk` rate-limit syntax.
- Adding a concrete Redis client dependency.
- Rate-limiting static assets by default.
- Moving auth, validation, or business policy into generated code.

## Users And Permissions

- Primary users: Go developers shipping generated GOWDK binaries.
- Roles or permissions: no new permission model.
- Data visibility rules: limiter keys are owned by the configured addon key
  function; generated code does not log request data.

## User Flow

1. The project enables `ratelimit.Addon()` in `gowdk.Config`.
2. Generated app source includes `RegisterRateLimiter`.
3. User-owned Go creates `*ratelimit.Limiter` with an in-memory or custom Redis
   store and registers it during app startup.
4. Generated request-time handlers write rate-limit headers for allowed
   requests and return HTTP 429 when the limit is exceeded.

## Requirements

### Functional

- Generated action, API, SSR, and split-backend proxy handlers call the limiter
  before guards and user handler logic.
- When no limiter is registered, request-time handlers continue normally.
- Limiter store errors return the addon default error handler.
- Exceeded limits return the addon default limit handler and `Retry-After`.
- Generated code imports `addons/ratelimit` only when the feature is enabled and
  generated request-time handlers exist.

### Non-Functional

- Performance: one limiter decision per generated request-time route dispatch.
- Reliability: generated source must format and compile.
- Accessibility: no UI impact.
- Security/privacy: no concrete Redis dependency or secret handling in generated
  code.
- Observability: standard rate-limit response headers are written by the addon.

## Acceptance Criteria

- [x] Generated source exposes `RegisterRateLimiter(*ratelimit.Limiter)`.
- [x] Generated handlers call the registered limiter before guards and user
      logic.
- [x] A generated binary with a registered in-memory limiter returns 429 after
      the configured request budget.
- [x] Docs explain in-memory and Redis-client registration without requiring a
      concrete Redis dependency.

## Edge Cases

- Ratelimit addon enabled but no request-time handlers exist.
- Ratelimit addon enabled but no limiter is registered.
- Limiter store returns an error.
- Split frontend proxy and backend binary both opt into their own limiter.

## Dependencies

- Internal: `internal/appgen`, `addons/ratelimit`.
- External: none.

## Open Questions

- Future `.gwdk` or config syntax may define per-route policies once cache and
  route policy syntax is stable.

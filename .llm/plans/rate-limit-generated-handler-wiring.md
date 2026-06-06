# Implementation Plan: Rate-Limit Generated Handler Wiring

## Context

Spec: `.llm/features/rate-limit-generated-handler-wiring.md`

## Assumptions

- Rate limiting remains optional and addon-gated.
- User-owned Go registers the limiter; generated code does not create Redis
  clients or choose production policy.

## Proposed Changes

- Add appgen AST helpers for `RegisterRateLimiter`, a package-level limiter, and
  `runRateLimit`.
- Thread rate-limit checks into generated action, API, SSR, and split-backend
  proxy handlers before guards and user logic.
- Import `addons/ratelimit` only when the feature and request-time routes are
  present.
- Document registration with in-memory and Redis store examples.

## Files Expected To Change

- `internal/appgen/source_rate_limit.go`
- `internal/appgen/source.go`
- `internal/appgen/source_actions.go`
- `internal/appgen/source_api.go`
- `internal/appgen/source_ssr.go`
- `internal/appgen/source_backend.go`
- `internal/appgen/appgen_test.go`
- `docs/reference/addons.md`
- `docs/reference/config.md`
- `docs/product/requirements.md`
- `docs/engineering/architecture.md`

## Data And API Impact

- Generated app packages gain `RegisterRateLimiter(*ratelimit.Limiter)` when
  rate limiting is enabled for request-time routes.
- No persisted data format changes.

## Tests

- Unit: generated source assertions for imports, registration hook, and route
  ordering.
- Integration: generated binary with registered in-memory limiter returns 429
  after the configured request budget.
- End-to-end: covered by generated binary appgen test.
- Manual: optional local app can register Redis store adapter.

## Verification Commands

```sh
go test ./internal/appgen
go test ./addons/ratelimit ./internal/appgen
go test ./...
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Remove generated registration declarations and rate-limit statements from
  appgen; the standalone addon middleware remains usable in user-owned servers.

## Risks

- Generated frontend proxy and backend binaries can both rate-limit if both
  register limiters; docs should call this out as a deployment choice.

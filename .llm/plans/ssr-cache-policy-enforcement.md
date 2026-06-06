# Implementation Plan: SSR Cache Policy Enforcement

## Context

Spec: `.llm/features/ssr-cache-policy-enforcement.md`

## Assumptions

- `@cache` remains a literal HTTP `Cache-Control` value.
- Cache safety is explicit author responsibility for SSR pages.
- Non-success request-time responses remain no-store.

## Proposed Changes

- Add a runtime HTML writer that accepts an explicit cache policy and falls back
  to no-store.
- Update SSR appgen to call the explicit-cache writer when `SSRRoute.Cache` is
  set.
- Add generated-source and generated-binary tests.
- Update cache documentation and requirements status.

## Files Expected To Change

- `runtime/response/response.go`
- `runtime/response/response_test.go`
- `internal/appgen/source_ssr.go`
- `internal/appgen/appgen_test.go`
- `docs/reference/deployment.md`
- `docs/reference/routing.md`
- `docs/product/requirements.md`

## Data And API Impact

- Runtime response API gains `WriteHTML(http.ResponseWriter, *http.Request,
  string, string) error`.
- No persisted data format changes.

## Tests

- Unit: runtime response cache writer tests.
- Integration: appgen generated source tests.
- End-to-end: generated binary SSR response header test.
- Manual: run a generated app with an `@cache` SSR page.

## Verification Commands

```sh
go test ./runtime/response ./internal/appgen
go test ./...
go build ./cmd/gowdk
git diff --check
```

## Rollback Plan

- Revert appgen to `WriteNoStoreHTML` for every SSR response and keep `@cache`
  metadata informational only.

## Risks

- Authors can cache personalized SSR output if they opt into `@cache` on a page
  backed by per-user `load {}` data. Documentation must keep this explicit.

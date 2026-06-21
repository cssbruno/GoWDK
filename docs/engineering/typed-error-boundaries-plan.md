# Implementation Plan: Typed Error Boundaries

## Context

Spec: `docs/product/typed-error-boundaries.md`

Issue: `cssbruno/GoWDK#510`

## Assumptions

- The smallest safe slice is typed expected errors plus generated SSR status
  mapping.
- Layout-level error-boundary composition needs a separate source contract and
  diagnostics design.

## Proposed Changes

- Add expected error kinds and helper constructors to `runtime/response`.
- Keep helpers backed by `HandlerError` so existing generated endpoint logic
  continues to use `HandlerStatus` and `HandlerErrorMessage`.
- Change generated SSR load error handling to compute status from
  `HandlerStatus`.
- Add generated-binary coverage for expected not-found SSR load errors using
  generated `404.html`.
- Update error docs and product status.

## Files Expected To Change

- `runtime/response`
- `internal/appgen/source_ssr.go`
- `internal/appgen/appgen_test.go`
- `docs/product`, `docs/engineering`, `docs/reference`, `docs/language`

## Data And API Impact

- Adds public `response.ErrorKind` constants and expected-error constructors.
- Generated SSR apps may now return 4xx/422 statuses for load errors that wrap
  `response.HandlerError` or use the new helpers.
- Existing ordinary load errors still return HTTP 500.

## Tests

- Unit: expected error helper status mapping.
- Integration: generated source includes `HandlerStatus` for SSR load errors.
- End-to-end: generated binary serves a `response.NotFound` SSR load error as
  HTTP 404 with `404.html`.

## Verification Commands

```sh
go test ./runtime/response ./internal/appgen
go test ./...
scripts/test-go-modules.sh
go build ./cmd/gowdk
```

## Rollback Plan

- Revert expected-error helpers and generated SSR status mapping.
- Existing `response.NewHandlerError` and ordinary error handling remain the
  fallback.

## Risks

- Client-facing expected-error messages are app-owned text. Docs must warn users
  not to include secrets.
- Layout-level error boundary syntax remains unresolved and should not be
  implied by this slice.

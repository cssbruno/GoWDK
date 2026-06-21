# Implementation Plan: Typed Error Boundaries

## Context

Spec: `docs/product/typed-error-boundaries.md`

Issue: `cssbruno/GoWDK#510`

## Assumptions

- Expected error messages are explicit app-owned client text.
- File-backed layout `error` metadata is the smallest coherent layout boundary
  contract because it reuses existing generated HTML error documents.
- Templated in-layout error regions need a separate source contract and
  diagnostics design.

## Proposed Changes

- Add expected error kinds and helper constructors to `runtime/response`.
- Keep helpers backed by `HandlerError` so existing generated endpoint logic
  continues to use `HandlerStatus` and `HandlerErrorMessage`.
- Change generated SSR load error handling to compute status from
  `HandlerStatus`.
- Let layout files declare `error "/errors/layout.html"` and lower that through
  parser, IR, buildgen, appgen route metadata, and runtime error-page lookup.
- Select SSR 500 boundaries in this order: endpoint/route-local error page,
  nearest layout error page, outer layout error pages, global `500.html`, then
  `http.Error`.
- Add generated-binary coverage for expected not-found SSR load errors using
  generated `404.html`.
- Update error docs and product status.

## Files Expected To Change

- `runtime/response`
- `runtime/app`
- `internal/parser`, `internal/gwdkir`, `internal/buildgen`
- `internal/appgen/source_ssr.go`
- `internal/appgen/appgen_test.go`
- `docs/product`, `docs/engineering`, `docs/reference`, `docs/language`

## Data And API Impact

- Adds public `response.ErrorKind` constants and expected-error constructors.
- Adds layout `ErrorPage` metadata to compiler IR.
- Adds generated route `LayoutErrorPages` metadata consumed by runtime error
  page lookup.
- Generated SSR apps may now return 4xx/422 statuses for load errors that wrap
  `response.HandlerError` or use the new helpers.
- Existing ordinary load errors still return HTTP 500.
- Existing route-local error pages keep precedence over layout boundaries.

## Tests

- Unit: expected error helper status mapping.
- Parser/buildgen: layout `error` metadata lowers into ordered boundary
  metadata.
- Runtime: route-local error pages win, layout error pages are selected before
  global `500.html`, and missing layout documents fall through.
- Integration: generated source includes `HandlerStatus` for SSR load errors.
- End-to-end: generated binary serves a `response.NotFound` SSR load error as
  HTTP 404 with `404.html`.
- End-to-end: generated binary serves an ordinary SSR load failure with the
  nearest layout-level generated error page.

## Verification Commands

```sh
go test ./runtime/response ./internal/appgen
go test ./...
scripts/test-go-modules.sh
go build ./cmd/gowdk
```

## Rollback Plan

- Revert expected-error helpers and generated SSR status mapping.
- Revert layout `error` metadata and runtime lookup additions.
- Existing `response.NewHandlerError` and ordinary error handling remain the
  fallback.

## Risks

- Client-facing expected-error messages are app-owned text. Docs must warn users
  not to include secrets.
- File-backed layout boundaries do not render typed error values inside layout
  templates; that remains future syntax.

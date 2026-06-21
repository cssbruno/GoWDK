# Feature Spec: Typed Error Boundaries

## Problem

Generated request-time lanes can hide internal failures, but expected application
errors are still mostly status integers and route-local fallback pages. SSR load
errors in particular were treated as internal server errors even when the app
intended a not-found, forbidden, or validation response.

## Goals

- Provide typed expected-error helpers for common generated-boundary categories.
- Map expected errors to stable HTTP statuses in generated SSR load handling.
- Let layout files declare generated HTML error boundaries that compose with
  route-local and global error pages.
- Preserve no-store generated error boundaries and production detail hiding.

## Non-Goals

- Do not expose panic values, stack traces, submitted form values, or secrets.
- Do not add templated in-layout error-region syntax in this slice.
- Do not replace app-owned normal `response.Response` results for actions/APIs.

## Users And Permissions

- Primary users: app developers writing SSR load handlers and generated endpoint
  handlers.
- Roles or permissions: no new authorization model.
- Data visibility rules: helper messages are client-facing application text;
  ordinary 5xx details remain hidden.

## User Flow

1. A load handler detects an expected condition.
2. The handler returns `response.NotFound`, `response.Forbidden`,
   `response.ValidationFailed`, or `response.ServerError`.
3. Generated SSR writes a no-store response with the mapped status and the
   applicable generated error page or safe fallback text.
4. Internal SSR failures prefer a route-local `error` page, then the nearest
   loaded layout-level `error` page, then outer layout boundaries, then
   `500.html`.

## Requirements

### Functional

- Expected not-found errors map to HTTP 404.
- Expected forbidden errors map to HTTP 403.
- Expected validation errors map to HTTP 422.
- Expected server errors map to HTTP 500.
- SSR load errors use `response.HandlerStatus` instead of forcing 500.
- Layout files can declare `error "/errors/layout.html"`.
- Layout-level error pages compose with SSR route metadata in nearest-to-
  outermost order after route-local `error` pages and before global `500.html`.

### Non-Functional

- Security/privacy: ordinary 5xx error details stay hidden.
- Reliability: existing `response.NewHandlerError` behavior remains compatible.
- Performance: no runtime reflection or new dependencies.

## Acceptance Criteria

- [x] Runtime helpers produce typed `HandlerError` values with stable statuses.
- [x] Generated SSR load errors honor expected statuses.
- [x] Generated 404 pages are selected for expected not-found SSR load errors.
- [x] Layout-level generated HTML error pages are selected for SSR 500
  boundaries when no route-local error page applies.
- [x] Docs state that richer templated error-region syntax remains planned.

## Edge Cases

- Empty expected-error messages default to the HTTP status text.
- Existing route-local `error` pages still apply to internal server errors.
- Expected 403 and 422 responses use safe fallback text until status-specific
  generated documents are defined.
- Missing layout-level error documents fall through to the next boundary.

## Dependencies

- Internal: `runtime/response`, generated SSR adapter code, runtime error pages,
  parser/layout IR metadata.
- External: none.

## Open Questions

- Should action/API endpoint-local error pages render expected 4xx statuses, or
  remain panic/internal-error boundaries only?
- Should a later source contract support templated in-layout error regions with
  typed error values?

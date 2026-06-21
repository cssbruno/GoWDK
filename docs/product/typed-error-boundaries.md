# Feature Spec: Typed Error Boundaries

## Problem

Generated request-time lanes can hide internal failures, but expected application
errors are still mostly status integers and route-local fallback pages. SSR load
errors in particular were treated as internal server errors even when the app
intended a not-found, forbidden, or validation response.

## Goals

- Provide typed expected-error helpers for common generated-boundary categories.
- Map expected errors to stable HTTP statuses in generated SSR load handling.
- Preserve no-store generated error boundaries and production detail hiding.

## Non-Goals

- Do not expose panic values, stack traces, submitted form values, or secrets.
- Do not add layout-level error boundary syntax in this slice.
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

## Requirements

### Functional

- Expected not-found errors map to HTTP 404.
- Expected forbidden errors map to HTTP 403.
- Expected validation errors map to HTTP 422.
- Expected server errors map to HTTP 500.
- SSR load errors use `response.HandlerStatus` instead of forcing 500.

### Non-Functional

- Security/privacy: ordinary 5xx error details stay hidden.
- Reliability: existing `response.NewHandlerError` behavior remains compatible.
- Performance: no runtime reflection or new dependencies.

## Acceptance Criteria

- [x] Runtime helpers produce typed `HandlerError` values with stable statuses.
- [x] Generated SSR load errors honor expected statuses.
- [x] Generated 404 pages are selected for expected not-found SSR load errors.
- [x] Docs state that layout-level error boundaries remain planned.

## Edge Cases

- Empty expected-error messages default to the HTTP status text.
- Existing route-local `error` pages still apply to internal server errors.
- Expected 403 and 422 responses use safe fallback text until typed/layout
  boundary rendering exists for those statuses.

## Dependencies

- Internal: `runtime/response`, generated SSR adapter code, runtime error pages.
- External: none.

## Open Questions

- What source syntax should layout-level error boundary composition use?
- Should action/API endpoint-local error pages render expected 4xx statuses, or
  remain panic/internal-error boundaries only?

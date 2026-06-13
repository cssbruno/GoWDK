# M7 SSR And Hybrid Implementation Plan

This plan records the M7 closure scope for GitHub issues #7, #9, #10, #25,
#63, and #177.

## Implementation

- Retire the `fragment_dynamic_route` diagnostic and allow fragment endpoint
  routes to use `{name}`, `{name:type}`, and final-segment `{name...}` params.
- Populate `appgen.FragmentEndpoint.RouteParams` from `internal/gwdkir` route
  text.
- Generate standalone fragment handlers with exact static cases first and
  ordered `runtime/route.Match` checks for dynamic routes.
- Attach raw params through `runtime/app.WithParams` and decoded typed params
  through `runtime/app.WithTypedParams` before rate limits, guards, static
  fallback output, or same-package fragment hooks run.
- Add `runtime/guard` redirect and custom response helpers and have generated
  guard failures write those responses with the existing no-store policy.
- Let generated backend-only apps and split frontend proxy checks dispatch
  dynamic fragment route patterns.
- Extend dynamic-route ambiguity validation from page/rest-only coverage to the
  same-method generated request namespace, including fragments, APIs, actions,
  Go endpoints, and contract references.
- Update partials, routing, deployment, hooks, product requirements, roadmap,
  architecture, and release-plan docs with current M7 behavior and deferred
  hybrid/guard/cache follow-ups.

## Verification

- `go test ./internal/source`
- `go test ./runtime/app`
- `go test ./internal/compiler`
- `go test ./internal/appgen`
- `go test ./...`
- `scripts/test-go-modules.sh`

## Follow-Ups

- Broader SSR and fragment examples remain tracked in the release plan.
- Richer request-local state beyond the current context helpers remains planned.
- Hybrid streaming, browser-owned data refresh, and non-HTTP revalidation remain
  deferred until the base request-time lane is stable.

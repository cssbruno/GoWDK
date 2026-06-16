# M7 SSR And Hybrid Spec

M7 hardens the current request-time page lane. GOWDK still defaults pages to
build-time SPA output; request-time page behavior is selected explicitly with
`server {}` or `go server {}` and requires the SSR addon.

## In Scope

- Concrete and dynamic request-time SSR pages in generated binaries.
- `server { => { field, nested.path } }` execution through same-package Go load
  functions.
- Raw and typed route params through `runtime/app.Params(ctx)` and
  `runtime/app.TypedParams(ctx)`.
- Standalone concrete and dynamic fragment routes, including typed fragment
  route params for request-time fragment hooks.
- Route-local SSR `error` pages, optional generated `404.html`/`500.html`, and
  no-store panic boundaries for generated request-time lanes.
- Guard enforcement for generated SSR, action, API, and fragment routes through
  `runtime/guard` and optional native RBAC through `runtime/auth`, including
  no-store guard redirect and response helpers.
- Current HTTP cache policy: generated assets use asset-manifest policy, SPA
  HTML defaults to `no-cache`, request-time endpoint responses default to
  `no-store`, and page `cache` / `revalidate` compile into Cache-Control for
  successful SPA and SSR HTML.
- Hybrid source contract decision: no separate `.gwdk` hybrid syntax yet.
  Hybrid remains internal/configured route metadata; source authors choose
  request-time page behavior with `server {}` or `go server {}`.

## Out Of Scope

- Hybrid streaming.
- Browser-owned server-data refresh.
- Non-HTTP revalidation.
- Implicit action invalidation of page load data.
- Richer request-local state beyond the current `context.Context`,
  `runtime/app`, and `runtime/guard.Context` helpers.
- Generated per-route param struct types.

## Acceptance

- Dynamic fragment routes compile, route, and pass raw and typed params to
  same-package fragment hooks.
- Dynamic fragment paths participate in same-method route conflict validation
  with pages, fragments, APIs, actions, Go endpoints, and contract routes.
- Current SSR, hybrid, cache, guard, and request context contracts are
  documented in the product, language, routing, deployment, hooks, and
  architecture docs.
- Guards can return ordinary errors for fail-closed 403 responses or explicit
  guard helper errors for safe local redirects and custom no-store responses.

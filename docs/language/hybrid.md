# Hybrid Rendering

Hybrid rendering is a request-time page contract, not a separate page syntax.
The product-level source of truth is
[Hybrid Lifecycle Contract](../product/hybrid-lifecycle-spec.md).

## Source Contract

Pages default to build-time SPA output. A page uses the hybrid request-time lane
when project config sets `Render.Default` to `gowdk.Hybrid`, or when compiler IR
selects hybrid for a request-time page. Pages that declare `server {}` or
`go server {}` use the integrated request-time page lane with their effective
render mode.

Hybrid requires the SSR addon because generated hybrid handlers use the same
request-time page runtime as SSR. Without the feature gate, validation reports
`missing_ssr_addon`. A `server {}` block on a page that remains SPA reports
`server_requires_request_render`.

There is no `.gwdk` metadata declaration that means "hybrid" today.

## Entry And Navigation

Initial entry, direct browser refresh, and enhanced navigation to a generated
hybrid route use the same generated GET handler. Dynamic route params are
decoded before guards, load functions, or rendering run.

Build-time output evaluates `build {}` data for static values, but skips
request-time hybrid page HTML and records a `request_time_page_skipped`
build-report event with `data.mode=hybrid`. Generated app output serves the
route at request time.

If the page declares `server {}`, generated app output calls the same-package
load function on every request and replaces the generated request-time
placeholders in the page and layout render scope. A hybrid page without
`server {}` still uses the request-time route lane, but has no page load
function.

## Refresh

Hybrid refresh is explicit:

- Actions decide their own redirect, fragment, JSON, or reload result.
- Actions do not implicitly rerun page `server {}` data.
- Fragments own their own request-time data and return no-store fragment
  responses.
- `g:command` with a bound `g:query` region can return single-flight patches to
  the command caller.
- Realtime query invalidation can use `/_gowdk/realtime/query-refresh` for
  eligible public, parameterless SSR or hybrid query regions on the current
  route.
- Fragment-owned, API-owned, protected, dynamic, or wrong-route query regions do
  not synthesize patches; the browser runtime falls back to current-document
  refresh.

Generated JavaScript may coordinate navigation and refresh, but it does not own
route truth, authorization, server data, validation, action behavior, or cache
policy.

## Guards, Layouts, Errors, And Cache

Generated hybrid routes run page guards before request-time load or render
logic. Declared layouts compose at request time and can read the same
request-time render scope. Route-local and layout-level error documents follow
the SSR error boundary order. Missing routes remain ordinary generated-app
routing failures; hybrid does not create a SPA fallback for paths the route
table does not own.

Successful hybrid HTML uses the page `cache` and `revalidate` policy when
declared. Without an explicit page cache policy it uses
`Cache-Control: no-store`. Guard failures, load redirects, generated errors,
route-local error pages, fragments, actions, APIs, and CSRF-mutated HTML always
stay no-store.

## Reports

Config-selected hybrid pages appear as `hybrid` in `gowdk routes`. Route
reports include effective render mode, guards, layouts, route params, and page
cache policy when declared. Build reports include hybrid prerender skips and
cache-policy summaries.

## Unsupported

Current hybrid output does not support streaming responses, browser-owned
server-data refresh, non-HTTP revalidation, or implicit action invalidation of
page load data. There is no accepted source syntax for those behaviors today.

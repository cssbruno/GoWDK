# Hybrid Lifecycle Contract

Status: implemented for the current request-time hybrid slice.

## Source Contract

Hybrid has no separate page-level source declaration. Pages default to SPA
output. A page becomes request-time page output when:

- project config sets `Render.Default` to `gowdk.Hybrid`; or
- the page declares `server {}` or `go server {}` and the effective route mode
  is hybrid through config or IR.

Hybrid request-time pages require the SSR feature gate, because generated
hybrid handlers use the same integrated request-time page lane as SSR. Without
that feature, validation reports `missing_ssr_addon`. A `server {}` block on a
page that remains SPA reports `server_requires_request_render`.

The current source contract has no syntax for hybrid response streaming,
browser-owned server-data refresh, or non-HTTP revalidation. Those behaviors are
unsupported rather than implicit.

## Render Lifecycle

Build-time output still evaluates `build {}` for static values. Request-time
hybrid pages are skipped from prerendered HTML and recorded as
`request_time_page_skipped` build-report events with `data.mode=hybrid`.

Generated app output serves hybrid pages from the request-time page handler:

- Initial entry, direct browser refresh, and enhanced navigation to the same
  route all use the same generated GET handler.
- `build {}` values provide the static render scope.
- `server {}` values, when declared, are loaded per request and replace the
  generated request-time placeholders in the page and layout render scope.
- A hybrid page without `server {}` still uses the request-time route lane, but
  has no page load function.
- Dynamic params are decoded before guards, load functions, or rendering run.

Generated hybrid handlers compose declared layouts at request time. Route-local
and layout-level error documents follow the SSR error boundary order.

## Refresh Contract

Hybrid refresh is explicit:

- Action handlers decide their own redirect, fragment, JSON, or reload result.
  Actions do not implicitly rerun page `server {}` data.
- Standalone and action-returned fragments own their own request-time data and
  return no-store fragment responses.
- `g:command` plus bound `g:query` regions can return single-flight patches for
  the command caller.
- Query invalidation can use `/_gowdk/realtime/query-refresh` for eligible
  public, parameterless SSR or hybrid query regions on the current route.
- Unsupported fragment-owned, API-owned, protected, dynamic, or wrong-route
  query regions fall back to current-document refresh instead of synthesizing
  unsafe patches.

Generated JavaScript may enhance navigation and refresh orchestration, but it
does not own route truth, authorization, server data, validation, action
behavior, or cache policy.

## Guards, Errors, And Missing Routes

Generated hybrid routes run page guards before request-time load or render
logic. Guard failures, route param failures, redirects, route-local error pages,
panic boundaries, generated 404/500 pages, and missing routes use the same
no-store safety rules as SSR.

Missing generated routes stay ordinary generated-app routing failures. A hybrid
route does not create a SPA fallback for paths the route table does not own.

## Cache And Reports

Successful hybrid HTML uses the page `cache` and `revalidate` policy when
declared. Without an explicit page cache policy it is `Cache-Control: no-store`.
Safety responses always stay no-store, including guard failures, load
redirects, generated errors, route-local error pages, fragments, actions, APIs,
and CSRF-mutated HTML.

Route reports expose the effective page render mode as `hybrid`, route params,
guards, layouts, and page cache policy. Build reports expose hybrid prerender
skips and cache-policy summaries.

## Verification

The contract is covered by:

- `internal/compiler` validation tests for SSR feature gating and effective
  hybrid route metadata.
- `internal/buildgen` tests for hybrid request-time skip reporting and SSR
  artifact metadata.
- `internal/appgen` generated-binary tests for hybrid routes, request-aware
  layouts, guard/error no-store responses, route/query refresh, and hybrid
  cache headers.
- `internal/clientrt` browser-runtime tests for query refresh, command
  single-flight patches, document-refresh fallback, and navigation failure
  events.

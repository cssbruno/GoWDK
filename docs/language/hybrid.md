# Hybrid Rendering

Hybrid rendering is not exposed as separate source syntax.

Pages default to build-time SPA output. Use `server {}` or `go server {}` when a
page must run through generated request-time rendering. Both require the SSR
addon.

The compiler still has internal `hybrid` route metadata for generated route
reports and configured render defaults, but there is no page metadata
declaration for selecting hybrid behavior in `.gwdk` files. A page without
`server {}` remains build-time SPA output unless project config sets
`Render.Default` to `gowdk.Hybrid`; a page with `server {}` or `go server {}`
uses the integrated request-time page lane with the page's effective render
mode.

Current generated hybrid behavior is deliberately narrow:

- Concrete and dynamic request-time pages can be built into generated binaries.
- Config-selected hybrid pages appear as `hybrid` in `gowdk routes`.
- Build-time output skips hybrid pages and records a `request_time_page_skipped`
  build-report event with `data.mode=hybrid`.
- Page-level `cache` and `revalidate` use the same HTTP Cache-Control contract
  as SPA and SSR HTML.
- Actions and fragments refresh data explicitly through redirects, fragment
  responses, JSON, or reload responses.

Deferred hybrid behavior:

- streaming responses;
- browser-owned server-data refresh;
- non-HTTP revalidation;
- implicit action invalidation of page load data.

# Data Lifecycle

GOWDK has separate build-time, request-time page, and endpoint data lanes.
Generated JavaScript does not own page loading policy.

## Lane Summary

| Construct | Runs | Owns | Current contract |
| --- | --- | --- | --- |
| `paths {}` | build time | concrete dynamic SPA routes | Literal records only. Required for dynamic SPA pages unless the page uses request-time rendering. |
| `build {}` | build time | static page data | Literal records plus imported or same-package Go functions, with optional `gowdk.BuildParams` route params. |
| `server {}` | request time | SSR page data | One same-package `Load<PageID>` function returns `map[string]any` data or an exported typed result struct. |
| `act` | request time | POST/action endpoint behavior | Same-package Go handler returns `runtime/response.Response`. |
| `api` | request time | API endpoint behavior | Same-package Go handler returns `runtime/response.Response`. |
| `fragment` | request time | partial endpoint behavior | Same-package Go hook or static generated fragment body. |

## Current Rules

- `build {}` data is rendered into generated static output. It must not depend
  on the incoming HTTP request.
- `server {}` selects request-time SSR and requires the SSR addon.
- Generated SSR calls one same-package function named `Load<PageID>`.
- Supported load signatures are:

```go
func LoadDashboard(ssr.LoadContext) map[string]any
func LoadDashboard(ssr.LoadContext) (map[string]any, error)
func LoadDashboard(ssr.LoadContext) DashboardData
func LoadDashboard(ssr.LoadContext) (DashboardData, error)
```

- One `server {}` block can declare multiple fields. They come from the single
  returned map or typed result struct, including dotted paths such as
  `user.name`.
- Typed load result structs must be exported same-package structs. Exported
  fields are visible by Go field name or `json` tag name; `json:"-"` hides a
  field. Generated SSR adapters convert top-level struct fields into the
  existing load-data map without runtime reflection.
- Layouts do not have independent `server {}` data yet. Request-time layout data
  composition is planned.
- Load redirects use `ssr.RedirectTo("/path")` or
  `ssr.Redirect("/path", status)`. Redirect targets must be local absolute
  paths.
- Not-found, forbidden, validation, and typed expected-error helpers for load
  are planned. Today, guards handle guarded access and other load errors use
  the generated SSR error-page path.
- For typed load result structs, declared `server {}` field paths are checked
  against exported result fields. Map-returning load functions remain dynamic.

## Invalidation And Refresh

- Full POST actions and enhanced POST actions share the same user Go handler
  ownership. The handler response decides redirect, HTML, JSON, or fragment
  behavior.
- GOWDK does not automatically rerun `server {}` after an action today.
- Partial updates use explicit fragment responses or standalone fragment
  endpoints. Fragments own their request-time data through the fragment Go hook.
- Fragments do not declare compiler-tracked data dependencies today.
- Generated client navigation does not prefetch or reuse `server {}` data today.
  Any future prefetch or reuse must be an explicit generated-client feature,
  not hidden browser-owned loading policy.

## Boundaries

- User Go owns auth, business validation, storage, service calls, and response
  semantics.
- Generated Go owns adapter glue: decode, dispatch, context metadata, response
  writing, guards, CSRF checks, panic boundaries, and cache defaults.
- Generated JavaScript may enhance form submissions, fragments, islands, and
  static SPA navigation. It must not become the authority for routes, auth,
  validation, server data, action behavior, cache, or page loading policy.
- Actions do not invalidate `server {}` data implicitly. Use redirects,
  fragments, JSON, or `response.ReloadPage()` to make the lifecycle visible in
  the action result.

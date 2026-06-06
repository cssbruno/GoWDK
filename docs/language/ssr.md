# SSR

SSR is optional and must not become the default framework identity.

## Current Support

- `@render ssr` is parsed as a render mode.
- `@render ssr` requires the SSR addon during validation.
- `@render hybrid` defaults to SPA output. It requires the SSR addon only when
  explicit request-time behavior such as `load {}` is present.
- `gowdk build --ssr --app <dir> --bin <file>` can generate a binary that
  serves first-slice `@render ssr` pages rendered from `view {}` and literal or
  imported `build {}` data.
- Dynamic SSR routes such as `/blog/{slug}` can be matched by generated
  binaries in the first supported slice. Route params render through generated
  placeholders and request-time HTML escaping.
- `load {}` is allowed only with `@render ssr` or `@render hybrid`. The first
  generated execution slice supports field declarations such as
  `load { => { user, title } }` and calls a same-package exported Go function
  named `Load<PageID>`.
- Supported load function signatures are
  `func LoadDashboard(ssr.LoadContext) map[string]any` and
  `func LoadDashboard(ssr.LoadContext) (map[string]any, error)`. Returned
  scalar values replace generated SSR placeholders with request-time HTML
  escaping.
- Load functions can return `ssr.RedirectTo("/login")` or
  `ssr.Redirect("/login", http.StatusTemporaryRedirect)` to ask generated SSR
  handlers to write a no-store local redirect. Redirect URLs must be local
  absolute paths.
- Generated embedded apps load optional `404.html` and `500.html` documents
  from build output and serve them for not-found responses and generated SSR
  load failures. Missing error documents fall back to `http.Error`.
- Generated SSR route handlers run inside a runtime panic boundary. A panic
  before response headers are written becomes a no-store HTTP 500 response,
  using `500.html` when present, without exposing the panic value.
- The SSR addon exposes a small router registration contract for generated SSR
  page handlers.
- The SSR addon provides a default HTTP 500 error handler contract for
  request-time SSR failures.
- `@guard` uses comma-separated guard IDs such as `@guard auth.required,
  billing.active`. The SSR addon exposes `GuardFunc`, `GuardRegistry`, and
  ordered guard execution contracts. Generated SSR, action, and API handlers
  run declared guards before user logic and fail closed with HTTP 403 when a
  guard is missing or returns an error.

## Planned Support

Future SSR work must define request layouts, custom error boundaries, route
registration integration, and exactly how hybrid pages avoid becoming implicit
full-page SSR.

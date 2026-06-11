# SSR

SSR is optional and must not become the default framework identity.

## Current Support

- Pages default to build-time SPA output.
- `load {}` selects request-time SSR and requires the SSR addon.
- `go ssr {}` also selects request-time SSR and requires the SSR addon.
- `gowdk build --ssr --app <dir> --bin <file>` can generate a binary that
  serves concrete and dynamic request-time SSR pages rendered from `view {}`,
  literal or imported `build {}` data, and declared `load {}` data.
- Dynamic SSR routes such as `/blog/{slug}` can be matched by generated
  binaries in the first supported slice. Route params render through generated
  placeholders and request-time HTML escaping.
- Generated SSR supports declared identifier and dotted-path fields such as
  `load { => { user, title, account.plan } }` and calls a same-package exported
  Go function named `Load<PageID>`. `<PageID>` is the explicit `page` value
  when present, otherwise the filename-derived page ID.
- Supported load function signatures are
  `func LoadDashboard(ssr.LoadContext) map[string]any` and
  `func LoadDashboard(ssr.LoadContext) (map[string]any, error)`. Returned
  values replace generated SSR placeholders with request-time HTML escaping.
  Dotted paths resolve through nested maps with string keys, structs, pointers,
  interfaces, exported Go field names, and `json` tag names.
- Load functions can return `ssr.RedirectTo("/login")` or
  `ssr.Redirect("/login", http.StatusTemporaryRedirect)` to ask generated SSR
  handlers to write a no-store local redirect. Redirect URLs must be local
  absolute paths.
- `error "/errors/dashboard.html"` declares a route-local generated HTML
  error document for SSR load failures, generated render failures, and route
  panics before response headers are written on that page. The path is
  output-relative, may be written with a leading slash, must end in `.html`,
  and must not contain `..`, query strings, fragments, or backslashes.
- Generated embedded apps load optional `404.html` and `500.html` documents
  from build output, plus any route-local `error` documents selected by SSR
  routes. Missing error documents fall back to `http.Error`.
- Generated SSR route handlers run inside a runtime panic boundary. A panic
  before response headers are written becomes a no-store HTTP 500 response,
  using the route-local `error` page when declared or `500.html` when present,
  without exposing the panic value.
- The SSR addon exposes a small router registration contract for generated SSR
  page handlers.
- The SSR addon provides a default HTTP 500 error handler contract for
  request-time SSR failures.
- `guard` is optional, but a page is not public by default: a page that
  declares no `guard` warns (`missing_page_guard`) and its route is denied
  (403) at request time until access is stated (see
  [guards.md](guards.md) for the full access contract). `guard public` marks an
  intentionally public page and must stand alone. Non-public guards use
  comma-separated guard IDs such as `guard auth.required, billing.active`.
  Protected page guards require request-time page rendering so the page GET
  route can be gated before HTML is returned. The SSR addon exposes
  `GuardFunc`, `GuardRegistry`, and ordered guard execution contracts.
  Generated SSR, action, and API handlers run declared guards before user
  logic. A guarded generated app will not compile unless required guard backing
  functions exist. Native RBAC guard IDs use `role:<name>` and
  `permission:<name>` and resolve through an application-owned
  `runtime/auth.Provider`.

Generated app packages that include at least one guarded SSR, action, or API
route require backing functions in the generated app package:

```go
func GOWDKGuardRegistry() ssr.GuardRegistry // required when custom guard IDs are used
func GOWDKAuthProvider() auth.Provider      // required when role:/permission: IDs are used
```

Define custom guards in app startup code that is compiled with the generated app
package. If this function is missing while custom guard IDs are declared, `go
build` fails.

```go
package gowdkapp

import gowdkssr "github.com/cssbruno/gowdk/addons/ssr"

func GOWDKGuardRegistry() gowdkssr.GuardRegistry {
	return gowdkssr.GuardRegistry{
		"auth.required": func(ctx gowdkssr.LoadContext) error {
			return nil
		},
	}
}
```

For native RBAC guards, define only the application-owned principal source. If
this function is missing while `role:` or `permission:` guard IDs are declared,
`go build` fails.

```go
import (
	"net/http"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
)

func GOWDKAuthProvider() gowdkauth.Provider {
	return gowdkauth.ProviderFunc(func(request *http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{
			ID:          "user-1",
			Roles:       []string{"admin"},
			Permissions: []string{"dashboard.read"},
		}, nil
	})
}
```

Feature packages that declare page, action, or API handlers should not import
the generated `gowdkapp` package. Keep registration in the generated app
package to avoid import cycles.

Native RBAC guards are a defense-in-depth redundancy layer for generated
route/page access. They must never replace backend authorization for protected
resources in normal Go handlers and services.

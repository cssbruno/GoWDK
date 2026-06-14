# Hooks, Guards, And Middleware

GOWDK's current hook model is small and `net/http`-first.

## Current Contracts

| Extension point | Type | Scope |
| --- | --- | --- |
| Generated app handler | `http.Handler` | Wrap with normal Go middleware in app startup. |
| Guards | `runtime/guard.Registry`, `runtime/auth.Provider` | Generated action, API, fragment, and SSR routes with `guard`. |
| Rate limiting | `*ratelimit.Limiter` | Generated action, API, fragment, SSR, and split-backend proxy routes when the addon is enabled. |
| Handler context | `context.Context` | User handlers read request metadata, raw route params, and typed route params through `runtime/app` helpers. |

Generated apps expose `Handler() (http.Handler, error)` and
`ServeMux() (*http.ServeMux, error)`. App-owned startup code can wrap the
handler with ordinary middleware:

```go
handler, err := gowdkapp.Handler()
if err != nil {
	panic(err)
}
wrapped := myMiddleware(handler)
http.ListenAndServe(":8080", wrapped)
```

## Guards

`guard` is optional, but a page is not public by default: a page that declares
no `guard` warns (`missing_page_guard`) and its route is denied (403) at request
time until access is stated. Use `guard public` to serve the page on purpose.
`public` is a compile-time marker, must be the only guard on that page, and does
not require runtime backing code.

Routes with non-public `guard` IDs require backing code in the generated app
package. A guarded generated app will not compile until the required hook
exists. Non-public page guards also require request-time page rendering for the
page GET route; build-time SPA pages emit static HTML and cannot enforce
frontend access.

```go
import gowdkguard "github.com/cssbruno/gowdk/runtime/guard"

func GOWDKGuardRegistry() gowdkguard.Registry {
	return gowdkguard.Registry{
		"auth.required": func(ctx gowdkguard.Context) error {
			return nil
		},
	}
}
```

`addons/ssr.GuardRegistry` and `addons/ssr.LoadContext` remain aliases for
existing SSR-facing guard code.

Native RBAC guards reuse `guard` IDs:

```gwdk
guard role:admin, permission:patients.read
```

Generated app packages with native RBAC guard IDs require:

```go
func GOWDKAuthProvider() auth.Provider
```

Define the application-owned principal source from generated app startup code:

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
			Permissions: []string{"patients.read"},
		}, nil
	})
}
```

RBAC guard behavior:

- `role:<name>` requires the principal to have that role.
- `permission:<name>` requires the principal to have that permission.
- Multiple guard IDs are enforced in declaration order, so multiple RBAC guards
  are an AND check.
- A missing `GOWDKAuthProvider` function fails at Go compile time. A nil
  principal, provider error, or missing role/permission fails closed with HTTP
  403.
- GOWDK does not manage users, passwords, OAuth, sessions, tenants, or storage.
  The auth provider adapts application-owned identity into `auth.Principal`.
- Native RBAC guards are a defense-in-depth redundancy layer for generated
  route/page access. They must never replace backend authorization around
  protected resources, data access, or service methods.

Guard behavior (see [guards.md](../language/guards.md) for the full access
contract):

- Missing `guard` is a `missing_page_guard` warning and the route is denied
  (403) at request time — except on a page that also declares `act`/`api`/
  `fragment` endpoints, where it is a build error.
- `guard public` marks intentional public access and cannot be combined with
  protected guard IDs.
- Non-public page guards on build-time SPA/action page routes fail validation;
  add `load {}` or `go ssr {}` with the SSR addon when the page itself is
  protected.
- Guards run in declaration order.
- Missing custom guard backing code fails at Go compile time.
- Guard errors fail closed with HTTP 403.
- Guards run before action decoding, API handler calls, fragment hooks, SSR
  `load {}`, and user business logic.
- Guards return `nil` or `error`. Ordinary errors fail closed with HTTP 403.
  `runtime/guard.RedirectTo`, `runtime/guard.Redirect`, and
  `runtime/guard.Respond` are the explicit no-store redirect/custom-response
  helpers for guard failures.

Guard redirect and response helpers keep the guard signature small while making
the few intentional non-403 outcomes visible in code:

```go
import (
	"net/http"

	gowdkguard "github.com/cssbruno/gowdk/runtime/guard"
	gowdkresponse "github.com/cssbruno/gowdk/runtime/response"
)

func GOWDKGuardRegistry() gowdkguard.Registry {
	return gowdkguard.Registry{
		"auth.required": func(ctx gowdkguard.Context) error {
			return gowdkguard.RedirectTo("/login")
		},
		"api.auth": func(ctx gowdkguard.Context) error {
			return gowdkguard.Respond(gowdkresponse.JSONBody(http.StatusUnauthorized, `{"error":"login required"}`))
		},
	}
}
```

Guard redirects must be local absolute paths. Protocol-relative URLs,
backslashes, newlines, and non-3xx redirect statuses are rejected before the
generated app can write them.

## Rate Limiting

When `ratelimit.Addon()` is enabled, generated apps expose:

```go
gowdkapp.RegisterRateLimiter(limiter)
```

Generated request-time routes call the registered limiter before guards and
user handler logic. If no limiter is registered, requests continue.

## Ordering

Current generated request-time order:

1. Attach route or endpoint context metadata, including raw and typed route
   params when the route declares them.
2. Install panic boundary for supported generated lanes.
3. Run rate limiter when enabled and registered.
4. Run guards when declared.
5. Run CSRF validation for generated action POSTs when enabled.
6. Decode generated action input when applicable.
7. Call user Go handler, fragment hook, or SSR load/render path.
8. Write the returned `runtime/response.Response` or generated HTML.

## Non-Goals

- No generated route rewriting hook.
- No generated response transform hook.
- No generated fetch/navigation interception hook.
- No custom GOWDK context type; user code receives `context.Context`.
- No framework-specific middleware in generated core. Chi, Echo, Gin, and Fiber
  adapters wrap the same `http.Handler`.

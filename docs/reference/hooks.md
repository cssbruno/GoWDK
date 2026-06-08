# Hooks, Guards, And Middleware

GOWDK's current hook model is small and `net/http`-first.

## Current Contracts

| Extension point | Type | Scope |
| --- | --- | --- |
| Generated app handler | `http.Handler` | Wrap with normal Go middleware in app startup. |
| Guards | `addons/ssr.GuardRegistry`, `runtime/auth.Provider` | Generated request-time routes with `@guard`. |
| Rate limiting | `*ratelimit.Limiter` | Generated action, API, fragment, SSR, and split-backend proxy routes when the addon is enabled. |
| Handler context | `context.Context` | User handlers read request metadata through `runtime/app` helpers. |

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

Every page source must declare `@guard`. Use `@guard public` when the page is
intentionally public. `public` is a compile-time marker, must be the only guard
on that page, and does not require runtime backing code.

Routes with non-public `@guard` IDs require backing code in the generated app
package. A guarded generated app will not compile until the required hook
exists. Non-public page guards also require request-time page rendering for the
page GET route; build-time SPA pages emit static HTML and cannot enforce
frontend access.

```go
func GOWDKGuardRegistry() ssr.GuardRegistry {
	return ssr.GuardRegistry{
		"auth.required": func(ctx ssr.LoadContext) error {
			return nil
		},
	}
}
```

Native RBAC guards reuse `@guard` IDs:

```gwdk
@guard role:admin, permission:patients.read
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

Guard behavior:

- Missing `@guard` fails source validation for real page files.
- `@guard public` marks intentional public access and cannot be combined with
  protected guard IDs.
- Non-public page guards on build-time SPA/action page routes fail validation;
  add `load {}` or `go ssr {}` with the SSR addon when the page itself is
  protected.
- Guards run in declaration order.
- Missing custom guard backing code fails at Go compile time.
- Guard errors fail closed with HTTP 403.
- Guards run before action decoding, API handler calls, fragment hooks, SSR
  `load {}`, and user business logic.
- Guards return `nil` or `error` today. Redirect/custom response guard results
  are planned.

## Rate Limiting

When `ratelimit.Addon()` is enabled, generated apps expose:

```go
gowdkapp.RegisterRateLimiter(limiter)
```

Generated request-time routes call the registered limiter before guards and
user handler logic. If no limiter is registered, requests continue.

## Ordering

Current generated request-time order:

1. Attach route or endpoint context metadata.
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
- No framework-specific middleware in generated core. Echo, Gin, and Fiber
  adapters wrap the same `http.Handler`.

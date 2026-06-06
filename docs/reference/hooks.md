# Hooks, Guards, And Middleware

GOWDK's current hook model is small and `net/http`-first.

## Current Contracts

| Extension point | Type | Scope |
| --- | --- | --- |
| Generated app handler | `http.Handler` | Wrap with normal Go middleware in app startup. |
| Guards | `addons/ssr.GuardRegistry` | Generated SSR, action, API, and fragment routes with `@guard`. |
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

Routes with `@guard` use a generated registration hook:

```go
func init() {
	gowdkapp.RegisterGuards(ssr.GuardRegistry{
		"auth.required": func(ctx ssr.LoadContext) error {
			return nil
		},
	})
}
```

Guard behavior:

- Guards run in declaration order.
- Missing guard registrations fail closed with HTTP 403.
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

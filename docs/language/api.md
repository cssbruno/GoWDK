# API

APIs are endpoint declarations. A page declares the exported same-package Go
symbol, HTTP method, and endpoint path in `.gwdk`; normal Go owns the behavior.

```gowdk
package api

api Health GET "/api/health" @error "/errors/api-health.html"
```

Supported methods are `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.
The route must be a quoted absolute route path.
Old `api health { ... }` blocks are rejected with a migration diagnostic.

Generated apps bind same-package Go handlers for the first API slice. `api
Health GET "/api/health"` maps exactly to exported Go function `Health` in a
same-package `.go` file or default `go {}` block when the function has
signature:

```go
func Health(context.Context, *http.Request) (response.Response, error)
```

Bound API handlers return `runtime/response.Response`. In development/default
mode, missing or unsupported handlers are not build errors; generated apps
return HTTP 501 for those routes with a clear message.

In production mode, explicitly declared APIs must bind to supported
same-package Go handlers. Missing or unsupported handlers fail the build unless
`Build.AllowMissingBackend` or `--allow-missing-backend` is set to intentionally
generate HTTP 501 stubs during a migration.

Feature packages that declare API handlers may import stable public GOWDK
packages such as `runtime/response` and `runtime/app`; they must not import
generated app packages, generated `gowdkapp` output, generated `cmd/server`
code, or build output directories. Generated app source imports feature
packages, never the other way around.

Generated bound API adapters attach endpoint metadata to the handler context.
Handlers can call `app.Endpoint(ctx)` from `runtime/app` to read the generated
endpoint kind, page ID, symbol name, method, path, and optional generated error
page.

The optional endpoint-local `@error` suffix selects a generated HTML error page
for API panics before response headers are written. Returned handler errors
still follow normal `runtime/response.Response` behavior.

APIs declared on guarded pages share the generated app guard hooks with SSR
pages and actions. Custom guards require `GOWDKGuardRegistry`; native RBAC guard
IDs such as `role:admin` and `permission:reports.read` require
`GOWDKAuthProvider`. Missing backing hooks fail the generated app Go build.
Generated API handlers run guards before user handler calls. Treat these as
defense-in-depth redundancy for generated route/page access, never as backend
resource authorization. If the page itself is protected, use request-time page
rendering; build-time SPA HTML cannot enforce frontend page access.

## Production Notes

- API handlers own authentication, backend authorization, request validation,
  storage, service calls, and response shape in normal Go.
- Bound API handlers return `runtime/response.Response`; generated adapters
  only dispatch by method/path, call the handler, and write the returned
  response.
- Generated API responses and generated API error responses use
  `Cache-Control: no-store` in the current first slice.
- Handler errors are written with `runtime/response.HandlerStatus`, defaulting
  to HTTP 500 when the error does not carry an explicit status. Error messages
  should not include secrets, tokens, credentials, or submitted sensitive data.
- Missing or unsupported generated API bindings return HTTP 501 only in
  development/default mode or when an explicit missing-backend migration flag is
  set.
- Generated action CSRF wiring does not protect API endpoints. State-changing
  APIs need normal Go auth/session checks and an explicit CSRF, same-site, or
  token strategy appropriate to the API client.

Future API behavior must define:

- Request body and query decoding.
- Authentication and authorization hooks.
- Error response shape.
- Interaction with SPA/action pages without full-page SSR.

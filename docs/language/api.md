# API

APIs are endpoint declarations. A page declares the exported same-package Go
symbol, HTTP method, and endpoint path in `.gwdk`; normal Go owns the behavior.

```gowdk
package api

api Health GET "/api/health" error "/errors/api-health.html"
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

## Handler Helpers

API handlers can use `github.com/cssbruno/gowdk/addons/api` for the current
public helper contract:

```go
package api

import (
	"context"
	"net/http"

	gowdkapi "github.com/cssbruno/gowdk/addons/api"
	"github.com/cssbruno/gowdk/runtime/response"
)

type CreatePatientInput struct {
	Name string `json:"name"`
}

func CreatePatient(ctx context.Context, request *http.Request) (response.Response, error) {
	input, err := gowdkapi.DecodeJSON[CreatePatientInput](request)
	if err != nil {
		return gowdkapi.Error(http.StatusBadRequest, "invalid_json", "Invalid JSON body")
	}

	active, ok, err := gowdkapi.QueryBool(request, "active")
	if err != nil {
		return gowdkapi.Error(http.StatusBadRequest, "invalid_query", "Invalid query")
	}
	_ = input
	_ = active
	_ = ok

	return gowdkapi.JSON(http.StatusCreated, map[string]any{"ok": true})
}
```

`DecodeJSON[T]` decodes the capped request body into `T`, accepts
`application/json` and `+json` content types, rejects unknown object fields,
rejects trailing JSON values, and rejects an empty or non-JSON `Content-Type`.

Query helpers read from `request.URL.Query()`:

- `QueryString(request, name) (string, bool)`
- `QueryStrings(request, name) []string`
- `QueryBool(request, name) (bool, bool, error)`
- `QueryInt(request, name) (int, bool, error)`
- `QueryInt64(request, name) (int64, bool, error)`

Response helpers return `runtime/response.Response`:

- `JSON(status, value)` marshals a JSON response.
- `Error(status, code, message)` returns `{ "ok": false, "error": ... }`.
- `NoContent()` returns a 204 response.

Generated bound API adapters attach endpoint metadata to the handler context.
Handlers can call `app.Endpoint(ctx)` from `runtime/app` to read the generated
endpoint kind, page ID, symbol name, method, path, and optional generated error
page.

The optional endpoint-local `error` suffix selects a generated HTML error page
for API panics before response headers are written. Returned handler errors
still follow normal `runtime/response.Response` behavior.

## Examples

`examples/endpoints/src/endpoints/api.page.gwdk` declares session, search, JSON CRUD, and
webhook endpoints. `examples/endpoints/src/endpoints/handlers.go` keeps validation, JSON
decoding, response shape, and webhook policy in normal Go handlers.

APIs declared on guarded pages share generated app guard backing with SSR pages
and actions. `auth.Addon` supplies `auth.required` and native RBAC session guard
backing when configured. Custom guards require `GOWDKGuardRegistry`; native RBAC
guard IDs such as `role:admin` and `permission:reports.read` require
`GOWDKAuthProvider` only without `auth.Addon`. Missing backing hooks fail the
generated app Go build.
Generated API handlers run guards before user handler calls. Treat these as
defense-in-depth redundancy for generated route/page access, never as backend
resource authorization. If the page itself is protected, use request-time page
rendering; build-time SPA HTML cannot enforce frontend page access.

## Production Notes

- API handlers own authentication, backend authorization, domain validation,
  storage, service calls, and response shape in normal Go.
- `addons/api` helpers cover strict JSON body decoding, typed query access, and
  JSON response envelopes without requiring framework-specific adapters.
- Bound API handlers return `runtime/response.Response`; generated adapters
  only dispatch by method/path, call the handler, and write the returned
  response.
- Generated API responses and generated API error responses use
  `Cache-Control: no-store` in the current first slice.
- Generated API adapters dispatch only the declared HTTP method/path pair;
  unsupported methods do not call user handlers.
- Handler errors are written with `runtime/response.HandlerStatus`, defaulting
  to HTTP 500 when the error does not carry an explicit status. Ordinary 5xx
  responses use generic status text; expose only intentional client-facing
  messages through `runtime/response.HandlerError.Message`.
- Missing or unsupported generated API bindings return HTTP 501 only in
  development/default mode or when an explicit missing-backend migration flag is
  set.
- Generated state-changing API endpoints validate the generated CSRF token by
  default. Browser clients must send the token in the configured CSRF header
  such as `X-GOWDK-CSRF`; non-browser API designs can opt out with
  `Build.CSRF.Disabled` only when they enforce another cross-site request
  strategy.

Future API behavior must define:

- Authentication and authorization hooks.
- Generated typed handler signatures beyond
  `func(context.Context, *http.Request) (response.Response, error)`.
- Per-route body/query/result contracts and route-param accessors.
- CORS policy and richer content negotiation.
- Interaction with SPA/action pages without full-page SSR.

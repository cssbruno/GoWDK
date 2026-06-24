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

Generated apps bind same-package Go handlers. `api Health GET "/api/health"`
maps exactly to exported Go function `Health` in a same-package `.go` file or
default `go {}` block.

The raw escape-hatch signature is:

```go
func Health(context.Context, *http.Request) (response.Response, error)
```

Typed generated signatures are:

```go
func Health(context.Context) (HealthResult, error)
func Search(context.Context, SearchInput) (SearchResult, error)
func Search(context.Context, *SearchInput) (SearchResult, error)
```

Typed input and result values must be exported same-package structs. For
`GET` and `HEAD`, generated adapters decode typed input from the query string.
For other methods, generated adapters decode strict JSON object bodies without
request-time reflection. Typed results are returned as no-store JSON using the
result struct fields and OpenAPI schema generated from the same metadata.

Typed result structs can choose a status code by implementing:

```go
func (result CreatePatientResult) APIStatus() int { return http.StatusCreated }
```

Non-positive values fall back to `200 OK`. Use the raw
`response.Response` handler signature when an endpoint needs custom content
types, redirects, empty responses, or fully app-owned response writing.

In development/default mode, missing or unsupported handlers are not build
errors; generated apps return HTTP 501 for those routes with a clear message.

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
- `ResultStatus(result, fallback)` returns the optional typed result
  `APIStatus()` value when present.

Generated bound API adapters attach endpoint metadata to the handler context.
Handlers can call `app.Endpoint(ctx)` from `runtime/app` to read the generated
endpoint kind, page ID, symbol name, method, path, and optional generated error
page.

The optional endpoint-local `error` suffix selects a generated HTML error page
for API panics before response headers are written. Returned handler errors
still follow normal `runtime/response.Response` behavior.

## CORS

Generated API routes are same-origin by default. Enable cross-origin browser
access with `Build.CORS` in `gowdk.config.go`:

```go
Build: gowdk.BuildConfig{
	CORS: gowdk.CORSConfig{
		Enabled: true,
		AllowedOrigins: []string{"https://app.example"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Content-Type", "X-CSRF"},
		ExposedHeaders: []string{"X-Total-Count"},
		AllowCredentials: true,
		MaxAgeSeconds: 600,
	},
}
```

The policy applies to generated API, command, and query routes. Preflight
requests are answered before guards, rate limits, CSRF, and user handlers. CORS
does not replace authentication or authorization. `AllowedOrigins: []string{"*"}`
is allowed only when `AllowCredentials` is false.

An individual `.gwdk` API endpoint can declare a narrower or overriding policy
with a trailing `cors` clause:

```gowdk
api Health GET "/api/health" cors origins "https://app.example" headers "Content-Type,X-CSRF" credentials true maxAge 600
```

Supported options are `origins`, `methods`, `headers`, `expose`,
`credentials`, and `maxAge`. List options use quoted comma-separated values.
When `Build.CORS` is enabled, omitted endpoint options inherit from it; options
declared on the endpoint override the inherited values. When `Build.CORS` is
disabled, an endpoint `cors` clause must provide enough policy to validate, at
minimum `origins`. Endpoint policies use the same safety rules as config-level
CORS, including rejecting `origins "*"` with `credentials true`.

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
- `addons/api` helpers cover strict JSON body decoding, typed query access,
  typed result status selection, and JSON response envelopes without requiring
  framework-specific adapters.
- Bound raw API handlers return `runtime/response.Response`; typed API handlers
  return exported structs that generated adapters encode as JSON.
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
- Generated API/command/query endpoints are same-origin unless `Build.CORS` or
  an endpoint-local `cors` clause enables a CORS policy. Preflight requests for
  matching endpoints fail closed with HTTP 403 when no policy allows them.
- Generated state-changing API endpoints validate the generated CSRF token by
  default. Browser clients must send the token in the configured CSRF header
  such as `X-GOWDK-CSRF`; non-browser API designs can opt out with
  `Build.CSRF.Disabled` only when they enforce another cross-site request
  strategy.

Future API behavior must define:

- Authentication and authorization hooks.
- Route-param, header, and richer endpoint-scoped typed input contracts.
- Custom typed content negotiation.
- Interaction with SPA pages that declare backend endpoints without full-page SSR.

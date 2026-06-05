# API

The current parser records `api {}` or `api <name> {}` block declarations.
The first implemented API route metadata subset accepts one method/route line:

```gowdk
api health {
  GET "/api/health"
}
```

Supported methods are `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.
The route must be a quoted absolute route path.

Generated apps bind same-package Go handlers for the first API slice. `api
session` maps to exported Go function `Session` in the same package as the
`.gwdk` file when the function has signature:

```go
func Session(context.Context, *http.Request) (response.Response, error)
```

Bound API handlers return `runtime/response.Response`. Missing or unsupported
handlers are not build errors; generated apps return HTTP 501 for those routes
with a clear message.

Future API behavior must define:

- Request body and query decoding.
- Authentication and authorization hooks.
- Error response shape.
- Interaction with SPA/action pages without full-page SSR.

# Errors And Boundaries

GOWDK separates expected handler results from unexpected generated-lane
failures.

## Current Contracts

Expected errors are user-owned handler results:

- Return `runtime/response.Response` values for normal not-found, forbidden,
  invalid request, conflict, validation, redirect, HTML, fragment, and JSON
  outcomes.
- Return typed expected errors with `response.NotFound`,
  `response.Forbidden`, `response.ValidationFailed`, or
  `response.ServerError` when a generated boundary should map the error to
  404, 403, 422, or 500.
- Return `response.NewHandlerError(status, message, cause)` when a generated
  action or API handler should fail with a specific HTTP status.
- Return ordinary Go errors only for failures where HTTP 500 is acceptable.
  Generated action and API adapters use `response.HandlerStatus`, defaulting to
  HTTP 500.
- Return `ssr.RedirectTo("/path")` or `ssr.Redirect("/path", status)` from SSR
  load functions for safe local redirects.
- Return typed expected errors from SSR load functions when the page should use
  a non-500 generated boundary. Expected 404 responses use `404.html` when it
  exists. Expected 500 responses use the route-local `error` page or `500.html`
  when available. Expected 403 and 422 responses use safe fallback text until
  layout/status-specific boundaries are defined.

Unexpected errors are generated-lane failures:

- Generated SSR, action, and API lanes recover panics before response headers
  are written.
- Panic values are not rendered.
- Generated request-shape failures use generic messages such as `invalid form`,
  `invalid csrf token`, or `validation failed`.
- Generated form decoding and validation do not echo submitted values.

## Generated Error Pages

Generated embedded apps load these optional HTML files from build output:

| File | Used for |
| --- | --- |
| `404.html` | Not-found responses from generated app serving. |
| `500.html` | Internal generated error responses when no route-local page applies. |

Example output:

```text
dist/
  index.html
  404.html
  500.html
  errors/
    dashboard.html
```

Route-local SSR error pages use `error`:

```gwdk
route "/dashboard"
guard auth.required
error "/errors/dashboard.html"

server {
}
```

Endpoint-local action and API error pages also use `error`:

```gwdk
act Submit POST "/signup" error "/errors/signup.html"
api Health GET "/api/health" error "/errors/api-health.html"
```

`error` paths are output-relative. They may start with `/`, must end in
`.html`, and must not contain `..`, query strings, fragments, or backslashes.
Missing error documents fall back to `http.Error`.

## Cache Policy

Generated error responses use `Cache-Control: no-store`.

This includes generated `404.html`, `500.html`, route-local `error` pages,
panic-boundary responses, invalid generated forms, invalid CSRF responses,
validation failures, guard failures, missing backend stubs, and SSR load
failures. Successful SSR pages use their declared page cache policy instead.

## Boundaries

Supported boundary syntax:

- Page/route boundary: `error` on SSR pages.
- Endpoint boundary: `error` on `act` and `api`.
- Global fallback pages: `404.html` and `500.html`.

Not supported today:

- Layout-level error boundary syntax.
- Component-level error boundary syntax.
- Fragment-specific error boundary syntax.
- Generated response-transform hooks.
- Rendering panic values, submitted form values, secrets, or stack traces.

## Logging And Rendering

Generated panic boundaries render safe fixed messages or generated HTML error
documents. They intentionally do not render panic values.

Generated action, API, fragment, contract, SSR load, and addon 5xx responses
hide ordinary returned error details. Apps can expose an intentional
client-facing message by returning `response.HandlerError` with an explicit
`Message`; 4xx handler errors keep their application message contract.

Runtime panic logs and compiler diagnostics apply conservative redaction before
writing to logs, terminal output, JSON diagnostics, or LSP diagnostics. The
redaction policy masks common credential surfaces:

- DSN passwords such as `postgres://user:password@host`.
- Bearer and Basic authorization header values.
- `password`, `passwd`, `pwd`, `secret`, `token`, `_gowdk_csrf`,
  `csrf_token`, `cookie`, `set-cookie`, `auth_token`, `session`,
  `session_id`, `jwt`, `api_key`, `access_key`, `access_token`,
  `refresh_token`, `id_token`, `client_secret`, and `private_key` values when
  they appear as `name=value` or `name: value`.

Limitations: app-owned logging is outside GOWDK's control, and explicit
client-facing `HandlerError.Message` values are trusted as app-owned text. Do
not put secrets, credentials, submitted sensitive values, SQL details, or
internal service details in messages intended for clients.

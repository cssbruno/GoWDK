# Errors And Boundaries

GOWDK separates expected handler results from unexpected generated-lane
failures.

## Current Contracts

Expected errors are user-owned handler results:

- Return `runtime/response.Response` values for normal not-found, forbidden,
  invalid request, conflict, validation, redirect, HTML, fragment, and JSON
  outcomes.
- Return `response.NewHandlerError(status, message, cause)` when a generated
  action or API handler should fail with a specific HTTP status.
- Return ordinary Go errors only for failures where HTTP 500 is acceptable.
  Generated action and API adapters use `response.HandlerStatus`, defaulting to
  HTTP 500.
- Return `ssr.RedirectTo("/path")` or `ssr.Redirect("/path", status)` from SSR
  load functions for safe local redirects.

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

Route-local SSR error pages use `@error`:

```gwdk
@page dashboard
@route "/dashboard"
@error "/errors/dashboard.html"
```

Endpoint-local action and API error pages also use `@error`:

```gwdk
act Submit POST "/signup" @error "/errors/signup.html"
api Health GET "/api/health" @error "/errors/api-health.html"
```

`@error` paths are output-relative. They may start with `/`, must end in
`.html`, and must not contain `..`, query strings, fragments, or backslashes.
Missing error documents fall back to `http.Error`.

## Cache Policy

Generated error responses use `Cache-Control: no-store`.

This includes generated `404.html`, `500.html`, route-local `@error` pages,
panic-boundary responses, invalid generated forms, invalid CSRF responses,
validation failures, guard failures, missing backend stubs, and SSR load
failures. Successful SSR pages use their declared page cache policy instead.

## Boundaries

Supported boundary syntax:

- Page/route boundary: `@error` on SSR pages.
- Endpoint boundary: `@error` on `act` and `api`.
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

Returned handler errors are user-owned. If a handler returns an error message
that includes secrets, tokens, credentials, submitted values, SQL details, or
internal service details, GOWDK will not rewrite that message. Prefer returning
a safe `response.Response` or `response.NewHandlerError` message and logging
the detailed cause in app-owned middleware or handler code.

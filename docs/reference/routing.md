# Routing Reference

Routes are declared inside `.gwdk` files. File location does not define route
identity.

## Page Routes

Every current page file must declare a page ID and a route:

```gwdk
@page home
@route "/"

view {
  <main>
    <h1>Home</h1>
  </main>
}
```

Current route rules:

- Routes must start with `/`.
- `/` is the only route that may end with `/`.
- Routes must not include query strings, fragments, backslashes, whitespace,
  control characters, empty segments, `.`, or `..`.
- Dynamic params must be whole path segments, such as `/blog/{slug}`.
- Param names use `[A-Za-z_][A-Za-z0-9_]*`.
- A route cannot repeat the same param name.
- Duplicate page route patterns are rejected. `/blog/{slug}` and `/blog/{id}`
  are the same route pattern.

## SPA Routes

SPA render is the default:

```gwdk
@page docs
@route "/docs"

view {
  <main>
    <h1>Docs</h1>
  </main>
}
```

`gowdk build --out <dir>` writes the route as spa HTML. For `/docs`, the
current output is `<dir>/docs/index.html`. For `/`, the output is
`<dir>/index.html`.

When a SPA page, layout, or referenced component contains a literal internal
link such as `<a href="/docs">`, the build emits the small
`assets/gowdk/gowdk.js` enhancement runtime. That runtime intercepts normal
same-origin link clicks, fetches the real generated HTML page, replaces the
current document head/body, updates browser history, and preserves focus/scroll
where possible. It does not define routes or decide whether a route exists; the
generated files or generated server remain the source of truth, and direct page
open/refresh must keep working.

## Dynamic SPA Routes

Dynamic SPA routes require `paths {}`. Action endpoints on a dynamic SPA page
inherit that page's generated concrete paths:

```gwdk
@page blog.post
@route "/blog/{slug}"
@render spa

paths {
  => { slug: "hello-gowdk" }
  => { slug: "compile-first" }
}

view {
  <main>
    <h1>{slug}</h1>
    <p>{param("slug")}</p>
  </main>
}
```

The implemented `paths {}` subset accepts literal string records. Route params
from those records are available to the current spa interpolation scope and
to literal `build {}` string interpolation.

Build:

```sh
gowdk build --out /tmp/gowdk-dynamic examples/pages/blog-post.page.gwdk
```

Generated output:

```text
/tmp/gowdk-dynamic/blog/hello-gowdk/index.html
/tmp/gowdk-dynamic/blog/compile-first/index.html
```

Imported Go build functions do not receive route params yet.

## Action Endpoints

An `act` declaration on a page adds a `POST` endpoint in the current generated
app slice:

```gwdk
package signup

@page signup
@route "/signup"
@render action

act Submit POST "/signup"

view {
  <form g:post={Submit}>
    <input name="email" required />
    <button type="submit">Sign up</button>
  </form>
}
```

App-shell HTML lowers `g:post={Submit}` to a normal POST form. Generated apps
built with `--app --bin` serve concrete action endpoints. If the same directory
as the `.gwdk` file contains an exported Go function with the exact declared
symbol, the generated handler calls it when it uses one of these signatures:

```go
func Submit(context.Context) (response.Response, error)
func Submit(context.Context, SignupInput) (response.Response, error)
func Submit(context.Context, *SignupInput) (response.Response, error)
func Submit(context.Context, form.Values) (response.Response, error)
```

Missing or unsupported functions generate HTTP 501 handlers.

Actions can also be declared on the exported Go handler itself:

```go
//gowdk:act POST /signup
func Submit(context.Context, SignupInput) (response.Response, error)
```

Go comment action endpoints are standalone backend endpoints. They use the same
binding and generated adapter pipeline as `.gwdk` action declarations, but they
do not infer page-local form schemas, fragments, or guards from `.gwdk` page
markup.

When `Build.CSRF.Enabled` is set, generated action handlers validate CSRF
tokens before generated decoding or user handlers run. Missing or invalid
tokens return HTTP 403 with `invalid csrf token` and `Cache-Control: no-store`.

## API Routes

API endpoint metadata is parsed, appears in route plans, and can bind to
same-package Go handlers:

```gwdk
package api

@page status
@route "/status"

api Health GET "/api/health"

view {
  <main>
    <h1>Status</h1>
  </main>
}
```

Supported methods today: `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.

`api Health GET "/api/health"` maps to exported Go function `Health` in the
same package as the `.gwdk` file when the function has signature
`func(context.Context, *http.Request) (response.Response, error)`. Missing or
unsupported functions generate HTTP 501 handlers.

APIs can also be declared on the exported Go handler itself:

```go
//gowdk:api GET /api/health
func Health(context.Context, *http.Request) (response.Response, error)
```

The compiler discovers Go endpoint comments only in selected source packages,
does not infer endpoints from function names, and does not scan framework route
registrations. If a Go comment endpoint and a `.gwdk` endpoint declare the same
method/path pair, validation fails with a route conflict diagnostic.

## SSR Routes

SSR is optional and must be enabled for validation:

```sh
gowdk check --ssr examples/ssr/simple-ssr.page.gwdk
```

First-slice concrete and dynamic `@render ssr` pages without `load {}` can be
generated into an embedded app and binary:

```sh
gowdk build --ssr --out /tmp/gowdk-ssr-build \
  --app /tmp/gowdk-ssr-app \
  --bin /tmp/gowdk-ssr-site \
  examples/ssr/dynamic-ssr.page.gwdk
```

Dynamic SSR route params render through generated placeholders and request-time
HTML escaping. Generated SSR handlers attach route metadata through
`runtime/app.Route(ctx)` and dynamic params through `runtime/app.Params(ctx)`.
User Go can decode those params with `runtime/route` helpers:

```go
params := app.Params(ctx)
id, ok, err := route.Int(params, "id")
if err != nil {
  return response.HTMLBody(400, "invalid route param"), err
}
if !ok {
  return response.HTMLBody(404, "missing route param"), nil
}
_ = id
```

The helpers support `String`, `Int`, `Int64`, `Uint`, `Uint64`, `Bool`, and
`Float64`. `Required` returns a missing-param error when a required param is not
present. Decode errors name the param and expected type without echoing the raw
request value. Route-param type declarations and generated typed bindings are
still planned.

`load {}` execution and broad request-time user logic are still planned.

## Route Plans

Use `gowdk routes` to inspect validated route and endpoint metadata:

```sh
gowdk routes --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk
```

The current JSON schema is version `1`. `routes` contains only page/file route
kinds such as `static`, `spa`, `ssr`, and `hybrid`; `endpoints` contains one
framework-neutral endpoint record per action/API declaration. Endpoint records
include `endpointSource` (`gwdk` today), source file and source span, `.gwdk`
package, Go package path/name when known, exact declared symbol, method, path,
planned adapter handler information, and backend binding status/message.
Backend binding details repeat the Go package name, import path when known,
handler symbol, and supported signature/input metadata when the handler is
bound. The `info` list reports disabled route-mode lanes, for example SSR
disabled on a SPA route.

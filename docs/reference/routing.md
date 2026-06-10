# Routing Reference

Routes are declared inside `.gwdk` files. File location does not define route
identity.

## Page Routes

Every current page file must declare a route and access guard. Page ID derives
from the filename unless `@page` is present:

```gwdk
@route "/"
@guard public

view {
  <main>
    <h1>Home</h1>
  </main>
}
```

Use explicit `@page` only when page identity should not follow the filename.

Current route rules:

- Routes must start with `/`.
- `/` is the only route that may end with `/`.
- Routes must not include query strings, fragments, backslashes, whitespace,
  control characters, empty segments, `.`, or `..`.
- Dynamic params must be whole path segments, such as `/blog/{slug}`.
- Param names use `[A-Za-z_][A-Za-z0-9_]*`.
- A route cannot repeat the same param name.
- Duplicate page route patterns are rejected. `/blog/{slug}` and `/blog/{id}`
  are the same route pattern, and `/docs/{path...}` and `/docs/{rest...}` are
  the same route pattern.

### Rest Params

A page route may declare one rest (catch-all) param as its final segment:

```gwdk
@route "/docs/{path...}"
@guard public

go ssr {}

view {
  <main>
    <h1>{param("path")}</h1>
  </main>
}
```

Rest param contract:

- `{name...}` is allowed only as the final route segment. A rest param before
  the end of the route is rejected with a `malformed_route` diagnostic.
- A rest param matches one or more remaining request path segments. `/docs`
  does not match `/docs/{path...}`; `/docs/intro` and `/docs/guides/routing`
  do. Each matched segment still rejects empty, `.`, and `..` values.
- The captured value is the remaining segments joined with `/`, for example
  `guides/routing`. Read it with `param("name")` in the view, or in request-time
  Go through `app.Params(ctx)` and `route.Required(params, "name")`.
- Rest params are always strings. Typed rest params such as `{path...:int}` are
  rejected.
- Rest params require request-time (SSR) rendering, because build-time SPA
  paths cannot enumerate and escape multi-segment values. Declare `load {}` or
  `go ssr {}` on the page.
- Rest params are only supported on page routes; action, API, fragment, and Go
  comment endpoint paths reject them.
- Rest routes participate in ambiguity validation: `/docs/{path...}` overlaps
  `/docs/{slug}`, `/docs/{section}/{slug}`, and concrete routes such as
  `/docs/guides/intro`, so those combinations are rejected as
  `ambiguous_dynamic_route`.

Unsupported route forms today:

- Optional params such as `/docs/{slug?}`. The diagnostic is explicit: optional
  route parameters are not supported; declare explicit routes for each shape
  (rest parameters `{name...}` are supported as the final segment).
- Route groups that affect URL shape independently from explicit `@route`.
- Page/API same-path content negotiation. A page route and endpoint may share a
  path only when their HTTP methods do not conflict, such as `GET /signup` page
  plus `POST /signup` action.

### Trailing Slash Policy

Routes are canonical without a trailing slash. The policy is explicit:

- Declarations: omit trailing slashes except for `/`. `@route "/blog/hello/"`
  is rejected with `malformed_route`.
- Requests: generated servers respond to `GET` and `HEAD` requests whose path
  carries a trailing slash (and is not `/`) with a `308 Permanent Redirect` to
  the cleaned canonical path, preserving the query string. `GET /blog/hello/?page=2`
  redirects to `/blog/hello?page=2` instead of serving duplicate content.
- `POST` behavior is unchanged: generated action handlers tolerate a trailing
  slash on concrete POST routes as a compatibility fallback and redirect to the
  declared target when configured.

Pages may declare response cache intent with `@cache`. The value is carried as
route metadata and should be a literal HTTP `Cache-Control` value:

```gwdk
@route "/docs"
@guard public
@cache "public, max-age=60"
```

Pages may also declare stale-while-revalidate behavior with `@revalidate`.
Values may be whole seconds or Go-style whole-second durations such as `60s`,
`5m`, or `1h`. `@revalidate` requires `@cache` and appends a concrete
`stale-while-revalidate=<seconds>` directive to the generated Cache-Control
header:

```gwdk
@route "/docs"
@guard public
@cache "public, max-age=60"
@revalidate 5m
```

Generated binaries apply explicit page `@cache` values to successful static SPA
HTML and SSR HTML responses. When `@revalidate` is present, generated binaries
send the appended stale-while-revalidate directive for the same successful
responses. Request-time safety policies still win for actions, APIs, partial
responses, SSR load redirects, CSRF HTML mutation, and generated request-time
errors; those use `no-store`.

## SPA Routes

SPA render is the default:

```gwdk
@route "/docs"
@guard public

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
@route "/blog/{slug}"
@guard public

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

@route "/signup"
@guard public

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

@route "/status"
@guard public

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

First-slice concrete and dynamic request-time SSR pages with declared `load {}`
fields can be
generated into an embedded app and binary:

```sh
gowdk build --ssr --out /tmp/gowdk-ssr-build \
  --app /tmp/gowdk-ssr-app \
  --bin /tmp/gowdk-ssr-site \
  examples/ssr/dynamic-ssr.page.gwdk
```

Dynamic SSR route params render through generated placeholders and request-time
HTML escaping. Params can be declared as `{name}`, `{name:type}`, or — as the
final segment only — `{name...}` (always a string). Supported
types are `string`, `int`, `int64`, `uint`, `uint64`, `bool`, and `float64`.
Generated SSR handlers attach route metadata through `runtime/app.Route(ctx)`,
raw dynamic params through `runtime/app.Params(ctx)`, and decoded typed params
through `runtime/app.TypedParams(ctx)`.

There are no generated per-route param struct types yet. Request-time user code
should use `app.Params(ctx)`, `app.TypedParams(ctx)`, or the `runtime/route`
typed helpers. Per-route structs may be added later only if the generated API
stays stable and simpler than the current helpers.

User Go can still decode raw params with `runtime/route` helpers:

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
request value. Generated typed SSR bindings return `400` for invalid typed route
params and `404` for missing route params before guards or page rendering run.

Endpoint user code can read generated endpoint metadata with
`runtime/app.Endpoint(ctx)`. This is the stable accessor for action, API, and
fragment handler metadata today. Typed load-result and action-result data
accessors are deferred until those result contracts are stable.

`load { => { field, user.name } }` execution calls same-package Go
`Load<PageID>` functions at request time through `ssr.LoadContext`. Returned
declared identifiers and dotted paths are resolved from nested maps with string
keys, structs, pointers, interfaces, exported Go field names, and `json` tag
names, then HTML-escaped into generated placeholders.

## Route Plans

Use `gowdk routes` to inspect validated route and endpoint metadata:

```sh
gowdk routes --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk
```

The current JSON schema is version `1`. `routes` contains only page/file route
kinds such as `static`, `spa`, `ssr`, and `hybrid`; `endpoints` contains one
framework-neutral endpoint record per action/API/fragment declaration and
routable `g:command`/`g:query` contract reference. Endpoint records include
`endpointSource` (`gwdk` or `contract`), source file and source span, `.gwdk`
package, Go package path/name when known, exact declared symbol or contract
reference, method, path, planned adapter handler information, and binding
status/message. Backend binding details repeat the Go package name, import path
when known, handler symbol, and supported signature/input metadata when the
handler is bound. Contract binding details include the contract kind, reference
name, binding status, local input type, result type, roles, handler, register
function, and message when known. The `info` list reports disabled route-mode
lanes, for example SSR disabled on a SPA route.

Use `gowdk inspect ir` when route debugging needs the full typed compiler IR
instead of the route-report schema. The IR output is for M2 compiler debugging
and snapshots; keep `gowdk routes` for route and endpoint report integrations.

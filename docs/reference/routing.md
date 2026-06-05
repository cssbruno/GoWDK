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

## Dynamic SPA Routes

Dynamic spa or action routes require `paths {}`:

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

## Action Routes

An `act` block on a page adds a same-page `POST` route in the current generated
app slice:

```gwdk
@page signup
@route "/signup"
@render action

act submit {
  input := form SignupInput
  valid(input)?
  -> "/signup?ok=1"
}

view {
  <form g:post={submit}>
    <input name="email" required />
    <button type="submit">Sign up</button>
  </form>
}
```

App-shell HTML lowers `g:post={submit}` to a normal POST form. Generated apps
built with `--app --bin` serve concrete action routes. If the same directory as
the `.gwdk` file contains an exported Go function named from the block, for
example `act submit` -> `Submit`, and it has signature
`func(context.Context, form.Values) (response.Response, error)`, the generated
handler calls it. Missing or unsupported functions generate HTTP 501 handlers.

Generated action handlers do not wire CSRF checks yet.

## API Routes

API route metadata is parsed, appears in route plans, and can bind to
same-package Go handlers:

```gwdk
@page status
@route "/status"

api health {
  GET "/api/health"
}

view {
  <main>
    <h1>Status</h1>
  </main>
}
```

Supported methods today: `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.

`api health` maps to exported Go function `Health` in the same package as the
`.gwdk` file when the function has signature
`func(context.Context, *http.Request) (response.Response, error)`. Missing or
unsupported functions generate HTTP 501 handlers.

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
HTML escaping. `load {}` execution, guard enforcement, and broad request-time
user logic are still planned.

## Route Plans

Use `gowdk routes` to inspect the validated route-binding plan:

```sh
gowdk routes --ssr examples/pages/*.gwdk examples/actions/*.gwdk examples/partials/*.gwdk examples/api/*.gwdk examples/ssr/*.gwdk
```

The current JSON schema is version `1` and includes route kind, method, route
pattern, page ID, planned handler information, and backend binding status for
action/API routes.

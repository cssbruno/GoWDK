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

## Static Routes

Static render is the default:

```gwdk
@page docs
@route "/docs"

view {
  <main>
    <h1>Docs</h1>
  </main>
}
```

`gowdk build --out <dir>` writes the route as static HTML. For `/docs`, the
current output is `<dir>/docs/index.html`. For `/`, the output is
`<dir>/index.html`.

## Dynamic Static Routes

Dynamic static or action routes require `paths {}`:

```gwdk
@page blog.post
@route "/blog/{slug}"
@render static

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
from those records are available to the current static interpolation scope and
to literal `build {}` string interpolation.

Build:

```sh
gowdk build --out /tmp/gowdk-dynamic examples/basic/blog-post.page.gwdk
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

Static HTML lowers `g:post={submit}` to a normal POST form. Generated apps built
with `--app --bin` can serve first-slice POST redirect handlers for concrete
page routes. They can also return first-slice partial fragments when the action
declares `fragment "#id" { ... }` and the request includes GOWDK partial
headers.

Generated action handlers do not execute user Go action logic or CSRF checks
yet.

## API Routes

API route metadata is parsed and appears in route plans:

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

Supported method metadata today: `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.

Generated API handlers are planned.

## SSR Routes

SSR is optional and must be enabled for validation:

```sh
gowdk check --ssr examples/basic/simple-ssr.page.gwdk
```

Simple concrete `@render ssr` pages without `load {}` can be generated into an
embedded app and binary:

```sh
gowdk build --ssr --out /tmp/gowdk-ssr-build \
  --app /tmp/gowdk-ssr-app \
  --bin /tmp/gowdk-ssr-site \
  examples/basic/simple-ssr.page.gwdk
```

`load {}` parsing, guard enforcement, dynamic SSR routes, and broad
request-time user logic are still planned.

## Route Plans

Use `gowdk routes` to inspect the validated route-binding plan:

```sh
gowdk routes --ssr examples/basic/*.gwdk
```

The current JSON schema is version `1` and includes route kind, method, route
pattern, page ID, and planned handler information.

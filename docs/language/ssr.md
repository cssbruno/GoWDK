# SSR

SSR is optional and must not become the default framework identity.

## Current Support

- Pages default to build-time SPA output.
- `server {}` selects request-time SSR and requires the SSR addon.
- `go server {}` also selects request-time SSR and requires the SSR addon.
- `gowdk build --ssr --app <dir> --bin <file>` can generate a binary that
  serves concrete and dynamic request-time SSR pages rendered from `view {}`,
  literal or imported `build {}` data, and declared `server {}` data.
- Dynamic SSR routes such as `/blog/{slug}` can be matched by generated
  binaries in the first supported slice. Route params render through generated
  placeholders and request-time HTML escaping. Generated handlers attach raw
  params through `runtime/app.Params(ctx)` and decoded typed params through
  `runtime/app.TypedParams(ctx)` before guards, load functions, or rendering
  run. Invalid typed params return 400; missing params return 404.
- Generated SSR supports declared identifier and dotted-path fields such as
  `server { => { user, title, account.plan } }` and calls a same-package exported
  Go function named `Load<PageID>`. `<PageID>` is the explicit `page` value
  when present, otherwise the filename-derived page ID.
- Supported load function signatures are
  `func LoadDashboard(ssr.LoadContext) map[string]any` and
  `func LoadDashboard(ssr.LoadContext) (map[string]any, error)`. Returned
  values replace generated SSR placeholders with request-time HTML escaping.
  Dotted paths resolve through nested maps with string keys, structs, pointers,
  interfaces, exported Go field names, and `json` tag names.
- Load functions can return `ssr.RedirectTo("/login")` or
  `ssr.Redirect("/login", http.StatusTemporaryRedirect)` to ask generated SSR
  handlers to write a no-store local redirect. Redirect URLs must be local
  absolute paths.
- `error "/errors/dashboard.html"` declares a route-local generated HTML
  error document for SSR load failures, generated render failures, and route
  panics before response headers are written on that page. The path is
  output-relative, may be written with a leading slash, must end in `.html`,
  and must not contain `..`, query strings, fragments, or backslashes.
- Generated embedded apps load optional `404.html` and `500.html` documents
  from build output, plus any route-local `error` documents selected by SSR
  routes. Missing error documents fall back to `http.Error`.
- Generated SSR route handlers run inside a runtime panic boundary. A panic
  before response headers are written becomes a no-store HTTP 500 response,
  using the route-local `error` page when declared or `500.html` when present,
  without exposing the panic value.
- Non-redirect `server {}` failures also use the same 5xx message policy:
  ordinary error details are hidden, and only explicit
  `response.HandlerError.Message` values are rendered to clients.
- Page layouts compose around SSR pages at request time. Declared load data is
  merged into the request render scope before the page and layout stack are
  written.
- Successful SSR HTML uses the page `cache`/`revalidate` policy when declared
  and otherwise uses `Cache-Control: no-store`. Load redirects, guard failures,
  route-local error pages, and panic boundaries are always no-store.
- The SSR addon exposes a small router registration contract for generated SSR
  page handlers.
- The SSR addon provides a default HTTP 500 error handler contract for
  request-time SSR failures.
- `guard` is optional, but a page is not public by default: a page that
  declares no `guard` warns (`missing_page_guard`) and its route is denied
  (403) at request time until access is stated (see
  [guards.md](guards.md) for the full access contract). `guard public` marks an
  intentionally public page and must stand alone. Non-public guards use
  comma-separated guard IDs such as `guard auth.required, billing.active`.
  Protected page guards require request-time page rendering so the page GET
  route can be gated before HTML is returned. `runtime/guard` exposes
  `Context`, `Registry`, and ordered guard execution contracts. Generated SSR,
  action, API, and fragment handlers run declared guards before user
  logic. A guarded generated app will not compile unless required guard backing
  functions exist. Ordinary guard errors fail closed with HTTP 403. Guards can
  intentionally return `runtime/guard.RedirectTo`, `runtime/guard.Redirect`, or
  `runtime/guard.Respond` errors to write no-store redirects or custom
  responses. Native RBAC guard IDs use `role:<name>` and
  `permission:<name>` and resolve through an application-owned
  `runtime/auth.Provider`.

Generated app packages that include at least one guarded SSR, action, API, or
fragment route require backing functions in the generated app package unless
`auth.Addon` supplies them. With `auth.Addon(auth.Options{...})`, generated
startup configures the session manager, registers `auth.required`, and uses that
session manager for native `role:` / `permission:` guards.

```go
func GOWDKGuardRegistry() gowdkguard.Registry // required when custom guard IDs are used
func GOWDKAuthProvider() auth.Provider        // required when role:/permission: IDs are used without auth.Addon
```

Define custom guards in app startup code that is compiled with the generated app
package. If this function is missing while custom guard IDs are declared, `go
build` fails.

```go
package gowdkapp

import gowdkguard "github.com/cssbruno/gowdk/runtime/guard"

func GOWDKGuardRegistry() gowdkguard.Registry {
	return gowdkguard.Registry{
		"auth.required": func(ctx gowdkguard.Context) error {
			return nil
		},
	}
}
```

For native RBAC guards, define only the application-owned principal source. If
this function is missing while `role:` or `permission:` guard IDs are declared,
`go build` fails.

```go
import (
	"net/http"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
)

func GOWDKAuthProvider() gowdkauth.Provider {
	return gowdkauth.ProviderFunc(func(request *http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{
			ID:          "user-1",
			Roles:       []string{"admin"},
			Permissions: []string{"dashboard.read"},
		}, nil
	})
}
```

Feature packages that declare page, action, or API handlers should not import
the generated `gowdkapp` package. Keep registration in the generated app
package to avoid import cycles.

Native RBAC guards are a defense-in-depth redundancy layer for generated
route/page access. They must never replace backend authorization for protected
resources in normal Go handlers and services.

## Lane inference: one directive, two lanes

GOWDK has two execution lanes for `g:for` and `g:if`, and the compiler picks the
lane from the **data source**, not from a separate directive:

- When the operand is a **`server {}` request-time field** (or, when nested, the
  enclosing row item), `g:for`/`g:if` render **server-side** at request time, with
  escape-by-default interpolation — no HTML is built in Go and no client island is
  involved.
- When the operand is **client `state`/`store`**, `g:for`/`g:if` bind a **reactive
  client island**.

So `g:for={col in columns}` over a `server {}` field is a server-rendered list,
while `g:for={todo in todos}` over component `state` is a client island — same
directive, lane chosen by where the data lives. A name that is neither a declared
`server {}` field nor client state is rejected. There are no separate `g:each`/`g:when` directives; the lane is inferred. <!-- removed-syntax-ok: documents the g:each/g:when -> g:for/g:if rename -->

## Server-rendered lists (`g:for` over `server {}`)

Request-time pages render collection data — board columns, chat logs, activity
feeds, search results, inboxes — declaratively with `g:for` over a `server {}`
field. Every interpolated value is HTML-escaped.

```gwdk
page board
route "/board"
guard public
server { => { columns } }
view {
  <section class="board">
    <div class="column" g:for={col in columns}>
      <h2>{col.title}</h2>
      <article class="card" g:for={issue in col.issues}>
        <span>{issue.id}</span> {issue.title}
      </article>
    </div>
  </section>
}
```

```go
func LoadBoard(ssr.LoadContext) (map[string]any, error) {
	b := issues.Board()
	return map[string]any{"columns": b.Columns}, nil
}
```

Contract:

- A top-level `g:for` over a declared `server {}` field renders server-side. The
  same `g:for` over component `state`/`store` is a client island instead — the
  lane follows the source.
- Rows interpolate the item with `{item.Field}` (dotted paths such as
  `{item.author.name}` are supported) and the optional index with
  `g:for={item, i in field}` then `{i}`. Field values are matched against map
  keys, exported Go struct fields, or json tags, and are always escaped.
- Server lists nest. A nested `g:for={child in item.children}` must reference the
  enclosing row item; its slice is resolved per parent row. Nested directives
  inherit the server lane.
- Rows support static markup, item interpolation, nested `g:for`, and nested
  `g:if` only. Components, other client directives (`g:on:*`, `g:bind:*`,
  islands), and `g:unsafe-html` are not part of a server row. Request-time
  (tainted) values remain rejected in URL, event-handler, `style`, and `srcdoc`
  attributes.
- A server-rendered `g:for` requires the SSR addon and a request-time page; it
  has no SPA/static output form. `g:key` is accepted but ignored server-side.

## Server-rendered conditionals (`g:if` over `server {}`)

`g:if` over a `server {}` field renders its element (and subtree) at request time
only when the condition holds. This covers the everyday empty-state, auth-gated
section, and feature-flag patterns over request-time data.

```gwdk
page board
route "/board"
guard public
server { => { count, status } }
view {
  <section>
    <p g:if={count > 0 && status == "open"}>You have {count} open items</p>
    <p g:if={!count}>No issues yet</p>
  </section>
}
```

```go
func LoadBoard(ssr.LoadContext) (map[string]any, error) {
	b := issues.Board()
	return map[string]any{"count": b.Count, "status": b.Status}, nil
}
```

Contract:

- A top-level `g:if` whose condition references a `server {}` field renders
  server-side; over client `state`/`store` the same `g:if` is a client
  conditional instead.
- A top-level server `g:if` accepts a full bool expression — comparisons (`==`,
  `!=`, `<`, `<=`, `>`, `>=`), logic (`&&`, `||`, `!`), and literals — over
  `server {}` fields, evaluated at request time. A value with no operator is a
  truthiness check (non-zero number, non-empty string, `true`, non-empty
  slice/map). Evaluation that fails (e.g. a missing field) fails closed: the
  branch is hidden. Function calls are not evaluated server-side — compute those
  in Go and expose a field.
- A `g:if` branch shares the enclosing scope: a top-level branch interpolates
  `server {}` fields (`{count}`); a `g:if` inside a server `g:for` row references
  the row item (`{issue.id}`), and a **nested** server `g:if` is a single row
  field (`g:if={issue.urgent}`), not a compound expression.
- Server `g:for` and `g:if` nest in either direction: a list inside a branch, a
  conditional inside a row.
- The empty/else branch is a sibling `g:if={!field}`. `g:else`/`g:else-if` are
  client-only chains and cannot follow a server `g:if`.
- A server-rendered `g:if` requires the SSR addon and a request-time page; it has
  no SPA/static output form.

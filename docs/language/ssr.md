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
  `server { => { user, title, account.plan } }`. `Config.Interop.Loads`
  explicitly maps the page ID to an exported Go function with
  `gowdk.RegisterLoad("dashboard", dashboard.Load)`.
- Supported load function signatures are
  `func LoadDashboard(ssr.LoadContext) map[string]any`,
  `func LoadDashboard(ssr.LoadContext) (map[string]any, error)`,
  `func LoadDashboard(ssr.LoadContext) DashboardData`, and
  `func LoadDashboard(ssr.LoadContext) (DashboardData, error)`.
  Typed result structs must be exported structs. Exported fields
  are exposed by Go field name or `json` tag name, and `json:"-"` hides a field.
  Returned values replace generated SSR placeholders with request-time HTML
  escaping. Dotted paths resolve through nested maps with string keys, structs,
  pointers, interfaces, exported Go field names, and `json` tag names.
- Load functions can return `ssr.RedirectTo("/login")` or
  `ssr.Redirect("/login", http.StatusTemporaryRedirect)` to ask generated SSR
  handlers to write a no-store local redirect. Redirect URLs must be local
  absolute paths.
- Load functions can return typed expected errors from `runtime/response`, such
  as `response.NotFound`, `response.Forbidden`, `response.ValidationFailed`, or
  `response.ServerError`. Generated SSR maps those to 404, 403, 422, or 500 and
  keeps the response no-store.
- `error "/errors/dashboard.html"` declares a route-local generated HTML
  error document for SSR load failures, generated render failures, and route
  panics before response headers are written on that page. The path is
  output-relative, may be written with a leading slash, must end in `.html`,
  and must not contain `..`, query strings, fragments, or backslashes.
- Layout files can also declare `error "/errors/app-shell.html"`. SSR 500
  boundaries select route-local error pages first, then the nearest layout
  error page, then outer layout error pages, then `500.html`.
- Generated embedded apps load optional `404.html` and `500.html` documents
  from build output, plus any route-local or layout-level `error` documents
  selected by SSR routes. Missing error documents fall through to the next
  boundary and eventually `http.Error`.
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
  logic. Missing backing registrations are compiler diagnostics. Ordinary guard
  errors fail closed with HTTP 403. Guards can
  intentionally return `runtime/guard.RedirectTo`, `runtime/guard.Redirect`, or
  `runtime/guard.Respond` errors to write no-store redirects or custom
  responses. Native RBAC guard IDs use `role:<name>` and
  `permission:<name>` and resolve through an application-owned
  `runtime/auth.Provider`.

Declare request-time loads and custom backing providers explicitly in config.
Registrations accept real Go function values and live in ordinary app packages:

```go
Interop: gowdk.InteropConfig{
	Loads: []gowdk.LoadRegistration{
		gowdk.RegisterLoad("dashboard", dashboard.Load),
	},
	Guards:       gowdk.RegisterGuards(security.Guards),
	AuthProvider: gowdk.RegisterAuthProvider(security.AuthProvider),
},
```

`RegisterLoad` replaces the `Load<PageID>` naming convention. Custom guards need
`RegisterGuards`. Native `role:`/`permission:` guards need
`RegisterAuthProvider` only when `auth.Addon` is not configured. The addon still
supplies `auth.required` and its session-backed provider automatically.

```go
import (
	"net/http"

	gowdkauth "github.com/cssbruno/gowdk/runtime/auth"
)

func AuthProvider() gowdkauth.Provider {
	return gowdkauth.ProviderFunc(func(request *http.Request) (*gowdkauth.Principal, error) {
		return &gowdkauth.Principal{
			ID:          "user-1",
			Roles:       []string{"admin"},
			Permissions: []string{"dashboard.read"},
		}, nil
	})
}
```

Feature packages never import the generated `gowdkapp` package. Missing typed
registrations fail during compiler validation and appear in `inspect
go-bindings`.

Native RBAC guards are a defense-in-depth redundancy layer for generated
route/page access. They must never replace backend authorization for protected
resources in normal Go handlers and services.

## Explicit directive lanes

GOWDK has two execution lanes for `g:for` and `g:if`. Source declares the lane
with `g:lane="server"` or `g:lane="client"`, and the compiler verifies it
against data ownership:

- When the operand is a **`server {}` request-time field** (or, when nested, the
  enclosing row item), `g:for`/`g:if` render **server-side** at request time, with
  escape-by-default interpolation — no HTML is built in Go and no client island is
  involved.
- When the operand is **client `state`/`store`**, `g:for`/`g:if` bind a **reactive
  client island**.

So `g:for={col in columns} g:lane="server"` over a `server {}` field is a
server-rendered list, while component state uses
`g:for={todo in todos} g:lane="client"`. A missing lane gets
`directive_lane_required`; a declaration that disagrees with its data gets
`directive_lane_mismatch`. There are no separate `g:each`/`g:when` directives. <!-- removed-syntax-ok: documents the g:each/g:when -> g:for/g:if rename -->

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
    <div class="column" g:for={col in columns} g:lane="server">
      <h2>{col.title}</h2>
      <article class="card" g:for={issue in col.issues} g:lane="server">
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

- A top-level `g:for` with `g:lane="server"` must reference a declared
  `server {}` field. Component `state`/`store` requires `g:lane="client"`.
- Rows interpolate the item with `{item.Field}` (dotted paths such as
  `{item.author.name}` are supported) and the optional index with
  `g:for={item, i in field} g:lane="server"` then `{i}`. Field values are matched against map
  keys, exported Go struct fields, or json tags, and are always escaped.
- Server lists nest. A nested `g:for={child in item.children} g:lane="server"` must reference the
  enclosing row item; its slice is resolved per parent row. Nested directives
  declare `g:lane="server"` too.
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
    <p g:if={count > 0 && status == "open"} g:lane="server">You have {count} open items</p>
    <p g:if={!count} g:lane="server">No issues yet</p>
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

- A top-level `g:if` with `g:lane="server"` must reference a `server {}` field;
  client `state`/`store` uses `g:lane="client"`.
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
  field (`g:if={issue.urgent} g:lane="server"`), not a compound expression.
- Server `g:for` and `g:if` nest in either direction: a list inside a branch, a
  conditional inside a row.
- The empty/else branch is a sibling `g:if={!field} g:lane="server"`. `g:else`/`g:else-if` are
  client-only chains and cannot follow a server `g:if`.
- A server-rendered `g:if` requires the SSR addon and a request-time page; it has
  no SPA/static output form.

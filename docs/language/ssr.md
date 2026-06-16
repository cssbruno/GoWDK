# SSR

SSR is optional and must not become the default framework identity.

## Current Support

- Pages default to build-time SPA output.
- `load {}` selects request-time SSR and requires the SSR addon.
- `go ssr {}` also selects request-time SSR and requires the SSR addon.
- `gowdk build --ssr --app <dir> --bin <file>` can generate a binary that
  serves concrete and dynamic request-time SSR pages rendered from `view {}`,
  literal or imported `build {}` data, and declared `load {}` data.
- Dynamic SSR routes such as `/blog/{slug}` can be matched by generated
  binaries in the first supported slice. Route params render through generated
  placeholders and request-time HTML escaping. Generated handlers attach raw
  params through `runtime/app.Params(ctx)` and decoded typed params through
  `runtime/app.TypedParams(ctx)` before guards, load functions, or rendering
  run. Invalid typed params return 400; missing params return 404.
- Generated SSR supports declared identifier and dotted-path fields such as
  `load { => { user, title, account.plan } }` and calls a same-package exported
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
- Non-redirect `load {}` failures also use the same 5xx message policy:
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
fragment route require backing functions in the generated app package:

```go
func GOWDKGuardRegistry() gowdkguard.Registry // required when custom guard IDs are used
func GOWDKAuthProvider() auth.Provider        // required when role:/permission: IDs are used
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

## Server-rendered lists (`g:each`)

Request-time pages render collection data — board columns, chat logs, activity
feeds, search results, inboxes — declaratively with `g:each`, the server-side
counterpart to the client-only `g:for`. The list is rendered server-side at
request time with escape-by-default interpolation: no HTML is built in Go, no
client island is involved, and every interpolated value is HTML-escaped.

```gwdk
page board
route "/board"
guard public
load { => { columns } }
view {
  <section class="board">
    <div class="column" g:each={col in columns}>
      <h2>{col.title}</h2>
      <article class="card" g:each={issue in col.issues}>
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

- A top-level `g:each` collection must be a declared `load {}` field. Iterating
  client/island state belongs to `g:for`; iterating request-time `load {}` data
  with `g:for` is rejected at `gowdk check` with a diagnostic pointing at
  `g:each`.
- Rows interpolate the item with `{item.Field}` (dotted paths such as
  `{item.author.name}` are supported) and the optional index with
  `g:each={item, i in field}` then `{i}`. Field values are matched against map
  keys, exported Go struct fields, or json tags, and are always escaped.
- `g:each` lists nest. A nested `g:each={child in item.children}` must reference
  the enclosing row item; its slice is resolved per parent row.
- Rows support static markup, item interpolation, and nested `g:each` only.
  Components, client directives (`g:on:*`, `g:if`, `g:bind:*`, islands), and
  `g:html` are not part of a server row. Request-time (tainted) values remain
  rejected in URL, event-handler, `style`, and `srcdoc` attributes.
- `g:each` requires the SSR addon and a request-time page; it has no SPA/static
  output form.

## Server-rendered conditionals (`g:when`)

`g:when` is the server-side counterpart to the client-only `g:if`. It renders
its element (and subtree) at request time only when an SSR `load {}` field is
truthy, with a leading `!` for the inverse. This covers the everyday empty-state,
auth-gated section, and feature-flag patterns over request-time data.

```gwdk
page board
route "/board"
guard public
load { => { hasItems, count } }
view {
  <section>
    <p g:when={hasItems}>You have {count} items</p>
    <p g:when={!hasItems}>No issues yet</p>
  </section>
}
```

```go
func LoadBoard(ssr.LoadContext) (map[string]any, error) {
	b := issues.Board()
	return map[string]any{"hasItems": b.Count > 0, "count": b.Count}, nil
}
```

Contract:

- A top-level `g:when` condition must be a declared `load {}` field, optionally
  negated with a leading `!`. Branching client/island state belongs to `g:if`;
  branching request-time `load {}` data with `g:if` is rejected at `gowdk check`
  with a diagnostic pointing at `g:when`.
- The condition is a single field reference, not a compound expression. A value
  is truthy when it is a non-zero number, non-empty string, `true`, or a
  non-empty slice/map. Compute compound conditions in Go and expose a bool load
  field.
- A `g:when` branch shares the enclosing scope: a top-level branch interpolates
  `load {}` fields (`{count}`); a `g:when` inside a `g:each` row references the
  row item (`{issue.id}`), and its condition must reference the row item
  (`g:when={issue.urgent}`).
- `g:when` and `g:each` nest in either direction: a list inside a branch, a
  conditional inside a row. Branches support static markup, scoped
  interpolation, nested `g:each`, and nested `g:when` only.
- The empty/else branch is expressed with a sibling `g:when={!field}`; an
  `else`/`else-if` chain is not part of this directive.
- `g:when` requires the SSR addon and a request-time page; it has no SPA/static
  output form.

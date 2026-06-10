# Current .gwdk Language Spec

This is the current `.gwdk` language contract for the experimental 0.x line. It
describes implemented and partial syntax only as far as the compiler supports
it today. Planned syntax is listed so unsupported source can fail clearly
instead of becoming accidental behavior.

Detailed behavior stays in the feature pages linked from
[GOWDK Language](README.md).

## Status Terms

- Implemented: accepted by the current compiler and covered by tests or a
  documented verification command.
- Partial: accepted for a narrower slice than the final contract.
- Planned: accepted direction, but not stable source behavior.
- Unsupported: intentionally rejected or not accepted by the current parser.

## File Model

`.gwdk` files are package-peer source files. They live beside normal `.go`
files and declare the same Go package name:

```gwdk
package pages
```

The package declaration must be the first non-comment declaration. GOWDK source
files do not derive route identity from folders.

Current file kinds:

- Page files declare `@route` and `view {}`. `@guard` is optional but a page is
  not public by default (see below). They may declare `@page` when they need an
  explicit stable page ID.
- Component files declare `@component` and usually `view {}`.
- Layout files declare `@layout` and `view {}` with a `<slot />`.

Planned or partial file kinds:

- API-only files are planned separately. Current pages still own a page `GET`
  route and must declare `view {}`.
- Island-only and plugin-adjacent files are not separate stable file kinds yet.

## Comments

Line comments start with `//`. Empty lines and line comments are ignored by the
metadata parser outside block bodies.

## Annotations

Annotations start with `@` at the beginning of the trimmed line.

Implemented or partial annotations:

- `@page <id>`
- `@route "<path>"`
- `@title "<text>"`
- `@description "<text>"`
- `@canonical "<url>"`
- `@image "<url>"`
- `@layout <id>[, <id>...]`
- `@cache "<policy>"`
- `@revalidate <seconds|duration>`
- `@error "<path.html>"`
- `@guard <id>[, <id>...]`
- `@component <Name>`

Unknown annotations are errors. Malformed `@` lines are errors.

`@guard` is optional on page sources, but a page is not public by default. A
page that declares no `@guard` builds with a `missing_page_guard` warning and
its route is denied (403) at request time until access is stated; the build
still succeeds. Use `@guard public` to serve a page on purpose. Use custom guard
IDs or native RBAC IDs such as `role:admin` and `permission:posts.write` when the
page is protected. `@guard public` must stand alone. Access is never granted by
omission.

`@page` is optional for file-backed page sources. When omitted, the compiler
derives the page ID from the source filename by removing `.page.gwdk` or
`.gwdk`. For example, `src/pages/blog-post.page.gwdk` derives page ID
`blog-post`. Add `@page blog.post` when a route, filename, or file location
change must not change page identity.

## Imports And Uses

Go imports bind normal Go packages for build data, component props/state, and
handler references:

```gwdk
import interop "github.com/acme/app/interop"
import "github.com/acme/app/ui"
```

GOWDK `use` declarations bind discovered `.gwdk` source packages for
cross-package component calls:

```gwdk
use ui "components"
```

Go imports do not import `.gwdk` files. `use` values are package names from
discovered GOWDK sources, not Go import paths.

## Routes

Routes are explicit:

```gwdk
@route "/patients/{id:int}"
```

Current route rules:

- Routes must start with `/`.
- `/` is the only route that may end with `/`.
- Query strings, fragments, backslashes, whitespace, control characters, empty
  segments, `.`, and `..` are invalid.
- Dynamic params are whole segments such as `{slug}` or `{id:int}`.
- Param names use Go-like identifier spelling.
- Supported param types are `string`, `int`, `int64`, `uint`, `uint64`,
  `bool`, and `float64`.
- Duplicate params in one route are invalid.
- Duplicate route patterns are invalid.

Dynamic SPA routes require `paths {}` unless the page selects request-time
rendering with `load {}` or `go ssr {}`.

## Blocks

Implemented or partial top-level blocks:

- `paths {}`: build-time dynamic SPA path declarations.
- `build {}`: build-time page data.
- `load {}`: request-time page data; requires the SSR addon.
- `view {}`: page, component, or layout markup.
- `style {}`: component/page-local CSS body capture.
- `go {}`: optional same-package Go extraction.
- `go ssr {}`: optional request-time load-handler extraction.
- `go client {}`: optional browser-side Go WASM mount extraction.
- `go addon.<name> {}`: addon-owned Go block validation and emission.
- `js "<relative-file.js|.mjs|.ts>"`: scoped browser module asset.
- `js {}`: inline scoped browser module asset for small cases.

Unsupported top-level block declarations that look like `name ... {` are
rejected until their feature slice is implemented.

## Build-Time Data

`paths {}` supports the first literal record subset:

```gwdk
paths {
  => { slug: "hello-gowdk" }
}
```

`build {}` supports literal records, references to earlier fields, route
params, and no-argument Go function calls from imported or same-package Go:

```gwdk
build {
  => { title: "Hello", count: 2 }
  => FeaturedCopyForBuild()
  => interop.FeaturedCopyForBuild()
}
```

Arbitrary build-time Go statements are not stable source behavior.

## Request-Time Data

`load {}` selects the request-time page lane and requires SSR to be enabled.
Generated SSR supports declared field placeholders and same-package Go load
functions through `ssr.LoadContext` for the current slice.

`go ssr {}` can provide generated SSR load handlers when request-time rendering
is enabled.

## Endpoints

Actions and APIs are top-level endpoint declarations:

```gwdk
act Submit POST "/signup"
api Health GET "/api/health"
```

Endpoint-local error pages are supported:

```gwdk
act Submit POST "/signup" @error "/errors/signup.html"
```

Current rules:

- Action methods are currently `POST`.
- API methods support `GET`, `POST`, `PUT`, `PATCH`, and `DELETE`.
- Declarations name exact exported Go handler symbols.
- Behavior lives in normal same-package Go handlers or supported extracted Go
  blocks.
- Old `act name { ... }` and `api name { ... }` blocks are rejected with
  migration diagnostics.

## View Markup

`view {}` supports the current GOWDK markup subset:

- Lowercase HTML elements.
- Text and quoted attribute interpolation.
- Escaped text and attributes by default.
- Self-closing component calls.
- Same-package component calls by bare component name.
- Cross-package component calls through `use` aliases.
- Shorthand class and id attributes on HTML elements.
- Boolean attributes and first-slice expression attributes.
- `<slot />` in layouts and components.
- First-slice form enhancement directives such as `g:post`, `g:target`, and
  `g:swap`.
- First-slice local island directives such as `g:on:*` and `g:island`.
- The explicit `g:html={Expr}` raw HTML directive on non-void elements.

`view {}` expands only through GOWDK-owned AST nodes and `g:` directives. The
current parser does not implement arbitrary external template semantics.
Unsupported template tags and unknown `g:` directives must fail with
diagnostics (`unsupported_markup_syntax` and `unsupported_markup_directive`
message families) rather than being treated as raw HTML or silently ignored.

## Components

Components use `@component <Name>`. They can define props and state contracts
through imported Go types and can render same-package or `use`-qualified child
components.

Current component support is partial:

- String-like props and first typed Go prop/state contracts are supported.
- Component CSS and assets can be scoped and emitted.
- Component-level `@wasm` can emit browser WASM island assets.
- Rich non-string props, broad lifecycle behavior, child-to-parent events, and
  a full reactive graph are planned.

## Scoped JavaScript

Scoped browser modules are explicit source declarations:

```gwdk
js "./dashboard.ts"

js {
  console.log("loaded")
}
```

Path-based modules are preferred. Script declarations are page- or
component-scoped. TypeScript is transform-only; GOWDK does not type-check it.
Bundling, minification, import-graph following, and JavaScript tree shaking are
not implemented.

Generated JavaScript is enhancement only. It must not own routing truth, auth,
trusted validation, server state, business rules, or cache policy.

## Raw HTML Policy

Generated HTML escapes text and attributes by default; that default is
unchanged. The one explicit, stable escape hatch is the GOWDK-owned
`g:html={Expr}` element directive: the element's attributes stay escaped and
the expression's resolved string is written as the element content without
escaping. Only trusted or sanitized HTML may be fed to `g:html`; see
[markup.md](markup.md) for the full contract and restrictions. Raw HTML syntax
from other template languages, such as `{@html ...}`, remains unsupported and
fails loudly with guidance toward `g:html`.

## Diagnostics Policy

Compiler failures should include a diagnostic code, source position, source
range when known, severity, message, and short suggestion when useful. Current
public diagnostics are listed in [diagnostics.md](diagnostics.md) and
[Diagnostics Reference](../reference/diagnostics.md).

Parser recovery is still partial. `parse_error` remains the broad parser code
until more specific parser diagnostics are stabilized.

## Compatibility Policy

During 0.x, public contracts can still change. Compatibility rules for language
work:

- Implemented syntax should keep tests or documented verification.
- Partial syntax must stay labeled partial in docs.
- Planned syntax must not be silently accepted as generated behavior.
- Deprecated syntax should fail with migration diagnostics before removal.
- Unsupported syntax should fail before generated output is accepted.
- Docs must change in the same commit as behavior changes.

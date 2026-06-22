# Syntax

The current parser recognizes a small top-level subset.

## Comments

Line comments start with `//`. Empty lines and line comments are ignored by the metadata parser.

## Package

Every real page, component, and layout `.gwdk` file must start with a Go-style
package declaration as the first non-comment declaration:

```gwdk
package auth
```

The package name must match sibling `.go` files in the same directory.
`gowdk.config.go` is project configuration and is not treated as a sibling
application package for this check.

Malformed package declarations are rejected with
`malformed_package_declaration`:

```gwdk
package 123
```

## Metadata

Metadata must start at the beginning of the trimmed line:

```gwdk
route "/"
title "Home"
description "Compile-first Go web output."
canonical "https://example.com/"
image "https://example.com/social.png"
robots "index,follow"
noindex false
preload "/assets/app.css" as "style"
prefetch "/docs"
layout root, marketing
cache "public, max-age=60"
revalidate 5m
error "/errors/home.html"
guard public
component Hero
layout root
```

Supported metadata declarations:

- `page <id>`: optional stable page ID override. When omitted from a
  file-backed page, the ID derives from the filename by removing `.page.gwdk`
  or `.gwdk`.
- `route "<path>"`: required route path. Quotes are trimmed.
- `title "<text>"`: optional HTML document title for generated page output.
- `description "<text>"`: optional HTML document description meta value.
- `canonical "<url>"`: optional canonical URL link for generated page output.
- `image "<url>"`: optional social preview image URL for Open Graph and
  Twitter metadata.
- `robots "<policy>"`: optional robots meta content.
- `noindex [true|false]`: optional shorthand for adding `noindex` to the
  robots meta content. A bare `noindex` line is treated as `true`.
- `preload "<href>" [as "<type>"]`: optional head preload link. Absolute
  URLs must be `http` or `https`; protocol-relative and active-content URLs
  are rejected.
- `prefetch "<href>" [as "<type>"]`: optional head prefetch link with the
  same URL restrictions as `preload`.
- `layout <id>[, <id>...]`: optional page layout IDs, or a layout identity in
  `.layout.gwdk` files.
- `cache "<policy>"`: optional page Cache-Control policy for successful
  generated static SPA HTML and SSR HTML responses.
- `revalidate <seconds|duration>`: optional stale-while-revalidate duration
  such as `60`, `60s`, `5m`, or `1h`; requires `cache`.
- `error "<path.html>"`: optional route-local generated HTML error page for
  SSR load, generated render, and panic failures. The path is output-relative
  after normalization and must stay inside generated output.
- `guard <id>[, <id>...]`: page access metadata. Optional, but a page is not
  public by default: omitting `guard` warns (`missing_page_guard`) and the
  route is denied (403) until stated. Use `guard public` for intentionally
  public pages. Use custom guard IDs or native RBAC IDs such as `role:admin` and
  `permission:posts.write` for protected pages.
- `component <Name>`: component ID for `.cmp.gwdk` build inputs.

Unknown metadata declarations are rejected. Lines starting with `@` are rejected
as malformed legacy metadata.

`guard public` must be the only guard on a page. Protected pages can declare
multiple non-public guards; generated handlers enforce them in declaration
order. Protected page guards require request-time page rendering for frontend
page access; add `server {}` or `go server {}` with the SSR addon when the page is
not public.

Current route validation accepts canonical absolute paths only:

- Routes must start with `/`.
- `/` is the only route that may end with a trailing slash.
- Routes must not contain query strings, fragments, backslashes, whitespace,
  control characters, empty segments, `.`, or `..`.
- Dynamic route params must be whole path segments such as `/blog/{slug}` or
  `/patients/{id:int}`.
- Route param names use `[A-Za-z_][A-Za-z0-9_]*` and may not repeat in one
  route.
- Route param types are optional and support `string`, `int`, `int64`, `uint`,
  `uint64`, `bool`, and `float64`.
- Duplicate page route patterns are invalid. `/blog/{slug}` and `/blog/{id}`
  are the same pattern.

## Scoped JavaScript

Pages and components can declare browser module files with top-level `js`
declarations:

```gwdk
js "./dashboard.ts"
```

The path is relative to the declaring `.gwdk` file and must end in `.js`,
`.mjs`, or `.ts`. GOWDK copies JavaScript files into generated output, transforms
TypeScript files into `.js` module output, and emits `<script type="module">`
only for the page that declares it, or for pages that call a component that
declares it. TypeScript is transform-only; GOWDK does not type-check it.

Inline browser code is supported for small cases, but path-based modules are
preferred:

```gwdk
js {
  console.log("loaded")
}
```

This is scoped asset inclusion, not JavaScript bundling; imported JavaScript or
TypeScript dependencies are not followed yet.

## Blocks

The parser recognizes these top-level block declarations:

```gwdk
paths {
build {
server {
view {
```

Actions and APIs are top-level endpoint declarations, not blocks:

```gwdk
act Submit POST "/signup" error "/errors/signup.html"
api Health GET "/api/health" error "/errors/api-health.html"
```

The endpoint-local `error` suffix is optional. When present on `act` or `api`,
generated action/API panic boundaries use that generated HTML page before
falling back to `500.html`.

Page files may also declare top-level Go imports before blocks:

```gwdk
import interop "github.com/cssbruno/gowdk/examples/go-interop"
```

These are normal Go package imports used for Go types/functions. They do not
import other `.gwdk` files.

Malformed Go imports are rejected with `malformed_go_import`:

```gwdk
import interop github.com/acme/app/interop
```

GOWDK source packages use a separate `use` declaration:

```gwdk
use ui "components"
```

The quoted value is a discovered `.gwdk` package name, not a Go import path.
Pages and components use this for cross-package component calls:

```gwdk
view {
  <ui.Hero title="GOWDK" />
}
```

Same-package `.gwdk` and `.go` files are peers and need no import. A page can
call same-package components by bare component name, such as `<Hero />`.
Cross-package page calls must use a declared alias, such as `<ui.Hero />`.
Imported components still resolve their own same-package child components by
bare name inside the imported component body.
Component aliases are scoped to the component that declares them:

```gwdk
package marketing

use icons "icons"

component Hero

view {
  <section><icons.Badge /></section>
}
```

Component files can declare Go imports for typed props and state contracts:

```gwdk
import ui "github.com/acme/app/ui"
import "github.com/acme/app/components"
```

Aliased imports use the explicit alias. Unaliased imports use the package name
reported by `go list`, matching ordinary Go import behavior. Relative import
paths are rejected for typed component contracts.

Build-time Go interop supports imported or same-package no-argument function
calls in `build {}`:

```gwdk
build {
  => interop.FeaturedCopyForBuild()
  => FeaturedCopyForBuild()
}
```

The function must return `T` or `(T, error)` where `T` is a JSON-encodable
object. Scalar fields are exposed to `view {}` as string interpolation data.

Unsupported top-level block declarations that look like `name ... {` are
rejected until their feature slice is implemented.

`internal/gwdkast` defines the typed GOWDK AST. `internal/parser.ParseSyntax`
returns that AST for the current subset: package declarations, metadata declarations,
Go imports, GOWDK uses, stores, typed component contracts, blocks, parsed
`view {}` markup nodes, literal `paths {}` and `build {}` records, endpoint
declarations, and source spans.
The manifest parser still preserves raw block body text for compatibility.
SPA builds can expand literal `paths {}` lines such as:

```gwdk
=> { slug: "hello-gowdk" }
```

SPA builds can also render multiple literal `build {}` lines such as:

```gwdk
=> { title: "Hello" }
=> { count: 2, live: true }
=> { headline: "{title} {slug}", copy: field("headline") }
=> { total: (count + 3) * 2, visible: live && total > 5 }
```

Literal build values can be strings, numbers, booleans, `nil`/`null`,
`param("name")`, `field("name")`, or a bare reference to an earlier build
field. Build expressions support string concatenation with `+`, numeric
`+`, `-`, `*`, `/`, and `%`, boolean `!`, `&&`, and `||`, equality and ordered
comparisons, parentheses, and unary `+`/`-`. Duplicate build fields are
rejected.

### Build-time iteration and transforms

Build values can also be **lists** and **objects**, and `build {}` can reshape
them at build time with bounded, deterministic iteration:

```gwdk
=> { prices: [12, 49, 8, 99, 25] }
=> { premium: [p for p in field("prices") if p >= 25] }
=> { premiumCount: count(field("premium")), revenue: sum(field("premium")) }
=> { tiers: [ {label: "tier-" + n, slot: n} for n in seq(1, 4) ] }
=> { tierLabels: [t.label for t in field("tiers")] }
=> { heading: join(field("tierLabels"), " / ") }
```

- `[a, b, c]` is a list literal; `{ name: value, ... }` is an object literal.
- `seq(end)` and `seq(start, end)` produce a half-open integer range.
- A comprehension `[expr for v in source]` maps each element of a list `source`;
  add `if cond` to filter, and `for v, i in source` to bind a zero-based index.
- `v.field` reads an object field and `list[i]` indexes a list.
- Reductions `count`, `sum`, `join(list, sep)`, `first`, `last`, `take(list, n)`,
  and `reverse` operate on lists; `seq` and these builtins compose anywhere as
  ordinary calls.
- Bracket and brace forms (list/object literals and comprehensions) are whole
  field-value forms. Chain multi-step transforms by binding an intermediate list
  field and reading it back with `field("name")`, exactly like earlier-field
  references.

Evaluation stays pure and deterministic: no I/O or randomness, lists and objects
serialize to canonical JSON, and re-running the build over the same inputs yields
byte-identical output. Iteration is bounded — a `build {}` block may produce at
most 50,000 list elements and nest expressions at most 64 deep — and exceeding a
limit is a build diagnostic. Genuinely complex logic still belongs in a normal Go
build function, which may return slice and struct fields. A build-time list does
not yet feed a `g:for` prerender region; build-time iteration reshapes data into
scalars (and JSON values) consumed via interpolation.

Inside `view {}`, route params can be referenced explicitly with
`{param("slug")}` in text, quoted attributes, and component prop values. SPA
builds reject `param(...)` references that are not declared by the page route.
Inside quoted attributes, escape the inner quotes as `{param(\"slug\")}`.
HTML elements can use first-slice shorthand classes and IDs:

```gwdk
<main #hero .text-4xl .font-bold class="lead">
```

This is normalized to ordinary `id` and `class` attributes during spa
rendering. Duplicate IDs on one element are rejected.
Attributes can use quoted strings, booleans, or first-slice expression values
such as `data-title={post.Title}`.

Old `act name { ... }` and `api name { ... }` forms are rejected with migration
diagnostics.

Current page files must declare `view {}` because every page owns a page `GET`
route. API-only file or route semantics are planned separately.

Inside `view {}`, the current spa markup subset supports
`<form g:post={Submit}>` when `Submit` is a supported action on the same page.
Forms with `g:post` can also declare first-slice partial metadata:

```gwdk
<form g:post={Refresh} g:target="#patients" g:swap="outerHTML">
```

`g:target` must be a spa id selector that references an `id` in the same
direct `view {}` markup subset. Current `g:swap` modes are `innerHTML` and
`outerHTML`. SPA builds emit the partial client runtime only for pages that
use partial form metadata with a fragment-producing action.

Layout files can declare a layout ID and `view {}` body:

```gwdk
layout root

view {
  <slot />
}
```

Component files can also declare string props:

```gwdk
props {
  title string
}
```

Component files can instead declare imported Go struct contracts:

```gwdk
props ui.CounterProps
state ui.CounterState = ui.NewCounterState()
```

The state initializer must be a no-argument function whose return type matches
the declared state type.

Stateful component files can declare a component-local client block:

```gwdk
client {
  func Increment() {
    Count++
  }

  func Add(step int) {
    let next int = Count + step
    Count = next
  }
}

view {
  <button g:on:click={Increment()}>{Count}</button>
  <button g:on:click={Add(Count + 1)}>+ more</button>
  <form g:on:submit.prevent={Save()}></form>
  <input g:on:input.debounce(250ms)={Search()} />
}
```

The implemented client block slice supports `func Name(...) { ... }` handlers
and `async func Name(...) { ... }` handlers with `string`, `int`, `float`, and
`bool` parameters. The older `fn Name(...)` spelling remains accepted. Async
handlers cannot declare return types. `g:on:*` calls can pass typed scalar
expressions as arguments. Handler statements currently support field
increment/decrement, scalar locals such as
`let next int = Count + step`, and assignment from typed scalar expressions
using `+`, `-`, `*`, `/`, `%`, comparisons, `&&`, `||`, `!`, unary `-`, and
parentheses. Local variables are visible only to later statements in the same
client function, lifecycle block, or effect block. Expressions can read nested
fields and indexed values from Go-typed object and slice state, such as
`User.Name` and `Items[0].Name`. Expressions also support Go-ish conditional
values such as `if Open { "open" } else { "closed" }`, plus bounded
`switch`/`match` expressions:

```gwdk
client {
  computed StatusLabel string {
    return switch Status { case "draft": "Draft" case "live": "Live" default: "Unknown" }
  }
}
```

The expression form is inline:
`switch Status { case "draft": "Draft" default: "Unknown" }`. `match` is an
alias for `switch`. A default branch is required, case values must be comparable
with the switch value, and all result branches must return compatible scalar
types.

Async handlers can use the compiler-owned `await fetchJSON[T](urlExpr)` form
only in assignment statements, such as:

```gwdk
client {
  async fn Refresh() {
    Items = await fetchJSON[[]ui.Item]("/api/items")
  }
}
```

`await` is rejected outside async handlers and is not allowed in `let`
initializers. The target must be a state field whose type matches the fetched
type. Async handlers still follow source order for statements in the handler;
computed values and DOM bindings update after state assignments settle.

Component views can also render local async placeholders with a bounded await
block:

```gwdk
{#await fetchJSON[[]ui.Item]("/api/items")}
  <p>Loading</p>
{:then items}
  <ul>
    <li g:for={item in items} g:key={item.ID}>{item.Name}</li>
  </ul>
{:catch err}
  <p>{err.message}</p>
{/await}
```

This markup form is client-island local behavior. It accepts only
`fetchJSON[T](urlExpr)`, not arbitrary promises or value-returning async helper
functions.

Client blocks can declare return-valued helper functions for internal
expression reuse:

```gwdk
client {
  fn Next(value int) int {
    let doubled int = value * 2
    return switch doubled { case 0: 1 default: doubled + 1 }
  }

  fn Add() {
    Count = Next(Count)
  }
}
```

Helpers must declare a scalar return type and end with `return expr`. Before
the final return, helpers may declare scalar locals with
`let name type = expr`; those locals are visible to later helper locals and the
return expression only. Helpers are callable from client expressions such as
assignments, local initializers, handler arguments, and list mutation arguments.
Helpers are not event handlers, so `g:on:click={Next(Count)}` is rejected;
events must call a non-return handler such as `Add()`. Helper call graphs,
including calls from helper local initializers, are validated at compile time
and recursive cycles are rejected. JavaScript-style ternaries, broader built-ins
such as date/time helpers, and recursion remain compile errors today.

Expressions support the first compiler-owned built-ins:

```gwdk
client {
  computed TotalLabel string {
    return string(len(Items))
  }

  computed MatchesQuery bool {
    return contains(lower(Name), lower(Query))
  }

  fn SetTotal() {
    Count = len(Items) + int("1")
  }
}
```

`len(value)` accepts strings and arrays and returns `int`. `lower(value)` and
`upper(value)` accept strings and return strings. `contains(value, query)`
accepts strings and returns `bool`; it is intended for small component-local
filters such as `g:if={contains(lower(item.Name), lower(Query))}` inside
`g:for`. `string(value)` converts scalar values to `string`. `int(value)` and
`float(value)` accept strings or numeric values and return the requested numeric
type.

Generated island JavaScript does not evaluate arbitrary JavaScript source from
`client {}`. It interprets the compiler-owned expression subset above,
including conditionals, switch/match expressions, scalar operators, field/index
reads, helper calls, async fetch assignments, and the listed built-ins.

Client blocks can declare computed values:

```gwdk
client {
  computed Label string {
    if Open {
      return "open"
    }
    return "closed"
  }

  computed Visible bool {
    return Label == "open"
  }
}
```

Computed values are read-only derived values. They can depend on props, state,
and other computed values. The compiler builds a dependency graph, emits
computed values in evaluation order, and rejects dependency cycles. The
generated island runtime recomputes computed values after state changes before
updating text, attributes, classes, styles, and `g:if` bindings.

State updates are batched by the generated island runtime around one event,
lifecycle, effect, or async continuation. The batch order is: apply state
statements, recompute computed values in dependency order, update text and
attribute bindings, update class/style/binding directives, then update
conditional and keyed-list DOM regions.

Event directives support `.prevent`, `.stop`, `.once`, `.capture`,
`.debounce(duration)`, and `.throttle(duration)` modifier chains. Durations
must be positive integer `ms` or `s` values. Debounce and throttle cannot be
combined on the same listener. Element event expressions can read the
compiler-owned DOM event object through `event.value`, `event.checked`,
`event.key`, `event.code`, `event.clientX`, and `event.clientY`.

Client blocks can run controlled lifecycle and effect statements:

```gwdk
client {
  on mount {
    Open = true
  }

  effect when Count {
    Dirty = true
    return {
      Dirty = false
    }
  }

  on destroy {
    Open = false
  }
}
```

Lifecycle and effect statements use the same state-mutation subset as client
functions. `effect when Field` requires a state field dependency and reruns
after that state value changes. Effects can return a cleanup block with
`return { ... }`; cleanup statements run before the effect reruns and before
the island unloads. Effects are guarded by the generated runtime so cycles
cannot run forever. Arbitrary DOM access is not implemented.

Page-scoped stores are declared at page scope and used explicitly inside
component client blocks:

```gwdk
store cart ui.CartState = ui.NewCartState()
```

```gwdk
client {
  use cart
}
```

A `use` may carry the store's Go type — `use cart ui.CartState` — to bind the
store's fields into the component's client scope so they can be referenced
without redeclaring a matching `state`. The type is resolved against the
component's imports. A `clear <store>` statement (valid inside client functions,
mount, destroy, and effect blocks) resets a used store to its build-time init
value and drops any persisted copy; clearing a store the component does not
`use` is a compile error.

Store types and init functions are validated with the same Go type machinery
as component state. Store names are page-local unless a component uses a
qualified GOWDK source alias such as `use stores.cart`. Store state is
serialized into browser-visible enhancement state and must not contain secrets
or trusted authorization, validation, database, or action state. JS and WASM
islands both share a store through the browser store registry and re-render on
cross-island changes; the current contract validates declarations and explicit
uses, but does not make stores global app state.

A page store may opt into browser persistence with a trailing `persist` scope:

```gwdk
store cart ui.CartState = ui.NewCartState() persist "local"
```

The scope must be `"local"` (localStorage) or `"session"` (sessionStorage); any
other value is rejected with `page_store_persist_scope_invalid`. Persisted store
state is keyed by store name, restores over the build-time init value on load,
is discarded when the store's struct shape changes, and warns when a persisted
field name resembles a secret (nested fields included). Declaring the same store
name with a different `local`/`session` scope across pages warns with
`page_store_persist_scope_conflict`.

Client blocks can declare limited DOM refs for safe methods:

```gwdk
client {
  ref searchInput HTMLInputElement

  fn FocusSearch() {
    searchInput.Focus()
  }
}

view {
  <input g:ref={searchInput} />
  <button g:on:click={FocusSearch()}>Focus</button>
}
```

Each declared ref must be bound exactly once with `g:ref`. The supported ref
methods are `Focus`, `Blur`, and `ScrollIntoView`; arbitrary DOM access is not
part of the client language.

Elements inside stateful components can use first-slice conditional rendering:

```gwdk
view {
  <section g:if={Open}>Open content</section>
  <section g:else-if={Loading}>Loading</section>
  <section g:else>Closed</section>
}
```

`g:if` and `g:else-if` must be bool expressions. `g:else` must immediately
follow a sibling `g:if` or `g:else-if` chain and must not have a value. Static
first render may include `hidden` on inactive branches. After island mount, the
generated runtime mounts the active branch and unmounts inactive branches.

Elements inside stateful components can render array state with first-slice
list rendering:

```gwdk
view {
  <li g:for={item in Items} g:key={item.ID}>{item.Name}</li>
  <li g:for={item, i in Items} g:key={item.ID}>{i}: {item.Name}</li>
}
```

`g:for` currently supports `item in Items` and `item, i in Items`, where
`Items` resolves to a Go-typed slice or array field. `g:key` is required and
must be a scalar expression in the loop scope. The first slice renders initial
rows from state, refreshes rows during island render passes, and reuses/reorders
existing row elements by key.

Client handlers can mutate state arrays with compiler-owned list built-ins:

```gwdk
client {
  fn AddItem() {
    append(Items, { ID: "third", Name: "Third", Done: false })
  }

  fn RemoveFirst() {
    remove(Items, 0)
  }

  fn MoveSecondFirst() {
    move(Items, 1, 0)
  }
}
```

`append` requires a state array and an object literal whose fields are checked
against the Go item type. `remove` and `move` require integer indices. These are
GOWDK client-language built-ins, not arbitrary JavaScript function calls.

Elements inside stateful components can toggle classes with bool expressions:

```gwdk
view {
  <button class="tab" class:active={Open}>Toggle</button>
}
```

SPA classes are preserved, and class toggles update through the generated
JavaScript island runtime.

Elements inside stateful components can bind individual style properties:

```gwdk
view {
  <div style="overflow: hidden" style:height.px={PanelHeight}></div>
  <div style:width.%={WidthPercent}></div>
}
```

Style binding expressions must be string or numeric. Unit suffixes append the
unit after evaluation; percent uses `.%`. SPA `style` declarations are
preserved. Raw `style={expr}` attributes remain rejected until broader style
safety rules exist.

Text controls inside stateful components can use first-slice two-way binding:

```gwdk
view {
  <input g:bind:value={Query} />
  <textarea g:bind:value={Query}></textarea>
  <select g:bind:value={SelectedID}>
    <option value="a">A</option>
    <option value="b">B</option>
  </select>
  <input type="radio" name="choice" value="a" g:bind:value={SelectedID} />
  <input type="radio" name="choice" value="b" g:bind:value={SelectedID} />
  <p>{Query}</p>
}
```

`g:bind:value` currently supports `<input>`, `<textarea>`, and `<select>`, and
the target must be a string state field. Numeric state fields can bind to
`<input type="number">`; the generated island runtime parses the control value
back into an integer or float. Radio inputs must declare a spa `value`, and
the bound string state stores the selected radio value.

Checkbox inputs can bind bool state:

```gwdk
view {
  <input type="checkbox" g:bind:checked={Enabled} />
}
```

Event object binding is planned separately.

Safe non-URL attributes inside stateful components can be reactive:

```gwdk
view {
  <button disabled={Saving} aria-expanded={Open}>Save</button>
}
```

Boolean HTML attributes such as `disabled` are toggled as attributes. Scalar
and ARIA attributes are stringified. Reactive URL attributes, `style`, and
event-handler attributes are rejected until dedicated safety rules exist.

## Lexer Tokens

Language tooling currently tokenizes metadata declarations, identifiers, strings, `{`, `}`, `,`, `:`, `?`, `=>`, newlines, text, illegal tokens, and EOF.

Identifiers may include letters, digits, underscores, dots, and hyphens after the first character. Quoted strings support escaped characters and report an error if a newline appears before the closing quote.

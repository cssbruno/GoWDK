# Syntax

The current parser recognizes a small top-level subset.

## Comments

Line comments start with `//`. Empty lines and line comments are ignored by the metadata parser.

## Annotations

Annotations must start at the beginning of the trimmed line:

```gwdk
@page home
@route "/"
@layout root, marketing
@render spa
@guard auth.required, billing.active
@component Hero
@layout root
```

Supported annotations:

- `@page <id>`: required page ID.
- `@route "<path>"`: required route path. Quotes are trimmed.
- `@layout <id>[, <id>...]`: optional page layout IDs, or a layout identity in
  `.layout.gwdk` files.
- `@render spa|action|hybrid|ssr`: optional render mode.
- `@guard <id>[, <id>...]`: optional guard metadata.
- `@component <Name>`: component ID for `.cmp.gwdk` build inputs.

Unknown annotations are rejected. Lines starting with `@` that do not match
`@<identifier>` are rejected as malformed annotations.

Current route validation accepts canonical absolute paths only:

- Routes must start with `/`.
- `/` is the only route that may end with a trailing slash.
- Routes must not contain query strings, fragments, backslashes, whitespace,
  control characters, empty segments, `.`, or `..`.
- Dynamic route params must be whole path segments such as `/blog/{slug}`.
- Route param names use `[A-Za-z_][A-Za-z0-9_]*` and may not repeat in one
  route.
- Duplicate page route patterns are invalid. `/blog/{slug}` and `/blog/{id}`
  are the same pattern.

## Blocks

The parser recognizes these top-level block declarations:

```gwdk
paths {
build {
load {
view {
act subscribe {
api patients {
api {
```

Page files may also declare top-level Go imports before blocks:

```gwdk
import interop "github.com/cssbruno/gowdk/examples/go-interop"
```

Component files can declare Go imports for typed props and state contracts:

```gwdk
import ui "github.com/acme/app/ui"
import "github.com/acme/app/components"
```

Aliased imports use the explicit alias. Unaliased imports use the package name
reported by `go list`, matching ordinary Go import behavior. Relative import
paths are rejected for typed component contracts.

The first build-time Go interop subset supports one imported no-argument
function call in `build {}`:

```gwdk
build {
  => interop.FeaturedCopyForBuild()
}
```

The function must return a JSON-encodable object. Scalar fields are exposed to
`view {}` as string interpolation data.

API blocks may declare one route metadata line in the current subset:

```gwdk
api health {
  GET "/api/health"
}
```

Unsupported top-level block declarations that look like `name ... {` are
rejected until their feature slice is implemented.

`internal/parser.ParseSyntax` returns a typed AST for the current subset:
annotations, blocks, parsed `view {}` markup nodes, literal `paths {}` and
`build {}` records, action statements, API route statements, and source spans.
The manifest parser still preserves raw block body text for compatibility.
SPA builds can expand literal `paths {}` lines such as:

```gwdk
=> { slug: "hello-gowdk" }
```

SPA builds can also render one literal `build {}` line such as:

```gwdk
=> { title: "Hello" }
```

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

`act {}` bodies currently support the first form-input/validation-intent/local
redirect subset:

```gwdk
input := form SignupInput
valid(input)?
-> "/signup?ok=1"
```

Broader statement syntax inside preserved block bodies is still opaque to the parser.

Current page files must declare `view {}` because every page owns a page `GET`
route. API-only file or route semantics are planned separately.

Inside `view {}`, the current spa markup subset supports
`<form g:post={submit}>` when `submit` is a supported action on the same page.
Forms with `g:post` can also declare first-slice partial metadata:

```gwdk
<form g:post={refresh} g:target="#patients" g:swap="outerHTML">
```

`g:target` must be a spa id selector that references an `id` in the same
direct `view {}` markup subset. Current `g:swap` modes are `innerHTML` and
`outerHTML`. SPA builds emit the partial client runtime only for pages that
use partial form metadata with a fragment-producing action.

Layout files can declare a layout ID and `view {}` body:

```gwdk
@layout root

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
  fn Increment() {
    Count++
  }

  fn Add(step int) {
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

The implemented client block slice supports `fn Name(...) { ... }` handlers
with `string`, `int`, `float`, and `bool` parameters. `g:on:*` calls can pass
typed scalar expressions as arguments. Handler statements currently support
field increment/decrement, scalar locals such as
`let next int = Count + step`, and assignment from typed scalar expressions
using `+`, `-`, `*`, `/`, `%`, comparisons, `&&`, `||`, `!`, unary `-`, and
parentheses. Local variables are visible only to later statements in the same
client function, lifecycle block, or effect block. Expressions can read nested
fields and indexed values from Go-typed object and slice state, such as
`User.Name` and `Items[0].Name`. Expressions also support Go-ish conditional
values: `if Open { "open" } else { "closed" }`.

Client blocks can declare return-valued helper functions for internal
expression reuse:

```gwdk
client {
  fn Next(value int) int {
    return value + 1
  }

  fn Add() {
    Count = Next(Count)
  }
}
```

Helpers must declare a scalar return type, contain exactly one `return expr`
statement, and are callable from client expressions such as assignments, local
initializers, handler arguments, and list mutation arguments. Helpers are not
event handlers, so `g:on:click={Next(Count)}` is rejected; events must call a
non-return handler such as `Add()`. Helper call graphs are validated at compile
time and recursive cycles are rejected. Loops, JavaScript-style ternaries,
event object reads, computed helper calls, view binding helper calls, broader
built-ins such as date/time helpers, and recursion remain compile errors today.

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

Client blocks can declare computed values:

```gwdk
client {
  computed Label string {
    return if Open { "open" } else { "closed" }
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

Event directives support `.prevent`, `.stop`, `.once`, `.capture`,
`.debounce(duration)`, and `.throttle(duration)` modifier chains. Durations
must be positive integer `ms` or `s` values. Debounce and throttle cannot be
combined on the same listener.

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
follow a sibling `g:if` or `g:else-if` chain and must not have a value. The
first slice keeps all branches in the DOM and toggles `hidden`; mount/unmount
conditionals are planned separately.

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

Language tooling currently tokenizes annotations, identifiers, strings, `{`, `}`, `,`, `:`, `?`, `=>`, newlines, text, illegal tokens, and EOF.

Identifiers may include letters, digits, underscores, dots, and hyphens after the first character. Quoted strings support escaped characters and report an error if a newline appears before the closing quote.

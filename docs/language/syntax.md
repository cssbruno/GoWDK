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
@render static
@guard auth.required, billing.active
@component Hero
@layout root
```

Supported annotations:

- `@page <id>`: required page ID.
- `@route "<path>"`: required route path. Quotes are trimmed.
- `@layout <id>[, <id>...]`: optional page layout IDs, or a layout identity in
  `.layout.gwdk` files.
- `@render static|action|hybrid|ssr`: optional render mode.
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
Static builds can expand literal `paths {}` lines such as:

```gwdk
=> { slug: "hello-gowdk" }
```

Static builds can also render one literal `build {}` line such as:

```gwdk
=> { title: "Hello" }
```

Inside `view {}`, route params can be referenced explicitly with
`{param("slug")}` in text, quoted attributes, and component prop values. Static
builds reject `param(...)` references that are not declared by the page route.
Inside quoted attributes, escape the inner quotes as `{param(\"slug\")}`.
HTML elements can use first-slice shorthand classes and IDs:

```gwdk
<main #hero .text-4xl .font-bold class="lead">
```

This is normalized to ordinary `id` and `class` attributes during static
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

Inside `view {}`, the current static markup subset supports
`<form g:post={submit}>` when `submit` is a supported action on the same page.
Forms with `g:post` can also declare first-slice partial metadata:

```gwdk
<form g:post={refresh} g:target="#patients" g:swap="outerHTML">
```

`g:target` must be a static id selector that references an `id` in the same
direct `view {}` markup subset. Current `g:swap` modes are `innerHTML` and
`outerHTML`; browser runtime behavior is still planned.

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

## Lexer Tokens

Language tooling currently tokenizes annotations, identifiers, strings, `{`, `}`, `,`, `:`, `?`, `=>`, newlines, text, illegal tokens, and EOF.

Identifiers may include letters, digits, underscores, dots, and hyphens after the first character. Quoted strings support escaped characters and report an error if a newline appears before the closing quote.

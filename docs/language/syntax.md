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
```

Supported annotations:

- `@page <id>`: required page ID.
- `@route "<path>"`: required route path. Quotes are trimmed.
- `@layout <id>[, <id>...]`: optional layout IDs.
- `@render static|action|hybrid|ssr`: optional render mode.
- `@guard <id>[, <id>...]`: optional guard metadata.
- `@component <Name>`: component ID for `.cmp.gwdk` build inputs.

Unknown annotations are ignored today.

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

`paths {}`, `build {}`, and `view {}` body text are currently preserved. Static
builds can expand literal `paths {}` lines such as:

```gwdk
=> { slug: "hello-gowdk" }
```

Static builds can also render one literal `build {}` line such as:

```gwdk
=> { title: "Hello" }
```

`act {}` bodies currently support the first form-input/validation-intent/local
redirect subset:

```gwdk
input := form SignupInput
valid(input)?
-> "/signup?ok=1"
```

Other block bodies are currently opaque to the parser.

Inside `view {}`, the current static markup subset supports
`<form g:post={submit}>` when `submit` is a supported action on the same page.

Component files can also declare string props:

```gwdk
props {
  title string
}
```

## Lexer Tokens

Language tooling currently tokenizes annotations, identifiers, strings, `{`, `}`, `,`, `:`, `?`, `=>`, newlines, text, illegal tokens, and EOF.

Identifiers may include letters, digits, underscores, dots, and hyphens after the first character. Quoted strings support escaped characters and report an error if a newline appears before the closing quote.

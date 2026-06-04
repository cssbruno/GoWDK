# Grammar

This is the grammar accepted by the current metadata parser. It is intentionally line-oriented and incomplete.

```text
file        = line*
line        = blank | comment | annotation | blockDecl | actionDecl | apiDecl | other
blank       = whitespace*
comment     = whitespace* "//" text

annotation  = "@" ident value
blockDecl   = ("paths" | "build" | "load" | "view") whitespace* "{"
actionDecl  = "act" whitespace+ blockName whitespace* "{"
apiDecl     = "api" (whitespace+ blockName)? whitespace* "{"
actionLine  = actionInput | actionValidation | actionRedirect
actionInput = ident whitespace* ":=" whitespace* "form" whitespace+ ident
actionValidation = "valid(" ident ")?"
actionRedirect = "->" whitespace* string

ident       = letterOrUnderscore (letter | digit | "_")*
blockName   = letterOrUnderscore (letter | digit | "_" | "." | "-")*
```

The parser currently scans each trimmed line independently. It records declarations
and captures raw body text for `paths {}`, `build {}`, and `view {}` blocks
until a line that contains only `}`. `gowdk build` parses the first literal
`paths {}` and `build {}` subsets at static-generation time:

```text
literalReturn = "=>" whitespace* "{" literalField ("," literalField)* "}"
literalField  = ident ":" string
```

The parser also validates the first supported `act {}` body subset:

```gwdk
input := form SignupInput
valid(input)?
-> "/signup?ok=1"
```

It does not validate nested block structure, broader statement syntax, full
markup syntax, expressions, or most block body contents.

Future grammar work must define a real AST for annotations, statements, expressions, markup, components, partial fragments, actions, APIs, and source spans.

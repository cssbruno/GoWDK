# Grammar

This is the grammar accepted by the current metadata parser. It is intentionally line-oriented and incomplete.

```text
file        = line*
line        = blank | comment | annotation | importDecl | blockDecl | actionDecl | apiDecl | unsupportedBlock | other
blank       = whitespace*
comment     = whitespace* "//" text

annotation  = "@" ident value
importDecl  = "import" (whitespace+ ident)? whitespace+ string
blockDecl   = ("paths" | "build" | "load" | "view") whitespace* "{"
actionDecl  = "act" whitespace+ blockName whitespace* "{"
apiDecl     = "api" (whitespace+ blockName)? whitespace* "{"
unsupportedBlock = blockName text "{"
actionLine  = actionInput | actionValidation | actionRedirect | actionFragment
actionInput = ident whitespace* ":=" whitespace* "form" whitespace+ ident
actionValidation = "valid(" ident ")?"
actionRedirect = "->" whitespace* string
actionFragment = "fragment" whitespace+ string whitespace* "{" fragmentBody "}"
apiLine     = apiMethod whitespace+ string
apiMethod   = "GET" | "POST" | "PUT" | "PATCH" | "DELETE"

ident       = letterOrUnderscore (letter | digit | "_")*
blockName   = letterOrUnderscore (letter | digit | "_" | "." | "-")*
```

The parser currently scans each trimmed line independently. It records declarations
and captures raw body text for `paths {}`, `build {}`, `load {}`, and `view {}`
blocks until a line that contains only `}`. `act {}` captures and validates the first
form-input/validation/redirect/fragment metadata subset. `api {}` captures and
validates the first method/route metadata subset. `gowdk build` parses the first literal
`paths {}` and `build {}` subsets at static-generation time:

```text
literalReturn = "=>" whitespace* "{" literalField ("," literalField)* "}"
literalField  = ident ":" string
buildCall     = "=>" whitespace* ident "." ident "()"
```

Unknown or malformed annotations fail at parse time. Unsupported top-level block
declarations fail when they have an identifier-like first token and a trailing
`{`. Static builds also accept the first imported `buildCall` subset when the
page declares the referenced import.

The parser also validates the first supported `act {}` body subset:

```gwdk
input := form SignupInput
valid(input)?
-> "/signup?ok=1"
fragment "#target" {
  <p>Updated</p>
}
```

It validates first-slice action fragment targets and captures their body text,
but does not generate fragment handlers yet. It does not validate broader
statement syntax, full markup syntax, expressions, or most block body contents.

The canonical AST, recovery, and semantic-analysis model lives in the language
docs in this directory; implementation remains incremental.

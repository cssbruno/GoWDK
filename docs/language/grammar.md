# Grammar

This is the grammar accepted by the current metadata parser. It is intentionally line-oriented and incomplete.

```text
file        = line*
line        = blank | comment | packageDecl | annotation | importDecl | useDecl | blockDecl | goDecl | actionDecl | apiDecl | unsupportedBlock | other
blank       = whitespace*
comment     = whitespace* "//" text

packageDecl = "package" whitespace+ ident
annotation  = "@" ident value
importDecl  = "import" (whitespace+ ident)? whitespace+ string
useDecl     = "use" whitespace+ ident whitespace+ string
blockDecl   = ("paths" | "build" | "load" | "view" | "style") whitespace* "{"
goDecl  = "go" (whitespace+ blockName)? whitespace* "{"
actionDecl  = "act" whitespace+ ident whitespace+ "POST" whitespace+ string
apiDecl     = "api" whitespace+ ident whitespace+ apiMethod whitespace+ string
unsupportedBlock = blockName text "{"
apiMethod   = "GET" | "POST" | "PUT" | "PATCH" | "DELETE"

ident       = letterOrUnderscore (letter | digit | "_")*
blockName   = letterOrUnderscore (letter | digit | "_" | "." | "-")*
```

The parser currently scans each trimmed line independently. It records
declarations and captures raw body text for `paths {}`, `build {}`, `load {}`,
`go {}`, `go target {}`, `view {}`, and `style {}` blocks until their
closing `}`. CSS braces inside `style {}` and Go braces inside `go {}` do
not close those blocks early. Go block bodies are parsed as Go during semantic
validation. `act` and `api`
declarations name exact exported Go handler symbols; behavior lives in normal
same-package Go handlers. `gowdk build` parses the first literal `paths {}` and
`build {}` subsets at app-generation time:

```text
literalReturn = "=>" whitespace* "{" literalField ("," literalField)* "}"
literalField  = ident ":" string
buildCall     = "=>" whitespace* ident "." ident "()"
```

Unknown or malformed annotations fail at parse time. Unsupported top-level block
declarations fail when they have an identifier-like first token and a trailing
`{`. SPA builds also accept the first imported `buildCall` subset when the
page declares the referenced import.

Default `go {}` blocks can provide no-argument build-data functions for
`build { => LocalFunc() }`. Saved default `go {}` blocks are type-checked with
sibling Go files in the same package during validation. `go ssr {}` can provide
generated SSR load handlers when request-time rendering is enabled. Generated
app source writes default `go {}` and `go ssr {}` blocks under `gowdk_go/`.
Page-level `go client {}` blocks that export `GOWDKMount<PageID>` with
`//go:wasmexport` compile to client-side Go WASM and emit a page mount loader.
Targets such as `addon.contracts` are preserved for lane-specific extraction.
Configured addons that implement
`gowdk.GoBlockConsumer` can validate `go addon.<name> {}` blocks and emit
generated app Go files.

Old `act name { ... }` and `api name { ... }` forms are rejected with migration
diagnostics.

It validates first-slice action fragment targets, captures their body text, and
the generated embedded app can serve the first rendered action fragment response
for partial POSTs. It does not validate broader statement syntax, full markup
syntax, expressions, or most block body contents.

The canonical AST, recovery, and semantic-analysis model lives in the language
docs in this directory; implementation remains incremental.

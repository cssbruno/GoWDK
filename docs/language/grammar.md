# Grammar

This is the grammar accepted by the current metadata parser. It is intentionally line-oriented and incomplete.

Accepted and rejected syntax is pinned by the machine-checked conformance corpus
in [Conformance Corpus](conformance.md), which is the contract source of truth
when this grammar drifts.

```text
file        = line*
line        = blank | comment | packageDecl | metadataDecl | importDecl | useDecl | blockDecl | goDecl | actionDecl | apiDecl | unsupportedBlock | other
blank       = whitespace*
comment     = whitespace* "//" text

packageDecl = "package" whitespace+ ident
metadataDecl = metadataKeyword value
importDecl  = "import" (whitespace+ ident)? whitespace+ string
useDecl     = "use" whitespace+ ident whitespace+ string
blockDecl   = ("paths" | "build" | "server" | "view" | "style") whitespace* "{"
goDecl  = "go" (whitespace+ blockName)? whitespace* "{"
actionDecl  = "act" whitespace+ ident whitespace+ "POST" whitespace+ string
apiDecl     = "api" whitespace+ ident whitespace+ apiMethod whitespace+ string
unsupportedBlock = blockName text "{"
apiMethod   = "GET" | "POST" | "PUT" | "PATCH" | "DELETE"

ident       = letterOrUnderscore (letter | digit | "_")*
blockName   = letterOrUnderscore (letter | digit | "_" | "." | "-")*
```

Audit policy files use the `*.audit.gwdk` suffix and a separate top-level
grammar:

```text
auditFile    = (blank | comment | packageDecl | policyDecl | testDecl)*
policyDecl   = "policy" whitespace+ ident (whitespace+ "extends" whitespace+ ident ("," whitespace* ident)*)? whitespace* "{"
policyLine   = applyLine | requireLine | denyLine | allowLine
applyLine    = ("match" | "apply" whitespace+ "to") whitespace+ string
requireLine  = "require" whitespace+ ("csrf" | "guard" whitespace+ value | "header" whitespace+ string | "max_body" whitespace+ value | "no_secrets_in_bundle") (whitespace+ "as" whitespace+ value)?
denyLine     = "deny" whitespace+ ("public" | "raw_html") (whitespace+ "as" whitespace+ value)?
allowLine    = "allow" whitespace+ "raw_html" whitespace+ value
testDecl     = "test" whitespace+ ident whitespace* "{"
testLine     = "expect" whitespace+ method whitespace+ string (whitespace+ "as" whitespace+ string)? whitespace+ "status" whitespace+ statusCode
             | "expect" whitespace+ "header" whitespace+ string whitespace+ string
value        = ident | string
method       = "GET" | "HEAD" | "POST" | "PUT" | "PATCH" | "DELETE"
statusCode   = digit digit digit
```

The parser currently scans each trimmed line independently. It records
declarations and captures raw body text for `paths {}`, `build {}`, `server {}`,
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

The current `view {}` parser accepts the supported markup subset documented in
[Markup](markup.md). Contract and realtime directives use package-qualified Go
references:

```text
contractRef = ident "." ident
g:command   = "g:command" "=" (string | "{" contractRef "}")
g:query     = "g:query" "=" (string | "{" contractRef "}")
g:subscribe = "g:subscribe" "=" (string | "{" contractRef "}")
```

`g:subscribe` must appear on the same element as `g:query` and must reference a
presentation-event contract.

Unknown or malformed legacy metadatas fail at parse time. Unsupported top-level block
declarations fail when they have an identifier-like first token and a trailing
`{`. SPA builds also accept the first imported `buildCall` subset when the
page declares the referenced import.

Default `go {}` blocks can provide no-argument build-data functions for
`build { => LocalFunc() }`. Saved default `go {}` blocks are type-checked with
sibling Go files in the same package during validation. `go server {}` can provide
generated SSR load handlers when request-time rendering is enabled. Generated
app source writes default `go {}` and `go server {}` blocks under `gowdk_go/`.
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

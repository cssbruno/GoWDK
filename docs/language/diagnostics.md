# Diagnostics

## Current Shape

CLI JSON diagnostics use:

```json
{
  "diagnostics": [
    {
      "file": "examples/pages/home.page.gwdk",
      "code": "missing_page_guard",
      "pos": {"line": 3, "column": 1},
      "range": {
        "start": {"line": 3, "column": 1},
        "end": {"line": 3, "column": 11}
      },
      "severity": "error",
      "message": "home is missing @guard. Add @guard public for an intentionally public page, or add protected guard IDs such as @guard auth.required"
    }
  ]
}
```

Positions and ranges are 1-based; range end columns are exclusive. Lexer,
parser, and compiler validation diagnostics include ranges when the source line
is known. Compiler validation ranges are derived from parser-recorded source
spans for annotations, block declarations, route params, actions, APIs, guards,
layouts, components, and CSS references. Parser errors use the public
`parse_error` code until parser recovery has more specific codes.

The optional `suggestion` field carries a short structured fix hint for common
mistakes such as missing `paths {}` on dynamic spa routes, unknown client or
view fields, missing `g:key`, and malformed `g:for` syntax.

Warnings are non-fatal. The first accessibility warning is `missing_img_alt`,
emitted for literal `<img>` elements without an explicit `alt` attribute.

## Current Code Registry

The diagnostic-code registry, stability policy, naming rules, and
`gowdk explain` behavior live in
[diagnostic-codes.md](../reference/diagnostic-codes.md). The implementation
source of truth is `internal/diagnostics/registry.go`.

Lexer diagnostics can emit `unterminated_string`; parser diagnostics emit
`parse_error` until parser recovery has more specific stable codes.

## Planned Work

Diagnostics still need parser recovery and broader body-level syntax errors.

# Diagnostics

## Current Shape

CLI JSON diagnostics use:

```json
{
  "diagnostics": [
    {
      "file": "examples/pages/home.page.gwdk",
      "code": "missing_view_block",
      "pos": {"line": 1, "column": 1},
      "range": {
        "start": {"line": 1, "column": 1},
        "end": {"line": 1, "column": 6}
      },
      "severity": "error",
      "message": "missing @page",
      "suggestion": "Add an @page declaration before page-level blocks."
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

## Current Code Registry

The diagnostic-code registry, stability policy, naming rules, and
`gowdk explain` behavior live in
[diagnostic-codes.md](../reference/diagnostic-codes.md). The implementation
source of truth is `internal/diagnostics/registry.go`.

Lexer diagnostics can emit `unterminated_string`; parser diagnostics emit
`parse_error` until parser recovery has more specific stable codes.

## Planned Work

Diagnostics still need parser recovery and broader body-level syntax errors.

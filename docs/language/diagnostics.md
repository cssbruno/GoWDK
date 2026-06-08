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

The source of truth for public diagnostic codes is
`internal/diagnostics/registry.go`. The registry records each code, area,
stability level, and short summary. `internal/diagnostics` tests scan non-test
Go source for emitted diagnostic-code literals so newly emitted codes must be
added to the registry.

Current stability levels:

- `stable`: safe for CLI, editor, and docs references during the 0.x line
  unless release notes call out a migration.
- `experimental`: emitted by partial feature slices and may change while the
  feature hardens.
- `addon`: emitted by addon-owned validation or fallback addon diagnostics.

All current diagnostics use severity `error`. Future warning/info diagnostics
must extend the registry and JSON schema docs before shipping.

Code naming uses lower snake case. Prefer names in this form:

- `<surface>_<problem>` for source validation, such as
  `component_field_error`.
- `duplicate_<thing>`, `missing_<thing>`, `unknown_<thing>`,
  `invalid_<thing>`, and `unsupported_<thing>` for common validation classes.
- `<feature>_requires_<dependency>` for missing feature-gate behavior.

Lexer diagnostics can emit `unterminated_string`; parser diagnostics emit
`parse_error` until parser recovery has more specific stable codes.

## Planned Work

Diagnostics still need parser recovery and broader body-level syntax errors.

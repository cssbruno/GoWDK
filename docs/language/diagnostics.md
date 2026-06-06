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

## Current Validation Codes

Compiler validation exposes these public codes when available:

- `missing_ssr_addon`
- `duplicate_page_id`
- `duplicate_component_name`
- `redundant_component_implementation`
- `invalid_go_import`
- `duplicate_go_import_alias`
- `component_contract_error`
- `component_field_error`
- `component_client_error`
- `duplicate_layout_id`
- `unknown_layout_id`
- `malformed_route`
- `duplicate_route_param`
- `duplicate_route`
- `route_method_conflict`
- `missing_view_block`
- `spa_dynamic_route_missing_paths`
- `load_requires_request_render`
- `invalid_css_selection`
- `duplicate_css_selection`

Lexer diagnostics can also emit `unterminated_string`; parser diagnostics emit
`parse_error` in the current line-oriented parser.

## Planned Work

Diagnostics still need parser recovery and broader body-level syntax errors.

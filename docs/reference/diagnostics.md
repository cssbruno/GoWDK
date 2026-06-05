# Diagnostics Reference

`gowdk check --json` prints:

```json
{
  "diagnostics": [
    {
      "file": "examples/basic/dashboard.page.gwdk",
      "code": "missing_ssr_addon",
      "pos": {"line": 3, "column": 1},
      "range": {
        "start": {"line": 3, "column": 1},
        "end": {"line": 3, "column": 12}
      },
      "severity": "error",
      "message": "dashboard: dashboard.page.gwdk uses @render ssr, but the SSR addon is not enabled. Fix: enable ssr.Addon() in gowdk.config.go"
    }
  ]
}
```

Current diagnostic fields:

- `file`: source file path when known.
- `code`: stable diagnostic category when known.
- `pos.line`: 1-based line when known; zero means no exact position is available.
- `pos.column`: 1-based column when known; zero means no exact position is available.
- `range`: optional 1-based source range. End is exclusive.
- `severity`: currently `error`.
- `message`: user-facing diagnostic message.

Current compiler diagnostic codes include:

- `parse_error`
- `component_client_error`
- `component_contract_error`
- `component_field_error`
- `duplicate_component_name`
- `duplicate_layout_id`
- `duplicate_page_id`
- `missing_ssr_addon`
- `redundant_component`
- route diagnostics such as `duplicate_page_route`,
  `ambiguous_dynamic_page_route`, and `route_method_conflict`
- render-mode diagnostics such as `static_dynamic_route_missing_paths` and
  `static_load_block`

Parser diagnostics include line-level ranges. Compiler diagnostics include
ranges when the source span is known. Component `client {}` diagnostics point
to the offending statement line when available, and supported expression
validation failures can narrow the range to the failing expression columns.

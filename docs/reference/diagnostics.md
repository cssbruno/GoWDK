# Diagnostics Reference

`gowdk check --json` prints:

```json
{
  "diagnostics": [
    {
      "file": "examples/ssr/dashboard.page.gwdk",
      "code": "missing_ssr_addon",
      "pos": {"line": 3, "column": 1},
      "range": {
        "start": {"line": 3, "column": 1},
        "end": {"line": 3, "column": 12}
      },
      "severity": "error",
      "message": "dashboard: dashboard.page.gwdk uses @render ssr, but the SSR addon is not enabled. Fix: enable ssr.Addon() in gowdk.config.go",
      "suggestion": "Enable ssr.Addon() in gowdk.config.go or change the page render mode."
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
- `suggestion`: optional editor-facing fix hint for common mistakes.

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
- render-mode diagnostics such as `spa_dynamic_route_missing_paths` and
  `load_requires_request_render`
- spa-build WASM package diagnostics such as `wasm_package_build_error`,
  `wasm_package_entrypoint_error`, and `unsupported_wasm_import`

Parser diagnostics include line-level ranges. Compiler diagnostics include
ranges when the source span is known. Component `client {}` diagnostics point
to the offending statement line when available, and supported expression
validation failures can narrow the range to the failing expression columns.
Common route, render-mode, client-field, view-field, event, and `g:for`
mistakes include structured suggestions when GOWDK can offer a concrete next
step.

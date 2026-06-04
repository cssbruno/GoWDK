# Diagnostics

## Current Shape

CLI JSON diagnostics use:

```json
{
  "diagnostics": [
    {
      "file": "examples/basic/home.page.gwdk",
      "pos": {"line": 1, "column": 1},
      "severity": "error",
      "message": "missing @page"
    }
  ]
}
```

Positions are 1-based. Some compiler validation diagnostics currently have file and message but no exact source range.

## Current Validation Codes

Compiler validation uses these internal codes:

- `missing_ssr_addon`
- `duplicate_page_id`
- `duplicate_component_name`
- `static_dynamic_route_missing_paths`
- `load_requires_request_render`

The public JSON shape does not expose codes yet.

## Planned Work

Diagnostics need stable public codes, source ranges, parse recovery, suggested fixes, duplicate route validation, unknown annotation diagnostics, malformed route diagnostics, and body-level syntax errors.

# Diagnostics Reference

`gowdk check --json` prints:

```json
{
  "diagnostics": [
    {
      "file": "examples/basic/dashboard.page.gwdk",
      "pos": {"line": 0, "column": 0},
      "severity": "error",
      "message": "dashboard: dashboard.page.gwdk uses @render ssr, but the SSR addon is not enabled. Fix: enable ssr.Addon() in gowdk.config.go"
    }
  ]
}
```

Current diagnostic fields:

- `file`: source file path when known.
- `pos.line`: 1-based line when known; zero means no exact position is available.
- `pos.column`: 1-based column when known; zero means no exact position is available.
- `severity`: currently `error`.
- `message`: user-facing diagnostic message.

Stable public diagnostic codes and source ranges are planned.

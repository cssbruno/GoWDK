# Diagnostics Reference

`gowdk check --json` prints:

```json
{
  "diagnostics": [
    {
      "file": "examples/ssr/dashboard.page.gwdk",
      "code": "missing_ssr_addon",
      "pos": {"line": 5, "column": 1},
      "range": {
        "start": {"line": 5, "column": 1},
        "end": {"line": 5, "column": 7}
      },
      "severity": "error",
      "message": "dashboard: dashboard.page.gwdk uses request-time page behavior, but the SSR addon is not enabled. Fix: enable ssr.Addon() in gowdk.config.go",
      "suggestion": "Enable ssr.Addon() in gowdk.config.go or remove request-time page behavior."
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

## Code Registry

The dedicated diagnostic-code reference is
[diagnostic-codes.md](diagnostic-codes.md). It covers the registry source,
stability policy, naming rules, current code areas, and `gowdk explain`.

Use `gowdk explain <diagnostic-code>` for details and next steps:

```sh
gowdk explain missing_ssr_addon
gowdk explain --json spa_dynamic_route_missing_paths
```

Unknown codes return a non-zero exit status with close-code suggestions.

Parser diagnostics include line-level ranges. Compiler diagnostics include
ranges when the source span is known. Component `client {}` diagnostics point
to the offending statement line when available, and supported expression
validation failures can narrow the range to the failing expression columns.
Common route, render-mode, endpoint-comment, client-field, view-field, event,
and `g:for` mistakes include structured suggestions when GOWDK can offer a
concrete next step.

## Span Coverage

Current v0.1-supported language surfaces report source locations as follows:

- Parser syntax errors, including unsupported `paths {}` and `build {}` forms,
  report the offending source line with a line range.
- Route validation uses metadata declaration, block, and route-param spans where
  available, including route declarations, request-time blocks, and dynamic
  route params.
- View and component field validation uses parsed view-node spans for the
  offending directive, field, component call, or interpolation expression.
- Component `client {}` validation reports the offending statement line and
  narrows supported expression failures to expression columns.
- Package validation points at the `.gwdk` package declaration or the nearest
  page/component/layout declaration when the package declaration is missing.
- Build-data validation rejects unsupported statement shapes during parsing and
  reports the offending line; build execution errors that come from external Go
  execution keep their command/error context rather than a precise `.gwdk`
  expression range.

## P0/P1 Constraint Diagnostics

GOWDK keeps the v0.1 language boundary explicit through diagnostics and tests:

- No arbitrary JavaScript as the app contract: unsupported `client {}`
  statements, unknown client values/functions, unsafe reactive URL attributes,
  and unsupported event modifiers fail with `component_client_error` or
  `component_field_error`.
- No external template semantics: familiar external-template blocks such as
  `{#if}`, `{@html}`, snippets, await blocks, and debug tags fail as parse/view
  diagnostics with guidance toward GOWDK-native constructs.
- No generated JavaScript as trusted business logic: frontend templates must not
  declare backend facts with `g:event`; command/query/action behavior remains
  backend-owned and invalid references fail compiler diagnostics before build
  output or generated adapters are accepted.

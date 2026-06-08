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

Current diagnostic codes include:

- `parse_error`
- `component_client_error`
- `component_contract_error`
- `component_field_error`
- `duplicate_component_name`
- `duplicate_layout_id`
- `duplicate_page_id`
- GOWDK source import diagnostics such as `malformed_gowdk_use`,
  `duplicate_gowdk_use_alias`, `unknown_gowdk_use_package`,
  `unknown_gowdk_use_alias`, `unknown_gowdk_component`, and
  `unsupported_gowdk_use_scope`
- package diagnostics such as `missing_package_declaration`,
  `package_must_be_first`, `package_mismatch`, and `go_package_error`.
  `go_package_error` covers sibling Go parse errors, mixed package names, and
  `go/types` type-check errors.
- endpoint diagnostics such as `invalid_backend_handler_name`,
  `invalid_go_endpoint_handler`, `duplicate_go_endpoint_comment`,
  `unsupported_action_method`, `old_action_block_syntax`, and
  `old_api_block_syntax`
- backend binding diagnostics such as `backend_binding_required`
  for strict production action/API handlers that are missing or use an
  unsupported signature. Migration builds can opt into documented HTTP 501
  stubs with `--allow-missing-backend` or `Build.AllowMissingBackend`.
- `missing_ssr_addon`
- `parse_error` for unsupported `paths {}` and `build {}` statement shapes
  outside the current literal-record or no-argument build-data call contract.
- `redundant_component`
- route diagnostics such as `duplicate_page_route`,
  `ambiguous_dynamic_page_route`, and `route_method_conflict`
- render-mode diagnostics such as `spa_dynamic_route_missing_paths` and
  `load_requires_request_render`
- spa-build WASM package diagnostics such as `wasm_package_build_error`,
  `wasm_package_entrypoint_error`, `wasm_package_export_error`, and
  `unsupported_wasm_import`

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
- Route and render-mode validation uses annotation and route-param spans where
  available, including `@render`, route declarations, and dynamic route params.
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
  `{#if}`, `{@html}`, `{@render}`, snippets, await blocks, and debug tags fail
  as parse/view diagnostics with guidance toward GOWDK-native constructs.
- No generated JavaScript as trusted business logic: frontend templates must not
  declare backend facts with `g:event`; command/query/action behavior remains
  backend-owned and invalid references fail compiler diagnostics before build
  output or generated adapters are accepted.

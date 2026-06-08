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

The source of truth for public diagnostic codes is
`internal/diagnostics/registry.go`. The registry records the code, area,
stability level, and summary. `go test ./internal/diagnostics` scans non-test
Go source for emitted diagnostic-code literals and fails when a new emitted
code is missing from the registry.

Use `gowdk explain <diagnostic-code>` for user-facing details and next steps:

```sh
gowdk explain missing_ssr_addon
gowdk explain --json spa_dynamic_route_missing_paths
```

Unknown codes return a non-zero exit status with close-code suggestions.

Stability levels:

- `stable`: safe for CLI, editor, and docs references during the 0.x line
  unless release notes call out a migration.
- `experimental`: emitted by partial feature slices and may change while the
  feature hardens.
- `addon`: emitted by addon-owned validation or fallback addon diagnostics.

Current code areas:

- Parser and lexer: `parse_error`, `package_must_be_first`,
  `old_action_block_syntax`, `old_api_block_syntax`, `malformed_gowdk_use`,
  `unterminated_string`.
- Packages and imports: `missing_package_declaration`, `package_mismatch`,
  `go_package_error`, `invalid_go_import`, `duplicate_go_import_alias`.
- GOWDK source imports: `duplicate_gowdk_use_alias`,
  `unknown_gowdk_use_package`, `unknown_gowdk_use_alias`,
  `unknown_gowdk_component`, `unsupported_gowdk_use_scope`.
- Pages, routes, and render lanes: `duplicate_page_id`, `malformed_route`,
  `duplicate_route_param`, `duplicate_route`, `ambiguous_dynamic_route`,
  `route_method_conflict`, `missing_view_block`, `missing_ssr_addon`,
  `spa_dynamic_route_missing_paths`, `load_requires_request_render`,
  `spa_disabled`, `ssr_disabled`, `missing_page_guard`,
  `public_guard_exclusive`.
- Backend endpoints: `invalid_backend_handler_name`,
  `invalid_go_endpoint_handler`, `go_endpoint_parse_error`,
  `duplicate_go_endpoint_comment`, `unsupported_action_method`,
  `backend_binding_required`.
- Layouts, CSS, and cache: `duplicate_layout_id`, `unknown_layout_id`,
  `invalid_css_selection`, `duplicate_css_selection`,
  `revalidate_requires_cache`, `duplicate_revalidate_policy`.
- Components, stores, and markup: `duplicate_component_name`,
  `redundant_component_implementation`, `component_contract_error`,
  `component_field_error`, `component_client_error`,
  `duplicate_component_emit`, `duplicate_page_store`, `page_store_error`,
  `unknown_component_store`, `view_parse_error`.
- Go blocks and generated app wiring: `invalid_go_block`,
  `go_client_requires_page`, `go_ssr_requires_request_render`,
  `unknown_go_block_target`, `unknown_addon_go_block_target`,
  `unsupported_addon_go_block_target`, `addon_go_block_diagnostic`,
  `generated_app_import_cycle`.
- Partials and fragments: `unsupported_fragment_method`,
  `fragment_dynamic_route`.
- Contracts: `contract_handler_invalid`, `contract_handler_missing`,
  `contract_type_invalid`, `contract_result_invalid`,
  `contract_input_invalid`, `contract_event_name_invalid`,
  `contract_event_category_invalid`, `duplicate_command_owner`,
  `contract_reference_missing`, `contract_reference_invalid`,
  `contract_reference_role_not_allowed`.
- WASM and browser Go: `unsupported_wasm_import`,
  `wasm_package_build_error`, `wasm_package_entrypoint_error`,
  `wasm_package_export_error`, `client_go_block_wasm_source_error`,
  `client_go_block_wasm_build_error`,
  `client_go_block_wasm_entrypoint_error`,
  `client_go_block_wasm_import_error`,
  `client_go_block_wasm_export_error`.

Code naming uses lower snake case. New codes should use predictable names such
as `duplicate_<thing>`, `missing_<thing>`, `unknown_<thing>`,
`invalid_<thing>`, `unsupported_<thing>`, or
`<feature>_requires_<dependency>`.

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
- Route validation uses annotation, block, and route-param spans where
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

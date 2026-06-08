# Diagnostic Codes

Diagnostic codes are the stable handle for compiler, parser, build, and editor
findings. Use them for bug reports, editor integrations, CI policy, and
user-facing help.

## Explain A Code

Use `gowdk explain` when a diagnostic includes a `code`:

```sh
gowdk explain missing_ssr_addon
gowdk explain --json spa_dynamic_route_missing_paths
```

Plain text is for humans. `--json` is for editors and tools.

Unknown codes return a non-zero exit status with close-code suggestions:

```sh
gowdk explain missing_ssr_adon
```

## JSON Shape

`gowdk explain --json <code>` prints:

```json
{
  "code": "missing_ssr_addon",
  "area": "rendering",
  "stability": "stable",
  "summary": "request-time page behavior requires the SSR addon",
  "details": "The source selects request-time page rendering...",
  "nextSteps": [
    "Enable ssr.Addon() in gowdk.config.go when request-time page rendering is intentional."
  ],
  "invalid": "...",
  "fixed": "..."
}
```

Optional fields are omitted when no detailed explanation exists yet.

## Registry Source

The implementation source of truth is `internal/diagnostics/registry.go`.
The registry stores:

- `code`: lower snake case diagnostic ID.
- `area`: broad subsystem such as `parser`, `routing`, `components`, or
  `contracts`.
- `stability`: compatibility level.
- `summary`: short description.

`go test ./internal/diagnostics` scans non-test Go source for emitted
diagnostic-code literals and fails when a new emitted code is missing from the
registry.

## Stability

- `stable`: safe for CLI, editor, and docs references during the 0.x line
  unless release notes call out a migration.
- `experimental`: emitted by partial feature slices and may change while the
  feature hardens.
- `addon`: emitted by addon-owned validation or fallback addon diagnostics.

All current diagnostics use severity `error`. Future warning/info diagnostics
must extend the registry and JSON schema docs before shipping.

## Naming

Code names use lower snake case. Prefer predictable forms:

- `<surface>_<problem>`, such as `component_field_error`.
- `duplicate_<thing>`, `missing_<thing>`, `unknown_<thing>`,
  `invalid_<thing>`, and `unsupported_<thing>`.
- `<feature>_requires_<dependency>`.

Parser diagnostics can still emit broad `parse_error` while parser recovery
gets more specific stable codes.

## Current Areas

- Parser and lexer: `parse_error`, `package_must_be_first`,
  `old_action_block_syntax`, `old_api_block_syntax`, `malformed_gowdk_use`,
  `unterminated_string`.
- Packages and imports: `missing_package_declaration`, `package_mismatch`,
  `go_package_error`, `invalid_go_import`, `duplicate_go_import_alias`.
- GOWDK source imports: `duplicate_gowdk_use_alias`,
  `unknown_gowdk_use_package`, `unknown_gowdk_use_alias`,
  `unknown_gowdk_component`, `unsupported_gowdk_use_scope`.
- Pages, routes, guards, and render lanes: `duplicate_page_id`,
  `malformed_route`, `duplicate_route_param`, `duplicate_route`,
  `ambiguous_dynamic_route`, `route_method_conflict`, `missing_view_block`,
  `missing_ssr_addon`, `spa_dynamic_route_missing_paths`,
  `load_requires_request_render`, `spa_disabled`, `ssr_disabled`,
  `missing_page_guard`, `public_guard_exclusive`, and
  `guard_requires_request_render`.
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

## Adding A Code

When adding or renaming a diagnostic code:

1. Add or update the registry entry in `internal/diagnostics/registry.go`.
2. Add or update `gowdk explain` detail when the next step is not obvious.
3. Update this page when a new area or stability rule appears.
4. Run `go test ./internal/diagnostics`.

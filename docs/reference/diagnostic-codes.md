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
  "severity": "error",
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
- `severity`: default severity, one of `error`, `warning`, or `info`.
- `fix`: optional safe rewrite metadata with a title, description, and named
  rewriter.
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

Most diagnostics use severity `error`. Accessibility diagnostics and
guardless-page diagnostics can use severity `warning`; route-mode notes can use
severity `info`. Warnings are reported by `gowdk check` but do not make the
command fail unless `--warnings-as-errors` is passed.

## Fixes

Some diagnostics have registry-backed fixes. `gowdk fix` applies those safe
single-file rewrites, and LSP code actions use the same metadata:

```sh
gowdk fix --dry-run --code old_action_block_syntax
gowdk fix --code unknown_gowdk_use_alias
```

Old endpoint syntax fixes migrate empty blocks. Blocks that still contain
behavior are refused because moving behavior into Go is not mechanically safe.

## Naming

Code names use lower snake case. Prefer predictable forms:

- `<surface>_<problem>`, such as `component_field_error`.
- `duplicate_<thing>`, `missing_<thing>`, `unknown_<thing>`,
  `invalid_<thing>`, and `unsupported_<thing>`.
- `<feature>_requires_<dependency>`.

Parser diagnostics emit stable codes for common unsupported syntax and keep
`parse_error` as the fallback for unknown parser failures.

## Current Areas

- Parser and lexer: `parse_error`, `package_must_be_first`,
  `malformed_legacy_metadata`, `old_action_block_syntax`,
  `old_api_block_syntax`, `malformed_gowdk_use`,
  `unsupported_literal_record_syntax`, `unsupported_top_level_block`,
  `unsupported_layout_metadata`, `invalid_component_prop`,
  `unsupported_component_prop_type`, `unterminated_string`.
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
  `invalid_go_endpoint_handler`, `malformed_go_endpoint_comment`,
  `go_endpoint_parse_error`, `duplicate_go_endpoint_comment`,
  `unsupported_action_method`, `backend_binding_required`,
  `unsupported_backend_signature`, `unexported_backend_handler`,
  `ambiguous_backend_handler`.
- Layouts, CSS, and cache: `duplicate_layout_id`, `unknown_layout_id`,
  `invalid_css_selection`, `duplicate_css_selection`,
  `revalidate_requires_cache`, `duplicate_revalidate_policy`.
- Components, stores, and markup: `duplicate_component_name`,
  `redundant_component_implementation`, `component_contract_error`,
  `component_field_error`, `component_client_error`,
  `duplicate_component_emit`, `duplicate_page_store`, `page_store_error`,
  `page_store_persist_key_conflict`, `page_store_persist_scope_conflict`,
  `page_store_persist_scope_invalid`, `page_store_persist_secret_field`,
  `unknown_component_store`, `view_parse_error`.
- Accessibility: `missing_img_alt`.
- Go blocks and generated app wiring: `invalid_go_block`,
  `go_client_requires_page`, `go_ssr_requires_request_render`,
  `unknown_go_block_target`, `unknown_addon_go_block_target`,
  `unsupported_addon_go_block_target`, `addon_go_block_diagnostic`,
  `generated_app_import_cycle`.
- Partials and fragments: `unsupported_fragment_method`.
- Contracts: `contract_handler_invalid`, `contract_handler_missing`,
  `contract_type_invalid`, `contract_result_invalid`,
  `contract_input_invalid`, `contract_event_name_invalid`,
  `contract_event_category_invalid`, `duplicate_command_owner`,
  `contract_route_invalid`, `contract_reference_missing`,
  `contract_reference_invalid`,
  `contract_reference_role_not_allowed`.
- Realtime: `missing_realtime_addon`, `realtime_subscription_parse_error`,
  `realtime_subscription_missing`, `realtime_subscription_invalid`,
  `realtime_subscription_role_not_allowed`.
- WASM and browser Go: `unsupported_wasm_import`,
  `wasm_package_build_error`, `wasm_package_entrypoint_error`,
  `wasm_package_export_error`, `client_go_block_wasm_source_error`,
  `client_go_block_wasm_build_error`,
  `client_go_block_wasm_entrypoint_error`,
  `client_go_block_wasm_import_error`,
  `client_go_block_wasm_export_error`.
- Security audit (`gowdk audit`): `audit_action_missing_csrf`,
  `audit_api_missing_csrf`, `audit_api_public_by_omission`,
  `audit_command_missing_csrf`,
  `audit_guardless_endpoint_page`, `audit_bundle_secret`,
  `audit_client_route_unguarded`,
  `audit_headers_missing`, `audit_headers_runtime_missing`,
  `audit_raw_html_sink`, `audit_max_body_exceeds_policy`,
  `audit_public_not_allowed`, `audit_required_guard_missing`,
  `audit_runtime_mismatch`, `audit_test_failed`, `policy_duplicate_name`,
  `policy_extends_cycle`, `policy_unknown_extends`,
  `policy_unknown_selector`, `policy_selector_matched_nothing`. These are
  experimental and emitted by `gowdk audit`, declared audit policies, or the
  optional runtime audit test runner.

## Adding A Code

When adding or renaming a diagnostic code:

1. Add or update the registry entry in `internal/diagnostics/registry.go`,
   including severity and any safe fix metadata.
2. Add or update `gowdk explain` detail when the next step is not obvious.
3. Update this page when a new area or stability rule appears.
4. Run `go test ./internal/diagnostics`.

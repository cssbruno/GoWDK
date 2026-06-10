package diagnostics

// Stability describes how much compatibility a diagnostic code currently has.
type Stability string

const (
	StabilityStable       Stability = "stable"
	StabilityExperimental Stability = "experimental"
	StabilityAddon        Stability = "addon"
)

// Code describes one public diagnostic code emitted by GOWDK tooling.
type Code struct {
	Code      string
	Area      string
	Stability Stability
	Summary   string
}

// Registry is the current inventory of diagnostic codes emitted by the parser,
// compiler, build generator, contract scanner, and language tooling.
var Registry = []Code{
	{Code: "addon_go_block_diagnostic", Area: "go-block", Stability: StabilityAddon, Summary: "addon-provided go block diagnostic without a custom code"},
	{Code: "ambiguous_dynamic_route", Area: "routing", Stability: StabilityStable, Summary: "dynamic page route overlaps another dynamic page route"},
	{Code: "backend_binding_required", Area: "backend", Stability: StabilityStable, Summary: "strict builds require a supported backend handler binding"},
	{Code: "client_go_block_wasm_build_error", Area: "wasm", Stability: StabilityExperimental, Summary: "page go client WASM build failed"},
	{Code: "client_go_block_wasm_entrypoint_error", Area: "wasm", Stability: StabilityExperimental, Summary: "page go client WASM entrypoint is missing or invalid"},
	{Code: "client_go_block_wasm_export_error", Area: "wasm", Stability: StabilityExperimental, Summary: "page go client WASM exports are missing or invalid"},
	{Code: "client_go_block_wasm_import_error", Area: "wasm", Stability: StabilityExperimental, Summary: "page go client WASM imports are invalid"},
	{Code: "client_go_block_wasm_source_error", Area: "wasm", Stability: StabilityExperimental, Summary: "page go client source materialization failed"},
	{Code: "component_client_error", Area: "components", Stability: StabilityStable, Summary: "component client block or local island behavior is invalid"},
	{Code: "component_contract_error", Area: "components", Stability: StabilityStable, Summary: "component Go props or state contract is invalid"},
	{Code: "component_field_error", Area: "components", Stability: StabilityStable, Summary: "component view references an invalid field or directive expression"},
	{Code: "contract_event_category_invalid", Area: "contracts", Stability: StabilityExperimental, Summary: "contract emitted-event category does not match scanned registrations"},
	{Code: "contract_event_name_invalid", Area: "contracts", Stability: StabilityExperimental, Summary: "contract event name is too broad or UI-owned"},
	{Code: "contract_handler_invalid", Area: "contracts", Stability: StabilityExperimental, Summary: "contract handler signature or export shape is invalid"},
	{Code: "contract_handler_missing", Area: "contracts", Stability: StabilityExperimental, Summary: "contract handler was not found in the scanned package"},
	{Code: "contract_input_invalid", Area: "contracts", Stability: StabilityExperimental, Summary: "contract input struct is invalid"},
	{Code: "contract_reference_invalid", Area: "contracts", Stability: StabilityExperimental, Summary: "GOWDK command or query reference could not bind to a valid contract"},
	{Code: "contract_reference_missing", Area: "contracts", Stability: StabilityExperimental, Summary: "GOWDK command or query reference has no matching contract"},
	{Code: "contract_reference_role_not_allowed", Area: "contracts", Stability: StabilityExperimental, Summary: "contract reference targets a registration that is not available to the web role"},
	{Code: "contract_result_invalid", Area: "contracts", Stability: StabilityExperimental, Summary: "contract result type is invalid"},
	{Code: "contract_type_invalid", Area: "contracts", Stability: StabilityExperimental, Summary: "contract type is invalid"},
	{Code: "cyclic_layout_reference", Area: "layouts", Stability: StabilityStable, Summary: "layout @layout inheritance forms a cycle"},
	{Code: "duplicate_command_owner", Area: "contracts", Stability: StabilityExperimental, Summary: "command has more than one owner registration"},
	{Code: "duplicate_component_emit", Area: "components", Stability: StabilityExperimental, Summary: "component declares the same emitted event more than once"},
	{Code: "duplicate_component_name", Area: "components", Stability: StabilityStable, Summary: "component name is declared more than once"},
	{Code: "duplicate_css_selection", Area: "css", Stability: StabilityStable, Summary: "page repeats the same CSS selection"},
	{Code: "duplicate_env_name", Area: "config", Stability: StabilityStable, Summary: "environment variable or secret name is declared more than once"},
	{Code: "duplicate_go_endpoint_comment", Area: "backend", Stability: StabilityStable, Summary: "Go handler has more than one gowdk endpoint comment"},
	{Code: "duplicate_go_import_alias", Area: "packages", Stability: StabilityStable, Summary: "GOWDK source declares the same Go import alias more than once"},
	{Code: "duplicate_gowdk_use_alias", Area: "source-imports", Stability: StabilityStable, Summary: "GOWDK source declares the same use alias more than once"},
	{Code: "duplicate_layout_id", Area: "layouts", Stability: StabilityStable, Summary: "layout identity is declared more than once"},
	{Code: "duplicate_page_id", Area: "pages", Stability: StabilityStable, Summary: "page identity is declared more than once"},
	{Code: "duplicate_page_store", Area: "stores", Stability: StabilityExperimental, Summary: "page declares the same store more than once"},
	{Code: "duplicate_revalidate_policy", Area: "cache", Stability: StabilityStable, Summary: "page combines @revalidate with a cache policy that already declares stale-while-revalidate"},
	{Code: "duplicate_route", Area: "routing", Stability: StabilityStable, Summary: "page route pattern is duplicated"},
	{Code: "duplicate_route_param", Area: "routing", Stability: StabilityStable, Summary: "route declares the same param more than once"},
	{Code: "env_name_required", Area: "config", Stability: StabilityStable, Summary: "environment variable contract entry is missing a name"},
	{Code: "fragment_dynamic_route", Area: "partials", Stability: StabilityExperimental, Summary: "fragment endpoint path is dynamic, which is not supported yet"},
	{Code: "generated_app_import_cycle", Area: "generated-go", Stability: StabilityStable, Summary: "generated app would import itself through user code"},
	{Code: "go_client_requires_page", Area: "go-block", Stability: StabilityExperimental, Summary: "go client block was declared outside a page"},
	{Code: "go_endpoint_parse_error", Area: "backend", Stability: StabilityStable, Summary: "Go endpoint comment scan failed to parse a Go file"},
	{Code: "go_package_error", Area: "packages", Stability: StabilityStable, Summary: "sibling Go package parse, package, or type-check validation failed"},
	{Code: "go_ssr_requires_request_render", Area: "go-block", Stability: StabilityExperimental, Summary: "go ssr block requires request-time page rendering"},
	{Code: "guard_requires_request_render", Area: "pages", Stability: StabilityStable, Summary: "protected page guard metadata requires request-time page rendering"},
	{Code: "invalid_backend_handler_name", Area: "backend", Stability: StabilityStable, Summary: "endpoint handler name is not an exported Go identifier"},
	{Code: "invalid_css_selection", Area: "css", Stability: StabilityStable, Summary: "page CSS selection is invalid"},
	{Code: "invalid_go_block", Area: "go-block", Stability: StabilityExperimental, Summary: "inline Go block does not parse as valid Go"},
	{Code: "invalid_go_endpoint_handler", Area: "backend", Stability: StabilityStable, Summary: "Go endpoint comment is not attached to an exported package-level function"},
	{Code: "invalid_go_import", Area: "packages", Stability: StabilityStable, Summary: "GOWDK source Go import declaration is invalid"},
	{Code: "layout_self_reference", Area: "layouts", Stability: StabilityStable, Summary: "layout lists itself in @layout; a layout cannot inherit from itself"},
	{Code: "layout_slot_count", Area: "layouts", Stability: StabilityStable, Summary: "layout must contain exactly one <slot /> placeholder"},
	{Code: "load_requires_request_render", Area: "rendering", Stability: StabilityStable, Summary: "load block requires request-time page rendering"},
	{Code: "malformed_gowdk_use", Area: "source-imports", Stability: StabilityStable, Summary: "GOWDK use declaration is malformed"},
	{Code: "malformed_route", Area: "routing", Stability: StabilityStable, Summary: "route path violates GOWDK route syntax"},
	{Code: "missing_img_alt", Area: "accessibility", Stability: StabilityStable, Summary: "image element is missing explicit alt text"},
	{Code: "missing_package_declaration", Area: "packages", Stability: StabilityStable, Summary: "GOWDK source is missing a package declaration"},
	{Code: "missing_page_guard", Area: "pages", Stability: StabilityStable, Summary: "page source is missing explicit access guard metadata"},
	{Code: "missing_required_env", Area: "config", Stability: StabilityStable, Summary: "required environment variable is not set"},
	{Code: "missing_required_secret", Area: "config", Stability: StabilityStable, Summary: "required secret environment variable is not set"},
	{Code: "missing_ssr_addon", Area: "rendering", Stability: StabilityStable, Summary: "request-time page behavior requires the SSR addon"},
	{Code: "missing_view_block", Area: "pages", Stability: StabilityStable, Summary: "page source is missing view markup for its GET route"},
	{Code: "old_action_block_syntax", Area: "backend", Stability: StabilityStable, Summary: "source uses removed action block syntax"},
	{Code: "old_api_block_syntax", Area: "backend", Stability: StabilityStable, Summary: "source uses removed API block syntax"},
	{Code: "package_mismatch", Area: "packages", Stability: StabilityStable, Summary: "GOWDK package does not match sibling Go files"},
	{Code: "package_must_be_first", Area: "packages", Stability: StabilityStable, Summary: "package declaration is not the first non-comment declaration"},
	{Code: "page_store_error", Area: "stores", Stability: StabilityExperimental, Summary: "page store type or initializer is invalid"},
	{Code: "parse_error", Area: "parser", Stability: StabilityStable, Summary: "line-oriented parser rejected source without a more specific code"},
	{Code: "public_guard_exclusive", Area: "pages", Stability: StabilityStable, Summary: "@guard public must be the only guard on an intentionally public page"},
	{Code: "redundant_component_implementation", Area: "components", Stability: StabilityStable, Summary: "same component has both GOWDK and generated Go implementations"},
	{Code: "revalidate_requires_cache", Area: "cache", Stability: StabilityStable, Summary: "@revalidate requires an explicit @cache policy"},
	{Code: "route_method_conflict", Area: "routing", Stability: StabilityStable, Summary: "two generated handlers claim the same method and route pattern"},
	{Code: "secret_env_in_vars", Area: "config", Stability: StabilityStable, Summary: "secret-looking environment variable is declared as a normal variable"},
	{Code: "secret_env_name_required", Area: "config", Stability: StabilityStable, Summary: "secret environment contract entry is missing a name"},
	{Code: "spa_disabled", Area: "routing", Stability: StabilityExperimental, Summary: "SPA route binding is disabled for this generated route lane"},
	{Code: "spa_dynamic_route_missing_paths", Area: "routing", Stability: StabilityStable, Summary: "dynamic SPA route is missing paths declarations"},
	{Code: "ssr_disabled", Area: "routing", Stability: StabilityExperimental, Summary: "SSR route binding is disabled for this generated route lane"},
	{Code: "unknown_addon_go_block_target", Area: "go-block", Stability: StabilityExperimental, Summary: "go addon block references an addon that is not enabled"},
	{Code: "unknown_component_store", Area: "stores", Stability: StabilityExperimental, Summary: "component references a page store that does not exist"},
	{Code: "unknown_go_block_target", Area: "go-block", Stability: StabilityExperimental, Summary: "go block target is not recognized"},
	{Code: "unknown_gowdk_component", Area: "source-imports", Stability: StabilityStable, Summary: "qualified component call references an unknown GOWDK component"},
	{Code: "unknown_gowdk_use_alias", Area: "source-imports", Stability: StabilityStable, Summary: "source references a GOWDK use alias that is not declared"},
	{Code: "unknown_gowdk_use_package", Area: "source-imports", Stability: StabilityStable, Summary: "use declaration references a GOWDK package that was not discovered"},
	{Code: "unknown_layout_id", Area: "layouts", Stability: StabilityStable, Summary: "page references a layout that does not exist"},
	{Code: "unsupported_action_method", Area: "backend", Stability: StabilityStable, Summary: "action endpoint uses a method other than POST"},
	{Code: "unsupported_addon_go_block_target", Area: "go-block", Stability: StabilityExperimental, Summary: "enabled addon does not consume the requested go block target"},
	{Code: "unsupported_fragment_method", Area: "partials", Stability: StabilityExperimental, Summary: "fragment endpoint uses an unsupported method"},
	{Code: "unsupported_gowdk_use_scope", Area: "source-imports", Stability: StabilityStable, Summary: "use declaration appears in an unsupported source scope"},
	{Code: "unsupported_markup_directive", Area: "markup", Stability: StabilityStable, Summary: "view markup uses a g: directive outside the GOWDK-owned directive contract"},
	{Code: "unsupported_markup_syntax", Area: "markup", Stability: StabilityStable, Summary: "view markup uses foreign template syntax instead of GOWDK-owned AST nodes and directives"},
	{Code: "unsupported_wasm_import", Area: "wasm", Stability: StabilityExperimental, Summary: "browser WASM package imports an unsupported Go package"},
	{Code: "unterminated_string", Area: "lexer", Stability: StabilityStable, Summary: "lexer found a string literal without a closing quote"},
	{Code: "view_parse_error", Area: "markup", Stability: StabilityStable, Summary: "view markup parser rejected source"},
	{Code: "wasm_package_build_error", Area: "wasm", Stability: StabilityExperimental, Summary: "component WASM package build failed"},
	{Code: "wasm_package_entrypoint_error", Area: "wasm", Stability: StabilityExperimental, Summary: "component WASM package entrypoint is missing or invalid"},
	{Code: "wasm_package_export_error", Area: "wasm", Stability: StabilityExperimental, Summary: "component WASM exports are missing or invalid"},
}

// Lookup returns the registry entry for code.
func Lookup(code string) (Code, bool) {
	for _, entry := range Registry {
		if entry.Code == code {
			return entry, true
		}
	}
	return Code{}, false
}

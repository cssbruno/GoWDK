package diagnostics

// Stability describes how much compatibility a diagnostic code currently has.
type Stability string

const (
	StabilityStable       Stability = "stable"
	StabilityExperimental Stability = "experimental"
	StabilityAddon        Stability = "addon"
)

// Severity describes the default impact of a diagnostic code. Individual
// emitters may still upgrade or downgrade a diagnostic when runtime context
// makes that more precise.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Fix describes one machine-readable rewrite available for a diagnostic code.
type Fix struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Rewriter    string `json:"rewriter,omitempty"`
}

const (
	FixEndpointHeaderFromMessage = "endpoint_header_from_message"
	FixInsertMissingUse          = "insert_missing_use"
)

// Code describes one public diagnostic code emitted by GOWDK tooling.
type Code struct {
	Code      string
	Area      string
	Stability Stability
	Severity  Severity
	Fix       *Fix
	Summary   string
}

var endpointHeaderFix = &Fix{
	Title:       "Replace old endpoint block header",
	Description: "Replace the removed endpoint block form with the current metadata declaration.",
	Rewriter:    FixEndpointHeaderFromMessage,
}

var missingUseFix = &Fix{
	Title:       "Add missing use declaration",
	Description: "Insert a use declaration for the referenced GOWDK component package alias.",
	Rewriter:    FixInsertMissingUse,
}

// Registry is the current inventory of diagnostic codes emitted by the parser,
// compiler, build generator, contract scanner, and language tooling.
var Registry = []Code{
	{Code: "addon_go_block_diagnostic", Area: "go-block", Stability: StabilityAddon, Severity: SeverityError, Summary: "addon-provided go block diagnostic without a custom code"},
	{Code: "ambiguous_backend_handler", Area: "backend", Stability: StabilityStable, Severity: SeverityWarning, Summary: "a backend handler is declared in both same-package Go and an inline go block"},
	{Code: "ambiguous_dynamic_route", Area: "routing", Stability: StabilityStable, Severity: SeverityError, Summary: "dynamic route pattern overlaps another route pattern"},
	{Code: "audit_action_missing_csrf", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "action endpoint does not enforce CSRF as required by the security baseline or policy"},
	{Code: "audit_api_public_by_omission", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "API endpoint inherits no protective guard and policy forbids public-by-omission APIs"},
	{Code: "audit_bundle_secret", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "embedded build output or build-time data carries a secret-shaped value"},
	{Code: "audit_client_route_unguarded", Area: "audit", Stability: StabilityExperimental, Severity: SeverityWarning, Summary: "a client or SPA route declares no guard and is protected only by the generated runtime default-deny gate, which is absent under static export"},
	{Code: "audit_command_missing_csrf", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "command endpoint does not enforce CSRF as required by the security baseline or policy"},
	{Code: "audit_guardless_endpoint_page", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "a page exposing backend endpoints declares no guard"},
	{Code: "audit_headers_missing", Area: "audit", Stability: StabilityExperimental, Severity: SeverityWarning, Summary: "generated app does not declare a security response header required by policy"},
	{Code: "audit_headers_runtime_missing", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "running app did not emit a security response header required by policy"},
	{Code: "audit_max_body_exceeds_policy", Area: "audit", Stability: StabilityExperimental, Severity: SeverityWarning, Summary: "endpoint request body limit is larger than the policy maximum"},
	{Code: "audit_public_not_allowed", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "target is public but policy denies public access"},
	{Code: "audit_raw_html_sink", Area: "audit", Stability: StabilityExperimental, Severity: SeverityWarning, Summary: "view renders raw HTML through g:html without a policy allowlist entry"},
	{Code: "audit_required_guard_missing", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "target does not declare a role or permission guard required by policy"},
	{Code: "audit_runtime_mismatch", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "runtime behavior does not match the declared security posture"},
	{Code: "audit_test_failed", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "an audit integration test expectation did not hold"},
	{Code: "backend_binding_required", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Summary: "strict builds require a supported backend handler binding"},
	{Code: "client_go_block_wasm_build_error", Area: "wasm", Stability: StabilityExperimental, Severity: SeverityError, Summary: "page go client WASM build failed"},
	{Code: "client_go_block_wasm_entrypoint_error", Area: "wasm", Stability: StabilityExperimental, Severity: SeverityError, Summary: "page go client WASM entrypoint is missing or invalid"},
	{Code: "client_go_block_wasm_export_error", Area: "wasm", Stability: StabilityExperimental, Severity: SeverityError, Summary: "page go client WASM exports are missing or invalid"},
	{Code: "client_go_block_wasm_import_error", Area: "wasm", Stability: StabilityExperimental, Severity: SeverityError, Summary: "page go client WASM imports are invalid"},
	{Code: "client_go_block_wasm_source_error", Area: "wasm", Stability: StabilityExperimental, Severity: SeverityError, Summary: "page go client source materialization failed"},
	{Code: "component_client_error", Area: "components", Stability: StabilityStable, Severity: SeverityError, Summary: "component client block or local island behavior is invalid"},
	{Code: "component_contract_error", Area: "components", Stability: StabilityStable, Severity: SeverityError, Summary: "component Go props or state contract is invalid"},
	{Code: "component_field_error", Area: "components", Stability: StabilityStable, Severity: SeverityError, Summary: "component view references an invalid field or directive expression"},
	{Code: "contract_event_category_invalid", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "contract emitted-event category does not match scanned registrations"},
	{Code: "contract_event_name_invalid", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "contract event name is too broad or UI-owned"},
	{Code: "contract_handler_invalid", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "contract handler signature or export shape is invalid"},
	{Code: "contract_handler_missing", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "contract handler was not found in the scanned package"},
	{Code: "contract_input_invalid", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "contract input struct is invalid"},
	{Code: "contract_reference_invalid", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "GOWDK command or query reference could not bind to a valid contract"},
	{Code: "contract_reference_missing", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "GOWDK command or query reference has no matching contract"},
	{Code: "contract_reference_parse_error", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "view contract reference directives could not be parsed"},
	{Code: "contract_reference_role_not_allowed", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "contract reference targets a registration that is not available to the web role"},
	{Code: "contract_result_invalid", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "contract result type is invalid"},
	{Code: "contract_route_invalid", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "contract reference route method or path is invalid"},
	{Code: "contract_type_invalid", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "contract type is invalid"},
	{Code: "cyclic_layout_reference", Area: "layouts", Stability: StabilityStable, Severity: SeverityError, Summary: "layout layout inheritance forms a cycle"},
	{Code: "duplicate_command_owner", Area: "contracts", Stability: StabilityExperimental, Severity: SeverityError, Summary: "command has more than one owner registration"},
	{Code: "duplicate_component_emit", Area: "components", Stability: StabilityExperimental, Severity: SeverityError, Summary: "component declares the same emitted event more than once"},
	{Code: "duplicate_component_name", Area: "components", Stability: StabilityStable, Severity: SeverityError, Summary: "component name is declared more than once"},
	{Code: "duplicate_css_selection", Area: "css", Stability: StabilityStable, Severity: SeverityError, Summary: "page repeats the same CSS selection"},
	{Code: "duplicate_env_name", Area: "config", Stability: StabilityStable, Severity: SeverityError, Summary: "environment variable or secret name is declared more than once"},
	{Code: "duplicate_go_endpoint_comment", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Summary: "Go handler has more than one gowdk endpoint comment"},
	{Code: "duplicate_go_import_alias", Area: "packages", Stability: StabilityStable, Severity: SeverityError, Summary: "GOWDK source declares the same Go import alias more than once"},
	{Code: "duplicate_gowdk_use_alias", Area: "source-imports", Stability: StabilityStable, Severity: SeverityError, Summary: "GOWDK source declares the same use alias more than once"},
	{Code: "duplicate_layout_id", Area: "layouts", Stability: StabilityStable, Severity: SeverityError, Summary: "layout identity is declared more than once"},
	{Code: "duplicate_page_id", Area: "pages", Stability: StabilityStable, Severity: SeverityError, Summary: "page identity is declared more than once"},
	{Code: "duplicate_page_store", Area: "stores", Stability: StabilityExperimental, Severity: SeverityError, Summary: "page declares the same store more than once"},
	{Code: "duplicate_revalidate_policy", Area: "cache", Stability: StabilityStable, Severity: SeverityError, Summary: "page combines revalidate with a cache policy that already declares stale-while-revalidate"},
	{Code: "duplicate_route", Area: "routing", Stability: StabilityStable, Severity: SeverityError, Summary: "page route pattern is duplicated"},
	{Code: "duplicate_route_param", Area: "routing", Stability: StabilityStable, Severity: SeverityError, Summary: "route declares the same param more than once"},
	{Code: "env_name_required", Area: "config", Stability: StabilityStable, Severity: SeverityError, Summary: "environment variable contract entry is missing a name"},
	{Code: "generated_app_import_cycle", Area: "generated-go", Stability: StabilityStable, Severity: SeverityError, Summary: "generated app would import itself through user code"},
	{Code: "go_client_requires_page", Area: "go-block", Stability: StabilityExperimental, Severity: SeverityError, Summary: "go client block was declared outside a page"},
	{Code: "go_endpoint_parse_error", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Summary: "Go endpoint comment scan failed to parse a Go file"},
	{Code: "go_endpoint_read_error", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Summary: "Go endpoint scan failed to read a source directory"},
	{Code: "go_package_error", Area: "packages", Stability: StabilityStable, Severity: SeverityError, Summary: "sibling Go package parse, package, or type-check validation failed"},
	{Code: "go_ssr_requires_request_render", Area: "go-block", Stability: StabilityExperimental, Severity: SeverityError, Summary: "go ssr block requires request-time page rendering"},
	{Code: "guard_requires_request_render", Area: "pages", Stability: StabilityStable, Severity: SeverityError, Summary: "protected page guard metadata requires request-time page rendering"},
	{Code: "invalid_backend_handler_name", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Summary: "endpoint handler name is not an exported Go identifier"},
	{Code: "invalid_component_prop", Area: "parser", Stability: StabilityStable, Severity: SeverityError, Summary: "component props block contains an invalid prop declaration"},
	{Code: "invalid_css_selection", Area: "css", Stability: StabilityStable, Severity: SeverityError, Summary: "page CSS selection is invalid"},
	{Code: "invalid_go_block", Area: "go-block", Stability: StabilityExperimental, Severity: SeverityError, Summary: "inline Go block does not parse as valid Go"},
	{Code: "invalid_go_endpoint_handler", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Summary: "Go endpoint comment is not attached to an exported package-level function"},
	{Code: "invalid_go_import", Area: "packages", Stability: StabilityStable, Severity: SeverityError, Summary: "GOWDK source Go import declaration is invalid"},
	{Code: "layout_self_reference", Area: "layouts", Stability: StabilityStable, Severity: SeverityError, Summary: "layout lists itself in layout; a layout cannot inherit from itself"},
	{Code: "layout_slot_count", Area: "layouts", Stability: StabilityStable, Severity: SeverityError, Summary: "layout must contain exactly one <slot /> placeholder"},
	{Code: "load_requires_request_render", Area: "rendering", Stability: StabilityStable, Severity: SeverityError, Summary: "load block requires request-time page rendering"},
	{Code: "malformed_go_endpoint_comment", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Summary: "Go endpoint comment uses an unsupported //gowdk shape"},
	{Code: "malformed_gowdk_use", Area: "source-imports", Stability: StabilityStable, Severity: SeverityError, Summary: "GOWDK use declaration is malformed"},
	{Code: "malformed_legacy_metadata", Area: "parser", Stability: StabilityStable, Severity: SeverityError, Summary: "source uses legacy @ metadata syntax"},
	{Code: "malformed_route", Area: "routing", Stability: StabilityStable, Severity: SeverityError, Summary: "route path violates GOWDK route syntax"},
	{Code: "missing_img_alt", Area: "accessibility", Stability: StabilityStable, Severity: SeverityWarning, Summary: "image element is missing explicit alt text"},
	{Code: "missing_package_declaration", Area: "packages", Stability: StabilityStable, Severity: SeverityError, Summary: "GOWDK source is missing a package declaration"},
	{Code: "missing_page_guard", Area: "pages", Stability: StabilityStable, Severity: SeverityWarning, Summary: "page declares no guard; warning (route denied 403) or error when it defines act/api/fragment endpoints"},
	{Code: "missing_realtime_addon", Area: "realtime", Stability: StabilityExperimental, Severity: SeverityError, Summary: "realtime subscriptions require the realtime addon"},
	{Code: "missing_required_env", Area: "config", Stability: StabilityStable, Severity: SeverityError, Summary: "required environment variable is not set"},
	{Code: "missing_required_secret", Area: "config", Stability: StabilityStable, Severity: SeverityError, Summary: "required secret environment variable is not set"},
	{Code: "missing_ssr_addon", Area: "rendering", Stability: StabilityStable, Severity: SeverityError, Summary: "request-time page behavior requires the SSR addon"},
	{Code: "missing_view_block", Area: "pages", Stability: StabilityStable, Severity: SeverityError, Summary: "page source is missing view markup for its GET route"},
	{Code: "old_action_block_syntax", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Fix: endpointHeaderFix, Summary: "source uses removed action block syntax"},
	{Code: "old_api_block_syntax", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Fix: endpointHeaderFix, Summary: "source uses removed API block syntax"},
	{Code: "package_mismatch", Area: "packages", Stability: StabilityStable, Severity: SeverityError, Summary: "GOWDK package does not match sibling Go files"},
	{Code: "package_must_be_first", Area: "packages", Stability: StabilityStable, Severity: SeverityError, Summary: "package declaration is not the first non-comment declaration"},
	{Code: "page_store_error", Area: "stores", Stability: StabilityExperimental, Severity: SeverityError, Summary: "page store type or initializer is invalid"},
	{Code: "page_store_persist_key_conflict", Area: "stores", Stability: StabilityExperimental, Severity: SeverityWarning, Summary: "persisted stores share a storage key but have different shapes"},
	{Code: "page_store_persist_scope_conflict", Area: "stores", Stability: StabilityExperimental, Severity: SeverityWarning, Summary: "persisted stores share a storage key but use different scopes"},
	{Code: "page_store_persist_scope_invalid", Area: "stores", Stability: StabilityExperimental, Severity: SeverityError, Summary: "page store persist scope is not \"local\" or \"session\""},
	{Code: "page_store_persist_secret_field", Area: "stores", Stability: StabilityExperimental, Severity: SeverityWarning, Summary: "persisted page store field resembles a secret"},
	{Code: "parse_error", Area: "parser", Stability: StabilityStable, Severity: SeverityError, Summary: "parser rejected source without a more specific code"},
	{Code: "policy_duplicate_name", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "two audit policies declare the same name"},
	{Code: "policy_extends_cycle", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "audit policy extends chain forms a cycle"},
	{Code: "policy_selector_matched_nothing", Area: "audit", Stability: StabilityExperimental, Severity: SeverityWarning, Summary: "audit policy selector matched no routes or endpoints"},
	{Code: "policy_unknown_extends", Area: "audit", Stability: StabilityExperimental, Severity: SeverityError, Summary: "audit policy extends a policy that is not defined"},
	{Code: "policy_unknown_selector", Area: "audit", Stability: StabilityExperimental, Severity: SeverityWarning, Summary: "audit policy uses an unrecognized selector form"},
	{Code: "public_guard_exclusive", Area: "pages", Stability: StabilityStable, Severity: SeverityError, Summary: "guard public must be the only guard on an intentionally public page"},
	{Code: "realtime_subscription_invalid", Area: "realtime", Stability: StabilityExperimental, Severity: SeverityError, Summary: "realtime subscription targets an invalid or non-presentation event contract"},
	{Code: "realtime_subscription_missing", Area: "realtime", Stability: StabilityExperimental, Severity: SeverityError, Summary: "realtime subscription has no matching presentation-event contract"},
	{Code: "realtime_subscription_parse_error", Area: "realtime", Stability: StabilityExperimental, Severity: SeverityError, Summary: "view realtime subscription directives could not be parsed"},
	{Code: "realtime_subscription_role_not_allowed", Area: "realtime", Stability: StabilityExperimental, Severity: SeverityError, Summary: "realtime subscription targets an event registration that is not available to the web role"},
	{Code: "redundant_component_implementation", Area: "components", Stability: StabilityStable, Severity: SeverityError, Summary: "same component has both GOWDK and generated Go implementations"},
	{Code: "revalidate_requires_cache", Area: "cache", Stability: StabilityStable, Severity: SeverityError, Summary: "revalidate requires an explicit cache policy"},
	{Code: "route_method_conflict", Area: "routing", Stability: StabilityStable, Severity: SeverityError, Summary: "two generated handlers claim the same method and route pattern"},
	{Code: "secret_env_in_vars", Area: "config", Stability: StabilityStable, Severity: SeverityError, Summary: "secret-looking environment variable is declared as a normal variable"},
	{Code: "secret_env_name_required", Area: "config", Stability: StabilityStable, Severity: SeverityError, Summary: "secret environment contract entry is missing a name"},
	{Code: "short_secret", Area: "config", Stability: StabilityStable, Severity: SeverityError, Summary: "secret environment variable is shorter than its required minimum length"},
	{Code: "spa_disabled", Area: "routing", Stability: StabilityExperimental, Severity: SeverityInfo, Summary: "SPA route binding is disabled for this generated route lane"},
	{Code: "spa_dynamic_route_missing_paths", Area: "routing", Stability: StabilityStable, Severity: SeverityError, Summary: "dynamic SPA route is missing paths declarations"},
	{Code: "ssr_disabled", Area: "routing", Stability: StabilityExperimental, Severity: SeverityInfo, Summary: "SSR route binding is disabled for this generated route lane"},
	{Code: "unexported_backend_handler", Area: "backend", Stability: StabilityStable, Severity: SeverityWarning, Summary: "a declared backend handler is missing but a same-named unexported Go function exists"},
	{Code: "unknown_addon_go_block_target", Area: "go-block", Stability: StabilityExperimental, Severity: SeverityError, Summary: "go addon block references an addon that is not enabled"},
	{Code: "unknown_component_store", Area: "stores", Stability: StabilityExperimental, Severity: SeverityError, Summary: "component references a page store that does not exist"},
	{Code: "unknown_go_block_target", Area: "go-block", Stability: StabilityExperimental, Severity: SeverityError, Summary: "go block target is not recognized"},
	{Code: "unknown_gowdk_component", Area: "source-imports", Stability: StabilityStable, Severity: SeverityError, Summary: "qualified component call references an unknown GOWDK component"},
	{Code: "unknown_gowdk_use_alias", Area: "source-imports", Stability: StabilityStable, Severity: SeverityError, Fix: missingUseFix, Summary: "source references a GOWDK use alias that is not declared"},
	{Code: "unknown_gowdk_use_package", Area: "source-imports", Stability: StabilityStable, Severity: SeverityError, Summary: "use declaration references a GOWDK package that was not discovered"},
	{Code: "unknown_layout_id", Area: "layouts", Stability: StabilityStable, Severity: SeverityError, Summary: "page references a layout that does not exist"},
	{Code: "unsupported_action_method", Area: "backend", Stability: StabilityStable, Severity: SeverityError, Summary: "action endpoint uses a method other than POST"},
	{Code: "unsupported_addon_go_block_target", Area: "go-block", Stability: StabilityExperimental, Severity: SeverityError, Summary: "enabled addon does not consume the requested go block target"},
	{Code: "unsupported_backend_signature", Area: "backend", Stability: StabilityStable, Severity: SeverityWarning, Summary: "a declared backend handler exists but its Go signature is unsupported"},
	{Code: "unsupported_component_prop_type", Area: "parser", Stability: StabilityStable, Severity: SeverityError, Summary: "component props block uses an unsupported scalar type"},
	{Code: "unsupported_fragment_method", Area: "partials", Stability: StabilityExperimental, Severity: SeverityError, Summary: "fragment endpoint uses an unsupported method"},
	{Code: "unsupported_gowdk_use_scope", Area: "source-imports", Stability: StabilityStable, Severity: SeverityError, Summary: "use declaration appears in an unsupported source scope"},
	{Code: "unsupported_layout_metadata", Area: "parser", Stability: StabilityStable, Severity: SeverityError, Summary: "layout source declares metadata that layouts do not support"},
	{Code: "unsupported_literal_record_syntax", Area: "parser", Stability: StabilityStable, Severity: SeverityError, Summary: "paths or build literal record syntax is unsupported"},
	{Code: "unsupported_markup_directive", Area: "markup", Stability: StabilityStable, Severity: SeverityError, Summary: "view markup uses a g: directive outside the GOWDK-owned directive contract"},
	{Code: "unsupported_markup_syntax", Area: "markup", Stability: StabilityStable, Severity: SeverityError, Summary: "view markup uses foreign template syntax instead of GOWDK-owned AST nodes and directives"},
	{Code: "unsupported_top_level_block", Area: "parser", Stability: StabilityStable, Severity: SeverityError, Summary: "source declares a top-level block that is not supported in that file kind"},
	{Code: "unsupported_wasm_import", Area: "wasm", Stability: StabilityExperimental, Severity: SeverityError, Summary: "browser WASM package imports an unsupported Go package"},
	{Code: "unterminated_string", Area: "lexer", Stability: StabilityStable, Severity: SeverityError, Summary: "lexer found a string literal without a closing quote"},
	{Code: "view_parse_error", Area: "markup", Stability: StabilityStable, Severity: SeverityError, Summary: "view markup parser rejected source"},
	{Code: "wasm_package_build_error", Area: "wasm", Stability: StabilityExperimental, Severity: SeverityError, Summary: "component WASM package build failed"},
	{Code: "wasm_package_entrypoint_error", Area: "wasm", Stability: StabilityExperimental, Severity: SeverityError, Summary: "component WASM package entrypoint is missing or invalid"},
	{Code: "wasm_package_export_error", Area: "wasm", Stability: StabilityExperimental, Severity: SeverityError, Summary: "component WASM exports are missing or invalid"},
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

// DefaultSeverity returns the registry severity for code.
func DefaultSeverity(code string) (Severity, bool) {
	entry, ok := Lookup(code)
	if !ok {
		return "", false
	}
	return entry.Severity, true
}

// FixFor returns the registry fix metadata for code.
func FixFor(code string) (Fix, bool) {
	entry, ok := Lookup(code)
	if !ok || entry.Fix == nil {
		return Fix{}, false
	}
	return *entry.Fix, true
}

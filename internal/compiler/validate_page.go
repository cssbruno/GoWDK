package compiler

import (
	"fmt"
	"os"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/runtime/auth"
)

func ValidatePage(config gowdk.Config, page gwdkir.Page) []ValidationError {
	mode := page.RenderMode(config.Render.DefaultMode())
	var diagnostics []ValidationError
	pageRoute, pageRouteIssues := parseRoute(page.Route)
	diagnostics = append(diagnostics, routeDiagnostics(page, "page route", pageRouteIssues, page.Spans.Route, page.Spans.RouteParams)...)
	diagnostics = append(diagnostics, validatePageGuards(page)...)
	diagnostics = append(diagnostics, validateProtectedPageGuardRender(page, mode)...)
	diagnostics = append(diagnostics, validatePageStores(page)...)
	diagnostics = append(diagnostics, validatePageCachePolicy(page)...)
	for _, action := range page.Blocks.Actions {
		if !isExportedHandlerName(action.Name) {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "invalid_backend_handler_name",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    action.Span,
				Message: fmt.Sprintf("%s action handler %q must be an exported Go identifier", page.ID, action.Name),
			})
		}
		method := strings.ToUpper(strings.TrimSpace(action.Method))
		if method == "" {
			method = "POST"
		}
		if method != "POST" {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "unsupported_action_method",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    action.Span,
				Message: fmt.Sprintf("%s action %s uses unsupported method %s; actions currently require POST", page.ID, action.Name, method),
			})
		}
		route := action.Route
		if route == "" {
			route = page.Route
		}
		actionRoute, issues := parseRoute(route)
		diagnostics = append(diagnostics, routeDiagnostics(page, fmt.Sprintf("action %s endpoint path", action.Name), issues, firstSpan(action.RouteSpan, action.Span, page.Spans.Route), action.RouteParams)...)
		if len(issues) == 0 && actionRoute.RestParam != "" {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "malformed_route",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    firstSpan(action.RouteSpan, action.Span, page.Spans.Route),
				Message: fmt.Sprintf("%s action %s endpoint path %q uses rest route parameter {%s...}; rest parameters are only supported on page routes", page.ID, action.Name, route, actionRoute.RestParam),
			})
		}
	}
	for _, api := range page.Blocks.APIs {
		if !isExportedHandlerName(api.Name) {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "invalid_backend_handler_name",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    api.Span,
				Message: fmt.Sprintf("%s API handler %q must be an exported Go identifier", page.ID, api.Name),
			})
		}
		label := "api endpoint path"
		if api.Name != "" {
			label = fmt.Sprintf("api %s endpoint path", api.Name)
		}
		if api.Route == "" {
			// An API without an explicit path inherits the page route, so a
			// rest page route flows into the endpoint and must be rejected
			// the same way an explicit rest endpoint path is.
			if len(pageRouteIssues) == 0 && pageRoute.RestParam != "" {
				diagnostics = append(diagnostics, ValidationError{
					Code:    "malformed_route",
					PageID:  page.ID,
					Source:  page.Source,
					Span:    firstSpan(api.Span, page.Spans.Route),
					Message: fmt.Sprintf("%s %s inherits page route %q which uses rest route parameter {%s...}; rest parameters are only supported on page routes", page.ID, label, page.Route, pageRoute.RestParam),
				})
			}
			continue
		}
		apiRoute, issues := parseRoute(api.Route)
		diagnostics = append(diagnostics, routeDiagnostics(page, label, issues, api.RouteSpan, api.RouteParams)...)
		if len(issues) == 0 && apiRoute.RestParam != "" {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "malformed_route",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    firstSpan(api.RouteSpan, api.Span, page.Spans.Route),
				Message: fmt.Sprintf("%s %s %q uses rest route parameter {%s...}; rest parameters are only supported on page routes", page.ID, label, api.Route, apiRoute.RestParam),
			})
		}
	}
	for _, fragment := range page.Blocks.Fragments {
		if !isExportedHandlerName(fragment.Name) {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "invalid_backend_handler_name",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    fragment.Span,
				Message: fmt.Sprintf("%s fragment handler %q must be an exported Go identifier", page.ID, fragment.Name),
			})
		}
		method := strings.ToUpper(strings.TrimSpace(fragment.Method))
		if method == "" {
			method = "GET"
		}
		if method != "GET" {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "unsupported_fragment_method",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    fragment.Span,
				Message: fmt.Sprintf("%s fragment %s uses unsupported method %s; fragments currently require GET", page.ID, fragment.Name, method),
			})
		}
		fragmentRoute, issues := parseRoute(fragment.Route)
		diagnostics = append(diagnostics, routeDiagnostics(page, fmt.Sprintf("fragment %s endpoint path", fragment.Name), issues, fragment.RouteSpan, fragment.RouteParams)...)
		if len(issues) == 0 && len(fragmentRoute.Params) > 0 {
			diagnostics = append(diagnostics, ValidationError{
				Code:   "fragment_dynamic_route",
				PageID: page.ID,
				Source: page.Source,
				Span:   firstNamedSpan(fragment.RouteParams, fragment.RouteSpan),
				Message: fmt.Sprintf(
					"%s fragment %s endpoint path %q must be concrete; dynamic fragment routes are not supported yet",
					page.ID,
					fragment.Name,
					fragment.Route,
				),
			})
		}
	}

	if requiresSSRFeature(mode, page) && !config.HasFeature(gowdk.FeatureSSR) {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "missing_ssr_addon",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Blocks.Spans.Load, firstGoBlockSpan(page, "ssr"), page.Spans.Page),
			Message: fmt.Sprintf(
				"%s.page.gwdk uses request-time page behavior, but the SSR addon is not enabled. Fix: enable ssr.Addon() in gowdk.config.go",
				page.ID,
			),
		})
	}
	if !page.Blocks.View {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "missing_view_block",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Spans.Page, page.Spans.Route),
			Message: fmt.Sprintf(
				"%s declares a page route but is missing view {}. Current pages must render HTML for their GET route",
				page.ID,
			),
		})
	}

	var params []string
	if len(pageRouteIssues) == 0 {
		params = pageRoute.Params
	}
	if isBuildTimeRoute(mode, page) && len(pageRouteIssues) == 0 && pageRoute.RestParam != "" {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "malformed_route",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstNamedSpan(page.Spans.RouteParams, page.Spans.Route),
			Message: fmt.Sprintf(
				"%s declares rest route parameter {%s...}, but render mode is %s; rest parameters match request paths at request time and require SSR rendering. Fix: declare request-time page behavior with load {} or go ssr {}",
				page.ID,
				pageRoute.RestParam,
				mode,
			),
		})
	} else if isBuildTimeRoute(mode, page) && len(params) > 0 && !page.Blocks.Paths {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "spa_dynamic_route_missing_paths",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstNamedSpan(page.Spans.RouteParams, page.Spans.Route),
			Message: fmt.Sprintf(
				"%s has dynamic route params: {%s}, but render mode is %s and no paths block exists. Fix: add paths { ... } or declare request-time page behavior with load {} or go ssr {}",
				page.ID,
				strings.Join(params, ", "),
				mode,
			),
		})
	}

	if page.Blocks.Load && mode != gowdk.SSR && mode != gowdk.Hybrid {
		diagnostics = append(diagnostics, ValidationError{
			Code:   "load_requires_request_render",
			PageID: page.ID,
			Source: page.Source,
			Span:   firstSpan(page.Blocks.Spans.Load, page.Spans.Render, page.Spans.Page),
			Message: fmt.Sprintf(
				"%s declares load {}, but load runs at request time and requires the SSR addon",
				page.ID,
			),
		})
	}
	diagnostics = append(diagnostics, validatePageCSS(page)...)

	return diagnostics
}

func validatePageGuards(page gwdkir.Page) []ValidationError {
	if !validateSourceBackedPageGuards(page) {
		return nil
	}
	if len(page.Guards) == 0 {
		// A guardless page route is denied (403) at request time, so warning is
		// enough. But act/api/fragment endpoints derived from the page inherit
		// the page guards: with none declared they would be publicly callable
		// even though the page route is denied. That contradicts the
		// not-public-by-default contract, so it is a hard error.
		if pageDeclaresBackendEndpoints(page) {
			return []ValidationError{{
				Code:     "missing_page_guard",
				PageID:   page.ID,
				Source:   page.Source,
				Span:     firstSpan(page.Spans.Page, page.Spans.Route),
				Severity: SeverityError,
				Message: fmt.Sprintf(
					"%s declares no guard but defines act/api/fragment endpoints, which would be publicly callable. Add guard public to make them public, or a protective guard such as guard auth.required",
					page.ID,
				),
			}}
		}
		return []ValidationError{{
			Code:     "missing_page_guard",
			PageID:   page.ID,
			Source:   page.Source,
			Span:     firstSpan(page.Spans.Page, page.Spans.Route),
			Severity: SeverityWarning,
			Message: fmt.Sprintf(
				"%s declares no guard; its route is denied (403) at request time. Add guard public to serve it, or a protective guard such as guard auth.required",
				page.ID,
			),
		}}
	}

	public := false
	for _, guard := range page.Guards {
		if auth.IsPublicGuard(guard) {
			public = true
			break
		}
	}
	if public && len(page.Guards) > 1 {
		return []ValidationError{{
			Code:    "public_guard_exclusive",
			PageID:  page.ID,
			Source:  page.Source,
			Span:    firstNamedSpan(page.Spans.Guard, firstSpan(page.Spans.Page, page.Spans.Route)),
			Message: fmt.Sprintf("%s declares guard public with other guards; public must be the only guard ID", page.ID),
		}}
	}
	return nil
}

func validateProtectedPageGuardRender(page gwdkir.Page, mode gowdk.RenderMode) []ValidationError {
	if !validateSourceBackedPageGuards(page) || !isBuildTimeRoute(mode, page) || !hasProtectedPageGuard(page) {
		return nil
	}
	return []ValidationError{{
		Code:    "guard_requires_request_render",
		PageID:  page.ID,
		Source:  page.Source,
		Span:    firstNamedSpan(page.Spans.Guard, firstSpan(page.Spans.Page, page.Spans.Route)),
		Message: fmt.Sprintf("%s declares protected guard IDs on a build-time page route. Add load {} or go ssr {} with the SSR addon so frontend page access is request-time guarded, or use guard public for an intentionally public page", page.ID),
	}}
}

func validateSourceBackedPageGuards(page gwdkir.Page) bool {
	if strings.TrimSpace(page.Source) == "" {
		return false
	}
	if _, err := os.Stat(page.Source); err != nil {
		return false
	}
	return true
}

func pageDeclaresBackendEndpoints(page gwdkir.Page) bool {
	return len(page.Blocks.Actions) > 0 || len(page.Blocks.APIs) > 0 || len(page.Blocks.Fragments) > 0
}

func hasProtectedPageGuard(page gwdkir.Page) bool {
	for _, guard := range page.Guards {
		if !auth.IsPublicGuard(guard) {
			return true
		}
	}
	return false
}

func firstGoBlockSpan(page gwdkir.Page, target string) source.SourceSpan {
	for _, block := range page.Blocks.GoBlocks {
		if block.Target == target {
			return block.Span
		}
	}
	return source.SourceSpan{}
}

func requiresSSRFeature(mode gowdk.RenderMode, page gwdkir.Page) bool {
	return mode == gowdk.SSR || mode == gowdk.Hybrid
}

func isBuildTimeRoute(mode gowdk.RenderMode, page gwdkir.Page) bool {
	switch mode {
	case gowdk.SPA, gowdk.Action:
		return true
	default:
		return false
	}
}

func validatePageCachePolicy(page gwdkir.Page) []ValidationError {
	if page.Revalidate == "" {
		return nil
	}
	if strings.TrimSpace(page.Cache) == "" {
		return []ValidationError{{
			Code:    "revalidate_requires_cache",
			PageID:  page.ID,
			Source:  page.Source,
			Span:    firstSpan(page.Spans.Revalidate, page.Spans.Page),
			Message: fmt.Sprintf("%s declares revalidate, but revalidation requires an explicit cache policy", page.ID),
		}}
	}
	if strings.Contains(strings.ToLower(page.Cache), "stale-while-revalidate") {
		return []ValidationError{{
			Code:    "duplicate_revalidate_policy",
			PageID:  page.ID,
			Source:  page.Source,
			Span:    firstSpan(page.Spans.Revalidate, page.Spans.Cache, page.Spans.Page),
			Message: fmt.Sprintf("%s declares revalidate and a cache policy that already contains stale-while-revalidate", page.ID),
		}}
	}
	return nil
}

func validatePageStores(page gwdkir.Page) []ValidationError {
	seen := map[string]gwdkir.Store{}
	var diagnostics []ValidationError
	for _, store := range page.Stores {
		if first, exists := seen[store.Name]; exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "duplicate_page_store",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    store.Span,
				Related: relatedSpan(page.Source, first.Span, fmt.Sprintf("store %q first declared here", store.Name)),
				Message: fmt.Sprintf(
					"%s declares duplicate store %q; first declared at line %d and duplicated at line %d",
					page.ID,
					store.Name,
					first.Span.Start.Line,
					store.Span.Start.Line,
				),
			})
			continue
		}
		seen[store.Name] = store
		resolved, err := gotypes.ResolveStruct(page.Imports, store.Type)
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "page_store_error",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    firstSpan(store.Type.Span, store.Span, page.Spans.Page),
				Message: fmt.Sprintf("page %s store %q type is invalid: %v", page.ID, store.Name, err),
			})
			continue
		}
		if err := gotypes.ValidateStateInit(page.Imports, gwdkir.StateContract{Type: store.Type, Init: store.Init, Span: store.Span}); err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "page_store_error",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    firstSpan(store.Init.Span, store.Span, page.Spans.Page),
				Message: fmt.Sprintf("page %s store %q init is invalid: %v", page.ID, store.Name, err),
			})
		}
		diagnostics = append(diagnostics, validateStorePersist(page, store, resolved)...)
	}
	return diagnostics
}

// validateStorePersist checks the optional `persist "<scope>"` modifier on a
// page store: the scope must be a known browser storage backend, and persisting
// a field whose name resembles a secret earns a warning because browser storage
// is readable by any script on the origin.
func validateStorePersist(page gwdkir.Page, store gwdkir.Store, resolved gotypes.Struct) []ValidationError {
	if store.Persist == "" {
		return nil
	}
	if store.Persist != "local" && store.Persist != "session" {
		return []ValidationError{{
			Code:    "page_store_persist_scope_invalid",
			PageID:  page.ID,
			Source:  page.Source,
			Span:    firstSpan(store.Span, page.Spans.Page),
			Message: fmt.Sprintf("page %s store %q persist scope %q is invalid; use \"local\" or \"session\"", page.ID, store.Name, store.Persist),
		}}
	}
	var diagnostics []ValidationError
	for _, field := range resolved.Fields {
		if !looksLikeSecretFieldName(field.Name) {
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:     "page_store_persist_secret_field",
			PageID:   page.ID,
			Source:   page.Source,
			Span:     firstSpan(store.Span, page.Spans.Page),
			Severity: SeverityWarning,
			Message:  fmt.Sprintf("page %s store %q persists field %q, which resembles a secret; %s browser storage is readable by any script on this origin", page.ID, store.Name, field.Name, store.Persist),
		})
	}
	return diagnostics
}

// looksLikeSecretFieldName flags field names that commonly hold credentials or
// trusted authorization state, which the store contract already forbids from
// browser-visible state and which persistence would write to disk.
func looksLikeSecretFieldName(name string) bool {
	lower := strings.ToLower(name)
	needles := []string{"password", "passwd", "secret", "token", "apikey", "api_key", "auth", "credential", "private_key", "privatekey", "ssn"}
	for _, needle := range needles {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}

func validatePageCSS(page gwdkir.Page) []ValidationError {
	if len(page.CSS) == 0 {
		return nil
	}
	if len(page.CSS) > 1 && containsCSSReference(page.CSS, "none") {
		return []ValidationError{{
			Code:   "invalid_css_selection",
			PageID: page.ID,
			Source: page.Source,
			Span:   spanForName(page.Spans.CSS, "none", page.Spans.Page),
			Message: fmt.Sprintf(
				"%s uses css none with other CSS inputs. Fix: use css none by itself or remove none",
				page.ID,
			),
		}}
	}

	seen := map[string]bool{}
	var diagnostics []ValidationError
	for _, name := range page.CSS {
		if !isCSSReferenceName(name) {
			diagnostics = append(diagnostics, ValidationError{
				Code:   "invalid_css_selection",
				PageID: page.ID,
				Source: page.Source,
				Span:   spanForName(page.Spans.CSS, name, page.Spans.Page),
				Message: fmt.Sprintf(
					"%s uses invalid css input %q. CSS inputs must be identifiers such as default, page, forms, or blog.post",
					page.ID,
					name,
				),
			})
			continue
		}
		if seen[name] {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "duplicate_css_selection",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    spanForName(page.Spans.CSS, name, page.Spans.Page),
				Message: fmt.Sprintf("%s repeats css input %q", page.ID, name),
			})
			continue
		}
		seen[name] = true
	}
	return diagnostics
}

func isCSSReferenceName(name string) bool {
	if name == "" {
		return false
	}
	for index := 0; index < len(name); index++ {
		char := name[index]
		if index == 0 {
			if char != '_' && (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') {
				return false
			}
			continue
		}
		if char == '_' || char == '.' || char == '-' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func containsCSSReference(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isExportedHandlerName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			return r >= 'A' && r <= 'Z'
		}
	}
	return false
}

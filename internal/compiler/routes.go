package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func validateUniquePageRoutes(config gowdk.Config, pages []gwdkir.Page) []ValidationError {
	seen := map[string]gwdkir.Page{}
	var diagnostics []ValidationError
	for _, page := range pages {
		for _, route := range localizedPageRoutes(config.I18N, page) {
			info, issues := parseRoute(route)
			if len(issues) > 0 {
				continue
			}
			first, exists := seen[info.Pattern]
			if !exists {
				seen[info.Pattern] = page
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:   "duplicate_route",
				PageID: page.ID,
				Source: page.Source,
				Span:   page.Spans.Route,
				Related: relatedSpan(
					first.Source,
					first.Spans.Route,
					fmt.Sprintf("route %q first declared here", route),
				),
				Message: duplicateRouteMessage(
					route,
					first.ID,
					first.Source,
					page.ID,
					page.Source,
				),
			})
		}
	}
	return diagnostics
}

// relatedSpan returns a single-element related-location slice for a conflict
// diagnostic's earlier declaration, or nil when the earlier span is unset so a
// missing location is never reported as a bogus 1:1 position.
func relatedSpan(src string, span source.SourceSpan, message string) []source.RelatedSpan {
	if !hasSpan(span) {
		return nil
	}
	return []source.RelatedSpan{{Source: src, Span: span, Message: message}}
}

func duplicateRouteMessage(route, firstID, firstSource, duplicateID, duplicateSource string) string {
	message := fmt.Sprintf("duplicate page route %q", route)
	if firstID != "" && duplicateID != "" {
		message = fmt.Sprintf("%s; first declared by page %s and duplicated by page %s", message, firstID, duplicateID)
	}
	if firstSource != "" && duplicateSource != "" {
		return fmt.Sprintf("%s (%s and %s)", message, firstSource, duplicateSource)
	}
	return message
}

func validateAmbiguousDynamicPageRoutes(config gowdk.Config, pages []gwdkir.Page, endpoints []gwdkir.Endpoint, sourceMap gwdkir.SourceMap, refs []gwdkir.ContractReference) []ValidationError {
	var registered []routeRegistration
	var diagnostics []ValidationError
	for _, current := range routeRegistrations(config, pages, endpoints, sourceMap, refs) {
		for _, previous := range registered {
			if current.Pattern == previous.Pattern {
				// Exact duplicates are reported by the duplicate-route and
				// route-method-conflict checks.
				continue
			}
			if current.Kind == "page" && previous.Kind == "page" {
				// Dynamic routes are compared against other dynamic routes.
				// Rest routes match one or more trailing segments, so they
				// are also compared against concrete routes that share their
				// fixed prefix.
				bothDynamic := patternIsDynamic(current.Pattern) && patternIsDynamic(previous.Pattern)
				restInvolved := patternHasRest(current.Pattern) || patternHasRest(previous.Pattern)
				if !bothDynamic && !restInvolved {
					continue
				}
			} else {
				// Same-method endpoints share one generated request-time
				// namespace with pages. Any dynamic overlap can shadow the
				// concrete handler that should own a request path.
				if current.Method != previous.Method {
					continue
				}
				if !patternIsDynamic(current.Pattern) && !patternIsDynamic(previous.Pattern) {
					continue
				}
			}
			if !routePatternsOverlap(current.Pattern, previous.Pattern) {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:    "ambiguous_dynamic_route",
				PageID:  current.PageID,
				Source:  current.Source,
				Span:    current.Span,
				Message: ambiguousDynamicRouteMessage(current, previous),
			})
		}
		registered = append(registered, current)
	}
	return diagnostics
}

func ambiguousDynamicRouteMessage(current, previous routeRegistration) string {
	message := fmt.Sprintf("ambiguous dynamic route %q overlaps %q", current.Route, previous.Route)
	if current.Owner != "" && previous.Owner != "" {
		message = fmt.Sprintf("%s; %s could match the same request path as %s", message, current.Owner, previous.Owner)
	}
	if current.Source != "" && previous.Source != "" {
		return fmt.Sprintf("%s (%s and %s)", message, current.Source, previous.Source)
	}
	return message
}

func routePatternsOverlap(left, right string) bool {
	leftSegments := routePatternSegments(left)
	rightSegments := routePatternSegments(right)
	leftRest := patternSegmentsHaveRest(leftSegments)
	rightRest := patternSegmentsHaveRest(rightSegments)
	switch {
	case leftRest && rightRest:
		prefix := len(leftSegments) - 1
		if len(rightSegments)-1 < prefix {
			prefix = len(rightSegments) - 1
		}
		return routeSegmentsCompatible(leftSegments[:prefix], rightSegments[:prefix])
	case leftRest:
		// A rest pattern matches one or more remaining segments, so any route
		// at least as long as the rest pattern with a compatible fixed prefix
		// overlaps it.
		if len(rightSegments) < len(leftSegments) {
			return false
		}
		return routeSegmentsCompatible(leftSegments[:len(leftSegments)-1], rightSegments[:len(leftSegments)-1])
	case rightRest:
		return routePatternsOverlap(right, left)
	default:
		if len(leftSegments) != len(rightSegments) {
			return false
		}
		return routeSegmentsCompatible(leftSegments, rightSegments)
	}
}

func routeSegmentsCompatible(left, right []string) bool {
	for index, leftSegment := range left {
		rightSegment := right[index]
		if leftSegment == "{}" || rightSegment == "{}" {
			continue
		}
		if leftSegment != rightSegment {
			return false
		}
	}
	return true
}

func patternSegmentsHaveRest(segments []string) bool {
	return len(segments) > 0 && segments[len(segments)-1] == restPatternPlaceholder
}

func patternHasRest(pattern string) bool {
	return patternSegmentsHaveRest(routePatternSegments(pattern))
}

func patternIsDynamic(pattern string) bool {
	return strings.Contains(pattern, "{}") || patternHasRest(pattern)
}

func routePatternSegments(pattern string) []string {
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func validateRouteMethodConflicts(config gowdk.Config, pages []gwdkir.Page, endpoints []gwdkir.Endpoint, sourceMap gwdkir.SourceMap, refs []gwdkir.ContractReference) []ValidationError {
	seen := map[string][]routeRegistration{}
	var diagnostics []ValidationError
	for _, registration := range routeRegistrations(config, pages, endpoints, sourceMap, refs) {
		key := registration.Method + " " + registration.Pattern
		for _, previous := range seen[key] {
			if previous.Kind == "page" && registration.Kind == "page" {
				continue
			}
			if allowedPageOwnedQueryRouteConflict(previous, registration) {
				continue
			}
			if identicalContractRouteRegistration(previous, registration) {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:   "route_method_conflict",
				PageID: registration.PageID,
				Source: registration.Source,
				Span:   registration.Span,
				Related: relatedSpan(
					previous.Source,
					previous.Span,
					fmt.Sprintf("%s first declared here", previous.Owner),
				),
				Message: fmt.Sprintf(
					"%s %s for %s conflicts with %s",
					registration.Method,
					registration.Route,
					registration.Owner,
					previous.Owner,
				),
			})
		}
		seen[key] = append(seen[key], registration)
	}
	return diagnostics
}

type routeRegistration struct {
	Kind     string
	Owner    string
	Method   string
	Route    string
	Pattern  string
	Contract string
	PageID   string
	Source   string
	Span     source.SourceSpan
}

func routeRegistrations(config gowdk.Config, pages []gwdkir.Page, endpoints []gwdkir.Endpoint, sourceMap gwdkir.SourceMap, refs []gwdkir.ContractReference) []routeRegistration {
	var registrations []routeRegistration
	for _, page := range pages {
		for _, route := range localizedPageRoutes(config.I18N, page) {
			pageInfo, pageIssues := parseRoute(route)
			if len(pageIssues) > 0 {
				continue
			}
			registrations = append(registrations, routeRegistration{
				Kind:    "page",
				Owner:   "page " + page.ID,
				Method:  "GET",
				Route:   route,
				Pattern: pageInfo.Pattern,
				PageID:  page.ID,
				Source:  page.Source,
				Span:    page.Spans.Route,
			})
		}
		_, pageIssues := parseRoute(page.Route)
		if len(pageIssues) == 0 {
			for _, action := range page.Blocks.Actions {
				route := action.Route
				if route == "" {
					route = page.Route
				}
				method := strings.ToUpper(strings.TrimSpace(action.Method))
				if method == "" {
					method = "POST"
				}
				for _, currentRoute := range actionRegistrationRoutes(config.I18N, page.Route, route, action.Route == "") {
					info, issues := parseRoute(currentRoute)
					if len(issues) > 0 {
						continue
					}
					registrations = append(registrations, routeRegistration{
						Kind:    "action",
						Owner:   "action " + page.ID + "." + action.Name,
						Method:  method,
						Route:   currentRoute,
						Pattern: info.Pattern,
						PageID:  page.ID,
						Source:  page.Source,
						Span:    firstSpan(action.RouteSpan, action.Span, page.Spans.Route),
					})
				}
			}
		}

		for _, api := range page.Blocks.APIs {
			route := api.Route
			if route == "" {
				route = page.Route
			}
			info, issues := parseRoute(route)
			if len(issues) > 0 {
				continue
			}
			method := strings.ToUpper(strings.TrimSpace(api.Method))
			if method == "" {
				method = "GET"
			}
			name := api.Name
			if name == "" {
				name = "<anonymous>"
			}
			registrations = append(registrations, routeRegistration{
				Kind:    "api",
				Owner:   "api " + page.ID + "." + name,
				Method:  method,
				Route:   route,
				Pattern: info.Pattern,
				PageID:  page.ID,
				Source:  page.Source,
				Span:    firstSpan(api.RouteSpan, api.Span, page.Spans.Route),
			})
		}
		for _, fragment := range page.Blocks.Fragments {
			info, issues := parseRoute(fragment.Route)
			if len(issues) > 0 {
				continue
			}
			method := strings.ToUpper(strings.TrimSpace(fragment.Method))
			if method == "" {
				method = "GET"
			}
			registrations = append(registrations, routeRegistration{
				Kind:    "fragment",
				Owner:   "fragment " + page.ID + "." + fragment.Name,
				Method:  method,
				Route:   fragment.Route,
				Pattern: info.Pattern,
				PageID:  page.ID,
				Source:  page.Source,
				Span:    firstSpan(fragment.RouteSpan, fragment.Span, page.Spans.Route),
			})
		}
	}
	for _, endpoint := range endpoints {
		if endpoint.Source != gwdkir.EndpointSourceGo {
			continue
		}
		endpointSource, _ := sourceMap.Endpoint(endpoint.SemanticID())
		info, issues := parseRoute(endpoint.Path)
		if len(issues) > 0 {
			continue
		}
		kind := strings.TrimSpace(standaloneEndpointKindLabel(endpoint, endpointSource))
		if kind == "" {
			kind = "endpoint"
		}
		method := strings.ToUpper(strings.TrimSpace(endpoint.Method))
		registrations = append(registrations, routeRegistration{
			Kind:    kind,
			Owner:   fmt.Sprintf("%s %s.%s", kind, endpoint.Package, endpoint.Symbol),
			Method:  method,
			Route:   endpoint.Path,
			Pattern: info.Pattern,
			PageID:  endpoint.PageID,
			Source:  endpoint.SourceFile,
			Span:    firstSpan(endpointSource.RouteSpan, endpoint.Span),
		})
	}
	for _, ref := range refs {
		if strings.TrimSpace(ref.Method) == "" || strings.TrimSpace(ref.Path) == "" {
			continue
		}
		if source.BackendRouteMethod(ref.Method) != contractReferenceRouteMethod(ref.Kind) {
			continue
		}
		if err := source.ValidateBackendRoutePath(ref.Path); err != nil {
			continue
		}
		route := source.BackendRoutePath(ref.Path)
		info, issues := parseRoute(route)
		if len(issues) > 0 {
			continue
		}
		kind := "command"
		if ref.Kind == gwdkir.ContractQuery {
			kind = "query"
		}
		method := strings.ToUpper(strings.TrimSpace(ref.Method))
		registrations = append(registrations, routeRegistration{
			Kind:     "contract_" + kind,
			Owner:    fmt.Sprintf("%s contract %s", kind, ref.Name),
			Method:   method,
			Route:    route,
			Pattern:  info.Pattern,
			Contract: ref.Name,
			PageID:   ref.OwnerID,
			Source:   ref.Source,
			Span:     ref.Span,
		})
	}
	return registrations
}

func localizedPageRoutes(config gowdk.I18NConfig, page gwdkir.Page) []string {
	localized := config.LocalizedRoutes(page.Route)
	routes := make([]string, 0, len(localized))
	for _, route := range localized {
		routes = append(routes, route.Route)
	}
	return routes
}

func actionRegistrationRoutes(config gowdk.I18NConfig, pageRoute string, route string, inherited bool) []string {
	if !inherited || !config.Enabled() {
		return []string{route}
	}
	localized := config.LocalizedRoutes(pageRoute)
	routes := make([]string, 0, len(localized))
	for _, item := range localized {
		routes = append(routes, item.Route)
	}
	return routes
}

func allowedPageOwnedQueryRouteConflict(first routeRegistration, current routeRegistration) bool {
	return pageOwnedQueryRouteConflict(first, current) || pageOwnedQueryRouteConflict(current, first)
}

func identicalContractRouteRegistration(first routeRegistration, current routeRegistration) bool {
	if !strings.HasPrefix(first.Kind, "contract_") || first.Kind != current.Kind {
		return false
	}
	return first.Method == current.Method &&
		first.Route == current.Route &&
		first.Contract == current.Contract &&
		first.PageID == current.PageID
}

func pageOwnedQueryRouteConflict(page routeRegistration, query routeRegistration) bool {
	return page.Kind == "page" &&
		query.Kind == "contract_query" &&
		query.PageID == page.PageID &&
		query.Route == page.Route
}

func validateStandaloneEndpoints(program gwdkir.Program) []ValidationError {
	var diagnostics []ValidationError
	for _, endpoint := range program.Endpoints {
		if endpoint.Source != gwdkir.EndpointSourceGo {
			continue
		}
		endpointSource, _ := program.EndpointSource(endpoint.SemanticID())
		page := gwdkir.Page{ID: endpoint.PageID, Source: endpoint.SourceFile}
		if !isExportedHandlerName(endpoint.Symbol) {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "invalid_backend_handler_name",
				PageID:  page.ID,
				Source:  endpoint.SourceFile,
				Span:    endpoint.Span,
				Message: fmt.Sprintf("%s endpoint handler %q must be an exported Go identifier", standaloneEndpointKindLabel(endpoint, endpointSource), endpoint.Symbol),
			})
		}
		method := strings.ToUpper(strings.TrimSpace(endpoint.Method))
		if endpoint.Kind == gwdkir.EndpointAction {
			if method != "POST" {
				diagnostics = append(diagnostics, ValidationError{
					Code:    "unsupported_action_method",
					PageID:  page.ID,
					Source:  endpoint.SourceFile,
					Span:    endpoint.Span,
					Message: fmt.Sprintf("Go action endpoint %s uses unsupported method %s; actions currently require POST", endpoint.Symbol, method),
				})
			}
		}
		info, issues := parseRoute(endpoint.Path)
		label := fmt.Sprintf("Go %s endpoint path", standaloneEndpointKindLabel(endpoint, endpointSource))
		diagnostics = append(diagnostics, routeDiagnostics(page, label, issues, firstSpan(endpointSource.RouteSpan, endpoint.Span), endpointSource.RouteParams)...)
		if len(issues) == 0 && info.RestParam != "" {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "malformed_route",
				PageID:  page.ID,
				Source:  endpoint.SourceFile,
				Span:    firstSpan(endpointSource.RouteSpan, endpoint.Span),
				Message: fmt.Sprintf("%s declares invalid %s: route %q uses rest route parameter {%s...}; rest parameters are only supported on page routes", page.ID, label, endpoint.Path, info.RestParam),
			})
		}
	}
	return diagnostics
}

func standaloneEndpointKindLabel(endpoint gwdkir.Endpoint, endpointSource gwdkir.EndpointSourceMap) string {
	if endpointSource.Kind != "" {
		return endpointSource.Kind
	}
	if endpoint.Kind == gwdkir.EndpointAction {
		return "act"
	}
	return string(endpoint.Kind)
}

func routeDiagnostics(page gwdkir.Page, label string, issues []routeIssue, routeSpan source.SourceSpan, paramSpans []source.NamedSpan) []ValidationError {
	if len(issues) == 0 {
		return nil
	}
	diagnostics := make([]ValidationError, 0, len(issues))
	for _, issue := range issues {
		diagnostics = append(diagnostics, ValidationError{
			Code:   issue.Code,
			PageID: page.ID,
			Source: page.Source,
			Span:   routeIssueSpan(issue, routeSpan, paramSpans),
			Message: fmt.Sprintf(
				"%s declares invalid %s: %s",
				page.ID,
				label,
				issue.Message,
			),
		})
	}
	return diagnostics
}

const restPatternPlaceholder = source.RestRoutePatternPlaceholder

type routeInfo = source.RoutePattern
type routeIssue = source.RouteIssue

func routeIssueSpan(issue routeIssue, routeSpan source.SourceSpan, paramSpans []source.NamedSpan) source.SourceSpan {
	if issue.Param != "" {
		if issue.ParamOccurrence > 1 {
			return spanForNameOccurrence(paramSpans, issue.Param, issue.ParamOccurrence, routeSpan)
		}
		return spanForName(paramSpans, issue.Param, routeSpan)
	}
	return routeSpan
}

func parseRoute(route string) (routeInfo, []routeIssue) {
	return source.ParseRoutePattern(route)
}

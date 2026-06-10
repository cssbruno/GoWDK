package compiler

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func validateUniquePageRoutes(pages []gwdkir.Page) []ValidationError {
	seen := map[string]gwdkir.Page{}
	var diagnostics []ValidationError
	for _, page := range pages {
		info, issues := parseRoute(page.Route)
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
			Message: duplicateRouteMessage(
				page.Route,
				first.ID,
				first.Source,
				page.ID,
				page.Source,
			),
		})
	}
	return diagnostics
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

func validateAmbiguousDynamicPageRoutes(pages []gwdkir.Page, endpoints []gwdkir.GoEndpoint) []ValidationError {
	var registered []routeRegistration
	var diagnostics []ValidationError
	for _, current := range routeRegistrations(pages, endpoints) {
		for _, previous := range registered {
			if current.Pattern == previous.Pattern {
				// Exact duplicates are reported by the duplicate-route and
				// route-method-conflict checks.
				continue
			}
			bothPages := current.Kind == "page" && previous.Kind == "page"
			if bothPages {
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
				// Endpoints cannot declare rest routes themselves, but a
				// same-method endpoint inside a rest page's namespace would
				// shadow part of it at request time, so flag that overlap.
				restPage := (current.Kind == "page" && patternHasRest(current.Pattern)) ||
					(previous.Kind == "page" && patternHasRest(previous.Pattern))
				if !restPage || current.Method != previous.Method {
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
	message := fmt.Sprintf("ambiguous dynamic page route %q overlaps %q", current.Route, previous.Route)
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

func validateRouteMethodConflicts(pages []gwdkir.Page, endpoints []gwdkir.GoEndpoint) []ValidationError {
	seen := map[string]routeRegistration{}
	var diagnostics []ValidationError
	for _, registration := range routeRegistrations(pages, endpoints) {
		key := registration.Method + " " + registration.Pattern
		first, exists := seen[key]
		if !exists {
			seen[key] = registration
			continue
		}
		if first.Kind == "page" && registration.Kind == "page" {
			continue
		}
		diagnostics = append(diagnostics, ValidationError{
			Code:   "route_method_conflict",
			PageID: registration.PageID,
			Source: registration.Source,
			Span:   registration.Span,
			Message: fmt.Sprintf(
				"%s %s for %s conflicts with %s",
				registration.Method,
				registration.Route,
				registration.Owner,
				first.Owner,
			),
		})
	}
	return diagnostics
}

type routeRegistration struct {
	Kind    string
	Owner   string
	Method  string
	Route   string
	Pattern string
	PageID  string
	Source  string
	Span    source.SourceSpan
}

func routeRegistrations(pages []gwdkir.Page, endpoints []gwdkir.GoEndpoint) []routeRegistration {
	var registrations []routeRegistration
	for _, page := range pages {
		pageInfo, pageIssues := parseRoute(page.Route)
		if len(pageIssues) == 0 {
			registrations = append(registrations, routeRegistration{
				Kind:    "page",
				Owner:   "page " + page.ID,
				Method:  "GET",
				Route:   page.Route,
				Pattern: pageInfo.Pattern,
				PageID:  page.ID,
				Source:  page.Source,
				Span:    page.Spans.Route,
			})
			for _, action := range page.Blocks.Actions {
				route := action.Route
				if route == "" {
					route = page.Route
				}
				info, issues := parseRoute(route)
				if len(issues) > 0 {
					continue
				}
				method := strings.ToUpper(strings.TrimSpace(action.Method))
				if method == "" {
					method = "POST"
				}
				registrations = append(registrations, routeRegistration{
					Kind:    "action",
					Owner:   "action " + page.ID + "." + action.Name,
					Method:  method,
					Route:   route,
					Pattern: info.Pattern,
					PageID:  page.ID,
					Source:  page.Source,
					Span:    firstSpan(action.RouteSpan, action.Span, page.Spans.Route),
				})
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
		info, issues := parseRoute(endpoint.Route)
		if len(issues) > 0 {
			continue
		}
		kind := strings.TrimSpace(endpoint.Kind)
		if kind == "" {
			kind = "endpoint"
		}
		method := strings.ToUpper(strings.TrimSpace(endpoint.Method))
		registrations = append(registrations, routeRegistration{
			Kind:    kind,
			Owner:   fmt.Sprintf("%s %s.%s", kind, endpoint.Package, endpoint.Name),
			Method:  method,
			Route:   endpoint.Route,
			Pattern: info.Pattern,
			PageID:  standaloneEndpointPageID(endpoint.Package, endpoint.Name),
			Source:  endpoint.Source,
			Span:    firstSpan(endpoint.RouteSpan, endpoint.Span),
		})
	}
	return registrations
}

func validateStandaloneEndpoints(endpoints []gwdkir.GoEndpoint) []ValidationError {
	var diagnostics []ValidationError
	for _, endpoint := range endpoints {
		page := gwdkir.Page{ID: standaloneEndpointPageID(endpoint.Package, endpoint.Name), Source: endpoint.Source}
		if !isExportedHandlerName(endpoint.Name) {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "invalid_backend_handler_name",
				PageID:  page.ID,
				Source:  endpoint.Source,
				Span:    endpoint.Span,
				Message: fmt.Sprintf("%s endpoint handler %q must be an exported Go identifier", endpoint.Kind, endpoint.Name),
			})
		}
		method := strings.ToUpper(strings.TrimSpace(endpoint.Method))
		if method == "" {
			if endpoint.Kind == "act" || endpoint.Kind == "action" {
				method = "POST"
			} else {
				method = "GET"
			}
		}
		if endpoint.Kind == "act" || endpoint.Kind == "action" {
			if method != "POST" {
				diagnostics = append(diagnostics, ValidationError{
					Code:    "unsupported_action_method",
					PageID:  page.ID,
					Source:  endpoint.Source,
					Span:    endpoint.Span,
					Message: fmt.Sprintf("Go action endpoint %s uses unsupported method %s; actions currently require POST", endpoint.Name, method),
				})
			}
		}
		info, issues := parseRoute(endpoint.Route)
		label := fmt.Sprintf("Go %s endpoint path", endpoint.Kind)
		diagnostics = append(diagnostics, routeDiagnostics(page, label, issues, firstSpan(endpoint.RouteSpan, endpoint.Span), endpoint.RouteParams)...)
		if len(issues) == 0 && info.RestParam != "" {
			diagnostics = append(diagnostics, ValidationError{
				Code:    "malformed_route",
				PageID:  page.ID,
				Source:  endpoint.Source,
				Span:    firstSpan(endpoint.RouteSpan, endpoint.Span),
				Message: fmt.Sprintf("%s declares invalid %s: route %q uses rest route parameter {%s...}; rest parameters are only supported on page routes", page.ID, label, endpoint.Route, info.RestParam),
			})
		}
	}
	return diagnostics
}

func standaloneEndpointPageID(packageName, name string) string {
	if packageName == "" {
		return name
	}
	return packageName + "." + name
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

// restPatternPlaceholder is the normalized pattern segment for a trailing rest
// parameter such as {path...}. It differs from the single-segment placeholder
// "{}" so /a/{x...} and /a/{y...} normalize to the same pattern while staying
// distinct from /a/{x}.
const restPatternPlaceholder = "{**}"

type routeInfo struct {
	Pattern string
	Params  []string
	// RestParam names the trailing {name...} parameter, when declared.
	RestParam string
}

type routeIssue struct {
	Code            string
	Message         string
	Param           string
	ParamOccurrence int
}

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
	var issues []routeIssue
	if route == "" {
		return routeInfo{}, []routeIssue{{
			Code:    "malformed_route",
			Message: "route is required",
		}}
	}
	if strings.TrimSpace(route) != route {
		issues = append(issues, routeIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must not include leading or trailing whitespace", route),
		})
	}
	if !strings.HasPrefix(route, "/") {
		issues = append(issues, routeIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must start with /", route),
		})
	}
	if strings.Contains(route, "#") || routeContainsQueryOutsideParams(route) {
		issues = append(issues, routeIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must not contain query strings or fragments", route),
		})
	}
	if strings.Contains(route, `\`) {
		issues = append(issues, routeIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must use / path separators", route),
		})
	}
	if containsSpaceOrControl(route) {
		issues = append(issues, routeIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must not contain whitespace or control characters", route),
		})
	}
	if route == "/" {
		return routeInfo{Pattern: "/"}, issues
	}
	if !strings.HasPrefix(route, "/") {
		return routeInfo{}, issues
	}

	rawSegments := strings.Split(strings.TrimPrefix(route, "/"), "/")
	segments := make([]string, 0, len(rawSegments))
	params := make([]string, 0, len(rawSegments))
	restParam := ""
	paramCounts := map[string]int{}
	for index, segment := range rawSegments {
		switch {
		case segment == "":
			issues = append(issues, routeIssue{
				Code:    "malformed_route",
				Message: fmt.Sprintf("route %q must not contain empty path segments; omit trailing slashes except for /", route),
			})
		case segment == "." || segment == "..":
			issues = append(issues, routeIssue{
				Code:    "malformed_route",
				Message: fmt.Sprintf("route %q contains unsafe path segment %q", route, segment),
			})
		case strings.ContainsAny(segment, "{}"):
			param, ok := routeParamSegment(segment)
			if !ok {
				issues = append(issues, routeIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has invalid route parameter segment %q; use {name} or {name:type} as the whole segment", route, segment),
				})
				continue
			}
			if strings.HasSuffix(param.Name, "?") {
				issues = append(issues, routeIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q uses optional route parameter %q; optional route parameters are not supported; declare explicit routes for each shape (rest parameters {name...} are supported as the final segment)", route, segment),
				})
				continue
			}
			if param.Rest && param.Name == "" {
				issues = append(issues, routeIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has rest route parameter segment %q without a name; declare it as {name...}", route, segment),
				})
				continue
			}
			if !param.Rest && strings.Contains(param.Name, ".") {
				issues = append(issues, routeIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has invalid route parameter segment %q; rest route parameters use exactly three dots, such as {name...}", route, segment),
				})
				continue
			}
			if !isRouteParamName(param.Name) {
				issues = append(issues, routeIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has invalid route parameter name %q", route, param.Name),
				})
				continue
			}
			if param.Rest && param.HasType {
				issues = append(issues, routeIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q declares typed rest route parameter %q; rest route parameters are always strings, declare it as {%s...}", route, segment, param.Name),
					Param:   param.Name,
				})
				continue
			}
			if !isRouteParamType(param.Type) {
				issues = append(issues, routeIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has invalid route parameter type %q for %q; supported types are string, int, int64, uint, uint64, bool, float64", route, param.Type, param.Name),
					Param:   param.Name,
				})
				continue
			}
			if param.Rest && index != len(rawSegments)-1 {
				issues = append(issues, routeIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q declares rest route parameter {%s...} before the end of the route; rest parameters must be the last segment", route, param.Name),
					Param:   param.Name,
				})
				continue
			}
			paramCounts[param.Name]++
			if paramCounts[param.Name] > 1 {
				issues = append(issues, routeIssue{
					Code:            "duplicate_route_param",
					Message:         fmt.Sprintf("route %q repeats route parameter %q", route, param.Name),
					Param:           param.Name,
					ParamOccurrence: paramCounts[param.Name],
				})
				continue
			}
			params = append(params, param.Name)
			if param.Rest {
				restParam = param.Name
				segments = append(segments, restPatternPlaceholder)
				continue
			}
			segments = append(segments, "{}")
		default:
			segments = append(segments, segment)
		}
	}

	pattern := "/" + strings.Join(segments, "/")
	if len(segments) == 0 {
		pattern = "/"
	}
	return routeInfo{Pattern: pattern, Params: params, RestParam: restParam}, issues
}

type routeParamSegmentInfo struct {
	Name    string
	Type    string
	Rest    bool
	HasType bool
}

func routeParamSegment(segment string) (routeParamSegmentInfo, bool) {
	if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
		return routeParamSegmentInfo{}, false
	}
	if strings.Count(segment, "{") != 1 || strings.Count(segment, "}") != 1 {
		return routeParamSegmentInfo{}, false
	}
	value := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
	name, paramType, found := strings.Cut(value, ":")
	if !found {
		paramType = "string"
	}
	rest := strings.HasSuffix(name, "...")
	if rest {
		name = strings.TrimSuffix(name, "...")
	}
	return routeParamSegmentInfo{Name: name, Type: paramType, Rest: rest, HasType: found}, true
}

// routeContainsQueryOutsideParams reports whether route contains a "?" that is
// not inside a {param} segment, so optional-param forms such as {slug?} get a
// dedicated diagnostic instead of the query-string one.
func routeContainsQueryOutsideParams(route string) bool {
	depth := 0
	for _, r := range route {
		switch r {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		case '?':
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

func isRouteParamType(value string) bool {
	switch value {
	case "string", "int", "int64", "uint", "uint64", "bool", "float64":
		return true
	default:
		return false
	}
}

func isRouteParamName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if !isASCIIIdentStart(r) {
				return false
			}
			continue
		}
		if !isASCIIIdentPart(r) {
			return false
		}
	}
	return true
}

func isASCIIIdentStart(r rune) bool {
	return r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isASCIIIdentPart(r rune) bool {
	return isASCIIIdentStart(r) || (r >= '0' && r <= '9')
}

func containsSpaceOrControl(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

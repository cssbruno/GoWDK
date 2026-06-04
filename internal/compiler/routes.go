package compiler

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func validateUniquePageRoutes(pages []manifest.Page) []ValidationError {
	seen := map[string]manifest.Page{}
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

func validateAmbiguousDynamicPageRoutes(pages []manifest.Page) []ValidationError {
	var dynamicRoutes []routeRegistration
	var diagnostics []ValidationError
	for _, page := range pages {
		info, issues := parseRoute(page.Route)
		if len(issues) > 0 || len(info.Params) == 0 {
			continue
		}
		current := routeRegistration{
			Kind:    "page",
			Owner:   "page " + page.ID,
			Method:  "GET",
			Route:   page.Route,
			Pattern: info.Pattern,
			PageID:  page.ID,
			Source:  page.Source,
			Span:    page.Spans.Route,
		}
		for _, previous := range dynamicRoutes {
			if current.Pattern == previous.Pattern {
				continue
			}
			if !routePatternsOverlap(current.Pattern, previous.Pattern) {
				continue
			}
			diagnostics = append(diagnostics, ValidationError{
				Code:    "ambiguous_dynamic_route",
				PageID:  page.ID,
				Source:  page.Source,
				Span:    page.Spans.Route,
				Message: ambiguousDynamicRouteMessage(current, previous),
			})
		}
		dynamicRoutes = append(dynamicRoutes, current)
	}
	return diagnostics
}

func ambiguousDynamicRouteMessage(current, previous routeRegistration) string {
	message := fmt.Sprintf("ambiguous dynamic page route %q overlaps %q", current.Route, previous.Route)
	if current.PageID != "" && previous.PageID != "" {
		message = fmt.Sprintf("%s; page %s could match the same request path as page %s", message, current.PageID, previous.PageID)
	}
	if current.Source != "" && previous.Source != "" {
		return fmt.Sprintf("%s (%s and %s)", message, current.Source, previous.Source)
	}
	return message
}

func routePatternsOverlap(left, right string) bool {
	leftSegments := routePatternSegments(left)
	rightSegments := routePatternSegments(right)
	if len(leftSegments) != len(rightSegments) {
		return false
	}
	for index, leftSegment := range leftSegments {
		rightSegment := rightSegments[index]
		if leftSegment == "{}" || rightSegment == "{}" {
			continue
		}
		if leftSegment != rightSegment {
			return false
		}
	}
	return true
}

func routePatternSegments(pattern string) []string {
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func validateRouteMethodConflicts(pages []manifest.Page) []ValidationError {
	seen := map[string]routeRegistration{}
	var diagnostics []ValidationError
	for _, registration := range routeRegistrations(pages) {
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
	Span    manifest.SourceSpan
}

func routeRegistrations(pages []manifest.Page) []routeRegistration {
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
				registrations = append(registrations, routeRegistration{
					Kind:    "action",
					Owner:   "action " + page.ID + "." + action.Name,
					Method:  "POST",
					Route:   page.Route,
					Pattern: pageInfo.Pattern,
					PageID:  page.ID,
					Source:  page.Source,
					Span:    action.Span,
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
	}
	return registrations
}

func routeDiagnostics(page manifest.Page, label string, issues []routeIssue, routeSpan manifest.SourceSpan, paramSpans []manifest.NamedSpan) []ValidationError {
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

type routeInfo struct {
	Pattern string
	Params  []string
}

type routeIssue struct {
	Code    string
	Message string
	Param   string
}

func routeIssueSpan(issue routeIssue, routeSpan manifest.SourceSpan, paramSpans []manifest.NamedSpan) manifest.SourceSpan {
	if issue.Param != "" {
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
	if strings.ContainsAny(route, "?#") {
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
	seenParams := map[string]bool{}
	for _, segment := range rawSegments {
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
					Message: fmt.Sprintf("route %q has invalid route parameter segment %q; use {name} as the whole segment", route, segment),
				})
				continue
			}
			if !isRouteParamName(param) {
				issues = append(issues, routeIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has invalid route parameter name %q", route, param),
				})
				continue
			}
			if seenParams[param] {
				issues = append(issues, routeIssue{
					Code:    "duplicate_route_param",
					Message: fmt.Sprintf("route %q repeats route parameter %q", route, param),
					Param:   param,
				})
				continue
			}
			seenParams[param] = true
			params = append(params, param)
			segments = append(segments, "{}")
		default:
			segments = append(segments, segment)
		}
	}

	pattern := "/" + strings.Join(segments, "/")
	if len(segments) == 0 {
		pattern = "/"
	}
	return routeInfo{Pattern: pattern, Params: params}, issues
}

func routeParamSegment(segment string) (string, bool) {
	if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
		return "", false
	}
	if strings.Count(segment, "{") != 1 || strings.Count(segment, "}") != 1 {
		return "", false
	}
	return strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}"), true
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

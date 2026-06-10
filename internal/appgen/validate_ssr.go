package appgen

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

func validateSSRRoutes(routes []SSRRoute) error {
	seen := map[string]SSRRoute{}
	var dynamicRoutes []SSRRoute
	for index, route := range routes {
		if strings.TrimSpace(route.PageID) == "" {
			return fmt.Errorf("generated SSR route is missing page ID")
		}
		if err := validateSSRRoutePattern(route.Route); err != nil {
			return fmt.Errorf("generated SSR %s: %w", route.PageID, err)
		}
		if strings.TrimSpace(route.HTML) == "" {
			return fmt.Errorf("generated SSR %s has empty HTML", route.PageID)
		}
		if route.ErrorPage != "" {
			errorPage, err := source.ErrorPagePath(route.ErrorPage)
			if err != nil {
				return fmt.Errorf("generated SSR %s: %w", route.PageID, err)
			}
			route.ErrorPage = errorPage
			routes[index].ErrorPage = errorPage
		}
		if err := validateSSRReplacements(route); err != nil {
			return err
		}
		pattern := ssrRoutePattern(route.Route)
		params := ssrRoutePatternParams(route.Route)
		if previous, exists := seen[pattern]; exists {
			return fmt.Errorf("generated SSR %s route %q duplicates SSR page %s", route.PageID, route.Route, previous.PageID)
		}
		if len(params) > 0 {
			for _, previous := range dynamicRoutes {
				if pattern == ssrRoutePattern(previous.Route) {
					continue
				}
				if ssrRoutePatternsOverlap(pattern, ssrRoutePattern(previous.Route)) {
					return fmt.Errorf("generated SSR %s route %q overlaps dynamic SSR page %s route %q", route.PageID, route.Route, previous.PageID, previous.Route)
				}
			}
		}
		seen[pattern] = route
		if len(params) > 0 {
			dynamicRoutes = append(dynamicRoutes, route)
		}
	}
	return nil
}

func validateSSRRoutePattern(value string) error {
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("route %q must be an absolute path", value)
	}
	if strings.ContainsAny(value, "?#") {
		return fmt.Errorf("route %q must be a concrete path without query or fragment", value)
	}
	params := map[string]bool{}
	for _, segment := range strings.Split(strings.Trim(value, "/"), "/") {
		if segment == "" {
			continue
		}
		if strings.ContainsAny(segment, "{}") {
			if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") || strings.Count(segment, "{") != 1 || strings.Count(segment, "}") != 1 {
				return fmt.Errorf("route %q has invalid route parameter segment %q", value, segment)
			}
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			if before, _, found := strings.Cut(name, ":"); found {
				name = before
			}
			if !isIdentifier(name) {
				return fmt.Errorf("route %q has invalid route parameter name %q", value, name)
			}
			if params[name] {
				return fmt.Errorf("route %q declares duplicate route parameter %q", value, name)
			}
			params[name] = true
		}
	}
	return nil
}

func validateSSRReplacements(route SSRRoute) error {
	routeParams := map[string]bool{}
	for _, param := range ssrRoutePatternParams(route.Route) {
		routeParams[param] = true
	}
	seen := map[string]bool{}
	for _, replacement := range route.Replacements {
		if !routeParams[replacement.Param] {
			return fmt.Errorf("generated SSR %s replacement param %q is not declared by route %q", route.PageID, replacement.Param, route.Route)
		}
		if seen[replacement.Param] {
			return fmt.Errorf("generated SSR %s declares duplicate replacement param %q", route.PageID, replacement.Param)
		}
		if strings.TrimSpace(replacement.Placeholder) == "" {
			return fmt.Errorf("generated SSR %s replacement for %q has empty placeholder", route.PageID, replacement.Param)
		}
		seen[replacement.Param] = true
	}
	return nil
}

func ssrRoutePatternParams(route string) []string {
	var params []string
	for _, segment := range strings.Split(strings.Trim(route, "/"), "/") {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			if before, _, found := strings.Cut(name, ":"); found {
				name = before
			}
			params = append(params, name)
		}
	}
	return params
}

func ssrRoutePattern(route string) string {
	segments := ssrRoutePatternSegments(route)
	if len(segments) == 0 {
		return "/"
	}
	for index, segment := range segments {
		if strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}") {
			segments[index] = "{}"
		}
	}
	return "/" + strings.Join(segments, "/")
}

func ssrRoutePatternsOverlap(left, right string) bool {
	leftSegments := ssrRoutePatternSegments(left)
	rightSegments := ssrRoutePatternSegments(right)
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

func ssrRoutePatternSegments(route string) []string {
	trimmed := strings.Trim(route, "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '_'
		if !valid {
			return false
		}
		if index == 0 && char >= '0' && char <= '9' {
			return false
		}
	}
	return true
}

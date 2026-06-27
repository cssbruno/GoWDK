package source

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// RestRoutePatternPlaceholder is the normalized route pattern segment for a
// trailing rest parameter such as {path...}.
const RestRoutePatternPlaceholder = "{**}"

// RoutePattern describes the normalized shape of one route declaration.
type RoutePattern struct {
	Pattern   string
	Params    []string
	RestParam string
}

// RouteIssue describes one route validation problem without binding it to a
// compiler diagnostic type.
type RouteIssue struct {
	Code            string
	Message         string
	Param           string
	ParamOccurrence int
}

// ParseRoutePattern validates and normalizes a declared route. Dynamic segment
// names normalize to "{}"; a trailing rest segment normalizes to "{**}".
func ParseRoutePattern(route string) (RoutePattern, []RouteIssue) {
	var issues []RouteIssue
	if route == "" {
		return RoutePattern{}, []RouteIssue{{
			Code:    "malformed_route",
			Message: "route is required",
		}}
	}
	if strings.TrimSpace(route) != route {
		issues = append(issues, RouteIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must not include leading or trailing whitespace", route),
		})
	}
	if !strings.HasPrefix(route, "/") {
		issues = append(issues, RouteIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must start with /", route),
		})
	}
	if strings.Contains(route, "#") || RouteContainsQueryOutsideParams(route) {
		issues = append(issues, RouteIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must not contain query strings or fragments", route),
		})
	}
	if strings.Contains(route, `\`) {
		issues = append(issues, RouteIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must use / path separators", route),
		})
	}
	if containsSpaceOrControl(route) {
		issues = append(issues, RouteIssue{
			Code:    "malformed_route",
			Message: fmt.Sprintf("route %q must not contain whitespace or control characters", route),
		})
	}
	if route == "/" {
		return RoutePattern{Pattern: "/"}, issues
	}
	if !strings.HasPrefix(route, "/") {
		return RoutePattern{}, issues
	}

	rawSegments := strings.Split(strings.TrimPrefix(route, "/"), "/")
	segments := make([]string, 0, len(rawSegments))
	params := make([]string, 0, len(rawSegments))
	restParam := ""
	paramCounts := map[string]int{}
	for index, segment := range rawSegments {
		switch {
		case segment == "":
			issues = append(issues, RouteIssue{
				Code:    "malformed_route",
				Message: fmt.Sprintf("route %q must not contain empty path segments; omit trailing slashes except for /", route),
			})
		case segment == "." || segment == "..":
			issues = append(issues, RouteIssue{
				Code:    "malformed_route",
				Message: fmt.Sprintf("route %q contains unsafe path segment %q", route, segment),
			})
		case strings.ContainsAny(segment, "{}"):
			param, ok := ParseRouteParamSegment(segment)
			if !ok {
				issues = append(issues, RouteIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has invalid route parameter segment %q; use {name} or {name:type} as the whole segment", route, segment),
				})
				continue
			}
			if strings.HasSuffix(param.Name, "?") {
				issues = append(issues, RouteIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q uses optional route parameter %q; optional route parameters are not supported; declare explicit routes for each shape (rest parameters {name...} are supported as the final segment)", route, segment),
				})
				continue
			}
			if param.Rest && param.Name == "" {
				issues = append(issues, RouteIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has rest route parameter segment %q without a name; declare it as {name...}", route, segment),
				})
				continue
			}
			if !param.Rest && strings.Contains(param.Name, ".") {
				issues = append(issues, RouteIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has invalid route parameter segment %q; rest route parameters use exactly three dots, such as {name...}", route, segment),
				})
				continue
			}
			if !IsRouteParamName(param.Name) {
				issues = append(issues, RouteIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has invalid route parameter name %q", route, param.Name),
				})
				continue
			}
			if param.Rest && param.HasType {
				issues = append(issues, RouteIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q declares typed rest route parameter %q; rest route parameters are always strings, declare it as {%s...}", route, segment, param.Name),
					Param:   param.Name,
				})
				continue
			}
			if !IsRouteParamType(param.Type) {
				issues = append(issues, RouteIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q has invalid route parameter type %q for %q; supported types are string, int, int64, uint, uint64, bool, float64", route, param.Type, param.Name),
					Param:   param.Name,
				})
				continue
			}
			if param.Rest && index != len(rawSegments)-1 {
				issues = append(issues, RouteIssue{
					Code:    "malformed_route",
					Message: fmt.Sprintf("route %q declares rest route parameter {%s...} before the end of the route; rest parameters must be the last segment", route, param.Name),
					Param:   param.Name,
				})
				continue
			}
			paramCounts[param.Name]++
			if paramCounts[param.Name] > 1 {
				issues = append(issues, RouteIssue{
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
				segments = append(segments, RestRoutePatternPlaceholder)
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
	return RoutePattern{Pattern: pattern, Params: params, RestParam: restParam}, issues
}

// RouteParamSegmentInfo describes a single dynamic route segment.
type RouteParamSegmentInfo struct {
	Name    string
	Type    string
	Rest    bool
	HasType bool
}

// ParseRouteParamSegment parses {name}, {name:type}, and {name...} segments.
func ParseRouteParamSegment(segment string) (RouteParamSegmentInfo, bool) {
	if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
		return RouteParamSegmentInfo{}, false
	}
	if strings.Count(segment, "{") != 1 || strings.Count(segment, "}") != 1 {
		return RouteParamSegmentInfo{}, false
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
	return RouteParamSegmentInfo{Name: name, Type: paramType, Rest: rest, HasType: found}, true
}

// RouteContainsQueryOutsideParams reports whether route contains a "?" outside
// a {param} segment.
func RouteContainsQueryOutsideParams(route string) bool {
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

// IsRouteParamType reports whether value is a supported route param type.
func IsRouteParamType(value string) bool {
	switch value {
	case "string", "int", "int64", "uint", "uint64", "bool", "float64":
		return true
	default:
		return false
	}
}

// IsRouteParamName reports whether value is a valid route param identifier.
func IsRouteParamName(value string) bool {
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

// RouteParamsFromPath parses dynamic route parameters from a route pattern.
func RouteParamsFromPath(route string) []RouteParam {
	var params []RouteParam
	for index := 0; index < len(route); index++ {
		if route[index] != '{' {
			continue
		}
		end := strings.IndexByte(route[index:], '}')
		if end < 0 {
			continue
		}
		end += index
		param, ok := ParseRouteParamSegment(route[index : end+1])
		if ok && IsRouteParamName(param.Name) && IsRouteParamType(param.Type) && (!param.Rest || !param.HasType) {
			params = append(params, RouteParam{Name: param.Name, Type: param.Type})
		}
		index = end
	}
	return params
}

// ParseRouteDeclaration extracts typed params from a source route declaration
// and returns the normalized path used by downstream metadata.
func ParseRouteDeclaration(route string, lineNumber int, rawLine string) (string, []RouteParam, []NamedSpan, error) {
	matches := findStrictRouteParamMatches(route)
	if len(matches) == 0 {
		return route, nil, nil, nil
	}
	routeStart := strings.Index(rawLine, route)
	if routeStart < 0 {
		routeStart = 0
	}
	normalizedParts := make([]string, 0, len(matches)*3+1)
	last := 0
	params := make([]RouteParam, 0, len(matches))
	spans := make([]NamedSpan, 0, len(matches))
	for _, match := range matches {
		name := route[match.nameStart:match.nameEnd]
		paramType := "string"
		if match.typeStart >= 0 && match.typeEnd >= 0 {
			paramType = route[match.typeStart:match.typeEnd]
		}
		if !IsRouteParamType(paramType) {
			return "", nil, nil, fmt.Errorf("unsupported route parameter type %q for %s; supported types: string, int, int64, uint, uint64, bool, float64", paramType, name)
		}
		start := routeStart + match.start
		end := routeStart + match.end
		span := SourceSpan{
			Start: SourcePosition{Line: lineNumber, Column: routeRuneColumn(rawLine, start)},
			End:   SourcePosition{Line: lineNumber, Column: routeRuneColumn(rawLine, end)},
		}
		params = append(params, RouteParam{Name: name, Type: paramType, Span: span})
		spans = append(spans, NamedSpan{Name: name, Span: span})
		normalizedParts = append(normalizedParts, route[last:match.start], "{", name, "}")
		last = match.end
	}
	normalizedParts = append(normalizedParts, route[last:])
	return strings.Join(normalizedParts, ""), params, spans, nil
}

type routeParamMatch struct {
	start     int
	end       int
	nameStart int
	nameEnd   int
	typeStart int
	typeEnd   int
}

func findStrictRouteParamMatches(route string) []routeParamMatch {
	var matches []routeParamMatch
	for index := 0; index < len(route); index++ {
		if route[index] != '{' {
			continue
		}
		end := strings.IndexByte(route[index:], '}')
		if end < 0 {
			continue
		}
		end += index
		body := route[index+1 : end]
		colon := strings.IndexByte(body, ':')
		name := body
		paramType := ""
		if colon >= 0 {
			name = body[:colon]
			paramType = body[colon+1:]
		}
		if !IsRouteParamName(name) || (paramType != "" && !IsRouteParamName(paramType)) {
			continue
		}
		match := routeParamMatch{start: index, end: end + 1, nameStart: index + 1, nameEnd: index + 1 + len(name), typeStart: -1, typeEnd: -1}
		if colon >= 0 {
			match.typeStart = index + 1 + colon + 1
			match.typeEnd = match.typeStart + len(paramType)
		}
		matches = append(matches, match)
		index = end
	}
	return matches
}

func containsSpaceOrControl(value string) bool {
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func isASCIIIdentStart(r rune) bool {
	return r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
}

func isASCIIIdentPart(r rune) bool {
	return isASCIIIdentStart(r) || (r >= '0' && r <= '9')
}

func routeRuneColumn(line string, byteOffset int) int {
	if byteOffset > len(line) {
		byteOffset = len(line)
	}
	return utf8.RuneCountInString(line[:byteOffset]) + 1
}

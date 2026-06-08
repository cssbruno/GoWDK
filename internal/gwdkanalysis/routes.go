package gwdkanalysis

import (
	"regexp"
	"sort"

	"github.com/cssbruno/gowdk/internal/manifest"
)

var routeParamPattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(?::([A-Za-z_][A-Za-z0-9_]*))?\}`)

func routeParams(route string) []string {
	matches := routeParamPattern.FindAllStringSubmatch(route, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		name := match[1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func typedRouteParams(route string) []manifest.RouteParam {
	matches := routeParamPattern.FindAllStringSubmatch(route, -1)
	out := make([]manifest.RouteParam, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		name := match[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		paramType := match[2]
		if paramType == "" {
			paramType = "string"
		}
		out = append(out, manifest.RouteParam{Name: name, Type: paramType})
	}
	return out
}

func routeParamSpans(route string, fallback manifest.SourceSpan) []manifest.NamedSpan {
	params := routeParams(route)
	out := make([]manifest.NamedSpan, 0, len(params))
	for _, param := range params {
		out = append(out, manifest.NamedSpan{Name: param, Span: fallback})
	}
	return out
}

func namedSpans(values []string, fallback manifest.SourceSpan) []manifest.NamedSpan {
	out := make([]manifest.NamedSpan, 0, len(values))
	for _, value := range values {
		out = append(out, manifest.NamedSpan{Name: value, Span: fallback})
	}
	return out
}

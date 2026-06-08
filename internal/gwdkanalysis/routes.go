package gwdkanalysis

import (
	"sort"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func routeParams(route string) []string {
	params := manifest.RouteParamsFromPath(route)
	out := make([]string, 0, len(params))
	seen := map[string]bool{}
	for _, param := range params {
		if !seen[param.Name] {
			seen[param.Name] = true
			out = append(out, param.Name)
		}
	}
	sort.Strings(out)
	return out
}

func typedRouteParams(route string) []manifest.RouteParam {
	params := manifest.RouteParamsFromPath(route)
	out := make([]manifest.RouteParam, 0, len(params))
	seen := map[string]bool{}
	for _, param := range params {
		if seen[param.Name] {
			continue
		}
		seen[param.Name] = true
		out = append(out, param)
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

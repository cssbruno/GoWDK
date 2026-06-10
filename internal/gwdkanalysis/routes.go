package gwdkanalysis

import (
	"sort"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func routeParams(route string) []string {
	params := gwdkir.RouteParamsFromPath(route)
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

func typedRouteParams(route string) []source.RouteParam {
	params := gwdkir.RouteParamsFromPath(route)
	out := make([]source.RouteParam, 0, len(params))
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

func routeParamSpans(route string, fallback source.SourceSpan) []source.NamedSpan {
	params := routeParams(route)
	out := make([]source.NamedSpan, 0, len(params))
	for _, param := range params {
		out = append(out, source.NamedSpan{Name: param, Span: fallback})
	}
	return out
}

func namedSpans(values []string, fallback source.SourceSpan) []source.NamedSpan {
	out := make([]source.NamedSpan, 0, len(values))
	for _, value := range values {
		out = append(out, source.NamedSpan{Name: value, Span: fallback})
	}
	return out
}

package gwdkanalysis

import (
	"sort"

	"github.com/cssbruno/gowdk/internal/gwdkir"
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

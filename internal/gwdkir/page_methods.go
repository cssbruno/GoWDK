package gwdkir

import (
	"sort"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/source"
)

// These methods mirror the behavior previously provided by manifest.Page so
// generated-output packages can consume the IR page model directly instead of a
// reconstructed manifest. They read IR fields only and depend on no other model.

// CachePolicy returns the concrete Cache-Control policy generated for the page.
func (page Page) CachePolicy() string {
	return CachePolicyWithRevalidate(page.Cache, page.Revalidate)
}

// CachePolicyWithRevalidate appends the page revalidation directive to an
// explicit Cache-Control policy.
func CachePolicyWithRevalidate(cache string, revalidate string) string {
	if cache == "" || revalidate == "" {
		return cache
	}
	return cache + ", stale-while-revalidate=" + revalidate
}

// RenderMode resolves the effective render mode for the page, defaulting to SSR
// when the page declares request-time behavior and otherwise to defaultMode
// (SPA when unset).
func (page Page) RenderMode(defaultMode gowdk.RenderMode) gowdk.RenderMode {
	if page.Render != "" {
		return page.Render
	}
	if page.Blocks.Load || page.HasGoBlock("ssr") {
		return gowdk.SSR
	}
	if defaultMode == "" {
		return gowdk.SPA
	}
	return defaultMode
}

// HasGoBlock reports whether the page declares a go block for target.
func (page Page) HasGoBlock(target string) bool {
	for _, block := range page.Blocks.GoBlocks {
		if block.Target == target {
			return true
		}
	}
	return false
}

// DynamicParams returns route parameters declared with /path/{param} syntax.
func (page Page) DynamicParams() []string {
	params := page.TypedRouteParams()
	if len(params) == 0 {
		return nil
	}
	names := make([]string, 0, len(params))
	for _, param := range params {
		names = append(names, param.Name)
	}
	sort.Strings(names)
	return names
}

// TypedRouteParams returns route params with explicit type metadata. Untyped
// params are reported as string. Declared params are merged with params parsed
// from the route path so trailing rest params such as {path...}, which lowering
// does not extract, are still reported.
func (page Page) TypedRouteParams() []source.RouteParam {
	out := make([]source.RouteParam, 0, len(page.RouteParams))
	seen := map[string]bool{}
	for _, param := range page.RouteParams {
		if param.Name == "" || seen[param.Name] {
			continue
		}
		seen[param.Name] = true
		if param.Type == "" {
			param.Type = "string"
		}
		out = append(out, param)
	}
	for _, param := range RouteParamsFromPath(page.Route) {
		if seen[param.Name] {
			continue
		}
		seen[param.Name] = true
		if param.Type == "" {
			param.Type = "string"
		}
		out = append(out, param)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// RouteParamsFromPath parses dynamic route parameters from a route pattern of
// the form /path/{name}, /path/{name:type}, or /path/{name...}. Rest params
// are always string-typed.
func RouteParamsFromPath(route string) []source.RouteParam {
	return source.RouteParamsFromPath(route)
}

package appgen

import (
	"sort"
	"strings"
)

func ssrHandlerSource(routes []SSRRoute) string {
	if len(routes) == 0 {
		return `func (handler staticHandler) ssrExact(response http.ResponseWriter, request *http.Request) bool {
	return false
}

func (handler staticHandler) ssrDynamic(response http.ResponseWriter, request *http.Request) bool {
	return false
}`
	}

	sorted := append([]SSRRoute(nil), routes...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Route == sorted[j].Route {
			return sorted[i].PageID < sorted[j].PageID
		}
		return sorted[i].Route < sorted[j].Route
	})

	var builder strings.Builder
	builder.WriteString("func (handler staticHandler) ssrExact(response http.ResponseWriter, request *http.Request) bool {\n")
	builder.WriteString("\tswitch request.URL.Path {\n")
	for _, route := range sorted {
		if len(ssrRoutePatternParams(route.Route)) > 0 {
			continue
		}
		builder.WriteString("\tcase ")
		builder.WriteString(quote(route.Route))
		builder.WriteString(":\n")
		builder.WriteString("\t\twriteSSRHTML(response, request, ")
		builder.WriteString(goString(route.HTML))
		builder.WriteString(")\n")
		builder.WriteString("\t\treturn true\n")
	}
	builder.WriteString("\t}\n")
	builder.WriteString("\treturn false\n")
	builder.WriteString("}\n\n")
	builder.WriteString("func (handler staticHandler) ssrDynamic(response http.ResponseWriter, request *http.Request) bool {\n")
	for _, route := range sorted {
		if len(ssrRoutePatternParams(route.Route)) == 0 {
			continue
		}
		builder.WriteString("\tif params, ok := matchSSRRoute(")
		builder.WriteString(quote(route.Route))
		builder.WriteString(", request.URL.Path); ok {\n")
		builder.WriteString("\t\thtml := ")
		builder.WriteString(goString(route.HTML))
		builder.WriteString("\n")
		for _, replacement := range route.Replacements {
			builder.WriteString("\t\thtml = strings.ReplaceAll(html, ")
			builder.WriteString(goString(replacement.Placeholder))
			builder.WriteString(", escapeSSRValue(params[")
			builder.WriteString(goString(replacement.Param))
			builder.WriteString("]))\n")
		}
		builder.WriteString("\t\twriteSSRHTML(response, request, html)\n")
		builder.WriteString("\t\treturn true\n")
		builder.WriteString("\t}\n")
	}
	builder.WriteString("\treturn false\n")
	builder.WriteString("}")
	return builder.String()
}

package appgen

import (
	"sort"
	"strings"
)

func appPackageSource(actions []ActionRoute, ssr []SSRRoute) string {
	source := strings.ReplaceAll(appPackageSourceTemplate, "{{RUNTIME_IMPORTS}}", runtimeImportSource(actions, ssr))
	source = strings.ReplaceAll(source, "{{ACTION_HANDLER}}", actionHandlerSource(actions))
	source = strings.ReplaceAll(source, "{{SSR_HANDLER}}", ssrHandlerSource(ssr))
	return source
}

func runtimeImportSource(actions []ActionRoute, ssr []SSRRoute) string {
	imports := map[string]string{
		"gowdkruntime": "github.com/cssbruno/gowdk/runtime/app",
	}
	if len(actions) > 0 {
		imports["gowdkform"] = "github.com/cssbruno/gowdk/runtime/form"
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
		imports["path"] = "path"
		imports["strings"] = "strings"
	}
	if actionsUseValidation(actions) {
		imports["gowdkvalidation"] = "github.com/cssbruno/gowdk/runtime/validation"
	}
	if len(ssr) > 0 {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
	}
	if ssrUsesDynamicRoutes(ssr) {
		imports["gowdkroute"] = "github.com/cssbruno/gowdk/runtime/route"
	}
	if ssrUsesReplacements(ssr) {
		imports["gowdkhtml"] = "github.com/cssbruno/gowdk/runtime/html"
		imports["strings"] = "strings"
	}

	aliases := make([]string, 0, len(imports))
	for alias := range imports {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	var builder strings.Builder
	for _, alias := range aliases {
		builder.WriteString("\n\t")
		if alias != imports[alias] {
			builder.WriteString(alias)
			builder.WriteByte(' ')
		}
		builder.WriteString("\"")
		builder.WriteString(imports[alias])
		builder.WriteString("\"")
	}
	return builder.String()
}

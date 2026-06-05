package appgen

import (
	"sort"
	"strings"
)

func appPackageSource(options Options) string {
	direct := options
	if options.ProxyBackend {
		direct.Actions = nil
		direct.APIs = nil
	}
	source := strings.ReplaceAll(appPackageSourceTemplate, "{{RUNTIME_IMPORTS}}", runtimeImportSource(options))
	source = strings.ReplaceAll(source, "{{ACTION_CALLBACK}}", actionCallbackName(options))
	source = strings.ReplaceAll(source, "{{API_CALLBACK}}", apiCallbackName(options))
	source = strings.ReplaceAll(source, "{{ACTION_HANDLER}}", actionHandlerSource(direct.Actions))
	source = strings.ReplaceAll(source, "{{API_HANDLER}}", apiHandlerSource(direct.APIs))
	source = strings.ReplaceAll(source, "{{BACKEND_PROXY}}", backendProxySource(options))
	source = strings.ReplaceAll(source, "{{SSR_HANDLER}}", ssrHandlerSource(options.SSR))
	return source
}

func runtimeImportSource(options Options) string {
	imports := map[string]string{
		"gowdkruntime": "github.com/cssbruno/gowdk/runtime/app",
	}
	actions := options.Actions
	apis := options.APIs
	if options.ProxyBackend {
		actions = nil
		apis = nil
	}
	ssr := options.SSR
	if len(actions) > 0 {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
		imports["path"] = "path"
	}
	if actionsUseForm(actions) {
		imports["gowdkform"] = "github.com/cssbruno/gowdk/runtime/form"
		imports["strings"] = "strings"
	}
	if len(apis) > 0 {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
		imports["path"] = "path"
	}
	if options.ProxyBackend {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
		imports["neturl"] = "net/url"
		imports["os"] = "os"
		imports["httputil"] = "net/http/httputil"
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
	if !options.ProxyBackend {
		for importPath, alias := range backendImports(actions, apis) {
			imports[alias] = importPath
		}
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

func actionCallbackName(options Options) string {
	if options.ProxyBackend && hasBackendRoutes(options) {
		return "backendProxy"
	}
	return "action"
}

func apiCallbackName(options Options) string {
	if options.ProxyBackend && hasBackendRoutes(options) {
		return "backendProxy"
	}
	return "api"
}

func hasBackendRoutes(options Options) bool {
	return len(options.Actions) > 0 || len(options.APIs) > 0
}

func backendImports(actions []ActionRoute, apis []APIRoute) map[string]string {
	imports := map[string]string{}
	for _, action := range actions {
		if action.Binding.ImportPath != "" && action.BackendAlias != "" {
			imports[action.Binding.ImportPath] = action.BackendAlias
		}
	}
	for _, api := range apis {
		if api.Binding.ImportPath != "" && api.BackendAlias != "" {
			imports[api.Binding.ImportPath] = api.BackendAlias
		}
	}
	return imports
}

package appgen

import (
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func apiHandlerSource(apis []APIRoute) string {
	if len(apis) == 0 {
		return emptyAPIHandlerSource
	}

	var builder strings.Builder
	builder.WriteString("func api(response http.ResponseWriter, request *http.Request) bool {\n")
	builder.WriteString("\trequestPath := path.Clean(\"/\" + request.URL.Path)\n")
	builder.WriteString("\tswitch {\n")
	for _, api := range sortedAPIRoutes(apis) {
		builder.WriteString("\tcase request.Method == ")
		builder.WriteString(goString(api.Method))
		builder.WriteString(" && requestPath == ")
		builder.WriteString(quote(api.Route))
		builder.WriteString(":\n")
		if api.Binding.Status != manifest.BackendBindingBound {
			writeBackendNotImplemented(&builder, api.Binding, "API")
			builder.WriteString("\t\treturn true\n")
			continue
		}
		builder.WriteString("\t\tresult, err := ")
		builder.WriteString(api.BackendAlias)
		builder.WriteString(".")
		builder.WriteString(api.Binding.FunctionName)
		builder.WriteString("(request.Context(), request)\n")
		builder.WriteString("\t\tif err != nil {\n")
		builder.WriteString("\t\t\tgowdkresponse.WriteNoStoreError(response, gowdkresponse.HandlerStatus(err, http.StatusInternalServerError), err.Error())\n")
		builder.WriteString("\t\t\treturn true\n")
		builder.WriteString("\t\t}\n")
		builder.WriteString("\t\t_ = gowdkresponse.WriteNoStoreHTTP(response, result)\n")
		builder.WriteString("\t\treturn true\n")
	}
	builder.WriteString("\tdefault:\n")
	builder.WriteString("\t\treturn false\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n")
	return builder.String()
}

const emptyAPIHandlerSource = `func api(response http.ResponseWriter, request *http.Request) bool {
	return false
}`

func sortedAPIRoutes(apis []APIRoute) []APIRoute {
	sorted := append([]APIRoute(nil), apis...)
	sortAPIRoutes(sorted)
	return sorted
}

func sortAPIRoutes(apis []APIRoute) {
	sort.Slice(apis, func(i, j int) bool {
		if apis[i].Route == apis[j].Route {
			if apis[i].Method == apis[j].Method {
				return apis[i].APIName < apis[j].APIName
			}
			return apis[i].Method < apis[j].Method
		}
		return apis[i].Route < apis[j].Route
	})
}

func writeBackendNotImplemented(builder *strings.Builder, binding manifest.BackendBinding, kind string) {
	message := strings.TrimSpace(binding.Message)
	if message == "" {
		message = "GOWDK " + kind + " handler is not implemented"
	}
	builder.WriteString("\t\tgowdkresponse.WriteNoStoreError(response, http.StatusNotImplemented, ")
	builder.WriteString(goString(message))
	builder.WriteString(")\n")
}

func backendProxySource(options Options) string {
	if !options.ProxyBackend || !hasBackendRoutes(options) {
		return ""
	}
	var builder strings.Builder
	builder.WriteString("func backendProxy(response http.ResponseWriter, request *http.Request) bool {\n")
	builder.WriteString("\tif !isBackendRoute(request.Method, request.URL.Path) {\n")
	builder.WriteString("\t\treturn false\n")
	builder.WriteString("\t}\n")
	builder.WriteString("\torigin := strings.TrimSpace(os.Getenv(\"GOWDK_BACKEND_ORIGIN\"))\n")
	builder.WriteString("\tif origin == \"\" {\n")
	builder.WriteString("\t\tgowdkresponse.WriteNoStoreError(response, http.StatusBadGateway, \"GOWDK_BACKEND_ORIGIN is required for split backend routes\")\n")
	builder.WriteString("\t\treturn true\n")
	builder.WriteString("\t}\n")
	builder.WriteString("\ttarget, err := neturl.Parse(origin)\n")
	builder.WriteString("\tif err != nil || target.Scheme == \"\" || target.Host == \"\" {\n")
	builder.WriteString("\t\tgowdkresponse.WriteNoStoreError(response, http.StatusBadGateway, \"GOWDK_BACKEND_ORIGIN is invalid\")\n")
	builder.WriteString("\t\treturn true\n")
	builder.WriteString("\t}\n")
	builder.WriteString("\tproxy := httputil.NewSingleHostReverseProxy(target)\n")
	builder.WriteString("\tproxy.ServeHTTP(response, request)\n")
	builder.WriteString("\treturn true\n")
	builder.WriteString("}\n\n")
	builder.WriteString("func isBackendRoute(method string, requestPath string) bool {\n")
	builder.WriteString("\trequestPath = path.Clean(\"/\" + requestPath)\n")
	builder.WriteString("\tswitch {\n")
	for _, action := range sortedActionRoutes(options.Actions) {
		builder.WriteString("\tcase method == http.MethodPost && requestPath == ")
		builder.WriteString(quote(action.Route))
		builder.WriteString(":\n")
		builder.WriteString("\t\treturn true\n")
	}
	for _, api := range sortedAPIRoutes(options.APIs) {
		builder.WriteString("\tcase method == ")
		builder.WriteString(goString(api.Method))
		builder.WriteString(" && requestPath == ")
		builder.WriteString(quote(api.Route))
		builder.WriteString(":\n")
		builder.WriteString("\t\treturn true\n")
	}
	builder.WriteString("\tdefault:\n")
	builder.WriteString("\t\treturn false\n")
	builder.WriteString("\t}\n")
	builder.WriteString("}\n")
	return builder.String()
}

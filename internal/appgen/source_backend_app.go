package appgen

import (
	"sort"
	"strings"
)

const backendAppPackageSourceTemplate = `package gowdkapp

import (
	"net/http"
{{RUNTIME_IMPORTS}}
)

const maxActionBodyBytes int64 = 1 << 20

func Handler() (http.Handler, error) {
	return ServeMux()
}

func ServeMux() (*http.ServeMux, error) {
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if api(response, request) {
			return
		}
		if request.Method == http.MethodPost && action(response, request) {
			return
		}
		http.NotFound(response, request)
	}))
	return mux, nil
}

{{ACTION_HANDLER}}

{{API_HANDLER}}
`

func backendAppPackageSource(options Options) string {
	source := strings.ReplaceAll(backendAppPackageSourceTemplate, "{{RUNTIME_IMPORTS}}", backendRuntimeImportSource(options))
	source = strings.ReplaceAll(source, "{{ACTION_HANDLER}}", actionHandlerSource(options.Actions))
	source = strings.ReplaceAll(source, "{{API_HANDLER}}", apiHandlerSource(options.APIs))
	return source
}

func backendRuntimeImportSource(options Options) string {
	imports := map[string]string{}
	if len(options.Actions) > 0 || len(options.APIs) > 0 {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
	}
	if len(options.Actions) > 0 {
		imports["path"] = "path"
	}
	if actionsUseForm(options.Actions) {
		imports["gowdkform"] = "github.com/cssbruno/gowdk/runtime/form"
		imports["strings"] = "strings"
	}
	if len(options.APIs) > 0 {
		imports["path"] = "path"
	}
	if actionsUseValidation(options.Actions) {
		imports["gowdkvalidation"] = "github.com/cssbruno/gowdk/runtime/validation"
	}
	for importPath, alias := range backendImports(options.Actions, options.APIs) {
		imports[alias] = importPath
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

package appgen

import (
	"strings"
)

// Temporary generated-Go template exception: the backend-only app shell stays a
// raw template until it shares the same AST file builder as the embedded app
// shell. Do not add generated route, action, API, SSR, or decoder bodies here.
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
{{CSRF_SETUP}}
	mux := http.NewServeMux()
	mux.Handle("/", http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if backend(response, request) {
			return
		}
		http.NotFound(response, request)
	}))
	return mux, nil
}

{{ACTION_HANDLER}}

{{API_HANDLER}}

{{BACKEND_HANDLER}}

{{CSRF_HELPER}}
`

func backendAppPackageSource(options Options) string {
	source := strings.ReplaceAll(backendAppPackageSourceTemplate, "{{RUNTIME_IMPORTS}}", backendRuntimeImportSource(options))
	source = strings.ReplaceAll(source, "{{CSRF_SETUP}}", csrfSetupSource(options))
	source = strings.ReplaceAll(source, "{{ACTION_HANDLER}}", actionHandlerSource(options.Actions, csrfEnabled(options)))
	source = strings.ReplaceAll(source, "{{API_HANDLER}}", apiHandlerSource(options.APIs))
	source = strings.ReplaceAll(source, "{{BACKEND_HANDLER}}", backendHandlerSource(options.Actions, options.APIs))
	source = strings.ReplaceAll(source, "{{CSRF_HELPER}}", csrfHelperSource(options))
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
	if actionsParseForm(options.Actions) {
		imports["strings"] = "strings"
	}
	if actionsUseForm(options.Actions) {
		imports["gowdkform"] = "github.com/cssbruno/gowdk/runtime/form"
	}
	if len(options.APIs) > 0 {
		imports["path"] = "path"
	}
	if actionsUseValidation(options.Actions) {
		imports["gowdkvalidation"] = "github.com/cssbruno/gowdk/runtime/validation"
	}
	if csrfEnabled(options) {
		imports["errors"] = "errors"
		imports["gowdkactions"] = "github.com/cssbruno/gowdk/addons/actions"
		imports["os"] = "os"
		imports["strings"] = "strings"
	}
	for importPath, alias := range backendImports(options.Actions, options.APIs) {
		imports[alias] = importPath
	}

	return importSpecSource(imports)
}

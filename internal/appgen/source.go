package appgen

import (
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
)

func appPackageSource(options Options) string {
	direct := options
	if options.ProxyBackend {
		direct.Actions = nil
		direct.APIs = nil
	}
	source := strings.ReplaceAll(appPackageSourceTemplate, "{{RUNTIME_IMPORTS}}", runtimeImportSource(options))
	source = strings.ReplaceAll(source, "{{CSRF_SETUP}}", csrfSetupSource(options))
	source = strings.ReplaceAll(source, "{{CSRF_HANDLER_FIELD}}", csrfHandlerFieldSource(options))
	source = strings.ReplaceAll(source, "{{BACKEND_CALLBACK}}", backendCallbackName(options))
	source = strings.ReplaceAll(source, "{{ACTION_HANDLER}}", actionHandlerSource(direct.Actions, csrfEnabled(direct)))
	source = strings.ReplaceAll(source, "{{API_HANDLER}}", apiHandlerSource(direct.APIs))
	source = strings.ReplaceAll(source, "{{BACKEND_HANDLER}}", backendHandlerSource(direct.Actions, direct.APIs))
	source = strings.ReplaceAll(source, "{{BACKEND_PROXY}}", backendProxySource(options))
	source = strings.ReplaceAll(source, "{{CSRF_HELPER}}", csrfHelperSource(options))
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
	if actionsParseForm(actions) {
		imports["strings"] = "strings"
	}
	if actionsUseForm(actions) {
		imports["gowdkform"] = "github.com/cssbruno/gowdk/runtime/form"
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
	if csrfEnabled(options) {
		imports["errors"] = "errors"
		imports["gowdkactions"] = "github.com/cssbruno/gowdk/addons/actions"
		imports["os"] = "os"
		imports["strings"] = "strings"
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

	return importSpecSource(imports)
}

// Temporary generated-Go template exception: import specs remain text while the
// app shell owns the import block as a raw template. Replace this with AST
// import declarations when the shell templates are retired.
func importSpecSource(imports map[string]string) string {
	if len(imports) == 0 {
		return ""
	}
	aliases := make([]string, 0, len(imports))
	for alias := range imports {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	specs := make([]string, 0, len(aliases))
	for _, alias := range aliases {
		spec := "\t"
		if alias != imports[alias] {
			spec += alias + " "
		}
		spec += strconv.Quote(imports[alias])
		specs = append(specs, spec)
	}
	return "\n" + strings.Join(specs, "\n")
}

func csrfEnabled(options Options) bool {
	return options.Config.Build.CSRF.Enabled && len(options.Actions) > 0
}

// Temporary generated-Go template exception: this statement snippet belongs to
// the raw app-shell templates. Move it into the ServeMux AST builder when the
// shell templates are retired.
func csrfSetupSource(options Options) string {
	if !csrfEnabled(options) {
		return ""
	}
	return "\tcsrfTokenSource, err := newCSRF()\n\tif err != nil {\n\t\treturn nil, err\n\t}\n\tcsrfValidator = csrfTokenSource\n"
}

// Temporary generated-Go template exception: this composite literal field
// snippet belongs to the raw app-shell templates and should disappear with the
// app-shell AST migration.
func csrfHandlerFieldSource(options Options) string {
	if !csrfEnabled(options) {
		return ""
	}
	return "\t\tCSRF:       csrfTokenSource,\n"
}

func csrfHelperSource(options Options) string {
	if !csrfEnabled(options) {
		return ""
	}
	return printActionDecls([]ast.Decl{
		csrfValidatorVarDecl(),
		csrfNewFuncDecl(options.Config.Build.CSRF),
	})
}

func csrfValidatorVarDecl() ast.Decl {
	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{&ast.ValueSpec{
			Names: []*ast.Ident{id("csrfValidator")},
			Type:  sel("gowdkactions", "CSRFValidator"),
		}},
	}
}

func csrfNewFuncDecl(config gowdk.CSRFConfig) *ast.FuncDecl {
	secretEnv := config.SecretEnvName()
	options := []ast.Expr{keyValue("Secret", call(&ast.ArrayType{Elt: id("byte")}, id("secret")))}
	if config.CookieName != "" {
		options = append(options, keyValue("CookieName", stringLit(config.CookieName)))
	}
	if config.FieldName != "" {
		options = append(options, keyValue("FieldName", stringLit(config.FieldName)))
	}
	if config.HeaderName != "" {
		options = append(options, keyValue("HeaderName", stringLit(config.HeaderName)))
	}
	if config.Insecure {
		options = append(options, keyValue("Insecure", id("true")))
	}
	return funcDecl("newCSRF", nil, []*ast.Field{
		{Type: &ast.StarExpr{X: sel("gowdkactions", "CSRF")}},
		{Type: id("error")},
	}, []ast.Stmt{
		define([]ast.Expr{id("secret")}, call(sel("strings", "TrimSpace"), call(sel("os", "Getenv"), stringLit(secretEnv)))),
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: id("secret"), Op: token.EQL, Y: stringLit("")},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{
				id("nil"),
				call(sel("errors", "New"), stringLit(secretEnv+" is required when generated CSRF is enabled")),
			}}),
		},
		&ast.ReturnStmt{Results: []ast.Expr{call(sel("gowdkactions", "NewCSRF"), &ast.CompositeLit{
			Type: sel("gowdkactions", "CSRFOptions"),
			Elts: options,
		})}},
	})
}

func backendCallbackName(options Options) string {
	if options.ProxyBackend && hasBackendRoutes(options) {
		return "backendProxy"
	}
	return "backend"
}

func hasBackendRoutes(options Options) bool {
	return len(options.Actions) > 0 || len(options.APIs) > 0
}

func backendImports(actions []ActionEndpoint, apis []APIEndpoint) map[string]string {
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

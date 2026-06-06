package appgen

import (
	"bytes"
	"go/ast"
	"go/printer"
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
		direct.Fragments = nil
	}
	imports := runtimeImportMap(options)
	imports["embed"] = "embed"
	imports["fs"] = "io/fs"
	imports["http"] = "net/http"
	return printGoFile("gowdkapp", imports, append(appShellDecls(options), appGeneratedDecls(direct, options)...))
}

func runtimeImportSource(options Options) string {
	return importSpecSource(runtimeImportMap(options))
}

func runtimeImportMap(options Options) map[string]string {
	imports := map[string]string{
		"gowdkruntime": "github.com/cssbruno/gowdk/runtime/app",
	}
	actions := options.Actions
	apis := options.APIs
	fragments := options.Fragments
	if options.ProxyBackend {
		actions = nil
		apis = nil
		fragments = nil
	}
	ssr := options.SSR
	if len(actions) > 0 || len(fragments) > 0 {
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
	if actionsUseLengthValidation(actions) {
		imports["utf8"] = "unicode/utf8"
	}
	if actionsUsePatternValidation(actions) {
		imports["regexp"] = "regexp"
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
	if generatedUsesGuards(options) || ssrUsesLoad(ssr) {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
		imports["gowdkssr"] = "github.com/cssbruno/gowdk/addons/ssr"
	}
	if ssrUsesDynamicRoutes(ssr) {
		imports["gowdkroute"] = "github.com/cssbruno/gowdk/runtime/route"
	}
	if ssrUsesReplacements(ssr) || ssrUsesLoad(ssr) {
		imports["gowdkhtml"] = "github.com/cssbruno/gowdk/runtime/html"
		imports["strings"] = "strings"
	}
	if ssrUsesLoad(ssr) {
		imports["fmt"] = "fmt"
	}
	if generatedUsesRateLimit(options) {
		imports["gowdkratelimit"] = "github.com/cssbruno/gowdk/addons/ratelimit"
	}
	if !options.ProxyBackend {
		for importPath, alias := range backendImports(actions, apis, fragments, ssr) {
			imports[alias] = importPath
		}
	}

	return imports
}

func printGoFile(packageName string, imports map[string]string, decls []ast.Decl) string {
	file := &ast.File{Name: id(packageName)}
	if len(imports) > 0 {
		file.Decls = append(file.Decls, importDecl(imports))
	}
	file.Decls = append(file.Decls, decls...)

	var buffer bytes.Buffer
	_ = printer.Fprint(&buffer, token.NewFileSet(), file)
	return buffer.String()
}

func importDecl(imports map[string]string) ast.Decl {
	aliases := make([]string, 0, len(imports))
	for alias := range imports {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	specs := make([]ast.Spec, 0, len(aliases))
	for _, alias := range aliases {
		spec := &ast.ImportSpec{Path: stringLit(imports[alias])}
		if alias != importPackageName(imports[alias]) || forceImportAlias(imports[alias]) {
			spec.Name = id(alias)
		}
		specs = append(specs, spec)
	}
	return &ast.GenDecl{Tok: token.IMPORT, Specs: specs}
}

func importPackageName(importPath string) string {
	if index := strings.LastIndex(importPath, "/"); index >= 0 {
		return importPath[index+1:]
	}
	return importPath
}

func forceImportAlias(importPath string) bool {
	first := importPath
	if index := strings.Index(importPath, "/"); index >= 0 {
		first = importPath[:index]
	}
	return strings.Contains(first, ".")
}

func appShellDecls(options Options) []ast.Decl {
	decls := []ast.Decl{
		maxActionBodyBytesDecl(),
		embeddedFilesDecl(),
		handlerDecl(),
		serveMuxDecl(options, true),
	}
	return decls
}

func backendShellDecls(options Options) []ast.Decl {
	return []ast.Decl{
		maxActionBodyBytesDecl(),
		handlerDecl(),
		serveMuxDecl(options, false),
	}
}

func appGeneratedDecls(direct Options, full Options) []ast.Decl {
	adapter := backendAdapterIR(direct)
	decls := actionHandlerDecls(direct.Actions, csrfEnabled(direct), generatedUsesRateLimit(direct))
	decls = append(decls, apiFuncDecl(sortedAPIEndpoints(direct.APIs), generatedUsesRateLimit(direct)))
	decls = append(decls, fragmentFuncDecl(direct.Fragments, generatedUsesRateLimit(direct)))
	switch {
	case adapter.HasRegistrations():
		decls = append(decls, newBackendRouterDecl(adapter))
	case !full.ProxyBackend || !hasBackendRoutes(full):
		decls = append(decls, emptyBackendHandlerDecl())
	}
	if full.ProxyBackend {
		decls = append(decls, backendProxyDecl(generatedUsesRateLimit(full)), isBackendRouteDecl(full))
	}
	if csrfEnabled(direct) {
		decls = append(decls, csrfValidatorVarDecl(), csrfNewFuncDecl(direct.Config.Build.CSRF))
	}
	decls = append(decls, rateLimitDecls(full)...)
	decls = append(decls, guardDecls(full)...)
	decls = append(decls, ssrExactDecl(full.SSR, generatedUsesRateLimit(full)), ssrDynamicDecl(full.SSR, generatedUsesRateLimit(full)))
	return decls
}

func backendGeneratedDecls(options Options) []ast.Decl {
	adapter := backendAdapterIR(options)
	decls := actionHandlerDecls(options.Actions, csrfEnabled(options), generatedUsesRateLimit(options))
	decls = append(decls, apiFuncDecl(sortedAPIEndpoints(options.APIs), generatedUsesRateLimit(options)))
	decls = append(decls, fragmentFuncDecl(options.Fragments, generatedUsesRateLimit(options)))
	if adapter.HasRegistrations() {
		decls = append(decls, newBackendRouterDecl(adapter))
	} else {
		decls = append(decls, emptyBackendHandlerDecl())
	}
	if csrfEnabled(options) {
		decls = append(decls, csrfValidatorVarDecl(), csrfNewFuncDecl(options.Config.Build.CSRF))
	}
	decls = append(decls, rateLimitDecls(options)...)
	decls = append(decls, guardDecls(options)...)
	return decls
}

func actionHandlerDecls(actions []ActionEndpoint, csrf bool, rateLimit bool) []ast.Decl {
	sorted := sortedActionEndpoints(actions)
	decls := []ast.Decl{actionFuncDecl(sorted, csrf, rateLimit)}
	if len(sorted) > 0 {
		decls = append(decls, actionRequestPathDecl())
		decls = append(decls, actionDecoderDecls(sorted)...)
	}
	return decls
}

func maxActionBodyBytesDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.CONST, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("maxActionBodyBytes")},
		Type:  id("int64"),
		Values: []ast.Expr{&ast.BinaryExpr{
			X:  intLit(1),
			Op: token.SHL,
			Y:  intLit(20),
		}},
	}}}
}

func embeddedFilesDecl() ast.Decl {
	return &ast.GenDecl{
		Doc: &ast.CommentGroup{List: []*ast.Comment{{Text: "//go:embed app"}}},
		Tok: token.VAR,
		Specs: []ast.Spec{&ast.ValueSpec{
			Names: []*ast.Ident{id("embeddedFiles")},
			Type:  sel("embed", "FS"),
		}},
	}
}

func handlerDecl() ast.Decl {
	return funcDecl("Handler", nil, []*ast.Field{
		{Type: sel("http", "Handler")},
		{Type: id("error")},
	}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{call(sel("ServeMux"))}},
	})
}

func serveMuxDecl(options Options, embedded bool) ast.Decl {
	stmts := []ast.Stmt{}
	if embedded {
		stmts = append(stmts,
			define([]ast.Expr{id("root"), id("err")}, call(sel("fs", "Sub"), id("embeddedFiles"), stringLit("app"))),
			&ast.IfStmt{
				Cond: notNil("err"),
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
			},
		)
	}
	stmts = append(stmts, csrfSetupStmts(options)...)
	if needsBackendRouter(options, embedded) {
		stmts = append(stmts,
			define([]ast.Expr{id("backendRouter"), id("err")}, call(sel("newBackendRouter"))),
			&ast.IfStmt{
				Cond: notNil("err"),
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
			},
		)
	}
	stmts = append(stmts, define([]ast.Expr{id("mux")}, call(sel("http", "NewServeMux"))))
	if embedded {
		stmts = append(stmts, exprStmt(call(selExpr(id("mux"), "Handle"), stringLit("/"), &ast.CompositeLit{
			Type: sel("gowdkruntime", "Handler"),
			Elts: embeddedHandlerFields(options),
		})))
	} else {
		stmts = append(stmts, exprStmt(call(selExpr(id("mux"), "Handle"), stringLit("/"), backendOnlyHandlerExpr(options))))
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("mux"), id("nil")}})
	return funcDecl("ServeMux", nil, []*ast.Field{
		{Type: &ast.StarExpr{X: sel("http", "ServeMux")}},
		{Type: id("error")},
	}, stmts)
}

func needsBackendRouter(options Options, embedded bool) bool {
	if !hasBackendRoutes(options) {
		return false
	}
	return !embedded || !options.ProxyBackend
}

func csrfSetupStmts(options Options) []ast.Stmt {
	if !csrfEnabled(options) {
		return nil
	}
	return []ast.Stmt{
		define([]ast.Expr{id("csrfTokenSource"), id("err")}, call(sel("newCSRF"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
		},
		assign([]ast.Expr{id("csrfValidator")}, id("csrfTokenSource")),
	}
}

func embeddedHandlerFields(options Options) []ast.Expr {
	backend := ast.Expr(id(backendCallbackName(options)))
	if hasBackendRoutes(options) && !options.ProxyBackend {
		backend = call(selExpr(id("backendRouter"), "HandlerFunc"))
	}
	fields := []ast.Expr{
		keyValue("Root", id("root")),
		keyValue("Identity", call(sel("gowdkruntime", "InstanceIdentity"))),
		keyValue("Assets", call(sel("gowdkruntime", "LoadAssetManifest"), id("root"))),
		keyValue("ErrorPages", call(sel("gowdkruntime", "LoadErrorPages"), id("root"))),
		keyValue("Backend", backend),
	}
	if csrfEnabled(options) {
		fields = append(fields, keyValue("CSRF", id("csrfTokenSource")))
	}
	fields = append(fields,
		keyValue("SSRExact", id("ssrExact")),
		keyValue("SSRDynamic", id("ssrDynamic")),
	)
	return fields
}

func backendOnlyHandlerExpr(options Options) ast.Expr {
	if hasBackendRoutes(options) {
		return id("backendRouter")
	}
	return call(sel("http", "HandlerFunc"), backendOnlyHandlerFunc())
}

func backendOnlyHandlerFunc() ast.Expr {
	return &ast.FuncLit{
		Type: &ast.FuncType{Params: &ast.FieldList{List: actionParams()}},
		Body: block(
			&ast.IfStmt{
				Cond: call(sel("backend"), id("response"), id("request")),
				Body: block(&ast.ReturnStmt{}),
			},
			exprStmt(call(sel("http", "NotFound"), id("response"), id("request"))),
		),
	}
}

// importSpecSource is retained for narrow tests and package-level helpers. New
// generated Go files use importDecl through the AST file builder.
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
	return len(options.Actions) > 0 || len(options.APIs) > 0 || len(options.Fragments) > 0
}

func backendImports(actions []ActionEndpoint, apis []APIEndpoint, fragments []FragmentEndpoint, ssr []SSRRoute) map[string]string {
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
	for _, fragment := range fragments {
		if fragment.Binding.ImportPath != "" && fragment.BackendAlias != "" {
			imports[fragment.Binding.ImportPath] = fragment.BackendAlias
		}
	}
	for _, route := range ssr {
		if route.LoadBinding.ImportPath != "" && route.LoadBackendAlias != "" {
			imports[route.LoadBinding.ImportPath] = route.LoadBackendAlias
		}
	}
	return imports
}

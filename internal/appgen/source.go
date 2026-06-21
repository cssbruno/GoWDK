package appgen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/printer"
	"go/token"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func appPackageSource(options Options) (source string, err error) {
	defer recoverGeneratedIdentifierError(&err)

	direct := options
	if options.ProxyBackend {
		direct.Actions = nil
		direct.APIs = nil
		direct.Fragments = nil
		direct.IR = nil
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
		"sync":         "sync",
	}
	if generatedEnvFileLoadRequired(options) {
		imports["gowdkenvfile"] = "github.com/cssbruno/gowdk/runtime/envfile"
		imports["os"] = "os"
		imports["strings"] = "strings"
	}
	if generatedUsesAuthAddon(options) {
		imports["gowdkauthaddon"] = "github.com/cssbruno/gowdk/addons/auth"
	}
	if envRuntimeValidationRequired(options.Config.Env) {
		imports["errors"] = "errors"
		imports["os"] = "os"
		imports["strings"] = "strings"
	}
	adapter := backendAdapterIR(options)
	actions := adapter.Actions
	apis := adapter.APIs
	fragments := adapter.Fragments
	contractExposures := adapter.ContractExposures
	routableContracts := routableContractExposures(contractExposures)
	executableContracts := executableContractExposures(contractExposures)
	if options.ProxyBackend {
		actions = nil
		apis = nil
		fragments = nil
		routableContracts = nil
		executableContracts = nil
	}
	ssr := options.SSR
	if len(actions) > 0 || len(fragments) > 0 {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
	}
	if len(actions) > 0 || fragmentsUseExactRoutes(fragments) {
		imports["path"] = "path"
	}
	if fragmentsUseDynamicRoutes(fragments) {
		imports["gowdkroute"] = "github.com/cssbruno/gowdk/runtime/route"
	}
	if actionsUseFragments(actions) || fragmentsUseStaticFallback(fragments) {
		imports["gowdkpartial"] = "github.com/cssbruno/gowdk/runtime/partial"
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
	if len(routableContracts) > 0 {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
	}
	if len(executableContracts) > 0 {
		imports["context"] = "context"
		imports["gowdkcontracts"] = "github.com/cssbruno/gowdk/runtime/contracts"
	}
	if generatedRealtimeEnabled(options) {
		imports["context"] = "context"
		imports["gowdkcontracts"] = "github.com/cssbruno/gowdk/runtime/contracts"
		imports["gowdkrealtime"] = "github.com/cssbruno/gowdk/runtime/realtime"
	}
	if generatedObservabilityEnabled(options) {
		imports["gowdktrace"] = "github.com/cssbruno/gowdk/runtime/trace"
	}
	if generatedRealtimeStreamUsesRouteMatching(options) {
		imports["gowdkroute"] = "github.com/cssbruno/gowdk/runtime/route"
		imports["neturl"] = "net/url"
	}
	if len(executableCommandContractExposures(executableContracts)) > 0 {
		imports["sync"] = "sync"
	}
	if contractExposuresUseForm(executableContracts) {
		imports["gowdkform"] = "github.com/cssbruno/gowdk/runtime/form"
	}
	if contractExposuresParseForm(executableContracts) {
		imports["strings"] = "strings"
	}
	if options.ProxyBackend {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
		if adapter.HasDynamicRoutes() {
			imports["gowdkroute"] = "github.com/cssbruno/gowdk/runtime/route"
		}
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
	if csrfEnabled(options) {
		imports["errors"] = "errors"
		imports["gowdkactions"] = "github.com/cssbruno/gowdk/runtime/actions"
		imports["os"] = "os"
		imports["strings"] = "strings"
	}
	if len(ssr) > 0 {
		imports["gowdkresponse"] = "github.com/cssbruno/gowdk/runtime/response"
	}
	if generatedUsesGuards(options) {
		imports["gowdkguard"] = "github.com/cssbruno/gowdk/runtime/guard"
	}
	if ssrUsesLoad(ssr) {
		imports["gowdkssr"] = "github.com/cssbruno/gowdk/runtime/ssr"
	}
	if commandPatchRenderingEnabled(options) {
		imports["gowdkssr"] = "github.com/cssbruno/gowdk/runtime/ssr"
	}
	if generatedUsesGuards(options) {
		imports["gowdkauth"] = "github.com/cssbruno/gowdk/runtime/auth"
	}
	if ssrUsesDynamicRoutes(ssr) {
		imports["gowdkroute"] = "github.com/cssbruno/gowdk/runtime/route"
	}
	if ssrUsesReplacements(ssr) || ssrUsesLoadReplacements(ssr) {
		imports["gowdkhtml"] = "github.com/cssbruno/gowdk/runtime/html"
		imports["strings"] = "strings"
	}
	if ssrUsesLoadReplacements(ssr) {
		imports["fmt"] = "fmt"
	}
	if generatedUsesRateLimit(options) {
		imports["gowdkratelimit"] = "github.com/cssbruno/gowdk/runtime/ratelimit"
	}
	if !options.ProxyBackend {
		for importPath, alias := range backendImports(adapter, ssr) {
			imports[alias] = importPath
		}
		for importPath, alias := range backendContractImports(executableContracts) {
			imports[alias] = importPath
		}
	}

	return imports
}

func printGoFile(packageName string, imports map[string]string, decls []ast.Decl) (string, error) {
	file := &ast.File{Name: id(packageName)}
	if len(imports) > 0 {
		file.Decls = append(file.Decls, importDecl(imports))
	}
	file.Decls = append(file.Decls, decls...)

	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, token.NewFileSet(), file); err != nil {
		return "", fmt.Errorf("print generated %s package: %w", packageName, err)
	}
	return formatGoSource(buffer.Bytes())
}

// formatGoSource gofmt-formats generated Go source. A formatting failure means
// the generator emitted syntactically invalid code; it is returned rather than
// silently writing the broken source, which would otherwise surface much later
// as a confusing `go build` error in the generated app.
func formatGoSource(source []byte) (string, error) {
	formatted, err := format.Source(source)
	if err != nil {
		return "", fmt.Errorf("format generated Go source: %w", err)
	}
	return string(formatted), nil
}

func formatGoDeclSnippet(source string) (string, error) {
	const packagePrefix = "package gowdkapp\n\n"
	formatted, err := formatGoSource([]byte(packagePrefix + source))
	if err != nil {
		return "", err
	}
	if strings.HasPrefix(formatted, packagePrefix) {
		return strings.TrimSuffix(strings.TrimPrefix(formatted, packagePrefix), "\n"), nil
	}
	return source, nil
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
	providers := lifecycleServiceProviders(options)
	decls := []ast.Decl{
		maxActionBodyBytesDecl(options),
		maxAPIBodyBytesDecl(options),
		embeddedFilesDecl(),
	}
	decls = append(decls, middlewareDecls()...)
	decls = append(decls,
		appDecl(options),
		handlerDecl(),
		newServeMuxDecl(options, true),
		serveMuxDecl(options, true),
	)
	if len(providers) == 0 {
		decls = append(decls, configuredServicesDecl(nil))
	}
	decls = append(decls, loadEnvFileDecl(options)...)
	decls = append(decls, applyEnvDefaultsDecl(options.Config.Env)...)
	decls = append(decls, validateEnvContractDecl(options.Config.Env)...)
	return decls
}

func backendShellDecls(options Options) []ast.Decl {
	providers := lifecycleServiceProviders(options)
	decls := []ast.Decl{
		maxActionBodyBytesDecl(options),
		maxAPIBodyBytesDecl(options),
	}
	decls = append(decls, middlewareDecls()...)
	decls = append(decls,
		appDecl(options),
		handlerDecl(),
		newServeMuxDecl(options, false),
		serveMuxDecl(options, false),
	)
	if len(providers) == 0 {
		decls = append(decls, configuredServicesDecl(nil))
	}
	decls = append(decls, loadEnvFileDecl(options)...)
	decls = append(decls, applyEnvDefaultsDecl(options.Config.Env)...)
	decls = append(decls, validateEnvContractDecl(options.Config.Env)...)
	return decls
}

func appGeneratedDecls(direct Options, full Options) []ast.Decl {
	adapter := backendAdapterIR(direct)
	csrfOptions := direct
	if full.ProxyBackend {
		csrfOptions = full
	}
	decls := actionHandlerDecls(adapter.Actions, csrfEnabled(direct), generatedUsesRateLimit(direct))
	decls = append(decls, apiFuncDecl(adapter.APIs, csrfEnabled(direct), generatedUsesRateLimit(direct)))
	decls = append(decls, fragmentFuncDecl(adapter.Fragments, generatedUsesRateLimit(direct)))
	decls = append(decls, contractHandlerDecls(adapter.ContractExposures, csrfEnabled(direct), generatedUsesRateLimit(direct), generatedRealtimeQueryInvalidationsEnabled(direct), commandPatchRenderingEnabled(direct))...)
	decls = append(decls, contractDecoderDecls(adapter.ContractExposures)...)
	decls = append(decls, contractEventSinkDecls(adapter.ContractExposures, generatedRealtimeEnabled(direct), generatedRealtimeQueryInvalidationsEnabled(direct))...)
	decls = append(decls, contractRegistryDecls(adapter.ContractExposures)...)
	decls = append(decls, realtimeDecls(direct)...)
	decls = append(decls, observabilityDecls(full)...)
	switch {
	case adapter.HasRegistrations():
		decls = append(decls, newBackendRouterDecl(adapter, generatedObservabilityEnabled(full), corsPolicyExpr(direct)))
	case !full.ProxyBackend || !hasBackendRoutes(full):
		decls = append(decls, emptyBackendHandlerDecl())
	}
	if full.ProxyBackend {
		decls = append(decls, backendProxyDecl(generatedUsesRateLimit(full), generatedObservabilityEnabled(full)), isBackendRouteDecl(backendAdapterIR(full)))
	}
	csrf := csrfEnabled(csrfOptions)
	if csrf {
		// Gate the principal binding on full's guards, since guardDecls(full)
		// below is what declares the authProvider var the binding reads.
		decls = append(decls, csrfTokenSourceVarDecl(), csrfValidatorVarDecl(), csrfNewFuncDecl(full))
	}
	decls = append(decls, authSetupDecls(full)...)
	decls = append(decls, rateLimitDecls(full)...)
	decls = append(decls, guardDecls(full)...)
	traceSSR := generatedObservabilityEnabled(full)
	decls = append(decls, ssrExactDecl(full.SSR, generatedUsesRateLimit(full), csrf, traceSSR), ssrDynamicDecl(full.SSR, generatedUsesRateLimit(full), csrf, traceSSR))
	if generatedRealtimeQueryInvalidationsEnabled(direct) {
		decls = append(decls, ssrRegionRegistrationDecls(full.SSR)...)
	}
	return decls
}

func backendGeneratedDecls(options Options) []ast.Decl {
	adapter := backendAdapterIR(options)
	decls := actionHandlerDecls(adapter.Actions, csrfEnabled(options), generatedUsesRateLimit(options))
	decls = append(decls, apiFuncDecl(adapter.APIs, csrfEnabled(options), generatedUsesRateLimit(options)))
	decls = append(decls, fragmentFuncDecl(adapter.Fragments, generatedUsesRateLimit(options)))
	decls = append(decls, contractHandlerDecls(adapter.ContractExposures, csrfEnabled(options), generatedUsesRateLimit(options), generatedRealtimeQueryInvalidationsEnabled(options), false)...)
	decls = append(decls, contractDecoderDecls(adapter.ContractExposures)...)
	decls = append(decls, contractEventSinkDecls(adapter.ContractExposures, generatedRealtimeEnabled(options), generatedRealtimeQueryInvalidationsEnabled(options))...)
	decls = append(decls, contractRegistryDecls(adapter.ContractExposures)...)
	decls = append(decls, realtimeDecls(options)...)
	decls = append(decls, observabilityDecls(options)...)
	if adapter.HasRegistrations() {
		decls = append(decls, newBackendRouterDecl(adapter, generatedObservabilityEnabled(options), corsPolicyExpr(options)))
	} else {
		decls = append(decls, emptyBackendHandlerDecl())
	}
	if csrfEnabled(options) {
		decls = append(decls, csrfTokenSourceVarDecl(), csrfValidatorVarDecl(), csrfNewFuncDecl(options))
	}
	decls = append(decls, authSetupDecls(options)...)
	decls = append(decls, rateLimitDecls(options)...)
	decls = append(decls, guardDecls(options)...)
	return decls
}

func actionHandlerDecls(actions []BackendActionAdapter, csrf bool, rateLimit bool) []ast.Decl {
	sorted := sortedActionAdapters(actions)
	decls := []ast.Decl{actionFuncDecl(sorted, csrf, rateLimit)}
	if len(sorted) > 0 {
		decls = append(decls, actionRequestPathDecl())
		decls = append(decls, actionDecoderDecls(sorted)...)
	}
	return decls
}

func maxActionBodyBytesDecl(options Options) ast.Decl {
	return maxBodyBytesDecl("maxActionBodyBytes", options.Config.Build.BodyLimits.ActionLimitBytes())
}

func maxAPIBodyBytesDecl(options Options) ast.Decl {
	return maxBodyBytesDecl("maxAPIBodyBytes", options.Config.Build.BodyLimits.APILimitBytes())
}

func maxBodyBytesDecl(name string, bytes int64) ast.Decl {
	return &ast.GenDecl{Tok: token.CONST, Specs: []ast.Spec{&ast.ValueSpec{
		Names:  []*ast.Ident{id(name)},
		Type:   id("int64"),
		Values: []ast.Expr{bodyLimitExpr(bytes)},
	}}}
}

func bodyLimitExpr(bytes int64) ast.Expr {
	if bytes == gowdk.DefaultRequestBodyLimitBytes {
		return &ast.BinaryExpr{
			X:  intLit(1),
			Op: token.SHL,
			Y:  intLit(20),
		}
	}
	return int64Lit(bytes)
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
	return funcDecl("ServeMux", nil, []*ast.Field{
		{Type: &ast.StarExpr{X: sel("http", "ServeMux")}},
		{Type: id("error")},
	}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{call(id("newServeMux"), call(sel("gowdkruntime", "InstanceIdentity")))}},
	})
}

func newServeMuxDecl(options Options, embedded bool) ast.Decl {
	stmts := []ast.Stmt{}
	stmts = append(stmts, loadEnvFileStmt(options)...)
	stmts = append(stmts, applyEnvDefaultsStmt(options.Config.Env)...)
	stmts = append(stmts, authSetupStmts(options)...)
	stmts = append(stmts, validateEnvContractStmt(options.Config.Env)...)
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
	if generatedRealtimeEnabled(options) {
		stmts = append(stmts, exprStmt(call(selExpr(id("mux"), "Handle"), id("RealtimeEventsPath"), call(id("realtimeEventsHandler")))))
	}
	if embedded {
		stmts = append(stmts, exprStmt(call(selExpr(id("mux"), "Handle"), stringLit("/"), &ast.CallExpr{
			Fun: sel("gowdkruntime", "ApplyMiddlewares"),
			Args: []ast.Expr{&ast.UnaryExpr{Op: token.AND, X: &ast.CompositeLit{
				Type: sel("gowdkruntime", "Handler"),
				Elts: embeddedHandlerFields(options, id("identity")),
			}}, call(id("registeredMiddlewares"))},
			Ellipsis: token.Pos(1),
		})))
	} else {
		stmts = append(stmts, exprStmt(call(selExpr(id("mux"), "Handle"), stringLit("/"), backendOnlyHandlerExpr(options))))
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("mux"), id("nil")}})
	return funcDecl("newServeMux", []*ast.Field{
		{Names: []*ast.Ident{id("identity")}, Type: sel("gowdkruntime", "Identity")},
	}, []*ast.Field{
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
		&ast.DeclStmt{Decl: &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
			Names: []*ast.Ident{id("csrfErr")},
			Type:  id("error"),
		}}}},
		assign([]ast.Expr{id("csrfTokenSource"), id("csrfErr")}, call(sel("newCSRF"))),
		&ast.IfStmt{
			Cond: notNil("csrfErr"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("csrfErr")}}),
		},
		assign([]ast.Expr{id("csrfValidator")}, id("csrfTokenSource")),
	}
}

func embeddedHandlerFields(options Options, identity ast.Expr) []ast.Expr {
	backend := ast.Expr(id(backendCallbackName(options)))
	if hasBackendRoutes(options) && !options.ProxyBackend {
		backend = call(selExpr(id("backendRouter"), "HandlerFunc"))
	}
	fields := []ast.Expr{
		keyValue("Root", id("root")),
		keyValue("Identity", identity),
		keyValue("Assets", call(sel("gowdkruntime", "LoadAssetManifest"), id("root"))),
		keyValue("ErrorPages", errorPagesExpr(options)),
		keyValue("Backend", backend),
	}
	if headers := securityHeadersExpr(options); headers != nil {
		fields = append(fields, keyValue("SecurityHeaders", headers))
	}
	if csrfEnabled(options) {
		fields = append(fields, keyValue("CSRF", id("csrfTokenSource")))
	}
	if generatedObservabilityEnabled(options) {
		fields = append(fields,
			keyValue("Tracer", id("traceTracer")),
			keyValue("TraceHandler", call(selExpr(id("traceCollector"), "ViewerHandler"))),
			keyValue("TraceAccess", sel("gowdkruntime", "LocalTraceAccess")),
		)
	}
	fields = append(fields,
		keyValue("SSRExact", id("ssrExact")),
		keyValue("SSRDynamic", id("ssrDynamic")),
		keyValue("RequestTimeout", sel("gowdkruntime", "DefaultRequestTimeout")),
	)
	if denied := deniedRoutesExpr(options); denied != nil {
		fields = append(fields, keyValue("Denied", denied))
	}
	if patterns := deniedRoutePatternsExpr(options); patterns != nil {
		fields = append(fields, keyValue("DeniedPatterns", patterns))
	}
	return fields
}

func securityHeadersExpr(options Options) ast.Expr {
	if !options.Config.Build.SecurityHeaders.Enabled || len(options.Config.Build.SecurityHeaders.Headers) == 0 {
		return nil
	}
	headers := normalizedSecurityHeaders(options.Config.Build.SecurityHeaders.Headers)
	if len(headers) == 0 {
		return nil
	}
	elts := make([]ast.Expr, 0, len(headers))
	for _, header := range headers {
		elts = append(elts, &ast.KeyValueExpr{
			Key:   stringLit(header.Name),
			Value: stringLit(header.Value),
		})
	}
	return &ast.CompositeLit{
		Type: &ast.MapType{Key: id("string"), Value: id("string")},
		Elts: elts,
	}
}

type normalizedSecurityHeader struct {
	Name  string
	Value string
}

func normalizedSecurityHeaders(headers map[string]string) []normalizedSecurityHeader {
	type candidate struct {
		key   string
		name  string
		value string
	}
	candidates := make([]candidate, 0, len(headers))
	for name, value := range headers {
		clean := strings.TrimSpace(name)
		if clean == "" {
			continue
		}
		candidates = append(candidates, candidate{
			key:   strings.ToLower(clean),
			name:  http.CanonicalHeaderKey(clean),
			value: value,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].key != candidates[j].key {
			return candidates[i].key < candidates[j].key
		}
		if candidates[i].name != candidates[j].name {
			return candidates[i].name < candidates[j].name
		}
		return candidates[i].value < candidates[j].value
	})
	seen := map[string]bool{}
	out := make([]normalizedSecurityHeader, 0, len(candidates))
	for _, candidate := range candidates {
		if seen[candidate.key] {
			continue
		}
		seen[candidate.key] = true
		out = append(out, normalizedSecurityHeader{Name: candidate.name, Value: candidate.value})
	}
	return out
}

// deniedPageRoutes returns the concrete (non-dynamic) page routes that declared
// no guard. Such pages are denied (403) at request time until the author adds
// guard public. Request-time (SSR) pages enforce the same default in their own
// handler, so they are excluded here. Dynamic build-time pages expand to many
// concrete artifacts and are denied by pattern instead; see
// deniedPageRoutePatterns.
func deniedPageRoutes(options Options) []string {
	return guardlessBuildTimeRoutes(options, false)
}

// deniedPageRoutePatterns returns the dynamic page route patterns (e.g.
// /blog/{slug}) for guardless build-time pages. The pattern denies every
// concrete paths {} artifact, which the exact Denied map cannot enumerate.
func deniedPageRoutePatterns(options Options) []string {
	return guardlessBuildTimeRoutes(options, true)
}

func guardlessBuildTimeRoutes(options Options, dynamic bool) []string {
	if options.IR == nil {
		return nil
	}
	ssrRoutes := map[string]bool{}
	ssrPageIDs := map[string]bool{}
	for _, route := range options.SSR {
		ssrRoutes[route.Route] = true
		if route.PageID != "" {
			ssrPageIDs[route.PageID] = true
		}
	}
	var routes []string
	seen := map[string]bool{}
	for _, page := range options.IR.Pages {
		if len(page.Guards) != 0 || page.Route == "" {
			continue
		}
		if ssrRoutes[page.Route] || ssrPageIDs[page.ID] {
			continue
		}
		if strings.Contains(page.Route, "{") != dynamic {
			continue
		}
		for _, localized := range options.Config.I18N.LocalizedRoutes(page.Route) {
			if seen[localized.Route] {
				continue
			}
			seen[localized.Route] = true
			routes = append(routes, localized.Route)
		}
	}
	return routes
}

func deniedRoutesExpr(options Options) ast.Expr {
	routes := deniedPageRoutes(options)
	if len(routes) == 0 {
		return nil
	}
	elts := make([]ast.Expr, 0, len(routes))
	for _, route := range routes {
		elts = append(elts, &ast.KeyValueExpr{Key: stringLit(route), Value: id("true")})
	}
	return &ast.CompositeLit{
		Type: &ast.MapType{Key: id("string"), Value: id("bool")},
		Elts: elts,
	}
}

func deniedRoutePatternsExpr(options Options) ast.Expr {
	patterns := deniedPageRoutePatterns(options)
	if len(patterns) == 0 {
		return nil
	}
	elts := make([]ast.Expr, 0, len(patterns))
	for _, pattern := range patterns {
		elts = append(elts, stringLit(pattern))
	}
	return &ast.CompositeLit{
		Type: &ast.ArrayType{Elt: id("string")},
		Elts: elts,
	}
}

func errorPagesExpr(options Options) ast.Expr {
	paths := customErrorPagePaths(options)
	if len(paths) == 0 {
		return call(sel("gowdkruntime", "LoadErrorPages"), id("root"))
	}
	args := []ast.Expr{id("root")}
	for _, errorPath := range paths {
		args = append(args, &ast.CompositeLit{
			Type: sel("gowdkruntime", "ErrorPage"),
			Elts: []ast.Expr{
				keyValue("Path", stringLit(errorPath)),
			},
		})
	}
	return call(sel("gowdkruntime", "LoadErrorPagesWith"), args...)
}

func customErrorPagePaths(options Options) []string {
	adapter := backendAdapterIR(options)
	seen := map[string]bool{}
	for _, action := range adapter.Actions {
		if action.ErrorPage != "" {
			seen[action.ErrorPage] = true
		}
	}
	for _, api := range adapter.APIs {
		if api.ErrorPage != "" {
			seen[api.ErrorPage] = true
		}
	}
	for _, route := range options.SSR {
		if route.ErrorPage != "" {
			seen[route.ErrorPage] = true
		}
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func backendOnlyHandlerExpr(options Options) ast.Expr {
	handler := backendOnlyBaseHandlerExpr(options)
	if headers := securityHeadersExpr(options); headers != nil {
		handler = call(sel("http", "HandlerFunc"), backendOnlySecurityHeadersHandlerFunc(handler, headers))
	}
	return &ast.CallExpr{
		Fun:      sel("gowdkruntime", "ApplyMiddlewares"),
		Args:     []ast.Expr{handler, call(id("registeredMiddlewares"))},
		Ellipsis: token.Pos(1),
	}
}

func backendOnlyBaseHandlerExpr(options Options) ast.Expr {
	if hasBackendRoutes(options) {
		if generatedObservabilityEnabled(options) {
			return &ast.UnaryExpr{Op: token.AND, X: &ast.CompositeLit{
				Type: sel("gowdkruntime", "Handler"),
				Elts: []ast.Expr{
					keyValue("Backend", call(selExpr(id("backendRouter"), "HandlerFunc"))),
					keyValue("Tracer", id("traceTracer")),
					keyValue("TraceHandler", call(selExpr(id("traceCollector"), "ViewerHandler"))),
					keyValue("TraceAccess", sel("gowdkruntime", "LocalTraceAccess")),
					keyValue("RequestTimeout", sel("gowdkruntime", "DefaultRequestTimeout")),
				},
			}}
		}
		return id("backendRouter")
	}
	return call(sel("http", "HandlerFunc"), backendOnlyHandlerFunc())
}

func backendOnlySecurityHeadersHandlerFunc(handler ast.Expr, headers ast.Expr) ast.Expr {
	return &ast.FuncLit{
		Type: &ast.FuncType{Params: &ast.FieldList{List: actionParams()}},
		Body: block(
			&ast.RangeStmt{
				Key:   id("name"),
				Value: id("value"),
				Tok:   token.DEFINE,
				X:     headers,
				Body: block(
					&ast.IfStmt{
						Cond: &ast.BinaryExpr{
							X:  call(sel("strings", "TrimSpace"), id("name")),
							Op: token.EQL,
							Y:  stringLit(""),
						},
						Body: block(&ast.BranchStmt{Tok: token.CONTINUE}),
					},
					exprStmt(call(selExpr(call(selExpr(id("response"), "Header")), "Set"), id("name"), id("value"))),
				),
			},
			exprStmt(call(selExpr(handler, "ServeHTTP"), id("response"), id("request"))),
		),
	}
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
	adapter := backendAdapterIR(options)
	return options.Config.Build.CSRF.EnabledForGeneratedEndpoints() && (adapter.HasEndpointKind(BackendEndpointAction) || adapter.HasCSRFSensitiveAPI() || contractExposuresParseForm(executableContractExposures(adapter.ContractExposures)))
}

func (ir BackendAdapterIR) HasCSRFSensitiveAPI() bool {
	for _, api := range ir.APIs {
		if gwdkir.HTTPMethodRequiresCSRF(api.Method) {
			return true
		}
	}
	return false
}

func csrfHelperSource(options Options) (source string, err error) {
	defer recoverGeneratedIdentifierError(&err)

	if !csrfEnabled(options) {
		return "", nil
	}
	return printActionDecls([]ast.Decl{
		csrfTokenSourceVarDecl(),
		csrfValidatorVarDecl(),
		csrfNewFuncDecl(options),
	})
}

func csrfTokenSourceVarDecl() ast.Decl {
	return &ast.GenDecl{
		Tok: token.VAR,
		Specs: []ast.Spec{&ast.ValueSpec{
			Names: []*ast.Ident{id("csrfTokenSource")},
			Type:  &ast.StarExpr{X: sel("gowdkactions", "CSRF")},
		}},
	}
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

func csrfNewFuncDecl(genOptions Options) *ast.FuncDecl {
	config := genOptions.Config.Build.CSRF
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
	// Bind the token to the authenticated principal when the app resolves one,
	// so a token minted for one user is rejected for another. Only wired when
	// native RBAC guards exist, because that is what declares the authProvider
	// var the binding reads.
	if generatedUsesNativeRBACGuards(genOptions) {
		options = append(options, keyValue("Binding", csrfPrincipalBindingExpr()))
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

// csrfPrincipalBindingExpr builds the CSRFOptions.Binding func literal that
// ties a CSRF token to the authenticated principal's ID. It returns nil for
// anonymous requests so the token remains a valid signed double-submit token
// before login and becomes session-bound once a principal resolves.
func csrfPrincipalBindingExpr() ast.Expr {
	return &ast.FuncLit{
		Type: &ast.FuncType{
			Params: &ast.FieldList{List: []*ast.Field{
				{Names: []*ast.Ident{id("request")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
			}},
			Results: &ast.FieldList{List: []*ast.Field{
				{Type: &ast.ArrayType{Elt: id("byte")}},
			}},
		},
		Body: block(
			&ast.IfStmt{
				Cond: &ast.BinaryExpr{X: id("authProvider"), Op: token.EQL, Y: id("nil")},
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil")}}),
			},
			define([]ast.Expr{id("principal"), id("err")}, call(selExpr(id("authProvider"), "Principal"), id("request"))),
			&ast.IfStmt{
				Cond: &ast.BinaryExpr{
					X:  notNil("err"),
					Op: token.LOR,
					Y:  &ast.BinaryExpr{X: id("principal"), Op: token.EQL, Y: id("nil")},
				},
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil")}}),
			},
			&ast.ReturnStmt{Results: []ast.Expr{
				call(&ast.ArrayType{Elt: id("byte")}, selExpr(id("principal"), "ID")),
			}},
		),
	}
}

func backendCallbackName(options Options) string {
	if options.ProxyBackend && hasBackendRoutes(options) {
		return "backendProxy"
	}
	return "backend"
}

func hasBackendRoutes(options Options) bool {
	return backendAdapterIR(options).HasRegistrations()
}

func backendImports(adapter BackendAdapterIR, ssr []SSRRoute) map[string]string {
	imports := adapter.BackendImports()
	for _, route := range ssr {
		// Guardless routes are denied before rendering, so their load handler is
		// never called and contributes no import. Importing it anyway leaves an
		// unused import that fails the generated app build.
		if len(route.Guards) == 0 {
			continue
		}
		if route.LoadBinding.ImportPath != "" && route.LoadBackendAlias != "" {
			imports[route.LoadBinding.ImportPath] = route.LoadBackendAlias
		}
	}
	return imports
}

package appgen

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

type lifecycleServiceProvider struct {
	ImportPath string
	Function   string
	Alias      string
}

func appDecl(options Options) ast.Decl {
	stmts := []ast.Stmt{
		define([]ast.Expr{id("identity")}, call(sel("gowdkruntime", "InstanceIdentity"))),
		define([]ast.Expr{id("mux"), id("err")}, call(id("newServeMux"), id("identity"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
		},
		define([]ast.Expr{id("handler")}, applyRegisteredMiddlewaresExpr(id("mux"))),
		define([]ast.Expr{id("values")}, &ast.CompositeLit{
			Type: &ast.MapType{Key: id("string"), Value: id("any")},
		}),
	}
	stmts = append(stmts, lifecycleValueStmts(options)...)
	stmts = append(stmts,
		define([]ast.Expr{id("services"), id("err")}, call(id("configuredServices"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
		},
	)
	stmts = append(stmts, observabilityServiceStmts(options)...)
	stmts = append(stmts,
		&ast.ReturnStmt{Results: []ast.Expr{&ast.UnaryExpr{Op: token.AND, X: &ast.CompositeLit{
			Type: sel("gowdkruntime", "Application"),
			Elts: []ast.Expr{
				keyValue("Handler", id("handler")),
				keyValue("Mux", id("mux")),
				keyValue("Identity", id("identity")),
				keyValue("Services", id("services")),
				keyValue("Values", id("values")),
			},
		}}, id("nil")}},
	)
	return funcDecl("App", nil, []*ast.Field{
		{Type: &ast.StarExpr{X: sel("gowdkruntime", "Application")}},
		{Type: id("error")},
	}, stmts)
}

func lifecycleValueStmts(options Options) []ast.Stmt {
	if !lifecycleContractRegistryEnabled(options) {
		return nil
	}
	return []ast.Stmt{assign([]ast.Expr{
		&ast.IndexExpr{
			X:     id("values"),
			Index: sel("gowdkruntime", "ServiceValueContractRegistry"),
		},
	}, call(id("ContractRegistry")))}
}

func lifecycleContractRegistryEnabled(options Options) bool {
	if options.ProxyBackend {
		return false
	}
	return len(executableContractExposures(backendAdapterIR(options).ContractExposures)) > 0
}

func configuredServicesDecl(providers []lifecycleServiceProvider) ast.Decl {
	if len(providers) == 0 {
		return funcDecl("configuredServices", nil, []*ast.Field{
			{Type: &ast.ArrayType{Elt: sel("gowdkruntime", "Service")}},
			{Type: id("error")},
		}, []ast.Stmt{
			&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("nil")}},
		})
	}

	stmts := []ast.Stmt{
		define([]ast.Expr{id("services")}, &ast.CompositeLit{
			Type: &ast.ArrayType{Elt: sel("gowdkruntime", "Service")},
		}),
	}
	for index, provider := range providers {
		name := fmt.Sprintf("provided%d", index)
		stmts = append(stmts,
			define([]ast.Expr{id(name), id("err")}, call(sel(provider.Alias, provider.Function))),
			&ast.IfStmt{
				Cond: notNil("err"),
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
			},
			assign([]ast.Expr{id("services")}, &ast.CallExpr{
				Fun:      id("append"),
				Args:     []ast.Expr{id("services"), id(name)},
				Ellipsis: token.Pos(1),
			}),
		)
	}
	stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("services"), id("nil")}})
	return funcDecl("configuredServices", nil, []*ast.Field{
		{Type: &ast.ArrayType{Elt: sel("gowdkruntime", "Service")}},
		{Type: id("error")},
	}, stmts)
}

func lifecycleServiceFileSources(options Options) (map[string][]byte, error) {
	providers := lifecycleServiceProviders(options)
	if len(providers) == 0 {
		return map[string][]byte{}, nil
	}
	source, err := lifecycleServiceFileSource("!js", providers)
	if err != nil {
		return nil, err
	}
	jsSource, err := lifecycleServiceFileSource("js", nil)
	if err != nil {
		return nil, err
	}
	return map[string][]byte{
		lifecycleFileName: []byte(source),
		lifecycleJSName:   []byte(jsSource),
	}, nil
}

func lifecycleServiceFileSource(buildTag string, providers []lifecycleServiceProvider) (string, error) {
	imports := map[string]string{
		"gowdkruntime": "github.com/cssbruno/gowdk/runtime/app",
	}
	for _, provider := range providers {
		imports[provider.Alias] = provider.ImportPath
	}
	source, err := printGoFile("gowdkapp", imports, []ast.Decl{configuredServicesDecl(providers)})
	if err != nil {
		return "", err
	}
	return "//go:build " + buildTag + "\n\n" + source, nil
}

func lifecycleServiceProviders(options Options) []lifecycleServiceProvider {
	aliases := map[string]string{}
	used := lifecycleReservedAliases(options)
	providers := make([]lifecycleServiceProvider, 0, len(options.Config.Lifecycle.Services))
	for _, service := range options.Config.Lifecycle.Services {
		importPath := strings.TrimSpace(service.ImportPath)
		function := strings.TrimSpace(service.Function)
		if importPath == "" || function == "" {
			continue
		}
		alias, ok := aliases[importPath]
		if !ok {
			alias = nextLifecycleServiceAlias(used)
			aliases[importPath] = alias
		}
		providers = append(providers, lifecycleServiceProvider{
			ImportPath: importPath,
			Function:   function,
			Alias:      alias,
		})
	}
	return providers
}

func lifecycleReservedAliases(options Options) map[string]bool {
	used := map[string]bool{}
	for alias := range generatedImportAliasUseCounts() {
		used[alias] = true
	}
	if options.ProxyBackend {
		return used
	}
	adapter := backendAdapterIR(options)
	for _, alias := range backendImports(adapter, options.SSR) {
		used[alias] = true
	}
	for _, alias := range backendContractImports(executableContractExposures(adapter.ContractExposures)) {
		used[alias] = true
	}
	return used
}

func nextLifecycleServiceAlias(used map[string]bool) string {
	for index := 0; ; index++ {
		alias := fmt.Sprintf("gowdkservice%d", index)
		if !used[alias] {
			used[alias] = true
			return alias
		}
	}
}

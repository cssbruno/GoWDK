package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/runtime/auth"
)

// ssrRegionRegistrationDecls generates an init() that registers a runtime region
// renderer for every eligible g:query region, keyed by query type. A registered
// renderer lets the single-flight g:command write path render exactly the regions
// a command invalidated inline in its response, so the submitting client applies
// them without a second page fetch. Regions that need route context the command
// request lacks are not registered (the build extractor omits them), so the
// client refetch remains their fallback.
func ssrRegionRegistrationDecls(routes []SSRRoute) []ast.Decl {
	var stmts []ast.Stmt
	for _, route := range ssrRegionRoutes(routes) {
		for _, region := range route.QueryRegions {
			stmts = append(stmts, exprStmt(call(
				sel("gowdkssr", "RegisterRegion"),
				regionRendererExpr(route, region),
			)))
		}
	}
	if len(stmts) == 0 {
		return nil
	}
	return []ast.Decl{funcDecl("init", nil, nil, stmts)}
}

// ssrRegionRoutes returns the routes whose g:query regions can be rendered
// standalone at command time: a bound, parameterless load whose data the region
// resolves against without dynamic route context. Protected and guardless
// routes are intentionally skipped; their normal page request performs guard
// checks before load, so inline command patches fall back to a page refetch.
func ssrRegionRoutes(routes []SSRRoute) []SSRRoute {
	var eligible []SSRRoute
	for _, route := range routes {
		if len(route.QueryRegions) == 0 {
			continue
		}
		if !ssrRegionRouteIsPublic(route) {
			continue
		}
		if !route.HasLoad || route.LoadBinding.Status != source.BackendBindingBound || route.LoadBackendAlias == "" {
			continue
		}
		if len(route.RouteParams) > 0 || len(route.DynamicParams) > 0 {
			continue
		}
		eligible = append(eligible, route)
	}
	return eligible
}

func ssrRegionRouteIsPublic(route SSRRoute) bool {
	return len(route.Guards) == 1 && auth.IsPublicGuard(route.Guards[0])
}

func commandPatchRenderingEnabled(options Options) bool {
	if !generatedRealtimeQueryInvalidationsEnabled(options) {
		return false
	}
	if len(ssrRegionRoutes(options.SSR)) == 0 {
		return false
	}
	return len(executableCommandContractExposures(backendAdapterIR(options).ContractExposures)) > 0
}

func realtimeQueryRefreshHandlerDecl() ast.Decl {
	return funcDecl("realtimeQueryRefreshHandler", nil, []*ast.Field{
		{Type: sel("http", "Handler")},
	}, []ast.Stmt{
		&ast.ReturnStmt{Results: []ast.Expr{call(sel("http", "HandlerFunc"), &ast.FuncLit{
			Type: &ast.FuncType{Params: &ast.FieldList{List: actionParams()}},
			Body: block(
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{X: selExpr(id("request"), "Method"), Op: token.NEQ, Y: sel("http", "MethodGet")},
					Body: block(
						writeNoStoreJSONErrorStmt(sel("http", "StatusMethodNotAllowed"), "method not allowed"),
						&ast.ReturnStmt{},
					),
				},
				define([]ast.Expr{id("queries")}, &ast.IndexExpr{
					X:     call(selExpr(selExpr(id("request"), "URL"), "Query")),
					Index: stringLit("query"),
				}),
				define([]ast.Expr{id("patches")}, call(sel("gowdkssr", "RenderInvalidatedRegions"), id("request"), id("queries"))),
				&ast.IfStmt{
					Cond: &ast.BinaryExpr{X: id("patches"), Op: token.EQL, Y: id("nil")},
					Body: block(assign([]ast.Expr{id("patches")}, &ast.CompositeLit{Type: &ast.ArrayType{Elt: sel("gowdkssr", "RegionPatch")}})),
				},
				define([]ast.Expr{id("result"), id("err")}, call(sel("gowdkresponse", "JSONValue"), sel("http", "StatusOK"), id("patches"))),
				&ast.IfStmt{
					Cond: notNil("err"),
					Body: block(
						writeNoStoreHandlerJSONErrorExprStmt(id("err"), sel("http", "StatusInternalServerError")),
						&ast.ReturnStmt{},
					),
				},
				writeNoStoreHTTPStmt(id("result")),
			),
		})}},
	})
}

func regionRendererExpr(route SSRRoute, region SSRQueryRegion) ast.Expr {
	elts := []ast.Expr{
		keyValue("QueryType", stringLit(region.QueryType)),
		keyValue("Template", stringLit(region.Template)),
	}
	if len(region.ListSpecs) > 0 {
		elts = append(elts, keyValue("Lists", ssrListSpecsExpr(region.ListSpecs)))
	}
	if len(region.CondSpecs) > 0 {
		elts = append(elts, keyValue("Conds", ssrCondSpecsExpr(region.CondSpecs)))
	}
	if len(region.LoadReplacements) > 0 {
		elts = append(elts, keyValue("LoadFields", regionLoadFieldsExpr(region.LoadReplacements)))
	}
	elts = append(elts, keyValue("Load", regionLoadThunk(route)))
	return &ast.CompositeLit{Type: sel("gowdkssr", "RegionRenderer"), Elts: elts}
}

func regionLoadFieldsExpr(replacements []SSRLoadReplacement) ast.Expr {
	elts := make([]ast.Expr, 0, len(replacements))
	for _, replacement := range replacements {
		fieldElts := []ast.Expr{
			keyValue("Path", stringLit(replacement.Path)),
			keyValue("Placeholder", stringLit(replacement.Placeholder)),
		}
		if replacement.URL {
			fieldElts = append(fieldElts, keyValue("URL", id("true")))
		}
		elts = append(elts, &ast.CompositeLit{Type: sel("gowdkssr", "RegionLoadField"), Elts: fieldElts})
	}
	return &ast.CompositeLit{
		Type: &ast.ArrayType{Elt: sel("gowdkssr", "RegionLoadField")},
		Elts: elts,
	}
}

// regionLoadthunk builds the func(*http.Request) (map[string]any, error)
// closure that runs a region's page load {} against a synthetic page GET
// request. This mirrors the normal SSR route context closely enough for
// parameterless public pages while dynamic or guarded pages stay on fallback.
func regionLoadThunk(route SSRRoute) ast.Expr {
	loadCall := call(sel(route.LoadBackendAlias, route.LoadBinding.FunctionName), id("loadContext"))
	stmts := ssrRouteContextStmts(route, false)
	stmts = append(stmts,
		define([]ast.Expr{id("pageURL")}, &ast.StarExpr{X: selExpr(id("request"), "URL")}),
		assign([]ast.Expr{selExpr(id("pageURL"), "Path")}, stringLit(route.Route)),
		assign([]ast.Expr{selExpr(id("pageURL"), "RawPath")}, stringLit("")),
		assign([]ast.Expr{selExpr(id("pageURL"), "RawQuery")}, stringLit("")),
		define([]ast.Expr{id("pageRequest")}, call(selExpr(id("request"), "Clone"), call(selExpr(id("request"), "Context")))),
		assign([]ast.Expr{selExpr(id("pageRequest"), "Method")}, sel("http", "MethodGet")),
		assign([]ast.Expr{selExpr(id("pageRequest"), "URL")}, &ast.UnaryExpr{Op: token.AND, X: id("pageURL")}),
		assign([]ast.Expr{id("request")}, id("pageRequest")),
		define([]ast.Expr{id("loadContext")}, call(sel("gowdkssr", "NewLoadContext"), id("request"), id("nil"))),
	)
	stmts = append(stmts, regionLoadReturnStmts(route.LoadBinding, loadCall)...)
	return &ast.FuncLit{
		Type: &ast.FuncType{
			Params: &ast.FieldList{List: []*ast.Field{
				{Names: []*ast.Ident{id("request")}, Type: &ast.StarExpr{X: sel("http", "Request")}},
			}},
			Results: &ast.FieldList{List: []*ast.Field{
				{Type: &ast.MapType{Key: id("string"), Value: id("any")}},
				{Type: id("error")},
			}},
		},
		Body: block(stmts...),
	}
}

func regionLoadReturnStmts(binding source.BackendBinding, loadCall ast.Expr) []ast.Stmt {
	switch {
	case loadSignatureReturnsStruct(binding.Signature) && loadSignatureReturnsError(binding.Signature):
		stmts := []ast.Stmt{
			define([]ast.Expr{id("typedLoadData"), id("err")}, loadCall),
			&ast.IfStmt{
				Cond: notNil("err"),
				Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("nil"), id("err")}}),
			},
		}
		stmts = append(stmts, ssrStructLoadDataStmts(binding, id("typedLoadData"))...)
		stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("loadData"), id("nil")}})
		return stmts
	case loadSignatureReturnsStruct(binding.Signature):
		stmts := []ast.Stmt{define([]ast.Expr{id("typedLoadData")}, loadCall)}
		stmts = append(stmts, ssrStructLoadDataStmts(binding, id("typedLoadData"))...)
		stmts = append(stmts, &ast.ReturnStmt{Results: []ast.Expr{id("loadData"), id("nil")}})
		return stmts
	case loadSignatureReturnsError(binding.Signature):
		return []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{loadCall}}}
	default:
		return []ast.Stmt{&ast.ReturnStmt{Results: []ast.Expr{loadCall, id("nil")}}}
	}
}

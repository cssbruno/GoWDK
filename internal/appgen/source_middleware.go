package appgen

import (
	"go/ast"
	"go/token"
)

func middlewareDecls() []ast.Decl {
	return []ast.Decl{
		middlewareStateDecl(),
		registerMiddlewareDecl(),
		registeredMiddlewaresDecl(),
	}
}

func middlewareStateDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{
		&ast.ValueSpec{
			Names: []*ast.Ident{id("middlewareMu")},
			Type:  sel("sync", "RWMutex"),
		},
		&ast.ValueSpec{
			Names: []*ast.Ident{id("middlewares")},
			Type:  &ast.ArrayType{Elt: sel("gowdkruntime", "Middleware")},
		},
	}}
}

func registerMiddlewareDecl() ast.Decl {
	return funcDecl("RegisterMiddleware", []*ast.Field{
		{Names: []*ast.Ident{id("middleware")}, Type: sel("gowdkruntime", "Middleware")},
	}, nil, []ast.Stmt{
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: id("middleware"), Op: token.EQL, Y: id("nil")},
			Body: block(&ast.ReturnStmt{}),
		},
		exprStmt(call(selExpr(id("middlewareMu"), "Lock"))),
		&ast.DeferStmt{Call: call(selExpr(id("middlewareMu"), "Unlock"))},
		assign([]ast.Expr{id("middlewares")}, call(id("append"), id("middlewares"), id("middleware"))),
	})
}

func registeredMiddlewaresDecl() ast.Decl {
	return funcDecl("registeredMiddlewares", nil, []*ast.Field{
		{Type: &ast.ArrayType{Elt: sel("gowdkruntime", "Middleware")}},
	}, []ast.Stmt{
		exprStmt(call(selExpr(id("middlewareMu"), "RLock"))),
		&ast.DeferStmt{Call: call(selExpr(id("middlewareMu"), "RUnlock"))},
		&ast.ReturnStmt{Results: []ast.Expr{
			&ast.CallExpr{
				Fun: id("append"),
				Args: []ast.Expr{
					call(&ast.ArrayType{Elt: sel("gowdkruntime", "Middleware")}, id("nil")),
					id("middlewares"),
				},
				Ellipsis: token.Pos(1),
			},
		}},
	})
}

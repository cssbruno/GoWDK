package appgen

import (
	"go/ast"
	"go/token"
)

func serverMainSource(gowdkappImportPath string) (string, error) {
	return printGoFile("main", map[string]string{
		"context":      "context",
		"gowdkapp":     gowdkappImportPath,
		"gowdkruntime": "github.com/cssbruno/gowdk/runtime/app",
		"log":          "log",
		"http":         "net/http",
		"os":           "os",
		"strings":      "strings",
		"time":         "time",
	}, []ast.Decl{
		serverMainDecl(),
		serverEnvDecl(),
	})
}

func serverMainDecl() ast.Decl {
	return funcDecl("main", nil, nil, []ast.Stmt{
		define([]ast.Expr{id("application"), id("err")}, call(sel("gowdkapp", "App"))),
		&ast.IfStmt{
			Cond: notNil("err"),
			Body: block(exprStmt(call(sel("log", "Fatal"), id("err")))),
		},
		define([]ast.Expr{id("addr")}, call(sel("env"), stringLit("GOWDK_ADDR"), stringLit("127.0.0.1:8080"))),
		define([]ast.Expr{id("server")}, &ast.UnaryExpr{
			Op: token.AND,
			X: &ast.CompositeLit{
				Type: sel("http", "Server"),
				Elts: []ast.Expr{
					keyValue("Addr", id("addr")),
					keyValue("Handler", selExpr(id("application"), "Handler")),
					keyValue("ReadHeaderTimeout", durationExpr(5)),
					keyValue("ReadTimeout", durationExpr(10)),
					keyValue("WriteTimeout", durationExpr(30)),
					keyValue("IdleTimeout", durationExpr(60)),
					keyValue("MaxHeaderBytes", &ast.BinaryExpr{X: intLit(1), Op: token.SHL, Y: intLit(20)}),
				},
			},
		}),
		exprStmt(call(sel("log", "Printf"), stringLit("serving embedded GOWDK app at http://%s"), id("addr"))),
		&ast.IfStmt{
			Init: define([]ast.Expr{id("err")}, call(sel("gowdkruntime", "Run"), call(sel("context", "Background")), id("server"), id("application"), &ast.CompositeLit{
				Type: sel("gowdkruntime", "RunOptions"),
				Elts: []ast.Expr{keyValue("ShutdownTimeout", durationExpr(10))},
			})),
			Cond: notNil("err"),
			Body: block(exprStmt(call(sel("log", "Fatal"), id("err")))),
		},
	})
}

func serverEnvDecl() ast.Decl {
	return funcDecl("env", []*ast.Field{
		{Names: []*ast.Ident{id("name")}, Type: id("string")},
		{Names: []*ast.Ident{id("fallback")}, Type: id("string")},
	}, []*ast.Field{{Type: id("string")}}, []ast.Stmt{
		define([]ast.Expr{id("value")}, call(sel("strings", "TrimSpace"), call(sel("os", "Getenv"), id("name")))),
		&ast.IfStmt{
			Cond: &ast.BinaryExpr{X: id("value"), Op: token.EQL, Y: stringLit("")},
			Body: block(&ast.ReturnStmt{Results: []ast.Expr{id("fallback")}}),
		},
		&ast.ReturnStmt{Results: []ast.Expr{id("value")}},
	})
}

func durationExpr(seconds int) ast.Expr {
	return &ast.BinaryExpr{X: intLit(seconds), Op: token.MUL, Y: sel("time", "Second")}
}

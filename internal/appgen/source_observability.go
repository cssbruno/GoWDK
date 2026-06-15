package appgen

import (
	"go/ast"
	"go/token"

	"github.com/cssbruno/gowdk"
)

func generatedObservabilityEnabled(options Options) bool {
	return options.Config.HasFeature(gowdk.FeatureObservability) && options.Config.Build.DebugAssets()
}

func observabilityDecls(options Options) []ast.Decl {
	if !generatedObservabilityEnabled(options) {
		return nil
	}
	return []ast.Decl{
		traceCollectorVarDecl(),
		traceTracerVarDecl(),
	}
}

func traceCollectorVarDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names:  []*ast.Ident{id("traceCollector")},
		Values: []ast.Expr{call(sel("gowdktrace", "NewCollector"), intLit(1024))},
	}}}
}

func traceTracerVarDecl() ast.Decl {
	return &ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
		Names: []*ast.Ident{id("traceTracer")},
		Values: []ast.Expr{call(sel("gowdktrace", "NewTracer"),
			call(sel("gowdktrace", "WithSink"), id("traceCollector")),
		)},
	}}}
}

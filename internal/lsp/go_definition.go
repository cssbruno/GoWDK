package lsp

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strings"
)

func (server *Server) goDefinition(name string) (location, bool) {
	if !isExportedGOWDKName(name) {
		return location{}, false
	}
	for _, doc := range server.documents {
		if !strings.HasSuffix(doc.Path, ".go") {
			continue
		}
		fileSet := token.NewFileSet()
		file, err := goparser.ParseFile(fileSet, doc.Path, doc.Text, 0)
		if err != nil {
			continue
		}
		if item, ok := goDefinitionInFile(fileSet, file, doc, name); ok {
			return item, true
		}
	}
	return location{}, false
}

func goDefinitionInFile(fileSet *token.FileSet, file *ast.File, doc document, name string) (location, bool) {
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			if typed.Name.Name == name {
				return goNameLocation(fileSet, doc.URI, typed.Name), true
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				if item, ok := goDefinitionInSpec(fileSet, doc.URI, spec, name); ok {
					return item, true
				}
			}
		}
	}
	return location{}, false
}

func goDefinitionInSpec(fileSet *token.FileSet, uri string, spec ast.Spec, name string) (location, bool) {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		if typed.Name.Name == name {
			return goNameLocation(fileSet, uri, typed.Name), true
		}
	case *ast.ValueSpec:
		for _, ident := range typed.Names {
			if ident.Name == name {
				return goNameLocation(fileSet, uri, ident), true
			}
		}
	}
	return location{}, false
}

func goNameLocation(fileSet *token.FileSet, uri string, ident *ast.Ident) location {
	start := fileSet.Position(ident.Pos())
	startPosition := position{Line: start.Line - 1, Character: start.Column - 1}
	return location{
		URI: uri,
		Range: lspRange{
			Start: startPosition,
			End: position{
				Line:      startPosition.Line,
				Character: startPosition.Character + utf16Length(ident.Name),
			},
		},
	}
}

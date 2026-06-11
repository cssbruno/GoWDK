package compiler

import (
	"fmt"
	"go/ast"
	"strings"

	"github.com/cssbruno/gowdk/internal/goblockgen"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func inspectInlineScriptFeaturePackage(page gwdkir.Page, target string) featurePackage {
	pkg := featurePackage{
		ImportPath: goblockgen.GeneratedImportPath(page.Package),
		Name:       goblockgen.SafePackageName(page.Package),
		Functions:  map[string]featureFunction{},
	}
	var files []*ast.File
	var importMaps []map[string]string
	for _, block := range page.Blocks.GoBlocks {
		if strings.TrimSpace(block.Target) != target {
			continue
		}
		file, err := goblockgen.ParseFile(page.Package, block)
		if err != nil {
			continue
		}
		files = append(files, file)
		importMaps = append(importMaps, goblockgen.ImportAliases(file, page.Imports))
	}
	if len(files) == 0 {
		return pkg
	}
	inputStructs := collectInputStructs(files)
	for index, file := range files {
		imports := importMaps[index]
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil || !fn.Name.IsExported() {
				continue
			}
			signature, inputType, inputPointer := backendSignature(fn.Type, imports)
			var inputFields []source.BackendInputField
			var supportMessage string
			if signature == source.BackendSignatureActionForm || signature == source.BackendSignatureActionFormPtr {
				inputStruct, ok := inputStructs[inputType]
				if !ok {
					supportMessage = fmt.Sprintf("typed action input %s must be an exported struct in the same package", inputType)
					signature = ""
				} else if inputStruct.Message != "" {
					supportMessage = inputStruct.Message
					signature = ""
				} else {
					inputFields = append([]source.BackendInputField(nil), inputStruct.Fields...)
				}
			}
			pkg.Functions[fn.Name.Name] = featureFunction{
				Name:           fn.Name.Name,
				Signature:      signature,
				InputType:      inputType,
				InputPointer:   inputPointer,
				InputFields:    inputFields,
				SupportMessage: supportMessage,
			}
		}
	}
	return pkg
}

func backendSignature(function *ast.FuncType, imports map[string]string) (source.BackendSignatureKind, string, bool) {
	if kind, inputType, inputPointer, ok := actionSignature(function, imports); ok {
		return kind, inputType, inputPointer
	}
	if isAPISignature(function, imports) {
		return source.BackendSignatureAPI, "", false
	}
	if signature, ok := loadSignature(function, imports); ok {
		return signature, "", false
	}
	return "", "", false
}

func actionSignature(function *ast.FuncType, imports map[string]string) (source.BackendSignatureKind, string, bool, bool) {
	if function == nil || function.Params == nil || function.Results == nil {
		return "", "", false, false
	}
	if len(function.Results.List) != 2 {
		return "", "", false, false
	}
	if !isSelector(function.Results.List[0].Type, imports, responseImportPath, "Response") ||
		!isError(function.Results.List[1].Type) {
		return "", "", false, false
	}
	if len(function.Params.List) != 1 && len(function.Params.List) != 2 {
		return "", "", false, false
	}
	if !isSelector(function.Params.List[0].Type, imports, contextImportPath, "Context") {
		return "", "", false, false
	}
	if len(function.Params.List) == 1 {
		return source.BackendSignatureAction0, "", false, true
	}
	second := function.Params.List[1].Type
	if isSelector(second, imports, formImportPath, "Values") {
		return source.BackendSignatureActionValues, "", false, true
	}
	if ident, ok := second.(*ast.Ident); ok && ident.IsExported() {
		return source.BackendSignatureActionForm, ident.Name, false, true
	}
	if pointer, ok := second.(*ast.StarExpr); ok {
		if ident, ok := pointer.X.(*ast.Ident); ok && ident.IsExported() {
			return source.BackendSignatureActionFormPtr, ident.Name, true, true
		}
	}
	return "", "", false, false
}

func isAPISignature(function *ast.FuncType, imports map[string]string) bool {
	if function == nil || function.Params == nil || function.Results == nil {
		return false
	}
	if len(function.Params.List) != 2 || len(function.Results.List) != 2 {
		return false
	}
	request, ok := function.Params.List[1].Type.(*ast.StarExpr)
	return ok &&
		isSelector(function.Params.List[0].Type, imports, contextImportPath, "Context") &&
		isSelector(request.X, imports, httpImportPath, "Request") &&
		isSelector(function.Results.List[0].Type, imports, responseImportPath, "Response") &&
		isError(function.Results.List[1].Type)
}

func loadSignature(function *ast.FuncType, imports map[string]string) (source.BackendSignatureKind, bool) {
	if function == nil || function.Params == nil || function.Results == nil {
		return "", false
	}
	if len(function.Params.List) != 1 || !isSelector(function.Params.List[0].Type, imports, ssrImportPath, "LoadContext") {
		return "", false
	}
	if len(function.Results.List) == 1 && isMapStringAny(function.Results.List[0].Type) {
		return source.BackendSignatureLoad, true
	}
	if len(function.Results.List) == 2 && isMapStringAny(function.Results.List[0].Type) && isError(function.Results.List[1].Type) {
		return source.BackendSignatureLoadError, true
	}
	return "", false
}

func isMapStringAny(expression ast.Expr) bool {
	mapType, ok := expression.(*ast.MapType)
	if !ok {
		return false
	}
	key, ok := mapType.Key.(*ast.Ident)
	if !ok || key.Name != "string" {
		return false
	}
	if value, ok := mapType.Value.(*ast.Ident); ok && value.Name == "any" {
		return true
	}
	_, ok = mapType.Value.(*ast.InterfaceType)
	return ok
}

func isSelector(expression ast.Expr, imports map[string]string, importPath, name string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil || selector.Sel.Name != name {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	return imports[ident.Name] == importPath
}

func isError(expression ast.Expr) bool {
	ident, ok := expression.(*ast.Ident)
	return ok && ident.Name == "error"
}

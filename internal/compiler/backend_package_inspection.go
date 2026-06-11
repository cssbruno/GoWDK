package compiler

import (
	"fmt"
	"go/ast"
	"go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

func inspectFeaturePackage(dir string) featurePackage {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	pkg := featurePackage{Dir: absDir, Functions: map[string]featureFunction{}}
	loaded, err := loadFeaturePackage(absDir)
	if err != nil {
		pkg.LoadError = err.Error()
		return pkg
	}
	pkg.ImportPath = normalizedPackageImportPath(loaded.PkgPath)
	if len(loaded.CompiledGoFiles) > 0 || len(loaded.Syntax) > 0 {
		pkg.Name = loaded.Name
	}
	pkg.Functions = typedFeatureFunctions(loaded)
	return pkg
}

func loadFeaturePackage(absDir string) (*packages.Package, error) {
	config := &packages.Config{
		Dir: absDir,
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo,
		Tests: false,
	}
	loaded, err := packages.Load(config, ".")
	if err != nil {
		return nil, err
	}
	if len(loaded) == 0 {
		return &packages.Package{}, nil
	}
	pkg := loaded[0]
	if len(pkg.Errors) > 0 {
		if packageHasNoGoFiles(pkg.Errors) {
			return pkg, nil
		}
		return pkg, fmt.Errorf("%s", pkg.Errors[0].Msg)
	}
	if pkg.Types == nil {
		return pkg, fmt.Errorf("package %s did not provide type information", packageLabel(featurePackage{ImportPath: pkg.PkgPath, Name: pkg.Name}))
	}
	return pkg, nil
}

func normalizedPackageImportPath(importPath string) string {
	if importPath == "." {
		return ""
	}
	return importPath
}

func packageHasNoGoFiles(errors []packages.Error) bool {
	if len(errors) == 0 {
		return false
	}
	for _, err := range errors {
		message := err.Msg
		if !strings.Contains(message, "no Go files") &&
			!strings.Contains(message, "build constraints exclude all Go files") {
			return false
		}
	}
	return true
}

func typedFeatureFunctions(pkg *packages.Package) map[string]featureFunction {
	functions := map[string]featureFunction{}
	if pkg == nil || pkg.Types == nil {
		return functions
	}
	if pkg.TypesInfo != nil {
		for _, file := range pkg.Syntax {
			for _, declaration := range file.Decls {
				fn, ok := declaration.(*ast.FuncDecl)
				if !ok || fn.Recv != nil || fn.Name == nil || !fn.Name.IsExported() {
					continue
				}
				obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func)
				if !ok || obj == nil {
					continue
				}
				if function, ok := typedFeatureFunction(obj, pkg.Types); ok {
					functions[function.Name] = function
				}
			}
		}
	}
	if len(functions) > 0 {
		return functions
	}
	scope := pkg.Types.Scope()
	if scope == nil {
		return functions
	}
	for _, name := range scope.Names() {
		obj, ok := scope.Lookup(name).(*types.Func)
		if !ok || obj == nil || !obj.Exported() {
			continue
		}
		if function, ok := typedFeatureFunction(obj, pkg.Types); ok {
			functions[function.Name] = function
		}
	}
	return functions
}

func typedFeatureFunction(obj *types.Func, pkg *types.Package) (featureFunction, bool) {
	signature, ok := obj.Type().(*types.Signature)
	if !ok {
		return featureFunction{}, false
	}
	kind, inputType, inputPointer, inputFields, supportMessage := backendTypedSignature(signature, pkg)
	return featureFunction{
		Name:           obj.Name(),
		Signature:      kind,
		InputType:      inputType,
		InputPointer:   inputPointer,
		InputFields:    inputFields,
		SupportMessage: supportMessage,
	}, true
}

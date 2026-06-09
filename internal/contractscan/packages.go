package contractscan

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

type fileScan struct {
	Contracts      []Contract
	Diagnostics    []Diagnostic
	EmitsByHandler map[string][]EventRef
}

type inputStruct struct {
	Fields  []source.BackendInputField
	Message string
}

type contractTypeInfo struct {
	Exported bool
	Struct   bool
}

type parsedGoFile struct {
	Path    string
	Rel     string
	Package string
	File    *ast.File
	Aliases map[string]bool
	Imports map[string]string
}

func parseScanPackages(fset *token.FileSet, root string, files []string) ([][]parsedGoFile, error) {
	groups := map[string][]parsedGoFile{}
	var keys []string
	for _, path := range files {
		parsed, err := parseScanFile(fset, root, path)
		if err != nil {
			return nil, err
		}
		key := filepath.Dir(parsed.Path) + "\x00" + parsed.Package
		if _, exists := groups[key]; !exists {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], parsed)
	}
	sort.Strings(keys)
	packages := make([][]parsedGoFile, 0, len(keys))
	for _, key := range keys {
		packages = append(packages, groups[key])
	}
	return packages, nil
}

func parseScanFile(fset *token.FileSet, root string, path string) (parsedGoFile, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return parsedGoFile{}, err
	}
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return parsedGoFile{}, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	return parsedGoFile{
		Path:    path,
		Rel:     rel,
		Package: file.Name.Name,
		File:    file,
		Aliases: contractsImportAliases(file),
		Imports: goImportAliases(file),
	}, nil
}

func scanPackage(fset *token.FileSet, files []parsedGoFile, inspectionCache *packageInspectionCache) fileScan {
	astFiles := make([]*ast.File, 0, len(files))
	for _, file := range files {
		astFiles = append(astFiles, file.File)
	}
	types := collectContractTypes(astFiles)
	inputStructs := collectContractInputStructs(astFiles)
	packageDir := ""
	if len(files) > 0 {
		packageDir = filepath.Dir(files[0].Path)
	}
	typedPackage := inspectTypedPackage(fset, packageDir, astFiles, inspectionCache)
	var contracts []Contract
	var diagnostics []Diagnostic
	emitsByHandler := map[string][]EventRef{}
	for _, file := range files {
		diagnostics = append(diagnostics, generatedAppImportDiagnostics(fset, file)...)
	}
	for _, file := range files {
		if len(file.Aliases) == 0 {
			continue
		}
		discovered := scanContractRegistrations(fset, file.File, file.Aliases, file.Imports, file.Rel)
		contracts = append(contracts, discovered...)
		for handler, emits := range emittedEventsByHandler(fset, file.File, file.Aliases, file.Imports) {
			emitsByHandler[handler] = append(emitsByHandler[handler], emits...)
		}
	}
	applyContractInputFields(contracts, inputStructs)
	diagnostics = append(diagnostics, validateContractTypes(contracts, types, typedPackage.Types)...)
	diagnostics = append(diagnostics, validateEventNames(contracts)...)
	diagnostics = append(diagnostics, validateContracts(contracts, typedPackage.Functions)...)
	diagnostics = append(diagnostics, validateContractInputStructs(contracts, inputStructs)...)
	return fileScan{
		Contracts:      contracts,
		Diagnostics:    diagnostics,
		EmitsByHandler: emitsByHandler,
	}
}

func collectContractTypes(files []*ast.File) map[string]contractTypeInfo {
	types := map[string]contractTypeInfo{}
	for _, file := range files {
		for _, declaration := range file.Decls {
			gen, ok := declaration.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil {
					continue
				}
				_, isStruct := typeSpec.Type.(*ast.StructType)
				types[typeSpec.Name.Name] = contractTypeInfo{
					Exported: typeSpec.Name.IsExported(),
					Struct:   isStruct,
				}
			}
		}
	}
	return types
}

func collectContractInputStructs(files []*ast.File) map[string]inputStruct {
	structs := map[string]inputStruct{}
	for _, file := range files {
		for _, declaration := range file.Decls {
			gen, ok := declaration.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil || !typeSpec.Name.IsExported() {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				structs[typeSpec.Name.Name] = contractInputStruct(typeSpec.Name.Name, structType)
			}
		}
	}
	return structs
}

func applyContractInputFields(contracts []Contract, structs map[string]inputStruct) {
	for index := range contracts {
		if contracts[index].Kind != runtimecontracts.Command && contracts[index].Kind != runtimecontracts.Query {
			continue
		}
		inputStruct, ok := structs[contracts[index].Type]
		if !ok || inputStruct.Message != "" {
			continue
		}
		contracts[index].InputFields = append([]source.BackendInputField(nil), inputStruct.Fields...)
	}
}

func validateContractInputStructs(contracts []Contract, structs map[string]inputStruct) []Diagnostic {
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		if contract.Kind != runtimecontracts.Command && contract.Kind != runtimecontracts.Query {
			continue
		}
		inputStruct, ok := structs[contract.Type]
		if !ok || inputStruct.Message == "" {
			continue
		}
		diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_input_invalid", inputStruct.Message))
	}
	return diagnostics
}

func generatedAppImportDiagnostics(fset *token.FileSet, file parsedGoFile) []Diagnostic {
	var diagnostics []Diagnostic
	for _, importSpec := range file.File.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil || !isGeneratedAppImportPath(importPath) {
			continue
		}
		position := fset.Position(importSpec.Pos())
		diagnostics = append(diagnostics, Diagnostic{
			Severity: "error",
			Code:     "generated_app_import_cycle",
			Package:  file.Package,
			Source:   file.Rel,
			Line:     position.Line,
			Column:   position.Column,
			Message:  fmt.Sprintf("feature package must not import generated app output %q; keep generated app startup and registration code outside feature packages", importPath),
		})
	}
	return diagnostics
}

func isGeneratedAppImportPath(importPath string) bool {
	return importPath == generatedAppModulePath || strings.HasPrefix(importPath, generatedAppModulePath+"/")
}

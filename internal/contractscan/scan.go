// Package contractscan discovers runtime contract registrations in normal Go
// source using the standard Go AST.
package contractscan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"os"
	"os/exec"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

const RuntimeImportPath = "github.com/cssbruno/gowdk/runtime/contracts"
const generatedAppModulePath = "gowdk-generated-app"

// Contract describes one discovered registration call.
type Contract struct {
	Kind             runtimecontracts.Kind          `json:"kind"`
	EventCategory    runtimecontracts.EventCategory `json:"eventCategory,omitempty"`
	Package          string                         `json:"package,omitempty"`
	Type             string                         `json:"type"`
	TypeImportPath   string                         `json:"typeImportPath,omitempty"`
	Result           string                         `json:"result,omitempty"`
	ResultImportPath string                         `json:"resultImportPath,omitempty"`
	Handler          string                         `json:"handler,omitempty"`
	Register         string                         `json:"register,omitempty"`
	InputFields      []manifest.BackendInputField   `json:"inputFields,omitempty"`
	Emits            []EventRef                     `json:"emits,omitempty"`
	Roles            []string                       `json:"roles,omitempty"`
	Source           string                         `json:"source"`
	Line             int                            `json:"line"`
	Column           int                            `json:"column"`
}

// Diagnostic describes a validation issue found while scanning contracts.
type Diagnostic struct {
	Severity       string                `json:"severity"`
	Code           string                `json:"code,omitempty"`
	Kind           runtimecontracts.Kind `json:"kind,omitempty"`
	Package        string                `json:"package,omitempty"`
	Type           string                `json:"type,omitempty"`
	TypeImportPath string                `json:"typeImportPath,omitempty"`
	Handler        string                `json:"handler,omitempty"`
	Source         string                `json:"source"`
	Line           int                   `json:"line"`
	Column         int                   `json:"column"`
	Message        string                `json:"message"`
}

// EventRef describes one event a command handler can emit.
type EventRef struct {
	Category       runtimecontracts.EventCategory `json:"category"`
	Type           string                         `json:"type"`
	TypeImportPath string                         `json:"typeImportPath,omitempty"`
}

// Report is the full discovery output.
type Report struct {
	Version     int          `json:"version"`
	Root        string       `json:"root"`
	Contracts   []Contract   `json:"contracts"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
}

// Scan walks root and reports registrations that call runtime/contracts helpers.
func Scan(root string) (Report, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Report{}, err
	}
	var files []string
	if err := filepath.WalkDir(absRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if shouldSkipDir(entry.Name()) && path != absRoot {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return Report{}, err
	}
	sort.Strings(files)
	fset := token.NewFileSet()
	var contracts []Contract
	var diagnostics []Diagnostic
	emitsByHandler := map[string][]EventRef{}
	packages, err := parseScanPackages(fset, absRoot, files)
	if err != nil {
		return Report{}, err
	}
	inspectionCache := newPackageInspectionCache()
	for _, pkg := range packages {
		discovered := scanPackage(fset, pkg, inspectionCache)
		contracts = append(contracts, discovered.Contracts...)
		diagnostics = append(diagnostics, discovered.Diagnostics...)
		for handler, emits := range discovered.EmitsByHandler {
			emitsByHandler[handler] = append(emitsByHandler[handler], emits...)
		}
	}
	diagnostics = append(diagnostics, duplicateCommandDiagnostics(contracts)...)
	for index := range contracts {
		if contracts[index].Kind != runtimecontracts.Command {
			continue
		}
		contracts[index].Emits = copyEventRefs(emitsByHandler[contracts[index].Handler])
	}
	diagnostics = append(diagnostics, emittedEventCategoryDiagnostics(contracts)...)
	sort.Slice(contracts, func(i, j int) bool {
		if contracts[i].Kind != contracts[j].Kind {
			return contracts[i].Kind < contracts[j].Kind
		}
		if contracts[i].EventCategory != contracts[j].EventCategory {
			return contracts[i].EventCategory < contracts[j].EventCategory
		}
		if contracts[i].Package != contracts[j].Package {
			return contracts[i].Package < contracts[j].Package
		}
		if contracts[i].Type != contracts[j].Type {
			return contracts[i].Type < contracts[j].Type
		}
		if contracts[i].Source != contracts[j].Source {
			return contracts[i].Source < contracts[j].Source
		}
		return contracts[i].Line < contracts[j].Line
	})
	sort.Slice(diagnostics, func(i, j int) bool {
		if diagnostics[i].Source != diagnostics[j].Source {
			return diagnostics[i].Source < diagnostics[j].Source
		}
		if diagnostics[i].Line != diagnostics[j].Line {
			return diagnostics[i].Line < diagnostics[j].Line
		}
		return diagnostics[i].Column < diagnostics[j].Column
	})
	return Report{Version: 1, Root: absRoot, Contracts: contracts, Diagnostics: diagnostics}, nil
}

// Filter returns contracts of kind. Empty kind returns a copy of all contracts.
func (report Report) Filter(kind runtimecontracts.Kind) []Contract {
	out := make([]Contract, 0, len(report.Contracts))
	for _, contract := range report.Contracts {
		if kind == "" || contract.Kind == kind {
			out = append(out, contract)
		}
	}
	return out
}

// JSON returns deterministic indented JSON.
func (report Report) JSON(kind runtimecontracts.Kind) ([]byte, error) {
	out := struct {
		Version     int          `json:"version"`
		Root        string       `json:"root"`
		Contracts   []Contract   `json:"contracts"`
		Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	}{
		Version:     report.Version,
		Root:        report.Root,
		Contracts:   report.Filter(kind),
		Diagnostics: report.Diagnostics,
	}
	return json.MarshalIndent(out, "", "  ")
}

// LinkReferences resolves GOWDK IR contract references against scanned Go
// runtime contract registrations.
func LinkReferences(refs []gwdkir.ContractReference, report Report) []gwdkir.ContractReference {
	if len(refs) == 0 {
		return nil
	}
	contracts := map[gwdkir.ContractKind]map[string]Contract{
		gwdkir.ContractCommand: {},
		gwdkir.ContractQuery:   {},
	}
	for _, contract := range report.Contracts {
		refKind, ok := irContractKind(contract.Kind)
		if !ok {
			continue
		}
		for _, key := range contractReferenceKeys(contract) {
			contracts[refKind][key] = contract
		}
	}
	invalid := map[gwdkir.ContractKind]map[string]Diagnostic{
		gwdkir.ContractCommand: {},
		gwdkir.ContractQuery:   {},
	}
	for _, diagnostic := range report.Diagnostics {
		refKind, ok := irContractKind(diagnostic.Kind)
		if !ok {
			continue
		}
		for _, key := range diagnosticReferenceKeys(diagnostic) {
			invalid[refKind][key] = diagnostic
		}
	}
	linked := make([]gwdkir.ContractReference, len(refs))
	for index, ref := range refs {
		linked[index] = ref
		kindContracts, ok := contracts[ref.Kind]
		if !ok {
			if linked[index].Status == "" {
				linked[index].Status = gwdkir.ContractBindingUnknown
			}
			continue
		}
		contract, ok := lookupContractReference(kindContracts, ref)
		if !ok {
			linked[index].Status = gwdkir.ContractBindingMissing
			linked[index].Message = fmt.Sprintf("%s %s has no scanned Go registration", ref.Kind, ref.Name)
			continue
		}
		linked[index].Handler = contract.Handler
		linked[index].Register = contract.Register
		if linked[index].Type == "" {
			linked[index].Type = contract.Type
		}
		linked[index].Result = contract.Result
		linked[index].Roles = append([]string(nil), contract.Roles...)
		linked[index].InputFields = append([]manifest.BackendInputField(nil), contract.InputFields...)
		if diagnostic, bad := lookupContractDiagnostic(invalid[ref.Kind], ref); bad {
			linked[index].Status = gwdkir.ContractBindingInvalid
			linked[index].Message = diagnostic.Message
			continue
		}
		linked[index].Status = gwdkir.ContractBindingBound
	}
	return linked
}

func lookupContractReference(contracts map[string]Contract, ref gwdkir.ContractReference) (Contract, bool) {
	for _, key := range contractReferenceLookupKeys(ref) {
		if contract, ok := contracts[key]; ok {
			return contract, true
		}
	}
	return Contract{}, false
}

func lookupContractDiagnostic(diagnostics map[string]Diagnostic, ref gwdkir.ContractReference) (Diagnostic, bool) {
	for _, key := range contractReferenceLookupKeys(ref) {
		if diagnostic, ok := diagnostics[key]; ok {
			return diagnostic, true
		}
	}
	return Diagnostic{}, false
}

func contractReferenceLookupKeys(ref gwdkir.ContractReference) []string {
	var keys []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, key := range keys {
			if key == value {
				return
			}
		}
		keys = append(keys, value)
	}
	add(ref.Name)
	if ref.Type != "" {
		if ref.ImportPath != "" {
			add(contractImportTypeKey(ref.ImportPath, ref.Type))
		}
		add(ref.Type)
		if ref.ImportAlias != "" {
			add(ref.ImportAlias + "." + ref.Type)
		}
	}
	return keys
}

func irContractKind(kind runtimecontracts.Kind) (gwdkir.ContractKind, bool) {
	switch kind {
	case runtimecontracts.Command:
		return gwdkir.ContractCommand, true
	case runtimecontracts.Query:
		return gwdkir.ContractQuery, true
	default:
		return "", false
	}
}

func contractReferenceKeys(contract Contract) []string {
	keys := []string{contract.Type}
	if contract.TypeImportPath != "" {
		keys = append(keys, contractImportTypeKey(contract.TypeImportPath, localContractName(contract.Type)))
	}
	if contract.Package != "" && contract.Type != "" && !strings.Contains(contract.Type, ".") {
		keys = append(keys, contract.Package+"."+contract.Type)
	}
	return keys
}

func diagnosticReferenceKeys(diagnostic Diagnostic) []string {
	keys := []string{diagnostic.Type}
	if diagnostic.TypeImportPath != "" {
		keys = append(keys, contractImportTypeKey(diagnostic.TypeImportPath, localContractName(diagnostic.Type)))
	}
	if diagnostic.Package != "" && diagnostic.Type != "" && !strings.Contains(diagnostic.Type, ".") {
		keys = append(keys, diagnostic.Package+"."+diagnostic.Type)
	}
	return keys
}

func contractImportTypeKey(importPath string, typeName string) string {
	return strings.TrimSpace(importPath) + "\x00" + strings.TrimSpace(localContractName(typeName))
}

type fileScan struct {
	Contracts      []Contract
	Diagnostics    []Diagnostic
	EmitsByHandler map[string][]EventRef
}

type inputStruct struct {
	Fields  []manifest.BackendInputField
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
	source, err := os.ReadFile(path)
	if err != nil {
		return parsedGoFile{}, err
	}
	file, err := parser.ParseFile(fset, path, source, 0)
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
	functions := typedFunctions(fset, packageDir, astFiles, inspectionCache)
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
	diagnostics = append(diagnostics, validateContractTypes(contracts, types)...)
	diagnostics = append(diagnostics, validateEventNames(contracts)...)
	diagnostics = append(diagnostics, validateContracts(contracts, functions)...)
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
		contracts[index].InputFields = append([]manifest.BackendInputField(nil), inputStruct.Fields...)
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

func contractInputStruct(typeName string, structType *ast.StructType) inputStruct {
	if structType == nil || structType.Fields == nil {
		return inputStruct{}
	}
	seen := map[string]bool{}
	var fields []manifest.BackendInputField
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			return inputStruct{Message: fmt.Sprintf("contract input %s cannot use embedded fields", typeName)}
		}
		formName, skip, explicit, err := contractFormTagName(field)
		if err != nil {
			return inputStruct{Message: fmt.Sprintf("contract input %s has invalid form tag: %v", typeName, err)}
		}
		var exportedNames []*ast.Ident
		for _, name := range field.Names {
			if name != nil && name.IsExported() {
				exportedNames = append(exportedNames, name)
			}
		}
		if len(exportedNames) == 0 || skip {
			continue
		}
		if explicit && len(exportedNames) > 1 {
			return inputStruct{Message: fmt.Sprintf("contract input %s cannot reuse one explicit form tag across multiple fields", typeName)}
		}
		fieldType, ok := contractInputFieldType(field.Type)
		if !ok {
			return inputStruct{Message: fmt.Sprintf("contract input %s uses unsupported field type", typeName)}
		}
		for _, name := range exportedNames {
			nameFormName := formName
			if nameFormName == "" {
				nameFormName = name.Name
			}
			if seen[nameFormName] {
				return inputStruct{Message: fmt.Sprintf("contract input %s maps multiple fields to form field %q", typeName, nameFormName)}
			}
			seen[nameFormName] = true
			fields = append(fields, manifest.BackendInputField{
				FieldName: name.Name,
				FormName:  nameFormName,
				Type:      fieldType,
			})
		}
	}
	return inputStruct{Fields: fields}
}

func contractFormTagName(field *ast.Field) (string, bool, bool, error) {
	if field == nil || field.Tag == nil {
		return "", false, false, nil
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false, false, err
	}
	value, ok, err := contractStructTagValue(tag, "form")
	if err != nil || !ok {
		return "", false, ok, err
	}
	name, _, _ := strings.Cut(value, ",")
	if name == "-" {
		return "", true, true, nil
	}
	return strings.TrimSpace(name), false, true, nil
}

func contractStructTagValue(tag string, key string) (string, bool, error) {
	for tag != "" {
		tag = strings.TrimLeft(tag, " ")
		if tag == "" {
			return "", false, nil
		}
		keyEnd := strings.IndexByte(tag, ':')
		if keyEnd <= 0 {
			return "", false, fmt.Errorf("malformed struct tag")
		}
		name := tag[:keyEnd]
		rest := tag[keyEnd+1:]
		if rest == "" || rest[0] != '"' {
			return "", false, fmt.Errorf("malformed struct tag")
		}
		valueEnd := 1
		for valueEnd < len(rest) {
			if rest[valueEnd] == '\\' {
				valueEnd += 2
				continue
			}
			if rest[valueEnd] == '"' {
				break
			}
			valueEnd++
		}
		if valueEnd >= len(rest) || rest[valueEnd] != '"' {
			return "", false, fmt.Errorf("malformed struct tag")
		}
		rawValue := rest[:valueEnd+1]
		value, err := strconv.Unquote(rawValue)
		if err != nil {
			return "", false, err
		}
		if name == key {
			return value, true, nil
		}
		tag = rest[valueEnd+1:]
	}
	return "", false, nil
}

func contractInputFieldType(expression ast.Expr) (string, bool) {
	if ident, ok := expression.(*ast.Ident); ok {
		switch ident.Name {
		case "string", "bool", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
			return ident.Name, true
		default:
			return "", false
		}
	}
	array, ok := expression.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return "", false
	}
	ident, ok := array.Elt.(*ast.Ident)
	if !ok || ident.Name != "string" {
		return "", false
	}
	return "[]string", true
}

func scanContractRegistrations(fset *token.FileSet, file *ast.File, aliases map[string]bool, imports map[string]string, source string) []Contract {
	var contracts []Contract
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		register := contractRegisterFunction(fn, aliases)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, typeArgs := registrationSelector(call.Fun)
			if selector == nil {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok || !aliases[ident.Name] {
				return true
			}
			kind, category, ok := registrationKind(selector.Sel.Name)
			if !ok {
				return true
			}
			position := fset.Position(call.Pos())
			contract := Contract{
				Kind:          kind,
				EventCategory: category,
				Package:       file.Name.Name,
				Source:        source,
				Line:          position.Line,
				Column:        position.Column,
				Handler:       handlerName(call),
				Register:      register,
				Roles:         roleNames(call),
			}
			if len(typeArgs) > 0 {
				contract.Type, contract.TypeImportPath = contractTypeName(fset, typeArgs[0], imports)
			}
			if len(typeArgs) > 1 {
				contract.Result, contract.ResultImportPath = contractTypeName(fset, typeArgs[1], imports)
			}
			contracts = append(contracts, contract)
			return true
		})
	}
	return contracts
}

func contractTypeName(fset *token.FileSet, expr ast.Expr, imports map[string]string) (string, string) {
	name := exprString(fset, expr)
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil {
		return name, ""
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok || ident == nil {
		return name, ""
	}
	return name, imports[ident.Name]
}

func validateContractTypes(contracts []Contract, types map[string]contractTypeInfo) []Diagnostic {
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		diagnostics = append(diagnostics, validateContractType(contract, types)...)
		if contract.Kind == runtimecontracts.Command || contract.Kind == runtimecontracts.Query {
			diagnostics = append(diagnostics, validateContractResultType(contract, types)...)
		}
	}
	return diagnostics
}

func validateContractType(contract Contract, types map[string]contractTypeInfo) []Diagnostic {
	if strings.TrimSpace(contract.Type) == "" {
		return []Diagnostic{contractDiagnostic(contract, "contract_type_invalid", fmt.Sprintf("%s registration must declare a contract type", contract.Kind))}
	}
	return validateLocalContractType(contract, types, contract.Type, "contract_type_invalid", fmt.Sprintf("%s contract type", contract.Kind))
}

func validateContractResultType(contract Contract, types map[string]contractTypeInfo) []Diagnostic {
	if strings.TrimSpace(contract.Result) == "" {
		return []Diagnostic{contractDiagnostic(contract, "contract_result_invalid", fmt.Sprintf("%s registration must declare a result type", contract.Kind))}
	}
	return validateLocalContractType(contract, types, contract.Result, "contract_result_invalid", fmt.Sprintf("%s result type", contract.Kind))
}

func validateLocalContractType(contract Contract, types map[string]contractTypeInfo, name string, code string, label string) []Diagnostic {
	if !isLocalIdentifier(name) {
		return nil
	}
	info, ok := types[name]
	if !ok {
		return []Diagnostic{contractDiagnostic(contract, code, fmt.Sprintf("%s %s was not found in the scanned package", label, name))}
	}
	if !info.Exported {
		return []Diagnostic{contractDiagnostic(contract, code, fmt.Sprintf("%s %s must be exported", label, name))}
	}
	if !info.Struct {
		return []Diagnostic{contractDiagnostic(contract, code, fmt.Sprintf("%s %s must be a struct", label, name))}
	}
	return nil
}

func validateEventNames(contracts []Contract) []Diagnostic {
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		if contract.Kind != runtimecontracts.Event {
			continue
		}
		if message := eventNameMessage(contract); message != "" {
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_event_name_invalid", message))
		}
	}
	return diagnostics
}

func eventNameMessage(contract Contract) string {
	name := localContractName(contract.Type)
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, "Changed") {
		return fmt.Sprintf("event %s is too vague; use a specific backend fact such as PatientCreated or PatientUpdated", name)
	}
	words := camelWords(name)
	if hasAnyWord(words, uiEventSubjects) && hasAnyWord(words, uiEventActions) {
		return fmt.Sprintf("event %s looks like a browser UI event; UI events must trigger commands or queries, not backend events", name)
	}
	return ""
}

func localContractName(name string) string {
	name = strings.TrimSpace(name)
	if index := strings.LastIndex(name, "."); index >= 0 {
		name = name[index+1:]
	}
	return name
}

var uiEventSubjects = map[string]bool{
	"Button":    true,
	"Checkbox":  true,
	"Component": true,
	"Dialog":    true,
	"Dropdown":  true,
	"Field":     true,
	"Form":      true,
	"Input":     true,
	"Modal":     true,
	"Page":      true,
	"Select":    true,
	"Tab":       true,
	"View":      true,
}

var uiEventActions = map[string]bool{
	"Blurred":   true,
	"Changed":   true,
	"Clicked":   true,
	"Closed":    true,
	"Focused":   true,
	"Hovered":   true,
	"Opened":    true,
	"Pressed":   true,
	"Selected":  true,
	"Submitted": true,
	"Toggled":   true,
	"Typed":     true,
}

func camelWords(value string) []string {
	var words []string
	start := 0
	var previous rune
	for index, current := range value {
		if index > 0 && previous >= 'a' && previous <= 'z' && current >= 'A' && current <= 'Z' {
			words = append(words, value[start:index])
			start = index
		}
		previous = current
	}
	if start < len(value) {
		words = append(words, value[start:])
	}
	return words
}

func hasAnyWord(words []string, choices map[string]bool) bool {
	for _, word := range words {
		if choices[word] {
			return true
		}
	}
	return false
}

func contractRegisterFunction(fn *ast.FuncDecl, aliases map[string]bool) string {
	if fn.Name == nil || fn.Name.Name == "init" || fn.Recv != nil || fn.Type == nil || fn.Type.Params == nil {
		return ""
	}
	for _, field := range fn.Type.Params.List {
		if isRegistryPointer(field.Type, aliases) {
			return fn.Name.Name
		}
	}
	return ""
}

func isRegistryPointer(expr ast.Expr, aliases map[string]bool) bool {
	pointer, ok := expr.(*ast.StarExpr)
	if !ok {
		return false
	}
	selector, ok := pointer.X.(*ast.SelectorExpr)
	if !ok || selector.Sel == nil || selector.Sel.Name != "Registry" {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && aliases[ident.Name]
}

type functionInfo struct {
	Signature *types.Signature
	Package   *types.Package
}

func typedFunctions(fset *token.FileSet, packageDir string, files []*ast.File, inspectionCache *packageInspectionCache) map[string]functionInfo {
	info := &types.Info{
		Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{},
	}
	config := types.Config{
		Importer: contractScanImporter(packageDir, fset, files, inspectionCache),
		Error:    func(error) {},
	}
	packageName := ""
	if len(files) > 0 && files[0].Name != nil {
		packageName = files[0].Name.Name
	}
	pkg, _ := config.Check(packageName, fset, files, info)
	functions := map[string]functionInfo{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			obj, ok := info.Defs[fn.Name].(*types.Func)
			if !ok || obj == nil {
				continue
			}
			signature, ok := obj.Type().(*types.Signature)
			if !ok {
				continue
			}
			functions[fn.Name.Name] = functionInfo{Signature: signature, Package: pkg}
		}
	}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel == nil {
				return true
			}
			obj, ok := info.Uses[selector.Sel].(*types.Func)
			if !ok || obj == nil {
				return true
			}
			signature, ok := obj.Type().(*types.Signature)
			if !ok {
				return true
			}
			functions[exprString(fset, selector)] = functionInfo{Signature: signature, Package: pkg}
			return true
		})
	}
	return functions
}

type packageInspectionCache struct {
	exports         map[string]map[string]string
	loadExportFiles func(packageDir string, importPaths []string) (map[string]string, error)
}

func newPackageInspectionCache() *packageInspectionCache {
	return &packageInspectionCache{
		exports:         map[string]map[string]string{},
		loadExportFiles: scanGoListExportFiles,
	}
}

func (cache *packageInspectionCache) exportFiles(packageDir string, importPaths []string) (map[string]string, error) {
	if cache == nil {
		return scanGoListExportFiles(packageDir, importPaths)
	}
	key := packageInspectionCacheKey(packageDir, importPaths)
	if exports, ok := cache.exports[key]; ok {
		return exports, nil
	}
	exports, err := cache.loadExportFiles(packageDir, importPaths)
	if err != nil {
		return nil, err
	}
	cache.exports[key] = exports
	return exports, nil
}

func packageInspectionCacheKey(packageDir string, importPaths []string) string {
	paths := append([]string(nil), importPaths...)
	sort.Strings(paths)
	return packageDir + "\x00" + strings.Join(paths, "\x00")
}

func contractScanImporter(packageDir string, fset *token.FileSet, files []*ast.File, inspectionCache *packageInspectionCache) types.Importer {
	importPaths := scanImportedGoPaths(files)
	if packageDir == "" || len(importPaths) == 0 {
		return importer.Default()
	}
	exports, err := inspectionCache.exportFiles(packageDir, importPaths)
	if err != nil || len(exports) == 0 {
		return importer.Default()
	}
	return importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		exportPath := exports[path]
		if exportPath == "" {
			return nil, fmt.Errorf("missing export data for %s", path)
		}
		return os.Open(exportPath)
	})
}

func scanImportedGoPaths(files []*ast.File) []string {
	seen := map[string]bool{}
	var paths []string
	for _, file := range files {
		for _, spec := range file.Imports {
			if spec.Path == nil {
				continue
			}
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || path == "" || seen[path] {
				continue
			}
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func scanGoListExportFiles(packageDir string, importPaths []string) (map[string]string, error) {
	args := append([]string{"list", "-deps", "-export", "-json"}, importPaths...)
	command := exec.Command("go", args...)
	command.Dir = packageDir
	output, err := command.Output()
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	exports := map[string]string{}
	for {
		var item struct {
			ImportPath string
			Export     string
		}
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if item.ImportPath == "" || item.Export == "" {
			continue
		}
		exports[item.ImportPath] = item.Export
	}
	return exports, nil
}

func validateContracts(contracts []Contract, functions map[string]functionInfo) []Diagnostic {
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		switch {
		case strings.TrimSpace(contract.Handler) == "":
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_handler_invalid", fmt.Sprintf("%s registration must pass an exported handler function", contract.Kind)))
			continue
		case isLocalIdentifier(contract.Handler) && !ast.IsExported(contract.Handler):
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_handler_invalid", fmt.Sprintf("%s handler %s must be exported", contract.Kind, contract.Handler)))
			continue
		}
		if !isLocalIdentifier(contract.Handler) && !isSelectorHandler(contract.Handler) {
			continue
		}
		function := functions[contract.Handler]
		if function.Signature == nil {
			if !isLocalIdentifier(contract.Handler) {
				continue
			}
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_handler_missing", fmt.Sprintf("handler %s was not found in the scanned package", contract.Handler)))
			continue
		}
		if message := validateHandlerSignature(contract, function); message != "" {
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_handler_invalid", message))
		}
	}
	return diagnostics
}

func duplicateCommandDiagnostics(contracts []Contract) []Diagnostic {
	byCommand := map[string][]Contract{}
	for _, contract := range contracts {
		if contract.Kind != runtimecontracts.Command {
			continue
		}
		key := commandIdentity(contract)
		if key == "" {
			continue
		}
		byCommand[key] = append(byCommand[key], contract)
	}
	var diagnostics []Diagnostic
	for _, matches := range byCommand {
		if len(matches) < 2 {
			continue
		}
		handlers := make([]string, 0, len(matches))
		for _, match := range matches {
			handlers = append(handlers, emptyDiagnosticValue(match.Handler))
		}
		sort.Strings(handlers)
		for _, match := range matches[1:] {
			diagnostics = append(diagnostics, contractDiagnostic(match, "duplicate_command_owner", fmt.Sprintf("command %s has multiple owner registrations: %s", commandIdentity(match), strings.Join(handlers, ", "))))
		}
	}
	return diagnostics
}

func emittedEventCategoryDiagnostics(contracts []Contract) []Diagnostic {
	events := map[string]map[runtimecontracts.EventCategory]bool{}
	for _, contract := range contracts {
		if contract.Kind != runtimecontracts.Event {
			continue
		}
		key := eventIdentity(contract.TypeImportPath, contract.Type)
		if key == "" {
			continue
		}
		if events[key] == nil {
			events[key] = map[runtimecontracts.EventCategory]bool{}
		}
		events[key][contract.EventCategory] = true
	}
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		if contract.Kind != runtimecontracts.Command || len(contract.Emits) == 0 {
			continue
		}
		for _, emit := range contract.Emits {
			key := eventIdentity(emit.TypeImportPath, emit.Type)
			registeredCategories := events[key]
			if len(registeredCategories) == 0 || registeredCategories[emit.Category] {
				continue
			}
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_event_category_invalid", fmt.Sprintf("command %s emits %s event %s but scanned registrations use event categories %s", contract.Type, emit.Category, emit.Type, eventCategoryList(registeredCategories))))
		}
	}
	return diagnostics
}

func eventIdentity(importPath string, typeName string) string {
	typeName = strings.TrimSpace(localContractName(typeName))
	if typeName == "" {
		return ""
	}
	importPath = strings.TrimSpace(importPath)
	if importPath == "" {
		return typeName
	}
	return importPath + "\x00" + typeName
}

func eventCategoryList(categories map[runtimecontracts.EventCategory]bool) string {
	values := make([]string, 0, len(categories))
	for category := range categories {
		values = append(values, string(category))
	}
	sort.Strings(values)
	return strings.Join(values, ", ")
}

func commandIdentity(contract Contract) string {
	if contract.Type == "" {
		return ""
	}
	if strings.Contains(contract.Type, ".") || contract.Package == "" {
		return contract.Type
	}
	return contract.Package + "." + contract.Type
}

func emptyDiagnosticValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "(unknown)"
	}
	return value
}

func validateHandlerSignature(contract Contract, function functionInfo) string {
	switch contract.Kind {
	case runtimecontracts.Query, runtimecontracts.Command:
		return validateRequestHandlerSignature(contract, function)
	case runtimecontracts.Event, runtimecontracts.Job:
		return validateEffectHandlerSignature(contract, function)
	default:
		return ""
	}
}

func validateRequestHandlerSignature(contract Contract, function functionInfo) string {
	signature := function.Signature
	if signature.Params().Len() != 2 {
		return fmt.Sprintf("%s handler %s must accept context.Context and %s", contract.Kind, contract.Handler, contract.Type)
	}
	if got := scanTypeString(signature.Params().At(0).Type(), function.Package); got != "context.Context" {
		return fmt.Sprintf("%s handler %s first parameter must be context.Context, got %s", contract.Kind, contract.Handler, got)
	}
	if got := scanTypeString(signature.Params().At(1).Type(), function.Package); got != contract.Type {
		return fmt.Sprintf("%s handler %s second parameter must be %s, got %s", contract.Kind, contract.Handler, contract.Type, got)
	}
	if signature.Results().Len() != 2 {
		return fmt.Sprintf("%s handler %s must return %s and error", contract.Kind, contract.Handler, contract.Result)
	}
	if got := scanTypeString(signature.Results().At(0).Type(), function.Package); got != contract.Result {
		return fmt.Sprintf("%s handler %s first result must be %s, got %s", contract.Kind, contract.Handler, contract.Result, got)
	}
	if got := scanTypeString(signature.Results().At(1).Type(), function.Package); got != "error" {
		return fmt.Sprintf("%s handler %s second result must be error, got %s", contract.Kind, contract.Handler, got)
	}
	return ""
}

func validateEffectHandlerSignature(contract Contract, function functionInfo) string {
	signature := function.Signature
	if signature.Params().Len() != 2 {
		return fmt.Sprintf("%s handler %s must accept context.Context and %s", contract.Kind, contract.Handler, contract.Type)
	}
	if got := scanTypeString(signature.Params().At(0).Type(), function.Package); got != "context.Context" {
		return fmt.Sprintf("%s handler %s first parameter must be context.Context, got %s", contract.Kind, contract.Handler, got)
	}
	if got := scanTypeString(signature.Params().At(1).Type(), function.Package); got != contract.Type {
		return fmt.Sprintf("%s handler %s second parameter must be %s, got %s", contract.Kind, contract.Handler, contract.Type, got)
	}
	if signature.Results().Len() != 1 {
		return fmt.Sprintf("%s handler %s must return error", contract.Kind, contract.Handler)
	}
	if got := scanTypeString(signature.Results().At(0).Type(), function.Package); got != "error" {
		return fmt.Sprintf("%s handler %s result must be error, got %s", contract.Kind, contract.Handler, got)
	}
	return ""
}

func scanTypeString(typ types.Type, local *types.Package) string {
	return types.TypeString(typ, func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		if local != nil && pkg.Path() == local.Path() {
			return ""
		}
		return pkg.Name()
	})
}

func isLocalIdentifier(value string) bool {
	if value == "" || strings.Contains(value, ".") {
		return false
	}
	return token.IsIdentifier(value)
}

func isSelectorHandler(value string) bool {
	qualifier, name, ok := strings.Cut(value, ".")
	return ok && token.IsIdentifier(qualifier) && token.IsIdentifier(name) && ast.IsExported(name)
}

func contractDiagnostic(contract Contract, code string, message string) Diagnostic {
	return Diagnostic{
		Severity:       "error",
		Code:           code,
		Kind:           contract.Kind,
		Package:        contract.Package,
		Type:           contract.Type,
		TypeImportPath: contract.TypeImportPath,
		Handler:        contract.Handler,
		Source:         contract.Source,
		Line:           contract.Line,
		Column:         contract.Column,
		Message:        message,
	}
}

func contractsImportAliases(file *ast.File) map[string]bool {
	aliases := map[string]bool{}
	for _, importSpec := range file.Imports {
		path, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil || path != RuntimeImportPath {
			continue
		}
		switch {
		case importSpec.Name == nil:
			aliases["contracts"] = true
		case importSpec.Name.Name != "." && importSpec.Name.Name != "_":
			aliases[importSpec.Name.Name] = true
		}
	}
	return aliases
}

func goImportAliases(file *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, importSpec := range file.Imports {
		importPath, err := strconv.Unquote(importSpec.Path.Value)
		if err != nil || importPath == "" {
			continue
		}
		alias := ""
		switch {
		case importSpec.Name == nil:
			alias = pathpkg.Base(importPath)
		case importSpec.Name.Name == "." || importSpec.Name.Name == "_":
			continue
		default:
			alias = importSpec.Name.Name
		}
		if alias != "" {
			aliases[alias] = importPath
		}
	}
	return aliases
}

func registrationSelector(expr ast.Expr) (*ast.SelectorExpr, []ast.Expr) {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		return typed, nil
	case *ast.IndexExpr:
		selector, _ := typed.X.(*ast.SelectorExpr)
		return selector, []ast.Expr{typed.Index}
	case *ast.IndexListExpr:
		selector, _ := typed.X.(*ast.SelectorExpr)
		return selector, typed.Indices
	default:
		return nil, nil
	}
}

func registrationKind(name string) (runtimecontracts.Kind, runtimecontracts.EventCategory, bool) {
	switch name {
	case "RegisterQuery":
		return runtimecontracts.Query, "", true
	case "RegisterCommand":
		return runtimecontracts.Command, "", true
	case "RegisterDomainEvent":
		return runtimecontracts.Event, runtimecontracts.DomainEvent, true
	case "RegisterIntegrationEvent":
		return runtimecontracts.Event, runtimecontracts.IntegrationEvent, true
	case "RegisterPresentationEvent":
		return runtimecontracts.Event, runtimecontracts.PresentationEvent, true
	case "RegisterJob":
		return runtimecontracts.Job, "", true
	default:
		return "", "", false
	}
}

func emittedEventsByHandler(fset *token.FileSet, file *ast.File, aliases map[string]bool, imports map[string]string) map[string][]EventRef {
	out := map[string][]EventRef{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		seen := map[EventRef]bool{}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, typeArgs := registrationSelector(call.Fun)
			if selector == nil {
				return true
			}
			ident, ok := selector.X.(*ast.Ident)
			if !ok || !aliases[ident.Name] {
				return true
			}
			category, ok := emitCategory(selector.Sel.Name)
			if !ok {
				return true
			}
			eventType, eventImportPath := emittedEventType(fset, call, typeArgs, imports)
			if eventType == "" {
				return true
			}
			ref := EventRef{Category: category, Type: eventType, TypeImportPath: eventImportPath}
			if !seen[ref] {
				out[fn.Name.Name] = append(out[fn.Name.Name], ref)
				seen[ref] = true
			}
			return true
		})
	}
	for handler := range out {
		sort.Slice(out[handler], func(i, j int) bool {
			if out[handler][i].Category != out[handler][j].Category {
				return out[handler][i].Category < out[handler][j].Category
			}
			return out[handler][i].Type < out[handler][j].Type
		})
	}
	return out
}

func emitCategory(name string) (runtimecontracts.EventCategory, bool) {
	switch name {
	case "EmitDomain":
		return runtimecontracts.DomainEvent, true
	case "EmitIntegration":
		return runtimecontracts.IntegrationEvent, true
	case "EmitPresentation":
		return runtimecontracts.PresentationEvent, true
	default:
		return "", false
	}
}

func emittedEventType(fset *token.FileSet, call *ast.CallExpr, typeArgs []ast.Expr, imports map[string]string) (string, string) {
	if len(typeArgs) > 0 {
		return contractTypeName(fset, typeArgs[0], imports)
	}
	if len(call.Args) < 2 {
		return "", ""
	}
	switch arg := call.Args[1].(type) {
	case *ast.CompositeLit:
		return contractTypeName(fset, arg.Type, imports)
	case *ast.UnaryExpr:
		if literal, ok := arg.X.(*ast.CompositeLit); ok {
			return contractTypeName(fset, literal.Type, imports)
		}
	}
	return "", ""
}

func handlerName(call *ast.CallExpr) string {
	if len(call.Args) < 2 {
		return ""
	}
	return exprString(token.NewFileSet(), call.Args[1])
}

func roleNames(call *ast.CallExpr) []string {
	if len(call.Args) <= 2 {
		return nil
	}
	roles := make([]string, 0, len(call.Args)-2)
	for _, arg := range call.Args[2:] {
		roles = append(roles, roleName(arg))
	}
	return roles
}

func roleName(expr ast.Expr) string {
	switch typed := expr.(type) {
	case *ast.SelectorExpr:
		switch typed.Sel.Name {
		case "RoleWeb":
			return string(runtimecontracts.RoleWeb)
		case "RoleWorker":
			return string(runtimecontracts.RoleWorker)
		case "RoleCron":
			return string(runtimecontracts.RoleCron)
		case "RoleAPI":
			return string(runtimecontracts.RoleAPI)
		case "RoleAdmin":
			return string(runtimecontracts.RoleAdmin)
		}
	case *ast.BasicLit:
		value, err := strconv.Unquote(typed.Value)
		if err == nil {
			return value
		}
	}
	return exprString(token.NewFileSet(), expr)
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	if expr == nil {
		return ""
	}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, fset, expr); err != nil {
		return fmt.Sprint(expr)
	}
	return buffer.String()
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", ".gowdk", "vendor", "node_modules", "dist", "bin":
		return true
	default:
		return false
	}
}

func copyEventRefs(values []EventRef) []EventRef {
	if len(values) == 0 {
		return nil
	}
	copied := make([]EventRef, len(values))
	copy(copied, values)
	return copied
}

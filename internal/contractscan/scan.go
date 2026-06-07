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
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	runtimecontracts "github.com/cssbruno/gowdk/runtime/contracts"
)

const RuntimeImportPath = "github.com/cssbruno/gowdk/runtime/contracts"

// Contract describes one discovered registration call.
type Contract struct {
	Kind          runtimecontracts.Kind          `json:"kind"`
	EventCategory runtimecontracts.EventCategory `json:"eventCategory,omitempty"`
	Package       string                         `json:"package,omitempty"`
	Type          string                         `json:"type"`
	Result        string                         `json:"result,omitempty"`
	Handler       string                         `json:"handler,omitempty"`
	Emits         []EventRef                     `json:"emits,omitempty"`
	Roles         []string                       `json:"roles,omitempty"`
	Source        string                         `json:"source"`
	Line          int                            `json:"line"`
	Column        int                            `json:"column"`
}

// Diagnostic describes a validation issue found while scanning contracts.
type Diagnostic struct {
	Severity string                `json:"severity"`
	Code     string                `json:"code,omitempty"`
	Kind     runtimecontracts.Kind `json:"kind,omitempty"`
	Package  string                `json:"package,omitempty"`
	Type     string                `json:"type,omitempty"`
	Handler  string                `json:"handler,omitempty"`
	Source   string                `json:"source"`
	Line     int                   `json:"line"`
	Column   int                   `json:"column"`
	Message  string                `json:"message"`
}

// EventRef describes one event a command handler can emit.
type EventRef struct {
	Category runtimecontracts.EventCategory `json:"category"`
	Type     string                         `json:"type"`
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
	for _, file := range files {
		discovered, err := scanFile(fset, absRoot, file)
		if err != nil {
			return Report{}, err
		}
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
		if linked[index].Type == "" {
			linked[index].Type = contract.Type
		}
		linked[index].Result = contract.Result
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
	if contract.Package != "" && contract.Type != "" && !strings.Contains(contract.Type, ".") {
		keys = append(keys, contract.Package+"."+contract.Type)
	}
	return keys
}

func diagnosticReferenceKeys(diagnostic Diagnostic) []string {
	keys := []string{diagnostic.Type}
	if diagnostic.Package != "" && diagnostic.Type != "" && !strings.Contains(diagnostic.Type, ".") {
		keys = append(keys, diagnostic.Package+"."+diagnostic.Type)
	}
	return keys
}

type fileScan struct {
	Contracts      []Contract
	Diagnostics    []Diagnostic
	EmitsByHandler map[string][]EventRef
}

func scanFile(fset *token.FileSet, root string, path string) (fileScan, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return fileScan{}, err
	}
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return fileScan{}, err
	}
	aliases := contractsImportAliases(file)
	if len(aliases) == 0 {
		return fileScan{}, nil
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)
	var contracts []Contract
	ast.Inspect(file, func(node ast.Node) bool {
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
			Source:        rel,
			Line:          position.Line,
			Column:        position.Column,
			Handler:       handlerName(call),
			Roles:         roleNames(call),
		}
		if len(typeArgs) > 0 {
			contract.Type = exprString(fset, typeArgs[0])
		}
		if len(typeArgs) > 1 {
			contract.Result = exprString(fset, typeArgs[1])
		}
		contracts = append(contracts, contract)
		return true
	})
	functions := typedFunctions(fset, file)
	return fileScan{
		Contracts:      contracts,
		Diagnostics:    validateContracts(contracts, functions),
		EmitsByHandler: emittedEventsByHandler(fset, file, aliases),
	}, nil
}

type functionInfo struct {
	Signature *types.Signature
	Package   *types.Package
}

func typedFunctions(fset *token.FileSet, file *ast.File) map[string]functionInfo {
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}}
	config := types.Config{
		Importer: importer.Default(),
		Error:    func(error) {},
	}
	pkg, _ := config.Check(file.Name.Name, fset, []*ast.File{file}, info)
	functions := map[string]functionInfo{}
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
	return functions
}

func validateContracts(contracts []Contract, functions map[string]functionInfo) []Diagnostic {
	var diagnostics []Diagnostic
	for _, contract := range contracts {
		if contract.Handler == "" || strings.Contains(contract.Handler, ".") {
			continue
		}
		function := functions[contract.Handler]
		if function.Signature == nil {
			diagnostics = append(diagnostics, contractDiagnostic(contract, "contract_handler_missing", fmt.Sprintf("handler %s was not found in the scanned file", contract.Handler)))
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

func contractDiagnostic(contract Contract, code string, message string) Diagnostic {
	return Diagnostic{
		Severity: "error",
		Code:     code,
		Kind:     contract.Kind,
		Package:  contract.Package,
		Type:     contract.Type,
		Handler:  contract.Handler,
		Source:   contract.Source,
		Line:     contract.Line,
		Column:   contract.Column,
		Message:  message,
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

func emittedEventsByHandler(fset *token.FileSet, file *ast.File, aliases map[string]bool) map[string][]EventRef {
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
			eventType := emittedEventType(fset, call, typeArgs)
			if eventType == "" {
				return true
			}
			ref := EventRef{Category: category, Type: eventType}
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

func emittedEventType(fset *token.FileSet, call *ast.CallExpr, typeArgs []ast.Expr) string {
	if len(typeArgs) > 0 {
		return exprString(fset, typeArgs[0])
	}
	if len(call.Args) < 2 {
		return ""
	}
	switch arg := call.Args[1].(type) {
	case *ast.CompositeLit:
		return exprString(fset, arg.Type)
	case *ast.UnaryExpr:
		if literal, ok := arg.X.(*ast.CompositeLit); ok {
			return exprString(fset, literal.Type)
		}
	}
	return ""
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

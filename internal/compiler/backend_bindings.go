package compiler

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

const (
	actionHandlerKind   = "action"
	apiHandlerKind      = "api"
	fragmentHandlerKind = "fragment"
	loadHandlerKind     = "load"

	contextImportPath  = "context"
	formImportPath     = "github.com/cssbruno/gowdk/runtime/form"
	httpImportPath     = "net/http"
	responseImportPath = "github.com/cssbruno/gowdk/runtime/response"
	ssrImportPath      = "github.com/cssbruno/gowdk/addons/ssr"
)

// BindBackendHandlers discovers same-package Go handlers for act and api blocks.
// Discovery is intentionally non-fatal: missing packages, missing functions, and
// unsupported signatures are reported as binding metadata so generated apps can
// emit clear 501 responses.
func BindBackendHandlers(app manifest.Manifest) manifest.Manifest {
	var bindings []manifest.BackendBinding
	cache := map[string]featurePackage{}
	for _, page := range app.Pages {
		if len(page.Blocks.Actions) == 0 && len(page.Blocks.APIs) == 0 && len(page.Blocks.Fragments) == 0 && !page.Blocks.Load {
			continue
		}
		dir := sourceDir(page.Source)
		pkg, ok := cache[dir]
		if !ok {
			pkg = inspectFeaturePackage(dir)
			cache[dir] = pkg
		}
		if page.Blocks.Load {
			bindings = append(bindings, bindLoad(page, pkg))
		}
		for _, action := range page.Blocks.Actions {
			bindings = append(bindings, bindAction(page, action, pkg))
		}
		for _, api := range page.Blocks.APIs {
			bindings = append(bindings, bindAPI(page, api, pkg))
		}
		for _, fragment := range page.Blocks.Fragments {
			if binding, ok := bindFragment(page, fragment, pkg); ok {
				bindings = append(bindings, binding)
			}
		}
	}
	for _, endpoint := range app.Endpoints {
		dir := sourceDir(endpoint.Source)
		pkg, ok := cache[dir]
		if !ok {
			pkg = inspectFeaturePackage(dir)
			cache[dir] = pkg
		}
		switch endpoint.Kind {
		case "act", "action":
			bindings = append(bindings, bindStandaloneAction(endpoint, pkg))
		case "api":
			bindings = append(bindings, bindStandaloneAPI(endpoint, pkg))
		}
	}
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].Source == bindings[j].Source {
			if bindings[i].Kind == bindings[j].Kind {
				return bindings[i].BlockName < bindings[j].BlockName
			}
			return bindings[i].Kind < bindings[j].Kind
		}
		return bindings[i].Source < bindings[j].Source
	})
	app.BackendBindings = bindings
	loadBindings := map[string]manifest.BackendBinding{}
	for _, binding := range bindings {
		if binding.Kind == loadHandlerKind {
			loadBindings[binding.PageID] = binding
		}
	}
	for index := range app.Pages {
		if binding, ok := loadBindings[app.Pages[index].ID]; ok {
			app.Pages[index].LoadBinding = binding
		}
	}
	return app
}

func bindLoad(page manifest.Page, pkg featurePackage) manifest.BackendBinding {
	functionName := loadFunctionName(page.ID)
	binding := baseBackendBinding(page, loadHandlerKind, functionName, "GET", page.Route, pkg)
	function, ok := pkg.Functions[functionName]
	if !ok {
		binding.Status = manifest.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK SSR load handler %s.%s is not implemented", packageLabel(pkg), functionName)
		return binding
	}
	if !function.Load() {
		binding.Status = manifest.BackendBindingUnsupportedSignature
		binding.Message = fmt.Sprintf("GOWDK SSR load handler %s.%s must have signature func(ssr.LoadContext) map[string]any or func(ssr.LoadContext) (map[string]any, error)", packageLabel(pkg), functionName)
		return binding
	}
	binding.Signature = function.Signature
	binding.Status = manifest.BackendBindingBound
	return binding
}

func bindStandaloneAction(endpoint manifest.EndpointDeclaration, pkg featurePackage) manifest.BackendBinding {
	method := endpoint.Method
	if method == "" {
		method = "POST"
	}
	binding := baseStandaloneBackendBinding(endpoint, actionHandlerKind, method, pkg)
	function, ok := pkg.Functions[binding.FunctionName]
	if !ok {
		binding.Status = manifest.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK action handler %s.%s is not implemented", packageLabel(pkg), binding.FunctionName)
		return binding
	}
	if !function.Action() {
		binding.Status = manifest.BackendBindingUnsupportedSignature
		if function.SupportMessage != "" {
			binding.Message = fmt.Sprintf("GOWDK action handler %s.%s is unsupported: %s", packageLabel(pkg), binding.FunctionName, function.SupportMessage)
		} else {
			binding.Message = fmt.Sprintf("GOWDK action handler %s.%s must have signature func(context.Context) (response.Response, error), func(context.Context, Input) (response.Response, error), func(context.Context, *Input) (response.Response, error), or func(context.Context, form.Values) (response.Response, error)", packageLabel(pkg), binding.FunctionName)
		}
		return binding
	}
	binding.Signature = function.Signature
	binding.InputType = function.InputType
	binding.InputPointer = function.InputPointer
	binding.InputFields = function.InputFields
	binding.Status = manifest.BackendBindingBound
	return binding
}

func bindStandaloneAPI(endpoint manifest.EndpointDeclaration, pkg featurePackage) manifest.BackendBinding {
	method := endpoint.Method
	if method == "" {
		method = "GET"
	}
	binding := baseStandaloneBackendBinding(endpoint, apiHandlerKind, method, pkg)
	function, ok := pkg.Functions[binding.FunctionName]
	if !ok {
		binding.Status = manifest.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK API handler %s.%s is not implemented", packageLabel(pkg), binding.FunctionName)
		return binding
	}
	if !function.API() {
		binding.Status = manifest.BackendBindingUnsupportedSignature
		binding.Message = fmt.Sprintf("GOWDK API handler %s.%s must have signature func(context.Context, *http.Request) (response.Response, error)", packageLabel(pkg), binding.FunctionName)
		return binding
	}
	binding.Signature = function.Signature
	binding.Status = manifest.BackendBindingBound
	return binding
}

func bindAction(page manifest.Page, action manifest.Action, pkg featurePackage) manifest.BackendBinding {
	method := action.Method
	if method == "" {
		method = "POST"
	}
	route := action.Route
	if route == "" {
		route = page.Route
	}
	binding := baseBackendBinding(page, actionHandlerKind, action.Name, method, route, pkg)
	function, ok := pkg.Functions[binding.FunctionName]
	if !ok {
		binding.Status = manifest.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK action handler %s.%s is not implemented", packageLabel(pkg), binding.FunctionName)
		return binding
	}
	if !function.Action() {
		binding.Status = manifest.BackendBindingUnsupportedSignature
		if function.SupportMessage != "" {
			binding.Message = fmt.Sprintf("GOWDK action handler %s.%s is unsupported: %s", packageLabel(pkg), binding.FunctionName, function.SupportMessage)
		} else {
			binding.Message = fmt.Sprintf("GOWDK action handler %s.%s must have signature func(context.Context) (response.Response, error), func(context.Context, Input) (response.Response, error), func(context.Context, *Input) (response.Response, error), or func(context.Context, form.Values) (response.Response, error)", packageLabel(pkg), binding.FunctionName)
		}
		return binding
	}
	binding.Signature = function.Signature
	binding.InputType = function.InputType
	binding.InputPointer = function.InputPointer
	binding.InputFields = function.InputFields
	binding.Status = manifest.BackendBindingBound
	return binding
}

func bindAPI(page manifest.Page, api manifest.API, pkg featurePackage) manifest.BackendBinding {
	method := strings.TrimSpace(api.Method)
	if method == "" {
		method = "GET"
	}
	route := strings.TrimSpace(api.Route)
	if route == "" {
		route = page.Route
	}
	binding := baseBackendBinding(page, apiHandlerKind, api.Name, method, route, pkg)
	function, ok := pkg.Functions[binding.FunctionName]
	if !ok {
		binding.Status = manifest.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK API handler %s.%s is not implemented", packageLabel(pkg), binding.FunctionName)
		return binding
	}
	if !function.API() {
		binding.Status = manifest.BackendBindingUnsupportedSignature
		binding.Message = fmt.Sprintf("GOWDK API handler %s.%s must have signature func(context.Context, *http.Request) (response.Response, error)", packageLabel(pkg), binding.FunctionName)
		return binding
	}
	binding.Signature = function.Signature
	binding.Status = manifest.BackendBindingBound
	return binding
}

func bindFragment(page manifest.Page, fragment manifest.FragmentEndpoint, pkg featurePackage) (manifest.BackendBinding, bool) {
	method := strings.TrimSpace(fragment.Method)
	if method == "" {
		method = "GET"
	}
	binding := baseBackendBinding(page, fragmentHandlerKind, fragment.Name, method, strings.TrimSpace(fragment.Route), pkg)
	function, ok := pkg.Functions[binding.FunctionName]
	if !ok {
		return manifest.BackendBinding{}, false
	}
	if !function.Fragment() {
		binding.Status = manifest.BackendBindingUnsupportedSignature
		binding.Message = fmt.Sprintf("GOWDK fragment handler %s.%s must have signature func(context.Context) (response.Response, error)", packageLabel(pkg), binding.FunctionName)
		return binding, true
	}
	binding.Signature = manifest.BackendSignatureFragment
	binding.Status = manifest.BackendBindingBound
	return binding, true
}

func baseBackendBinding(page manifest.Page, kind, blockName, method, route string, pkg featurePackage) manifest.BackendBinding {
	return manifest.BackendBinding{
		Kind:         kind,
		PageID:       page.ID,
		Source:       page.Source,
		BlockName:    blockName,
		Method:       method,
		Route:        route,
		ImportPath:   pkg.ImportPath,
		PackageName:  pkg.Name,
		FunctionName: blockName,
		Status:       manifest.BackendBindingMissing,
	}
}

func baseStandaloneBackendBinding(endpoint manifest.EndpointDeclaration, kind, method string, pkg featurePackage) manifest.BackendBinding {
	return manifest.BackendBinding{
		Kind:         kind,
		PageID:       standaloneEndpointPageID(endpoint),
		Source:       endpoint.Source,
		BlockName:    endpoint.Name,
		Method:       method,
		Route:        endpoint.Route,
		ImportPath:   pkg.ImportPath,
		PackageName:  pkg.Name,
		FunctionName: endpoint.Name,
		Status:       manifest.BackendBindingMissing,
	}
}

func packageLabel(pkg featurePackage) string {
	if pkg.ImportPath != "" {
		return pkg.ImportPath
	}
	if pkg.Name != "" {
		return pkg.Name
	}
	return "feature"
}

func sourceDir(source string) string {
	if strings.TrimSpace(source) == "" {
		return "."
	}
	return filepath.Dir(source)
}

type featurePackage struct {
	Dir        string
	ImportPath string
	Name       string
	Functions  map[string]featureFunction
}

type inputStruct struct {
	Fields  []manifest.BackendInputField
	Message string
}

type featureFunction struct {
	Name           string
	Signature      manifest.BackendSignatureKind
	InputType      string
	InputPointer   bool
	InputFields    []manifest.BackendInputField
	SupportMessage string
}

func (function featureFunction) Action() bool {
	switch function.Signature {
	case manifest.BackendSignatureAction0, manifest.BackendSignatureActionValues, manifest.BackendSignatureActionForm, manifest.BackendSignatureActionFormPtr:
		return true
	default:
		return false
	}
}

func (function featureFunction) API() bool {
	return function.Signature == manifest.BackendSignatureAPI
}

func (function featureFunction) Fragment() bool {
	return function.Signature == manifest.BackendSignatureAction0 || function.Signature == manifest.BackendSignatureFragment
}

func (function featureFunction) Load() bool {
	return function.Signature == manifest.BackendSignatureLoad || function.Signature == manifest.BackendSignatureLoadError
}

func inspectFeaturePackage(dir string) featurePackage {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}
	pkg := featurePackage{Dir: absDir, Functions: map[string]featureFunction{}}
	info := goListDir(absDir)
	pkg.ImportPath = info.ImportPath
	pkg.Name = info.Name

	entries, err := os.ReadDir(absDir)
	if err != nil {
		return pkg
	}
	fileSet := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		filePath := filepath.Join(absDir, entry.Name())
		file, err := parser.ParseFile(fileSet, filePath, nil, 0)
		if err != nil {
			continue
		}
		if pkg.Name == "" {
			pkg.Name = file.Name.Name
		}
		files = append(files, file)
	}

	inputStructs := collectInputStructs(files)
	for _, file := range files {
		imports := astImportAliases(file)
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil || !fn.Name.IsExported() {
				continue
			}
			signature, inputType, inputPointer := backendSignature(fn.Type, imports)
			var inputFields []manifest.BackendInputField
			var supportMessage string
			if signature == manifest.BackendSignatureActionForm || signature == manifest.BackendSignatureActionFormPtr {
				inputStruct, ok := inputStructs[inputType]
				if !ok {
					supportMessage = fmt.Sprintf("typed action input %s must be an exported struct in the same package", inputType)
					signature = ""
				} else if inputStruct.Message != "" {
					supportMessage = inputStruct.Message
					signature = ""
				} else {
					inputFields = append([]manifest.BackendInputField(nil), inputStruct.Fields...)
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

func collectInputStructs(files []*ast.File) map[string]inputStruct {
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
				structs[typeSpec.Name.Name] = backendInputStruct(typeSpec.Name.Name, structType)
			}
		}
	}
	return structs
}

func backendInputStruct(typeName string, structType *ast.StructType) inputStruct {
	if structType == nil || structType.Fields == nil {
		return inputStruct{}
	}
	seen := map[string]bool{}
	var fields []manifest.BackendInputField
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			return inputStruct{Message: fmt.Sprintf("typed action input %s cannot use embedded fields", typeName)}
		}
		formName, skip, explicit, err := formTagName(field)
		if err != nil {
			return inputStruct{Message: fmt.Sprintf("typed action input %s has invalid form tag: %v", typeName, err)}
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
			return inputStruct{Message: fmt.Sprintf("typed action input %s cannot reuse one explicit form tag across multiple fields", typeName)}
		}
		fieldType, ok := backendInputFieldType(field.Type)
		if !ok {
			return inputStruct{Message: fmt.Sprintf("typed action input %s uses unsupported field type", typeName)}
		}
		for _, name := range exportedNames {
			nameFormName := formName
			if nameFormName == "" {
				nameFormName = name.Name
			}
			if seen[nameFormName] {
				return inputStruct{Message: fmt.Sprintf("typed action input %s maps multiple fields to form field %q", typeName, nameFormName)}
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

func formTagName(field *ast.Field) (string, bool, bool, error) {
	if field == nil || field.Tag == nil {
		return "", false, false, nil
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false, false, err
	}
	value, ok, err := structTagValue(tag, "form")
	if err != nil || !ok {
		return "", false, ok, err
	}
	name, _, _ := strings.Cut(value, ",")
	if name == "-" {
		return "", true, true, nil
	}
	return strings.TrimSpace(name), false, true, nil
}

func structTagValue(tag string, key string) (string, bool, error) {
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

func backendInputFieldType(expression ast.Expr) (string, bool) {
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

type goListDirInfo struct {
	ImportPath string
	Name       string
}

func goListDir(dir string) goListDirInfo {
	command := exec.Command("go", "list", "-json", ".")
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		return goListDirInfo{}
	}
	var info goListDirInfo
	if err := json.Unmarshal(output, &info); err != nil {
		return goListDirInfo{}
	}
	return info
}

func astImportAliases(file *ast.File) map[string]string {
	imports := map[string]string{}
	for _, spec := range file.Imports {
		importPath := strings.Trim(spec.Path.Value, `"`)
		if importPath == "" {
			continue
		}
		name := path.Base(importPath)
		if spec.Name != nil && spec.Name.Name != "" && spec.Name.Name != "." && spec.Name.Name != "_" {
			name = spec.Name.Name
		}
		imports[name] = importPath
	}
	return imports
}

func backendSignature(function *ast.FuncType, imports map[string]string) (manifest.BackendSignatureKind, string, bool) {
	if kind, inputType, inputPointer, ok := actionSignature(function, imports); ok {
		return kind, inputType, inputPointer
	}
	if isAPISignature(function, imports) {
		return manifest.BackendSignatureAPI, "", false
	}
	if signature, ok := loadSignature(function, imports); ok {
		return signature, "", false
	}
	return "", "", false
}

func actionSignature(function *ast.FuncType, imports map[string]string) (manifest.BackendSignatureKind, string, bool, bool) {
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
		return manifest.BackendSignatureAction0, "", false, true
	}
	second := function.Params.List[1].Type
	if isSelector(second, imports, formImportPath, "Values") {
		return manifest.BackendSignatureActionValues, "", false, true
	}
	if ident, ok := second.(*ast.Ident); ok && ident.IsExported() {
		return manifest.BackendSignatureActionForm, ident.Name, false, true
	}
	if pointer, ok := second.(*ast.StarExpr); ok {
		if ident, ok := pointer.X.(*ast.Ident); ok && ident.IsExported() {
			return manifest.BackendSignatureActionFormPtr, ident.Name, true, true
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

func loadSignature(function *ast.FuncType, imports map[string]string) (manifest.BackendSignatureKind, bool) {
	if function == nil || function.Params == nil || function.Results == nil {
		return "", false
	}
	if len(function.Params.List) != 1 || !isSelector(function.Params.List[0].Type, imports, ssrImportPath, "LoadContext") {
		return "", false
	}
	if len(function.Results.List) == 1 && isMapStringAny(function.Results.List[0].Type) {
		return manifest.BackendSignatureLoad, true
	}
	if len(function.Results.List) == 2 && isMapStringAny(function.Results.List[0].Type) && isError(function.Results.List[1].Type) {
		return manifest.BackendSignatureLoadError, true
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

func loadFunctionName(pageID string) string {
	return "Load" + exportedIdentifier(pageID)
}

func exportedIdentifier(value string) string {
	var builder strings.Builder
	uppercaseNext := true
	for _, char := range strings.TrimSpace(value) {
		if char >= 'a' && char <= 'z' {
			if uppercaseNext {
				char = char - 'a' + 'A'
			}
			builder.WriteRune(char)
			uppercaseNext = false
			continue
		}
		if char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' {
			if builder.Len() == 0 && char >= '0' && char <= '9' {
				builder.WriteByte('P')
			}
			builder.WriteRune(char)
			uppercaseNext = false
			continue
		}
		uppercaseNext = true
	}
	if builder.Len() == 0 {
		return "Page"
	}
	return builder.String()
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

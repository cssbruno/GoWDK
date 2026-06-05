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
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/manifest"
)

const (
	actionHandlerKind = "action"
	apiHandlerKind    = "api"

	contextImportPath  = "context"
	formImportPath     = "github.com/cssbruno/gowdk/runtime/form"
	httpImportPath     = "net/http"
	responseImportPath = "github.com/cssbruno/gowdk/runtime/response"
)

// BindBackendHandlers discovers same-package Go handlers for act and api blocks.
// Discovery is intentionally non-fatal: missing packages, missing functions, and
// unsupported signatures are reported as binding metadata so generated apps can
// emit clear 501 responses.
func BindBackendHandlers(app manifest.Manifest) manifest.Manifest {
	var bindings []manifest.BackendBinding
	cache := map[string]featurePackage{}
	for _, page := range app.Pages {
		if len(page.Blocks.Actions) == 0 && len(page.Blocks.APIs) == 0 {
			continue
		}
		dir := sourceDir(page.Source)
		pkg, ok := cache[dir]
		if !ok {
			pkg = inspectFeaturePackage(dir)
			cache[dir] = pkg
		}
		for _, action := range page.Blocks.Actions {
			bindings = append(bindings, bindAction(page, action, pkg))
		}
		for _, api := range page.Blocks.APIs {
			bindings = append(bindings, bindAPI(page, api, pkg))
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
	return app
}

func bindAction(page manifest.Page, action manifest.Action, pkg featurePackage) manifest.BackendBinding {
	route := page.Route
	binding := baseBackendBinding(page, actionHandlerKind, action.Name, "POST", route, pkg)
	function, ok := pkg.Functions[binding.FunctionName]
	if !ok {
		binding.Status = manifest.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK action handler %s.%s is not implemented", packageLabel(pkg), binding.FunctionName)
		return binding
	}
	if !function.Action {
		binding.Status = manifest.BackendBindingUnsupportedSignature
		binding.Message = fmt.Sprintf("GOWDK action handler %s.%s must have signature func(context.Context, form.Values) (response.Response, error)", packageLabel(pkg), binding.FunctionName)
		return binding
	}
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
	if !function.API {
		binding.Status = manifest.BackendBindingUnsupportedSignature
		binding.Message = fmt.Sprintf("GOWDK API handler %s.%s must have signature func(context.Context, *http.Request) (response.Response, error)", packageLabel(pkg), binding.FunctionName)
		return binding
	}
	binding.Status = manifest.BackendBindingBound
	return binding
}

func baseBackendBinding(page manifest.Page, kind, blockName, method, route string, pkg featurePackage) manifest.BackendBinding {
	functionName := exportedBackendFunction(blockName)
	return manifest.BackendBinding{
		Kind:         kind,
		PageID:       page.ID,
		Source:       page.Source,
		BlockName:    blockName,
		Method:       method,
		Route:        route,
		ImportPath:   pkg.ImportPath,
		PackageName:  pkg.Name,
		FunctionName: functionName,
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

func exportedBackendFunction(value string) string {
	var builder strings.Builder
	upperNext := true
	for _, char := range strings.TrimSpace(value) {
		valid := char == '_' || unicode.IsLetter(char) || unicode.IsDigit(char)
		if !valid || char == '_' {
			upperNext = true
			continue
		}
		if builder.Len() == 0 && unicode.IsDigit(char) {
			builder.WriteByte('X')
		}
		if upperNext {
			builder.WriteRune(unicode.ToUpper(char))
			upperNext = false
			continue
		}
		builder.WriteRune(char)
	}
	if builder.Len() == 0 {
		return "Handler"
	}
	return builder.String()
}

type featurePackage struct {
	Dir        string
	ImportPath string
	Name       string
	Functions  map[string]featureFunction
}

type featureFunction struct {
	Name   string
	Action bool
	API    bool
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
		imports := astImportAliases(file)
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name == nil || !fn.Name.IsExported() {
				continue
			}
			pkg.Functions[fn.Name.Name] = featureFunction{
				Name:   fn.Name.Name,
				Action: isActionSignature(fn.Type, imports),
				API:    isAPISignature(fn.Type, imports),
			}
		}
	}
	return pkg
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

func isActionSignature(function *ast.FuncType, imports map[string]string) bool {
	if function == nil || function.Params == nil || function.Results == nil {
		return false
	}
	if len(function.Params.List) != 2 || len(function.Results.List) != 2 {
		return false
	}
	return isSelector(function.Params.List[0].Type, imports, contextImportPath, "Context") &&
		isSelector(function.Params.List[1].Type, imports, formImportPath, "Values") &&
		isSelector(function.Results.List[0].Type, imports, responseImportPath, "Response") &&
		isError(function.Results.List[1].Type)
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

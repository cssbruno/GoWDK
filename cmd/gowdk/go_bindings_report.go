package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

type goBindingsReport struct {
	Version  int             `json:"version"`
	Bindings []goBindingJSON `json:"bindings"`
}

type goBindingJSON struct {
	Kind           string                    `json:"kind"`
	Source         string                    `json:"source,omitempty"`
	SourceSpan     *sourceSpanJSON           `json:"sourceSpan,omitempty"`
	Package        string                    `json:"package,omitempty"`
	PageID         string                    `json:"pageId,omitempty"`
	Symbol         string                    `json:"symbol,omitempty"`
	ExpectedSymbol string                    `json:"expectedSymbol,omitempty"`
	ImportAlias    string                    `json:"importAlias,omitempty"`
	PackagePath    string                    `json:"packagePath,omitempty"`
	PackageName    string                    `json:"packageName,omitempty"`
	Method         string                    `json:"method,omitempty"`
	Route          string                    `json:"route,omitempty"`
	Status         string                    `json:"status"`
	Signature      string                    `json:"signature,omitempty"`
	InputType      string                    `json:"inputType,omitempty"`
	InputFields    []goBindingInputFieldJSON `json:"inputFields,omitempty"`
	Message        string                    `json:"message,omitempty"`
	Suggestion     string                    `json:"suggestion,omitempty"`
}

type goBindingInputFieldJSON struct {
	FieldName string `json:"fieldName"`
	FormName  string `json:"formName"`
	Type      string `json:"type"`
}

func buildGoBindingsReport(config gowdk.Config, ir gwdkir.Program) goBindingsReport {
	var bindings []goBindingJSON
	metadata := compiler.BuildRouteMetadataFromIR(config, ir)
	inputFields := goBindingInputFieldsByEndpoint(ir)
	for _, endpoint := range metadata.Endpoints {
		if endpoint.Contract.Name != "" {
			bindings = append(bindings, contractGoBinding(endpoint))
			continue
		}
		binding := endpointGoBinding(endpoint)
		binding.InputFields = backendInputFieldsJSON(inputFields[goBindingEndpointKey(string(endpoint.Kind), endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Route)])
		bindings = append(bindings, binding)
	}
	for _, page := range ir.Pages {
		if page.Blocks.Load {
			bindings = append(bindings, loadGoBinding(page))
		}
		bindings = append(bindings, buildDataGoBindings(page)...)
	}
	sort.SliceStable(bindings, func(i, j int) bool {
		left, right := bindings[i], bindings[j]
		for _, cmp := range []int{
			strings.Compare(left.Source, right.Source),
			strings.Compare(left.Kind, right.Kind),
			strings.Compare(left.PageID, right.PageID),
			strings.Compare(left.Symbol, right.Symbol),
			strings.Compare(left.Route, right.Route),
		} {
			if cmp < 0 {
				return true
			}
			if cmp > 0 {
				return false
			}
		}
		return false
	})
	return goBindingsReport{Version: 1, Bindings: bindings}
}

func goBindingInputFieldsByEndpoint(ir gwdkir.Program) map[string][]source.BackendInputField {
	out := map[string][]source.BackendInputField{}
	for _, endpoint := range ir.Endpoints {
		key := goBindingEndpointKey(string(endpoint.Kind), endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Path)
		out[key] = append([]source.BackendInputField(nil), endpoint.Binding.InputFields...)
	}
	return out
}

func goBindingEndpointKey(kind, pageID, symbol, method, route string) string {
	return strings.Join([]string{kind, pageID, symbol, method, route}, "\x00")
}

func endpointGoBinding(endpoint compiler.EndpointBinding) goBindingJSON {
	status := string(endpoint.BindingStatus)
	if status == "" {
		status = "unknown"
	}
	message := endpoint.BindingMessage
	suggestion := backendBindingSuggestion(string(endpoint.Kind), endpoint.Symbol, status)
	if status == "unknown" && endpoint.Kind == compiler.EndpointFragment {
		message = "fragment has no attached Go binding; static .gwdk fragment output does not require a Go handler"
		suggestion = "Add an exported fragment handler with func(context.Context) (response.Response, error) when Go-backed fragment behavior is needed."
	}
	return goBindingJSON{
		Kind:           string(endpoint.Kind),
		Source:         endpoint.Source,
		SourceSpan:     endpointSourceSpanJSON(endpoint.SourceSpan),
		Package:        endpoint.Package,
		PageID:         endpoint.PageID,
		Symbol:         endpoint.Symbol,
		ExpectedSymbol: endpoint.Symbol,
		PackagePath:    endpoint.BindingImportPath,
		PackageName:    endpoint.BindingPackage,
		Method:         endpoint.Method,
		Route:          endpoint.Route,
		Status:         status,
		Signature:      string(endpoint.BindingSignature),
		InputType:      endpoint.BindingInputType,
		Message:        message,
		Suggestion:     suggestion,
	}
}

func contractGoBinding(endpoint compiler.EndpointBinding) goBindingJSON {
	status := string(endpoint.Contract.Status)
	if status == "" {
		status = "unknown"
	}
	return goBindingJSON{
		Kind:           string(endpoint.Kind),
		Source:         endpoint.Source,
		SourceSpan:     endpointSourceSpanJSON(endpoint.SourceSpan),
		Package:        endpoint.Package,
		PageID:         endpoint.PageID,
		Symbol:         endpoint.Contract.Name,
		ExpectedSymbol: endpoint.Contract.Name,
		ImportAlias:    endpoint.Contract.ImportAlias,
		PackagePath:    endpoint.Contract.ImportPath,
		Method:         endpoint.Method,
		Route:          endpoint.Route,
		Status:         status,
		Signature:      endpoint.Contract.Handler,
		InputType:      endpoint.Contract.Type,
		Message:        endpoint.Contract.Message,
		Suggestion:     contractBindingSuggestion(string(endpoint.Kind), endpoint.Contract.Name, status),
	}
}

func loadGoBinding(page gwdkir.Page) goBindingJSON {
	status := string(page.LoadBinding.Status)
	if status == "" {
		status = "unknown"
	}
	symbol := page.LoadBinding.FunctionName
	if symbol == "" {
		symbol = "Load" + source.ExportedIdentifier(page.ID, "Page")
	}
	return goBindingJSON{
		Kind:           "load",
		Source:         page.Source,
		SourceSpan:     endpointSourceSpanJSON(page.Blocks.Spans.Load),
		Package:        page.Package,
		PageID:         page.ID,
		Symbol:         symbol,
		ExpectedSymbol: symbol,
		PackagePath:    page.LoadBinding.ImportPath,
		PackageName:    page.LoadBinding.PackageName,
		Method:         "GET",
		Route:          page.Route,
		Status:         status,
		Signature:      string(page.LoadBinding.Signature),
		Message:        page.LoadBinding.Message,
		Suggestion:     backendBindingSuggestion("load", symbol, status),
	}
}

func buildDataGoBindings(page gwdkir.Page) []goBindingJSON {
	if !page.Blocks.Build {
		return nil
	}
	ref, ok := goBindingBuildDataCall(page)
	if !ok {
		var err error
		ref, ok, err = parseGoBindingBuildDataCall(page.Blocks.BuildBody)
		if err != nil {
			return []goBindingJSON{{
				Kind:       "build",
				Source:     page.Source,
				SourceSpan: endpointSourceSpanJSON(page.Blocks.Spans.Build),
				Package:    page.Package,
				PageID:     page.ID,
				Status:     "invalid",
				Message:    err.Error(),
			}}
		}
		if !ok {
			return nil
		}
	}
	binding := goBindingJSON{
		Kind:           "build",
		Source:         page.Source,
		SourceSpan:     endpointSourceSpanJSON(page.Blocks.Spans.Build),
		Package:        page.Package,
		PageID:         page.ID,
		Symbol:         ref.Function,
		ExpectedSymbol: ref.Function,
		ImportAlias:    ref.Alias,
		Status:         "unverified",
		Message:        "build data function is executed by gowdk build",
		Suggestion:     "Run gowdk build to execute the function and validate JSON-encodable output.",
	}
	if ref.Alias != "" {
		if imported, ok := findGoBindingImport(page.Imports, ref.Alias); ok {
			binding.PackagePath = imported.Path
		} else {
			binding.Status = "missing"
			binding.Message = fmt.Sprintf("build import %q is not declared", ref.Alias)
			binding.Suggestion = "Add a matching Go import declaration or change the build function alias."
		}
		return []goBindingJSON{binding}
	}
	importPath, err := inspectSamePackageImportPath(page.Source)
	if err != nil {
		binding.Status = "missing"
		binding.Message = err.Error()
		binding.Suggestion = "Make the page directory a buildable Go package or import the build-data package explicitly."
		return []goBindingJSON{binding}
	}
	binding.PackagePath = importPath
	binding.PackageName = page.Package
	return []goBindingJSON{binding}
}

func goBindingBuildDataCall(page gwdkir.Page) (goBindingBuildCallRef, bool) {
	if page.Blocks.BuildCall == nil {
		return goBindingBuildCallRef{}, false
	}
	return goBindingBuildCallRef{
		Alias:    page.Blocks.BuildCall.Alias,
		Function: page.Blocks.BuildCall.Function,
	}, true
}

type goBindingBuildCallRef struct {
	Alias    string
	Function string
}

func parseGoBindingBuildDataCall(body string) (goBindingBuildCallRef, bool, error) {
	lines := goBindingSignificantLines(body)
	if len(lines) != 1 {
		return goBindingBuildCallRef{}, false, nil
	}
	expr, ok := strings.CutPrefix(strings.TrimSpace(lines[0]), "=>")
	if !ok {
		return goBindingBuildCallRef{}, false, nil
	}
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "{") {
		return goBindingBuildCallRef{}, false, nil
	}
	parsed, err := parser.ParseExpr(expr)
	if err != nil {
		return goBindingBuildCallRef{}, true, fmt.Errorf("parse build call: %w", err)
	}
	call, ok := parsed.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return goBindingBuildCallRef{}, false, nil
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return goBindingBuildCallRef{Function: fun.Name}, true, nil
	case *ast.SelectorExpr:
		alias, ok := fun.X.(*ast.Ident)
		if !ok {
			return goBindingBuildCallRef{}, true, fmt.Errorf("build data call receiver must be an import alias")
		}
		return goBindingBuildCallRef{Alias: alias.Name, Function: fun.Sel.Name}, true, nil
	default:
		return goBindingBuildCallRef{}, false, nil
	}
}

func goBindingSignificantLines(body string) []string {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func findGoBindingImport(imports []gwdkir.Import, alias string) (gwdkir.Import, bool) {
	for _, item := range imports {
		if item.Alias == alias {
			return item, true
		}
	}
	return gwdkir.Import{}, false
}

func inspectSamePackageImportPath(sourcePath string) (string, error) {
	dir := "."
	if strings.TrimSpace(sourcePath) != "" {
		dir = filepath.Dir(sourcePath)
	}
	command := exec.Command("go", "list", "-json", ".")
	command.Dir = dir
	output, err := command.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if stderr := strings.TrimSpace(string(exit.Stderr)); stderr != "" {
				return "", fmt.Errorf("same-package build data function requires a buildable Go package for %s: %w\n%s", dir, err, stderr)
			}
		}
		return "", fmt.Errorf("same-package build data function requires a buildable Go package for %s: %w", dir, err)
	}
	var info struct {
		ImportPath string
	}
	if err := json.Unmarshal(output, &info); err != nil {
		return "", fmt.Errorf("inspect Go package for %s: %w", dir, err)
	}
	if strings.TrimSpace(info.ImportPath) == "" {
		return "", fmt.Errorf("same-package build data function requires a buildable Go package for %s", dir)
	}
	return info.ImportPath, nil
}

func backendBindingSuggestion(kind, symbol, status string) string {
	switch status {
	case string(source.BackendBindingMissing):
		switch kind {
		case "action":
			return fmt.Sprintf("Add exported function %s with a supported action signature, or run gowdk generate stubs.", symbol)
		case "api":
			return fmt.Sprintf("Add exported function %s with func(context.Context, *http.Request) (response.Response, error), or run gowdk generate stubs.", symbol)
		case "load":
			return fmt.Sprintf("Add exported function %s with func(ssr.LoadContext) map[string]any or func(ssr.LoadContext) (map[string]any, error).", symbol)
		default:
			return fmt.Sprintf("Add exported function %s with the supported %s signature.", symbol, kind)
		}
	case string(source.BackendBindingUnsupportedSignature):
		return fmt.Sprintf("Change %s to a supported %s signature.", symbol, kind)
	default:
		return ""
	}
}

func contractBindingSuggestion(kind, symbol, status string) string {
	switch status {
	case string(gwdkir.ContractBindingMissing):
		return fmt.Sprintf("Register %s contract %s with the web role or remove the generated web reference.", kind, symbol)
	case string(gwdkir.ContractBindingInvalid):
		return fmt.Sprintf("Fix the Go contract registration or handler signature for %s.", symbol)
	default:
		return ""
	}
}

func backendInputFieldsJSON(fields []source.BackendInputField) []goBindingInputFieldJSON {
	if len(fields) == 0 {
		return nil
	}
	out := make([]goBindingInputFieldJSON, 0, len(fields))
	for _, field := range fields {
		out = append(out, goBindingInputFieldJSON{
			FieldName: field.FieldName,
			FormName:  field.FormName,
			Type:      field.Type,
		})
	}
	return out
}

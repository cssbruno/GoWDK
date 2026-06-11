package compiler

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// BindBackendHandlers discovers same-package Go handlers for act and api blocks,
// attaches the resulting binding metadata to the program's endpoints and page
// load bindings, and returns the full binding record list (sorted by source,
// kind, and block name) for reporting and public manifest JSON.
// Discovery is intentionally non-fatal: missing packages, missing functions, and
// unsupported signatures are reported as binding metadata so generated apps can
// emit clear 501 responses.
func BindBackendHandlers(ir *gwdkir.Program) []source.BackendBinding {
	bindings := computeBackendBindings(*ir)
	gwdkanalysis.AttachBackendBindings(ir, bindings)
	return bindings
}

// computeBackendBindings derives the binding records without mutating the
// program, for callers that only need the records (e.g. the production binding
// policy check on an unbound program).
func computeBackendBindings(ir gwdkir.Program) []source.BackendBinding {
	var bindings []source.BackendBinding
	cache := map[string]featurePackage{}
	for _, page := range ir.Pages {
		if len(page.Blocks.Actions) == 0 && len(page.Blocks.APIs) == 0 && len(page.Blocks.Fragments) == 0 && !page.Blocks.Load {
			continue
		}
		dir := sourceDir(page.Source)
		pkg, ok := cache[dir]
		if !ok {
			pkg = inspectFeaturePackage(dir)
			cache[dir] = pkg
		}
		var inlinePkg featurePackage
		inlineLoaded := false
		defaultInlinePkg := func() featurePackage {
			if !inlineLoaded {
				inlinePkg = inspectInlineScriptFeaturePackage(page, "")
				inlineLoaded = true
			}
			return inlinePkg
		}
		if page.Blocks.Load {
			bindings = append(bindings, bindLoad(page, pkg))
		}
		for _, action := range page.Blocks.Actions {
			binding := bindAction(page, action, pkg)
			if binding.Status == source.BackendBindingMissing {
				inlineBinding := bindAction(page, action, defaultInlinePkg())
				if inlineBinding.Status != source.BackendBindingMissing {
					binding = inlineBinding
				}
			}
			bindings = append(bindings, binding)
		}
		for _, api := range page.Blocks.APIs {
			binding := bindAPI(page, api, pkg)
			if binding.Status == source.BackendBindingMissing {
				inlineBinding := bindAPI(page, api, defaultInlinePkg())
				if inlineBinding.Status != source.BackendBindingMissing {
					binding = inlineBinding
				}
			}
			bindings = append(bindings, binding)
		}
		for _, fragment := range page.Blocks.Fragments {
			if binding, ok := bindFragment(page, fragment, pkg); ok {
				bindings = append(bindings, binding)
			} else if binding, ok := bindFragment(page, fragment, defaultInlinePkg()); ok {
				bindings = append(bindings, binding)
			}
		}
	}
	for _, endpoint := range ir.GoEndpoints {
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
	return bindings
}

func bindLoad(page gwdkir.Page, pkg featurePackage) source.BackendBinding {
	functionName := loadFunctionName(page.ID)
	if function, ok := pkg.Functions[functionName]; ok {
		binding := baseBackendBinding(page, loadHandlerKind, functionName, "GET", page.Route, pkg)
		if !function.Load() {
			binding.Status = source.BackendBindingUnsupportedSignature
			binding.Message = fmt.Sprintf("GOWDK SSR load handler %s.%s must have signature func(ssr.LoadContext) map[string]any or func(ssr.LoadContext) (map[string]any, error)", bindingPackageLabel(binding, pkg), functionName)
			return binding
		}
		binding.Signature = function.Signature
		binding.Status = source.BackendBindingBound
		return binding
	}
	inlinePkg := inspectInlineScriptFeaturePackage(page, "ssr")
	if function, ok := inlinePkg.Functions[functionName]; ok {
		binding := baseBackendBinding(page, loadHandlerKind, functionName, "GET", page.Route, inlinePkg)
		if !function.Load() {
			binding.Status = source.BackendBindingUnsupportedSignature
			binding.Message = fmt.Sprintf("GOWDK SSR load handler %s.%s must have signature func(ssr.LoadContext) map[string]any or func(ssr.LoadContext) (map[string]any, error)", bindingPackageLabel(binding, inlinePkg), functionName)
			return binding
		}
		binding.Signature = function.Signature
		binding.Status = source.BackendBindingBound
		return binding
	}
	binding := baseBackendBinding(page, loadHandlerKind, functionName, "GET", page.Route, pkg)
	binding.Status = source.BackendBindingMissing
	if pkg.LoadError != "" {
		binding.Message = fmt.Sprintf("GOWDK SSR load handler %s.%s could not be inspected: %s", bindingPackageLabel(binding, pkg), functionName, pkg.LoadError)
	} else {
		binding.Message = fmt.Sprintf("GOWDK SSR load handler %s.%s is not implemented", bindingPackageLabel(binding, pkg), functionName)
	}
	return binding
}

func bindStandaloneAction(endpoint gwdkir.GoEndpoint, pkg featurePackage) source.BackendBinding {
	method := endpoint.Method
	if method == "" {
		method = "POST"
	}
	return bindActionEndpoint(baseStandaloneBackendBinding(endpoint, actionHandlerKind, method, pkg), pkg)
}

func bindActionEndpoint(binding source.BackendBinding, pkg featurePackage) source.BackendBinding {
	if pkg.LoadError != "" {
		binding.Status = source.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK action handler %s.%s could not be inspected: %s", bindingPackageLabel(binding, pkg), binding.FunctionName, pkg.LoadError)
		return binding
	}
	function, ok := pkg.Functions[binding.FunctionName]
	if !ok {
		binding.Status = source.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK action handler %s.%s is not implemented", bindingPackageLabel(binding, pkg), binding.FunctionName)
		return binding
	}
	if !function.Action() {
		binding.Status = source.BackendBindingUnsupportedSignature
		if function.SupportMessage != "" {
			binding.Message = fmt.Sprintf("GOWDK action handler %s.%s is unsupported: %s", bindingPackageLabel(binding, pkg), binding.FunctionName, function.SupportMessage)
		} else {
			binding.Message = fmt.Sprintf("GOWDK action handler %s.%s must have signature func(context.Context) (response.Response, error), func(context.Context, Input) (response.Response, error), func(context.Context, *Input) (response.Response, error), or func(context.Context, form.Values) (response.Response, error)", bindingPackageLabel(binding, pkg), binding.FunctionName)
		}
		return binding
	}
	binding.Signature = function.Signature
	binding.InputType = function.InputType
	binding.InputPointer = function.InputPointer
	binding.InputFields = function.InputFields
	binding.Status = source.BackendBindingBound
	return binding
}

func bindStandaloneAPI(endpoint gwdkir.GoEndpoint, pkg featurePackage) source.BackendBinding {
	method := endpoint.Method
	if method == "" {
		method = "GET"
	}
	return bindAPIEndpoint(baseStandaloneBackendBinding(endpoint, apiHandlerKind, method, pkg), pkg)
}

func bindAPIEndpoint(binding source.BackendBinding, pkg featurePackage) source.BackendBinding {
	if pkg.LoadError != "" {
		binding.Status = source.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK API handler %s.%s could not be inspected: %s", bindingPackageLabel(binding, pkg), binding.FunctionName, pkg.LoadError)
		return binding
	}
	function, ok := pkg.Functions[binding.FunctionName]
	if !ok {
		binding.Status = source.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK API handler %s.%s is not implemented", bindingPackageLabel(binding, pkg), binding.FunctionName)
		return binding
	}
	if !function.API() {
		binding.Status = source.BackendBindingUnsupportedSignature
		binding.Message = fmt.Sprintf("GOWDK API handler %s.%s must have signature func(context.Context, *http.Request) (response.Response, error)", bindingPackageLabel(binding, pkg), binding.FunctionName)
		return binding
	}
	binding.Signature = function.Signature
	binding.Status = source.BackendBindingBound
	return binding
}

func bindAction(page gwdkir.Page, action gwdkir.Action, pkg featurePackage) source.BackendBinding {
	method := action.Method
	if method == "" {
		method = "POST"
	}
	route := action.Route
	if route == "" {
		route = page.Route
	}
	return bindActionEndpoint(baseBackendBinding(page, actionHandlerKind, action.Name, method, route, pkg), pkg)
}

func bindAPI(page gwdkir.Page, api gwdkir.API, pkg featurePackage) source.BackendBinding {
	method := strings.TrimSpace(api.Method)
	if method == "" {
		method = "GET"
	}
	route := strings.TrimSpace(api.Route)
	if route == "" {
		route = page.Route
	}
	return bindAPIEndpoint(baseBackendBinding(page, apiHandlerKind, api.Name, method, route, pkg), pkg)
}

func bindFragment(page gwdkir.Page, fragment gwdkir.FragmentEndpoint, pkg featurePackage) (source.BackendBinding, bool) {
	method := strings.TrimSpace(fragment.Method)
	if method == "" {
		method = "GET"
	}
	binding := baseBackendBinding(page, fragmentHandlerKind, fragment.Name, method, strings.TrimSpace(fragment.Route), pkg)
	function, ok := pkg.Functions[binding.FunctionName]
	if !ok {
		return source.BackendBinding{}, false
	}
	if !function.Fragment() {
		binding.Status = source.BackendBindingUnsupportedSignature
		binding.Message = fmt.Sprintf("GOWDK fragment handler %s.%s must have signature func(context.Context) (response.Response, error)", bindingPackageLabel(binding, pkg), binding.FunctionName)
		return binding, true
	}
	binding.Signature = source.BackendSignatureFragment
	binding.Status = source.BackendBindingBound
	return binding, true
}

func baseBackendBinding(page gwdkir.Page, kind, blockName, method, route string, pkg featurePackage) source.BackendBinding {
	return source.BackendBinding{
		Kind:         kind,
		PageID:       page.ID,
		Source:       page.Source,
		BlockName:    blockName,
		Method:       method,
		Route:        route,
		ImportPath:   pkg.ImportPath,
		PackageName:  bindingPackageName(pkg.Name, page.Package),
		FunctionName: blockName,
		Status:       source.BackendBindingMissing,
	}
}

func baseStandaloneBackendBinding(endpoint gwdkir.GoEndpoint, kind, method string, pkg featurePackage) source.BackendBinding {
	return source.BackendBinding{
		Kind:         kind,
		PageID:       standaloneEndpointPageID(endpoint.Package, endpoint.Name),
		Source:       endpoint.Source,
		BlockName:    endpoint.Name,
		Method:       method,
		Route:        endpoint.Route,
		ImportPath:   pkg.ImportPath,
		PackageName:  bindingPackageName(pkg.Name, endpoint.Package),
		FunctionName: endpoint.Name,
		Status:       source.BackendBindingMissing,
	}
}

func sourceDir(sourcePath string) string {
	if strings.TrimSpace(sourcePath) == "" {
		return "."
	}
	return filepath.Dir(sourcePath)
}

func loadFunctionName(pageID string) string {
	return "Load" + source.ExportedIdentifier(pageID, "Page")
}

// BackendBindingsFromIR derives the backend binding records already attached to
// the program's endpoints, without inspecting Go packages on disk. Callers that
// only need binding metadata for reporting (e.g. build reports) should use this
// instead of re-running handler discovery.
func BackendBindingsFromIR(ir gwdkir.Program) []source.BackendBinding {
	out := make([]source.BackendBinding, 0, len(ir.Endpoints))
	for _, page := range ir.Pages {
		if !page.Blocks.Load {
			continue
		}
		binding := loadBindingFromIR(page)
		if binding.Status != "" || binding.ImportPath != "" || binding.FunctionName != "" {
			out = append(out, binding)
		}
	}
	for _, endpoint := range ir.Endpoints {
		binding := backendBindingFromIR(endpoint)
		if binding.Status != "" || binding.ImportPath != "" || binding.FunctionName != "" {
			out = append(out, binding)
		}
	}
	return out
}

func loadBindingFromIR(page gwdkir.Page) source.BackendBinding {
	return source.BackendBinding{
		Kind:         loadHandlerKind,
		PageID:       page.ID,
		Source:       page.Source,
		Method:       "GET",
		Route:        page.Route,
		ImportPath:   page.LoadBinding.ImportPath,
		PackageName:  page.LoadBinding.PackageName,
		FunctionName: page.LoadBinding.FunctionName,
		Signature:    page.LoadBinding.Signature,
		InputType:    page.LoadBinding.InputType,
		InputPointer: page.LoadBinding.InputPointer,
		InputFields:  append([]source.BackendInputField(nil), page.LoadBinding.InputFields...),
		Status:       page.LoadBinding.Status,
		Message:      page.LoadBinding.Message,
	}
}

func backendBindingFromIR(endpoint gwdkir.Endpoint) source.BackendBinding {
	kind := "action"
	switch endpoint.Kind {
	case gwdkir.EndpointAPI:
		kind = "api"
	case gwdkir.EndpointFragment:
		kind = "fragment"
	}
	return source.BackendBinding{
		Kind:         kind,
		PageID:       endpoint.PageID,
		Source:       endpoint.SourceFile,
		BlockName:    endpoint.Symbol,
		Method:       endpoint.Method,
		Route:        endpoint.Path,
		ImportPath:   endpoint.Binding.ImportPath,
		PackageName:  endpoint.Binding.PackageName,
		FunctionName: endpoint.Binding.FunctionName,
		Signature:    endpoint.Binding.Signature,
		InputType:    endpoint.Binding.InputType,
		InputPointer: endpoint.Binding.InputPointer,
		InputFields:  append([]source.BackendInputField(nil), endpoint.Binding.InputFields...),
		Status:       endpoint.Binding.Status,
		Message:      endpoint.Binding.Message,
	}
}

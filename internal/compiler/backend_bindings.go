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
			inlinePkg := defaultInlinePkg()
			bindings = append(bindings, resolveBackendBinding(
				bindAction(page, action, pkg),
				bindAction(page, action, inlinePkg),
				pkg, inlinePkg, action.Name,
			))
		}
		for _, api := range page.Blocks.APIs {
			inlinePkg := defaultInlinePkg()
			bindings = append(bindings, resolveBackendBinding(
				bindAPI(page, api, pkg),
				bindAPI(page, api, inlinePkg),
				pkg, inlinePkg, api.Name,
			))
		}
		for _, fragment := range page.Blocks.Fragments {
			if binding, ok := resolveFragmentBinding(page, fragment, pkg, defaultInlinePkg()); ok {
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
	// A broken same-package Go package cannot be inspected: surface that instead
	// of falling back to an inline go ssr {} block and reporting a misleading
	// bound status. The broken package itself is reported by go_package_error.
	if pkg.LoadError != "" {
		binding := baseBackendBinding(page, loadHandlerKind, functionName, "GET", page.Route, pkg)
		binding.Status = source.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK SSR load handler %s.%s could not be inspected: %s", bindingPackageLabel(binding, pkg), functionName, pkg.LoadError)
		return binding
	}
	inlinePkg := inspectInlineScriptFeaturePackage(page, "ssr")
	_, inSame := pkg.Functions[functionName]
	_, inInline := inlinePkg.Functions[functionName]
	switch {
	case inSame && inInline:
		binding := bindLoadFromPackage(page, functionName, pkg)
		binding.Ambiguous = true
		binding.Message = ambiguousHandlerMessage(loadHandlerKind, functionName)
		return binding
	case inSame:
		return bindLoadFromPackage(page, functionName, pkg)
	case inInline:
		return bindLoadFromPackage(page, functionName, inlinePkg)
	default:
		binding := baseBackendBinding(page, loadHandlerKind, functionName, "GET", page.Route, pkg)
		binding.Status = source.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK SSR load handler %s.%s is not implemented", bindingPackageLabel(binding, pkg), functionName)
		binding = markUnexportedCandidate(binding, pkg)
		if !binding.UnexportedCandidate {
			binding = markUnexportedCandidate(binding, inlinePkg)
		}
		return binding
	}
}

func bindLoadFromPackage(page gwdkir.Page, functionName string, pkg featurePackage) source.BackendBinding {
	binding := baseBackendBinding(page, loadHandlerKind, functionName, "GET", page.Route, pkg)
	function := pkg.Functions[functionName]
	if !function.Load() {
		binding.Status = source.BackendBindingUnsupportedSignature
		binding.Message = fmt.Sprintf("GOWDK SSR load handler %s.%s must have signature func(ssr.LoadContext) map[string]any or func(ssr.LoadContext) (map[string]any, error)", bindingPackageLabel(binding, pkg), functionName)
		return binding
	}
	binding.Signature = function.Signature
	binding.Status = source.BackendBindingBound
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
		return markUnexportedCandidate(binding, pkg)
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
		return markUnexportedCandidate(binding, pkg)
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

// bindMissingFragmentCandidate emits a missing fragment binding only when a
// same-named unexported Go function exists in one of the inspected packages
// (same-package or inline go block), so a casing mistake surfaces the
// unexported_backend_handler warning instead of silently falling back to static
// fragment output. A fragment with no candidate at all stays unbound, because
// fragments do not require a Go handler.
func bindMissingFragmentCandidate(page gwdkir.Page, fragment gwdkir.FragmentEndpoint, pkgs ...featurePackage) (source.BackendBinding, bool) {
	method := strings.TrimSpace(fragment.Method)
	if method == "" {
		method = "GET"
	}
	for _, pkg := range pkgs {
		binding := baseBackendBinding(page, fragmentHandlerKind, fragment.Name, method, strings.TrimSpace(fragment.Route), pkg)
		binding.Status = source.BackendBindingMissing
		binding.Message = fmt.Sprintf("GOWDK fragment handler %s.%s is not implemented", bindingPackageLabel(binding, pkg), binding.FunctionName)
		if binding = markUnexportedCandidate(binding, pkg); binding.UnexportedCandidate {
			return binding, true
		}
	}
	return source.BackendBinding{}, false
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

// resolveBackendBinding chooses the binding for an action/api block across its
// two possible Go sources — sibling same-package code and the inline default
// go {} block — without masking failures:
//
//   - When the same-package package failed to compile (LoadError), the
//     could-not-inspect binding is kept instead of falling back to the inline
//     result, so a broken package never looks bound. The compile error itself is
//     reported by go_package_error.
//   - When the handler is declared in BOTH sources it is ambiguous: the binding
//     is flagged so tooling reports the conflict instead of silently preferring
//     one source.
//   - Otherwise the source that actually declares the handler wins, and when
//     neither does, an unexported near-miss in either source is surfaced.
func resolveBackendBinding(samePackage, inline source.BackendBinding, samePkg, inlinePkg featurePackage, name string) source.BackendBinding {
	if samePkg.LoadError != "" {
		return samePackage
	}
	inSame := packageHasFunction(samePkg, name)
	inInline := packageHasFunction(inlinePkg, name)
	switch {
	case inSame && inInline:
		out := samePackage
		out.Ambiguous = true
		out.Message = ambiguousHandlerMessage(out.Kind, name)
		return out
	case inSame:
		return samePackage
	case inInline:
		return inline
	default:
		return preferInlineBinding(samePackage, inline)
	}
}

// resolveFragmentBinding mirrors resolveBackendBinding for fragments, which
// never require a Go handler: a fragment with no bound handler renders static
// .gwdk output. It returns ok=false (no binding, static) unless a handler is
// declared, ambiguous, or only present as an unexported near-miss.
func resolveFragmentBinding(page gwdkir.Page, fragment gwdkir.FragmentEndpoint, samePkg, inlinePkg featurePackage) (source.BackendBinding, bool) {
	if samePkg.LoadError != "" {
		return source.BackendBinding{}, false
	}
	inSame := packageHasFunction(samePkg, fragment.Name)
	inInline := packageHasFunction(inlinePkg, fragment.Name)
	switch {
	case inSame && inInline:
		binding, _ := bindFragment(page, fragment, samePkg)
		binding.Ambiguous = true
		binding.Message = ambiguousHandlerMessage(fragmentHandlerKind, fragment.Name)
		return binding, true
	case inSame:
		return bindFragment(page, fragment, samePkg)
	case inInline:
		return bindFragment(page, fragment, inlinePkg)
	default:
		return bindMissingFragmentCandidate(page, fragment, samePkg, inlinePkg)
	}
}

func packageHasFunction(pkg featurePackage, name string) bool {
	_, ok := pkg.Functions[name]
	return ok
}

func ambiguousHandlerMessage(kind, name string) string {
	return fmt.Sprintf("GOWDK %s handler %s is declared in both same-package Go and an inline go {} block; declare it in exactly one source", kind, name)
}

// preferInlineBinding resolves an action/api block that did not bind in its
// same-package Go code by consulting the inline default go {} block. It prefers
// the inline result when the inline block actually binds (bound or unsupported
// signature) and, failing that, when only the inline block surfaces an
// unexported same-named near-miss the user almost certainly meant to bind.
func preferInlineBinding(samePackage, inline source.BackendBinding) source.BackendBinding {
	if inline.Status != source.BackendBindingMissing {
		return inline
	}
	if !samePackage.UnexportedCandidate && inline.UnexportedCandidate {
		return inline
	}
	return samePackage
}

// markUnexportedCandidate flags a missing binding when a same-named unexported
// Go function exists in the inspected package, and appends the near-miss to the
// binding message so tooling can explain that the function is present but not
// exported. It is a no-op when no such candidate exists.
func markUnexportedCandidate(binding source.BackendBinding, pkg featurePackage) source.BackendBinding {
	if !pkg.hasUnexported(binding.FunctionName) {
		return binding
	}
	binding.UnexportedCandidate = true
	binding.Message += fmt.Sprintf(
		"; an unexported function %s exists in the same package — export it as %s",
		firstRuneLower(binding.FunctionName),
		binding.FunctionName,
	)
	return binding
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

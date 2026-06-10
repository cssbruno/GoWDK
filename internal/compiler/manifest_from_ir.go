package compiler

import (
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/source"
)

// ManifestFromIR reconstructs a manifest.Manifest from compiler IR so the
// existing manifest-typed validators can run against an IR-first build path.
//
// This is the single IR->manifest conversion seam in the codebase. It lives in
// compiler (the package that owns validation) rather than being duplicated in
// each generated-output package. As individual validators move to read IR
// directly, this converter shrinks and is eventually removed; until then it
// keeps the build path validating identical data whether it starts from a
// parsed manifest or from IR.
func ManifestFromIR(ir gwdkir.Program) manifest.Manifest {
	app := manifest.Manifest{
		Pages:           make([]manifest.Page, 0, len(ir.Pages)),
		Components:      make([]manifest.Component, 0, len(ir.Components)),
		Layouts:         make([]manifest.Layout, 0, len(ir.Layouts)),
		Endpoints:       goEndpointsFromIR(ir.GoEndpoints),
		BackendBindings: BackendBindingsFromIR(ir),
	}
	for _, page := range ir.Pages {
		app.Pages = append(app.Pages, pageFromIR(page))
	}
	for _, component := range ir.Components {
		app.Components = append(app.Components, componentFromIR(component))
	}
	for _, layout := range ir.Layouts {
		app.Layouts = append(app.Layouts, layoutFromIR(layout))
	}
	return app
}

// BackendBindingsFromIR derives just the backend binding records from IR
// endpoints and page load bindings, without reconstructing the full
// page/component/layout manifest. Callers that only need bindings (e.g. build
// reporting) should use this instead of ManifestFromIR(ir).BackendBindings,
// which would allocate the whole model.
func BackendBindingsFromIR(ir gwdkir.Program) []manifest.BackendBinding {
	out := make([]manifest.BackendBinding, 0, len(ir.Endpoints)+len(ir.Pages))
	for _, endpoint := range ir.Endpoints {
		binding := backendBindingFromIR(endpoint)
		if binding.Status != "" || binding.ImportPath != "" || binding.FunctionName != "" {
			out = append(out, binding)
		}
	}
	for _, page := range ir.Pages {
		binding := loadBindingFromIR(page)
		if binding.Status != "" || binding.ImportPath != "" || binding.FunctionName != "" {
			out = append(out, binding)
		}
	}
	return out
}

// goEndpointsFromIR reconstructs the standalone Go endpoint declarations from
// their lossless IR mirror. This is a one-to-one field copy, so validation that
// reads manifest.Endpoints (route conflicts, standalone-endpoint shape) runs
// identically whether it starts from a parsed manifest or from IR.
func goEndpointsFromIR(endpoints []gwdkir.GoEndpoint) []manifest.EndpointDeclaration {
	if len(endpoints) == 0 {
		return nil
	}
	out := make([]manifest.EndpointDeclaration, 0, len(endpoints))
	for _, endpoint := range endpoints {
		out = append(out, manifest.EndpointDeclaration{
			Kind:          endpoint.Kind,
			SourceKind:    manifest.EndpointSource(endpoint.SourceKind),
			Package:       endpoint.Package,
			Source:        endpoint.Source,
			Name:          endpoint.Name,
			Method:        endpoint.Method,
			Route:         endpoint.Route,
			ErrorPage:     endpoint.ErrorPage,
			Span:          endpoint.Span,
			RouteSpan:     endpoint.RouteSpan,
			RouteParams:   append([]source.NamedSpan(nil), endpoint.RouteParams...),
			ErrorPageSpan: endpoint.ErrorPageSpan,
		})
	}
	return out
}

func backendBindingFromIR(endpoint gwdkir.Endpoint) manifest.BackendBinding {
	kind := actionHandlerKind
	switch endpoint.Kind {
	case gwdkir.EndpointAPI:
		kind = apiHandlerKind
	case gwdkir.EndpointFragment:
		kind = fragmentHandlerKind
	}
	return manifest.BackendBinding{
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

func pageFromIR(page gwdkir.Page) manifest.Page {
	return manifest.Page{
		Source:      page.Source,
		Package:     page.Package,
		ID:          page.ID,
		Route:       page.Route,
		RouteParams: append([]source.RouteParam(nil), page.RouteParams...),
		Render:      page.Render,
		Cache:       page.Cache,
		Revalidate:  page.Revalidate,
		ErrorPage:   page.ErrorPage,
		Metadata:    manifest.PageMetadata(page.Metadata),
		Layouts:     append([]string(nil), page.Layouts...),
		Guard:       append([]string(nil), page.Guards...),
		CSS:         append([]string(nil), page.CSS...),
		JS:          append([]string(nil), page.JS...),
		InlineJS:    copyInlineScriptsFromIR(page.InlineJS),
		Imports:     importsFromIR(page.Imports),
		Uses:        usesFromIR(page.Uses),
		Stores:      storesFromIR(page.Stores),
		Paths:       page.Blocks.Paths,
		Blocks:      blocksFromIR(page.Blocks),
		LoadBinding: loadBindingFromIR(page),
		Spans: manifest.PageSpans{
			Package:     page.Spans.Package,
			Page:        page.Spans.Page,
			Route:       page.Spans.Route,
			Render:      page.Spans.Render,
			Cache:       page.Spans.Cache,
			Revalidate:  page.Spans.Revalidate,
			ErrorPage:   page.Spans.ErrorPage,
			Title:       page.Spans.Title,
			Description: page.Spans.Description,
			Canonical:   page.Spans.Canonical,
			Image:       page.Spans.Image,
			Layouts:     append([]source.NamedSpan(nil), page.Spans.Layouts...),
			Guard:       append([]source.NamedSpan(nil), page.Spans.Guard...),
			CSS:         append([]source.NamedSpan(nil), page.Spans.CSS...),
			JS:          append([]source.NamedSpan(nil), page.Spans.JS...),
			InlineJS:    append([]source.NamedSpan(nil), page.Spans.InlineJS...),
			RouteParams: append([]source.NamedSpan(nil), page.Spans.RouteParams...),
		},
	}
}

func loadBindingFromIR(page gwdkir.Page) manifest.BackendBinding {
	binding := page.LoadBinding
	if binding.Status == "" && binding.ImportPath == "" && binding.FunctionName == "" {
		return manifest.BackendBinding{}
	}
	return manifest.BackendBinding{
		Kind:         loadHandlerKind,
		PageID:       page.ID,
		Source:       page.Source,
		BlockName:    binding.FunctionName,
		Method:       "GET",
		Route:        page.Route,
		ImportPath:   binding.ImportPath,
		PackageName:  binding.PackageName,
		FunctionName: binding.FunctionName,
		Signature:    binding.Signature,
		Status:       binding.Status,
		Message:      binding.Message,
	}
}

func componentFromIR(component gwdkir.Component) manifest.Component {
	return manifest.Component{
		Source:      component.Source,
		Package:     component.Package,
		Name:        component.Name,
		Imports:     importsFromIR(component.Imports),
		Uses:        usesFromIR(component.Uses),
		CSS:         append([]string(nil), component.CSS...),
		JS:          append([]string(nil), component.JS...),
		InlineJS:    copyInlineScriptsFromIR(component.InlineJS),
		Assets:      append([]string(nil), component.Assets...),
		Props:       propsFromIR(component.Props),
		PropsType:   goTypeRefFromIR(component.PropsType),
		State:       stateContractFromIR(component.State),
		WASM:        manifest.WASMContract(component.WASM),
		Exports:     exportsFromIR(component.Exports),
		Emits:       emitsFromIR(component.Emits),
		Blocks:      blocksFromIR(component.Blocks),
		Span:        component.Span,
		PackageSpan: component.PackageSpan,
		Spans: manifest.ComponentSpans{
			CSS:      append([]source.NamedSpan(nil), component.Spans.CSS...),
			JS:       append([]source.NamedSpan(nil), component.Spans.JS...),
			InlineJS: append([]source.NamedSpan(nil), component.Spans.InlineJS...),
			Assets:   append([]source.NamedSpan(nil), component.Spans.Assets...),
		},
	}
}

func copyInlineScriptsFromIR(scripts []source.InlineScript) []source.InlineScript {
	if len(scripts) == 0 {
		return nil
	}
	out := make([]source.InlineScript, len(scripts))
	copy(out, scripts)
	return out
}

func layoutFromIR(layout gwdkir.Layout) manifest.Layout {
	return manifest.Layout{
		Source:      layout.Source,
		Package:     layout.Package,
		ID:          layout.ID,
		Uses:        usesFromIR(layout.Uses),
		Blocks:      blocksFromIR(layout.Blocks),
		Span:        layout.Span,
		PackageSpan: layout.PackageSpan,
	}
}

func blocksFromIR(blocks gwdkir.Blocks) manifest.Blocks {
	return manifest.Blocks{
		PathsBody:  blocks.PathsBody,
		Build:      blocks.Build,
		BuildBody:  blocks.BuildBody,
		Load:       blocks.Load,
		LoadBody:   blocks.LoadBody,
		Client:     blocks.Client,
		ClientBody: blocks.ClientBody,
		GoBlocks:   scriptsFromIR(blocks.GoBlocks),
		View:       blocks.View,
		ViewBody:   blocks.ViewBody,
		Style:      blocks.Style,
		StyleBody:  blocks.StyleBody,
		Actions:    actionsFromIR(blocks.Actions),
		APIs:       apisFromIR(blocks.APIs),
		Fragments:  fragmentEndpointsFromIR(blocks.Fragments),
		Spans: manifest.BlockSpans{
			Paths:         blocks.Spans.Paths,
			Build:         blocks.Spans.Build,
			Load:          blocks.Spans.Load,
			Client:        blocks.Spans.Client,
			GoBlocks:      append([]source.NamedSpan(nil), blocks.Spans.GoBlocks...),
			View:          blocks.Spans.View,
			ViewBodyStart: blocks.Spans.ViewBodyStart,
			Actions:       append([]source.NamedSpan(nil), blocks.Spans.Actions...),
			APIs:          append([]source.NamedSpan(nil), blocks.Spans.APIs...),
			Fragments:     append([]source.NamedSpan(nil), blocks.Spans.Fragments...),
			Exports:       blocks.Spans.Exports,
			Emits:         blocks.Spans.Emits,
		},
	}
}

func scriptsFromIR(scripts []gwdkir.GoBlock) []manifest.GoBlock {
	out := make([]manifest.GoBlock, 0, len(scripts))
	for _, script := range scripts {
		out = append(out, manifest.GoBlock{
			Target: script.Target,
			Body:   script.Body,
			Span:   script.Span,
		})
	}
	return out
}

func actionsFromIR(actions []gwdkir.Action) []manifest.Action {
	out := make([]manifest.Action, 0, len(actions))
	for _, action := range actions {
		out = append(out, manifest.Action{
			Name:           action.Name,
			Method:         action.Method,
			Route:          action.Route,
			Body:           action.Body,
			InputName:      action.InputName,
			InputType:      action.InputType,
			ValidatesInput: action.ValidatesInput,
			Redirect:       action.Redirect,
			Fragments:      fragmentsFromIR(action.Fragments),
			ErrorPage:      action.ErrorPage,
			Span:           action.Span,
			RouteSpan:      action.RouteSpan,
			RouteParams:    append([]source.NamedSpan(nil), action.RouteParams...),
			InputSpan:      action.InputSpan,
			ValidationSpan: action.ValidationSpan,
			RedirectSpan:   action.RedirectSpan,
			ErrorPageSpan:  action.ErrorPageSpan,
		})
	}
	return out
}

func apisFromIR(apis []gwdkir.API) []manifest.API {
	out := make([]manifest.API, 0, len(apis))
	for _, api := range apis {
		out = append(out, manifest.API{
			Name:          api.Name,
			Method:        api.Method,
			Route:         api.Route,
			ErrorPage:     api.ErrorPage,
			Span:          api.Span,
			RouteSpan:     api.RouteSpan,
			RouteParams:   append([]source.NamedSpan(nil), api.RouteParams...),
			ErrorPageSpan: api.ErrorPageSpan,
		})
	}
	return out
}

func fragmentsFromIR(fragments []gwdkir.Fragment) []manifest.Fragment {
	out := make([]manifest.Fragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, manifest.Fragment{Target: fragment.Target, Body: fragment.Body, Span: fragment.Span})
	}
	return out
}

func fragmentEndpointsFromIR(fragments []gwdkir.FragmentEndpoint) []manifest.FragmentEndpoint {
	out := make([]manifest.FragmentEndpoint, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, manifest.FragmentEndpoint{
			Name:        fragment.Name,
			Method:      fragment.Method,
			Route:       fragment.Route,
			Target:      fragment.Target,
			Body:        fragment.Body,
			Span:        fragment.Span,
			RouteSpan:   fragment.RouteSpan,
			TargetSpan:  fragment.TargetSpan,
			RouteParams: append([]source.NamedSpan(nil), fragment.RouteParams...),
		})
	}
	return out
}

func importsFromIR(imports []gwdkir.Import) []manifest.Import {
	out := make([]manifest.Import, 0, len(imports))
	for _, item := range imports {
		out = append(out, manifest.Import{Alias: item.Alias, Path: item.Path, Span: item.Span})
	}
	return out
}

func usesFromIR(uses []gwdkir.Use) []manifest.Use {
	out := make([]manifest.Use, 0, len(uses))
	for _, item := range uses {
		out = append(out, manifest.Use{Alias: item.Alias, Package: item.Package, Span: item.Span})
	}
	return out
}

func storesFromIR(stores []gwdkir.Store) []manifest.Store {
	out := make([]manifest.Store, 0, len(stores))
	for _, store := range stores {
		out = append(out, manifest.Store{
			Name: store.Name,
			Type: goTypeRefFromIR(store.Type),
			Init: goFuncRefFromIR(store.Init),
			Span: store.Span,
		})
	}
	return out
}

func propsFromIR(props []gwdkir.Prop) []manifest.Prop {
	out := make([]manifest.Prop, 0, len(props))
	for _, prop := range props {
		out = append(out, manifest.Prop{Name: prop.Name, Type: prop.Type, Span: prop.Span})
	}
	return out
}

func exportsFromIR(exports []gwdkir.Export) []manifest.Export {
	out := make([]manifest.Export, 0, len(exports))
	for _, export := range exports {
		out = append(out, manifest.Export{Name: export.Name, Type: export.Type, Span: export.Span})
	}
	return out
}

func emitsFromIR(emits []gwdkir.Emit) []manifest.Emit {
	out := make([]manifest.Emit, 0, len(emits))
	for _, emit := range emits {
		params := make([]manifest.EmitParam, 0, len(emit.Params))
		for _, param := range emit.Params {
			params = append(params, manifest.EmitParam{Name: param.Name, Type: param.Type, Span: param.Span})
		}
		out = append(out, manifest.Emit{Name: emit.Name, Params: params, Span: emit.Span})
	}
	return out
}

func stateContractFromIR(state gwdkir.StateContract) manifest.StateContract {
	return manifest.StateContract{
		Type: goTypeRefFromIR(state.Type),
		Init: goFuncRefFromIR(state.Init),
		Span: state.Span,
	}
}

func goTypeRefFromIR(ref gwdkir.GoRef) manifest.GoTypeRef {
	return manifest.GoTypeRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

func goFuncRefFromIR(ref gwdkir.GoRef) manifest.GoFuncRef {
	return manifest.GoFuncRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

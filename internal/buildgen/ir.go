package buildgen

import (
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func buildModelFromIR(ir gwdkir.Program) manifest.Manifest {
	app := manifest.Manifest{
		Pages:           make([]manifest.Page, 0, len(ir.Pages)),
		Components:      make([]manifest.Component, 0, len(ir.Components)),
		Layouts:         make([]manifest.Layout, 0, len(ir.Layouts)),
		BackendBindings: make([]manifest.BackendBinding, 0, len(ir.Endpoints)),
	}
	for _, page := range ir.Pages {
		app.Pages = append(app.Pages, buildPageFromIR(page))
	}
	for _, component := range ir.Components {
		app.Components = append(app.Components, buildComponentFromIR(component))
	}
	for _, layout := range ir.Layouts {
		app.Layouts = append(app.Layouts, buildLayoutFromIR(layout))
	}
	for _, endpoint := range ir.Endpoints {
		binding := buildBackendBindingFromIR(endpoint)
		if binding.Status != "" || binding.ImportPath != "" || binding.FunctionName != "" {
			app.BackendBindings = append(app.BackendBindings, binding)
		}
	}
	return app
}

func buildBackendBindingFromIR(endpoint gwdkir.Endpoint) manifest.BackendBinding {
	kind := "action"
	if endpoint.Kind == gwdkir.EndpointAPI {
		kind = "api"
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
		InputFields:  append([]manifest.BackendInputField(nil), endpoint.Binding.InputFields...),
		Status:       endpoint.Binding.Status,
		Message:      endpoint.Binding.Message,
	}
}

func buildPageFromIR(page gwdkir.Page) manifest.Page {
	return manifest.Page{
		Source:      page.Source,
		Package:     page.Package,
		ID:          page.ID,
		Route:       page.Route,
		RouteParams: append([]manifest.RouteParam(nil), page.RouteParams...),
		Render:      page.Render,
		Cache:       page.Cache,
		Revalidate:  page.Revalidate,
		Metadata:    manifest.PageMetadata(page.Metadata),
		Layouts:     append([]string(nil), page.Layouts...),
		Guard:       append([]string(nil), page.Guards...),
		CSS:         append([]string(nil), page.CSS...),
		Imports:     buildImportsFromIR(page.Imports),
		Uses:        buildUsesFromIR(page.Uses),
		Stores:      buildStoresFromIR(page.Stores),
		Paths:       page.Blocks.PathsBody != "",
		Blocks:      buildBlocksFromIR(page.Blocks),
		LoadBinding: buildLoadBindingFromIR(page),
		Spans: manifest.PageSpans{
			Package:     page.Spans.Package,
			Page:        page.Spans.Page,
			Route:       page.Spans.Route,
			Render:      page.Spans.Render,
			Cache:       page.Spans.Cache,
			Revalidate:  page.Spans.Revalidate,
			Title:       page.Spans.Title,
			Description: page.Spans.Description,
			Canonical:   page.Spans.Canonical,
			Image:       page.Spans.Image,
			Layouts:     append([]manifest.NamedSpan(nil), page.Spans.Layouts...),
			Guard:       append([]manifest.NamedSpan(nil), page.Spans.Guard...),
			CSS:         append([]manifest.NamedSpan(nil), page.Spans.CSS...),
			RouteParams: append([]manifest.NamedSpan(nil), page.Spans.RouteParams...),
		},
	}
}

func buildLoadBindingFromIR(page gwdkir.Page) manifest.BackendBinding {
	binding := page.LoadBinding
	if binding.Status == "" && binding.ImportPath == "" && binding.FunctionName == "" {
		return manifest.BackendBinding{}
	}
	return manifest.BackendBinding{
		Kind:         "load",
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

func buildComponentFromIR(component gwdkir.Component) manifest.Component {
	return manifest.Component{
		Source:      component.Source,
		Package:     component.Package,
		Name:        component.Name,
		Imports:     buildImportsFromIR(component.Imports),
		Uses:        buildUsesFromIR(component.Uses),
		CSS:         append([]string(nil), component.CSS...),
		Assets:      append([]string(nil), component.Assets...),
		Props:       buildPropsFromIR(component.Props),
		PropsType:   buildGoTypeRefFromIR(component.PropsType),
		State:       buildStateContractFromIR(component.State),
		WASM:        manifest.WASMContract(component.WASM),
		Exports:     buildExportsFromIR(component.Exports),
		Emits:       buildEmitsFromIR(component.Emits),
		Blocks:      buildBlocksFromIR(component.Blocks),
		Span:        component.Span,
		PackageSpan: component.PackageSpan,
		Spans: manifest.ComponentSpans{
			CSS:    append([]manifest.NamedSpan(nil), component.Spans.CSS...),
			Assets: append([]manifest.NamedSpan(nil), component.Spans.Assets...),
		},
	}
}

func buildLayoutFromIR(layout gwdkir.Layout) manifest.Layout {
	return manifest.Layout{
		Source:      layout.Source,
		Package:     layout.Package,
		ID:          layout.ID,
		Uses:        buildUsesFromIR(layout.Uses),
		Blocks:      buildBlocksFromIR(layout.Blocks),
		Span:        layout.Span,
		PackageSpan: layout.PackageSpan,
	}
}

func buildBlocksFromIR(blocks gwdkir.Blocks) manifest.Blocks {
	return manifest.Blocks{
		PathsBody:  blocks.PathsBody,
		Build:      blocks.Build,
		BuildBody:  blocks.BuildBody,
		Load:       blocks.Load,
		LoadBody:   blocks.LoadBody,
		Client:     blocks.Client,
		ClientBody: blocks.ClientBody,
		View:       blocks.View,
		ViewBody:   blocks.ViewBody,
		Actions:    buildActionsFromIR(blocks.Actions),
		APIs:       buildAPIsFromIR(blocks.APIs),
		Fragments:  buildFragmentEndpointsFromIR(blocks.Fragments),
		Spans: manifest.BlockSpans{
			Paths:     blocks.Spans.Paths,
			Build:     blocks.Spans.Build,
			Load:      blocks.Spans.Load,
			Client:    blocks.Spans.Client,
			View:      blocks.Spans.View,
			Actions:   append([]manifest.NamedSpan(nil), blocks.Spans.Actions...),
			APIs:      append([]manifest.NamedSpan(nil), blocks.Spans.APIs...),
			Fragments: append([]manifest.NamedSpan(nil), blocks.Spans.Fragments...),
			Exports:   blocks.Spans.Exports,
			Emits:     blocks.Spans.Emits,
		},
	}
}

func buildActionsFromIR(actions []gwdkir.Action) []manifest.Action {
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
			Fragments:      buildFragmentsFromIR(action.Fragments),
			Span:           action.Span,
			RouteSpan:      action.RouteSpan,
			RouteParams:    append([]manifest.NamedSpan(nil), action.RouteParams...),
			InputSpan:      action.InputSpan,
			ValidationSpan: action.ValidationSpan,
			RedirectSpan:   action.RedirectSpan,
		})
	}
	return out
}

func buildAPIsFromIR(apis []gwdkir.API) []manifest.API {
	out := make([]manifest.API, 0, len(apis))
	for _, api := range apis {
		out = append(out, manifest.API{
			Name:        api.Name,
			Method:      api.Method,
			Route:       api.Route,
			Span:        api.Span,
			RouteSpan:   api.RouteSpan,
			RouteParams: append([]manifest.NamedSpan(nil), api.RouteParams...),
		})
	}
	return out
}

func buildFragmentsFromIR(fragments []gwdkir.Fragment) []manifest.Fragment {
	out := make([]manifest.Fragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, manifest.Fragment{Target: fragment.Target, Body: fragment.Body, Span: fragment.Span})
	}
	return out
}

func buildFragmentEndpointsFromIR(fragments []gwdkir.FragmentEndpoint) []manifest.FragmentEndpoint {
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
			RouteParams: append([]manifest.NamedSpan(nil), fragment.RouteParams...),
		})
	}
	return out
}

func buildImportsFromIR(imports []gwdkir.Import) []manifest.Import {
	out := make([]manifest.Import, 0, len(imports))
	for _, item := range imports {
		out = append(out, manifest.Import{Alias: item.Alias, Path: item.Path, Span: item.Span})
	}
	return out
}

func buildUsesFromIR(uses []gwdkir.Use) []manifest.Use {
	out := make([]manifest.Use, 0, len(uses))
	for _, item := range uses {
		out = append(out, manifest.Use{Alias: item.Alias, Package: item.Package, Span: item.Span})
	}
	return out
}

func buildStoresFromIR(stores []gwdkir.Store) []manifest.Store {
	out := make([]manifest.Store, 0, len(stores))
	for _, store := range stores {
		out = append(out, manifest.Store{
			Name: store.Name,
			Type: buildGoTypeRefFromIR(store.Type),
			Init: buildGoFuncRefFromIR(store.Init),
			Span: store.Span,
		})
	}
	return out
}

func buildPropsFromIR(props []gwdkir.Prop) []manifest.Prop {
	out := make([]manifest.Prop, 0, len(props))
	for _, prop := range props {
		out = append(out, manifest.Prop{Name: prop.Name, Type: prop.Type, Span: prop.Span})
	}
	return out
}

func buildExportsFromIR(exports []gwdkir.Export) []manifest.Export {
	out := make([]manifest.Export, 0, len(exports))
	for _, export := range exports {
		out = append(out, manifest.Export{Name: export.Name, Type: export.Type, Span: export.Span})
	}
	return out
}

func buildEmitsFromIR(emits []gwdkir.Emit) []manifest.Emit {
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

func buildStateContractFromIR(state gwdkir.StateContract) manifest.StateContract {
	return manifest.StateContract{
		Type: buildGoTypeRefFromIR(state.Type),
		Init: buildGoFuncRefFromIR(state.Init),
		Span: state.Span,
	}
}

func buildGoTypeRefFromIR(ref gwdkir.GoRef) manifest.GoTypeRef {
	return manifest.GoTypeRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

func buildGoFuncRefFromIR(ref gwdkir.GoRef) manifest.GoFuncRef {
	return manifest.GoFuncRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

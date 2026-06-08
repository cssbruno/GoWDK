package gwdkanalysis

import (
	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func routeKind(mode gowdk.RenderMode, page manifest.Page) gwdkir.RouteKind {
	switch mode {
	case gowdk.SSR:
		return gwdkir.RouteSSR
	case gowdk.Hybrid:
		return gwdkir.RouteHybrid
	default:
		return gwdkir.RouteSPA
	}
}

func lowerIRPage(page manifest.Page) gwdkir.Page {
	return gwdkir.Page{
		Source:      page.Source,
		Package:     page.Package,
		ID:          page.ID,
		Route:       page.Route,
		RouteParams: copyRouteParams(page.TypedRouteParams()),
		Render:      page.Render,
		Cache:       page.Cache,
		Revalidate:  page.Revalidate,
		ErrorPage:   page.ErrorPage,
		Metadata:    gwdkir.PageMetadata(page.Metadata),
		Layouts:     append([]string(nil), page.Layouts...),
		Guards:      append([]string(nil), page.Guard...),
		CSS:         append([]string(nil), page.CSS...),
		Imports:     lowerIRImports(page.Imports),
		Uses:        lowerIRUses(page.Uses),
		Stores:      lowerIRStores(page.Stores),
		Blocks:      lowerIRBlocks(page.Blocks),
		Spans: gwdkir.PageSpans{
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
			Layouts:     append([]manifest.NamedSpan(nil), page.Spans.Layouts...),
			Guard:       append([]manifest.NamedSpan(nil), page.Spans.Guard...),
			CSS:         append([]manifest.NamedSpan(nil), page.Spans.CSS...),
			RouteParams: append([]manifest.NamedSpan(nil), page.Spans.RouteParams...),
		},
	}
}

func copyRouteParams(params []manifest.RouteParam) []manifest.RouteParam {
	if len(params) == 0 {
		return nil
	}
	out := make([]manifest.RouteParam, len(params))
	copy(out, params)
	return out
}

func lowerIRComponent(component manifest.Component) gwdkir.Component {
	return gwdkir.Component{
		Source:      component.Source,
		Package:     component.Package,
		Name:        component.Name,
		Imports:     lowerIRImports(component.Imports),
		Uses:        lowerIRUses(component.Uses),
		CSS:         append([]string(nil), component.CSS...),
		Assets:      append([]string(nil), component.Assets...),
		Props:       lowerIRProps(component.Props),
		PropsType:   lowerIRGoTypeRef(component.PropsType),
		State:       lowerIRStateContract(component.State),
		WASM:        gwdkir.WASMContract(component.WASM),
		Exports:     lowerIRExports(component.Exports),
		Emits:       lowerIREmits(component.Emits),
		Blocks:      lowerIRBlocks(component.Blocks),
		Span:        component.Span,
		PackageSpan: component.PackageSpan,
		Spans: gwdkir.ComponentSpans{
			CSS:    append([]manifest.NamedSpan(nil), component.Spans.CSS...),
			Assets: append([]manifest.NamedSpan(nil), component.Spans.Assets...),
		},
	}
}

func lowerIRLayout(layout manifest.Layout) gwdkir.Layout {
	return gwdkir.Layout{
		Source:      layout.Source,
		Package:     layout.Package,
		ID:          layout.ID,
		Uses:        lowerIRUses(layout.Uses),
		Blocks:      lowerIRBlocks(layout.Blocks),
		Span:        layout.Span,
		PackageSpan: layout.PackageSpan,
	}
}

func lowerIRBlocks(blocks manifest.Blocks) gwdkir.Blocks {
	return gwdkir.Blocks{
		PathsBody:  blocks.PathsBody,
		Build:      blocks.Build,
		BuildBody:  blocks.BuildBody,
		Load:       blocks.Load,
		LoadBody:   blocks.LoadBody,
		Client:     blocks.Client,
		ClientBody: blocks.ClientBody,
		GoBlocks:   lowerIRGoBlocks(blocks.GoBlocks),
		View:       blocks.View,
		ViewBody:   blocks.ViewBody,
		Style:      blocks.Style,
		StyleBody:  blocks.StyleBody,
		Actions:    lowerIRActions(blocks.Actions),
		APIs:       lowerIRAPIs(blocks.APIs),
		Fragments:  lowerIRFragmentEndpoints(blocks.Fragments),
		Spans: gwdkir.BlockSpans{
			Paths:         blocks.Spans.Paths,
			Build:         blocks.Spans.Build,
			Load:          blocks.Spans.Load,
			Client:        blocks.Spans.Client,
			GoBlocks:      append([]manifest.NamedSpan(nil), blocks.Spans.GoBlocks...),
			View:          blocks.Spans.View,
			ViewBodyStart: blocks.Spans.ViewBodyStart,
			Actions:       append([]manifest.NamedSpan(nil), blocks.Spans.Actions...),
			APIs:          append([]manifest.NamedSpan(nil), blocks.Spans.APIs...),
			Fragments:     append([]manifest.NamedSpan(nil), blocks.Spans.Fragments...),
			Exports:       blocks.Spans.Exports,
			Emits:         blocks.Spans.Emits,
		},
	}
}

func lowerIRActions(actions []manifest.Action) []gwdkir.Action {
	out := make([]gwdkir.Action, 0, len(actions))
	for _, action := range actions {
		out = append(out, gwdkir.Action{
			Name:           action.Name,
			Method:         action.Method,
			Route:          action.Route,
			Body:           action.Body,
			InputName:      action.InputName,
			InputType:      action.InputType,
			ValidatesInput: action.ValidatesInput,
			Redirect:       action.Redirect,
			Fragments:      lowerIRFragments(action.Fragments),
			ErrorPage:      action.ErrorPage,
			Span:           action.Span,
			RouteSpan:      action.RouteSpan,
			RouteParams:    append([]manifest.NamedSpan(nil), action.RouteParams...),
			InputSpan:      action.InputSpan,
			ValidationSpan: action.ValidationSpan,
			RedirectSpan:   action.RedirectSpan,
			ErrorPageSpan:  action.ErrorPageSpan,
		})
	}
	return out
}

func lowerIRAPIs(apis []manifest.API) []gwdkir.API {
	out := make([]gwdkir.API, 0, len(apis))
	for _, api := range apis {
		out = append(out, gwdkir.API{
			Name:          api.Name,
			Method:        api.Method,
			Route:         api.Route,
			ErrorPage:     api.ErrorPage,
			Span:          api.Span,
			RouteSpan:     api.RouteSpan,
			RouteParams:   append([]manifest.NamedSpan(nil), api.RouteParams...),
			ErrorPageSpan: api.ErrorPageSpan,
		})
	}
	return out
}

func lowerIRGoBlocks(scripts []manifest.GoBlock) []gwdkir.GoBlock {
	out := make([]gwdkir.GoBlock, 0, len(scripts))
	for _, block := range scripts {
		out = append(out, gwdkir.GoBlock{
			Target: block.Target,
			Body:   block.Body,
			Span:   block.Span,
		})
	}
	return out
}

func lowerIRFragments(fragments []manifest.Fragment) []gwdkir.Fragment {
	out := make([]gwdkir.Fragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, gwdkir.Fragment{Target: fragment.Target, Body: fragment.Body, Span: fragment.Span})
	}
	return out
}

func lowerIRFragmentEndpoints(fragments []manifest.FragmentEndpoint) []gwdkir.FragmentEndpoint {
	out := make([]gwdkir.FragmentEndpoint, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, gwdkir.FragmentEndpoint{
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

func lowerIRImports(in []manifest.Import) []gwdkir.Import {
	out := make([]gwdkir.Import, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Import{Alias: item.Alias, Path: item.Path, Span: item.Span})
	}
	return out
}

func lowerIRUses(in []manifest.Use) []gwdkir.Use {
	out := make([]gwdkir.Use, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Use{Alias: item.Alias, Package: item.Package, Span: item.Span})
	}
	return out
}

func lowerIRStores(in []manifest.Store) []gwdkir.Store {
	out := make([]gwdkir.Store, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Store{
			Name: item.Name,
			Type: lowerIRGoTypeRef(item.Type),
			Init: lowerIRGoFuncRef(item.Init),
			Span: item.Span,
		})
	}
	return out
}

func lowerIRProps(in []manifest.Prop) []gwdkir.Prop {
	out := make([]gwdkir.Prop, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Prop{Name: item.Name, Type: item.Type, Span: item.Span})
	}
	return out
}

func lowerIRExports(in []manifest.Export) []gwdkir.Export {
	out := make([]gwdkir.Export, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Export{Name: item.Name, Type: item.Type, Span: item.Span})
	}
	return out
}

func lowerIREmits(in []manifest.Emit) []gwdkir.Emit {
	out := make([]gwdkir.Emit, 0, len(in))
	for _, item := range in {
		params := make([]gwdkir.EmitParam, 0, len(item.Params))
		for _, param := range item.Params {
			params = append(params, gwdkir.EmitParam{Name: param.Name, Type: param.Type, Span: param.Span})
		}
		out = append(out, gwdkir.Emit{Name: item.Name, Params: params, Span: item.Span})
	}
	return out
}

func lowerIRStateContract(state manifest.StateContract) gwdkir.StateContract {
	return gwdkir.StateContract{
		Type: lowerIRGoTypeRef(state.Type),
		Init: lowerIRGoFuncRef(state.Init),
		Span: state.Span,
	}
}

func lowerIRGoTypeRef(ref manifest.GoTypeRef) gwdkir.GoRef {
	return gwdkir.GoRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

func lowerIRGoFuncRef(ref manifest.GoFuncRef) gwdkir.GoRef {
	return gwdkir.GoRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

func appendPackageImports(pkg *gwdkir.Package, imports []manifest.Import) {
	for _, item := range imports {
		irImport := gwdkir.Import{Alias: item.Alias, Path: item.Path, Span: item.Span}
		if !hasImport(pkg.Imports, irImport) {
			pkg.Imports = append(pkg.Imports, irImport)
		}
	}
}

func appendPackageUses(pkg *gwdkir.Package, uses []manifest.Use) {
	for _, item := range uses {
		irUse := gwdkir.Use{Alias: item.Alias, Package: item.Package, Span: item.Span}
		if !hasUse(pkg.Uses, irUse) {
			pkg.Uses = append(pkg.Uses, irUse)
		}
	}
}

func appendPackageStores(pkg *gwdkir.Package, stores []manifest.Store) {
	for _, item := range stores {
		pkg.Stores = append(pkg.Stores, gwdkir.Store{
			Name: item.Name,
			Type: gwdkir.GoRef{Alias: item.Type.Alias, Name: item.Type.Name, Span: item.Type.Span},
			Init: gwdkir.GoRef{Alias: item.Init.Alias, Name: item.Init.Name, Span: item.Init.Span},
			Span: item.Span,
		})
	}
}

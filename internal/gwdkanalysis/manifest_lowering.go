package gwdkanalysis

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// LowerPage lowers one page AST into manifest compatibility records.
func LowerPage(source string, ast gwdkast.File) (manifest.Page, error) {
	page := manifest.Page{Source: source}
	if ast.Package != nil {
		page.Package = ast.Package.Name
		page.Spans.Package = ast.Package.Span
	}
	page.Imports = lowerImports(ast.Imports)
	page.Uses = lowerUses(ast.Uses)
	page.Stores = lowerStores(ast.Stores)
	if ast.Page != nil {
		page.ID = ast.Page.ID
		page.Spans.Page = ast.Page.Span
	}
	if ast.Route != nil {
		page.Route = ast.Route.Path
		page.RouteParams = lowerRouteParams(ast.Route.Params)
		page.Spans.Route = ast.Route.Span
		page.Spans.RouteParams = lowerRouteParamSpans(ast.Route.Params)
	}
	if ast.Cache != nil {
		page.Cache = ast.Cache.Policy
		page.Spans.Cache = ast.Cache.Span
	}
	if ast.Revalidate != nil {
		page.Revalidate = ast.Revalidate.Seconds
		page.Spans.Revalidate = ast.Revalidate.Span
	}
	if ast.ErrorPage != nil {
		page.ErrorPage = ast.ErrorPage.Path
		page.Spans.ErrorPage = ast.ErrorPage.Span
	}
	for _, layout := range ast.Layouts {
		page.Layouts = append(page.Layouts, layout.ID)
		page.Spans.Layouts = append(page.Spans.Layouts, manifest.NamedSpan{Name: layout.ID, Span: layout.Span})
	}
	for _, guard := range ast.Guards {
		page.Guard = append(page.Guard, guard.Name)
		page.Spans.Guard = append(page.Spans.Guard, manifest.NamedSpan{Name: guard.Name, Span: guard.Span})
	}
	for _, asset := range ast.CSS {
		page.CSS = append(page.CSS, asset.Path)
		page.Spans.CSS = append(page.Spans.CSS, manifest.NamedSpan{Name: asset.Path, Span: asset.Span})
	}
	for _, asset := range ast.JS {
		if strings.TrimSpace(asset.Path) != "" {
			page.JS = append(page.JS, asset.Path)
			page.Spans.JS = append(page.Spans.JS, manifest.NamedSpan{Name: asset.Path, Span: asset.Span})
			continue
		}
		name := manifest.InlineScriptName(len(page.InlineJS))
		page.InlineJS = append(page.InlineJS, manifest.InlineScript{Name: name, Body: asset.Inline, Span: asset.Span})
		page.Spans.InlineJS = append(page.Spans.InlineJS, manifest.NamedSpan{Name: name, Span: asset.Span})
	}

	for _, annotation := range ast.Annotations {
		if hasTypedPageAnnotation(ast, annotation.Name) {
			continue
		}
		if err := applyPageAnnotation(&page, annotation); err != nil {
			return manifest.Page{}, err
		}
	}
	for _, block := range ast.Blocks {
		applyPageBlock(&page, block)
	}
	for _, endpoint := range ast.Actions {
		page.Blocks.Actions = append(page.Blocks.Actions, manifest.Action{
			Name:          endpoint.Name,
			Method:        endpoint.Method,
			Route:         endpoint.Route,
			ErrorPage:     endpoint.ErrorPage,
			Span:          endpoint.Span,
			RouteSpan:     endpoint.Span,
			RouteParams:   routeParamSpans(endpoint.Route, endpoint.Span),
			ErrorPageSpan: endpoint.ErrorPageSpan,
		})
		page.Blocks.Spans.Actions = append(page.Blocks.Spans.Actions, manifest.NamedSpan{Name: endpoint.Name, Span: endpoint.Span})
	}
	for _, endpoint := range ast.APIs {
		page.Blocks.APIs = append(page.Blocks.APIs, manifest.API{
			Name:          endpoint.Name,
			Method:        endpoint.Method,
			Route:         endpoint.Route,
			ErrorPage:     endpoint.ErrorPage,
			Span:          endpoint.Span,
			RouteSpan:     endpoint.Span,
			RouteParams:   routeParamSpans(endpoint.Route, endpoint.Span),
			ErrorPageSpan: endpoint.ErrorPageSpan,
		})
		page.Blocks.Spans.APIs = append(page.Blocks.Spans.APIs, manifest.NamedSpan{Name: endpoint.Name, Span: endpoint.Span})
	}
	for _, fragment := range ast.Fragments {
		page.Blocks.Fragments = append(page.Blocks.Fragments, manifest.FragmentEndpoint{
			Name:        fragment.Name,
			Method:      fragment.Method,
			Route:       fragment.Route,
			Target:      fragment.Target,
			Body:        fragment.Body,
			Span:        fragment.Span,
			RouteSpan:   fragment.RouteSpan,
			TargetSpan:  fragment.TargetSpan,
			RouteParams: routeParamSpans(fragment.Route, fragment.RouteSpan),
		})
		page.Blocks.Spans.Fragments = append(page.Blocks.Spans.Fragments, manifest.NamedSpan{Name: fragment.Name, Span: fragment.Span})
	}
	if page.ID == "" {
		return manifest.Page{}, fmt.Errorf("%s: missing @page", source)
	}
	if page.Route == "" {
		return manifest.Page{}, fmt.Errorf("%s: missing @route", source)
	}
	return page, nil
}

// LowerComponent lowers one component AST into manifest compatibility records.
func LowerComponent(source string, ast gwdkast.File) (manifest.Component, error) {
	component := manifest.Component{Source: source}
	if ast.Package != nil {
		component.Package = ast.Package.Name
		component.PackageSpan = ast.Package.Span
	}
	component.Imports = lowerImports(ast.Imports)
	component.Uses = lowerUses(ast.Uses)
	for _, asset := range ast.CSS {
		component.CSS = append(component.CSS, asset.Path)
		component.Spans.CSS = append(component.Spans.CSS, manifest.NamedSpan{Name: asset.Path, Span: asset.Span})
	}
	for _, asset := range ast.JS {
		if strings.TrimSpace(asset.Path) != "" {
			component.JS = append(component.JS, asset.Path)
			component.Spans.JS = append(component.Spans.JS, manifest.NamedSpan{Name: asset.Path, Span: asset.Span})
			continue
		}
		name := manifest.InlineScriptName(len(component.InlineJS))
		component.InlineJS = append(component.InlineJS, manifest.InlineScript{Name: name, Body: asset.Inline, Span: asset.Span})
		component.Spans.InlineJS = append(component.Spans.InlineJS, manifest.NamedSpan{Name: name, Span: asset.Span})
	}
	for _, asset := range ast.Assets {
		component.Assets = append(component.Assets, asset.Path)
		component.Spans.Assets = append(component.Spans.Assets, manifest.NamedSpan{Name: asset.Path, Span: asset.Span})
	}
	if ast.PropsType != nil {
		component.PropsType = lowerGoTypeRef(*ast.PropsType)
	}
	if ast.State != nil {
		component.State = manifest.StateContract{
			Type: lowerGoTypeRef(ast.State.Type),
			Init: lowerGoFuncRef(ast.State.Init),
			Span: ast.State.Span,
		}
	}
	if ast.WASM != nil {
		component.WASM = manifest.WASMContract{Package: ast.WASM.Package, Span: ast.WASM.Span}
	}
	if ast.Component != nil {
		component.Name = ast.Component.Name
		component.Span = ast.Component.Span
	}

	for _, annotation := range ast.Annotations {
		switch annotation.Name {
		case "component":
			if ast.Component != nil {
				continue
			}
			component.Name = strings.TrimSpace(annotation.Value)
			component.Span = annotation.Span
		case "wasm":
			if component.WASM.Package == "" {
				component.WASM = manifest.WASMContract{Package: trimQuotes(annotation.Value), Span: annotation.Span}
			}
		case "css":
			if len(component.CSS) == 0 {
				component.CSS = splitCSSList(annotation.Value)
				component.Spans.CSS = namedSpans(component.CSS, annotation.Span)
			}
		case "js":
			if len(component.JS) == 0 {
				component.JS = splitCSSList(annotation.Value)
				component.Spans.JS = namedSpans(component.JS, annotation.Span)
			}
		case "asset":
			if len(component.Assets) == 0 {
				component.Assets = splitCSSList(annotation.Value)
				component.Spans.Assets = namedSpans(component.Assets, annotation.Span)
			}
		default:
			return manifest.Component{}, fmt.Errorf("%s: unsupported component annotation @%s", source, annotation.Name)
		}
	}
	for _, block := range ast.Blocks {
		switch block.Kind {
		case "props":
			component.Props = lowerProps(block.Props)
		case "exports":
			component.Exports = lowerExports(block.Exports)
			component.Blocks.Spans.Exports = block.Span
		case "emits":
			component.Emits = lowerEmits(block.Emits)
			component.Blocks.Spans.Emits = block.Span
		case "client":
			component.Blocks.Client = true
			component.Blocks.ClientBody = block.Body
			component.Blocks.Spans.Client = block.Span
		case "go":
			component.Blocks.GoBlocks = append(component.Blocks.GoBlocks, manifest.GoBlock{
				Target: block.Name,
				Body:   block.Body,
				Span:   block.Span,
			})
			component.Blocks.Spans.GoBlocks = append(component.Blocks.Spans.GoBlocks, manifest.NamedSpan{Name: block.Name, Span: block.Span})
		case "view":
			component.Blocks.View = true
			component.Blocks.ViewBody = block.Body
			component.Blocks.Spans.View = block.Span
			component.Blocks.Spans.ViewBodyStart = block.BodyStart
		case "style":
			component.Blocks.Style = strings.TrimSpace(block.StyleBody) != ""
			component.Blocks.StyleBody = block.StyleBody
		default:
			return manifest.Component{}, fmt.Errorf("%s: unsupported component block %q", source, block.Kind)
		}
	}
	if component.Name == "" {
		return manifest.Component{}, fmt.Errorf("%s: missing @component", source)
	}
	return component, nil
}

// LowerLayout lowers one layout AST into manifest compatibility records.
func LowerLayout(source string, ast gwdkast.File) (manifest.Layout, error) {
	layout := manifest.Layout{Source: source}
	if ast.Package != nil {
		layout.Package = ast.Package.Name
		layout.PackageSpan = ast.Package.Span
	}
	layout.Uses = lowerUses(ast.Uses)
	if ast.Layout != nil {
		layout.ID = ast.Layout.ID
		layout.Span = ast.Layout.Span
	}
	for _, annotation := range ast.Annotations {
		switch annotation.Name {
		case "layout":
			if ast.Layout != nil {
				continue
			}
			layout.ID = trimQuotes(annotation.Value)
			layout.Span = annotation.Span
		default:
			return manifest.Layout{}, fmt.Errorf("%s: unsupported layout annotation @%s", source, annotation.Name)
		}
	}
	for _, block := range ast.Blocks {
		if block.Kind == "go" {
			layout.Blocks.GoBlocks = append(layout.Blocks.GoBlocks, manifest.GoBlock{
				Target: block.Name,
				Body:   block.Body,
				Span:   block.Span,
			})
			layout.Blocks.Spans.GoBlocks = append(layout.Blocks.Spans.GoBlocks, manifest.NamedSpan{Name: block.Name, Span: block.Span})
			continue
		}
		if block.Kind == "style" {
			layout.Blocks.Style = strings.TrimSpace(block.StyleBody) != ""
			layout.Blocks.StyleBody = block.StyleBody
			continue
		}
		if block.Kind != "view" {
			return manifest.Layout{}, fmt.Errorf("%s: unsupported layout block %q", source, block.Kind)
		}
		layout.Blocks.View = true
		layout.Blocks.ViewBody = block.Body
		layout.Blocks.Spans.View = block.Span
		layout.Blocks.Spans.ViewBodyStart = block.BodyStart
	}
	if layout.ID == "" {
		return manifest.Layout{}, fmt.Errorf("%s: missing @layout", source)
	}
	return layout, nil
}

func applyPageAnnotation(page *manifest.Page, annotation gwdkast.Annotation) error {
	value := strings.TrimSpace(annotation.Value)
	switch annotation.Name {
	case "page":
		page.ID = value
		page.Spans.Page = annotation.Span
	case "route":
		page.Route = trimQuotes(value)
		page.Spans.Route = annotation.Span
		page.Spans.RouteParams = routeParamSpans(page.Route, annotation.Span)
	case "cache":
		policy, err := cachePolicyValue(value)
		if err != nil {
			return err
		}
		page.Cache = policy
		page.Spans.Cache = annotation.Span
	case "revalidate":
		seconds, err := revalidateSecondsValue(value)
		if err != nil {
			return err
		}
		page.Revalidate = seconds
		page.Spans.Revalidate = annotation.Span
	case "error":
		errorPage, err := manifest.ErrorPagePath(trimQuotes(value))
		if err != nil {
			return err
		}
		page.ErrorPage = errorPage
		page.Spans.ErrorPage = annotation.Span
	case "layout":
		page.Layouts = splitCommaList(value)
		page.Spans.Layouts = namedSpans(page.Layouts, annotation.Span)
	case "title":
		page.Metadata.Title = trimQuotes(value)
		page.Spans.Title = annotation.Span
	case "description":
		page.Metadata.Description = trimQuotes(value)
		page.Spans.Description = annotation.Span
	case "canonical":
		page.Metadata.Canonical = trimQuotes(value)
		page.Spans.Canonical = annotation.Span
	case "image":
		page.Metadata.Image = trimQuotes(value)
		page.Spans.Image = annotation.Span
	case "guard":
		page.Guard = splitCommaList(value)
		page.Spans.Guard = namedSpans(page.Guard, annotation.Span)
	case "css":
		page.CSS = splitCSSList(value)
		page.Spans.CSS = namedSpans(page.CSS, annotation.Span)
	default:
		return fmt.Errorf("unsupported page annotation @%s", annotation.Name)
	}
	return nil
}

func hasTypedPageAnnotation(ast gwdkast.File, name string) bool {
	switch name {
	case "page":
		return ast.Page != nil
	case "route":
		return ast.Route != nil
	case "cache":
		return ast.Cache != nil
	case "layout":
		return len(ast.Layouts) > 0
	case "guard":
		return len(ast.Guards) > 0
	case "css":
		return len(ast.CSS) > 0
	default:
		return false
	}
}

func applyPageBlock(page *manifest.Page, block gwdkast.Block) {
	switch block.Kind {
	case "paths":
		page.Paths = true
		page.Blocks.PathsBody = block.Body
		page.Blocks.Spans.Paths = block.Span
	case "build":
		page.Blocks.Build = true
		page.Blocks.BuildBody = block.Body
		page.Blocks.Spans.Build = block.Span
	case "load":
		page.Blocks.Load = true
		page.Blocks.LoadBody = block.Body
		page.Blocks.Spans.Load = block.Span
	case "client":
		page.Blocks.Client = true
		page.Blocks.ClientBody = block.Body
		page.Blocks.Spans.Client = block.Span
	case "go":
		page.Blocks.GoBlocks = append(page.Blocks.GoBlocks, manifest.GoBlock{
			Target: block.Name,
			Body:   block.Body,
			Span:   block.Span,
		})
		page.Blocks.Spans.GoBlocks = append(page.Blocks.Spans.GoBlocks, manifest.NamedSpan{Name: block.Name, Span: block.Span})
	case "view":
		page.Blocks.View = true
		page.Blocks.ViewBody = block.Body
		page.Blocks.Spans.View = block.Span
		page.Blocks.Spans.ViewBodyStart = block.BodyStart
	case "style":
		page.Blocks.Style = strings.TrimSpace(block.StyleBody) != ""
		page.Blocks.StyleBody = block.StyleBody
	}
}

func lowerImports(in []gwdkast.Import) []manifest.Import {
	out := make([]manifest.Import, 0, len(in))
	for _, item := range in {
		out = append(out, manifest.Import{Alias: item.Alias, Path: item.Path, Span: item.Span})
	}
	return out
}

func lowerUses(in []gwdkast.Use) []manifest.Use {
	out := make([]manifest.Use, 0, len(in))
	for _, item := range in {
		out = append(out, manifest.Use{Alias: item.Alias, Package: item.Package, Span: item.Span})
	}
	return out
}

func lowerStores(in []gwdkast.Store) []manifest.Store {
	out := make([]manifest.Store, 0, len(in))
	for _, item := range in {
		out = append(out, manifest.Store{
			Name: item.Name,
			Type: lowerGoTypeRef(item.Type),
			Init: lowerGoFuncRef(item.Init),
			Span: item.Span,
		})
	}
	return out
}

func lowerRouteParamSpans(in []gwdkast.RouteParam) []manifest.NamedSpan {
	out := make([]manifest.NamedSpan, 0, len(in))
	for _, param := range in {
		out = append(out, manifest.NamedSpan{Name: param.Name, Span: param.Span})
	}
	return out
}

func lowerRouteParams(in []gwdkast.RouteParam) []manifest.RouteParam {
	out := make([]manifest.RouteParam, 0, len(in))
	for _, param := range in {
		paramType := param.Type
		if paramType == "" {
			paramType = "string"
		}
		out = append(out, manifest.RouteParam{Name: param.Name, Type: paramType, Span: param.Span})
	}
	return out
}

func lowerProps(in []gwdkast.Prop) []manifest.Prop {
	out := make([]manifest.Prop, 0, len(in))
	for _, item := range in {
		out = append(out, manifest.Prop{Name: item.Name, Type: item.Type, Span: item.Span})
	}
	return out
}

func lowerExports(in []gwdkast.Export) []manifest.Export {
	out := make([]manifest.Export, 0, len(in))
	for _, item := range in {
		out = append(out, manifest.Export{Name: item.Name, Type: item.Type, Span: item.Span})
	}
	return out
}

func lowerEmits(in []gwdkast.Emit) []manifest.Emit {
	out := make([]manifest.Emit, 0, len(in))
	for _, item := range in {
		params := make([]manifest.EmitParam, 0, len(item.Params))
		for _, param := range item.Params {
			params = append(params, manifest.EmitParam{Name: param.Name, Type: param.Type, Span: param.Span})
		}
		out = append(out, manifest.Emit{Name: item.Name, Params: params, Span: item.Span})
	}
	return out
}

func lowerGoTypeRef(ref gwdkast.GoTypeRef) manifest.GoTypeRef {
	return manifest.GoTypeRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

func lowerGoFuncRef(ref gwdkast.GoFuncRef) manifest.GoFuncRef {
	return manifest.GoFuncRef{Alias: ref.Alias, Name: ref.Name, Span: ref.Span}
}

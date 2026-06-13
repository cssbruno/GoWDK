package parser

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func lowerPageSyntax(src []byte, ast gwdkast.File, defaultID string) (gwdkir.Page, error) {
	var page gwdkir.Page
	if ast.Package != nil {
		page.Package = ast.Package.Name
		page.Spans.Package = ast.Package.Span
	}
	page.Imports = lowerSyntaxImports(ast.Imports)
	page.Uses = lowerSyntaxUses(ast.Uses)
	page.Stores = lowerSyntaxStores(ast.Stores)
	if err := lowerPageSyntaxMetadata(src, ast, &page); err != nil {
		return gwdkir.Page{}, err
	}
	for _, block := range ast.Blocks {
		applyPageSyntaxBlock(&page, block)
	}
	for _, endpoint := range ast.Actions {
		rawLine := sourceLineText(src, endpoint.Span.Start.Line)
		page.Blocks.Actions = append(page.Blocks.Actions, gwdkir.Action{
			Name:          endpoint.Name,
			Method:        endpoint.Method,
			Route:         endpoint.Route,
			ErrorPage:     endpoint.ErrorPage,
			Span:          endpoint.Span,
			RouteSpan:     endpoint.Span,
			RouteParams:   routeParamSpans(endpoint.Route, endpoint.Span.Start.Line, rawLine),
			ErrorPageSpan: endpoint.ErrorPageSpan,
		})
		page.Blocks.Spans.Actions = append(page.Blocks.Spans.Actions, source.NamedSpan{Name: endpoint.Name, Span: endpoint.Span})
	}
	for _, endpoint := range ast.APIs {
		rawLine := sourceLineText(src, endpoint.Span.Start.Line)
		page.Blocks.APIs = append(page.Blocks.APIs, gwdkir.API{
			Name:          endpoint.Name,
			Method:        endpoint.Method,
			Route:         endpoint.Route,
			ErrorPage:     endpoint.ErrorPage,
			Span:          endpoint.Span,
			RouteSpan:     endpoint.Span,
			RouteParams:   routeParamSpans(endpoint.Route, endpoint.Span.Start.Line, rawLine),
			ErrorPageSpan: endpoint.ErrorPageSpan,
		})
		page.Blocks.Spans.APIs = append(page.Blocks.Spans.APIs, source.NamedSpan{Name: endpoint.Name, Span: endpoint.Span})
	}
	for _, fragment := range ast.Fragments {
		rawLine := sourceLineText(src, fragment.RouteSpan.Start.Line)
		page.Blocks.Fragments = append(page.Blocks.Fragments, gwdkir.FragmentEndpoint{
			Name:        fragment.Name,
			Method:      fragment.Method,
			Route:       fragment.Route,
			Target:      fragment.Target,
			Body:        fragment.Body,
			Span:        fragment.Span,
			RouteSpan:   fragment.RouteSpan,
			TargetSpan:  fragment.TargetSpan,
			RouteParams: routeParamSpans(fragment.Route, fragment.RouteSpan.Start.Line, rawLine),
		})
		page.Blocks.Spans.Fragments = append(page.Blocks.Spans.Fragments, source.NamedSpan{Name: fragment.Name, Span: fragment.Span})
	}

	if page.ID == "" {
		page.ID = strings.TrimSpace(defaultID)
	}
	if page.ID == "" {
		return gwdkir.Page{}, fmt.Errorf("missing page")
	}
	if page.Route == "" {
		return gwdkir.Page{}, fmt.Errorf("%s missing route", page.ID)
	}
	return page, nil
}

func lowerPageSyntaxMetadata(src []byte, ast gwdkast.File, page *gwdkir.Page) error {
	if ast.Page != nil {
		if ast.Page.ID == "" {
			return fmt.Errorf("line %d: page requires a value", ast.Page.Span.Start.Line)
		}
		page.ID = ast.Page.ID
		page.Spans.Page = ast.Page.Span
	}
	if ast.Route != nil {
		if ast.Route.Path == "" {
			return fmt.Errorf("line %d: route requires a value", ast.Route.Span.Start.Line)
		}
		page.Route = ast.Route.Path
		page.RouteParams = lowerSyntaxRouteParams(ast.Route.Params)
		page.Spans.Route = ast.Route.Span
		page.Spans.RouteParams = lowerSyntaxRouteParamSpans(ast.Route.Params)
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
		page.Spans.Layouts = append(page.Spans.Layouts, source.NamedSpan{Name: layout.ID, Span: layout.Span})
	}
	for _, guard := range ast.Guards {
		page.Guards = append(page.Guards, guard.Name)
		page.Spans.Guard = append(page.Spans.Guard, source.NamedSpan{Name: guard.Name, Span: guard.Span})
	}
	for _, css := range ast.CSS {
		page.CSS = append(page.CSS, css.Path)
		page.Spans.CSS = append(page.Spans.CSS, source.NamedSpan{Name: css.Path, Span: css.Span})
	}
	for _, script := range ast.JS {
		if strings.TrimSpace(script.Path) != "" {
			page.JS = append(page.JS, script.Path)
			page.Spans.JS = append(page.Spans.JS, source.NamedSpan{Name: script.Path, Span: script.Span})
			continue
		}
		name := source.InlineScriptName(len(page.InlineJS))
		page.InlineJS = append(page.InlineJS, source.InlineScript{Name: name, Body: script.Inline, Span: script.Span})
		page.Spans.InlineJS = append(page.Spans.InlineJS, source.NamedSpan{Name: name, Span: script.Span})
	}
	for _, metadata := range ast.Metadata {
		if pageMetadataLoweredFromAST(ast, metadata.Name) {
			continue
		}
		lineNumber := metadata.Span.Start.Line
		rawLine := sourceLineText(src, lineNumber)
		if err := applyMetadata(page, metadata.Name, metadata.Value, lineNumber, rawLine); err != nil {
			return withLine(lineNumber, err)
		}
	}
	return nil
}

func pageMetadataLoweredFromAST(ast gwdkast.File, name string) bool {
	switch name {
	case "page":
		return ast.Page != nil
	case "route":
		return ast.Route != nil
	case "cache":
		return ast.Cache != nil
	case "revalidate":
		return ast.Revalidate != nil
	case "error":
		return ast.ErrorPage != nil
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

func lowerSyntaxImports(in []gwdkast.Import) []gwdkir.Import {
	out := make([]gwdkir.Import, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Import{Alias: item.Alias, Path: item.Path, Span: item.Span})
	}
	return out
}

func lowerSyntaxUses(in []gwdkast.Use) []gwdkir.Use {
	out := make([]gwdkir.Use, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Use{Alias: item.Alias, Package: item.Package, Span: item.Span})
	}
	return out
}

func lowerSyntaxStores(in []gwdkast.Store) []gwdkir.Store {
	out := make([]gwdkir.Store, 0, len(in))
	for _, item := range in {
		out = append(out, gwdkir.Store{
			Name:    item.Name,
			Type:    gwdkir.GoRef{Alias: item.Type.Alias, Name: item.Type.Name, Span: item.Type.Span},
			Init:    gwdkir.GoRef{Alias: item.Init.Alias, Name: item.Init.Name, Span: item.Init.Span},
			Persist: item.Persist,
			Span:    item.Span,
		})
	}
	return out
}

func lowerSyntaxRouteParams(in []gwdkast.RouteParam) []source.RouteParam {
	out := make([]source.RouteParam, 0, len(in))
	for _, param := range in {
		paramType := param.Type
		if paramType == "" {
			paramType = "string"
		}
		out = append(out, source.RouteParam{Name: param.Name, Type: paramType, Span: param.Span})
	}
	return out
}

func lowerSyntaxRouteParamSpans(in []gwdkast.RouteParam) []source.NamedSpan {
	out := make([]source.NamedSpan, 0, len(in))
	for _, param := range in {
		out = append(out, source.NamedSpan{Name: param.Name, Span: param.Span})
	}
	return out
}

func applyPageSyntaxBlock(page *gwdkir.Page, block gwdkast.Block) {
	switch block.Kind {
	case "paths":
		page.Blocks.Paths = true
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
		page.Blocks.GoBlocks = append(page.Blocks.GoBlocks, gwdkir.GoBlock{
			Target: block.Name,
			Body:   block.Body,
			Span:   block.Span,
		})
		page.Blocks.Spans.GoBlocks = append(page.Blocks.Spans.GoBlocks, source.NamedSpan{Name: block.Name, Span: block.Span})
	case "view":
		page.Blocks.View = true
		page.Blocks.ViewBody = block.Body
		page.Blocks.Spans.View = block.Span
		page.Blocks.Spans.ViewBodyStart = block.BodyStart
	case "style":
		page.Blocks.StyleBody = block.StyleBody
		page.Blocks.Style = strings.TrimSpace(block.StyleBody) != ""
	}
}

func sourceLineText(src []byte, lineNumber int) string {
	if lineNumber <= 0 {
		return ""
	}
	lines := strings.Split(string(src), "\n")
	if lineNumber > len(lines) {
		return ""
	}
	return strings.TrimSuffix(lines[lineNumber-1], "\r")
}

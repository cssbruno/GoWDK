// Package gwdkanalysis lowers GOWDK AST files into normalized manifest and IR
// metadata.
package gwdkanalysis

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/cssscope"
	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

var routeParamPattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)(?::([A-Za-z_][A-Za-z0-9_]*))?\}`)

type SourceKind = gwdkir.SourceKind

const (
	SourcePage      = gwdkir.SourcePage
	SourceComponent = gwdkir.SourceComponent
	SourceLayout    = gwdkir.SourceLayout
)

// SourceFile is one parsed GOWDK AST file ready for analysis.
type SourceFile struct {
	Path string
	Kind SourceKind
	AST  gwdkast.File
}

// Result contains compatibility manifest records plus the stable IR produced
// from them.
type Result struct {
	Manifest manifest.Manifest
	IR       gwdkir.Program
}

// Analyze lowers parsed AST files into normalized compiler metadata.
func Analyze(config gowdk.Config, files []SourceFile) (Result, error) {
	var result Result
	result.IR.Version = gwdkir.Version

	for _, file := range files {
		switch file.Kind {
		case SourcePage:
			page, err := LowerPage(file.Path, file.AST)
			if err != nil {
				return Result{}, err
			}
			result.Manifest.Pages = append(result.Manifest.Pages, page)
		case SourceComponent:
			component, err := LowerComponent(file.Path, file.AST)
			if err != nil {
				return Result{}, err
			}
			result.Manifest.Components = append(result.Manifest.Components, component)
		case SourceLayout:
			layout, err := LowerLayout(file.Path, file.AST)
			if err != nil {
				return Result{}, err
			}
			result.Manifest.Layouts = append(result.Manifest.Layouts, layout)
		default:
			return Result{}, fmt.Errorf("unsupported GOWDK source kind %q for %s", file.Kind, file.Path)
		}
	}

	result.IR = BuildIR(config, result.Manifest)
	return result, nil
}

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
	if ast.Render != nil {
		page.Render = gowdk.RenderMode(ast.Render.Mode)
		page.Spans.Render = ast.Render.Span
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

// BuildIR converts a normalized manifest into the stable compiler IR.
func BuildIR(config gowdk.Config, app manifest.Manifest) gwdkir.Program {
	program := gwdkir.Program{Version: gwdkir.Version}
	packages := map[string]*gwdkir.Package{}
	ensurePackage := func(name string, source string) *gwdkir.Package {
		pkg := packages[name]
		if pkg == nil {
			pkg = &gwdkir.Package{Name: name}
			packages[name] = pkg
		}
		dir := filepath.Dir(source)
		if source != "" && !contains(pkg.SourceDirs, dir) {
			pkg.SourceDirs = append(pkg.SourceDirs, dir)
		}
		return pkg
	}

	for _, page := range app.Pages {
		program.Pages = append(program.Pages, lowerIRPage(page))
		pkg := ensurePackage(page.Package, page.Source)
		pkg.Files = append(pkg.Files, gwdkir.SourceFile{Path: page.Source, Kind: gwdkir.SourcePage, Package: page.Package, Name: page.ID, Span: page.Spans.Page})
		appendPackageImports(pkg, page.Imports)
		appendPackageUses(pkg, page.Uses)
		appendPackageStores(pkg, page.Stores)

		mode := page.RenderMode(config.Render.DefaultMode())
		program.Routes = append(program.Routes, gwdkir.Route{
			Kind:          routeKind(mode, page),
			Method:        "GET",
			Path:          page.Route,
			PageID:        page.ID,
			Package:       page.Package,
			Render:        mode,
			Cache:         page.CachePolicy(),
			DynamicParams: page.DynamicParams(),
			RouteParams:   copyRouteParams(page.TypedRouteParams()),
			Layouts:       append([]string(nil), page.Layouts...),
			Guards:        append([]string(nil), page.Guard...),
			Source:        page.Source,
			Span:          page.Spans.Route,
		})
		if page.Blocks.View {
			template := gwdkir.Template{
				OwnerKind: gwdkir.SourcePage,
				OwnerID:   page.ID,
				Package:   page.Package,
				Source:    page.Source,
				Route:     page.Route,
				Guards:    append([]string(nil), page.Guard...),
				Imports:   lowerIRImports(page.Imports),
				Body:      page.Blocks.ViewBody,
				Span:      page.Blocks.Spans.View,
				BodyStart: page.Blocks.Spans.ViewBodyStart,
			}
			program.Templates = append(program.Templates, template)
			appendContractReferences(&program, template)
		}
		for _, css := range page.CSS {
			name, useAlias, usePackage := assetUse(page.Uses, css)
			program.Assets = append(program.Assets, gwdkir.Asset{
				Kind:       gwdkir.AssetCSS,
				OwnerID:    page.ID,
				Package:    page.Package,
				Source:     page.Source,
				Path:       css,
				Name:       name,
				UseAlias:   useAlias,
				UsePackage: usePackage,
				Span:       spanForName(page.Spans.CSS, css, page.Spans.Page),
			})
		}
		for _, action := range page.Blocks.Actions {
			method := action.Method
			if method == "" {
				method = "POST"
			}
			path := action.Route
			if path == "" {
				path = page.Route
			}
			program.Endpoints = append(program.Endpoints, gwdkir.Endpoint{
				Kind:          gwdkir.EndpointAction,
				Source:        gwdkir.EndpointSourceGOWDK,
				Package:       page.Package,
				PageID:        page.ID,
				Symbol:        action.Name,
				Method:        method,
				Path:          path,
				ErrorPage:     action.ErrorPage,
				DynamicParams: routeParams(path),
				SourceFile:    page.Source,
				Span:          action.Span,
			})
		}
		for _, api := range page.Blocks.APIs {
			method := api.Method
			if method == "" {
				method = "GET"
			}
			path := api.Route
			if path == "" {
				path = page.Route
			}
			program.Endpoints = append(program.Endpoints, gwdkir.Endpoint{
				Kind:          gwdkir.EndpointAPI,
				Source:        gwdkir.EndpointSourceGOWDK,
				Package:       page.Package,
				PageID:        page.ID,
				Symbol:        api.Name,
				Method:        method,
				Path:          path,
				ErrorPage:     api.ErrorPage,
				DynamicParams: routeParams(path),
				SourceFile:    page.Source,
				Span:          api.Span,
			})
		}
		for _, fragment := range page.Blocks.Fragments {
			method := fragment.Method
			if method == "" {
				method = "GET"
			}
			program.Endpoints = append(program.Endpoints, gwdkir.Endpoint{
				Kind:          gwdkir.EndpointFragment,
				Source:        gwdkir.EndpointSourceGOWDK,
				Package:       page.Package,
				PageID:        page.ID,
				Symbol:        fragment.Name,
				Method:        method,
				Path:          fragment.Route,
				DynamicParams: routeParams(fragment.Route),
				SourceFile:    page.Source,
				Span:          fragment.Span,
			})
		}
	}

	for _, component := range app.Components {
		program.Components = append(program.Components, lowerIRComponent(component))
		pkg := ensurePackage(component.Package, component.Source)
		pkg.Files = append(pkg.Files, gwdkir.SourceFile{Path: component.Source, Kind: gwdkir.SourceComponent, Package: component.Package, Name: component.Name, Span: component.Span})
		appendPackageImports(pkg, component.Imports)
		appendPackageUses(pkg, component.Uses)
		for _, css := range component.CSS {
			hashKey := cssscope.HashKey("component", component.Package, component.Name, component.Source, css)
			program.Assets = append(program.Assets, gwdkir.Asset{
				Kind:    gwdkir.AssetCSS,
				OwnerID: component.Name,
				Package: component.Package,
				Source:  component.Source,
				Path:    css,
				ScopeID: cssscope.ScopeID(hashKey),
				HashKey: hashKey,
				Span:    spanForName(component.Spans.CSS, css, component.Span),
			})
		}
		for _, asset := range component.Assets {
			program.Assets = append(program.Assets, gwdkir.Asset{
				Kind:    gwdkir.AssetFile,
				OwnerID: component.Name,
				Package: component.Package,
				Source:  component.Source,
				Path:    asset,
				Span:    spanForName(component.Spans.Assets, asset, component.Span),
			})
		}
		if component.Blocks.View {
			template := gwdkir.Template{
				OwnerKind: gwdkir.SourceComponent,
				OwnerID:   component.Name,
				Package:   component.Package,
				Source:    component.Source,
				Imports:   lowerIRImports(component.Imports),
				Body:      component.Blocks.ViewBody,
				Span:      component.Blocks.Spans.View,
				BodyStart: component.Blocks.Spans.ViewBodyStart,
			}
			program.Templates = append(program.Templates, template)
			appendContractReferences(&program, template)
		}
		if component.Blocks.Client {
			program.ClientBehaviors = append(program.ClientBehaviors, gwdkir.ClientBehavior{
				Component: component.Name,
				Package:   component.Package,
				Source:    component.Source,
				Body:      component.Blocks.ClientBody,
				Span:      component.Blocks.Spans.Client,
			})
		}
		if component.WASM.Package != "" {
			program.Assets = append(program.Assets, gwdkir.Asset{
				Kind:    gwdkir.AssetWASM,
				OwnerID: component.Name,
				Package: component.Package,
				Source:  component.Source,
				Path:    component.WASM.Package,
				Span:    component.WASM.Span,
			})
		}
	}

	for _, layout := range app.Layouts {
		program.Layouts = append(program.Layouts, lowerIRLayout(layout))
		pkg := ensurePackage(layout.Package, layout.Source)
		pkg.Files = append(pkg.Files, gwdkir.SourceFile{Path: layout.Source, Kind: gwdkir.SourceLayout, Package: layout.Package, Name: layout.ID, Span: layout.Span})
		appendPackageUses(pkg, layout.Uses)
		if layout.Blocks.View {
			template := gwdkir.Template{
				OwnerKind: gwdkir.SourceLayout,
				OwnerID:   layout.ID,
				Package:   layout.Package,
				Source:    layout.Source,
				Body:      layout.Blocks.ViewBody,
				Span:      layout.Blocks.Spans.View,
				BodyStart: layout.Blocks.Spans.ViewBodyStart,
			}
			program.Templates = append(program.Templates, template)
			appendContractReferences(&program, template)
		}
	}
	for _, endpoint := range app.Endpoints {
		kind := gwdkir.EndpointAPI
		if endpoint.Kind == "act" || endpoint.Kind == "action" {
			kind = gwdkir.EndpointAction
		}
		method := endpoint.Method
		if method == "" {
			if kind == gwdkir.EndpointAction {
				method = "POST"
			} else {
				method = "GET"
			}
		}
		program.Endpoints = append(program.Endpoints, gwdkir.Endpoint{
			Kind:          kind,
			Source:        endpointSource(endpoint.SourceKind),
			Package:       endpoint.Package,
			PageID:        standaloneEndpointPageID(endpoint),
			Symbol:        endpoint.Name,
			Method:        method,
			Path:          endpoint.Route,
			DynamicParams: routeParams(endpoint.Route),
			SourceFile:    endpoint.Source,
			Span:          endpoint.Span,
		})
	}

	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pkg := packages[name]
		sort.Strings(pkg.SourceDirs)
		sort.Slice(pkg.Files, func(i, j int) bool { return pkg.Files[i].Path < pkg.Files[j].Path })
		program.Packages = append(program.Packages, *pkg)
	}
	sort.Slice(program.Routes, func(i, j int) bool { return program.Routes[i].Path < program.Routes[j].Path })
	sort.Slice(program.Endpoints, func(i, j int) bool {
		if program.Endpoints[i].Path == program.Endpoints[j].Path {
			return program.Endpoints[i].Method < program.Endpoints[j].Method
		}
		return program.Endpoints[i].Path < program.Endpoints[j].Path
	})
	attachBackendBindings(&program, app.BackendBindings)
	return program
}

func appendContractReferences(program *gwdkir.Program, template gwdkir.Template) {
	refs, err := view.ContractReferences(template.Body)
	if err != nil {
		return
	}
	for _, ref := range refs {
		method := ref.Method
		path := ref.Path
		if ref.Kind == view.ContractReferenceQuery && path == "" && template.Route != "" {
			method = "GET"
			path = template.Route
		}
		importAlias, contractType := splitContractReferenceName(ref.Name)
		importPath := contractReferenceImportPath(template.Imports, importAlias)
		program.ContractRefs = append(program.ContractRefs, gwdkir.ContractReference{
			Kind:        irContractReferenceKind(ref.Kind),
			Name:        ref.Name,
			ImportAlias: importAlias,
			ImportPath:  importPath,
			Type:        contractType,
			Guards:      append([]string(nil), template.Guards...),
			Method:      method,
			Path:        path,
			OwnerKind:   template.OwnerKind,
			OwnerID:     template.OwnerID,
			Package:     template.Package,
			Source:      template.Source,
			Span:        templateOffsetSpan(template, ref.Start, ref.End),
		})
	}
}

func splitContractReferenceName(name string) (string, string) {
	before, after, ok := strings.Cut(name, ".")
	if !ok {
		return "", name
	}
	return before, after
}

func contractReferenceImportPath(imports []gwdkir.Import, alias string) string {
	if alias == "" {
		return ""
	}
	for _, item := range imports {
		if item.Alias == alias {
			return item.Path
		}
	}
	return ""
}

func irContractReferenceKind(kind view.ContractReferenceKind) gwdkir.ContractKind {
	switch kind {
	case view.ContractReferenceQuery:
		return gwdkir.ContractQuery
	default:
		return gwdkir.ContractCommand
	}
}

func templateOffsetSpan(template gwdkir.Template, start int, end int) manifest.SourceSpan {
	if start < 0 || end <= start || start >= len([]rune(template.Body)) {
		return template.Span
	}
	startPos := templateOffsetPosition(template, start)
	endPos := templateOffsetPosition(template, end)
	if startPos.Line == 0 || endPos.Line == 0 {
		return template.Span
	}
	return manifest.SourceSpan{Start: startPos, End: endPos}
}

func templateOffsetPosition(template gwdkir.Template, offset int) manifest.SourcePosition {
	line := template.BodyStart.Line
	column := template.BodyStart.Column
	if line == 0 {
		line = template.Span.Start.Line + 1
		column = 1
	}
	for index, char := range []rune(template.Body) {
		if index == offset {
			return manifest.SourcePosition{Line: line, Column: column}
		}
		if char == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	if offset == len([]rune(template.Body)) {
		return manifest.SourcePosition{Line: line, Column: column}
	}
	return manifest.SourcePosition{}
}

func endpointSource(source manifest.EndpointSource) gwdkir.EndpointSource {
	if source == manifest.EndpointSourceGo {
		return gwdkir.EndpointSourceGo
	}
	return gwdkir.EndpointSourceGOWDK
}

func standaloneEndpointPageID(endpoint manifest.EndpointDeclaration) string {
	if endpoint.Package == "" {
		return endpoint.Name
	}
	return endpoint.Package + "." + endpoint.Name
}

func assetUse(uses []manifest.Use, path string) (name string, useAlias string, usePackage string) {
	alias, assetName, ok := strings.Cut(path, ".")
	if !ok {
		return path, "", ""
	}
	for _, use := range uses {
		if use.Alias == alias {
			return assetName, alias, use.Package
		}
	}
	return assetName, alias, ""
}

func attachBackendBindings(program *gwdkir.Program, bindings []manifest.BackendBinding) {
	byEndpoint := map[string]manifest.BackendBinding{}
	byLoadPage := map[string]manifest.BackendBinding{}
	for _, binding := range bindings {
		if binding.Kind == "load" {
			byLoadPage[binding.PageID] = binding
			continue
		}
		kind := gwdkir.EndpointAction
		if binding.Kind == "api" {
			kind = gwdkir.EndpointAPI
		} else if binding.Kind == "fragment" {
			kind = gwdkir.EndpointFragment
		}
		byEndpoint[endpointKey(kind, binding.PageID, binding.BlockName, binding.Method, binding.Route)] = binding
	}
	for index := range program.Endpoints {
		endpoint := &program.Endpoints[index]
		binding := byEndpoint[endpointKey(endpoint.Kind, endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Path)]
		endpoint.Binding = gwdkir.Binding{
			Status:       binding.Status,
			Message:      binding.Message,
			ImportPath:   binding.ImportPath,
			PackageName:  binding.PackageName,
			FunctionName: binding.FunctionName,
			Signature:    binding.Signature,
			InputType:    binding.InputType,
			InputPointer: binding.InputPointer,
			InputFields:  append([]manifest.BackendInputField(nil), binding.InputFields...),
		}
	}
	for index := range program.Pages {
		page := &program.Pages[index]
		binding := byLoadPage[page.ID]
		page.LoadBinding = gwdkir.Binding{
			Status:       binding.Status,
			Message:      binding.Message,
			ImportPath:   binding.ImportPath,
			PackageName:  binding.PackageName,
			FunctionName: binding.FunctionName,
			Signature:    binding.Signature,
		}
	}
}

func endpointKey(kind gwdkir.EndpointKind, pageID, symbol, method, route string) string {
	return strings.Join([]string{string(kind), pageID, symbol, method, route}, "\x00")
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
	case "render":
		page.Render = gowdk.RenderMode(value)
		page.Spans.Render = annotation.Span
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
	case "render":
		return ast.Render != nil
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

func routeParams(route string) []string {
	matches := routeParamPattern.FindAllStringSubmatch(route, -1)
	out := make([]string, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		name := match[1]
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func typedRouteParams(route string) []manifest.RouteParam {
	matches := routeParamPattern.FindAllStringSubmatch(route, -1)
	out := make([]manifest.RouteParam, 0, len(matches))
	seen := map[string]bool{}
	for _, match := range matches {
		name := match[1]
		if seen[name] {
			continue
		}
		seen[name] = true
		paramType := match[2]
		if paramType == "" {
			paramType = "string"
		}
		out = append(out, manifest.RouteParam{Name: name, Type: paramType})
	}
	return out
}

func routeParamSpans(route string, fallback manifest.SourceSpan) []manifest.NamedSpan {
	params := routeParams(route)
	out := make([]manifest.NamedSpan, 0, len(params))
	for _, param := range params {
		out = append(out, manifest.NamedSpan{Name: param, Span: fallback})
	}
	return out
}

func namedSpans(values []string, fallback manifest.SourceSpan) []manifest.NamedSpan {
	out := make([]manifest.NamedSpan, 0, len(values))
	for _, value := range values {
		out = append(out, manifest.NamedSpan{Name: value, Span: fallback})
	}
	return out
}

func splitCommaList(value string) []string {
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(trimQuotes(part))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func splitCSSList(value string) []string {
	value = strings.ReplaceAll(value, ",", " ")
	parts := strings.Fields(value)
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(trimQuotes(part))
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func cachePolicyValue(value string) (string, error) {
	policy := strings.TrimSpace(trimQuotes(value))
	if policy == "" {
		return "", fmt.Errorf("@cache requires a value")
	}
	if strings.ContainsAny(policy, "\r\n") {
		return "", fmt.Errorf("@cache must stay on one line")
	}
	return policy, nil
}

func revalidateSecondsValue(value string) (string, error) {
	raw := strings.TrimSpace(trimQuotes(value))
	if raw == "" {
		return "", fmt.Errorf("@revalidate requires a value")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return "", fmt.Errorf("@revalidate must stay on one line")
	}
	if seconds, err := strconv.Atoi(raw); err == nil {
		if seconds <= 0 {
			return "", fmt.Errorf("@revalidate requires a positive duration")
		}
		return strconv.Itoa(seconds), nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return "", fmt.Errorf("@revalidate requires a positive duration such as 60s, 5m, or 1h")
	}
	if duration%time.Second != 0 {
		return "", fmt.Errorf("@revalidate must resolve to whole seconds")
	}
	return strconv.FormatInt(int64(duration/time.Second), 10), nil
}

func spanForName(spans []manifest.NamedSpan, name string, fallback manifest.SourceSpan) manifest.SourceSpan {
	for _, span := range spans {
		if span.Name == name {
			return span.Span
		}
	}
	return fallback
}

func contains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func hasImport(values []gwdkir.Import, value gwdkir.Import) bool {
	for _, item := range values {
		if item.Alias == value.Alias && item.Path == value.Path {
			return true
		}
	}
	return false
}

func hasUse(values []gwdkir.Use, value gwdkir.Use) bool {
	for _, item := range values {
		if item.Alias == value.Alias && item.Package == value.Package {
			return true
		}
	}
	return false
}

func trimQuotes(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}

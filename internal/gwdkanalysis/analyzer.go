// Package gwdkanalysis lowers GOWDK AST files into normalized manifest and IR
// metadata.
package gwdkanalysis

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
)

var routeParamPattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

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
		page.Spans.Route = ast.Route.Span
		page.Spans.RouteParams = lowerRouteParamSpans(ast.Route.Params)
	}
	if ast.Render != nil {
		page.Render = gowdk.RenderMode(ast.Render.Mode)
		page.Spans.Render = ast.Render.Span
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
			Name:        endpoint.Name,
			Method:      endpoint.Method,
			Route:       endpoint.Route,
			Span:        endpoint.Span,
			RouteSpan:   endpoint.Span,
			RouteParams: routeParamSpans(endpoint.Route, endpoint.Span),
		})
		page.Blocks.Spans.Actions = append(page.Blocks.Spans.Actions, manifest.NamedSpan{Name: endpoint.Name, Span: endpoint.Span})
	}
	for _, endpoint := range ast.APIs {
		page.Blocks.APIs = append(page.Blocks.APIs, manifest.API{
			Name:        endpoint.Name,
			Method:      endpoint.Method,
			Route:       endpoint.Route,
			Span:        endpoint.Span,
			RouteSpan:   endpoint.Span,
			RouteParams: routeParamSpans(endpoint.Route, endpoint.Span),
		})
		page.Blocks.Spans.APIs = append(page.Blocks.Spans.APIs, manifest.NamedSpan{Name: endpoint.Name, Span: endpoint.Span})
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
		default:
			return manifest.Component{}, fmt.Errorf("%s: unsupported component annotation @%s", source, annotation.Name)
		}
	}
	for _, block := range ast.Blocks {
		switch block.Kind {
		case "props":
			component.Props = lowerProps(block.Props)
		case "emits":
			component.Emits = lowerEmits(block.Emits)
			component.Blocks.Spans.Emits = block.Span
		case "client":
			component.Blocks.Client = true
			component.Blocks.ClientBody = block.Body
			component.Blocks.Spans.Client = block.Span
		case "view":
			component.Blocks.View = true
			component.Blocks.ViewBody = block.Body
			component.Blocks.Spans.View = block.Span
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
		if block.Kind != "view" {
			return manifest.Layout{}, fmt.Errorf("%s: unsupported layout block %q", source, block.Kind)
		}
		layout.Blocks.View = true
		layout.Blocks.ViewBody = block.Body
		layout.Blocks.Spans.View = block.Span
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
			Kind:          routeKind(mode),
			Method:        "GET",
			Path:          page.Route,
			PageID:        page.ID,
			Package:       page.Package,
			Render:        mode,
			DynamicParams: page.DynamicParams(),
			Layouts:       append([]string(nil), page.Layouts...),
			Guards:        append([]string(nil), page.Guard...),
			Source:        page.Source,
			Span:          page.Spans.Route,
		})
		if page.Blocks.View {
			program.Templates = append(program.Templates, gwdkir.Template{
				OwnerKind: gwdkir.SourcePage,
				OwnerID:   page.ID,
				Package:   page.Package,
				Source:    page.Source,
				Body:      page.Blocks.ViewBody,
				Span:      page.Blocks.Spans.View,
			})
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
				DynamicParams: routeParams(path),
				SourceFile:    page.Source,
				Span:          api.Span,
			})
		}
	}

	for _, component := range app.Components {
		program.Components = append(program.Components, lowerIRComponent(component))
		pkg := ensurePackage(component.Package, component.Source)
		pkg.Files = append(pkg.Files, gwdkir.SourceFile{Path: component.Source, Kind: gwdkir.SourceComponent, Package: component.Package, Name: component.Name, Span: component.Span})
		appendPackageImports(pkg, component.Imports)
		appendPackageUses(pkg, component.Uses)
		if component.Blocks.View {
			program.Templates = append(program.Templates, gwdkir.Template{
				OwnerKind: gwdkir.SourceComponent,
				OwnerID:   component.Name,
				Package:   component.Package,
				Source:    component.Source,
				Body:      component.Blocks.ViewBody,
				Span:      component.Blocks.Spans.View,
			})
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
			program.Templates = append(program.Templates, gwdkir.Template{
				OwnerKind: gwdkir.SourceLayout,
				OwnerID:   layout.ID,
				Package:   layout.Package,
				Source:    layout.Source,
				Body:      layout.Blocks.ViewBody,
				Span:      layout.Blocks.Spans.View,
			})
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
	for _, binding := range bindings {
		kind := gwdkir.EndpointAction
		if binding.Kind == "api" {
			kind = gwdkir.EndpointAPI
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
	case "view":
		page.Blocks.View = true
		page.Blocks.ViewBody = block.Body
		page.Blocks.Spans.View = block.Span
	}
}

func routeKind(mode gowdk.RenderMode) gwdkir.RouteKind {
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
		Source:   page.Source,
		Package:  page.Package,
		ID:       page.ID,
		Route:    page.Route,
		Render:   page.Render,
		Metadata: gwdkir.PageMetadata(page.Metadata),
		Layouts:  append([]string(nil), page.Layouts...),
		Guards:   append([]string(nil), page.Guard...),
		CSS:      append([]string(nil), page.CSS...),
		Imports:  lowerIRImports(page.Imports),
		Uses:     lowerIRUses(page.Uses),
		Stores:   lowerIRStores(page.Stores),
		Blocks:   lowerIRBlocks(page.Blocks),
		Spans: gwdkir.PageSpans{
			Package:     page.Spans.Package,
			Page:        page.Spans.Page,
			Route:       page.Spans.Route,
			Render:      page.Spans.Render,
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

func lowerIRComponent(component manifest.Component) gwdkir.Component {
	return gwdkir.Component{
		Source:      component.Source,
		Package:     component.Package,
		Name:        component.Name,
		Imports:     lowerIRImports(component.Imports),
		Uses:        lowerIRUses(component.Uses),
		Props:       lowerIRProps(component.Props),
		PropsType:   lowerIRGoTypeRef(component.PropsType),
		State:       lowerIRStateContract(component.State),
		WASM:        gwdkir.WASMContract(component.WASM),
		Emits:       lowerIREmits(component.Emits),
		Blocks:      lowerIRBlocks(component.Blocks),
		Span:        component.Span,
		PackageSpan: component.PackageSpan,
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
		View:       blocks.View,
		ViewBody:   blocks.ViewBody,
		Actions:    lowerIRActions(blocks.Actions),
		APIs:       lowerIRAPIs(blocks.APIs),
		Spans: gwdkir.BlockSpans{
			Paths:   blocks.Spans.Paths,
			Build:   blocks.Spans.Build,
			Load:    blocks.Spans.Load,
			Client:  blocks.Spans.Client,
			View:    blocks.Spans.View,
			Actions: append([]manifest.NamedSpan(nil), blocks.Spans.Actions...),
			APIs:    append([]manifest.NamedSpan(nil), blocks.Spans.APIs...),
			Emits:   blocks.Spans.Emits,
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

func lowerIRAPIs(apis []manifest.API) []gwdkir.API {
	out := make([]gwdkir.API, 0, len(apis))
	for _, api := range apis {
		out = append(out, gwdkir.API{
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

func lowerIRFragments(fragments []manifest.Fragment) []gwdkir.Fragment {
	out := make([]gwdkir.Fragment, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, gwdkir.Fragment{Target: fragment.Target, Body: fragment.Body, Span: fragment.Span})
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

func lowerProps(in []gwdkast.Prop) []manifest.Prop {
	out := make([]manifest.Prop, 0, len(in))
	for _, item := range in {
		out = append(out, manifest.Prop{Name: item.Name, Type: item.Type, Span: item.Span})
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

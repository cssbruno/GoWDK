package gwdkanalysis

import (
	"path/filepath"
	"sort"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/cssscope"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/view"
)

// Sources are the parsed IR records one program is assembled from.
type Sources struct {
	Pages      []gwdkir.Page
	Components []gwdkir.Component
	Layouts    []gwdkir.Layout
	AuditSpecs []gwdkir.AuditSpec
}

// BuildProgram assembles the stable compiler IR from parsed IR records:
// routes, templates, assets, endpoints, and package groupings are derived
// here. Standalone Go endpoint discovery and backend handler binding enrich
// the returned program afterwards (compiler.DiscoverGoEndpoints /
// compiler.BindBackendHandlers).
func BuildProgram(config gowdk.Config, sources Sources) gwdkir.Program {
	builder := irBuilder{
		config:   config,
		program:  gwdkir.Program{Version: gwdkir.Version},
		packages: map[string]*gwdkir.Package{},
	}

	for _, page := range sources.Pages {
		builder.addPage(page)
	}
	for _, component := range sources.Components {
		builder.addComponent(component)
	}
	for _, layout := range sources.Layouts {
		builder.addLayout(layout)
	}
	for _, audit := range sources.AuditSpecs {
		builder.addAuditSpec(audit)
	}

	builder.finishPackages()
	builder.sortOutput()
	return builder.program
}

type irBuilder struct {
	config   gowdk.Config
	program  gwdkir.Program
	packages map[string]*gwdkir.Package
}

const endpointNoStoreCache = "no-store"

func (builder *irBuilder) ensurePackage(name string, src string) *gwdkir.Package {
	pkg := builder.packages[name]
	if pkg == nil {
		pkg = &gwdkir.Package{Name: name}
		builder.packages[name] = pkg
	}
	dir := filepath.Dir(src)
	if src != "" && !contains(pkg.SourceDirs, dir) {
		pkg.SourceDirs = append(pkg.SourceDirs, dir)
	}
	return pkg
}

func (builder *irBuilder) addAuditSpec(audit gwdkir.AuditSpec) {
	builder.program.AuditSpecs = append(builder.program.AuditSpecs, audit)
	pkg := builder.ensurePackage(audit.Package, audit.Source)
	pkg.Files = append(pkg.Files, gwdkir.SourceFile{Path: audit.Source, Kind: gwdkir.SourceAudit, Package: audit.Package, Name: filepath.Base(audit.Source), Span: audit.Span})
}

func (builder *irBuilder) addPage(page gwdkir.Page) {
	// Normalize route params once at program assembly: explicit declarations
	// win, otherwise they are derived from the route pattern, and untyped
	// params default to string.
	page.RouteParams = copyRouteParams(page.TypedRouteParams())
	builder.program.Pages = append(builder.program.Pages, page)
	pkg := builder.ensurePackage(page.Package, page.Source)
	pkg.Files = append(pkg.Files, gwdkir.SourceFile{Path: page.Source, Kind: gwdkir.SourcePage, Package: page.Package, Name: page.ID, Span: page.Spans.Page})
	appendPackageImports(pkg, page.Imports)
	appendPackageUses(pkg, page.Uses)
	appendPackageStores(pkg, page.Stores)

	mode := page.RenderMode(builder.config.Render.DefaultMode())
	builder.program.Routes = append(builder.program.Routes, gwdkir.Route{
		Kind:          routeKind(mode),
		Method:        "GET",
		Path:          page.Route,
		PageID:        page.ID,
		Package:       page.Package,
		Render:        mode,
		Cache:         page.CachePolicy(),
		DynamicParams: page.DynamicParams(),
		RouteParams:   copyRouteParams(page.TypedRouteParams()),
		Layouts:       append([]string(nil), page.Layouts...),
		Guards:        append([]string(nil), page.Guards...),
		Source:        page.Source,
		Span:          page.Spans.Route,
	})
	builder.addPageTemplate(page)
	builder.addPageAssets(page)
	builder.addPageEndpoints(page)
}

func (builder *irBuilder) addPageTemplate(page gwdkir.Page) {
	if !page.Blocks.View {
		return
	}
	template := gwdkir.Template{
		OwnerKind: gwdkir.SourcePage,
		OwnerID:   page.ID,
		Package:   page.Package,
		Source:    page.Source,
		Route:     page.Route,
		Guards:    append([]string(nil), page.Guards...),
		Imports:   append([]gwdkir.Import(nil), page.Imports...),
		Body:      page.Blocks.ViewBody,
		Nodes:     append([]view.Node(nil), page.Blocks.ViewNodes...),
		Span:      page.Blocks.Spans.View,
		BodyStart: page.Blocks.Spans.ViewBodyStart,
	}
	builder.addTemplate(template)
}

func (builder *irBuilder) addPageAssets(page gwdkir.Page) {
	for _, css := range page.CSS {
		name, useAlias, usePackage := assetUse(page.Uses, css)
		builder.program.Assets = append(builder.program.Assets, gwdkir.Asset{
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
	for _, script := range page.JS {
		builder.program.Assets = append(builder.program.Assets, gwdkir.Asset{
			Kind:    gwdkir.AssetJS,
			OwnerID: page.ID,
			Package: page.Package,
			Source:  page.Source,
			Path:    script,
			Span:    spanForName(page.Spans.JS, script, page.Spans.Page),
		})
	}
	for index, script := range page.InlineJS {
		name := script.Name
		if name == "" {
			name = source.InlineScriptName(index)
		}
		builder.program.Assets = append(builder.program.Assets, gwdkir.Asset{
			Kind:    gwdkir.AssetJS,
			OwnerID: page.ID,
			Package: page.Package,
			Source:  page.Source,
			Path:    name,
			Inline:  script.Body,
			Name:    "inline",
			Span:    script.Span,
		})
	}
}

func (builder *irBuilder) addPageEndpoints(page gwdkir.Page) {
	for _, action := range page.Blocks.Actions {
		path := endpointPath(action.Route, page.Route)
		builder.program.Endpoints = append(builder.program.Endpoints, gwdkir.Endpoint{
			Kind:          gwdkir.EndpointAction,
			Source:        gwdkir.EndpointSourceGOWDK,
			Package:       page.Package,
			PageID:        page.ID,
			Symbol:        action.Name,
			Method:        endpointMethod(action.Method, "POST"),
			Path:          path,
			Cache:         endpointNoStoreCache,
			Guards:        append([]string(nil), page.Guards...),
			CSRF:          builder.config.Build.CSRF.EnabledForGeneratedEndpoints(),
			ErrorPage:     action.ErrorPage,
			DynamicParams: routeParams(path),
			RouteParams:   copyRouteParams(gwdkir.RouteParamsFromPath(path)),
			SourceFile:    page.Source,
			Span:          action.Span,
		})
	}
	for _, api := range page.Blocks.APIs {
		path := endpointPath(api.Route, page.Route)
		builder.program.Endpoints = append(builder.program.Endpoints, gwdkir.Endpoint{
			Kind:          gwdkir.EndpointAPI,
			Source:        gwdkir.EndpointSourceGOWDK,
			Package:       page.Package,
			PageID:        page.ID,
			Symbol:        api.Name,
			Method:        endpointMethod(api.Method, "GET"),
			Path:          path,
			Cache:         endpointNoStoreCache,
			Guards:        append([]string(nil), page.Guards...),
			ErrorPage:     api.ErrorPage,
			DynamicParams: routeParams(path),
			RouteParams:   copyRouteParams(gwdkir.RouteParamsFromPath(path)),
			SourceFile:    page.Source,
			Span:          api.Span,
		})
	}
	for _, fragment := range page.Blocks.Fragments {
		builder.program.Endpoints = append(builder.program.Endpoints, gwdkir.Endpoint{
			Kind:          gwdkir.EndpointFragment,
			Source:        gwdkir.EndpointSourceGOWDK,
			Package:       page.Package,
			PageID:        page.ID,
			Symbol:        fragment.Name,
			Method:        endpointMethod(fragment.Method, "GET"),
			Path:          fragment.Route,
			Cache:         endpointNoStoreCache,
			Guards:        append([]string(nil), page.Guards...),
			DynamicParams: routeParams(fragment.Route),
			RouteParams:   copyRouteParams(gwdkir.RouteParamsFromPath(fragment.Route)),
			SourceFile:    page.Source,
			Span:          fragment.Span,
		})
	}
}

func (builder *irBuilder) addComponent(component gwdkir.Component) {
	builder.program.Components = append(builder.program.Components, component)
	pkg := builder.ensurePackage(component.Package, component.Source)
	pkg.Files = append(pkg.Files, gwdkir.SourceFile{Path: component.Source, Kind: gwdkir.SourceComponent, Package: component.Package, Name: component.Name, Span: component.Span})
	appendPackageImports(pkg, component.Imports)
	appendPackageUses(pkg, component.Uses)
	builder.addComponentAssets(component)
	builder.addComponentTemplate(component)
	if component.Blocks.Client {
		builder.program.ClientBehaviors = append(builder.program.ClientBehaviors, gwdkir.ClientBehavior{
			Component: component.Name,
			Package:   component.Package,
			Source:    component.Source,
			Body:      component.Blocks.ClientBody,
			Span:      component.Blocks.Spans.Client,
		})
	}
	if component.WASM.Package != "" {
		builder.program.Assets = append(builder.program.Assets, gwdkir.Asset{
			Kind:    gwdkir.AssetWASM,
			OwnerID: component.Name,
			Package: component.Package,
			Source:  component.Source,
			Path:    component.WASM.Package,
			Span:    component.WASM.Span,
		})
	}
}

func (builder *irBuilder) addComponentAssets(component gwdkir.Component) {
	for _, css := range component.CSS {
		hashKey := cssscope.HashKey("component", component.Package, component.Name, component.Source, css)
		builder.program.Assets = append(builder.program.Assets, gwdkir.Asset{
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
		builder.program.Assets = append(builder.program.Assets, gwdkir.Asset{
			Kind:    gwdkir.AssetFile,
			OwnerID: component.Name,
			Package: component.Package,
			Source:  component.Source,
			Path:    asset,
			Span:    spanForName(component.Spans.Assets, asset, component.Span),
		})
	}
	for _, script := range component.JS {
		builder.program.Assets = append(builder.program.Assets, gwdkir.Asset{
			Kind:    gwdkir.AssetJS,
			OwnerID: component.Name,
			Package: component.Package,
			Source:  component.Source,
			Path:    script,
			Span:    spanForName(component.Spans.JS, script, component.Span),
		})
	}
	for index, script := range component.InlineJS {
		name := script.Name
		if name == "" {
			name = source.InlineScriptName(index)
		}
		builder.program.Assets = append(builder.program.Assets, gwdkir.Asset{
			Kind:    gwdkir.AssetJS,
			OwnerID: component.Name,
			Package: component.Package,
			Source:  component.Source,
			Path:    name,
			Inline:  script.Body,
			Name:    "inline",
			Span:    script.Span,
		})
	}
}

func (builder *irBuilder) addComponentTemplate(component gwdkir.Component) {
	if !component.Blocks.View {
		return
	}
	builder.addTemplate(gwdkir.Template{
		OwnerKind: gwdkir.SourceComponent,
		OwnerID:   component.Name,
		Package:   component.Package,
		Source:    component.Source,
		Imports:   append([]gwdkir.Import(nil), component.Imports...),
		Body:      component.Blocks.ViewBody,
		Nodes:     append([]view.Node(nil), component.Blocks.ViewNodes...),
		Span:      component.Blocks.Spans.View,
		BodyStart: component.Blocks.Spans.ViewBodyStart,
	})
}

func (builder *irBuilder) addLayout(layout gwdkir.Layout) {
	builder.program.Layouts = append(builder.program.Layouts, layout)
	pkg := builder.ensurePackage(layout.Package, layout.Source)
	pkg.Files = append(pkg.Files, gwdkir.SourceFile{Path: layout.Source, Kind: gwdkir.SourceLayout, Package: layout.Package, Name: layout.ID, Span: layout.Span})
	if !layout.Blocks.View {
		return
	}
	builder.addTemplate(gwdkir.Template{
		OwnerKind: gwdkir.SourceLayout,
		OwnerID:   layout.ID,
		Package:   layout.Package,
		Source:    layout.Source,
		Body:      layout.Blocks.ViewBody,
		Nodes:     append([]view.Node(nil), layout.Blocks.ViewNodes...),
		Span:      layout.Blocks.Spans.View,
		BodyStart: layout.Blocks.Spans.ViewBodyStart,
	})
}

func (builder *irBuilder) addStandaloneEndpoint(endpoint gwdkir.GoEndpoint) {
	kind := gwdkir.EndpointAPI
	if endpoint.Kind == "act" || endpoint.Kind == "action" {
		kind = gwdkir.EndpointAction
	}
	defaultMethod := "GET"
	if kind == gwdkir.EndpointAction {
		defaultMethod = "POST"
	}
	builder.program.Endpoints = append(builder.program.Endpoints, gwdkir.Endpoint{
		Kind:          kind,
		Source:        endpoint.SourceKind,
		Package:       endpoint.Package,
		PageID:        standaloneEndpointPageID(endpoint),
		Symbol:        endpoint.Name,
		Method:        endpointMethod(endpoint.Method, defaultMethod),
		Path:          endpoint.Route,
		Cache:         endpointNoStoreCache,
		CSRF:          builder.config.Build.CSRF.EnabledForGeneratedEndpoints() && kind == gwdkir.EndpointAction,
		DynamicParams: routeParams(endpoint.Route),
		RouteParams:   copyRouteParams(gwdkir.RouteParamsFromPath(endpoint.Route)),
		SourceFile:    endpoint.Source,
		Span:          endpoint.Span,
	})
	// Preserve the raw declaration losslessly for validation, which needs the
	// exact kind, method, and spans before normalization.
	builder.program.GoEndpoints = append(builder.program.GoEndpoints, endpoint)
}

// AddStandaloneEndpoints appends discovered standalone Go endpoint declarations
// to an already-built program: each declaration is preserved losslessly in
// Program.GoEndpoints and normalized into Program.Endpoints, which is then
// re-sorted with the same ordering BuildIR produces so post-build discovery
// yields the same program as build-time discovery.
func AddStandaloneEndpoints(config gowdk.Config, program *gwdkir.Program, endpoints []gwdkir.GoEndpoint) {
	if len(endpoints) == 0 {
		return
	}
	builder := irBuilder{config: config, program: *program}
	for _, endpoint := range endpoints {
		builder.addStandaloneEndpoint(endpoint)
	}
	builder.sortOutput()
	*program = builder.program
}

func (builder *irBuilder) addTemplate(template gwdkir.Template) {
	builder.program.Templates = append(builder.program.Templates, template)
	appendContractReferences(&builder.program, template)
}

func (builder *irBuilder) finishPackages() {
	names := make([]string, 0, len(builder.packages))
	for name := range builder.packages {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		pkg := builder.packages[name]
		sort.Strings(pkg.SourceDirs)
		sort.Slice(pkg.Files, func(i, j int) bool { return pkg.Files[i].Path < pkg.Files[j].Path })
		builder.program.Packages = append(builder.program.Packages, *pkg)
	}
}

func (builder *irBuilder) sortOutput() {
	sort.Slice(builder.program.Routes, func(i, j int) bool { return builder.program.Routes[i].Path < builder.program.Routes[j].Path })
	sort.Slice(builder.program.Endpoints, func(i, j int) bool {
		if builder.program.Endpoints[i].Path == builder.program.Endpoints[j].Path {
			return builder.program.Endpoints[i].Method < builder.program.Endpoints[j].Method
		}
		return builder.program.Endpoints[i].Path < builder.program.Endpoints[j].Path
	})
}

func endpointMethod(method string, defaultMethod string) string {
	if method != "" {
		return method
	}
	return defaultMethod
}

func endpointPath(path string, defaultPath string) string {
	if path != "" {
		return path
	}
	return defaultPath
}

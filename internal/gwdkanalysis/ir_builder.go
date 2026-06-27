package gwdkanalysis

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/cssscope"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

// Sources are the parsed IR records one program is assembled from.
type Sources struct {
	Pages      []gwdkir.Page
	Components []gwdkir.Component
	Layouts    []gwdkir.Layout
	AuditSpecs []gwdkir.AuditSpec
}

// BuildProgram assembles the base stable compiler IR from parsed IR records:
// routes, templates, assets, endpoints, and package groupings are derived here.
// Source-taking compiler and codegen callers should use compiler.AssembleProgram
// so standalone Go endpoint discovery and backend handler binding run in the
// canonical order before downstream validation or output planning.
func BuildProgram(config gowdk.Config, sources Sources) gwdkir.Program {
	builder := irBuilder{
		config:   config,
		program:  gwdkir.Program{},
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
	builder.ensureViewNodes("page", page.ID, page.Source, page.Blocks.Spans.View, &page.Blocks)
	builder.ensureServerFields(page.ID, page.Source, &page.Blocks)
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
	route := gwdkir.Route{
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
	}
	route.ID = route.ExpectedID()
	builder.program.Routes = append(builder.program.Routes, route)
	builder.addPageTemplate(page)
	builder.addPageAssets(page)
	builder.addPageEndpoints(page)
}

func (builder *irBuilder) ensureServerFields(id, src string, blocks *gwdkir.Blocks) {
	if !blocks.Server || len(blocks.ServerFields) > 0 || strings.TrimSpace(blocks.ServerBody) == "" {
		return
	}
	fields, err := parseServerFields(blocks.ServerBody)
	if err != nil {
		builder.program.Diagnostics = append(builder.program.Diagnostics, gwdkir.Diagnostic{
			Code:    "view_parse_error",
			Source:  src,
			Span:    blocks.Spans.Server,
			Message: id + ": " + err.Error(),
		})
		return
	}
	blocks.ServerFields = fields
}

func parseServerFields(body string) ([]string, error) {
	var fields []string
	seen := map[string]bool{}
	for index, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		names, ok, err := parseServerLoadFieldsLine(line)
		if err != nil {
			return nil, fmt.Errorf("load line %d: %w", index+1, err)
		}
		if !ok {
			continue
		}
		for _, name := range names {
			if seen[name] {
				return nil, fmt.Errorf("duplicate load field %q", name)
			}
			seen[name] = true
			fields = append(fields, name)
		}
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("server load must include `=> { field }`")
	}
	return fields, nil
}

func parseServerLoadFieldsLine(line string) ([]string, bool, error) {
	body, ok := strings.CutPrefix(strings.TrimSpace(line), "=>")
	if !ok {
		return nil, false, nil
	}
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") {
		return nil, true, fmt.Errorf("load declaration must use `=> { field }`")
	}
	elements, err := splitServerLiteralElements(strings.TrimSpace(body[1 : len(body)-1]))
	if err != nil {
		return nil, true, err
	}
	names := make([]string, 0, len(elements))
	for _, element := range elements {
		name := element
		if colon := indexTopLevelByte(element, ':'); colon >= 0 {
			name = element[:colon]
			value := strings.TrimSpace(element[colon+1:])
			if value == "" {
				return nil, true, fmt.Errorf("keyed load field %q is missing a value", strings.TrimSpace(name))
			}
			if !topLevelExpressionBalanced(value) {
				return nil, true, fmt.Errorf("keyed load field %q has malformed expression", strings.TrimSpace(name))
			}
		}
		name = strings.TrimSpace(name)
		if !serverLoadFieldPath(name) {
			return nil, true, fmt.Errorf("load fields must be identifiers or dotted paths")
		}
		names = append(names, name)
	}
	return names, true, nil
}

func splitServerLiteralElements(inner string) ([]string, error) {
	if strings.TrimSpace(inner) == "" {
		return nil, nil
	}
	parts, err := splitTopLevel(inner, ',')
	if err != nil {
		return nil, err
	}
	if last := len(parts) - 1; last >= 0 && strings.TrimSpace(parts[last]) == "" {
		parts = parts[:last]
	}
	elements := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			return nil, fmt.Errorf("empty load field")
		}
		elements = append(elements, trimmed)
	}
	return elements, nil
}

func indexTopLevelByte(source string, target byte) int {
	scanner := newTopLevelScanner()
	for index := 0; index < len(source); index++ {
		if scanner.step(source[index]) && source[index] == target {
			return index
		}
	}
	return -1
}

func splitTopLevel(source string, sep byte) ([]string, error) {
	var parts []string
	scanner := newTopLevelScanner()
	start := 0
	for index := 0; index < len(source); index++ {
		if scanner.step(source[index]) && source[index] == sep {
			parts = append(parts, source[start:index])
			start = index + 1
		}
	}
	if !scanner.balanced() {
		return nil, fmt.Errorf("malformed load declaration")
	}
	return append(parts, source[start:]), nil
}

func topLevelExpressionBalanced(source string) bool {
	scanner := newTopLevelScanner()
	for index := 0; index < len(source); index++ {
		scanner.step(source[index])
	}
	return scanner.balanced()
}

type topLevelScanner struct {
	depth   int
	quote   byte
	escaped bool
	invalid bool
}

func newTopLevelScanner() *topLevelScanner {
	return &topLevelScanner{}
}

func (scanner *topLevelScanner) step(char byte) bool {
	if scanner.escaped {
		scanner.escaped = false
		return false
	}
	if scanner.quote != 0 {
		if char == '\\' && scanner.quote != '`' {
			scanner.escaped = true
			return false
		}
		if char == scanner.quote {
			scanner.quote = 0
		}
		return false
	}
	switch char {
	case '\'', '"', '`':
		scanner.quote = char
		return false
	case '{', '[', '(':
		scanner.depth++
		return false
	case '}', ']', ')':
		if scanner.depth > 0 {
			scanner.depth--
		} else {
			scanner.invalid = true
		}
		return false
	default:
		return scanner.depth == 0
	}
}

func (scanner *topLevelScanner) balanced() bool {
	return scanner.depth == 0 && scanner.quote == 0 && !scanner.escaped && !scanner.invalid
}

func serverLoadFieldPath(path string) bool {
	for _, part := range strings.Split(path, ".") {
		if !isServerFieldName(part) {
			return false
		}
	}
	return path != ""
}

func isServerFieldName(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
				return false
			}
			continue
		}
		if r != '_' && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
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
		Uses:      append([]gwdkir.Use(nil), page.Uses...),
		Body:      page.Blocks.ViewBody,
		Nodes:     append([]viewmodel.Node(nil), page.Blocks.ViewNodes...),
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
		endpoint := gwdkir.Endpoint{
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
		}
		endpoint.ID = endpoint.ExpectedID()
		builder.program.Endpoints = append(builder.program.Endpoints, endpoint)
	}
	for _, api := range page.Blocks.APIs {
		path := endpointPath(api.Route, page.Route)
		method := endpointMethod(api.Method, "GET")
		endpoint := gwdkir.Endpoint{
			Kind:          gwdkir.EndpointAPI,
			Source:        gwdkir.EndpointSourceGOWDK,
			Package:       page.Package,
			PageID:        page.ID,
			Symbol:        api.Name,
			Method:        method,
			Path:          path,
			Cache:         endpointNoStoreCache,
			Guards:        append([]string(nil), page.Guards...),
			CSRF:          builder.config.Build.CSRF.EnabledForGeneratedEndpoints() && gwdkir.HTTPMethodRequiresCSRF(method),
			ErrorPage:     api.ErrorPage,
			CORS:          cloneEndpointCORS(api.CORS),
			DynamicParams: routeParams(path),
			RouteParams:   copyRouteParams(gwdkir.RouteParamsFromPath(path)),
			SourceFile:    page.Source,
			Span:          api.Span,
		}
		endpoint.ID = endpoint.ExpectedID()
		builder.program.Endpoints = append(builder.program.Endpoints, endpoint)
	}
	for _, fragment := range page.Blocks.Fragments {
		endpoint := gwdkir.Endpoint{
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
		}
		endpoint.ID = endpoint.ExpectedID()
		builder.program.Endpoints = append(builder.program.Endpoints, endpoint)
	}
}

func (builder *irBuilder) addComponent(component gwdkir.Component) {
	builder.ensureViewNodes("component", component.Name, component.Source, component.Blocks.Spans.View, &component.Blocks)
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
		Uses:      append([]gwdkir.Use(nil), component.Uses...),
		Body:      component.Blocks.ViewBody,
		Nodes:     append([]viewmodel.Node(nil), component.Blocks.ViewNodes...),
		Span:      component.Blocks.Spans.View,
		BodyStart: component.Blocks.Spans.ViewBodyStart,
	})
}

func (builder *irBuilder) addLayout(layout gwdkir.Layout) {
	builder.ensureViewNodes("layout", layout.ID, layout.Source, layout.Blocks.Spans.View, &layout.Blocks)
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
		Uses:      append([]gwdkir.Use(nil), layout.Uses...),
		Body:      layout.Blocks.ViewBody,
		Nodes:     append([]viewmodel.Node(nil), layout.Blocks.ViewNodes...),
		Span:      layout.Blocks.Spans.View,
		BodyStart: layout.Blocks.Spans.ViewBodyStart,
	})
}

func (builder *irBuilder) ensureViewNodes(kind, id, src string, span source.SourceSpan, blocks *gwdkir.Blocks) {
	if !blocks.View || len(blocks.ViewNodes) > 0 || strings.TrimSpace(blocks.ViewBody) == "" {
		return
	}
	nodes, err := viewparse.Parse(blocks.ViewBody)
	if err != nil {
		code := "view_parse_error"
		if kind == "component" {
			code = "component_field_error"
		}
		builder.program.Diagnostics = append(builder.program.Diagnostics, gwdkir.Diagnostic{
			Code:    code,
			Source:  src,
			Span:    span,
			Message: id + ": " + err.Error(),
		})
		return
	}
	blocks.ViewNodes = nodes
}

func cloneEndpointCORS(cors gwdkir.EndpointCORS) gwdkir.EndpointCORS {
	cors.AllowedOrigins = append([]string(nil), cors.AllowedOrigins...)
	cors.AllowedMethods = append([]string(nil), cors.AllowedMethods...)
	cors.AllowedHeaders = append([]string(nil), cors.AllowedHeaders...)
	cors.ExposedHeaders = append([]string(nil), cors.ExposedHeaders...)
	return cors
}

func (builder *irBuilder) addStandaloneEndpoint(endpoint gwdkir.StandaloneEndpointDeclaration) {
	kind := gwdkir.EndpointAPI
	if endpoint.Kind == "act" || endpoint.Kind == "action" {
		kind = gwdkir.EndpointAction
	}
	defaultMethod := "GET"
	if kind == gwdkir.EndpointAction {
		defaultMethod = "POST"
	}
	method := endpointMethod(endpoint.Method, defaultMethod)
	normalized := gwdkir.Endpoint{
		Kind:          kind,
		Source:        endpoint.SourceKind,
		Package:       endpoint.Package,
		PageID:        standaloneEndpointPageID(endpoint),
		Symbol:        endpoint.Name,
		Method:        method,
		Path:          endpoint.Route,
		Cache:         endpointNoStoreCache,
		CSRF:          builder.config.Build.CSRF.EnabledForGeneratedEndpoints() && (kind == gwdkir.EndpointAction || kind == gwdkir.EndpointAPI && gwdkir.HTTPMethodRequiresCSRF(method)),
		DynamicParams: routeParams(endpoint.Route),
		RouteParams:   copyRouteParams(gwdkir.RouteParamsFromPath(endpoint.Route)),
		SourceFile:    endpoint.Source,
		Span:          endpoint.Span,
	}
	normalized.ID = normalized.ExpectedID()
	builder.program.Endpoints = append(builder.program.Endpoints, normalized)
	endpoint.ID = normalized.ID
	builder.program.SourceMap.Endpoints = append(builder.program.SourceMap.Endpoints, endpoint)
}

// AddStandaloneEndpoints appends discovered standalone Go endpoint declarations
// to an already-built program: each declaration is normalized into
// Program.Endpoints and preserved losslessly in Program.SourceMap, which is then
// re-sorted with the same ordering BuildIR produces so post-build discovery
// yields the same program as build-time discovery.
func AddStandaloneEndpoints(config gowdk.Config, program *gwdkir.Program, endpoints []gwdkir.StandaloneEndpointDeclaration) {
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
	appendRealtimeSubscriptions(&builder.program, template)
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
	sort.Slice(builder.program.SourceMap.Endpoints, func(i, j int) bool {
		return builder.program.SourceMap.Endpoints[i].ID < builder.program.SourceMap.Endpoints[j].ID
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

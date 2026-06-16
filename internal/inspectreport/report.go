package inspectreport

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

type SourcePosition struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type SourceSpan struct {
	Start SourcePosition `json:"start"`
	End   SourcePosition `json:"end"`
}

type Node struct {
	ID       string         `json:"id"`
	Kind     string         `json:"kind"`
	Name     string         `json:"name,omitempty"`
	Source   string         `json:"source,omitempty"`
	Span     *SourceSpan    `json:"span,omitempty"`
	Props    map[string]any `json:"props,omitempty"`
	Children []Node         `json:"children,omitempty"`
}

type TreeReport struct {
	Version int  `json:"version"`
	Root    Node `json:"root"`
}

type EndpointGraphReport struct {
	Version int         `json:"version"`
	Nodes   []GraphNode `json:"nodes"`
	Edges   []GraphEdge `json:"edges"`
}

type AssetGraphReport struct {
	Version int         `json:"version"`
	Nodes   []GraphNode `json:"nodes"`
	Edges   []GraphEdge `json:"edges"`
}

type GraphNode struct {
	ID     string         `json:"id"`
	Kind   string         `json:"kind"`
	Name   string         `json:"name,omitempty"`
	Source string         `json:"source,omitempty"`
	Span   *SourceSpan    `json:"span,omitempty"`
	Props  map[string]any `json:"props,omitempty"`
}

type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"`
}

func BuildTree(ir gwdkir.Program) TreeReport {
	root := Node{
		ID:   "program",
		Kind: "program",
		Name: "GOWDK Program",
	}
	packages := append([]gwdkir.Package(nil), ir.Packages...)
	sort.Slice(packages, func(i, j int) bool { return packages[i].Name < packages[j].Name })
	for _, pkg := range packages {
		root.Children = append(root.Children, packageNode(pkg, ir))
	}
	return TreeReport{Version: 1, Root: root}
}

func packageNode(pkg gwdkir.Package, ir gwdkir.Program) Node {
	node := Node{
		ID:   nodeID("package", pkg.Name),
		Kind: "package",
		Name: pkg.Name,
		Props: props(
			"sourceDirs", append([]string(nil), pkg.SourceDirs...),
		),
	}
	for _, page := range pagesForPackage(ir.Pages, pkg.Name) {
		node.Children = append(node.Children, pageNode(page, ir))
	}
	for _, component := range componentsForPackage(ir.Components, pkg.Name) {
		node.Children = append(node.Children, componentNode(component, ir))
	}
	for _, layout := range layoutsForPackage(ir.Layouts, pkg.Name) {
		node.Children = append(node.Children, layoutNode(layout, ir))
	}
	for _, endpoint := range standaloneEndpointsForPackage(ir.Endpoints, pkg.Name) {
		node.Children = append(node.Children, endpointNode(endpoint))
	}
	return node
}

func pageNode(page gwdkir.Page, ir gwdkir.Program) Node {
	node := Node{
		ID:     nodeID("page", page.Package, page.ID),
		Kind:   "page",
		Name:   page.ID,
		Source: page.Source,
		Span:   sourceSpan(page.Spans.Page),
		Props: props(
			"package", page.Package,
			"route", page.Route,
			"render", string(page.Render),
			"cache", page.CachePolicy(),
			"layouts", append([]string(nil), page.Layouts...),
			"guards", append([]string(nil), page.Guards...),
		),
	}
	if page.Route != "" {
		node.Children = append(node.Children, Node{
			ID:     nodeID("route", page.Package, page.ID, page.Route),
			Kind:   "route",
			Name:   page.Route,
			Source: page.Source,
			Span:   sourceSpan(page.Spans.Route),
			Props: props(
				"method", "GET",
				"params", routeParamProps(page.TypedRouteParams()),
			),
		})
	}
	for _, endpoint := range endpointsForOwner(ir.Endpoints, page.ID, page.Package) {
		node.Children = append(node.Children, endpointNode(endpoint))
	}
	node.Children = append(node.Children, contractRefNodes(ir.ContractRefs, gwdkir.SourcePage, page.ID)...)
	if template, ok := templateForOwner(ir.Templates, gwdkir.SourcePage, page.ID, page.Package); ok {
		node.Children = append(node.Children, templateNode(template))
	}
	return node
}

func componentNode(component gwdkir.Component, ir gwdkir.Program) Node {
	node := Node{
		ID:     nodeID("component", component.Package, component.Name),
		Kind:   "component",
		Name:   component.Name,
		Source: component.Source,
		Span:   sourceSpan(component.Span),
		Props: props(
			"package", component.Package,
			"props", propNames(component.Props),
			"state", goRefName(component.State.Type),
			"uses", usesProps(component.Uses),
		),
	}
	node.Children = append(node.Children, contractRefNodes(ir.ContractRefs, gwdkir.SourceComponent, component.Name)...)
	if template, ok := templateForOwner(ir.Templates, gwdkir.SourceComponent, component.Name, component.Package); ok {
		node.Children = append(node.Children, templateNode(template))
	}
	return node
}

func layoutNode(layout gwdkir.Layout, ir gwdkir.Program) Node {
	node := Node{
		ID:     nodeID("layout", layout.Package, layout.ID),
		Kind:   "layout",
		Name:   layout.ID,
		Source: layout.Source,
		Span:   sourceSpan(layout.Span),
		Props: props(
			"package", layout.Package,
			"layouts", append([]string(nil), layout.Layouts...),
		),
	}
	node.Children = append(node.Children, contractRefNodes(ir.ContractRefs, gwdkir.SourceLayout, layout.ID)...)
	if template, ok := templateForOwner(ir.Templates, gwdkir.SourceLayout, layout.ID, layout.Package); ok {
		node.Children = append(node.Children, templateNode(template))
	}
	return node
}

func templateNode(template gwdkir.Template) Node {
	node := Node{
		ID:     nodeID("view", string(template.OwnerKind), template.Package, template.OwnerID),
		Kind:   "view",
		Name:   "view",
		Source: template.Source,
		Span:   sourceSpan(template.Span),
		Props: props(
			"ownerKind", string(template.OwnerKind),
			"ownerId", template.OwnerID,
		),
	}
	nodes := template.Nodes
	if len(nodes) == 0 {
		var err error
		nodes, err = viewparse.Parse(template.Body)
		if err != nil {
			node.Children = append(node.Children, Node{
				ID:     nodeID(node.ID, "parse-error"),
				Kind:   "parse-error",
				Name:   err.Error(),
				Source: template.Source,
				Span:   sourceSpan(template.Span),
			})
			return node
		}
	}
	node.Children = append(node.Children, viewNodes(template, nodes, node.ID)...)
	return node
}

func viewNodes(template gwdkir.Template, nodes []viewmodel.Node, parentID string) []Node {
	out := make([]Node, 0, len(nodes))
	for index, raw := range nodes {
		childID := nodeID(parentID, fmt.Sprint(index))
		switch typed := raw.(type) {
		case viewmodel.Element:
			node := Node{
				ID:     childID,
				Kind:   "element",
				Name:   typed.Name,
				Source: template.Source,
				Span:   viewSpan(template, typed.Start, typed.End),
				Props: props(
					"attributes", attrsProps(typed.Attrs),
					"directives", directiveNames(typed.Attrs),
				),
			}
			node.Children = append(node.Children, viewNodes(template, typed.Children, childID)...)
			out = append(out, node)
		case viewmodel.ComponentCall:
			node := Node{
				ID:     childID,
				Kind:   "component-call",
				Name:   typed.Name,
				Source: template.Source,
				Span:   viewSpan(template, typed.Start, typed.End),
				Props: props(
					"attributes", attrsProps(typed.Attrs),
					"directives", directiveNames(typed.Attrs),
				),
			}
			node.Children = append(node.Children, viewNodes(template, typed.Children, childID)...)
			out = append(out, node)
		case viewmodel.Text:
			name := strings.TrimSpace(typed.Value)
			if name == "" {
				continue
			}
			out = append(out, Node{
				ID:     childID,
				Kind:   "text",
				Name:   trimText(name),
				Source: template.Source,
				Span:   viewSpan(template, typed.Start, typed.End),
			})
		}
	}
	return out
}

func endpointNode(endpoint gwdkir.Endpoint) Node {
	return Node{
		ID:     endpointID(endpoint.Kind, endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Path),
		Kind:   "endpoint",
		Name:   endpoint.Symbol,
		Source: endpoint.SourceFile,
		Span:   sourceSpan(endpoint.Span),
		Props: props(
			"kind", string(endpoint.Kind),
			"endpointSource", string(endpoint.Source),
			"package", endpoint.Package,
			"pageId", endpoint.PageID,
			"method", endpoint.Method,
			"route", endpoint.Path,
			"cache", endpoint.Cache,
			"guards", append([]string(nil), endpoint.Guards...),
			"csrf", endpoint.CSRF,
			"bindingStatus", string(endpoint.Binding.Status),
			"signature", string(endpoint.Binding.Signature),
			"inputType", endpoint.Binding.InputType,
		),
	}
}

func contractRefNodes(refs []gwdkir.ContractReference, ownerKind gwdkir.SourceKind, ownerID string) []Node {
	var out []Node
	for _, ref := range refs {
		if ref.OwnerKind != ownerKind || ref.OwnerID != ownerID {
			continue
		}
		out = append(out, Node{
			ID:     contractRefID(ref),
			Kind:   "contract-reference",
			Name:   ref.Name,
			Source: ref.Source,
			Span:   sourceSpan(ref.Span),
			Props: props(
				"kind", string(ref.Kind),
				"method", ref.Method,
				"route", ref.Path,
				"status", string(ref.Status),
				"type", ref.Type,
				"result", ref.Result,
				"roles", append([]string(nil), ref.Roles...),
				"guards", append([]string(nil), ref.Guards...),
			),
		})
	}
	return out
}

func BuildEndpointGraph(config gowdk.Config, ir gwdkir.Program) EndpointGraphReport {
	metadata := compiler.BuildRouteMetadataFromIR(config, ir)
	builder := graphBuilder{
		nodes: map[string]GraphNode{},
		edges: map[string]GraphEdge{},
	}
	for _, route := range metadata.Routes {
		pageID := pageGraphID(route.Package, route.PageID)
		builder.addNode(GraphNode{
			ID:     pageID,
			Kind:   "page",
			Name:   route.PageID,
			Source: route.Source,
			Span:   sourceSpan(route.SourceSpan),
			Props: props(
				"package", route.Package,
				"render", string(route.Render),
				"guards", append([]string(nil), route.Guards...),
			),
		})
		routeID := nodeID("route", route.Package, route.PageID, route.Method, route.Route)
		builder.addNode(GraphNode{
			ID:     routeID,
			Kind:   "route",
			Name:   route.Route,
			Source: route.Source,
			Span:   sourceSpan(route.SourceSpan),
			Props: props(
				"method", route.Method,
				"kind", string(route.Kind),
				"cache", route.Cache,
				"dynamicParams", append([]string(nil), route.DynamicParams...),
			),
		})
		builder.addEdge(pageID, routeID, "declares")
		for _, guard := range route.Guards {
			guardID := guardGraphID(guard)
			builder.addNode(GraphNode{ID: guardID, Kind: "guard", Name: guard})
			builder.addEdge(pageID, guardID, "uses_guard")
		}
	}
	for _, endpoint := range metadata.Endpoints {
		endpointID := endpointBindingID(endpoint)
		builder.addNode(GraphNode{
			ID:     endpointID,
			Kind:   "endpoint",
			Name:   endpoint.Symbol,
			Source: endpoint.Source,
			Span:   sourceSpan(endpoint.SourceSpan),
			Props: props(
				"kind", string(endpoint.Kind),
				"endpointSource", endpoint.EndpointSource,
				"package", endpoint.Package,
				"method", endpoint.Method,
				"route", endpoint.Route,
				"cache", endpoint.Cache,
				"guards", append([]string(nil), endpoint.Guards...),
				"csrf", endpoint.CSRF,
				"bindingStatus", string(endpoint.BindingStatus),
				"signature", string(endpoint.BindingSignature),
				"inputType", endpoint.BindingInputType,
			),
		})
		if endpoint.PageID != "" {
			pageID := pageGraphID(endpoint.Package, endpoint.PageID)
			builder.addNode(GraphNode{ID: pageID, Kind: "page", Name: endpoint.PageID, Source: endpoint.Source})
			builder.addEdge(pageID, endpointID, "owns_endpoint")
		}
		if endpoint.Handler != "" {
			handlerID := nodeID("handler", endpoint.Handler)
			builder.addNode(GraphNode{
				ID:   handlerID,
				Kind: "handler",
				Name: endpoint.Handler,
				Props: props(
					"bindingStatus", string(endpoint.BindingStatus),
					"packageName", endpoint.BindingPackage,
					"importPath", endpoint.BindingImportPath,
					"functionName", endpoint.BindingFunction,
				),
			})
			builder.addEdge(endpointID, handlerID, "handled_by")
		}
		for _, guard := range endpoint.Guards {
			guardID := guardGraphID(guard)
			builder.addNode(GraphNode{ID: guardID, Kind: "guard", Name: guard})
			builder.addEdge(endpointID, guardID, "uses_guard")
		}
		if endpoint.Contract.Name != "" {
			contractID := nodeID("contract", endpoint.Contract.Name)
			builder.addNode(GraphNode{
				ID:   contractID,
				Kind: "contract",
				Name: endpoint.Contract.Name,
				Props: props(
					"kind", string(endpoint.Contract.Kind),
					"status", string(endpoint.Contract.Status),
					"type", endpoint.Contract.Type,
					"result", endpoint.Contract.Result,
					"roles", append([]string(nil), endpoint.Contract.Roles...),
					"handler", endpoint.Contract.Handler,
					"register", endpoint.Contract.Register,
				),
			})
			builder.addEdge(endpointID, contractID, "references_contract")
		}
	}
	return EndpointGraphReport{Version: 1, Nodes: builder.sortedNodes(), Edges: builder.sortedEdges()}
}

func BuildAssetGraph(ir gwdkir.Program) AssetGraphReport {
	builder := graphBuilder{
		nodes: map[string]GraphNode{},
		edges: map[string]GraphEdge{},
	}
	ownerKinds := map[string]gwdkir.SourceKind{}
	fallbackOwnerKinds := map[string]gwdkir.SourceKind{}
	for _, page := range ir.Pages {
		builder.addOwnerNode(gwdkir.SourcePage, page.Package, page.ID, page.Source, page.Spans.Page)
		ownerKinds[assetOwnerKey(page.Package, page.ID, page.Source)] = gwdkir.SourcePage
		fallbackOwnerKinds[assetOwnerFallbackKey(page.Package, page.ID)] = gwdkir.SourcePage
	}
	for _, component := range ir.Components {
		builder.addOwnerNode(gwdkir.SourceComponent, component.Package, component.Name, component.Source, component.Span)
		ownerKinds[assetOwnerKey(component.Package, component.Name, component.Source)] = gwdkir.SourceComponent
		fallbackOwnerKinds[assetOwnerFallbackKey(component.Package, component.Name)] = gwdkir.SourceComponent
	}
	for _, layout := range ir.Layouts {
		builder.addOwnerNode(gwdkir.SourceLayout, layout.Package, layout.ID, layout.Source, layout.Span)
		ownerKinds[assetOwnerKey(layout.Package, layout.ID, layout.Source)] = gwdkir.SourceLayout
		fallbackOwnerKinds[assetOwnerFallbackKey(layout.Package, layout.ID)] = gwdkir.SourceLayout
	}
	for _, template := range ir.Templates {
		templateID := templateAssetGraphID(template.OwnerKind, template.Package, template.OwnerID)
		builder.addNode(GraphNode{
			ID:     templateID,
			Kind:   "template",
			Name:   template.OwnerID,
			Source: template.Source,
			Span:   sourceSpan(template.Span),
			Props: props(
				"ownerKind", string(template.OwnerKind),
				"package", template.Package,
				"route", template.Route,
				"guards", append([]string(nil), template.Guards...),
			),
		})
		builder.addEdge(ownerAssetGraphID(template.OwnerKind, template.Package, template.OwnerID), templateID, "has_template")
	}
	assets := append([]gwdkir.Asset(nil), ir.Assets...)
	sort.Slice(assets, func(i, j int) bool { return assetSortKey(assets[i]) < assetSortKey(assets[j]) })
	for _, asset := range assets {
		sourceKind := assetSourceKind(asset, ownerKinds, fallbackOwnerKinds)
		assetID := assetGraphID(asset, sourceKind)
		builder.addNode(GraphNode{
			ID:     assetID,
			Kind:   "asset",
			Name:   assetGraphName(asset),
			Source: asset.Source,
			Span:   sourceSpan(asset.Span),
			Props: props(
				"kind", string(asset.Kind),
				"package", asset.Package,
				"ownerId", asset.OwnerID,
				"ownerKind", string(sourceKind),
				"path", asset.Path,
				"name", asset.Name,
				"useAlias", asset.UseAlias,
				"usePackage", asset.UsePackage,
				"scopeId", asset.ScopeID,
				"hashKey", asset.HashKey,
				"inline", asset.Inline != "",
			),
		})
		ownerID := ownerAssetGraphID(sourceKind, asset.Package, asset.OwnerID)
		builder.addEdge(ownerID, assetID, "declares_asset")
		if asset.UsePackage != "" {
			packageID := nodeID("package", asset.UsePackage)
			builder.addNode(GraphNode{ID: packageID, Kind: "package", Name: asset.UsePackage})
			builder.addEdge(assetID, packageID, "uses_package")
		}
	}
	return AssetGraphReport{Version: 1, Nodes: builder.sortedNodes(), Edges: builder.sortedEdges()}
}

type graphBuilder struct {
	nodes map[string]GraphNode
	edges map[string]GraphEdge
}

func (builder *graphBuilder) addOwnerNode(kind gwdkir.SourceKind, packageName, ownerID, sourcePath string, span source.SourceSpan) {
	builder.addNode(GraphNode{
		ID:     ownerAssetGraphID(kind, packageName, ownerID),
		Kind:   string(kind),
		Name:   ownerID,
		Source: sourcePath,
		Span:   sourceSpan(span),
		Props:  props("package", packageName),
	})
}

func (builder *graphBuilder) addNode(node GraphNode) {
	if node.ID == "" {
		return
	}
	if previous, ok := builder.nodes[node.ID]; ok {
		builder.nodes[node.ID] = mergeGraphNode(previous, node)
		return
	}
	builder.nodes[node.ID] = node
}

func (builder *graphBuilder) addEdge(from, to, kind string) {
	if from == "" || to == "" || kind == "" {
		return
	}
	key := from + "\x00" + to + "\x00" + kind
	builder.edges[key] = GraphEdge{From: from, To: to, Kind: kind}
}

func (builder graphBuilder) sortedNodes() []GraphNode {
	nodes := make([]GraphNode, 0, len(builder.nodes))
	for _, node := range builder.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes
}

func (builder graphBuilder) sortedEdges() []GraphEdge {
	edges := make([]GraphEdge, 0, len(builder.edges))
	for _, edge := range builder.edges {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		left := edges[i].From + "\x00" + edges[i].To + "\x00" + edges[i].Kind
		right := edges[j].From + "\x00" + edges[j].To + "\x00" + edges[j].Kind
		return left < right
	})
	return edges
}

func mergeGraphNode(left, right GraphNode) GraphNode {
	if left.Kind == "" {
		left.Kind = right.Kind
	}
	if left.Name == "" {
		left.Name = right.Name
	}
	if left.Source == "" {
		left.Source = right.Source
	}
	if left.Span == nil {
		left.Span = right.Span
	}
	if left.Props == nil {
		left.Props = right.Props
	}
	return left
}

func pagesForPackage(pages []gwdkir.Page, packageName string) []gwdkir.Page {
	var out []gwdkir.Page
	for _, page := range pages {
		if page.Package == packageName {
			out = append(out, page)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func componentsForPackage(components []gwdkir.Component, packageName string) []gwdkir.Component {
	var out []gwdkir.Component
	for _, component := range components {
		if component.Package == packageName {
			out = append(out, component)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func layoutsForPackage(layouts []gwdkir.Layout, packageName string) []gwdkir.Layout {
	var out []gwdkir.Layout
	for _, layout := range layouts {
		if layout.Package == packageName {
			out = append(out, layout)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func standaloneEndpointsForPackage(endpoints []gwdkir.Endpoint, packageName string) []gwdkir.Endpoint {
	var out []gwdkir.Endpoint
	for _, endpoint := range endpoints {
		if endpoint.Package == packageName && endpoint.Source == gwdkir.EndpointSourceGo {
			out = append(out, endpoint)
		}
	}
	sort.Slice(out, func(i, j int) bool { return endpointSortKey(out[i]) < endpointSortKey(out[j]) })
	return out
}

func endpointsForOwner(endpoints []gwdkir.Endpoint, pageID string, packageName string) []gwdkir.Endpoint {
	var out []gwdkir.Endpoint
	for _, endpoint := range endpoints {
		if endpoint.PageID == pageID && endpoint.Package == packageName && endpoint.Source != gwdkir.EndpointSourceGo {
			out = append(out, endpoint)
		}
	}
	sort.Slice(out, func(i, j int) bool { return endpointSortKey(out[i]) < endpointSortKey(out[j]) })
	return out
}

func templateForOwner(templates []gwdkir.Template, kind gwdkir.SourceKind, ownerID string, packageName string) (gwdkir.Template, bool) {
	for _, template := range templates {
		if template.OwnerKind == kind && template.OwnerID == ownerID && template.Package == packageName {
			return template, true
		}
	}
	return gwdkir.Template{}, false
}

func endpointSortKey(endpoint gwdkir.Endpoint) string {
	return strings.Join([]string{endpoint.Path, endpoint.Method, string(endpoint.Kind), endpoint.PageID, endpoint.Symbol}, "\x00")
}

func endpointID(kind gwdkir.EndpointKind, pageID, symbol, method, path string) string {
	return nodeID("endpoint", string(kind), pageID, symbol, method, path)
}

func endpointBindingID(endpoint compiler.EndpointBinding) string {
	return nodeID("endpoint", string(endpoint.Kind), endpoint.PageID, endpoint.Symbol, endpoint.Method, endpoint.Route)
}

func contractRefID(ref gwdkir.ContractReference) string {
	return nodeID("contract-ref", string(ref.OwnerKind), ref.OwnerID, string(ref.Kind), ref.Name, ref.Method, ref.Path)
}

func pageGraphID(packageName, pageID string) string {
	return nodeID("page", packageName, pageID)
}

func guardGraphID(name string) string {
	return nodeID("guard", name)
}

func ownerAssetGraphID(kind gwdkir.SourceKind, packageName, ownerID string) string {
	return nodeID(string(kind), packageName, ownerID)
}

func templateAssetGraphID(kind gwdkir.SourceKind, packageName, ownerID string) string {
	return nodeID("template", string(kind), packageName, ownerID)
}

func assetGraphID(asset gwdkir.Asset, sourceKind gwdkir.SourceKind) string {
	return nodeID("asset", string(sourceKind), asset.Package, asset.OwnerID, string(asset.Kind), asset.Path, asset.Name)
}

func assetGraphName(asset gwdkir.Asset) string {
	if asset.Name != "" {
		return asset.Name
	}
	return asset.Path
}

func assetSourceKind(asset gwdkir.Asset, ownerKinds map[string]gwdkir.SourceKind, fallbackOwnerKinds map[string]gwdkir.SourceKind) gwdkir.SourceKind {
	if kind := ownerKinds[assetOwnerKey(asset.Package, asset.OwnerID, asset.Source)]; kind != "" {
		return kind
	}
	if kind := fallbackOwnerKinds[assetOwnerFallbackKey(asset.Package, asset.OwnerID)]; kind != "" {
		return kind
	}
	return gwdkir.SourceComponent
}

func assetOwnerKey(packageName, ownerID, sourcePath string) string {
	return strings.Join([]string{packageName, ownerID, sourcePath}, "\x00")
}

func assetOwnerFallbackKey(packageName, ownerID string) string {
	return strings.Join([]string{packageName, ownerID}, "\x00")
}

func assetSortKey(asset gwdkir.Asset) string {
	return strings.Join([]string{asset.Package, asset.OwnerID, string(asset.Kind), asset.Path, asset.Name}, "\x00")
}

func nodeID(parts ...string) string {
	var tokens []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			tokens = append(tokens, part)
		}
	}
	return strings.Join(tokens, ":")
}

func props(values ...any) map[string]any {
	out := map[string]any{}
	for i := 0; i+1 < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok || key == "" || emptyValue(values[i+1]) {
			continue
		}
		out[key] = values[i+1]
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func emptyValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []string:
		return len(typed) == 0
	case []map[string]string:
		return len(typed) == 0
	case map[string]string:
		return len(typed) == 0
	default:
		return false
	}
}

func sourceSpan(span source.SourceSpan) *SourceSpan {
	if span.Start.Line <= 0 || span.Start.Column <= 0 || span.End.Line <= 0 || span.End.Column <= 0 {
		return nil
	}
	return &SourceSpan{
		Start: SourcePosition{Line: span.Start.Line, Column: span.Start.Column},
		End:   SourcePosition{Line: span.End.Line, Column: span.End.Column},
	}
}

func viewSpan(template gwdkir.Template, start, end int) *SourceSpan {
	if template.BodyStart.Line <= 0 || template.BodyStart.Column <= 0 || start < 0 || end < start {
		return nil
	}
	base := firstSignificantRuneOffset(template.Body)
	adjustedStart := start - base
	adjustedEnd := end - base
	if adjustedStart < 0 {
		adjustedStart = 0
	}
	if adjustedEnd < adjustedStart {
		adjustedEnd = adjustedStart
	}
	return &SourceSpan{
		Start: viewPositionAt(template.Body, template.BodyStart, adjustedStart),
		End:   viewPositionAt(template.Body, template.BodyStart, adjustedEnd),
	}
}

func firstSignificantRuneOffset(body string) int {
	for index, char := range []rune(body) {
		if strings.TrimSpace(string(char)) != "" {
			return index
		}
	}
	return 0
}

func viewPositionAt(body string, start source.SourcePosition, offset int) SourcePosition {
	line, column := start.Line, start.Column
	runes := []rune(body)
	base := firstSignificantRuneOffset(body)
	limit := base + offset
	if limit > len(runes) {
		limit = len(runes)
	}
	for index := base; index < limit; index++ {
		if runes[index] == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return SourcePosition{Line: line, Column: column}
}

func attrsProps(attrs []viewmodel.Attr) map[string]string {
	out := map[string]string{}
	for _, attr := range attrs {
		if strings.HasPrefix(attr.Name, "g:") {
			continue
		}
		if attr.Boolean {
			out[attr.Name] = "true"
			continue
		}
		out[attr.Name] = attr.Value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func directiveNames(attrs []viewmodel.Attr) []string {
	var out []string
	for _, attr := range attrs {
		if strings.HasPrefix(attr.Name, "g:") {
			out = append(out, attr.Name)
		}
	}
	sort.Strings(out)
	return out
}

func routeParamProps(params []source.RouteParam) []map[string]string {
	out := make([]map[string]string, 0, len(params))
	for _, param := range params {
		item := map[string]string{"name": param.Name}
		if param.Type != "" {
			item["type"] = param.Type
		}
		out = append(out, item)
	}
	return out
}

func propNames(props []gwdkir.Prop) []string {
	out := make([]string, 0, len(props))
	for _, prop := range props {
		out = append(out, prop.Name)
	}
	sort.Strings(out)
	return out
}

func usesProps(uses []gwdkir.Use) map[string]string {
	out := map[string]string{}
	for _, use := range uses {
		out[use.Alias] = use.Package
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func goRefName(ref gwdkir.GoRef) string {
	if ref.Alias == "" {
		return ref.Name
	}
	if ref.Name == "" {
		return ref.Alias
	}
	return ref.Alias + "." + ref.Name
}

func trimText(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) <= 48 {
		return value
	}
	runes := []rune(value)
	return string(runes[:45]) + "..."
}

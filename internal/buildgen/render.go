package buildgen

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewanalysis"
	view "github.com/cssbruno/gowdk/internal/viewrender"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

type renderModePolicy string

const (
	renderModeSPA         renderModePolicy = "spa"
	renderModeRequestTime renderModePolicy = "request-time"
)

func renderPage(config gowdk.Config, page gwdkir.Page, route string, components map[string]view.Component, layouts map[string]gwdkir.Layout, stylesheets []gowdk.Stylesheet, actionFields map[string][]view.ActionInputField, data map[string]string, locale string, realtimeEventTypeNames map[string]string, queryTypeNames map[string]string, policy renderModePolicy) (string, ssrRegions, error) {
	mode := page.RenderMode(config.Render.DefaultMode())
	if policy == renderModeSPA && mode != gowdk.SPA {
		return "", ssrRegions{}, fmt.Errorf("%s: SPA build cannot emit request-time %s pages yet", page.ID, mode)
	}
	if policy == renderModeRequestTime && mode != gowdk.SSR && mode != gowdk.Hybrid {
		return "", ssrRegions{}, fmt.Errorf("%s: request-time build cannot emit %s pages", page.ID, mode)
	}
	if !page.Blocks.View {
		return "", ssrRegions{}, fmt.Errorf("%s: missing view {}", page.ID)
	}
	if strings.TrimSpace(page.Blocks.ViewBody) == "" {
		return "", ssrRegions{}, fmt.Errorf("%s: view {} is empty", page.ID)
	}
	viewNodes, err := composePageViewNodes(page, layouts)
	if err != nil {
		return "", ssrRegions{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	if err := validateViewParamReferences(page, viewNodes); err != nil {
		return "", ssrRegions{}, fmt.Errorf("%s: %w", page.ID, err)
	}

	pageComponents := componentRegistryForPage(page, components)
	var lists []view.SSRListReplacement
	var conds []view.SSRCondReplacement
	body, err := renderPageView(viewNodes, pageComponents, data, view.Options{
		Actions:                actionRoutes(page, route, data),
		ActionInputFields:      actionFields,
		Package:                page.Package,
		Tainted:                requestTimeTaintedFields(page, policy),
		RealtimeEventTypeNames: realtimeEventTypeNames,
		QueryTypeNames:         queryTypeNames,
		ServerListSink:         &lists,
		ServerCondSink:         &conds,
	})
	if err != nil {
		return "", ssrRegions{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	storeSeeds, err := pageStoreSeeds(page)
	if err != nil {
		return "", ssrRegions{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	scripts, err := pageScripts(config, page, viewNodes, pageComponents, queryTypeNames, policy)
	if err != nil {
		return "", ssrRegions{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	regions := ssrRegions{Lists: convertSSRListSpecs(lists), Conds: convertSSRCondSpecs(conds)}
	return document(config, page, route, body, stylesheets, storeSeeds, scripts, locale, data), regions, nil
}

// ssrRegions carries the server-rendered g:for lists and g:if conditionals
// collected from a request-time page render.
type ssrRegions struct {
	Lists []source.SSRListSpec
	Conds []source.SSRCondSpec
}

func (r ssrRegions) empty() bool {
	return len(r.Lists) == 0 && len(r.Conds) == 0
}

// convertSSRListSpecs lowers the view layer's collected g:for lists into the
// source representation carried through the app generator to the runtime region
// renderer.
func convertSSRListSpecs(lists []view.SSRListReplacement) []source.SSRListSpec {
	if len(lists) == 0 {
		return nil
	}
	specs := make([]source.SSRListSpec, 0, len(lists))
	for _, list := range lists {
		spec := source.SSRListSpec{
			Placeholder: list.Placeholder,
			SourcePath:  list.SourcePath,
			RowTemplate: list.RowTemplate,
			Fields:      convertSSRListFields(list.Fields),
			Lists:       convertSSRListSpecs(list.Lists),
			Conds:       convertSSRCondSpecs(list.Conds),
		}
		specs = append(specs, spec)
	}
	return specs
}

func convertSSRCondSpecs(conds []view.SSRCondReplacement) []source.SSRCondSpec {
	if len(conds) == 0 {
		return nil
	}
	specs := make([]source.SSRCondSpec, 0, len(conds))
	for _, cond := range conds {
		specs = append(specs, source.SSRCondSpec{
			Placeholder: cond.Placeholder,
			SourcePath:  cond.SourcePath,
			Negate:      cond.Negate,
			Expr:        cond.Expr,
			Template:    cond.Template,
			Fields:      convertSSRListFields(cond.Fields),
			Lists:       convertSSRListSpecs(cond.Lists),
			Conds:       convertSSRCondSpecs(cond.Conds),
		})
	}
	return specs
}

func convertSSRListFields(fields []view.SSRListField) []source.SSRListField {
	if len(fields) == 0 {
		return nil
	}
	out := make([]source.SSRListField, 0, len(fields))
	for _, field := range fields {
		out = append(out, source.SSRListField{
			Placeholder: field.Placeholder,
			Path:        field.Path,
			Index:       field.Index,
			URL:         field.URL,
		})
	}
	return out
}

func renderPageView(nodes []view.Node, components map[string]view.Component, data map[string]string, options view.Options) (string, error) {
	return view.RenderNodesWithOptions(nodes, components, data, options)
}

// requestTimeTaintedFields returns the interpolation names that carry
// request-time, attacker-influenceable data for a page. Currently this is the
// set of SSR load {} field paths, which must be treated like route params:
// rejected in URL/event/style/srcdoc attributes so an attacker-controlled value
// cannot inject a javascript:/data: URL past HTML-text escaping. Build {} data
// is trusted and route params taint syntactically via param("..."), so neither
// is included here.
func requestTimeTaintedFields(page gwdkir.Page, policy renderModePolicy) map[string]bool {
	if policy != renderModeRequestTime || !page.Blocks.Server {
		return nil
	}
	if len(page.Blocks.ServerFields) == 0 {
		return nil
	}
	tainted := make(map[string]bool, len(page.Blocks.ServerFields))
	for _, path := range page.Blocks.ServerFields {
		tainted[path] = true
	}
	return tainted
}

func composePageViewNodes(page gwdkir.Page, layouts map[string]gwdkir.Layout) ([]view.Node, error) {
	nodes := cloneViewNodes(page.Blocks.ViewNodes)
	if len(nodes) == 0 && strings.TrimSpace(page.Blocks.ViewBody) != "" {
		return nil, fmt.Errorf("view {} has source body but no parsed nodes")
	}
	if len(layouts) == 0 {
		return nodes, nil
	}
	for index := len(page.Layouts) - 1; index >= 0; index-- {
		layoutRef := page.Layouts[index]
		layout, ok := resolvePageLayout(page, layouts, layoutRef)
		if !ok {
			return nil, fmt.Errorf("layout %q is not available for app-shell composition", layoutRef)
		}
		next, err := composeLayoutNodesWithParents(layout, nodes, layouts, map[string]bool{})
		if err != nil {
			return nil, err
		}
		nodes = next
	}
	return nodes, nil
}

// composeLayoutWithParents wraps child in layout's slot, then wraps the result
// in layout's own layout parent chain (outermost last). The visiting set
// guards against cyclic inheritance, which validation also rejects.
func composeLayoutNodesWithParents(layout gwdkir.Layout, child []view.Node, layouts map[string]gwdkir.Layout, visiting map[string]bool) ([]view.Node, error) {
	key := layoutRegistryKey(layout.Package, layout.ID)
	if visiting[key] {
		return nil, fmt.Errorf("cyclic layout reference at %q", layout.ID)
	}
	visiting[key] = true
	defer delete(visiting, key)

	nodes, err := composeLayoutNodes(layout, child)
	if err != nil {
		return nil, err
	}
	for index := len(layout.Layouts) - 1; index >= 0; index-- {
		parent, ok := resolveLayoutParent(layout, layouts, layout.Layouts[index])
		if !ok {
			return nil, fmt.Errorf("parent layout %q is not available for app-shell composition", layout.Layouts[index])
		}
		nodes, err = composeLayoutNodesWithParents(parent, nodes, layouts, visiting)
		if err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func resolveLayoutParent(layout gwdkir.Layout, layouts map[string]gwdkir.Layout, ref string) (gwdkir.Layout, bool) {
	if _, _, ok := strings.Cut(ref, "."); ok {
		return gwdkir.Layout{}, false
	}
	if layout.Package != "" {
		if parent, ok := layouts[layoutRegistryKey(layout.Package, ref)]; ok {
			return parent, true
		}
	}
	parent, ok := layouts[layoutRegistryKey("", ref)]
	return parent, ok
}

func resolvePageLayout(page gwdkir.Page, layouts map[string]gwdkir.Layout, layoutRef string) (gwdkir.Layout, bool) {
	if alias, layoutID, ok := strings.Cut(layoutRef, "."); ok {
		for _, use := range page.Uses {
			if use.Alias == alias {
				layout, exists := layouts[layoutRegistryKey(use.Package, layoutID)]
				return layout, exists
			}
		}
		return gwdkir.Layout{}, false
	}
	if page.Package != "" {
		if layout, ok := layouts[layoutRegistryKey(page.Package, layoutRef)]; ok {
			return layout, true
		}
	}
	layout, ok := layouts[layoutRegistryKey("", layoutRef)]
	return layout, ok
}

func composeLayoutNodes(layout gwdkir.Layout, child []view.Node) ([]view.Node, error) {
	if len(layout.Blocks.ViewNodes) == 0 && strings.TrimSpace(layout.Blocks.ViewBody) != "" {
		return nil, fmt.Errorf("layout %s has source body but no parsed nodes", layout.ID)
	}
	nodes, slots := replaceLayoutSlotNodes(layout.Blocks.ViewNodes, child)
	if slots != 1 {
		return nil, fmt.Errorf("layout %s must contain exactly one <slot /> placeholder", layout.ID)
	}
	return nodes, nil
}

func replaceLayoutSlotNodes(nodes []view.Node, child []view.Node) ([]view.Node, int) {
	var out []view.Node
	slots := 0
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.Element:
			if isDefaultLayoutSlot(typed) {
				out = append(out, cloneViewNodes(child)...)
				slots++
				continue
			}
			typed.Children, slots = replaceLayoutSlotChildren(typed.Children, child, slots)
			out = append(out, typed)
		case view.ComponentCall:
			typed.Children, slots = replaceLayoutSlotChildren(typed.Children, child, slots)
			out = append(out, typed)
		case view.AwaitBlock:
			typed.Pending, slots = replaceLayoutSlotChildren(typed.Pending, child, slots)
			typed.Then, slots = replaceLayoutSlotChildren(typed.Then, child, slots)
			typed.Catch, slots = replaceLayoutSlotChildren(typed.Catch, child, slots)
			out = append(out, typed)
		default:
			out = append(out, node)
		}
	}
	return out, slots
}

func replaceLayoutSlotChildren(nodes []view.Node, child []view.Node, slots int) ([]view.Node, int) {
	replaced, count := replaceLayoutSlotNodes(nodes, child)
	return replaced, slots + count
}

func isDefaultLayoutSlot(node view.Element) bool {
	return node.Name == "slot" && len(node.Attrs) == 0 && len(node.Children) == 0
}

func cloneViewNodes(nodes []view.Node) []view.Node {
	if len(nodes) == 0 {
		return nil
	}
	out := make([]view.Node, 0, len(nodes))
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.Element:
			typed.Attrs = append([]view.Attr(nil), typed.Attrs...)
			typed.Children = cloneViewNodes(typed.Children)
			out = append(out, typed)
		case view.ComponentCall:
			typed.Attrs = append([]view.Attr(nil), typed.Attrs...)
			typed.Children = cloneViewNodes(typed.Children)
			out = append(out, typed)
		case view.AwaitBlock:
			typed.Pending = cloneViewNodes(typed.Pending)
			typed.Then = cloneViewNodes(typed.Then)
			typed.Catch = cloneViewNodes(typed.Catch)
			out = append(out, typed)
		default:
			out = append(out, node)
		}
	}
	return out
}

func validateViewParamReferences(page gwdkir.Page, nodes []view.Node) error {
	refs := viewanalysis.ParamReferencesFromNodes(nodes)
	if len(refs) == 0 {
		return nil
	}
	declared := map[string]bool{}
	for _, param := range page.DynamicParams() {
		declared[param] = true
	}
	for _, ref := range refs {
		if !declared[ref] {
			return fmt.Errorf("view references route param %q that is not declared by route %q", ref, page.Route)
		}
	}
	return nil
}

func actionRoutes(page gwdkir.Page, pageRoute string, data map[string]string) map[string]string {
	routes := map[string]string{}
	for _, action := range page.Blocks.Actions {
		route := action.Route
		if route == "" {
			route = pageRoute
		}
		route = expandRouteTemplate(route, data, url.PathEscape)
		routes[action.Name] = route
	}
	return routes
}

func pageScripts(config gowdk.Config, page gwdkir.Page, viewNodes []view.Node, components map[string]view.Component, queryTypeNames map[string]string, policy renderModePolicy) ([]gowdk.Script, error) {
	scripts := append([]gowdk.Script{}, nonEmptyScripts(config.Build.Scripts)...)
	hrefs, err := scopedScriptHrefs(page, "", viewNodes, components)
	if err != nil {
		return nil, err
	}
	for _, href := range hrefs {
		scripts = append(scripts, gowdk.Script{Src: href, Type: "module"})
	}
	usesPartial := pageUsesPartialRuntime(page, viewNodes)
	usesRealtime, err := pageUsesRealtimeRuntime(page, viewNodes, components, queryTypeNames)
	if err != nil {
		return nil, err
	}
	usesCommand, err := pageUsesCommandRuntime(page, viewNodes, components)
	if err != nil {
		return nil, err
	}
	if policy != renderModeSPA {
		// Request-time (SSR/hybrid) pages render live server data but otherwise
		// ship no client. They still need the client runtime when they declare
		// partial-update or realtime query regions: without it a g:query region
		// can never refetch and a g:target fragment form falls back to a full
		// navigation. A g:command write form needs it too, otherwise a bare submit
		// natively navigates to the adapter's raw JSON response. Emit the same
		// small runtime here so request-time pages get progressive enhancement
		// just like SPA pages.
		if usesPartial || usesRealtime || usesCommand {
			scripts = append(scripts, gowdk.Script{Src: clientRuntimeHref})
		}
		return scripts, nil
	}
	usesSPANavigation, err := pageUsesSPANavigationRuntime(config, page, viewNodes, components)
	if err != nil {
		return nil, err
	}
	if usesPartial || usesSPANavigation || usesRealtime || usesCommand {
		scripts = append(scripts, gowdk.Script{Src: clientRuntimeHref})
	}
	if len(page.Stores) > 0 {
		scripts = append(scripts, gowdk.Script{Src: storeRuntimeHref})
	}
	islandScripts, err := islandScriptHrefsForView("", viewNodes, components, page.Package, componentUses(page.Uses))
	if err != nil {
		return nil, err
	}
	for _, href := range islandScripts {
		scripts = append(scripts, gowdk.Script{Src: href})
	}
	for _, href := range clientGoBlockHrefs(page) {
		scripts = append(scripts, gowdk.Script{Src: href})
	}
	return scripts, nil
}

func pageUsesPartialRuntime(page gwdkir.Page, viewNodes []view.Node) bool {
	return len(page.Blocks.Actions) > 0 && nodesHaveAttr(viewNodes, "g:target")
}

func pageUsesRealtimeRuntime(page gwdkir.Page, viewNodes []view.Node, components map[string]view.Component, queryTypeNames map[string]string) (bool, error) {
	if viewHasRealtimeSubscription(viewNodes) {
		return true, nil
	}
	if viewHasInvalidatedQuery(viewNodes, queryTypeNames) {
		return true, nil
	}
	usages, err := recursiveViewComponentCallUsagesForView("", viewNodes, components, page.Package, componentUses(page.Uses))
	if err != nil {
		return false, err
	}
	for _, usage := range usages {
		if viewHasRealtimeSubscription(usage.component.Nodes) {
			return true, nil
		}
		if viewHasInvalidatedQuery(usage.component.Nodes, queryTypeNames) {
			return true, nil
		}
	}
	return false, nil
}

// pageUsesCommandRuntime reports whether the page (or a component it renders)
// declares a g:command write form. Such forms need the client interceptor so a
// submit posts in the background and applies the server's region refresh,
// instead of natively navigating to the adapter's raw JSON response.
func pageUsesCommandRuntime(page gwdkir.Page, viewNodes []view.Node, components map[string]view.Component) (bool, error) {
	if viewHasCommandForm(viewNodes) {
		return true, nil
	}
	usages, err := recursiveViewComponentCallUsagesForView("", viewNodes, components, page.Package, componentUses(page.Uses))
	if err != nil {
		return false, err
	}
	for _, usage := range usages {
		if viewHasCommandForm(usage.component.Nodes) {
			return true, nil
		}
	}
	return false, nil
}

func viewHasCommandForm(nodes []view.Node) bool {
	refs, err := viewanalysis.CommandReferencesFromNodes(nodes)
	return err == nil && len(refs) > 0
}

func viewHasRealtimeSubscription(nodes []view.Node) bool {
	refs, err := viewanalysis.SubscriptionReferencesFromNodes(nodes)
	return err == nil && len(refs) > 0
}

func viewHasInvalidatedQuery(nodes []view.Node, queryTypeNames map[string]string) bool {
	if len(queryTypeNames) == 0 {
		return false
	}
	refs, err := viewanalysis.QueryReferencesFromNodes(nodes)
	if err != nil {
		return false
	}
	for _, ref := range refs {
		if queryTypeNames[ref.Query] != "" {
			return true
		}
	}
	return false
}

func pageUsesSPANavigationRuntime(config gowdk.Config, page gwdkir.Page, viewNodes []view.Node, components map[string]view.Component) (bool, error) {
	mode := page.RenderMode(config.Render.DefaultMode())
	if mode != gowdk.SPA {
		return false, nil
	}
	if nodesHaveInternalLink(viewNodes) {
		return true, nil
	}
	usages, err := recursiveViewComponentCallUsagesForView("", viewNodes, components, page.Package, componentUses(page.Uses))
	if err != nil {
		return false, err
	}
	for _, usage := range usages {
		if nodesHaveInternalLink(usage.component.Nodes) {
			return true, nil
		}
	}
	return false, nil
}

func nodesHaveInternalLink(nodes []view.Node) bool {
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.Element:
			if strings.EqualFold(typed.Name, "a") {
				for _, attr := range typed.Attrs {
					if attr.Name == "href" && isInternalNavigationHref(attr.Value) {
						return true
					}
				}
			}
			if nodesHaveInternalLink(typed.Children) {
				return true
			}
		case view.ComponentCall:
			if nodesHaveInternalLink(typed.Children) {
				return true
			}
		}
	}
	return false
}

func nodesHaveAttr(nodes []view.Node, name string) bool {
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.Element:
			for _, attr := range typed.Attrs {
				if attr.Name == name {
					return true
				}
			}
			if nodesHaveAttr(typed.Children, name) {
				return true
			}
		case view.ComponentCall:
			for _, attr := range typed.Attrs {
				if attr.Name == name {
					return true
				}
			}
			if nodesHaveAttr(typed.Children, name) {
				return true
			}
		case view.AwaitBlock:
			if nodesHaveAttr(typed.Pending, name) || nodesHaveAttr(typed.Then, name) || nodesHaveAttr(typed.Catch, name) {
				return true
			}
		}
	}
	return false
}

func isInternalNavigationHref(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "{}") {
		return false
	}
	return value[0] == '/' && (len(value) <= 1 || (value[1] != '/' && value[1] != '\\'))
}

type pageStoreSeed struct {
	Name    string
	JSON    string
	Persist *storePersistSeed
}

// storePersistSeed is the persistence config carried to the browser store
// registry on the seed <script> tag. Version is a hash of the store's resolved
// struct shape so stale browser storage from an older shape is discarded.
type storePersistSeed struct {
	Scope   string
	Key     string
	Version string
}

func pageStoreSeeds(page gwdkir.Page) ([]pageStoreSeed, error) {
	if len(page.Stores) == 0 {
		return nil, nil
	}
	seeds := make([]pageStoreSeed, 0, len(page.Stores))
	for _, store := range page.Stores {
		payload, err := gotypes.RunStateInitJSON(page.Imports, gwdkir.StateContract{
			Type: store.Type,
			Init: store.Init,
			Span: store.Span,
		})
		if err != nil {
			return nil, fmt.Errorf("store %s init: %w", store.Name, err)
		}
		seed := pageStoreSeed{Name: store.Name, JSON: string(payload)}
		if store.Persist == "local" || store.Persist == "session" {
			resolved, err := gotypes.ResolveStruct(page.Imports, store.Type)
			if err != nil {
				return nil, fmt.Errorf("store %s persist: %w", store.Name, err)
			}
			seed.Persist = &storePersistSeed{
				Scope:   store.Persist,
				Key:     "gowdk:store:" + store.Name,
				Version: storeSchemaHash(resolved, seed.JSON),
			}
		}
		seeds = append(seeds, seed)
	}
	return seeds, nil
}

// storeSchemaHash derives a stable short token from a store's shape. It folds in
// both the resolved Go field names and fully-qualified types (catching field
// add/remove/retype, including nested fields) and the on-wire JSON keys of the
// seed (catching json-tag-only renames that leave the Go field unchanged). The
// token changes whenever the shape changes, which the browser runtime uses to
// discard persisted state written against an older shape.
func storeSchemaHash(resolved gotypes.Struct, seedJSON string) string {
	digest := fnv.New32a()
	typeKeys := make([]string, 0, len(resolved.FieldTypes))
	for name := range resolved.FieldTypes {
		typeKeys = append(typeKeys, name)
	}
	sort.Strings(typeKeys)
	for _, name := range typeKeys {
		digest.Write([]byte(name))
		digest.Write([]byte{0})
		digest.Write([]byte(resolved.FieldTypes[name]))
		digest.Write([]byte{0})
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal([]byte(seedJSON), &wire); err == nil {
		wireKeys := make([]string, 0, len(wire))
		for key := range wire {
			wireKeys = append(wireKeys, key)
		}
		sort.Strings(wireKeys)
		digest.Write([]byte{1})
		for _, key := range wireKeys {
			digest.Write([]byte(key))
			digest.Write([]byte{0})
		}
	}
	return strconv.FormatUint(uint64(digest.Sum32()), 16)
}

func document(config gowdk.Config, page gwdkir.Page, route string, body string, stylesheets []gowdk.Stylesheet, storeSeeds []pageStoreSeed, scripts []gowdk.Script, locale string, data map[string]string) string {
	title := page.ID
	if page.Metadata.Title != "" {
		title = page.Metadata.Title
	}
	image := page.Metadata.Image
	if image == "" {
		image = config.Build.Head.Image
	}
	head := []string{
		"<head>",
		`  <meta charset="utf-8">`,
		`  <meta name="viewport" content="width=device-width, initial-scale=1">`,
		"  <title>" + gowhtml.Escape(title) + "</title>",
	}
	if page.Metadata.Description != "" {
		head = append(head, "  <meta name=\"description\""+gowhtml.Attr("content", page.Metadata.Description)+">")
	}
	if page.Metadata.Canonical != "" {
		head = append(head, "  <link rel=\"canonical\""+gowhtml.Attr("href", page.Metadata.Canonical)+">")
	}
	if robots := robotsContent(page.Metadata); robots != "" {
		head = append(head, "  <meta name=\"robots\""+gowhtml.Attr("content", robots)+">")
	}
	if config.Build.Head.Favicon != "" {
		head = append(head, "  <link rel=\"icon\""+gowhtml.Attr("href", config.Build.Head.Favicon)+">")
	}
	for _, resource := range page.Metadata.Preload {
		head = append(head, headResourceTag("preload", resource))
	}
	for _, resource := range page.Metadata.Prefetch {
		head = append(head, headResourceTag("prefetch", resource))
	}
	if socialHeadEnabled(config.Build.Head, page.Metadata) {
		if config.Build.Head.SiteName != "" {
			head = append(head, "  <meta property=\"og:site_name\""+gowhtml.Attr("content", config.Build.Head.SiteName)+">")
		}
		head = append(head, "  <meta property=\"og:type\" content=\"website\">")
		if page.Metadata.Canonical != "" {
			head = append(head, "  <meta property=\"og:url\""+gowhtml.Attr("content", page.Metadata.Canonical)+">")
		}
		if title != "" {
			head = append(head, "  <meta property=\"og:title\""+gowhtml.Attr("content", title)+">")
		}
		if page.Metadata.Description != "" {
			head = append(head, "  <meta property=\"og:description\""+gowhtml.Attr("content", page.Metadata.Description)+">")
		}
		if image != "" {
			head = append(head, "  <meta property=\"og:image\""+gowhtml.Attr("content", image)+">")
		}
		card := config.Build.Head.TwitterCard
		if card == "" {
			card = "summary"
		}
		head = append(head, "  <meta name=\"twitter:card\""+gowhtml.Attr("content", card)+">")
		if title != "" {
			head = append(head, "  <meta name=\"twitter:title\""+gowhtml.Attr("content", title)+">")
		}
		if page.Metadata.Description != "" {
			head = append(head, "  <meta name=\"twitter:description\""+gowhtml.Attr("content", page.Metadata.Description)+">")
		}
		if image != "" {
			head = append(head, "  <meta name=\"twitter:image\""+gowhtml.Attr("content", image)+">")
		}
	}
	for _, payload := range structuredDataPayloads(page, route, title, image, data) {
		head = append(head, "  <script type=\"application/ld+json\">"+escapeScriptJSON(payload)+"</script>")
	}
	for _, stylesheet := range nonEmptyStylesheets(stylesheets) {
		head = append(head, "  <link rel=\"stylesheet\""+gowhtml.Attr("href", stylesheet.Href)+">")
	}
	for _, seed := range storeSeeds {
		if strings.TrimSpace(seed.Name) == "" {
			continue
		}
		attrs := gowhtml.Attr("data-gowdk-store", seed.Name)
		if seed.Persist != nil {
			attrs += gowhtml.Attr("data-gowdk-persist", seed.Persist.Scope)
			attrs += gowhtml.Attr("data-gowdk-persist-key", seed.Persist.Key)
			attrs += gowhtml.Attr("data-gowdk-persist-version", seed.Persist.Version)
		}
		head = append(head, "  <script type=\"application/json\""+attrs+">"+escapeScriptJSON(seed.JSON)+"</script>")
	}
	for _, script := range nonEmptyScripts(scripts) {
		tag := "  <script"
		if strings.TrimSpace(script.Type) != "" {
			tag += gowhtml.Attr("type", script.Type)
		}
		tag += gowhtml.Attr("src", script.Src)
		// Mark the store runtime so the SPA navigation runtime can run (and
		// hydrate) it before island bundles, which auto-mount on execution and read
		// the store registry during mount.
		if script.Src == storeRuntimeHref {
			tag += " data-gowdk-store-runtime"
		}
		tag += " defer></script>"
		head = append(head, tag)
	}
	head = append(head, "</head>")

	htmlAttrs := ""
	if strings.TrimSpace(locale) != "" {
		htmlAttrs += gowhtml.Attr("lang", strings.TrimSpace(locale))
	}
	if config.HasFeature(gowdk.FeatureObservability) && config.Build.DebugAssets() {
		htmlAttrs += " data-gowdk-trace"
	}
	return "<!doctype html>\n" +
		"<html" + htmlAttrs + ">\n" +
		strings.Join(head, "\n") + "\n" +
		"<body>\n" +
		body + "\n" +
		"</body>\n" +
		"</html>\n"
}

func structuredDataPayloads(page gwdkir.Page, route string, title string, image string, data map[string]string) []string {
	if len(page.Metadata.Structured) == 0 {
		return nil
	}
	payloads := make([]string, 0, len(page.Metadata.Structured))
	for _, item := range page.Metadata.Structured {
		payload, ok := structuredDataPayload(page, route, title, image, data, item.Kind)
		if ok {
			payloads = append(payloads, payload)
		}
	}
	return payloads
}

func structuredDataPayload(page gwdkir.Page, route string, title string, image string, data map[string]string, kind string) (string, bool) {
	node := map[string]any{
		"@context": "https://schema.org",
		"@type":    strings.TrimSpace(kind),
	}
	url := firstNonEmpty(page.Metadata.Canonical, route)
	description := firstDataValue(data, "description", "summary")
	if description == "" {
		description = page.Metadata.Description
	}
	switch strings.TrimSpace(kind) {
	case "WebPage":
		node["name"] = firstDataValue(data, "name", "title")
		if node["name"] == "" {
			node["name"] = title
		}
		if description != "" {
			node["description"] = description
		}
		if url != "" {
			node["url"] = url
		}
		if image != "" {
			node["image"] = image
		}
	case "Article":
		node["headline"] = firstDataValue(data, "headline", "title")
		if node["headline"] == "" {
			node["headline"] = title
		}
		if description != "" {
			node["description"] = description
		}
		if url != "" {
			node["url"] = url
		}
		if image != "" {
			node["image"] = image
		}
		if published := firstDataValue(data, "datePublished", "published", "date"); published != "" {
			node["datePublished"] = published
		}
		if author := firstDataValue(data, "author", "authorName"); author != "" {
			node["author"] = map[string]string{
				"@type": "Person",
				"name":  author,
			}
		}
	default:
		return "", false
	}
	payload, err := json.Marshal(node)
	if err != nil {
		return "", false
	}
	return string(payload), true
}

func firstDataValue(data map[string]string, keys ...string) string {
	for _, key := range keys {
		if value := strings.TrimSpace(data[key]); value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func nonEmptyScripts(scripts []gowdk.Script) []gowdk.Script {
	out := make([]gowdk.Script, 0, len(scripts))
	for _, script := range scripts {
		if strings.TrimSpace(script.Src) == "" {
			continue
		}
		out = append(out, script)
	}
	return out
}

func socialHeadEnabled(head gowdk.HeadConfig, metadata gwdkir.PageMetadata) bool {
	return head.SiteName != "" || head.Image != "" || head.TwitterCard != "" || metadata.Image != ""
}

func robotsContent(metadata gwdkir.PageMetadata) string {
	robots := strings.TrimSpace(metadata.Robots)
	if !metadata.NoIndex {
		return robots
	}
	if robots == "" {
		return "noindex"
	}
	for _, token := range strings.Split(robots, ",") {
		if strings.EqualFold(strings.TrimSpace(token), "noindex") {
			return robots
		}
	}
	return robots + ", noindex"
}

func headResourceTag(rel string, resource gwdkir.HeadResource) string {
	tag := "  <link" + gowhtml.Attr("rel", rel) + gowhtml.Attr("href", resource.Href)
	if resource.As != "" {
		tag += gowhtml.Attr("as", resource.As)
	}
	return tag + ">"
}

func escapeScriptJSON(payload string) string {
	return strings.ReplaceAll(payload, "<", "\\u003c")
}

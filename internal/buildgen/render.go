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
	"github.com/cssbruno/gowdk/internal/view"
	"github.com/cssbruno/gowdk/internal/viewanalysis"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

type renderModePolicy string

const (
	renderModeSPA         renderModePolicy = "spa"
	renderModeRequestTime renderModePolicy = "request-time"
)

func renderPage(config gowdk.Config, page gwdkir.Page, components map[string]view.Component, layouts map[string]gwdkir.Layout, stylesheets []gowdk.Stylesheet, actionFields map[string][]view.ActionInputField, data map[string]string, realtimeEventTypeNames map[string]string, queryTypeNames map[string]string, policy renderModePolicy) (string, error) {
	mode := page.RenderMode(config.Render.DefaultMode())
	if policy == renderModeSPA && mode != gowdk.SPA && mode != gowdk.Action {
		return "", fmt.Errorf("%s: SPA build cannot emit request-time %s pages yet", page.ID, mode)
	}
	if policy == renderModeRequestTime && mode != gowdk.SSR && mode != gowdk.Hybrid {
		return "", fmt.Errorf("%s: request-time build cannot emit %s pages", page.ID, mode)
	}
	if !page.Blocks.View {
		return "", fmt.Errorf("%s: missing view {}", page.ID)
	}
	if strings.TrimSpace(page.Blocks.ViewBody) == "" {
		return "", fmt.Errorf("%s: view {} is empty", page.ID)
	}
	viewSource, err := composePageViewSource(page, layouts)
	if err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}
	viewNodes := composedPageViewNodes(page)
	if err := validateViewParamReferences(page, viewSource, viewNodes); err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}

	pageComponents := componentRegistryForPage(page, components)
	body, err := renderPageView(viewSource, viewNodes, pageComponents, data, view.Options{
		Actions:                actionRoutes(page, data),
		ActionInputFields:      actionFields,
		Package:                page.Package,
		Tainted:                requestTimeTaintedFields(page, policy),
		RealtimeEventTypeNames: realtimeEventTypeNames,
		QueryTypeNames:         queryTypeNames,
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}
	storeSeeds, err := pageStoreSeeds(page)
	if err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}
	scripts, err := pageScripts(config, page, viewSource, viewNodes, pageComponents, queryTypeNames, policy)
	if err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}
	return document(config, page, body, stylesheets, storeSeeds, scripts), nil
}

func composedPageViewNodes(page gwdkir.Page) []view.Node {
	if len(page.Layouts) > 0 || len(page.Blocks.ViewNodes) == 0 {
		return nil
	}
	return page.Blocks.ViewNodes
}

func renderPageView(source string, nodes []view.Node, components map[string]view.Component, data map[string]string, options view.Options) (string, error) {
	if len(nodes) > 0 {
		return view.RenderNodesWithOptions(nodes, components, data, options)
	}
	return view.RenderWithOptions(source, components, data, options)
}

// requestTimeTaintedFields returns the interpolation names that carry
// request-time, attacker-influenceable data for a page. Currently this is the
// set of SSR load {} field paths, which must be treated like route params:
// rejected in URL/event/style/srcdoc attributes so an attacker-controlled value
// cannot inject a javascript:/data: URL past HTML-text escaping. Build {} data
// is trusted and route params taint syntactically via param("..."), so neither
// is included here.
func requestTimeTaintedFields(page gwdkir.Page, policy renderModePolicy) map[string]bool {
	if policy != renderModeRequestTime || !page.Blocks.Load {
		return nil
	}
	fields, err := parseLoadFields(page.Blocks.LoadBody)
	if err != nil {
		return nil
	}
	tainted := make(map[string]bool, len(fields))
	for _, path := range fields {
		tainted[path] = true
	}
	return tainted
}

func composePageViewSource(page gwdkir.Page, layouts map[string]gwdkir.Layout) (string, error) {
	source := page.Blocks.ViewBody
	if len(layouts) == 0 {
		return source, nil
	}
	for index := len(page.Layouts) - 1; index >= 0; index-- {
		layoutRef := page.Layouts[index]
		layout, ok := resolvePageLayout(page, layouts, layoutRef)
		if !ok {
			return "", fmt.Errorf("layout %q is not available for app-shell composition", layoutRef)
		}
		next, err := composeLayoutWithParents(layout, source, layouts, map[string]bool{})
		if err != nil {
			return "", err
		}
		source = next
	}
	return source, nil
}

// composeLayoutWithParents wraps child in layout's slot, then wraps the result
// in layout's own layout parent chain (outermost last). The visiting set
// guards against cyclic inheritance, which validation also rejects.
func composeLayoutWithParents(layout gwdkir.Layout, child string, layouts map[string]gwdkir.Layout, visiting map[string]bool) (string, error) {
	key := layoutRegistryKey(layout.Package, layout.ID)
	if visiting[key] {
		return "", fmt.Errorf("cyclic layout reference at %q", layout.ID)
	}
	visiting[key] = true
	defer delete(visiting, key)

	source, err := composeLayoutSource(layout, child)
	if err != nil {
		return "", err
	}
	for index := len(layout.Layouts) - 1; index >= 0; index-- {
		parent, ok := resolveLayoutParent(layout, layouts, layout.Layouts[index])
		if !ok {
			return "", fmt.Errorf("parent layout %q is not available for app-shell composition", layout.Layouts[index])
		}
		source, err = composeLayoutWithParents(parent, source, layouts, visiting)
		if err != nil {
			return "", err
		}
	}
	return source, nil
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

func composeLayoutSource(layout gwdkir.Layout, child string) (string, error) {
	matches := layoutSlotIndexes(layout.Blocks.ViewBody)
	if len(matches) != 1 {
		return "", fmt.Errorf("layout %s must contain exactly one <slot /> placeholder", layout.ID)
	}
	match := matches[0]
	return layout.Blocks.ViewBody[:match[0]] + child + layout.Blocks.ViewBody[match[1]:], nil
}

func validateViewParamReferences(page gwdkir.Page, source string, nodes []view.Node) error {
	var refs []string
	if len(nodes) > 0 {
		refs = viewanalysis.ParamReferencesFromNodes(nodes)
	} else {
		var err error
		refs, err = viewanalysis.ParamReferences(source)
		if err != nil {
			return err
		}
	}
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

func actionRoutes(page gwdkir.Page, data map[string]string) map[string]string {
	routes := map[string]string{}
	for _, action := range page.Blocks.Actions {
		route := action.Route
		if route == "" {
			route = page.Route
		}
		route = expandRouteTemplate(route, data, url.PathEscape)
		routes[action.Name] = route
	}
	return routes
}

func pageScripts(config gowdk.Config, page gwdkir.Page, viewSource string, viewNodes []view.Node, components map[string]view.Component, queryTypeNames map[string]string, policy renderModePolicy) ([]gowdk.Script, error) {
	scripts := append([]gowdk.Script{}, nonEmptyScripts(config.Build.Scripts)...)
	hrefs, err := scopedScriptHrefs(page, viewSource, viewNodes, components)
	if err != nil {
		return nil, err
	}
	for _, href := range hrefs {
		scripts = append(scripts, gowdk.Script{Src: href, Type: "module"})
	}
	if policy != renderModeSPA {
		return scripts, nil
	}
	usesSPANavigation, err := pageUsesSPANavigationRuntime(config, page, viewSource, viewNodes, components)
	if err != nil {
		return nil, err
	}
	usesRealtime, err := pageUsesRealtimeRuntime(page, viewSource, viewNodes, components, queryTypeNames)
	if err != nil {
		return nil, err
	}
	if pageUsesPartialRuntime(page, viewSource) || usesSPANavigation || usesRealtime {
		scripts = append(scripts, gowdk.Script{Src: clientRuntimeHref})
	}
	if len(page.Stores) > 0 {
		scripts = append(scripts, gowdk.Script{Src: storeRuntimeHref})
	}
	islandScripts, err := islandScriptHrefsForView(viewSource, viewNodes, components, page.Package, componentUses(page.Uses))
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

func pageUsesPartialRuntime(page gwdkir.Page, viewSource string) bool {
	if !strings.Contains(viewSource, "g:target") {
		return false
	}
	return len(page.Blocks.Actions) > 0
}

func pageUsesRealtimeRuntime(page gwdkir.Page, viewSource string, viewNodes []view.Node, components map[string]view.Component, queryTypeNames map[string]string) (bool, error) {
	if viewHasRealtimeSubscription(viewSource, viewNodes) {
		return true, nil
	}
	if viewHasInvalidatedQuery(viewSource, viewNodes, queryTypeNames) {
		return true, nil
	}
	usages, err := recursiveViewComponentCallUsagesForView(viewSource, viewNodes, components, page.Package, componentUses(page.Uses))
	if err != nil {
		return false, err
	}
	for _, usage := range usages {
		if viewHasRealtimeSubscription(usage.component.Body, usage.component.Nodes) {
			return true, nil
		}
		if viewHasInvalidatedQuery(usage.component.Body, usage.component.Nodes, queryTypeNames) {
			return true, nil
		}
	}
	return false, nil
}

func viewHasRealtimeSubscription(source string, nodes []view.Node) bool {
	if !strings.Contains(source, "g:subscribe") && len(nodes) == 0 {
		return false
	}
	if len(nodes) > 0 {
		refs, err := viewanalysis.SubscriptionReferencesFromNodes(nodes)
		return err == nil && len(refs) > 0
	}
	refs, err := viewanalysis.SubscriptionReferences(source)
	return err == nil && len(refs) > 0
}

func viewHasInvalidatedQuery(source string, nodes []view.Node, queryTypeNames map[string]string) bool {
	if len(queryTypeNames) == 0 {
		return false
	}
	if !strings.Contains(source, "g:query") && len(nodes) == 0 {
		return false
	}
	var refs []viewanalysis.QueryReference
	var err error
	if len(nodes) > 0 {
		refs, err = viewanalysis.QueryReferencesFromNodes(nodes)
	} else {
		refs, err = viewanalysis.QueryReferences(source)
	}
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

func pageUsesSPANavigationRuntime(config gowdk.Config, page gwdkir.Page, viewSource string, viewNodes []view.Node, components map[string]view.Component) (bool, error) {
	mode := page.RenderMode(config.Render.DefaultMode())
	if mode != gowdk.SPA && mode != gowdk.Action {
		return false, nil
	}
	if viewHasInternalLink(viewSource, viewNodes) {
		return true, nil
	}
	usages, err := recursiveViewComponentCallUsagesForView(viewSource, viewNodes, components, page.Package, componentUses(page.Uses))
	if err != nil {
		return false, err
	}
	for _, usage := range usages {
		if viewHasInternalLink(usage.component.Body, usage.component.Nodes) {
			return true, nil
		}
	}
	return false, nil
}

func viewHasInternalLink(source string, nodes []view.Node) bool {
	if len(nodes) > 0 {
		return nodesHaveInternalLink(nodes)
	}
	nodes, err := view.Parse(source)
	if err != nil {
		return false
	}
	return nodesHaveInternalLink(nodes)
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

func isInternalNavigationHref(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "{}") {
		return false
	}
	return value[0] == '/' && !(len(value) > 1 && (value[1] == '/' || value[1] == '\\'))
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

func document(config gowdk.Config, page gwdkir.Page, body string, stylesheets []gowdk.Stylesheet, storeSeeds []pageStoreSeed, scripts []gowdk.Script) string {
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
	if config.Build.Head.Favicon != "" {
		head = append(head, "  <link rel=\"icon\""+gowhtml.Attr("href", config.Build.Head.Favicon)+">")
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

func escapeScriptJSON(payload string) string {
	return strings.ReplaceAll(payload, "<", "\\u003c")
}

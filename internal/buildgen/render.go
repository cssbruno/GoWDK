package buildgen

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/view"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

type renderModePolicy string

const (
	renderModeSPA         renderModePolicy = "spa"
	renderModeRequestTime renderModePolicy = "request-time"
)

func renderPage(config gowdk.Config, page gwdkir.Page, components map[string]view.Component, layouts map[string]gwdkir.Layout, stylesheets []gowdk.Stylesheet, data map[string]string, policy renderModePolicy) (string, error) {
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
	if err := validateViewParamReferences(page, viewSource); err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}

	pageComponents := componentRegistryForPage(page, components)
	body, err := view.RenderWithOptions(viewSource, pageComponents, data, view.Options{
		Actions: actionRoutes(page, data),
		Package: page.Package,
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}
	storeSeeds, err := pageStoreSeeds(page)
	if err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}
	return document(config, page, body, stylesheets, storeSeeds, pageScripts(config, page, viewSource, pageComponents, policy)), nil
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
		next, err := composeLayoutSource(layout, source)
		if err != nil {
			return "", err
		}
		source = next
	}
	return source, nil
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

func validateViewParamReferences(page gwdkir.Page, source string) error {
	refs, err := view.ParamReferences(source)
	if err != nil {
		return err
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
		for name, value := range data {
			route = strings.ReplaceAll(route, "{"+name+"}", url.PathEscape(value))
		}
		routes[action.Name] = route
	}
	return routes
}

func pageScripts(config gowdk.Config, page gwdkir.Page, viewSource string, components map[string]view.Component, policy renderModePolicy) []gowdk.Script {
	scripts := append([]gowdk.Script{}, nonEmptyScripts(config.Build.Scripts)...)
	for _, href := range scopedScriptHrefs(page, viewSource, components) {
		scripts = append(scripts, gowdk.Script{Src: href, Type: "module"})
	}
	if policy != renderModeSPA {
		return scripts
	}
	if pageUsesPartialRuntime(page, viewSource) || pageUsesSPANavigationRuntime(config, page, viewSource, components) {
		scripts = append(scripts, gowdk.Script{Src: clientRuntimeHref})
	}
	if len(page.Stores) > 0 {
		scripts = append(scripts, gowdk.Script{Src: storeRuntimeHref})
	}
	for _, href := range islandScriptHrefs(viewSource, components, page.Package, componentUses(page.Uses)) {
		scripts = append(scripts, gowdk.Script{Src: href})
	}
	for _, href := range clientGoBlockHrefs(page) {
		scripts = append(scripts, gowdk.Script{Src: href})
	}
	return scripts
}

func pageUsesPartialRuntime(page gwdkir.Page, viewSource string) bool {
	if !strings.Contains(viewSource, "g:target") {
		return false
	}
	return len(page.Blocks.Actions) > 0
}

func pageUsesSPANavigationRuntime(config gowdk.Config, page gwdkir.Page, viewSource string, components map[string]view.Component) bool {
	mode := page.RenderMode(config.Render.DefaultMode())
	if mode != gowdk.SPA && mode != gowdk.Action {
		return false
	}
	if viewSourceHasInternalLink(viewSource) {
		return true
	}
	usages, err := recursiveViewComponentCallUsages(viewSource, components, page.Package, componentUses(page.Uses))
	if err != nil {
		return false
	}
	for _, usage := range usages {
		if viewSourceHasInternalLink(usage.component.Body) {
			return true
		}
	}
	return false
}

func viewSourceHasInternalLink(source string) bool {
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
	return strings.HasPrefix(value, "/") && !strings.HasPrefix(value, "//")
}

type pageStoreSeed struct {
	Name string
	JSON string
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
		seeds = append(seeds, pageStoreSeed{Name: store.Name, JSON: string(payload)})
	}
	return seeds, nil
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
		head = append(head, "  <script type=\"application/json\""+gowhtml.Attr("data-gowdk-store", seed.Name)+">"+escapeScriptJSON(seed.JSON)+"</script>")
	}
	for _, script := range nonEmptyScripts(scripts) {
		tag := "  <script"
		if strings.TrimSpace(script.Type) != "" {
			tag += gowhtml.Attr("type", script.Type)
		}
		tag += gowhtml.Attr("src", script.Src) + " defer></script>"
		head = append(head, tag)
	}
	head = append(head, "</head>")

	return "<!doctype html>\n" +
		"<html>\n" +
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
	payload = strings.ReplaceAll(payload, "</script", "<\\/script")
	payload = strings.ReplaceAll(payload, "</SCRIPT", "<\\/SCRIPT")
	return payload
}

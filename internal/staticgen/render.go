package staticgen

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

type renderModePolicy string

const (
	renderModeStatic      renderModePolicy = "static"
	renderModeRequestTime renderModePolicy = "request-time"
)

func renderPage(config gowdk.Config, page manifest.Page, components map[string]view.Component, layouts map[string]manifest.Layout, stylesheets []gowdk.Stylesheet, data map[string]string, policy renderModePolicy) (string, error) {
	mode := page.RenderMode(config.Render.DefaultMode())
	if policy == renderModeStatic && mode != gowdk.Static && mode != gowdk.Action {
		return "", fmt.Errorf("%s: static build cannot emit @render %s pages yet", page.ID, mode)
	}
	if policy == renderModeRequestTime && mode != gowdk.SSR && mode != gowdk.Hybrid {
		return "", fmt.Errorf("%s: SSR build cannot emit @render %s pages", page.ID, mode)
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

	body, err := view.RenderWithOptions(viewSource, components, data, view.Options{
		Actions: actionRoutes(page, data),
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}
	return document(page, body, stylesheets, pageScripts(page, viewSource, components, policy)), nil
}

func composePageViewSource(page manifest.Page, layouts map[string]manifest.Layout) (string, error) {
	source := page.Blocks.ViewBody
	if len(layouts) == 0 {
		return source, nil
	}
	for index := len(page.Layouts) - 1; index >= 0; index-- {
		layoutID := page.Layouts[index]
		layout, ok := layouts[layoutID]
		if !ok {
			return "", fmt.Errorf("layout %q is not available for static composition", layoutID)
		}
		next, err := composeLayoutSource(layout, source)
		if err != nil {
			return "", err
		}
		source = next
	}
	return source, nil
}

func composeLayoutSource(layout manifest.Layout, child string) (string, error) {
	matches := layoutSlotPattern.FindAllStringIndex(layout.Blocks.ViewBody, -1)
	if len(matches) != 1 {
		return "", fmt.Errorf("layout %s must contain exactly one <slot /> placeholder", layout.ID)
	}
	match := matches[0]
	return layout.Blocks.ViewBody[:match[0]] + child + layout.Blocks.ViewBody[match[1]:], nil
}

func validateViewParamReferences(page manifest.Page, source string) error {
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

func actionRoutes(page manifest.Page, data map[string]string) map[string]string {
	routes := map[string]string{}
	route := page.Route
	for name, value := range data {
		route = strings.ReplaceAll(route, "{"+name+"}", value)
	}
	for _, action := range page.Blocks.Actions {
		if strings.TrimSpace(action.Redirect) == "" && len(action.Fragments) == 0 {
			continue
		}
		routes[action.Name] = route
	}
	return routes
}

func pageScripts(page manifest.Page, viewSource string, components map[string]view.Component, policy renderModePolicy) []string {
	if policy != renderModeStatic {
		return nil
	}
	var scripts []string
	if pageUsesPartialRuntime(page, viewSource) {
		scripts = append(scripts, clientRuntimeHref)
	}
	scripts = append(scripts, islandScriptHrefs(viewSource, components)...)
	return scripts
}

func pageUsesPartialRuntime(page manifest.Page, viewSource string) bool {
	if !strings.Contains(viewSource, "g:target") {
		return false
	}
	for _, action := range page.Blocks.Actions {
		if len(action.Fragments) > 0 {
			return true
		}
	}
	return false
}

func document(page manifest.Page, body string, stylesheets []gowdk.Stylesheet, scripts []string) string {
	title := page.ID
	var head strings.Builder
	head.WriteString("<head>\n")
	head.WriteString(`  <meta charset="utf-8">` + "\n")
	head.WriteString("  <title>" + gowhtml.Escape(title) + "</title>\n")
	for _, stylesheet := range nonEmptyStylesheets(stylesheets) {
		head.WriteString("  <link rel=\"stylesheet\"" + gowhtml.Attr("href", stylesheet.Href) + ">\n")
	}
	for _, script := range scripts {
		if strings.TrimSpace(script) == "" {
			continue
		}
		head.WriteString("  <script" + gowhtml.Attr("src", script) + " defer></script>\n")
	}
	head.WriteString("</head>\n")

	return "<!doctype html>\n" +
		"<html>\n" +
		head.String() +
		"<body>\n" +
		body + "\n" +
		"</body>\n" +
		"</html>\n"
}

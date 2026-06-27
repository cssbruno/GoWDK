package buildgen

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	view "github.com/cssbruno/gowdk/internal/viewrender"
)

type SSRArtifact struct {
	PageID           string
	Route            string
	Render           gowdk.RenderMode
	Cache            string
	ErrorPage        string
	LayoutErrorPages []LayoutErrorPage
	Locale           string
	DynamicParams    []string
	RouteParams      []source.RouteParam
	Layouts          []string
	Guards           []string
	HasLoad          bool
	LoadBinding      source.BackendBinding
	HTML             string
	Replacements     []SSRReplacement
	LoadReplacements []SSRLoadReplacement
	ListSpecs        []SSRListSpec
	CondSpecs        []SSRCondSpec
	QueryRegions     []SSRQueryRegion
}

type LayoutErrorPage struct {
	Layout    string
	ErrorPage string
}

type SSRReplacement = source.SSRReplacement

type SSRLoadReplacement = source.SSRLoadReplacement

type SSRListSpec = source.SSRListSpec

type SSRCondSpec = source.SSRCondSpec

func SSRArtifacts(config gowdk.Config, sources gwdkanalysis.Sources, outputDir string) ([]SSRArtifact, error) {
	analyzed, err := compiler.AnalyzeProgram(config, sources)
	if err != nil {
		return nil, err
	}
	validated, err := compiler.ValidateAnalyzedProgram(config, analyzed)
	if err != nil {
		return nil, err
	}
	return SSRArtifactsFromValidatedProgram(config, validated, outputDir)
}

// SSRArtifactsFromIR renders request-time page artifacts from normalized
// compiler IR.
func SSRArtifactsFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string) ([]SSRArtifact, error) {
	validated, err := compiler.ValidateIR(config, ir)
	if err != nil {
		return nil, err
	}
	return SSRArtifactsFromValidatedProgram(config, validated, outputDir)
}

// SSRArtifactsFromValidatedProgram renders request-time page artifacts from
// compiler-validated IR.
func SSRArtifactsFromValidatedProgram(config gowdk.Config, validated compiler.ValidatedProgram, outputDir string) ([]SSRArtifact, error) {
	if !validated.Valid() {
		return nil, fmt.Errorf("validated program was not constructed by compiler validation")
	}
	ir := validated.Program()

	components, componentFailures := buildComponents(ir.Components)
	layouts, layoutFailures := buildLayouts(ir.Layouts)
	css, cssFailures := planCSS(config, ir, outputDir, components, layouts)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)
	actionFields := pageActionInputFields(ir)
	realtimeEventTypeNames := realtimeSubscriptionEventTypeNames(ir.RealtimeSubscriptions)
	queryTypeNames := queryInvalidationTypeNames(ir.QueryInvalidations)

	var artifacts []SSRArtifact
	var failures []string
	failures = append(failures, componentFailures...)
	failures = append(failures, layoutFailures...)
	failures = append(failures, cssFailures...)
	for _, page := range ir.Pages {
		if !isRequestTimePage(config, page) {
			continue
		}
		for _, route := range config.I18N.LocalizedRoutes(page.Route) {
			artifact, err := ssrArtifact(config, page, route, components, layouts, append(baseStylesheets, css.pageStylesheets[page.ID]...), actionFields[page.ID], realtimeEventTypeNames, queryTypeNames)
			if err != nil {
				failures = append(failures, err.Error())
				continue
			}
			artifacts = append(artifacts, artifact)
		}
	}
	if len(failures) > 0 {
		return nil, errors.New(strings.Join(failures, "\n"))
	}
	return artifacts, nil
}

func ssrArtifact(config gowdk.Config, page gwdkir.Page, route gowdk.LocalizedRoute, components map[string]view.Component, layouts map[string]gwdkir.Layout, stylesheets []gowdk.Stylesheet, actionFields map[string][]view.ActionInputField, realtimeEventTypeNames map[string]string, queryTypeNames map[string]string) (SSRArtifact, error) {
	render := page.RenderMode(config.Render.DefaultMode())
	routeData, replacements := ssrRouteData(page)
	buildData, err := parseBuildDataFromBlocks(page.Blocks, routeData, route.Locale, page.Imports, page.Source)
	if err != nil {
		return SSRArtifact{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	data, err := mergeBuildData(buildData, routeData)
	if err != nil {
		return SSRArtifact{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	loadData, loadReplacements, err := ssrLoadData(page, data)
	if err != nil {
		return SSRArtifact{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	for key, value := range loadData {
		data[key] = value
	}
	html, regions, err := renderPage(config, page, route.Route, components, layouts, stylesheets, actionFields, data, route.Locale, realtimeEventTypeNames, queryTypeNames, renderModeRequestTime)
	if err != nil {
		return SSRArtifact{}, err
	}
	if !regions.empty() && !page.Blocks.Server {
		return SSRArtifact{}, fmt.Errorf("%s: g:for and g:if over server data require a server {} block", page.ID)
	}
	html, replacements, loadReplacements = markPageURLPlaceholders(html, replacements, loadReplacements)
	regions = markRegionURLPlaceholders(regions)
	// A load field consumed only by g:for/g:if (resolved by the runtime region
	// renderer) leaves no scalar placeholder in the HTML. Drop those request-time
	// replacements so the handler does not stringify and substitute a value that
	// never appears in the output.
	replacements = usedSSRReplacements(html, replacements)
	loadReplacements = usedLoadReplacements(html, loadReplacements)
	queryRegions := ssrQueryRegions(html, regions.Lists, regions.Conds, loadReplacements, replacements, len(page.DynamicParams()) > 0)
	return SSRArtifact{
		PageID:           page.ID,
		Route:            route.Route,
		Render:           render,
		Cache:            page.CachePolicy(),
		ErrorPage:        page.ErrorPage,
		LayoutErrorPages: layoutErrorPagesForPage(page, layouts),
		Locale:           route.Locale,
		DynamicParams:    page.DynamicParams(),
		RouteParams:      append([]source.RouteParam(nil), page.TypedRouteParams()...),
		Layouts:          append([]string(nil), page.Layouts...),
		Guards:           append([]string(nil), page.Guards...),
		HasLoad:          page.Blocks.Server,
		LoadBinding:      sourceBackendBinding(page.LoadBinding),
		HTML:             html,
		Replacements:     replacements,
		LoadReplacements: loadReplacements,
		ListSpecs:        regions.Lists,
		CondSpecs:        regions.Conds,
		QueryRegions:     queryRegions,
	}, nil
}

func layoutErrorPagesForPage(page gwdkir.Page, layouts map[string]gwdkir.Layout) []LayoutErrorPage {
	if len(page.Layouts) == 0 || len(layouts) == 0 {
		return nil
	}
	var boundaries []LayoutErrorPage
	seen := map[string]bool{}
	for index := len(page.Layouts) - 1; index >= 0; index-- {
		layout, ok := resolvePageLayout(page, layouts, page.Layouts[index])
		if !ok {
			continue
		}
		boundaries = appendLayoutErrorPages(boundaries, seen, layout, layouts)
	}
	return boundaries
}

func appendLayoutErrorPages(boundaries []LayoutErrorPage, seen map[string]bool, layout gwdkir.Layout, layouts map[string]gwdkir.Layout) []LayoutErrorPage {
	key := layoutRegistryKey(layout.Package, layout.ID)
	if seen[key] {
		return boundaries
	}
	seen[key] = true
	if layout.ErrorPage != "" {
		boundaries = append(boundaries, LayoutErrorPage{
			Layout:    layoutRegistryDisplayName(layout.Package, layout.ID),
			ErrorPage: layout.ErrorPage,
		})
	}
	for index := len(layout.Layouts) - 1; index >= 0; index-- {
		parent, ok := resolveLayoutParent(layout, layouts, layout.Layouts[index])
		if !ok {
			continue
		}
		boundaries = appendLayoutErrorPages(boundaries, seen, parent, layouts)
	}
	return boundaries
}

// usedLoadReplacements keeps only the scalar load replacements whose placeholder
// still appears in the rendered HTML. A placeholder is absent when its load field
// is consumed solely by g:for, whose rows are expanded by the runtime list
// renderer rather than by request-time string substitution.
func usedLoadReplacements(html string, replacements []SSRLoadReplacement) []SSRLoadReplacement {
	if len(replacements) == 0 {
		return replacements
	}
	used := make([]SSRLoadReplacement, 0, len(replacements))
	for _, replacement := range replacements {
		if strings.Contains(html, replacement.Placeholder) {
			used = append(used, replacement)
		}
	}
	return used
}

func ssrRouteData(page gwdkir.Page) (map[string]string, []SSRReplacement) {
	params := page.DynamicParams()
	if len(params) == 0 {
		return nil, nil
	}
	data := map[string]string{}
	replacements := make([]SSRReplacement, 0, len(params))
	for _, param := range params {
		placeholder := "__GOWDK_SSR_PARAM_" + exportedSafe(page.ID) + "_" + param + "__"
		data[param] = placeholder
		replacements = append(replacements, SSRReplacement{Param: param, Placeholder: placeholder})
	}
	return data, replacements
}

func ssrLoadData(page gwdkir.Page, existing map[string]string) (map[string]string, []SSRLoadReplacement, error) {
	if !page.Blocks.Server {
		return nil, nil, nil
	}
	fields := page.Blocks.ServerFields
	if len(fields) == 0 {
		return nil, nil, fmt.Errorf("server {} must declare at least one field with `=> { field }`")
	}
	data := map[string]string{}
	replacements := make([]SSRLoadReplacement, 0, len(fields))
	for _, path := range fields {
		topLevel, _, _ := strings.Cut(path, ".")
		if _, exists := existing[path]; exists {
			return nil, nil, fmt.Errorf("load field %q conflicts with build data or route params", path)
		}
		if _, exists := existing[topLevel]; exists {
			return nil, nil, fmt.Errorf("load field %q conflicts with build data or route params", path)
		}
		placeholder := "__GOWDK_SSR_LOAD_" + exportedSafe(page.ID) + "_" + exportedSafe(path) + "__"
		data[path] = placeholder
		replacements = append(replacements, SSRLoadReplacement{Path: path, Placeholder: placeholder})
	}
	return data, replacements, nil
}

func exportedSafe(value string) string {
	out := make([]rune, 0, len(value))
	for _, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
		if valid {
			out = append(out, char)
			continue
		}
		out = append(out, '_')
	}
	if len(out) == 0 {
		return "page"
	}
	return string(out)
}

func isRequestTimePage(config gowdk.Config, page gwdkir.Page) bool {
	switch page.RenderMode(config.Render.DefaultMode()) {
	case gowdk.SSR:
		return true
	case gowdk.Hybrid:
		return true
	default:
		return false
	}
}

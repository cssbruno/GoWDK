package buildgen

import (
	"errors"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

type SSRArtifact struct {
	PageID       string
	Route        string
	HTML         string
	Replacements []SSRReplacement
}

type SSRReplacement struct {
	Param       string
	Placeholder string
}

func SSRArtifacts(config gowdk.Config, app manifest.Manifest, outputDir string) ([]SSRArtifact, error) {
	if err := compiler.ValidateManifest(config, app); err != nil {
		return nil, err
	}

	components, componentFailures := buildComponents(app.Components)
	layouts, layoutFailures := buildLayouts(app.Layouts)
	css, cssFailures := planCSS(config, app, outputDir)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)

	var artifacts []SSRArtifact
	var failures []string
	failures = append(failures, componentFailures...)
	failures = append(failures, layoutFailures...)
	failures = append(failures, cssFailures...)
	for _, page := range app.Pages {
		if !isRequestTimePage(config, page) {
			continue
		}
		artifact, err := ssrArtifact(config, page, components, layouts, append(baseStylesheets, css.pageStylesheets[page.ID]...))
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		artifacts = append(artifacts, artifact)
	}
	if len(failures) > 0 {
		return nil, errors.New(strings.Join(failures, "\n"))
	}
	return artifacts, nil
}

func ssrArtifact(config gowdk.Config, page manifest.Page, components map[string]view.Component, layouts map[string]manifest.Layout, stylesheets []gowdk.Stylesheet) (SSRArtifact, error) {
	if page.Blocks.Load {
		return SSRArtifact{}, fmt.Errorf("%s: generated SSR load {} execution is not implemented yet", page.ID)
	}
	routeData, replacements := ssrRouteData(page)
	buildData, err := parseBuildData(page.Blocks.BuildBody, routeData, page.Imports)
	if err != nil {
		return SSRArtifact{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	data, err := mergeBuildData(buildData, routeData)
	if err != nil {
		return SSRArtifact{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	html, err := renderPage(config, page, components, layouts, stylesheets, data, renderModeRequestTime)
	if err != nil {
		return SSRArtifact{}, err
	}
	return SSRArtifact{PageID: page.ID, Route: page.Route, HTML: html, Replacements: replacements}, nil
}

func ssrRouteData(page manifest.Page) (map[string]string, []SSRReplacement) {
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

func exportedSafe(value string) string {
	var builder strings.Builder
	for _, char := range value {
		valid := char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9'
		if valid {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('_')
	}
	if builder.Len() == 0 {
		return "page"
	}
	return builder.String()
}

func isRequestTimePage(config gowdk.Config, page manifest.Page) bool {
	switch page.RenderMode(config.Render.DefaultMode()) {
	case gowdk.SSR, gowdk.Hybrid:
		return true
	default:
		return false
	}
}

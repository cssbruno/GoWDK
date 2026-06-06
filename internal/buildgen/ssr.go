package buildgen

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

type SSRArtifact struct {
	PageID           string
	Route            string
	Render           gowdk.RenderMode
	Cache            string
	DynamicParams    []string
	RouteParams      []manifest.RouteParam
	Guards           []string
	HasLoad          bool
	LoadBinding      manifest.BackendBinding
	HTML             string
	Replacements     []SSRReplacement
	LoadReplacements []SSRLoadReplacement
}

type SSRReplacement struct {
	Param       string
	Placeholder string
}

type SSRLoadReplacement struct {
	Field       string
	Placeholder string
}

func SSRArtifacts(config gowdk.Config, app manifest.Manifest, outputDir string) ([]SSRArtifact, error) {
	return SSRArtifactsFromIR(config, gwdkanalysis.BuildIR(config, app), outputDir)
}

// SSRArtifactsFromIR renders request-time page artifacts from normalized
// compiler IR.
func SSRArtifactsFromIR(config gowdk.Config, ir gwdkir.Program, outputDir string) ([]SSRArtifact, error) {
	app := buildModelFromIR(ir)
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
	routeData, replacements := ssrRouteData(page)
	buildData, err := parseBuildData(page.Blocks.BuildBody, routeData, page.Imports, page.Source)
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
	html, err := renderPage(config, page, components, layouts, stylesheets, data, renderModeRequestTime)
	if err != nil {
		return SSRArtifact{}, err
	}
	return SSRArtifact{
		PageID:           page.ID,
		Route:            page.Route,
		Render:           page.Render,
		Cache:            page.Cache,
		DynamicParams:    page.DynamicParams(),
		RouteParams:      append([]manifest.RouteParam(nil), page.TypedRouteParams()...),
		Guards:           append([]string(nil), page.Guard...),
		HasLoad:          page.Blocks.Load,
		LoadBinding:      page.LoadBinding,
		HTML:             html,
		Replacements:     replacements,
		LoadReplacements: loadReplacements,
	}, nil
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

func ssrLoadData(page manifest.Page, existing map[string]string) (map[string]string, []SSRLoadReplacement, error) {
	if !page.Blocks.Load {
		return nil, nil, nil
	}
	fields, err := parseLoadFields(page.Blocks.LoadBody)
	if err != nil {
		return nil, nil, err
	}
	if len(fields) == 0 {
		return nil, nil, fmt.Errorf("load {} must declare at least one field with `=> { field }`")
	}
	data := map[string]string{}
	replacements := make([]SSRLoadReplacement, 0, len(fields))
	for _, field := range fields {
		if _, exists := existing[field]; exists {
			return nil, nil, fmt.Errorf("load field %q conflicts with build data or route params", field)
		}
		placeholder := "__GOWDK_SSR_LOAD_" + exportedSafe(page.ID) + "_" + field + "__"
		data[field] = placeholder
		replacements = append(replacements, SSRLoadReplacement{Field: field, Placeholder: placeholder})
	}
	return data, replacements, nil
}

func parseLoadFields(body string) ([]string, error) {
	lines := significantBuildLines(body)
	var fields []string
	seen := map[string]bool{}
	for index, line := range lines {
		literal, ok, err := parseLoadLiteralLine(line)
		if err != nil {
			return nil, fmt.Errorf("load line %d: %w", index+1, err)
		}
		if !ok {
			return nil, fmt.Errorf("load line %d must use `=> { field }`", index+1)
		}
		for _, element := range literal.Elts {
			name, ok := loadFieldName(element)
			if !ok {
				return nil, fmt.Errorf("load line %d: load fields must be bare identifiers", index+1)
			}
			if seen[name] {
				return nil, fmt.Errorf("duplicate load field %q", name)
			}
			seen[name] = true
			fields = append(fields, name)
		}
	}
	return fields, nil
}

func parseLoadLiteralLine(line string) (*ast.CompositeLit, bool, error) {
	body, ok := strings.CutPrefix(strings.TrimSpace(line), "=>")
	if !ok {
		return nil, false, nil
	}
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") {
		return nil, false, nil
	}
	expr, err := parser.ParseExpr("[]string" + body)
	if err != nil {
		return nil, true, fmt.Errorf("parse load literal: %w", err)
	}
	literal, ok := expr.(*ast.CompositeLit)
	if !ok {
		return nil, true, fmt.Errorf("load literal must be an object")
	}
	return literal, true, nil
}

func loadFieldName(expression ast.Expr) (string, bool) {
	identifier, ok := expression.(*ast.Ident)
	if !ok || !literalNamePattern.MatchString(identifier.Name) {
		return "", false
	}
	return identifier.Name, true
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
	case gowdk.SSR:
		return true
	case gowdk.Hybrid:
		return page.Blocks.Load
	default:
		return false
	}
}

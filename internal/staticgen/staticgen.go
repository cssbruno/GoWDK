// Package staticgen emits static HTML artifacts for build-time pages.
package staticgen

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/internal/compiler"
	"github.com/gowdk/gowdk/internal/manifest"
	"github.com/gowdk/gowdk/internal/view"
	runtimeasset "github.com/gowdk/gowdk/runtime/asset"
	gowhtml "github.com/gowdk/gowdk/runtime/html"
)

const routeManifestFile = "gowdk-routes.json"
const assetManifestFile = "gowdk-assets.json"

var (
	literalDeclarationPattern = regexp.MustCompile(`^=>\s*\{(.*)\}$`)
	literalNamePattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// Artifact describes one emitted file.
type Artifact struct {
	PageID string
	Route  string
	Path   string
}

// CSSArtifact describes one emitted CSS file.
type CSSArtifact struct {
	Path string
}

// Result describes a static build.
type Result struct {
	Artifacts         []Artifact
	CSSArtifacts      []CSSArtifact
	RouteManifestPath string
	AssetManifestPath string
}

type plannedArtifact struct {
	Artifact
	contents []byte
}

type plannedCSSArtifact struct {
	CSSArtifact
	contents []byte
}

type buildPlan struct {
	pages []plannedArtifact
	css   []plannedCSSArtifact
}

// Build emits static HTML files into outputDir.
func Build(config gowdk.Config, app manifest.Manifest, outputDir string) (Result, error) {
	if strings.TrimSpace(outputDir) == "" {
		return Result{}, fmt.Errorf("build output directory is required")
	}
	if err := compiler.ValidateManifest(config, app); err != nil {
		return Result{}, err
	}

	planned, err := plan(config, app, outputDir)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Artifacts:    make([]Artifact, 0, len(planned.pages)),
		CSSArtifacts: make([]CSSArtifact, 0, len(planned.css)),
	}
	for _, artifact := range planned.css {
		if err := os.MkdirAll(filepath.Dir(artifact.Path), 0o755); err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(artifact.Path, artifact.contents, 0o644); err != nil {
			return Result{}, err
		}
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
	}
	for _, artifact := range planned.pages {
		if err := os.MkdirAll(filepath.Dir(artifact.Path), 0o755); err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(artifact.Path, artifact.contents, 0o644); err != nil {
			return Result{}, err
		}
		result.Artifacts = append(result.Artifacts, artifact.Artifact)
	}
	manifestPath, err := writeRouteManifest(outputDir, result.Artifacts)
	if err != nil {
		return Result{}, err
	}
	result.RouteManifestPath = manifestPath
	assetManifestPath, err := writeAssetManifest(outputDir, result.CSSArtifacts)
	if err != nil {
		return Result{}, err
	}
	result.AssetManifestPath = assetManifestPath
	return result, nil
}

func plan(config gowdk.Config, app manifest.Manifest, outputDir string) (buildPlan, error) {
	components, componentFailures := buildComponents(app.Components)
	css, cssFailures := planCSS(config, app, outputDir)
	stylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	stylesheets = append(stylesheets, css.stylesheets...)
	var planned []plannedArtifact
	var failures []string
	seenOutputPaths := map[string]string{}
	failures = append(failures, componentFailures...)
	failures = append(failures, cssFailures...)
	for _, page := range app.Pages {
		pageArtifacts, err := pageOutputArtifacts(config, outputDir, page, components, stylesheets)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, artifact := range pageArtifacts {
			rel, err := relativeOutputPath(outputDir, artifact.Path)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: %v", page.ID, err))
				continue
			}
			if previousPage, ok := seenOutputPaths[artifact.Path]; ok {
				failures = append(failures, fmt.Sprintf("%s: generated output path %q duplicates page %s", page.ID, rel, previousPage))
				continue
			}
			seenOutputPaths[artifact.Path] = page.ID
			planned = append(planned, artifact)
		}
	}
	if len(failures) > 0 {
		return buildPlan{}, errors.New(strings.Join(failures, "\n"))
	}
	return buildPlan{pages: planned, css: css.assets}, nil
}

type pageOutput struct {
	route string
	data  map[string]string
}

func pageOutputArtifacts(config gowdk.Config, outputDir string, page manifest.Page, components map[string]view.Component, stylesheets []gowdk.Stylesheet) ([]plannedArtifact, error) {
	buildData, err := parseBuildData(page.Blocks.BuildBody)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", page.ID, err)
	}
	outputs, err := pageOutputs(page)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", page.ID, err)
	}
	artifacts := make([]plannedArtifact, 0, len(outputs))
	for _, output := range outputs {
		data, err := mergeBuildData(buildData, output.data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		html, err := renderPage(config, page, components, stylesheets, data)
		if err != nil {
			return nil, err
		}
		outputPath, err := outputPath(outputDir, output.route)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		artifacts = append(artifacts, plannedArtifact{
			Artifact: Artifact{PageID: page.ID, Route: output.route, Path: outputPath},
			contents: []byte(html),
		})
	}
	return artifacts, nil
}

func pageOutputs(page manifest.Page) ([]pageOutput, error) {
	params := page.DynamicParams()
	if len(params) == 0 {
		return []pageOutput{{route: page.Route}}, nil
	}

	declarations, err := parsePathDeclarations(page.Blocks.PathsBody)
	if err != nil {
		return nil, err
	}
	if len(declarations) == 0 {
		return nil, fmt.Errorf("paths {} must declare at least one path")
	}

	required := map[string]bool{}
	for _, param := range params {
		required[param] = true
	}

	outputs := make([]pageOutput, 0, len(declarations))
	for index, declaration := range declarations {
		for _, param := range params {
			value, ok := declaration[param]
			if !ok {
				return nil, fmt.Errorf("paths declaration %d missing route param %q", index+1, param)
			}
			if err := validateRouteParamValue(param, value); err != nil {
				return nil, fmt.Errorf("paths declaration %d: %w", index+1, err)
			}
		}
		for name := range declaration {
			if !required[name] {
				return nil, fmt.Errorf("paths declaration %d declares unused route param %q", index+1, name)
			}
		}

		route := page.Route
		for name, value := range declaration {
			route = strings.ReplaceAll(route, "{"+name+"}", value)
		}
		outputs = append(outputs, pageOutput{
			route: route,
			data:  cloneStringMap(declaration),
		})
	}
	return outputs, nil
}

func parsePathDeclarations(body string) ([]map[string]string, error) {
	return parseLiteralDeclarations(body, "paths", "path param")
}

func parsePathParams(source string) (map[string]string, error) {
	return parseLiteralStringMap(source, "path param")
}

func parseBuildData(body string) (map[string]string, error) {
	declarations, err := parseLiteralDeclarations(body, "build", "build field")
	if err != nil {
		return nil, err
	}
	if len(declarations) > 1 {
		return nil, fmt.Errorf("build {} supports one literal data declaration")
	}
	if len(declarations) == 0 {
		return nil, nil
	}
	return declarations[0], nil
}

func parseLiteralDeclarations(body, blockName, itemName string) ([]map[string]string, error) {
	var declarations []map[string]string
	for index, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		match := literalDeclarationPattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("%s line %d must use `=> { name: \"value\" }`", blockName, index+1)
		}
		params, err := parseLiteralStringMap(match[1], itemName)
		if err != nil {
			return nil, fmt.Errorf("%s line %d: %w", blockName, index+1, err)
		}
		declarations = append(declarations, params)
	}
	return declarations, nil
}

func parseLiteralStringMap(source, itemName string) (map[string]string, error) {
	assignments, err := splitPathAssignments(source)
	if err != nil {
		return nil, err
	}
	if len(assignments) == 0 {
		return nil, fmt.Errorf("literal declaration must include values")
	}

	params := map[string]string{}
	for _, assignment := range assignments {
		name, rawValue, ok := strings.Cut(assignment, ":")
		if !ok {
			return nil, fmt.Errorf("%s %q must use name: \"value\"", itemName, strings.TrimSpace(assignment))
		}
		name = strings.TrimSpace(name)
		if !literalNamePattern.MatchString(name) {
			return nil, fmt.Errorf("invalid %s name %q", itemName, name)
		}
		if _, exists := params[name]; exists {
			return nil, fmt.Errorf("duplicate %s %q", itemName, name)
		}
		value, err := parsePathString(strings.TrimSpace(rawValue))
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", itemName, name, err)
		}
		params[name] = value
	}
	return params, nil
}

func splitPathAssignments(source string) ([]string, error) {
	var assignments []string
	start := 0
	inString := false
	escaped := false
	for index, char := range source {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case ',':
			part := strings.TrimSpace(source[start:index])
			if part == "" {
				return nil, fmt.Errorf("empty path param assignment")
			}
			assignments = append(assignments, part)
			start = index + 1
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string")
	}
	part := strings.TrimSpace(source[start:])
	if part != "" {
		assignments = append(assignments, part)
	}
	return assignments, nil
}

func parsePathString(source string) (string, error) {
	if !strings.HasPrefix(source, `"`) {
		return "", fmt.Errorf("value must be a string literal")
	}
	value, err := strconv.Unquote(source)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("value must not be empty")
	}
	return value, nil
}

func validateRouteParamValue(name, value string) error {
	if strings.ContainsAny(value, "/?#") {
		return fmt.Errorf("route param %q value %q must not contain /, ?, or #", name, value)
	}
	if value == "." || value == ".." {
		return fmt.Errorf("route param %q value %q is unsafe", name, value)
	}
	return nil
}

func mergeBuildData(buildData, routeData map[string]string) (map[string]string, error) {
	merged := cloneStringMap(buildData)
	for key, value := range routeData {
		if _, exists := merged[key]; exists {
			return nil, fmt.Errorf("build data field %q conflicts with route param", key)
		}
		merged[key] = value
	}
	return merged, nil
}

func cloneStringMap(input map[string]string) map[string]string {
	output := map[string]string{}
	for key, value := range input {
		output[key] = value
	}
	return output
}

type cssPlan struct {
	assets      []plannedCSSArtifact
	stylesheets []gowdk.Stylesheet
}

func planCSS(config gowdk.Config, app manifest.Manifest, outputDir string) (cssPlan, []string) {
	var planned cssPlan
	var failures []string
	seen := map[string]bool{}
	context := gowdk.CSSContext{
		Sources:   cssSources(app),
		OutputDir: outputDir,
	}
	for _, addon := range config.Addons {
		processor, ok := addon.(gowdk.CSSProcessor)
		if !ok {
			continue
		}
		result, err := processor.ProcessCSS(context)
		if err != nil {
			failures = append(failures, fmt.Sprintf("css processor %s failed: %v", processor.Name(), err))
			continue
		}
		planned.stylesheets = append(planned.stylesheets, nonEmptyStylesheets(result.Stylesheets)...)
		for _, asset := range result.Assets {
			outputPath, err := cssOutputPath(outputDir, asset.Path)
			if err != nil {
				failures = append(failures, fmt.Sprintf("css processor %s: %v", processor.Name(), err))
				continue
			}
			if seen[outputPath] {
				failures = append(failures, fmt.Sprintf("css processor %s: duplicate css asset path %q", processor.Name(), asset.Path))
				continue
			}
			seen[outputPath] = true
			planned.assets = append(planned.assets, plannedCSSArtifact{
				CSSArtifact: CSSArtifact{Path: outputPath},
				contents:    append([]byte(nil), asset.Contents...),
			})
		}
	}
	return planned, failures
}

func cssSources(app manifest.Manifest) []gowdk.CSSSource {
	sources := make([]gowdk.CSSSource, 0, len(app.Pages)+len(app.Components))
	for _, page := range app.Pages {
		sources = append(sources, gowdk.CSSSource{
			Path: page.Source,
			Kind: "page",
			Name: page.ID,
		})
	}
	for _, component := range app.Components {
		sources = append(sources, gowdk.CSSSource{
			Path: component.Source,
			Kind: "component",
			Name: component.Name,
		})
	}
	return sources
}

func nonEmptyStylesheets(stylesheets []gowdk.Stylesheet) []gowdk.Stylesheet {
	out := make([]gowdk.Stylesheet, 0, len(stylesheets))
	for _, stylesheet := range stylesheets {
		if strings.TrimSpace(stylesheet.Href) == "" {
			continue
		}
		out = append(out, stylesheet)
	}
	return out
}

func cssOutputPath(outputDir, name string) (string, error) {
	if strings.TrimSpace(name) == "" {
		return "", fmt.Errorf("css asset path is required")
	}
	if filepath.IsAbs(name) {
		return "", fmt.Errorf("css asset path %q must be relative", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("css asset path %q must stay inside output directory", name)
	}
	return filepath.Join(outputDir, clean), nil
}

type routeManifest struct {
	Version int                  `json:"version"`
	Routes  []routeManifestEntry `json:"routes"`
}

type routeManifestEntry struct {
	PageID string `json:"page"`
	Route  string `json:"route"`
	Path   string `json:"path"`
}

func writeRouteManifest(outputDir string, artifacts []Artifact) (string, error) {
	routes := make([]routeManifestEntry, 0, len(artifacts))
	for _, artifact := range artifacts {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return "", err
		}
		routes = append(routes, routeManifestEntry{
			PageID: artifact.PageID,
			Route:  artifact.Route,
			Path:   rel,
		})
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Route == routes[j].Route {
			return routes[i].PageID < routes[j].PageID
		}
		return routes[i].Route < routes[j].Route
	})

	payload, err := json.MarshalIndent(routeManifest{Version: 1, Routes: routes}, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')

	manifestPath := filepath.Join(outputDir, routeManifestFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(manifestPath, payload, 0o644); err != nil {
		return "", err
	}
	return manifestPath, nil
}

func writeAssetManifest(outputDir string, cssArtifacts []CSSArtifact) (string, error) {
	files := make(map[string]string, len(cssArtifacts))
	for _, artifact := range cssArtifacts {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return "", err
		}
		files[rel] = rel
	}

	payload, err := json.MarshalIndent(runtimeasset.Manifest{Version: 1, Files: files}, "", "  ")
	if err != nil {
		return "", err
	}
	payload = append(payload, '\n')

	manifestPath := filepath.Join(outputDir, assetManifestFile)
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(manifestPath, payload, 0o644); err != nil {
		return "", err
	}
	return manifestPath, nil
}

func relativeOutputPath(outputDir, filePath string) (string, error) {
	rel, err := filepath.Rel(outputDir, filePath)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("artifact path %q must stay inside output directory", filePath)
	}
	return filepath.ToSlash(rel), nil
}

func buildComponents(components []manifest.Component) (map[string]view.Component, []string) {
	registry := map[string]view.Component{}
	var failures []string
	for _, component := range components {
		valid := true
		if component.Name == "" {
			failures = append(failures, "component missing name")
			continue
		}
		if _, exists := registry[component.Name]; exists {
			failures = append(failures, fmt.Sprintf("duplicate component %q", component.Name))
			continue
		}
		if !isComponentName(component.Name) {
			failures = append(failures, fmt.Sprintf("component %q must start with an uppercase letter", component.Name))
			continue
		}
		if !component.Blocks.View {
			failures = append(failures, fmt.Sprintf("component %s missing view {}", component.Name))
			continue
		}
		if strings.TrimSpace(component.Blocks.ViewBody) == "" {
			failures = append(failures, fmt.Sprintf("component %s view {} is empty", component.Name))
			continue
		}

		props := make([]string, 0, len(component.Props))
		seen := map[string]bool{}
		for _, prop := range component.Props {
			if prop.Type != "string" {
				failures = append(failures, fmt.Sprintf("component %s prop %s uses unsupported type %q", component.Name, prop.Name, prop.Type))
				valid = false
				continue
			}
			if seen[prop.Name] {
				failures = append(failures, fmt.Sprintf("component %s declares duplicate prop %q", component.Name, prop.Name))
				valid = false
				continue
			}
			seen[prop.Name] = true
			props = append(props, prop.Name)
		}
		if !valid {
			continue
		}
		registry[component.Name] = view.Component{
			Name:  component.Name,
			Props: props,
			Body:  component.Blocks.ViewBody,
		}
	}
	return registry, failures
}

func renderPage(config gowdk.Config, page manifest.Page, components map[string]view.Component, stylesheets []gowdk.Stylesheet, data map[string]string) (string, error) {
	mode := page.RenderMode(config.Render.DefaultMode())
	if mode != gowdk.Static && mode != gowdk.Action {
		return "", fmt.Errorf("%s: static build cannot emit @render %s pages yet", page.ID, mode)
	}
	if !page.Blocks.View {
		return "", fmt.Errorf("%s: missing view {}", page.ID)
	}
	if strings.TrimSpace(page.Blocks.ViewBody) == "" {
		return "", fmt.Errorf("%s: view {} is empty", page.ID)
	}

	body, err := view.RenderWithOptions(page.Blocks.ViewBody, components, data, view.Options{
		Actions: actionRoutes(page, data),
	})
	if err != nil {
		return "", fmt.Errorf("%s: %w", page.ID, err)
	}
	return document(page, body, stylesheets), nil
}

func actionRoutes(page manifest.Page, data map[string]string) map[string]string {
	routes := map[string]string{}
	route := page.Route
	for name, value := range data {
		route = strings.ReplaceAll(route, "{"+name+"}", value)
	}
	for _, action := range page.Blocks.Actions {
		if strings.TrimSpace(action.Redirect) == "" {
			continue
		}
		routes[action.Name] = route
	}
	return routes
}

func isComponentName(value string) bool {
	if value == "" {
		return false
	}
	first := []rune(value)[0]
	return first >= 'A' && first <= 'Z'
}

func document(page manifest.Page, body string, stylesheets []gowdk.Stylesheet) string {
	title := page.ID
	var head strings.Builder
	head.WriteString("<head>\n")
	head.WriteString(`  <meta charset="utf-8">` + "\n")
	head.WriteString("  <title>" + gowhtml.Escape(title) + "</title>\n")
	for _, stylesheet := range nonEmptyStylesheets(stylesheets) {
		head.WriteString("  <link rel=\"stylesheet\"" + gowhtml.Attr("href", stylesheet.Href) + ">\n")
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

func outputPath(outputDir, route string) (string, error) {
	if !strings.HasPrefix(route, "/") {
		return "", fmt.Errorf("route %q must start with /", route)
	}
	if strings.ContainsAny(route, "?#") {
		return "", fmt.Errorf("route %q must not contain query or fragment", route)
	}
	if strings.Contains(route, "{") || strings.Contains(route, "}") {
		return "", fmt.Errorf("route %q is dynamic", route)
	}

	trimmed := strings.Trim(route, "/")
	if trimmed == "" {
		return filepath.Join(outputDir, "index.html"), nil
	}

	for _, segment := range strings.Split(trimmed, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("route %q contains unsafe path segment %q", route, segment)
		}
	}

	segments := strings.Split(path.Clean("/"+trimmed), "/")[1:]
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." {
			return "", fmt.Errorf("route %q contains unsafe path segment %q", route, segment)
		}
	}
	parts := append([]string{outputDir}, segments...)
	parts = append(parts, "index.html")
	return filepath.Join(parts...), nil
}

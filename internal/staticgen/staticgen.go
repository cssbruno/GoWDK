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

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/discover"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

const routeManifestFile = "gowdk-routes.json"
const assetManifestFile = "gowdk-assets.json"
const defaultPageCSSDir = "assets/gowdk"

var (
	literalDeclarationPattern = regexp.MustCompile(`^=>\s*\{(.*)\}$`)
	literalNamePattern        = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	cssInputNamePattern       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
	layoutSlotPattern         = regexp.MustCompile(`<slot\s*/>`)
)

var (
	defaultCSSIncludes = []string{"**/*.css"}
	defaultCSSExcludes = []string{".git/**", "vendor/**", "node_modules/**"}
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

// SSRArtifact describes one generated request-time page route.
type SSRArtifact struct {
	PageID string
	Route  string
	HTML   string
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
	layouts, layoutFailures := buildLayouts(app.Layouts)
	css, cssFailures := planCSS(config, app, outputDir)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)
	var planned []plannedArtifact
	var failures []string
	seenOutputPaths := map[string]string{}
	failures = append(failures, componentFailures...)
	failures = append(failures, layoutFailures...)
	failures = append(failures, cssFailures...)
	for _, page := range app.Pages {
		if isRequestTimePage(config, page) {
			continue
		}
		stylesheets := append([]gowdk.Stylesheet{}, baseStylesheets...)
		stylesheets = append(stylesheets, css.pageStylesheets[page.ID]...)
		pageArtifacts, err := pageOutputArtifacts(config, outputDir, page, components, layouts, stylesheets)
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

// SSRArtifacts renders the first supported request-time page slice for
// generated embedded apps. It supports concrete @render ssr pages whose view can
// be rendered with compile-time data only; request-time load {} execution is
// still intentionally rejected.
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
	if len(page.DynamicParams()) > 0 {
		return SSRArtifact{}, fmt.Errorf("%s: generated SSR currently requires a concrete route", page.ID)
	}
	if page.Blocks.Load {
		return SSRArtifact{}, fmt.Errorf("%s: generated SSR load {} execution is not implemented yet", page.ID)
	}
	data, err := parseBuildData(page.Blocks.BuildBody, nil)
	if err != nil {
		return SSRArtifact{}, fmt.Errorf("%s: %w", page.ID, err)
	}
	html, err := renderPage(config, page, components, layouts, stylesheets, data, renderModeRequestTime)
	if err != nil {
		return SSRArtifact{}, err
	}
	return SSRArtifact{PageID: page.ID, Route: page.Route, HTML: html}, nil
}

func isRequestTimePage(config gowdk.Config, page manifest.Page) bool {
	switch page.RenderMode(config.Render.DefaultMode()) {
	case gowdk.SSR, gowdk.Hybrid:
		return true
	default:
		return false
	}
}

type pageOutput struct {
	route string
	data  map[string]string
}

func pageOutputArtifacts(config gowdk.Config, outputDir string, page manifest.Page, components map[string]view.Component, layouts map[string]manifest.Layout, stylesheets []gowdk.Stylesheet) ([]plannedArtifact, error) {
	outputs, err := pageOutputs(page)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", page.ID, err)
	}
	artifacts := make([]plannedArtifact, 0, len(outputs))
	for _, output := range outputs {
		buildData, err := parseBuildData(page.Blocks.BuildBody, output.data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		data, err := mergeBuildData(buildData, output.data)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		html, err := renderPage(config, page, components, layouts, stylesheets, data, renderModeStatic)
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

func parseBuildData(body string, routeParams map[string]string) (map[string]string, error) {
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
	data := declarations[0]
	for key, value := range data {
		interpolated, err := interpolateBuildValue(value, routeParams)
		if err != nil {
			return nil, fmt.Errorf("build field %s: %w", key, err)
		}
		data[key] = interpolated
	}
	return data, nil
}

func interpolateBuildValue(value string, routeParams map[string]string) (string, error) {
	if !strings.Contains(value, "{") {
		return value, nil
	}
	var out strings.Builder
	for {
		start := strings.Index(value, "{")
		if start < 0 {
			out.WriteString(value)
			return out.String(), nil
		}
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return "", fmt.Errorf("unterminated interpolation")
		}
		end += start
		out.WriteString(value[:start])
		name := strings.TrimSpace(value[start+1 : end])
		if param, ok := buildRouteParamExpression(name); ok {
			name = param
		}
		resolved, ok := routeParams[name]
		if !ok {
			return "", fmt.Errorf("unknown route param %q", name)
		}
		out.WriteString(resolved)
		value = value[end+1:]
	}
}

func buildRouteParamExpression(value string) (string, bool) {
	if !strings.HasPrefix(value, `param("`) || !strings.HasSuffix(value, `")`) {
		return "", false
	}
	name := strings.TrimPrefix(strings.TrimSuffix(value, `")`), `param("`)
	if !literalNamePattern.MatchString(name) {
		return "", false
	}
	return name, true
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
		return fmt.Errorf("route param %q value must not contain /, ?, or #", name)
	}
	if value == "." || value == ".." {
		return fmt.Errorf("route param %q value is unsafe", name)
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
	assets          []plannedCSSArtifact
	stylesheets     []gowdk.Stylesheet
	pageStylesheets map[string][]gowdk.Stylesheet
}

type cssInput struct {
	name     string
	path     string
	contents []byte
}

func planCSS(config gowdk.Config, app manifest.Manifest, outputDir string) (cssPlan, []string) {
	planned := cssPlan{pageStylesheets: map[string][]gowdk.Stylesheet{}}
	var failures []string
	seen := map[string]bool{}
	context := gowdk.CSSContext{
		Sources:   cssSources(app),
		OutputDir: outputDir,
		Build:     config.Build,
		CSS:       config.CSS,
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
	inputs, inputFailures := discoverCSSInputs(config, outputDir)
	failures = append(failures, inputFailures...)
	if len(inputFailures) == 0 {
		pageCSS, pageStylesheets, pageFailures := planPageCSS(config, app.Pages, outputDir, inputs, seen)
		failures = append(failures, pageFailures...)
		planned.assets = append(planned.assets, pageCSS...)
		for pageID, stylesheets := range pageStylesheets {
			planned.pageStylesheets[pageID] = append(planned.pageStylesheets[pageID], stylesheets...)
		}
	}
	return planned, failures
}

func discoverCSSInputs(config gowdk.Config, outputDir string) (map[string]cssInput, []string) {
	root, err := os.Getwd()
	if err != nil {
		return nil, []string{err.Error()}
	}

	includes := appendNonEmpty(nil, config.CSS.Include)
	if len(includes) == 0 {
		includes = append([]string{}, defaultCSSIncludes...)
	}
	excludes := append([]string{}, defaultCSSExcludes...)
	excludes = appendNonEmpty(excludes, config.CSS.Exclude)
	if pattern := cssOutputExcludePattern(root, outputDir); pattern != "" {
		excludes = append(excludes, pattern)
	}

	paths, err := discover.Files(root, includes, excludes)
	if err != nil {
		return nil, []string{fmt.Sprintf("css discovery failed: %v", err)}
	}

	inputs := map[string]cssInput{}
	var failures []string
	for _, filePath := range paths {
		name := cssInputName(filePath)
		if !cssInputNamePattern.MatchString(name) {
			failures = append(failures, fmt.Sprintf("css file %q exports invalid name %q", filePath, name))
			continue
		}
		if previous, exists := inputs[name]; exists {
			failures = append(failures, fmt.Sprintf("duplicate css export %q from %s and %s", name, previous.path, filePath))
			continue
		}
		contents, err := os.ReadFile(filePath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("read css file %q: %v", filePath, err))
			continue
		}
		inputs[name] = cssInput{name: name, path: filePath, contents: contents}
	}
	return inputs, failures
}

func cssInputName(filePath string) string {
	base := filepath.Base(filePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func planPageCSS(config gowdk.Config, pages []manifest.Page, outputDir string, inputs map[string]cssInput, seenAssets map[string]bool) ([]plannedCSSArtifact, map[string][]gowdk.Stylesheet, []string) {
	var assets []plannedCSSArtifact
	stylesheets := map[string][]gowdk.Stylesheet{}
	var failures []string
	for _, page := range pages {
		names, err := pageCSSInputNames(config, page, inputs)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		if len(names) == 0 {
			continue
		}
		assetPath, err := pageCSSOutputPath(config.CSS.Output, outputDir, page.ID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", page.ID, err))
			continue
		}
		if seenAssets[assetPath] {
			failures = append(failures, fmt.Sprintf("%s: duplicate css asset path %q", page.ID, assetPath))
			continue
		}
		seenAssets[assetPath] = true
		assets = append(assets, plannedCSSArtifact{
			CSSArtifact: CSSArtifact{Path: assetPath},
			contents:    pageCSSContents(names, inputs),
		})
		stylesheets[page.ID] = []gowdk.Stylesheet{{Href: pageCSSHref(config.CSS.Output, page.ID)}}
	}
	return assets, stylesheets, failures
}

func pageCSSInputNames(config gowdk.Config, page manifest.Page, inputs map[string]cssInput) ([]string, error) {
	references := page.CSS
	if len(references) == 0 {
		references = []string{"default", "page"}
	}
	if len(references) == 1 && references[0] == "none" {
		return nil, nil
	}

	var names []string
	seen := map[string]bool{}
	add := func(name string) error {
		if seen[name] {
			return nil
		}
		if _, ok := inputs[name]; !ok {
			return fmt.Errorf("%s: unknown css input %q", page.ID, name)
		}
		seen[name] = true
		names = append(names, name)
		return nil
	}

	for _, reference := range references {
		switch reference {
		case "default":
			defaults, err := defaultCSSInputs(config, inputs)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", page.ID, err)
			}
			for _, name := range defaults {
				if err := add(name); err != nil {
					return nil, err
				}
			}
		case "page":
			if _, ok := inputs[page.ID]; ok {
				if err := add(page.ID); err != nil {
					return nil, err
				}
			}
		case "none":
			return nil, fmt.Errorf("%s: @css none must be used by itself", page.ID)
		default:
			if err := add(reference); err != nil {
				return nil, err
			}
		}
	}
	return names, nil
}

func defaultCSSInputs(config gowdk.Config, inputs map[string]cssInput) ([]string, error) {
	if len(config.CSS.Default) > 0 {
		for _, name := range config.CSS.Default {
			if _, ok := inputs[name]; !ok {
				return nil, fmt.Errorf("unknown default css input %q", name)
			}
		}
		return append([]string{}, config.CSS.Default...), nil
	}
	if _, ok := inputs["global"]; ok {
		return []string{"global"}, nil
	}
	return nil, nil
}

func pageCSSContents(names []string, inputs map[string]cssInput) []byte {
	var builder strings.Builder
	for _, name := range names {
		input := inputs[name]
		builder.WriteString("/* gowdk css: ")
		builder.WriteString(name)
		builder.WriteString(" */\n")
		builder.Write(input.contents)
		if len(input.contents) == 0 || input.contents[len(input.contents)-1] != '\n' {
			builder.WriteString("\n")
		}
	}
	return []byte(builder.String())
}

func pageCSSOutputPath(output gowdk.CSSOutputConfig, outputDir string, pageID string) (string, error) {
	assetPath := path.Join(cssOutputDir(output), pageID+".css")
	return cssOutputPath(outputDir, assetPath)
}

func pageCSSHref(output gowdk.CSSOutputConfig, pageID string) string {
	prefix := strings.TrimSpace(output.HrefPrefix)
	if prefix == "" {
		prefix = "/" + cssOutputDir(output)
	}
	return path.Join("/", strings.Trim(prefix, "/"), pageID+".css")
}

func cssOutputDir(output gowdk.CSSOutputConfig) string {
	dir := strings.Trim(strings.TrimSpace(output.Dir), "/")
	if dir == "" {
		return defaultPageCSSDir
	}
	return path.Clean(filepath.ToSlash(dir))
}

func cssOutputExcludePattern(root string, outputDir string) string {
	if strings.TrimSpace(outputDir) == "" {
		return ""
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return ""
	}
	absoluteOutput, err := filepath.Abs(outputDir)
	if err != nil {
		return ""
	}
	rel, err := filepath.Rel(absoluteRoot, absoluteOutput)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return ""
	}
	return filepath.ToSlash(rel) + "/**"
}

func appendNonEmpty(values []string, patterns []string) []string {
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		values = append(values, pattern)
	}
	return values
}

func cssSources(app manifest.Manifest) []gowdk.CSSSource {
	sources := make([]gowdk.CSSSource, 0, len(app.Pages)+len(app.Components))
	for _, page := range app.Pages {
		sources = append(sources, gowdk.CSSSource{
			Path:       page.Source,
			Kind:       "page",
			Name:       page.ID,
			CSSClasses: cssClassesFromViewBody(page.Blocks.ViewBody),
		})
	}
	for _, component := range app.Components {
		sources = append(sources, gowdk.CSSSource{
			Path:       component.Source,
			Kind:       "component",
			Name:       component.Name,
			CSSClasses: cssClassesFromViewBody(component.Blocks.ViewBody),
		})
	}
	return sources
}

func cssClassesFromViewBody(body string) []string {
	if strings.TrimSpace(body) == "" {
		return nil
	}
	dependencies, err := view.ViewDependencies(body)
	if err != nil {
		return nil
	}
	return dependencies.CSSClasses
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

func buildLayouts(layouts []manifest.Layout) (map[string]manifest.Layout, []string) {
	registry := map[string]manifest.Layout{}
	var failures []string
	for _, layout := range layouts {
		if layout.ID == "" {
			failures = append(failures, "layout missing ID")
			continue
		}
		if _, exists := registry[layout.ID]; exists {
			failures = append(failures, fmt.Sprintf("duplicate layout %q", layout.ID))
			continue
		}
		if !layout.Blocks.View {
			failures = append(failures, fmt.Sprintf("layout %s missing view {}", layout.ID))
			continue
		}
		if strings.TrimSpace(layout.Blocks.ViewBody) == "" {
			failures = append(failures, fmt.Sprintf("layout %s view {} is empty", layout.ID))
			continue
		}
		registry[layout.ID] = layout
	}
	return registry, failures
}

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
	return document(page, body, stylesheets), nil
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

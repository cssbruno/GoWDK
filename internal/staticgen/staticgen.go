// Package staticgen emits static HTML artifacts for build-time pages.
package staticgen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/clientrt"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/discover"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

const routeManifestFile = "gowdk-routes.json"
const assetManifestFile = "gowdk-assets.json"
const defaultPageCSSDir = "assets/gowdk"
const clientRuntimeAssetPath = "assets/gowdk/" + clientrt.Filename
const clientRuntimeHref = "/" + clientRuntimeAssetPath
const DisableCSSDiscovery = "__gowdk_disable_css_discovery__"
const islandRuntimeDir = "assets/gowdk/islands"

var (
	literalDeclarationPattern = regexp.MustCompile(`^=>\s*\{(.*)\}$`)
	buildCallPattern          = regexp.MustCompile(`^=>\s*([A-Za-z_][A-Za-z0-9_]*)\.([A-Za-z_][A-Za-z0-9_]*)\(\)$`)
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

// AssetArtifact describes one emitted non-CSS asset file.
type AssetArtifact struct {
	Path string
}

// Result describes a static build.
type Result struct {
	Artifacts         []Artifact
	CSSArtifacts      []CSSArtifact
	AssetArtifacts    []AssetArtifact
	RouteManifestPath string
	AssetManifestPath string
}

// MemoryResult describes a static build whose artifacts were collected without
// writing to disk.
type MemoryResult struct {
	Result
	Files map[string][]byte
}

// SSRArtifact describes one generated request-time page route.
type SSRArtifact struct {
	PageID       string
	Route        string
	HTML         string
	Replacements []SSRReplacement
}

// SSRReplacement maps a generated placeholder back to a request route param.
type SSRReplacement struct {
	Param       string
	Placeholder string
}

type plannedArtifact struct {
	Artifact
	contents []byte
}

type plannedCSSArtifact struct {
	CSSArtifact
	contents []byte
}

type plannedAssetArtifact struct {
	AssetArtifact
	contents []byte
}

type buildPlan struct {
	pages  []plannedArtifact
	css    []plannedCSSArtifact
	assets []plannedAssetArtifact
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
		Artifacts:      make([]Artifact, 0, len(planned.pages)),
		CSSArtifacts:   make([]CSSArtifact, 0, len(planned.css)),
		AssetArtifacts: make([]AssetArtifact, 0, len(planned.assets)),
	}
	for _, artifact := range planned.css {
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, err
		}
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
	}
	for _, artifact := range planned.assets {
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, err
		}
		result.AssetArtifacts = append(result.AssetArtifacts, artifact.AssetArtifact)
	}
	for _, artifact := range planned.pages {
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, err
		}
		result.Artifacts = append(result.Artifacts, artifact.Artifact)
	}
	manifestPath, err := writeRouteManifest(outputDir, result.Artifacts)
	if err != nil {
		return Result{}, err
	}
	result.RouteManifestPath = manifestPath
	assetManifestPath, err := writeAssetManifest(outputDir, result.CSSArtifacts, result.AssetArtifacts)
	if err != nil {
		return Result{}, err
	}
	result.AssetManifestPath = assetManifestPath
	return result, nil
}

// BuildMemory renders static HTML, CSS, runtime assets, and manifests into an
// in-memory file map. File keys are slash-separated paths relative to outputDir.
func BuildMemory(config gowdk.Config, app manifest.Manifest, outputDir string) (MemoryResult, error) {
	if strings.TrimSpace(outputDir) == "" {
		return MemoryResult{}, fmt.Errorf("build output directory is required")
	}
	if err := compiler.ValidateManifest(config, app); err != nil {
		return MemoryResult{}, err
	}

	planned, err := plan(config, app, outputDir)
	if err != nil {
		return MemoryResult{}, err
	}

	result := MemoryResult{
		Result: Result{
			Artifacts:         make([]Artifact, 0, len(planned.pages)),
			CSSArtifacts:      make([]CSSArtifact, 0, len(planned.css)),
			AssetArtifacts:    make([]AssetArtifact, 0, len(planned.assets)),
			RouteManifestPath: filepath.Join(outputDir, routeManifestFile),
			AssetManifestPath: filepath.Join(outputDir, assetManifestFile),
		},
		Files: map[string][]byte{},
	}
	for _, artifact := range planned.css {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return MemoryResult{}, err
		}
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
		result.Files[rel] = append([]byte(nil), artifact.contents...)
	}
	for _, artifact := range planned.assets {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return MemoryResult{}, err
		}
		result.AssetArtifacts = append(result.AssetArtifacts, artifact.AssetArtifact)
		result.Files[rel] = append([]byte(nil), artifact.contents...)
	}
	for _, artifact := range planned.pages {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return MemoryResult{}, err
		}
		result.Artifacts = append(result.Artifacts, artifact.Artifact)
		result.Files[rel] = append([]byte(nil), artifact.contents...)
	}

	routeManifest, err := routeManifestPayload(outputDir, result.Artifacts)
	if err != nil {
		return MemoryResult{}, err
	}
	result.Files[routeManifestFile] = routeManifest
	assetManifest, err := assetManifestPayload(outputDir, result.CSSArtifacts, result.AssetArtifacts)
	if err != nil {
		return MemoryResult{}, err
	}
	result.Files[assetManifestFile] = assetManifest
	return result, nil
}

// BuildIncremental validates the full manifest and refreshes manifests, but
// only renders pages whose source path is listed in changedPageSources.
func BuildIncremental(config gowdk.Config, app manifest.Manifest, outputDir string, changedPageSources []string) (Result, error) {
	if strings.TrimSpace(outputDir) == "" {
		return Result{}, fmt.Errorf("build output directory is required")
	}
	if err := compiler.ValidateManifest(config, app); err != nil {
		return Result{}, err
	}

	changedPages := sourcePathSet(changedPageSources)
	components, componentFailures := buildComponents(app.Components)
	layouts, layoutFailures := buildLayouts(app.Layouts)
	css, cssFailures := planCSS(config, app, outputDir)
	baseStylesheets := append([]gowdk.Stylesheet{}, config.Build.Stylesheets...)
	baseStylesheets = append(baseStylesheets, css.stylesheets...)

	var failures []string
	failures = append(failures, componentFailures...)
	failures = append(failures, layoutFailures...)
	failures = append(failures, cssFailures...)
	if len(failures) > 0 {
		return Result{}, errors.New(strings.Join(failures, "\n"))
	}

	result := Result{
		Artifacts:      make([]Artifact, 0, len(app.Pages)),
		CSSArtifacts:   make([]CSSArtifact, 0, len(css.assets)),
		AssetArtifacts: make([]AssetArtifact, 0, 1),
	}
	previousRoutes, err := readRouteManifestIfExists(outputDir)
	if err != nil {
		return Result{}, err
	}
	changedPageIDs := map[string]bool{}
	for _, artifact := range css.assets {
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, err
		}
		result.CSSArtifacts = append(result.CSSArtifacts, artifact.CSSArtifact)
	}
	for _, artifact := range runtimeArtifacts(app, outputDir, layouts) {
		if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
			return Result{}, err
		}
		result.AssetArtifacts = append(result.AssetArtifacts, artifact.AssetArtifact)
	}

	seenOutputPaths := map[string]string{}
	for _, page := range app.Pages {
		if isRequestTimePage(config, page) {
			continue
		}
		routeArtifacts, err := pageRouteArtifacts(outputDir, page)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, artifact := range routeArtifacts {
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
			result.Artifacts = append(result.Artifacts, artifact)
		}

		if !sourcePathChanged(changedPages, page.Source) {
			continue
		}
		changedPageIDs[page.ID] = true
		stylesheets := append([]gowdk.Stylesheet{}, baseStylesheets...)
		stylesheets = append(stylesheets, css.pageStylesheets[page.ID]...)
		pageArtifacts, err := pageOutputArtifacts(config, outputDir, page, components, layouts, stylesheets)
		if err != nil {
			failures = append(failures, err.Error())
			continue
		}
		for _, artifact := range pageArtifacts {
			if err := writeFileIfChanged(artifact.Path, artifact.contents); err != nil {
				return Result{}, err
			}
		}
	}
	if len(failures) > 0 {
		return Result{}, errors.New(strings.Join(failures, "\n"))
	}
	if err := removeStaleChangedPageArtifacts(outputDir, previousRoutes, result.Artifacts, changedPageIDs); err != nil {
		return Result{}, err
	}

	manifestPath, err := writeRouteManifest(outputDir, result.Artifacts)
	if err != nil {
		return Result{}, err
	}
	result.RouteManifestPath = manifestPath
	assetManifestPath, err := writeAssetManifest(outputDir, result.CSSArtifacts, result.AssetArtifacts)
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
	return buildPlan{pages: planned, css: css.assets, assets: runtimeArtifacts(app, outputDir, layouts)}, nil
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

type pageOutput struct {
	route string
	data  map[string]string
}

func pageRouteArtifacts(outputDir string, page manifest.Page) ([]Artifact, error) {
	outputs, err := pageOutputs(page)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", page.ID, err)
	}
	artifacts := make([]Artifact, 0, len(outputs))
	for _, output := range outputs {
		outputPath, err := outputPath(outputDir, output.route)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", page.ID, err)
		}
		artifacts = append(artifacts, Artifact{PageID: page.ID, Route: output.route, Path: outputPath})
	}
	return artifacts, nil
}

func pageOutputArtifacts(config gowdk.Config, outputDir string, page manifest.Page, components map[string]view.Component, layouts map[string]manifest.Layout, stylesheets []gowdk.Stylesheet) ([]plannedArtifact, error) {
	outputs, err := pageOutputs(page)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", page.ID, err)
	}
	artifacts := make([]plannedArtifact, 0, len(outputs))
	for _, output := range outputs {
		buildData, err := parseBuildData(page.Blocks.BuildBody, output.data, page.Imports)
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

func parseBuildData(body string, routeParams map[string]string, imports []manifest.Import) (map[string]string, error) {
	lines := significantBuildLines(body)
	if len(lines) == 1 {
		if match := buildCallPattern.FindStringSubmatch(lines[0]); match != nil {
			return runBuildDataCall(match[1], match[2], imports)
		}
	}
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

func significantBuildLines(body string) []string {
	var lines []string
	for _, rawLine := range strings.Split(body, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func runBuildDataCall(alias, function string, imports []manifest.Import) (map[string]string, error) {
	item, ok := findBuildImport(alias, imports)
	if !ok {
		return nil, fmt.Errorf("build import %q is not declared", alias)
	}
	source, err := buildDataRunnerSource(alias, item.Path, function)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp("", "gowdk-build-data-*.go")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.WriteString(source); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}

	command := exec.Command("go", "run", path)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run build data function %s.%s: %w\n%s", alias, function, err, strings.TrimSpace(string(output)))
	}
	return parseBuildFunctionOutput(output)
}

func findBuildImport(alias string, imports []manifest.Import) (manifest.Import, bool) {
	for _, item := range imports {
		if item.Alias == alias {
			return item, true
		}
	}
	return manifest.Import{}, false
}

func buildDataRunnerSource(alias, importPath, function string) (string, error) {
	if !literalNamePattern.MatchString(alias) {
		return "", fmt.Errorf("invalid build import alias %q", alias)
	}
	if !literalNamePattern.MatchString(function) {
		return "", fmt.Errorf("invalid build function name %q", function)
	}
	if strings.TrimSpace(importPath) == "" {
		return "", fmt.Errorf("build import %q has an empty path", alias)
	}
	return fmt.Sprintf(`package main

import (
	"encoding/json"
	"os"

	%s %q
)

func main() {
	value := %s.%s()
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		panic(err)
	}
}
`, alias, importPath, alias, function), nil
}

func parseBuildFunctionOutput(output []byte) (map[string]string, error) {
	var raw map[string]any
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("decode build data output: %w", err)
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("build data function must return a non-empty JSON object")
	}
	data := map[string]string{}
	for key, value := range raw {
		if !literalNamePattern.MatchString(key) {
			return nil, fmt.Errorf("invalid build field name %q", key)
		}
		scalar, ok := buildScalarString(value)
		if !ok {
			return nil, fmt.Errorf("build field %s must be a string, number, boolean, or null", key)
		}
		data[key] = scalar
	}
	return data, nil
}

func buildScalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", true
	case string:
		if strings.TrimSpace(typed) == "" {
			return "", false
		}
		return typed, true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
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

func sourcePathSet(paths []string) map[string]bool {
	set := map[string]bool{}
	for _, sourcePath := range paths {
		abs, err := filepath.Abs(sourcePath)
		if err != nil {
			continue
		}
		set[filepath.Clean(abs)] = true
	}
	return set
}

func sourcePathChanged(set map[string]bool, sourcePath string) bool {
	abs, err := filepath.Abs(sourcePath)
	if err != nil {
		return false
	}
	return set[filepath.Clean(abs)]
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
	includes := appendNonEmpty(nil, config.CSS.Include)
	if len(includes) == 1 && includes[0] == DisableCSSDiscovery {
		return map[string]cssInput{}, nil
	}
	if len(includes) == 0 {
		includes = append([]string{}, defaultCSSIncludes...)
	}

	root, err := os.Getwd()
	if err != nil {
		return nil, []string{err.Error()}
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
	payload, err := routeManifestPayload(outputDir, artifacts)
	if err != nil {
		return "", err
	}

	manifestPath := filepath.Join(outputDir, routeManifestFile)
	if err := writeFileIfChanged(manifestPath, payload); err != nil {
		return "", err
	}
	return manifestPath, nil
}

func routeManifestPayload(outputDir string, artifacts []Artifact) ([]byte, error) {
	routes := make([]routeManifestEntry, 0, len(artifacts))
	for _, artifact := range artifacts {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return nil, err
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
		return nil, err
	}
	payload = append(payload, '\n')
	return payload, nil
}

func readRouteManifestIfExists(outputDir string) (routeManifest, error) {
	manifestPath := filepath.Join(outputDir, routeManifestFile)
	payload, err := os.ReadFile(manifestPath)
	if os.IsNotExist(err) {
		return routeManifest{}, nil
	}
	if err != nil {
		return routeManifest{}, err
	}
	var manifest routeManifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		return routeManifest{}, fmt.Errorf("read existing route manifest: %w", err)
	}
	return manifest, nil
}

func removeStaleChangedPageArtifacts(outputDir string, previous routeManifest, current []Artifact, changedPageIDs map[string]bool) error {
	if len(previous.Routes) == 0 || len(changedPageIDs) == 0 {
		return nil
	}
	keep := map[string]bool{}
	for _, artifact := range current {
		if !changedPageIDs[artifact.PageID] {
			continue
		}
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return err
		}
		keep[rel] = true
	}
	for _, route := range previous.Routes {
		if !changedPageIDs[route.PageID] || keep[route.Path] {
			continue
		}
		filePath, err := outputFilePath(outputDir, route.Path)
		if err != nil {
			return err
		}
		if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func outputFilePath(outputDir, rel string) (string, error) {
	if strings.TrimSpace(rel) == "" {
		return "", fmt.Errorf("route manifest path is required")
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("route manifest path %q must be relative", rel)
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("route manifest path %q must stay inside output directory", rel)
	}
	return filepath.Join(outputDir, clean), nil
}

func clientRuntimeArtifacts(pages []manifest.Page, outputDir string) []plannedAssetArtifact {
	for _, page := range pages {
		if pageUsesPartialRuntime(page, page.Blocks.ViewBody) {
			return []plannedAssetArtifact{{
				AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(clientRuntimeAssetPath))},
				contents:      clientrt.Source(),
			}}
		}
	}
	return nil
}

func runtimeArtifacts(app manifest.Manifest, outputDir string, layouts map[string]manifest.Layout) []plannedAssetArtifact {
	var artifacts []plannedAssetArtifact
	artifacts = append(artifacts, clientRuntimeArtifacts(app.Pages, outputDir)...)
	artifacts = append(artifacts, islandRuntimeArtifacts(app, outputDir, layouts)...)
	return dedupeAssetArtifacts(artifacts)
}

func islandRuntimeArtifacts(app manifest.Manifest, outputDir string, layouts map[string]manifest.Layout) []plannedAssetArtifact {
	stateful := statefulComponentNames(app.Components)
	planned := map[string]plannedAssetArtifact{}
	for _, page := range app.Pages {
		source, err := composePageViewSource(page, layouts)
		if err != nil {
			source = page.Blocks.ViewBody
		}
		usages, err := view.ComponentCallUsages(source)
		if err != nil {
			continue
		}
		for _, usage := range usages {
			switch usage.Island {
			case "wasm":
				addAsset(planned, islandWASMArtifact(outputDir, usage.Component))
				addAsset(planned, islandWASMLoaderArtifact(outputDir, usage.Component))
			case "":
				if stateful[usage.Component] {
					addAsset(planned, islandJSArtifact(outputDir, usage.Component))
				}
			}
		}
	}
	if len(planned) == 0 {
		return nil
	}
	paths := make([]string, 0, len(planned))
	for path := range planned {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	artifacts := make([]plannedAssetArtifact, 0, len(paths))
	for _, path := range paths {
		artifacts = append(artifacts, planned[path])
	}
	return artifacts
}

func islandScriptHrefs(source string, components map[string]view.Component) []string {
	usages, err := view.ComponentCallUsages(source)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var scripts []string
	for _, usage := range usages {
		href := ""
		switch usage.Island {
		case "wasm":
			href = "/" + islandWASMLoaderAssetPath(usage.Component)
		case "":
			component, ok := components[usage.Component]
			if ok && (component.StateJSON != "" || component.HandlersJSON != "") {
				href = "/" + islandJSAssetPath(usage.Component)
			}
		}
		if href == "" || seen[href] {
			continue
		}
		seen[href] = true
		scripts = append(scripts, href)
	}
	sort.Strings(scripts)
	return scripts
}

func statefulComponentNames(components []manifest.Component) map[string]bool {
	out := map[string]bool{}
	for _, component := range components {
		if component.State.Type.Name != "" || component.Blocks.Client {
			out[component.Name] = true
		}
	}
	return out
}

func addAsset(artifacts map[string]plannedAssetArtifact, artifact plannedAssetArtifact) {
	artifacts[artifact.Path] = artifact
}

func dedupeAssetArtifacts(artifacts []plannedAssetArtifact) []plannedAssetArtifact {
	if len(artifacts) < 2 {
		return artifacts
	}
	seen := map[string]plannedAssetArtifact{}
	for _, artifact := range artifacts {
		seen[artifact.Path] = artifact
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]plannedAssetArtifact, 0, len(paths))
	for _, path := range paths {
		out = append(out, seen[path])
	}
	return out
}

func islandJSArtifact(outputDir, componentName string) plannedAssetArtifact {
	assetPath := islandJSAssetPath(componentName)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(islandJSSource(componentName)),
	}
}

func islandWASMArtifact(outputDir, componentName string) plannedAssetArtifact {
	assetPath := islandWASMAssetPath(componentName)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00},
	}
}

func islandWASMLoaderArtifact(outputDir, componentName string) plannedAssetArtifact {
	assetPath := islandWASMLoaderAssetPath(componentName)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(islandWASMLoaderSource(componentName)),
	}
}

func islandJSAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".js")
}

func islandWASMAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".wasm")
}

func islandWASMLoaderAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".wasm.js")
}

func componentAssetName(componentName string) string {
	name := exportedSafe(componentName)
	if name == "" {
		return "component"
	}
	return name
}

func islandJSSource(componentName string) string {
	component := strconv.Quote(componentName)
	return fmt.Sprintf(`(() => {
  const component = %s;
  const selector = "gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"js\"]";
  const booleanAttrs = new Set(["allowfullscreen", "async", "autofocus", "autoplay", "checked", "controls", "default", "defer", "disabled", "formnovalidate", "hidden", "inert", "ismap", "loop", "multiple", "muted", "nomodule", "novalidate", "open", "readonly", "required", "reversed", "selected"]);

  function matchingBrace(source, openIndex) {
    let depth = 0;
    let inString = false;
    let escaped = false;
    for (let i = openIndex; i < source.length; i++) {
      const char = source[i];
      if (escaped) {
        escaped = false;
        continue;
      }
      if (inString) {
        if (char === "\\") escaped = true;
        else if (char === "\"") inString = false;
        continue;
      }
      if (char === "\"") inString = true;
      else if (char === "{") depth++;
      else if (char === "}") {
        depth--;
        if (depth === 0) return i;
      }
    }
    return -1;
  }

  function expressionSource(source) {
    source = source.trim();
    if (!source.startsWith("if ")) return source.replace(/\bnil\b/g, "null");
    const thenOpen = source.indexOf("{");
    if (thenOpen < 0) return source.replace(/\bnil\b/g, "null");
    const thenClose = matchingBrace(source, thenOpen);
    if (thenClose < 0) return source.replace(/\bnil\b/g, "null");
    const tail = source.slice(thenClose + 1).trim();
    if (!tail.startsWith("else")) return source.replace(/\bnil\b/g, "null");
    const elseOpen = source.indexOf("{", thenClose + 1);
    if (elseOpen < 0) return source.replace(/\bnil\b/g, "null");
    const elseClose = matchingBrace(source, elseOpen);
    if (elseClose < 0) return source.replace(/\bnil\b/g, "null");
    const cond = source.slice(2, thenOpen).trim();
    const thenExpr = source.slice(thenOpen + 1, thenClose).trim();
    const elseExpr = source.slice(elseOpen + 1, elseClose).trim();
    return "(" + expressionSource(cond) + " ? " + expressionSource(thenExpr) + " : " + expressionSource(elseExpr) + ")";
  }

  function callHelper(name, args, state, helpers, stack) {
    const helper = helpers && helpers[name];
    if (!helper) return null;
    stack = stack || [];
    if (stack.indexOf(name) >= 0) throw new Error("recursive GOWDK helper " + name);
    const nextScope = Object.create(null);
    (helper.params || []).forEach((param, index) => {
      nextScope[param] = args[index];
    });
    return valueOf(helper.return || "", state, nextScope, helpers, stack.concat([name]));
  }

  const builtins = Object.freeze({
    len(value) {
      if (value == null) return 0;
      if (typeof value === "string" || Array.isArray(value)) return value.length;
      return 0;
    },
    string(value) {
      if (value == null) return "";
      return String(value);
    },
    int(value) {
      const next = Number.parseInt(value, 10);
      return Number.isNaN(next) ? 0 : next;
    },
    float(value) {
      const next = Number.parseFloat(value);
      return Number.isNaN(next) ? 0 : next;
    }
  });

  function valueOf(token, state, scope, helpers, stack) {
    token = token.trim();
    if (token === "true") return true;
    if (token === "false") return false;
    if (token === "null" || token === "nil") return null;
    if (/^-?[0-9]+(?:\.[0-9]+)?$/.test(token)) return Number(token);
    if (token[0] === "\"") return JSON.parse(token);
    if (scope && Object.prototype.hasOwnProperty.call(scope, token)) return scope[token];
    if (Object.prototype.hasOwnProperty.call(state, token)) return state[token];
    const env = Object.assign(Object.create(null), builtins, state, scope || {});
    Object.keys(helpers || {}).forEach((name) => {
      env[name] = (...args) => callHelper(name, args, state, helpers, stack || []);
    });
    return Function("env", "with (env) { return (" + expressionSource(token) + "); }")(env);
  }

  function recomputeComputed(state, computeds, helpers) {
    (computeds || []).forEach((computed) => {
      state[computed.name] = valueOf(computed.expr, state, null, helpers);
    });
  }

  function splitArgs(source) {
    source = source.trim();
    if (!source) return [];
    const args = [];
    let start = 0;
    let depth = 0;
    let inString = false;
    let escaped = false;
    for (let i = 0; i < source.length; i++) {
      const char = source[i];
      if (escaped) {
        escaped = false;
        continue;
      }
      if (inString) {
        if (char === "\\") escaped = true;
        else if (char === "\"") inString = false;
        continue;
      }
      if (char === "\"") inString = true;
      else if (char === "(" || char === "[" || char === "{") depth++;
      else if (char === ")" || char === "]" || char === "}") depth--;
      else if (char === ",") {
        if (depth > 0) continue;
        args.push(source.slice(start, i).trim());
        start = i + 1;
      }
    }
    args.push(source.slice(start).trim());
    return args;
  }

  function applyExpression(expr, state, handlers, helpers, scope, refs, computeds) {
    expr = expr.trim();
    let local = expr.match(/^let\s+([A-Za-z_][A-Za-z0-9_]*)\s+[A-Za-z_][A-Za-z0-9_]*\s*=\s*(.+)$/);
    if (local) {
      if (!scope) return;
      scope[local[1]] = valueOf(local[2], state, scope, helpers);
      return;
    }
    let call = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\((.*)\)$/);
    if (call) {
      if (call[1] === "append" || call[1] === "remove" || call[1] === "move") {
        const args = splitArgs(call[2]);
        const field = (args[0] || "").trim();
        if (!Array.isArray(state[field])) return;
        if (call[1] === "append" && args.length === 2) {
          state[field] = state[field].concat([valueOf(args[1], state, scope, helpers)]);
          return;
        }
        if (call[1] === "remove" && args.length === 2) {
          const index = Number(valueOf(args[1], state, scope, helpers));
          if (!Number.isInteger(index) || index < 0 || index >= state[field].length) return;
          state[field] = state[field].slice(0, index).concat(state[field].slice(index + 1));
          return;
        }
        if (call[1] === "move" && args.length === 3) {
          const from = Number(valueOf(args[1], state, scope, helpers));
          const to = Number(valueOf(args[2], state, scope, helpers));
          if (!Number.isInteger(from) || !Number.isInteger(to) || from < 0 || from >= state[field].length || to < 0 || to >= state[field].length || from === to) return;
          const next = state[field].slice();
          const item = next.splice(from, 1)[0];
          next.splice(to, 0, item);
          state[field] = next;
          return;
        }
        return;
      }
      const handler = handlers[call[1]];
      if (!handler) return;
      const params = handler.params || [];
      const args = splitArgs(call[2]);
      const nextScope = Object.create(null);
      params.forEach((param, index) => {
        nextScope[param] = valueOf(args[index] || "", state, scope, helpers);
      });
      (handler.statements || handler).forEach((statement) => {
        applyExpression(statement, state, handlers, helpers, nextScope, refs, computeds);
        recomputeComputed(state, computeds, helpers);
      });
      return;
    }
    let refCall = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\.(Focus|Blur|ScrollIntoView)\(\)$/);
    if (refCall) {
      const node = refs && refs[refCall[1]];
      if (!node) return;
      if (refCall[2] === "Focus" && typeof node.focus === "function") node.focus();
      else if (refCall[2] === "Blur" && typeof node.blur === "function") node.blur();
      else if (refCall[2] === "ScrollIntoView" && typeof node.scrollIntoView === "function") node.scrollIntoView();
      return;
    }
    let match = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)(\+\+|--)$/);
    if (match) {
      const current = Number(state[match[1]] || 0);
      state[match[1]] = match[2] === "++" ? current + 1 : current - 1;
      return;
    }
    match = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$/);
    if (match) {
      const target = match[1];
      const value = match[2].trim();
      const toggle = value.match(/^!\s*([A-Za-z_][A-Za-z0-9_]*)$/);
      state[target] = toggle ? !Boolean(valueOf(toggle[1], state, scope, helpers)) : valueOf(value, state, scope, helpers);
    }
  }

  function applyStatements(statements, state, handlers, helpers, scope, refs, computeds) {
    (statements || []).forEach((statement) => {
      applyExpression(statement, state, handlers, helpers, scope || null, refs, computeds);
      recomputeComputed(state, computeds, helpers);
    });
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>"']/g, (char) => {
      if (char === "&") return "&amp;";
      if (char === "<") return "&lt;";
      if (char === ">") return "&gt;";
      if (char === "\"") return "&quot;";
      return "&#39;";
    });
  }

  function interpolateTemplate(template, state, scope, helpers) {
    return template.replace(/\{\{([^{}]+)\}\}/g, (_match, expr) => {
      const value = valueOf(expr, state, scope, helpers);
      return value == null ? "" : escapeHTML(value);
    });
  }

  function firstTemplateElement(html) {
    const holder = document.createElement("template");
    holder.innerHTML = html.trim();
    return holder.content.firstElementChild;
  }

  function syncElement(target, source) {
    Array.from(target.attributes).forEach((attr) => {
      if (!source.hasAttribute(attr.name)) target.removeAttribute(attr.name);
    });
    Array.from(source.attributes).forEach((attr) => {
      if (target.getAttribute(attr.name) !== attr.value) target.setAttribute(attr.name, attr.value);
    });
    if (target.innerHTML !== source.innerHTML) target.innerHTML = source.innerHTML;
  }

  function renderListLoops(root, state, helpers) {
    root.querySelectorAll("template[data-gowdk-for]").forEach((marker) => {
      const group = marker.getAttribute("data-gowdk-for");
      const itemName = marker.getAttribute("data-gowdk-for-var");
      const indexName = marker.getAttribute("data-gowdk-for-index-var");
      const source = marker.getAttribute("data-gowdk-for-source");
      const keyExpr = marker.getAttribute("data-gowdk-for-key");
      const template = marker.getAttribute("data-gowdk-for-template") || "";
      const items = valueOf(source, state, null, helpers);
      const existing = new Map();
      let cursor = marker.nextSibling;
      while (cursor) {
        const next = cursor.nextSibling;
        if (cursor.nodeType !== 1 || cursor.getAttribute("data-gowdk-for-item") !== group) break;
        existing.set(cursor.getAttribute("data-gowdk-key-value") || "", cursor);
        cursor = next;
      }
      if (!Array.isArray(items)) return;
      const fragment = document.createDocumentFragment();
      const used = new Set();
      items.forEach((item, index) => {
        const scope = Object.create(null);
        scope[itemName] = item;
        scope.index = index;
        if (indexName) scope[indexName] = index;
        const key = String(valueOf(keyExpr, state, scope, helpers) ?? "");
        const fresh = firstTemplateElement(interpolateTemplate(template, state, scope, helpers));
        if (!fresh) return;
        const reused = existing.get(key);
        if (reused && !used.has(key)) {
          syncElement(reused, fresh);
          fragment.appendChild(reused);
          used.add(key);
          return;
        }
        fragment.appendChild(fresh);
        used.add(key);
      });
      marker.parentNode.insertBefore(fragment, marker.nextSibling);
      existing.forEach((node, key) => {
        if (!used.has(key) && node.parentNode) node.parentNode.removeChild(node);
      });
    });
  }

  function eventModifiers(source) {
    const modifiers = { prevent: false, stop: false, once: false, capture: false, debounce: 0, throttle: 0 };
    (source || "").split(/\s+/).filter(Boolean).forEach((item) => {
      if (item === "prevent") modifiers.prevent = true;
      else if (item === "stop") modifiers.stop = true;
      else if (item === "once") modifiers.once = true;
      else if (item === "capture") modifiers.capture = true;
      else if (item.startsWith("debounce:")) modifiers.debounce = Number(item.slice("debounce:".length)) || 0;
      else if (item.startsWith("throttle:")) modifiers.throttle = Number(item.slice("throttle:".length)) || 0;
    });
    return modifiers;
  }

  function render(root, state, helpers) {
    renderListLoops(root, state, helpers);
    root.querySelectorAll("[data-gowdk-bind]").forEach((node) => {
      const field = node.getAttribute("data-gowdk-bind");
      node.textContent = state[field] == null ? "" : String(state[field]);
    });
    const conditionalGroups = new Map();
    root.querySelectorAll("[data-gowdk-if-group]").forEach((node) => {
      const group = node.getAttribute("data-gowdk-if-group");
      if (!conditionalGroups.has(group)) conditionalGroups.set(group, []);
      conditionalGroups.get(group).push(node);
    });
    conditionalGroups.forEach((nodes) => {
      nodes.sort((left, right) => Number(left.getAttribute("data-gowdk-if-index")) - Number(right.getAttribute("data-gowdk-if-index")));
      let matched = false;
      nodes.forEach((node) => {
        const condition = node.getAttribute("data-gowdk-if");
        const visible = !matched && (condition == null || Boolean(valueOf(condition, state, null, helpers)));
        node.hidden = !visible;
        if (visible) matched = true;
      });
    });
    root.querySelectorAll("[data-gowdk-bind-value]").forEach((node) => {
      const field = node.getAttribute("data-gowdk-bind-value");
      if (node.type === "radio") {
        node.checked = String(state[field] == null ? "" : state[field]) === node.value;
        return;
      }
      const value = state[field] == null ? "" : String(state[field]);
      if (document.activeElement !== node && node.value !== value) node.value = value;
    });
    root.querySelectorAll("[data-gowdk-bind-checked]").forEach((node) => {
      const field = node.getAttribute("data-gowdk-bind-checked");
      const checked = Boolean(state[field]);
      if (node.checked !== checked) node.checked = checked;
    });
    root.querySelectorAll("*").forEach((node) => {
      Array.from(node.attributes).forEach((attr) => {
        if (!attr.name.startsWith("data-gowdk-class-")) return;
        const name = attr.name.slice("data-gowdk-class-".length);
        node.classList.toggle(name, Boolean(valueOf(attr.value, state, null, helpers)));
      });
    });
    root.querySelectorAll("*").forEach((node) => {
      Array.from(node.attributes).forEach((attr) => {
        if (!attr.name.startsWith("data-gowdk-style-") || attr.name.startsWith("data-gowdk-style-unit-")) return;
        const name = attr.name.slice("data-gowdk-style-".length);
        const unit = node.getAttribute("data-gowdk-style-unit-" + name) || "";
        const value = valueOf(attr.value, state, null, helpers);
        if (value == null || value === false || value === "") node.style.removeProperty(name);
        else node.style.setProperty(name, String(value) + unit);
      });
    });
    root.querySelectorAll("*").forEach((node) => {
      Array.from(node.attributes).forEach((attr) => {
        if (!attr.name.startsWith("data-gowdk-attr-")) return;
        const name = attr.name.slice("data-gowdk-attr-".length);
        const value = valueOf(attr.value, state, null, helpers);
        if (booleanAttrs.has(name)) {
          if (Boolean(value)) node.setAttribute(name, "");
          else node.removeAttribute(name);
          return;
        }
        if (value == null || value === false) node.removeAttribute(name);
        else node.setAttribute(name, String(value));
      });
    });
    root.setAttribute("data-gowdk-state", JSON.stringify(state));
  }

  document.querySelectorAll(selector).forEach((root) => {
    const state = JSON.parse(root.getAttribute("data-gowdk-state") || "{}");
    const client = JSON.parse(root.getAttribute("data-gowdk-client") || "{}");
    const hasEnvelope = Boolean(client.handlers || client.helpers || client.mount || client.destroy || client.effects || client.computed);
    const handlers = hasEnvelope ? (client.handlers || {}) : client;
    const helpers = client.helpers || {};
    const mountStatements = client.mount || [];
    const destroyStatements = client.destroy || [];
    const effects = client.effects || [];
    const computeds = client.computed || [];
    const refs = Object.create(null);
    recomputeComputed(state, computeds, helpers);
    root.querySelectorAll("[data-gowdk-ref]").forEach((node) => {
      refs[node.getAttribute("data-gowdk-ref")] = node;
    });
    const effectValues = Object.create(null);
    const effectCleanups = Object.create(null);
    effects.forEach((effect) => {
      effectValues[effect.field] = state[effect.field];
    });
    const runEffectCleanup = (effect) => {
      const cleanup = effectCleanups[effect.field];
      if (!cleanup || cleanup.length === 0) return;
      effectCleanups[effect.field] = null;
      applyStatements(cleanup, state, handlers, helpers, null, refs, computeds);
    };
    const runAllEffectCleanups = () => {
      effects.forEach((effect) => runEffectCleanup(effect));
    };
    const settleEffects = () => {
      for (let pass = 0; pass < 10; pass++) {
        let ran = false;
        effects.forEach((effect) => {
          const current = state[effect.field];
          if (Object.is(effectValues[effect.field], current)) return;
          runEffectCleanup(effect);
          effectValues[effect.field] = current;
          applyStatements(effect.statements, state, handlers, helpers, null, refs, computeds);
          effectCleanups[effect.field] = effect.cleanup || null;
          ran = true;
        });
        if (!ran) return;
      }
    };
    const rerender = () => {
      render(root, state, helpers);
      bindInteractiveNodes();
    };
    const bindInteractiveNodes = () => {
      root.querySelectorAll("*").forEach((node) => {
        if (node.hasAttribute("data-gowdk-bind-value") && !node.hasAttribute("data-gowdk-bound-value")) {
          node.setAttribute("data-gowdk-bound-value", "");
          const field = node.getAttribute("data-gowdk-bind-value");
          const type = node.getAttribute("data-gowdk-bind-type") || "string";
          const event = node.tagName === "SELECT" || node.type === "radio" ? "change" : "input";
          node.addEventListener(event, () => {
            if (node.type === "radio") {
              if (!node.checked) return;
              state[field] = node.value;
            } else if (type === "int") {
              const next = parseInt(node.value, 10);
              state[field] = Number.isNaN(next) ? 0 : next;
            } else if (type === "float") {
              const next = parseFloat(node.value);
              state[field] = Number.isNaN(next) ? 0 : next;
            } else {
              state[field] = node.value;
            }
            recomputeComputed(state, computeds, helpers);
            settleEffects();
            recomputeComputed(state, computeds, helpers);
            rerender();
          });
        }
        if (node.hasAttribute("data-gowdk-bind-checked") && !node.hasAttribute("data-gowdk-bound-checked")) {
          node.setAttribute("data-gowdk-bound-checked", "");
          const field = node.getAttribute("data-gowdk-bind-checked");
          node.addEventListener("change", () => {
            state[field] = node.checked;
            recomputeComputed(state, computeds, helpers);
            settleEffects();
            recomputeComputed(state, computeds, helpers);
            rerender();
          });
        }
        Array.from(node.attributes).forEach((attr) => {
          if (!attr.name.startsWith("data-gowdk-on-")) return;
          const event = attr.name.slice("data-gowdk-on-".length);
          const boundAttr = "data-gowdk-bound-on-" + event;
          if (node.hasAttribute(boundAttr)) return;
          node.setAttribute(boundAttr, "");
          const modifiers = eventModifiers(node.getAttribute("data-gowdk-event-" + event));
          let debounceTimer = 0;
          let throttleUntil = 0;
          const invoke = () => {
            applyExpression(attr.value, state, handlers, helpers, null, refs, computeds);
            recomputeComputed(state, computeds, helpers);
            settleEffects();
            recomputeComputed(state, computeds, helpers);
            rerender();
          };
          const listener = (domEvent) => {
            if (modifiers.prevent) domEvent.preventDefault();
            if (modifiers.stop) domEvent.stopPropagation();
            if (modifiers.debounce > 0) {
              clearTimeout(debounceTimer);
              debounceTimer = setTimeout(invoke, modifiers.debounce);
              return;
            }
            if (modifiers.throttle > 0) {
              const now = Date.now();
              if (now < throttleUntil) return;
              throttleUntil = now + modifiers.throttle;
            }
            invoke();
          };
          node.addEventListener(event, listener, { once: modifiers.once, capture: modifiers.capture });
        });
      });
    };
    applyStatements(mountStatements, state, handlers, helpers, null, refs, computeds);
    settleEffects();
    recomputeComputed(state, computeds, helpers);
    if (destroyStatements.length > 0) {
      window.addEventListener("pagehide", () => {
        runAllEffectCleanups();
        applyStatements(destroyStatements, state, handlers, helpers, null, refs, computeds);
      }, { once: true });
    } else if (effects.length > 0) {
      window.addEventListener("pagehide", () => {
        runAllEffectCleanups();
      }, { once: true });
    }
    rerender();
  });
})();
`, component)
}

func islandWASMLoaderSource(componentName string) string {
	component := strconv.Quote(componentName)
	wasmPath := strconv.Quote("/" + islandWASMAssetPath(componentName))
	return fmt.Sprintf(`(() => {
  const component = %s;
  const wasmPath = %s;
  const roots = document.querySelectorAll("gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"wasm\"]");
  if (roots.length === 0 || typeof WebAssembly === "undefined") return;
  WebAssembly.instantiateStreaming(fetch(wasmPath), {}).catch(() => {});
})();
`, component, wasmPath)
}

func writeAssetManifest(outputDir string, cssArtifacts []CSSArtifact, assetArtifacts []AssetArtifact) (string, error) {
	payload, err := assetManifestPayload(outputDir, cssArtifacts, assetArtifacts)
	if err != nil {
		return "", err
	}

	manifestPath := filepath.Join(outputDir, assetManifestFile)
	if err := writeFileIfChanged(manifestPath, payload); err != nil {
		return "", err
	}
	return manifestPath, nil
}

func assetManifestPayload(outputDir string, cssArtifacts []CSSArtifact, assetArtifacts []AssetArtifact) ([]byte, error) {
	files := make(map[string]string, len(cssArtifacts)+len(assetArtifacts))
	for _, artifact := range cssArtifacts {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return nil, err
		}
		files[rel] = rel
	}
	for _, artifact := range assetArtifacts {
		rel, err := relativeOutputPath(outputDir, artifact.Path)
		if err != nil {
			return nil, err
		}
		files[rel] = rel
	}

	payload, err := json.MarshalIndent(runtimeasset.Manifest{Version: 1, Files: files}, "", "  ")
	if err != nil {
		return nil, err
	}
	payload = append(payload, '\n')
	return payload, nil
}

func writeFileIfChanged(filePath string, contents []byte) error {
	current, err := os.ReadFile(filePath)
	if err == nil && bytes.Equal(current, contents) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(filePath, contents, 0o644)
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

		props, propFailures := componentPropNames(component)
		for _, failure := range propFailures {
			failures = append(failures, failure)
			valid = false
		}
		state, stateTypes, stateJSON, err := componentInitialState(component)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s state: %v", component.Name, err))
			valid = false
		}
		handlers, handlersJSON, err := componentClientHandlers(component)
		if err != nil {
			failures = append(failures, fmt.Sprintf("component %s client: %v", component.Name, err))
			valid = false
		}
		refs, refFailures := componentClientRefs(component)
		for _, failure := range refFailures {
			failures = append(failures, failure)
			valid = false
		}
		computeds, computedFailures := componentClientComputeds(component)
		for _, failure := range computedFailures {
			failures = append(failures, failure)
			valid = false
		}
		if !valid {
			continue
		}
		registry[component.Name] = view.Component{
			Name:         component.Name,
			Props:        props,
			State:        state,
			StateJSON:    stateJSON,
			Handlers:     handlers,
			HandlersJSON: handlersJSON,
			StateTypes:   stateTypes,
			Refs:         refs,
			Computed:     computeds,
			Body:         component.Blocks.ViewBody,
		}
	}
	return registry, failures
}

func componentClientComputeds(component manifest.Component) ([]clientlang.Computed, []string) {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return nil, nil
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return nil, []string{fmt.Sprintf("component %s client: %v", component.Name, err)}
	}
	computeds, err := program.OrderedComputed()
	if err != nil {
		return nil, []string{fmt.Sprintf("component %s computed dependency graph: %v", component.Name, err)}
	}
	return computeds, nil
}

func componentClientRefs(component manifest.Component) (map[string]clientlang.Ref, []string) {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return nil, nil
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return nil, []string{fmt.Sprintf("component %s client: %v", component.Name, err)}
	}
	return program.RefMap(), nil
}

func componentClientHandlers(component manifest.Component) (map[string]clientlang.Handler, string, error) {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return nil, "", nil
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return nil, "", err
	}
	handlers := program.HandlerMap()
	helpers := program.HelperMap()
	if len(handlers) == 0 && len(helpers) == 0 && !program.NeedsBootstrap() {
		return nil, "", nil
	}
	computeds, err := program.OrderedComputed()
	if err != nil {
		return nil, "", err
	}
	var payload []byte
	if program.NeedsBootstrap() {
		payload, err = json.Marshal(clientlang.Bootstrap{
			Handlers: handlers,
			Helpers:  helpers,
			Mount:    append([]string(nil), program.Mount...),
			Destroy:  append([]string(nil), program.Destroy...),
			Effects:  append([]clientlang.Effect(nil), program.Effects...),
			Computed: computeds,
		})
	} else {
		payload, err = json.Marshal(handlers)
	}
	if err != nil {
		return nil, "", err
	}
	return handlers, string(payload), nil
}

func componentPropNames(component manifest.Component) ([]string, []string) {
	if component.PropsType.Name != "" {
		resolved, err := gotypes.ResolveStruct(component.Imports, component.PropsType)
		if err != nil {
			return nil, []string{fmt.Sprintf("component %s props: %v", component.Name, err)}
		}
		return resolved.FieldNames(), nil
	}
	props := make([]string, 0, len(component.Props))
	seen := map[string]bool{}
	var failures []string
	for _, prop := range component.Props {
		if prop.Type != "string" {
			failures = append(failures, fmt.Sprintf("component %s prop %s uses unsupported type %q", component.Name, prop.Name, prop.Type))
			continue
		}
		if seen[prop.Name] {
			failures = append(failures, fmt.Sprintf("component %s declares duplicate prop %q", component.Name, prop.Name))
			continue
		}
		seen[prop.Name] = true
		props = append(props, prop.Name)
	}
	return props, failures
}

func componentInitialState(component manifest.Component) (map[string]string, map[string]clientlang.ValueType, string, error) {
	if component.State.Type.Name == "" {
		return nil, nil, "", nil
	}
	resolved, err := gotypes.ResolveStruct(component.Imports, component.State.Type)
	if err != nil {
		return nil, nil, "", err
	}
	rawJSON, err := gotypes.RunStateInitJSON(component.Imports, component.State)
	if err != nil {
		return nil, nil, "", err
	}
	var raw map[string]any
	if err := json.Unmarshal(rawJSON, &raw); err != nil {
		return nil, nil, "", fmt.Errorf("decode state JSON: %w", err)
	}
	state := map[string]string{}
	stateTypes := map[string]clientlang.ValueType{}
	for _, field := range resolved.Fields {
		value, ok := raw[field.Name]
		if !ok {
			return nil, nil, "", fmt.Errorf("init JSON missing field %q", field.Name)
		}
		scalar, ok := stateValueString(value)
		if !ok {
			return nil, nil, "", fmt.Errorf("field %s must initialize to JSON-compatible state", field.Name)
		}
		state[field.Name] = scalar
		stateTypes[field.Name] = clientlang.NormalizeType(field.Type)
	}
	for field, typ := range resolved.FieldTypes {
		stateTypes[field] = clientlang.NormalizeType(typ)
	}
	return state, stateTypes, string(rawJSON), nil
}

func stateValueString(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", true
	case string:
		return typed, true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	case []any, map[string]any:
		payload, err := json.Marshal(typed)
		if err != nil {
			return "", false
		}
		return string(payload), true
	default:
		return "", false
	}
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

func isComponentName(value string) bool {
	if value == "" {
		return false
	}
	first := []rune(value)[0]
	return first >= 'A' && first <= 'Z'
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

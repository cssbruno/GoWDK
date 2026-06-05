// Package playground exposes an in-memory compiler suitable for browser
// playgrounds and WASM wrappers.
package playground

import (
	"path"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/manifest"
)

const defaultOutputDir = "dist/site"

// Project is a browser-compiler input. Files maps project-relative paths to
// file contents. The current browser compiler accepts .gwdk page, component,
// and layout files.
type Project struct {
	Files     map[string]string `json:"files"`
	Config    gowdk.Config      `json:"-"`
	OutputDir string            `json:"outputDir,omitempty"`
}

// Diagnostic is a source diagnostic safe to serialize to browser clients.
type Diagnostic struct {
	File     string   `json:"file"`
	Code     string   `json:"code,omitempty"`
	Pos      Position `json:"pos"`
	Range    *Range   `json:"range,omitempty"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
}

// Position is a 1-based source location.
type Position struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// Range is a 1-based source range. End is exclusive.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Route describes one app-shell HTML route produced by a compile.
type Route struct {
	PageID string `json:"page"`
	Route  string `json:"route"`
	Path   string `json:"path"`
}

// Result is the browser-compiler output.
type Result struct {
	Files       map[string]string `json:"files"`
	HTML        map[string]string `json:"html"`
	CSS         map[string]string `json:"css"`
	Routes      []Route           `json:"routes"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
}

// Compile parses, validates, and renders a project at build time without writing
// files. It is intended for playgrounds; generated apps, binaries, request-time
// SSR, and action execution remain native/server compiler features.
func Compile(project Project) Result {
	config := browserConfig(project.Config)
	outputDir := strings.TrimSpace(project.OutputDir)
	if outputDir == "" {
		outputDir = defaultOutputDir
	}

	app, diagnostics := parseProject(project.Files)
	if diagnostics.HasErrors() {
		return Result{Diagnostics: diagnosticsForBrowser(diagnostics)}
	}
	if len(app.Pages) == 0 {
		return Result{Diagnostics: []Diagnostic{{
			Severity: "error",
			Message:  "project must include at least one page file",
		}}}
	}

	memory, err := buildgen.BuildMemory(config, app, outputDir)
	if err != nil {
		return Result{Diagnostics: diagnosticsForBuildError(err, app)}
	}

	result := Result{
		Files:  map[string]string{},
		HTML:   map[string]string{},
		CSS:    map[string]string{},
		Routes: make([]Route, 0, len(memory.Artifacts)),
	}
	for filePath, contents := range memory.Files {
		text := string(contents)
		result.Files[filePath] = text
		switch {
		case strings.HasSuffix(filePath, ".html"):
			result.HTML[filePath] = text
		case strings.HasSuffix(filePath, ".css"):
			result.CSS[filePath] = text
		}
	}
	for _, artifact := range memory.Artifacts {
		result.Routes = append(result.Routes, Route{
			PageID: artifact.PageID,
			Route:  artifact.Route,
			Path:   relativeOutputPath(outputDir, artifact.Path),
		})
	}
	sort.Slice(result.Routes, func(i, j int) bool {
		if result.Routes[i].Route == result.Routes[j].Route {
			return result.Routes[i].PageID < result.Routes[j].PageID
		}
		return result.Routes[i].Route < result.Routes[j].Route
	})
	return result
}

func browserConfig(config gowdk.Config) gowdk.Config {
	if len(config.CSS.Include) == 0 {
		config.CSS.Include = []string{buildgen.DisableCSSDiscovery}
	}
	return config
}

func parseProject(files map[string]string) (manifest.Manifest, lang.Diagnostics) {
	var app manifest.Manifest
	var diagnostics lang.Diagnostics
	for _, filePath := range sortedFilePaths(files) {
		source := []byte(files[filePath])
		switch lang.ClassifySource(filePath, source) {
		case lang.FileKindComponent:
			component, fileDiagnostics := lang.ParseComponentSource(filePath, source)
			diagnostics = append(diagnostics, fileDiagnostics...)
			if !fileDiagnostics.HasErrors() {
				app.Components = append(app.Components, component)
			}
		case lang.FileKindLayout:
			layout, fileDiagnostics := lang.ParseLayoutSource(filePath, source)
			diagnostics = append(diagnostics, fileDiagnostics...)
			if !fileDiagnostics.HasErrors() {
				app.Layouts = append(app.Layouts, layout)
			}
		case lang.FileKindAsset, lang.FileKindPlugin:
			continue
		default:
			page, fileDiagnostics := lang.ParseSource(filePath, source)
			diagnostics = append(diagnostics, fileDiagnostics...)
			if !fileDiagnostics.HasErrors() {
				app.Pages = append(app.Pages, page)
			}
		}
	}
	return app, diagnostics
}

func sortedFilePaths(files map[string]string) []string {
	paths := make([]string, 0, len(files))
	cleanToOriginal := map[string]string{}
	for filePath := range files {
		if strings.TrimSpace(filePath) == "" {
			continue
		}
		clean := path.Clean(strings.TrimPrefix(filePath, "/"))
		paths = append(paths, clean)
		cleanToOriginal[clean] = filePath
	}
	sort.Strings(paths)
	for index, clean := range paths {
		paths[index] = cleanToOriginal[clean]
	}
	return paths
}

func diagnosticsForBrowser(diagnostics lang.Diagnostics) []Diagnostic {
	out := make([]Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, Diagnostic{
			File:     diagnostic.File,
			Code:     diagnostic.Code,
			Pos:      browserPosition(diagnostic.Pos),
			Range:    browserRange(diagnostic.Range),
			Severity: diagnostic.Severity,
			Message:  diagnostic.Message,
		})
	}
	return out
}

func diagnosticsForBuildError(err error, app manifest.Manifest) []Diagnostic {
	if validation, ok := err.(compiler.ValidationErrors); ok {
		return diagnosticsForBrowser(compilerDiagnostics(validation, app))
	}
	return []Diagnostic{{Severity: "error", Message: err.Error()}}
}

func compilerDiagnostics(err compiler.ValidationErrors, app manifest.Manifest) lang.Diagnostics {
	sources := map[string]string{}
	for _, page := range app.Pages {
		if page.Source != "" && sources[page.ID] == "" {
			sources[page.ID] = page.Source
		}
	}

	diagnostics := make(lang.Diagnostics, 0, len(err))
	for _, validation := range err {
		diagnostics = append(diagnostics, lang.Diagnostic{
			File:     validationSource(validation, sources),
			Code:     validation.Code,
			Pos:      sourcePosition(validation.Span.Start),
			Range:    sourceSpanRange(validation.Span),
			Severity: "error",
			Message:  validation.Error(),
		})
	}
	return diagnostics
}

func validationSource(validation compiler.ValidationError, sources map[string]string) string {
	if validation.Source != "" {
		return validation.Source
	}
	if validation.PageID != "" {
		return sources[validation.PageID]
	}
	return ""
}

func sourcePosition(position manifest.SourcePosition) lang.Position {
	return lang.Position{Line: position.Line, Column: position.Column}
}

func sourceSpanRange(span manifest.SourceSpan) *lang.Range {
	if span.Start.Line <= 0 || span.Start.Column <= 0 || span.End.Line <= 0 || span.End.Column <= 0 {
		return nil
	}
	return &lang.Range{
		Start: sourcePosition(span.Start),
		End:   sourcePosition(span.End),
	}
}

func browserPosition(position lang.Position) Position {
	return Position{Line: position.Line, Column: position.Column}
}

func browserRange(sourceRange *lang.Range) *Range {
	if sourceRange == nil {
		return nil
	}
	return &Range{
		Start: browserPosition(sourceRange.Start),
		End:   browserPosition(sourceRange.End),
	}
}

func relativeOutputPath(outputDir, filePath string) string {
	prefix := strings.Trim(path.Clean(strings.ReplaceAll(outputDir, "\\", "/")), "/")
	clean := strings.Trim(path.Clean(strings.ReplaceAll(filePath, "\\", "/")), "/")
	if prefix != "" && strings.HasPrefix(clean, prefix+"/") {
		return strings.TrimPrefix(clean, prefix+"/")
	}
	return clean
}

package lang

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/parser"
)

var parserLinePattern = regexp.MustCompile(`^line ([0-9]+): `)

// FileKind identifies the current source file category.
type FileKind string

const (
	FileKindPage      FileKind = "page"
	FileKindComponent FileKind = "component"
	FileKindLayout    FileKind = "layout"
	FileKindAsset     FileKind = "asset"
	FileKindPlugin    FileKind = "plugin"
)

// ParseFile reads and parses one .gwdk file.
func ParseFile(path string) (manifest.Page, Diagnostics) {
	source, err := os.ReadFile(path)
	if err != nil {
		return manifest.Page{}, Diagnostics{{
			File:     path,
			Severity: "error",
			Message:  err.Error(),
		}}
	}

	return ParseSource(path, source)
}

// ParseSource parses one .gwdk source buffer.
func ParseSource(path string, source []byte) (manifest.Page, Diagnostics) {
	_, diagnostics := Lex(string(source))
	for i := range diagnostics {
		diagnostics[i].File = path
	}
	if diagnostics.HasErrors() {
		return manifest.Page{}, diagnostics
	}

	page, err := parser.ParsePage(source)
	page.Source = path
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{
			File:     path,
			Code:     parserErrorCode(err.Error()),
			Pos:      parserErrorPosition(err.Error()),
			Range:    parserErrorRange(source, err.Error()),
			Severity: "error",
			Message:  err.Error(),
		})
	}
	return page, diagnostics
}

func parserErrorCode(message string) string {
	switch {
	case strings.Contains(message, "old action block syntax"):
		return "old_action_block_syntax"
	case strings.Contains(message, "old API block syntax"):
		return "old_api_block_syntax"
	case strings.Contains(message, "package declaration must be the first"):
		return "package_must_be_first"
	case strings.Contains(message, "malformed use"):
		return "malformed_gowdk_use"
	default:
		return "parse_error"
	}
}

func parserErrorPosition(message string) Position {
	match := parserLinePattern.FindStringSubmatch(message)
	if match == nil {
		return Position{}
	}
	line, err := strconv.Atoi(match[1])
	if err != nil {
		return Position{}
	}
	return Position{Line: line, Column: 1}
}

func parserErrorRange(source []byte, message string) *Range {
	position := parserErrorPosition(message)
	if position.Line <= 0 {
		return nil
	}
	lines := strings.Split(string(source), "\n")
	if position.Line > len(lines) {
		return sourceRange(position, Position{Line: position.Line, Column: 2})
	}
	endColumn := len([]rune(lines[position.Line-1])) + 1
	if endColumn <= 1 {
		endColumn = 2
	}
	return sourceRange(position, Position{Line: position.Line, Column: endColumn})
}

// ParseFiles parses multiple .gwdk files into a manifest.
func ParseFiles(paths []string) (manifest.Manifest, Diagnostics) {
	return ParseBuildFiles(paths)
}

// ParseBuildFiles parses explicit page and component files for build commands.
func ParseBuildFiles(paths []string) (manifest.Manifest, Diagnostics) {
	var app manifest.Manifest
	var diagnostics Diagnostics
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				File:     path,
				Severity: "error",
				Message:  err.Error(),
			})
			continue
		}
		switch ClassifySource(path, source) {
		case FileKindComponent:
			component, fileDiagnostics := ParseComponentSource(path, source)
			diagnostics = append(diagnostics, fileDiagnostics...)
			if !fileDiagnostics.HasErrors() {
				app.Components = append(app.Components, component)
			}
			continue
		case FileKindLayout:
			layout, fileDiagnostics := ParseLayoutSource(path, source)
			diagnostics = append(diagnostics, fileDiagnostics...)
			if !fileDiagnostics.HasErrors() {
				app.Layouts = append(app.Layouts, layout)
			}
			continue
		case FileKindAsset, FileKindPlugin:
			continue
		}
		page, fileDiagnostics := ParseSource(path, source)
		diagnostics = append(diagnostics, fileDiagnostics...)
		if !fileDiagnostics.HasErrors() {
			app.Pages = append(app.Pages, page)
		}
	}
	return app, diagnostics
}

// ParseLayoutSource parses one in-memory .layout.gwdk source buffer.
func ParseLayoutSource(path string, source []byte) (manifest.Layout, Diagnostics) {
	_, diagnostics := Lex(string(source))
	for i := range diagnostics {
		diagnostics[i].File = path
	}
	if diagnostics.HasErrors() {
		return manifest.Layout{}, diagnostics
	}

	layout, err := parser.ParseLayout(source)
	layout.Source = path
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{
			File:     path,
			Code:     parserErrorCode(err.Error()),
			Pos:      parserErrorPosition(err.Error()),
			Range:    parserErrorRange(source, err.Error()),
			Severity: "error",
			Message:  err.Error(),
		})
	}
	return layout, diagnostics
}

// ParseComponentSource parses one in-memory .cmp.gwdk source buffer.
func ParseComponentSource(path string, source []byte) (manifest.Component, Diagnostics) {
	_, diagnostics := Lex(string(source))
	for i := range diagnostics {
		diagnostics[i].File = path
	}
	if diagnostics.HasErrors() {
		return manifest.Component{}, diagnostics
	}

	component, err := parser.ParseComponent(source)
	component.Source = path
	if err != nil {
		diagnostics = append(diagnostics, Diagnostic{
			File:     path,
			Code:     parserErrorCode(err.Error()),
			Pos:      parserErrorPosition(err.Error()),
			Range:    parserErrorRange(source, err.Error()),
			Severity: "error",
			Message:  err.Error(),
		})
	}
	return component, diagnostics
}

// ClassifySource classifies a .gwdk source file using current file-kind rules.
func ClassifySource(path string, source []byte) FileKind {
	base := filepath.Base(path)
	if strings.HasSuffix(base, ".cmp.gwdk") {
		return FileKindComponent
	}
	if strings.HasSuffix(base, ".layout.gwdk") {
		return FileKindLayout
	}
	if strings.HasSuffix(base, ".asset.gwdk") {
		return FileKindAsset
	}
	if strings.HasSuffix(base, ".plugin.gwdk") {
		return FileKindPlugin
	}
	for _, line := range strings.Split(string(source), "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "//") {
			continue
		}
		switch {
		case isAnnotation(text, "@page"):
			return FileKindPage
		case isAnnotation(text, "@component"):
			return FileKindComponent
		case isAnnotation(text, "@layout"):
			return FileKindLayout
		case isAnnotation(text, "@asset"):
			return FileKindAsset
		case isAnnotation(text, "@plugin"):
			return FileKindPlugin
		}
	}
	return FileKindPage
}

func isAnnotation(text, annotation string) bool {
	if !strings.HasPrefix(text, annotation) {
		return false
	}
	if len(text) == len(annotation) {
		return true
	}
	next := text[len(annotation)]
	return next == ' ' || next == '\t'
}

// CheckFiles parses and validates .gwdk files.
func CheckFiles(config gowdk.Config, paths []string) (manifest.Manifest, Diagnostics) {
	app, diagnostics := ParseFiles(paths)
	if diagnostics.HasErrors() {
		return app, diagnostics
	}
	if err := compiler.ValidateManifest(config, app); err != nil {
		diagnostics = append(diagnostics, compilerDiagnostics(err, app)...)
	}
	if !diagnostics.HasErrors() {
		app = compiler.BindBackendHandlers(app)
	}
	return app, diagnostics
}

// CheckSource parses and validates one in-memory .gwdk source buffer.
func CheckSource(config gowdk.Config, path string, source []byte) (manifest.Page, Diagnostics) {
	switch ClassifySource(path, source) {
	case FileKindComponent:
		component, diagnostics := ParseComponentSource(path, source)
		if diagnostics.HasErrors() {
			return manifest.Page{}, diagnostics
		}
		app := manifest.Manifest{Components: []manifest.Component{component}}
		if err := compiler.ValidateManifest(config, app); err != nil {
			diagnostics = append(diagnostics, compilerDiagnostics(err, app)...)
		}
		return manifest.Page{}, diagnostics
	case FileKindLayout:
		_, diagnostics := ParseLayoutSource(path, source)
		return manifest.Page{}, diagnostics
	case FileKindAsset, FileKindPlugin:
		_, diagnostics := Lex(string(source))
		for i := range diagnostics {
			diagnostics[i].File = path
		}
		return manifest.Page{}, diagnostics
	}

	page, diagnostics := ParseSource(path, source)
	if diagnostics.HasErrors() {
		return page, diagnostics
	}
	app := manifest.Manifest{Pages: []manifest.Page{page}}
	if err := compiler.ValidateManifest(config, app); err != nil {
		diagnostics = append(diagnostics, compilerDiagnostics(err, app)...)
	}
	return page, diagnostics
}

// DiagnosticReport is the JSON shape consumed by editor integrations.
type DiagnosticReport struct {
	Diagnostics Diagnostics `json:"diagnostics"`
}

// CheckJSON returns editor-friendly JSON diagnostics for parsed files.
func CheckJSON(config gowdk.Config, paths []string) ([]byte, Diagnostics) {
	_, diagnostics := CheckFiles(config, paths)
	payload, err := json.MarshalIndent(DiagnosticReport{Diagnostics: diagnostics}, "", "  ")
	if err != nil {
		return nil, Diagnostics{{Severity: "error", Message: err.Error()}}
	}
	return append(payload, '\n'), diagnostics
}

// ManifestJSON returns the manifest JSON for parsed and validated files.
func ManifestJSON(config gowdk.Config, paths []string) ([]byte, Diagnostics) {
	app, diagnostics := CheckFiles(config, paths)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	app = applyDefaultRenderMode(app, config.Render.DefaultMode())
	payload, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return nil, Diagnostics{{Severity: "error", Message: err.Error()}}
	}
	return append(payload, '\n'), diagnostics
}

func applyDefaultRenderMode(app manifest.Manifest, defaultMode gowdk.RenderMode) manifest.Manifest {
	if defaultMode == "" || defaultMode == gowdk.SPA {
		return app
	}
	pages := append([]manifest.Page(nil), app.Pages...)
	for i := range pages {
		if pages[i].Render == "" {
			pages[i].Render = defaultMode
		}
	}
	app.Pages = pages
	return app
}

func compilerDiagnostics(err error, app manifest.Manifest) Diagnostics {
	sources := pageSources(app)
	switch typed := err.(type) {
	case compiler.ValidationErrors:
		diagnostics := make(Diagnostics, 0, len(typed))
		for _, validation := range typed {
			diagnostics = append(diagnostics, Diagnostic{
				File:       diagnosticSource(validation, sources),
				Code:       validation.Code,
				Pos:        sourcePosition(validation.Span.Start),
				Range:      sourceSpanRange(validation.Span),
				Severity:   "error",
				Message:    validation.Error(),
				Suggestion: diagnosticSuggestion(validation),
			})
		}
		return diagnostics
	default:
		return Diagnostics{{Severity: "error", Message: fmt.Sprint(err)}}
	}
}

func diagnosticSuggestion(validation compiler.ValidationError) string {
	message := validation.Message
	switch validation.Code {
	case "missing_package_declaration":
		return "Add package <name> before annotations, imports, and blocks."
	case "package_mismatch":
		return "Use the same package name as sibling .go files in this directory."
	case "go_package_error":
		return "Fix the sibling Go package before running GOWDK validation."
	case "duplicate_gowdk_use_alias":
		return "Use each GOWDK package alias only once in this file."
	case "unknown_gowdk_use_package":
		return "Make sure a discovered .cmp.gwdk file declares that package, or remove the use declaration."
	case "unknown_gowdk_use_alias":
		return "Declare the GOWDK package alias with use alias \"package\" before using qualified component tags."
	case "unknown_gowdk_component":
		return "Use a component exported by the imported GOWDK package, or fix the package alias."
	case "unsupported_gowdk_use_scope":
		return "Move this use declaration to the page that calls the imported component, or keep the component in the same package."
	case "missing_ssr_addon":
		return "Enable ssr.Addon() in gowdk.config.go or change the page render mode."
	case "spa_dynamic_route_missing_paths":
		return "Add paths { ... } for the dynamic spa route or switch the page to @render ssr."
	case "load_requires_request_render":
		return "Use @render ssr or @render hybrid for pages with load { ... }."
	case "component_client_error":
		if strings.Contains(message, "unknown island field") || strings.Contains(message, "unknown field") {
			return "Use a field declared by the component props/state contract, a local variable, or a computed value."
		}
		if strings.Contains(message, "await is only supported inside async client functions") {
			return "Mark the client function as async fn or remove await."
		}
		if strings.Contains(message, "unknown component event") {
			return "Declare the event in emits { ... } before emitting it."
		}
	case "component_field_error":
		if strings.Contains(message, "unknown view field") {
			return "Bind only declared props/state/computed fields or loop variables in view { ... }."
		}
		if strings.Contains(message, "unknown client function") {
			return "Declare a matching fn in client { ... } or use a supported inline state expression."
		}
		if strings.Contains(message, "g:for must use") {
			return "Use g:for={item in Items} or g:for={item, index in Items}."
		}
		if strings.Contains(message, "g:for requires g:key") {
			return "Add g:key with a stable scalar expression, such as g:key={item.ID}."
		}
	}
	return ""
}

func sourcePosition(position manifest.SourcePosition) Position {
	return Position{Line: position.Line, Column: position.Column}
}

func sourceSpanRange(span manifest.SourceSpan) *Range {
	if span.Start.Line <= 0 || span.Start.Column <= 0 || span.End.Line <= 0 || span.End.Column <= 0 {
		return nil
	}
	return sourceRange(sourcePosition(span.Start), sourcePosition(span.End))
}

func diagnosticSource(validation compiler.ValidationError, sources map[string]string) string {
	if validation.Source != "" {
		return validation.Source
	}
	if validation.PageID != "" {
		return sources[validation.PageID]
	}
	return ""
}

func pageSources(app manifest.Manifest) map[string]string {
	sources := map[string]string{}
	for _, page := range app.Pages {
		if page.Source != "" && sources[page.ID] == "" {
			sources[page.ID] = page.Source
		}
	}
	for _, component := range app.Components {
		if component.Source != "" && sources[component.Name] == "" {
			sources[component.Name] = component.Source
		}
	}
	return sources
}

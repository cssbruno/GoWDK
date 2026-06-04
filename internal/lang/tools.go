package lang

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/internal/compiler"
	"github.com/gowdk/gowdk/internal/manifest"
	"github.com/gowdk/gowdk/internal/parser"
)

var parserLinePattern = regexp.MustCompile(`^line ([0-9]+): `)

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
			Pos:      parserErrorPosition(err.Error()),
			Severity: "error",
			Message:  err.Error(),
		})
	}
	return page, diagnostics
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
		if isComponentFile(path, source) {
			component, fileDiagnostics := ParseComponentSource(path, source)
			diagnostics = append(diagnostics, fileDiagnostics...)
			if !fileDiagnostics.HasErrors() {
				app.Components = append(app.Components, component)
			}
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
			Pos:      parserErrorPosition(err.Error()),
			Severity: "error",
			Message:  err.Error(),
		})
	}
	return component, diagnostics
}

func isComponentFile(path string, source []byte) bool {
	if strings.HasSuffix(filepath.Base(path), ".cmp.gwdk") {
		return true
	}
	return strings.Contains(string(source), "@component")
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
	return app, diagnostics
}

// CheckSource parses and validates one in-memory .gwdk source buffer.
func CheckSource(config gowdk.Config, path string, source []byte) (manifest.Page, Diagnostics) {
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
	payload, err := json.MarshalIndent(app, "", "  ")
	if err != nil {
		return nil, Diagnostics{{Severity: "error", Message: err.Error()}}
	}
	return append(payload, '\n'), diagnostics
}

func compilerDiagnostics(err error, app manifest.Manifest) Diagnostics {
	sources := pageSources(app)
	switch typed := err.(type) {
	case compiler.ValidationErrors:
		diagnostics := make(Diagnostics, 0, len(typed))
		for _, validation := range typed {
			diagnostics = append(diagnostics, Diagnostic{
				File:     diagnosticSource(validation, sources),
				Severity: "error",
				Message:  validation.Error(),
			})
		}
		return diagnostics
	default:
		return Diagnostics{{Severity: "error", Message: fmt.Sprint(err)}}
	}
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
	return sources
}

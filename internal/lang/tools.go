package lang

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/contractscan"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/parser"
	"github.com/cssbruno/gowdk/internal/source"
)

// FileKind identifies the current source file category.
type FileKind string

const (
	FileKindPage      FileKind = "page"
	FileKindComponent FileKind = "component"
	FileKindLayout    FileKind = "layout"
	FileKindAsset     FileKind = "asset"
)

// ParseFile reads and parses one .gwdk file.
func ParseFile(path string) (gwdkir.Page, Diagnostics) {
	source, err := os.ReadFile(path)
	if err != nil {
		return gwdkir.Page{}, Diagnostics{{
			File:     path,
			Severity: "error",
			Message:  err.Error(),
		}}
	}

	return ParseSource(path, source)
}

// ParseSource parses one .gwdk source buffer.
func ParseSource(path string, source []byte) (gwdkir.Page, Diagnostics) {
	_, diagnostics := Lex(string(source))
	for i := range diagnostics {
		diagnostics[i].File = path
	}
	if diagnostics.HasErrors() {
		return gwdkir.Page{}, diagnostics
	}

	page, err := parser.ParsePageWithDefaultID(source, derivedPageID(path))
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
	lineText, ok := strings.CutPrefix(message, "line ")
	if !ok {
		return Position{}
	}
	lineText, _, ok = strings.Cut(lineText, ": ")
	if !ok {
		return Position{}
	}
	line, err := strconv.Atoi(lineText)
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

func derivedPageID(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	base := filepath.Base(path)
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	for _, suffix := range []string{".page.gwdk", ".gwdk"} {
		if strings.HasSuffix(base, suffix) {
			base = strings.TrimSuffix(base, suffix)
			break
		}
	}
	return strings.TrimSpace(base)
}

// ParseFiles parses multiple .gwdk files into IR source records.
func ParseFiles(paths []string) (gwdkanalysis.Sources, Diagnostics) {
	return ParseBuildFiles(paths)
}

// ParseBuildFiles parses explicit page and component files for build commands.
func ParseBuildFiles(paths []string) (gwdkanalysis.Sources, Diagnostics) {
	var app gwdkanalysis.Sources
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
		case FileKindAsset:
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
func ParseLayoutSource(path string, source []byte) (gwdkir.Layout, Diagnostics) {
	_, diagnostics := Lex(string(source))
	for i := range diagnostics {
		diagnostics[i].File = path
	}
	if diagnostics.HasErrors() {
		return gwdkir.Layout{}, diagnostics
	}

	layout, err := parser.ParseLayout(path, source)
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
func ParseComponentSource(path string, source []byte) (gwdkir.Component, Diagnostics) {
	_, diagnostics := Lex(string(source))
	for i := range diagnostics {
		diagnostics[i].File = path
	}
	if diagnostics.HasErrors() {
		return gwdkir.Component{}, diagnostics
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
	for _, line := range strings.Split(string(source), "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "//") {
			continue
		}
		switch {
		case isMetadataDeclaration(text, "page"):
			return FileKindPage
		case isMetadataDeclaration(text, "component"):
			return FileKindComponent
		case isMetadataDeclaration(text, "layout"):
			return FileKindLayout
		case isMetadataDeclaration(text, "asset"):
			return FileKindAsset
		}
	}
	return FileKindPage
}

func isMetadataDeclaration(text, keyword string) bool {
	if !strings.HasPrefix(text, keyword) {
		return false
	}
	if len(text) == len(keyword) {
		return true
	}
	next := text[len(keyword)]
	return next == ' ' || next == '\t'
}

// CheckResult is the validated program produced by CheckFiles: the IR with
// discovered standalone Go endpoints and backend handler bindings attached,
// plus the flat binding record list for the manifest JSON report.
type CheckResult struct {
	IR       gwdkir.Program
	Bindings []source.BackendBinding
}

// CheckOptions controls validation behavior that depends on project context.
type CheckOptions struct {
	ProjectRoot string
}

// CheckFiles parses and validates .gwdk files.
func CheckFiles(config gowdk.Config, paths []string) (CheckResult, Diagnostics) {
	return CheckFilesWithOptions(config, paths, CheckOptions{})
}

// CheckFilesWithOptions parses and validates .gwdk files with explicit project
// context for checks that need to inspect sibling Go code.
func CheckFilesWithOptions(config gowdk.Config, paths []string, options CheckOptions) (CheckResult, Diagnostics) {
	sources, diagnostics := ParseFiles(paths)
	if diagnostics.HasErrors() {
		return CheckResult{}, diagnostics
	}
	result := CheckResult{IR: gwdkanalysis.BuildProgram(config, sources)}
	if err := compiler.DiscoverGoEndpoints(&result.IR); err != nil {
		diagnostics = append(diagnostics, compilerDiagnostics(err, result.IR)...)
		return result, diagnostics
	}
	validate := compiler.ValidateProgramReport
	if len(paths) == 1 {
		// A single file can never satisfy cross-file checks (use packages,
		// component references), so validate it in source mode to avoid
		// false project-level errors.
		validate = compiler.ValidateSourceProgramReport
	}
	diagnostics = append(diagnostics, compilerDiagnostics(validate(config, result.IR), result.IR)...)
	diagnostics = append(diagnostics, accessibilityDiagnostics(result.IR)...)
	if !diagnostics.HasErrors() {
		result.Bindings = compiler.BindBackendHandlers(&result.IR)
		diagnostics = append(diagnostics, validateContractReferences(config, result.IR, options.ProjectRoot)...)
	}
	return result, diagnostics
}

func validateContractReferences(config gowdk.Config, ir gwdkir.Program, projectRoot string) Diagnostics {
	if strings.TrimSpace(projectRoot) == "" {
		projectRoot = "."
	}
	report, err := contractscan.Scan(projectRoot)
	if err != nil {
		return Diagnostics{{Severity: "error", Message: fmt.Sprintf("scan Go contracts: %v", err)}}
	}
	diagnostics := contractScanDiagnostics(report.Diagnostics)
	if len(ir.ContractRefs) == 0 {
		return diagnostics
	}
	ir.ContractRefs = contractscan.LinkReferences(ir.ContractRefs, report)
	if err := compiler.ValidateContractReferences(ir.ContractRefs); err != nil {
		diagnostics = append(diagnostics, compilerDiagnostics(err, ir)...)
	}
	return diagnostics
}

func contractScanDiagnostics(scanDiagnostics []contractscan.Diagnostic) Diagnostics {
	diagnostics := make(Diagnostics, 0, len(scanDiagnostics))
	for _, item := range scanDiagnostics {
		diagnostics = append(diagnostics, Diagnostic{
			File:     item.Source,
			Code:     item.Code,
			Pos:      Position{Line: item.Line, Column: item.Column},
			Severity: item.Severity,
			Message:  item.Message,
		})
	}
	return diagnostics
}

// CheckSource parses and validates one in-memory .gwdk source buffer.
func CheckSource(config gowdk.Config, path string, source []byte) (gwdkir.Page, Diagnostics) {
	switch ClassifySource(path, source) {
	case FileKindComponent:
		component, diagnostics := ParseComponentSource(path, source)
		if diagnostics.HasErrors() {
			return gwdkir.Page{}, diagnostics
		}
		ir := gwdkanalysis.BuildProgram(config, gwdkanalysis.Sources{Components: []gwdkir.Component{component}})
		diagnostics = append(diagnostics, compilerDiagnostics(compiler.ValidateSourceProgramReport(config, ir), ir)...)
		diagnostics = append(diagnostics, accessibilityDiagnostics(ir)...)
		return gwdkir.Page{}, diagnostics
	case FileKindLayout:
		layout, diagnostics := ParseLayoutSource(path, source)
		if diagnostics.HasErrors() {
			return gwdkir.Page{}, diagnostics
		}
		ir := gwdkanalysis.BuildProgram(config, gwdkanalysis.Sources{Layouts: []gwdkir.Layout{layout}})
		if err := compiler.ValidateSourceProgram(config, ir); err != nil {
			diagnostics = append(diagnostics, compilerDiagnostics(err, ir)...)
		}
		diagnostics = append(diagnostics, accessibilityDiagnostics(ir)...)
		return gwdkir.Page{}, diagnostics
	case FileKindAsset:
		_, diagnostics := Lex(string(source))
		for i := range diagnostics {
			diagnostics[i].File = path
		}
		return gwdkir.Page{}, diagnostics
	}

	page, diagnostics := ParseSource(path, source)
	if diagnostics.HasErrors() {
		return page, diagnostics
	}
	ir := gwdkanalysis.BuildProgram(config, gwdkanalysis.Sources{Pages: []gwdkir.Page{page}})
	diagnostics = append(diagnostics, compilerDiagnostics(compiler.ValidateSourceProgramReport(config, ir), ir)...)
	diagnostics = append(diagnostics, accessibilityDiagnostics(ir)...)
	return page, diagnostics
}

// DiagnosticReport is the JSON shape consumed by editor integrations.
type DiagnosticReport struct {
	Diagnostics Diagnostics `json:"diagnostics"`
}

// CheckJSON returns editor-friendly JSON diagnostics for parsed files.
func CheckJSON(config gowdk.Config, paths []string) ([]byte, Diagnostics) {
	return CheckJSONWithOptions(config, paths, CheckOptions{})
}

// CheckJSONWithOptions returns editor-friendly JSON diagnostics for parsed
// files with explicit project context.
func CheckJSONWithOptions(config gowdk.Config, paths []string, options CheckOptions) ([]byte, Diagnostics) {
	_, diagnostics := CheckFilesWithOptions(config, paths, options)
	payload, err := json.MarshalIndent(DiagnosticReport{Diagnostics: diagnostics}, "", "  ")
	if err != nil {
		return nil, Diagnostics{{Severity: "error", Message: err.Error()}}
	}
	return append(payload, '\n'), diagnostics
}

// ManifestJSON returns the manifest JSON report for parsed and validated
// files. The report shape is derived from the compiler IR.
func ManifestJSON(config gowdk.Config, paths []string) ([]byte, Diagnostics) {
	return ManifestJSONWithOptions(config, paths, CheckOptions{})
}

// ManifestJSONWithOptions returns the manifest JSON report with explicit
// project context.
func ManifestJSONWithOptions(config gowdk.Config, paths []string, options CheckOptions) ([]byte, Diagnostics) {
	result, diagnostics := CheckFilesWithOptions(config, paths, options)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}
	payload, err := marshalManifestJSON(result, config.Render.DefaultMode())
	if err != nil {
		return nil, Diagnostics{{Severity: "error", Message: err.Error()}}
	}
	return append(payload, '\n'), diagnostics
}

func applyDefaultRenderMode(pages []gwdkir.Page, defaultMode gowdk.RenderMode) []gwdkir.Page {
	if defaultMode == "" || defaultMode == gowdk.SPA {
		return pages
	}
	out := append([]gwdkir.Page(nil), pages...)
	for i := range out {
		if out[i].Render == "" {
			out[i].Render = defaultMode
		}
	}
	return out
}

func compilerDiagnostics(err error, ir gwdkir.Program) Diagnostics {
	sources := pageSources(ir)
	switch typed := err.(type) {
	case compiler.ValidationErrors:
		diagnostics := make(Diagnostics, 0, len(typed))
		for _, validation := range typed {
			severity := "error"
			if validation.Severity == compiler.SeverityWarning {
				severity = "warning"
			}
			diagnostics = append(diagnostics, Diagnostic{
				File:       diagnosticSource(validation, sources),
				Code:       validation.Code,
				Pos:        sourcePosition(validation.Span.Start),
				Range:      sourceSpanRange(validation.Span),
				Severity:   severity,
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
		return "Add package <name> before metadata, imports, and blocks."
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
		return "Enable ssr.Addon() in gowdk.config.go or remove request-time page behavior."
	case "spa_dynamic_route_missing_paths":
		return "Add paths { ... } for the dynamic spa route or declare request-time page behavior with load { ... } or go ssr { ... }."
	case "load_requires_request_render":
		return "Enable ssr.Addon() for pages with load { ... }."
	case "invalid_go_endpoint_handler":
		return "Move the gowdk endpoint comment onto an exported package-level function."
	case "duplicate_go_endpoint_comment":
		return "Keep only one //gowdk:act or //gowdk:api comment on the handler."
	case "route_method_conflict":
		return "Give each page route, action, API, or Go endpoint comment a unique method/path pair."
	case "unsupported_action_method":
		return "Use POST for action endpoints, or declare an API endpoint for other HTTP methods."
	case "invalid_backend_handler_name":
		return "Use the exact exported Go function name in the endpoint declaration."
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

func sourcePosition(position source.SourcePosition) Position {
	return Position{Line: position.Line, Column: position.Column}
}

func sourceSpanRange(span source.SourceSpan) *Range {
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

func pageSources(ir gwdkir.Program) map[string]string {
	sources := map[string]string{}
	for _, page := range ir.Pages {
		if page.Source != "" && sources[page.ID] == "" {
			sources[page.ID] = page.Source
		}
	}
	for _, component := range ir.Components {
		if component.Source != "" && sources[component.Name] == "" {
			sources[component.Name] = component.Source
		}
	}
	return sources
}

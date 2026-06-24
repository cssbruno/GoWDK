package lang

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

// CheckStandaloneFiles validates explicit files without loading project config,
// addons, hooks, environment files, or sibling Go packages. Each file is
// checked independently so detached editor buffers do not accidentally acquire
// project-wide semantics from another command-line argument.
func CheckStandaloneFiles(paths []string) Diagnostics {
	var diagnostics Diagnostics
	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{File: path, Severity: "error", Message: err.Error()})
			continue
		}
		_, fileDiagnostics := CheckSourceWithOptions(gowdk.Config{}, path, source, CheckOptions{})
		diagnostics = append(diagnostics, fileDiagnostics...)

		sources, parseDiagnostics := ParseBuildFiles([]string{path})
		if parseDiagnostics.HasErrors() {
			continue
		}
		program := gwdkanalysis.BuildProgram(gowdk.Config{}, sources)
		if contextDiagnostic := standaloneProjectContextDiagnostic(path, program); contextDiagnostic != nil {
			diagnostics = append(diagnostics, *contextDiagnostic)
		}
	}
	return diagnostics
}

// CheckStandaloneJSON returns detached-file diagnostics and explicitly records
// the reduced validation mode for editor and automation consumers.
func CheckStandaloneJSON(paths []string) ([]byte, Diagnostics) {
	diagnostics := CheckStandaloneFiles(paths)
	if diagnostics == nil {
		diagnostics = Diagnostics{}
	}
	payload, err := json.MarshalIndent(struct {
		Version     int         `json:"version"`
		Mode        string      `json:"mode"`
		Diagnostics Diagnostics `json:"diagnostics"`
	}{Version: 1, Mode: "standalone", Diagnostics: diagnostics}, "", "  ")
	if err != nil {
		return nil, Diagnostics{{Severity: "error", Message: err.Error()}}
	}
	return append(payload, '\n'), diagnostics
}

func standaloneProjectContextDiagnostic(path string, program gwdkir.Program) *Diagnostic {
	var reasons []string
	for _, pkg := range program.Packages {
		if len(pkg.Imports) > 0 {
			reasons = appendStandaloneReason(reasons, "Go imports")
		}
		if len(pkg.Uses) > 0 {
			reasons = appendStandaloneReason(reasons, "cross-file use declarations")
		}
	}
	for _, page := range program.Pages {
		if len(page.Layouts) > 0 {
			reasons = appendStandaloneReason(reasons, "layout resolution")
		}
		if page.Blocks.BuildCall != nil {
			reasons = appendStandaloneReason(reasons, "build-function binding")
		}
		if page.Blocks.Server || len(page.Blocks.GoBlocks) > 0 {
			reasons = appendStandaloneReason(reasons, "server or Go-block configuration")
		}
		if len(page.Blocks.Actions) > 0 || len(page.Blocks.APIs) > 0 || len(page.Blocks.Fragments) > 0 {
			reasons = appendStandaloneReason(reasons, "backend handler binding")
		}
	}
	if len(program.ContractRefs) > 0 || len(program.RealtimeSubscriptions) > 0 {
		reasons = appendStandaloneReason(reasons, "contract registration scanning")
	}
	if len(reasons) == 0 {
		return nil
	}
	return &Diagnostic{
		File:     path,
		Severity: "warning",
		Message: fmt.Sprintf(
			"project context required for %s; standalone checking covered syntax and source-local semantics only, rerun with gowdk.config.go for full validation",
			strings.Join(reasons, ", "),
		),
	}
}

func appendStandaloneReason(reasons []string, reason string) []string {
	for _, existing := range reasons {
		if existing == reason {
			return reasons
		}
	}
	return append(reasons, reason)
}

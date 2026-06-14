package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk/internal/appgen"
	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/diagnostics"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/securitymanifest"
)

const auditUsage = "usage: gowdk audit [--config <file>] [--module <name>] [--ssr] [--json] [--emit-tests[=<file>]] [--run] [files...]"

// auditReport is the gowdk audit result: the derived security posture plus the
// findings from evaluating the built-in baseline and declared policies against
// it.
type auditReport struct {
	Version  int                               `json:"version"`
	Status   string                            `json:"status"`
	Summary  auditSummary                      `json:"summary"`
	Findings []auditspec.Finding               `json:"findings"`
	Manifest securitymanifest.SecurityManifest `json:"manifest"`
}

type auditSummary struct {
	Routes    int `json:"routes"`
	Endpoints int `json:"endpoints"`
	Contracts int `json:"contracts"`
	Errors    int `json:"errors"`
	Warnings  int `json:"warnings"`
	Info      int `json:"info"`
}

type auditCommandOptions struct {
	EmitTests bool
	RunTests  bool
	TestPath  string
}

type auditExitError struct {
	errors int
}

func (err auditExitError) Error() string {
	return fmt.Sprintf("audit found %d error finding(s)", err.errors)
}

func (auditExitError) SilentCLIError() {}

// audit derives the security posture from validated IR, evaluates the baseline
// policy against it, and reports findings. It is a standalone command: gowdk
// build never runs it, so it can never fail a build implicitly. It exits
// non-zero when any error-severity finding exists so it can gate CI.
func audit(args []string) error {
	auditOptions, projectArgs, err := parseAuditCommandOptions(args)
	if err != nil {
		return err
	}
	options, paths, err := loadCommandInputs(projectArgs, "audit", true)
	if err != nil {
		return err
	}

	checked, diagnostics := lang.CheckFilesWithOptions(options.Config, paths, lang.CheckOptions{ProjectRoot: options.ProjectRoot})
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("audit failed: source has validation errors")
	}

	ir := checked.IR
	if err := linkIRContractReferences(&ir, options.ProjectRoot); err != nil {
		return err
	}

	manifest := securitymanifest.Build(options.Config, ir)
	declared := auditspec.PoliciesFromIR(ir.AuditSpecs)
	findings := auditspec.Evaluate(manifest, auditspec.ComposeBaseline(declared))
	testFindings, err := handleAuditTests(auditOptions, options, manifest, ir.AuditSpecs)
	if err != nil {
		return err
	}
	findings = append(findings, testFindings...)
	auditspec.SortFindings(findings)
	report := buildAuditReport(manifest, findings)

	if options.JSON {
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
	} else {
		printAuditReport(report)
	}

	if report.Summary.Errors > 0 {
		return auditExitError{errors: report.Summary.Errors}
	}
	return nil
}

func parseAuditCommandOptions(args []string) (auditCommandOptions, []string, error) {
	options := auditCommandOptions{TestPath: "gowdk_audit_test.go"}
	var projectArgs []string
	for _, arg := range args {
		switch {
		case arg == "--emit-tests":
			options.EmitTests = true
		case strings.HasPrefix(arg, "--emit-tests="):
			options.EmitTests = true
			options.TestPath = strings.TrimSpace(strings.TrimPrefix(arg, "--emit-tests="))
			if options.TestPath == "" {
				return options, nil, fmt.Errorf(auditUsage)
			}
		case arg == "--run":
			options.RunTests = true
		default:
			projectArgs = append(projectArgs, arg)
		}
	}
	return options, projectArgs, nil
}

func handleAuditTests(auditOptions auditCommandOptions, options cliOptions, manifest securitymanifest.SecurityManifest, specs []gwdkir.AuditSpec) ([]auditspec.Finding, error) {
	if !auditOptions.EmitTests && !auditOptions.RunTests {
		return nil, nil
	}
	source, err := appgen.StandaloneAuditTestSource(options.Config, manifest, specs)
	if err != nil {
		return nil, err
	}
	if len(source) == 0 {
		return nil, nil
	}

	testPath := auditOptions.TestPath
	if !filepath.IsAbs(testPath) {
		testPath = filepath.Join(options.ProjectRoot, testPath)
	}
	if auditOptions.EmitTests {
		if err := os.MkdirAll(filepath.Dir(testPath), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(testPath, source, 0o644); err != nil {
			return nil, err
		}
		fmt.Fprintf(os.Stderr, "wrote audit tests: %s\n", testPath)
	}

	if !auditOptions.RunTests {
		return nil, nil
	}

	runPath := testPath
	removeAfterRun := false
	if !auditOptions.EmitTests {
		temp, err := os.CreateTemp(options.ProjectRoot, "gowdk_audit_*_test.go")
		if err != nil {
			return nil, err
		}
		runPath = temp.Name()
		removeAfterRun = true
		if _, err := temp.Write(source); err != nil {
			_ = temp.Close()
			return nil, err
		}
		if err := temp.Close(); err != nil {
			return nil, err
		}
	}
	if removeAfterRun {
		defer os.Remove(runPath)
	}

	output, err := runAuditTestFile(options.ProjectRoot, runPath)
	if err == nil {
		fmt.Fprintf(os.Stderr, "audit tests passed: %s\n", runPath)
		return nil, nil
	}
	return []auditspec.Finding{{
		Code:        "audit_test_failed",
		Severity:    auditDiagnosticSeverity("audit_test_failed"),
		Target:      "runtime",
		Source:      runPath,
		Message:     "generated audit integration tests failed",
		Remediation: "Run go test on the emitted audit test file, then update runtime behavior or policy expectations.",
	}}, writeAuditRunOutput(output)
}

func runAuditTestFile(projectRoot, testPath string) (string, error) {
	command := exec.Command("go", "test", testPath)
	command.Dir = projectRoot
	output, err := command.CombinedOutput()
	return string(output), err
}

func writeAuditRunOutput(output string) error {
	output = strings.TrimSpace(output)
	if output != "" {
		fmt.Fprintln(os.Stderr, output)
	}
	return nil
}

func auditDiagnosticSeverity(code string) diagnostics.Severity {
	if severity, ok := diagnostics.DefaultSeverity(code); ok {
		return severity
	}
	return diagnostics.SeverityError
}

func buildAuditReport(manifest securitymanifest.SecurityManifest, findings []auditspec.Finding) auditReport {
	summary := auditspec.Summarize(findings)
	if findings == nil {
		findings = []auditspec.Finding{}
	}
	return auditReport{
		Version: 1,
		Status:  auditspec.Status(summary),
		Summary: auditSummary{
			Routes:    len(manifest.Routes),
			Endpoints: len(manifest.Endpoints),
			Contracts: len(manifest.Contracts),
			Errors:    summary.Errors,
			Warnings:  summary.Warnings,
			Info:      summary.Info,
		},
		Findings: findings,
		Manifest: manifest,
	}
}

func printAuditReport(report auditReport) {
	fmt.Printf("GOWDK audit: %s\n", strings.ToUpper(report.Status))
	fmt.Printf("Posture: %d route(s), %d endpoint(s), %d contract(s)\n", report.Summary.Routes, report.Summary.Endpoints, report.Summary.Contracts)
	fmt.Printf("Findings: %d error(s), %d warning(s), %d info\n", report.Summary.Errors, report.Summary.Warnings, report.Summary.Info)
	if len(report.Findings) == 0 {
		fmt.Println("No policy findings. Posture matches the security baseline.")
		return
	}
	for _, finding := range report.Findings {
		location := finding.Target
		if finding.Source != "" {
			location = finding.Source
		}
		fmt.Printf("[%s] %s: %s\n", strings.ToUpper(string(finding.Severity)), finding.Code, finding.Message)
		if location != "" {
			fmt.Printf("  at: %s\n", location)
		}
		if finding.Remediation != "" {
			fmt.Printf("  fix: %s\n", finding.Remediation)
		}
		fmt.Printf("  why: gowdk explain %s\n", finding.Code)
	}
}

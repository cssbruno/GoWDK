package main

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/appgen"
	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/buildgen"
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
	testFindings, err := handleAuditTests(auditOptions, options, ir, manifest)
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

func handleAuditTests(auditOptions auditCommandOptions, options cliOptions, ir gwdkir.Program, manifest securitymanifest.SecurityManifest) ([]auditspec.Finding, error) {
	if !auditOptions.EmitTests && !auditOptions.RunTests {
		return nil, nil
	}

	if auditOptions.EmitTests {
		testPath := auditOptions.TestPath
		if !filepath.IsAbs(testPath) {
			testPath = filepath.Join(options.ProjectRoot, testPath)
		}
		packageName := standaloneAuditPackageName(filepath.Dir(testPath))
		source, err := appgen.StandaloneAuditTestSourceWithPackage(packageName, options.Config, manifest, ir.AuditSpecs)
		if err != nil {
			return nil, err
		}
		if len(source) == 0 {
			return nil, nil
		}
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

	runPath, output, err := runGeneratedAppAuditTests(options, ir)
	if err == nil {
		if runPath != "" {
			fmt.Fprintf(os.Stderr, "audit generated app tests passed: %s\n", runPath)
		}
		return nil, nil
	}
	return []auditspec.Finding{{
		Code:        "audit_test_failed",
		Severity:    auditDiagnosticSeverity("audit_test_failed"),
		Target:      "runtime",
		Source:      runPath,
		Message:     "generated audit integration tests failed",
		Remediation: "Run gowdk audit --run locally, then update generated runtime behavior or policy expectations.",
	}}, writeAuditRunOutput(output)
}

func standaloneAuditPackageName(dir string) string {
	packages, err := parser.ParseDir(token.NewFileSet(), dir, func(info os.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, parser.PackageClauseOnly)
	if err != nil || len(packages) == 0 {
		return "gowdkaudit_test"
	}
	names := make([]string, 0, len(packages))
	for name := range packages {
		names = append(names, name)
	}
	sort.Strings(names)
	return names[0] + "_test"
}

func runGeneratedAppAuditTests(options cliOptions, ir gwdkir.Program) (string, string, error) {
	source, err := appgen.GeneratedAuditTestSource(appgen.Options{
		AutoRoutes: true,
		Config:     options.Config,
		IR:         &ir,
	})
	if err != nil || len(source) == 0 {
		return "", "", err
	}

	tempRoot, err := os.MkdirTemp("", "gowdk-audit-run-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(tempRoot)

	outputDir := filepath.Join(tempRoot, "output")
	appDir := filepath.Join(tempRoot, "app")
	if _, err := buildgen.BuildFromValidatedIR(options.Config, ir, outputDir); err != nil {
		return "", "", err
	}
	app, err := appgen.GenerateWithOptions(outputDir, appDir, appgen.Options{
		AutoRoutes: true,
		Config:     options.Config,
		IR:         &ir,
	})
	if err != nil {
		return "", "", err
	}

	testPath := filepath.Join(app.AppDir, "gowdkapp", "gowdk_audit_test.go")
	output, err := runGeneratedAppTestPackage(app.AppDir)
	return testPath, output, err
}

func runGeneratedAppTestPackage(appDir string) (string, error) {
	command := exec.Command("go", "test", "./gowdkapp")
	command.Dir = appDir
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

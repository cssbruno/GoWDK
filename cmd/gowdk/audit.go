package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/securitymanifest"
)

const auditUsage = "usage: gowdk audit [--config <file>] [--module <name>] [--ssr] [--json] [files...]"

// auditReport is the gowdk audit result: the derived security posture plus the
// findings from evaluating the built-in baseline (and, later, declared
// policies) against it.
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
	options, paths, err := loadCommandInputs(args, "audit", true)
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
	findings := auditspec.Evaluate(manifest, auditspec.Baseline())
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

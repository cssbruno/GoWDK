package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/diagnostics"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/securitymanifest"
	"github.com/cssbruno/gowdk/internal/securitytext"
)

// buildSecurityAuditError reports that gowdk build was blocked by error-severity
// security findings. It is not silent: main prints its message so the developer
// learns why the build stopped and how to proceed.
type buildSecurityAuditError struct {
	errors int
}

func (err buildSecurityAuditError) Error() string {
	return fmt.Sprintf(
		"build blocked by %d security error finding(s); fix them (run 'gowdk audit' for the full report) or pass --allow-insecure to build anyway",
		err.errors,
	)
}

// enforceBuildSecurityAudit derives the security posture from the validated IR
// and evaluates the built-in baseline together with any declared *.audit.gwdk
// policies. Unlike the standalone gowdk audit command, this runs as part of
// gowdk build so a critical exposure (a roleless contract, a missing CSRF guard,
// a public-by-omission API, a bundled secret) blocks shipping instead of relying
// on a team remembering to gate CI on gowdk audit. Error-severity findings block
// a production build (the baseline encodes production-readiness gates); other
// builds print a prominent summary so the exposure is visible without breaking
// dev iteration. The --allow-insecure flag downgrades the production gate to a
// warning for 0.x experimentation.
func enforceBuildSecurityAudit(options cliOptions, ir gwdkir.Program) error {
	manifest := securitymanifest.Build(options.Config, ir)
	declared := auditspec.PoliciesFromIR(ir.AuditSpecs)
	findings := auditspec.Evaluate(manifest, auditspec.ComposeBaseline(declared))
	auditspec.SortFindings(findings)
	return enforceBuildSecurityFindings(options, findings)
}

func enforceFinalBuildArtifactSecurityAudit(options cliOptions, result buildgen.Result) error {
	findings := finalBuildArtifactSecurityFindings(result)
	auditspec.SortFindings(findings)
	return enforceBuildSecurityFindings(options, findings)
}

func enforceBuildSecurityFindings(options cliOptions, findings []auditspec.Finding) error {
	summary := auditspec.Summarize(findings)
	if summary.Errors == 0 {
		return nil
	}

	printBuildSecurityFindings(findings)
	if options.Config.Build.Mode == gowdk.Production && !options.AllowInsecure {
		return buildSecurityAuditError{errors: summary.Errors}
	}

	reason := "this build is not in production mode"
	if options.AllowInsecure {
		reason = "--allow-insecure was set"
	}
	fmt.Fprintf(os.Stderr, "warning: %d security error finding(s) did not block the build because %s; run 'gowdk audit' to gate them in CI\n", summary.Errors, reason)
	return nil
}

func finalBuildArtifactSecurityFindings(result buildgen.Result) []auditspec.Finding {
	var findings []auditspec.Finding
	for _, artifactPath := range finalBuildArtifactPaths(result) {
		payload, err := os.ReadFile(artifactPath)
		if err != nil {
			findings = append(findings, auditspec.Finding{
				Code:        "audit_bundle_secret",
				Severity:    auditDiagnosticSeverity("audit_bundle_secret"),
				Target:      "artifact:" + filepath.ToSlash(artifactPath),
				Source:      filepath.ToSlash(artifactPath),
				Message:     fmt.Sprintf("final emitted artifact could not be scanned for bundled secrets: %v", err),
				Remediation: "Regenerate the artifact or remove unreadable output so the final build security scan has complete coverage.",
			})
			continue
		}
		if kind, ok := securitytext.FirstSecretKind(string(payload)); ok {
			findings = append(findings, auditspec.Finding{
				Code:        "audit_bundle_secret",
				Severity:    auditDiagnosticSeverity("audit_bundle_secret"),
				Target:      "artifact:" + filepath.ToSlash(artifactPath),
				Source:      filepath.ToSlash(artifactPath),
				Message:     fmt.Sprintf("final emitted artifact carries a secret-shaped value (%s)", kind),
				Remediation: "Move the secret to a runtime environment variable, or remove it from generated frontend output.",
			})
		}
	}
	return auditspec.EnrichFindings(findings)
}

func finalBuildArtifactPaths(result buildgen.Result) []string {
	seen := map[string]bool{}
	var paths []string
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" || seen[path] {
			return
		}
		seen[path] = true
		paths = append(paths, path)
	}
	for _, artifact := range result.Artifacts {
		add(artifact.Path)
	}
	for _, artifact := range result.CSSArtifacts {
		add(artifact.Path)
	}
	for _, artifact := range result.AssetArtifacts {
		add(artifact.Path)
	}
	add(result.RouteManifestPath)
	add(result.AssetManifestPath)
	add(result.SitemapPath)
	add(result.RobotsPath)
	add(result.OpenAPIPath)
	add(result.SecurityManifestPath)
	add(result.BuildReportPath)
	return paths
}

// printBuildSecurityFindings prints the error-severity findings that block the
// build, each with its remediation and the explain command, mirroring the
// standalone audit output so the two surfaces read the same.
func printBuildSecurityFindings(findings []auditspec.Finding) {
	fmt.Fprintln(os.Stderr, "security audit found build-blocking findings:")
	for _, finding := range findings {
		if finding.Severity != diagnostics.SeverityError {
			continue
		}
		fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", strings.ToUpper(string(finding.Severity)), finding.Code, finding.Message)
		location := finding.Target
		if finding.Source != "" {
			location = finding.Source
		}
		if location != "" {
			fmt.Fprintf(os.Stderr, "  at: %s\n", location)
		}
		if finding.Remediation != "" {
			fmt.Fprintf(os.Stderr, "  fix: %s\n", finding.Remediation)
		}
		fmt.Fprintf(os.Stderr, "  why: gowdk explain %s\n", finding.Code)
	}
}

package gowdkcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/diagnostics"
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
		"build blocked by %d security error finding(s); fix them (run 'gowdk audit' for the full report), add an explicit waiver, or scope a bypass with --allow-insecure=<code,...> (or --allow-insecure for all) to ship anyway",
		err.errors,
	)
}

// parseAllowInsecureCodes parses a comma-separated --allow-insecure=CODE1,CODE2
// list into a bypass set. An empty list is rejected so the scoped form always
// names at least one code; use the bare --allow-insecure for a blanket bypass.
func parseAllowInsecureCodes(value string) (map[string]bool, error) {
	codes := map[string]bool{}
	for _, raw := range strings.Split(value, ",") {
		code := strings.TrimSpace(raw)
		if code == "" {
			continue
		}
		codes[code] = true
	}
	if len(codes) == 0 {
		return nil, fmt.Errorf("--allow-insecure=<code,...> needs at least one diagnostic code; use --allow-insecure (no value) to bypass all findings")
	}
	return codes, nil
}

// buildErrorIsBypassed reports whether a production build may downgrade an
// error-severity finding to a warning: either the blanket --allow-insecure is
// set, or the finding's code is in the scoped --allow-insecure=CODE,... set.
func buildErrorIsBypassed(options cliOptions, code string) bool {
	if options.AllowInsecure {
		return true
	}
	return options.AllowInsecureCodes[code]
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
func enforceBuildSecurityAudit(options cliOptions, validated compiler.ValidatedProgram) error {
	ir := validated.Program()
	// Relativize source locations to the project root so the posture digest the
	// build-time gate computes (and the digests a waiver pins) match what gowdk
	// audit produces regardless of checkout location.
	manifest, err := securitymanifest.BuildFromValidatedProgram(options.Config, validated)
	if err != nil {
		return err
	}
	manifest = manifest.Relativize(options.ProjectRoot)
	declared := auditspec.PoliciesFromIR(ir.AuditSpecs)
	policies := relativizeAuditPolicies(auditspec.ComposeBaseline(declared), options.ProjectRoot)
	waiverCtx := auditspec.WaiverContext{
		PolicyDigest:  auditPolicyDigest(policies),
		PostureDigest: auditPostureDigest(manifest),
	}
	findings := auditspec.EvaluateWithWaivers(manifest, policies, waiverCtx)
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
	if summary.Waived > 0 {
		recordBuildSecurityWaivers(findings)
	}
	if summary.Errors == 0 {
		return nil
	}

	bypassedCodes, blocking := partitionBuildErrorBypass(options, findings)
	printBuildSecurityFindings(findings)
	if len(bypassedCodes) > 0 {
		fmt.Fprintf(os.Stderr, "security bypass: %d error finding(s) downgraded by --allow-insecure (codes: %s); this is recorded provenance, not a fix\n",
			summary.Errors-blocking, strings.Join(bypassedCodes, ", "))
	}

	if options.Config.Build.Mode == gowdk.Production {
		if blocking > 0 {
			cause := buildSecurityAuditError{errors: blocking}
			return operationErrorFromAuditFindings(cause.Error(), findings, cause)
		}
		return nil
	}

	if blocking > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d security error finding(s) did not block the build because this build is not in production mode; run 'gowdk audit' to gate them in CI\n", blocking)
	}
	return nil
}

// partitionBuildErrorBypass splits the non-waived error findings into the
// distinct codes a bypass downgrades and the count that still blocks.
func partitionBuildErrorBypass(options cliOptions, findings []auditspec.Finding) (bypassedCodes []string, blocking int) {
	seen := map[string]bool{}
	for _, finding := range findings {
		if finding.Severity != diagnostics.SeverityError || finding.Suppression != nil {
			continue
		}
		if buildErrorIsBypassed(options, finding.Code) {
			if !seen[finding.Code] {
				seen[finding.Code] = true
				bypassedCodes = append(bypassedCodes, finding.Code)
			}
			continue
		}
		blocking++
	}
	sort.Strings(bypassedCodes)
	return bypassedCodes, blocking
}

// recordBuildSecurityWaivers prints the explicit waivers applied during a build
// so a suppressed finding is always visible provenance in the build log.
func recordBuildSecurityWaivers(findings []auditspec.Finding) {
	for _, finding := range findings {
		if finding.Suppression == nil {
			continue
		}
		fmt.Fprintf(os.Stderr, "security waiver: %s on %s waived by %s until %s (%s)\n",
			finding.Code, finding.Target, finding.Suppression.Owner, finding.Suppression.Expires, finding.Suppression.Justification)
	}
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

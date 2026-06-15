package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/diagnostics"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/securitymanifest"
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

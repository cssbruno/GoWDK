package auditspec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/securitymanifest"
)

// evalSecurityHeaders audits the semantic strength of the configured security
// response headers. It only reasons over headers the app actually configured, so
// an app that opts out of generated security headers is left to the (separate)
// app-owned hardening posture rather than flagged here. Each finding attributes
// the source to the build config and carries remediation.
func evalSecurityHeaders(surface securitymanifest.FrontendSurface, policy Policy, buildMode string) []Finding {
	if len(surface.ConfiguredHeaders) == 0 {
		return nil
	}
	values := map[string]string{}
	for _, header := range surface.ConfiguredHeaders {
		key := strings.ToLower(strings.TrimSpace(header.Name))
		if key == "" {
			continue
		}
		values[key] = strings.TrimSpace(header.Value)
	}

	const source = "config:Build.SecurityHeaders"
	var findings []Finding
	add := func(code, target, message, remediation string) {
		rule := ruleWithCode(Rule{Kind: RuleCheckSecurityHeaders}, code)
		findings = append(findings, finding(rule, policy, target, source, message, remediation))
	}

	if csp, ok := values["content-security-policy"]; ok {
		if reason, weak := cspWeakness(csp); weak {
			add("audit_header_csp_weak", "header:Content-Security-Policy",
				fmt.Sprintf("Content-Security-Policy is weak: %s", reason),
				"Remove unsafe-inline/unsafe-eval and wildcard sources; pin explicit origins or 'self' on default-src/script-src.")
		}
	}

	if nosniff, ok := values["x-content-type-options"]; !ok || !strings.EqualFold(nosniff, "nosniff") {
		add("audit_header_nosniff_missing", "header:X-Content-Type-Options",
			"X-Content-Type-Options is not set to nosniff while other security headers are configured",
			"Set X-Content-Type-Options: nosniff so browsers do not MIME-sniff responses.")
	}

	if referrer, ok := values["referrer-policy"]; ok && referrerWeak(referrer) {
		add("audit_header_referrer_weak", "header:Referrer-Policy",
			fmt.Sprintf("Referrer-Policy %q can leak full URLs across origins", referrer),
			"Use strict-origin-when-cross-origin, strict-origin, same-origin, or no-referrer.")
	}

	if hsts, ok := values["strict-transport-security"]; ok {
		if reason, weak := hstsWeakness(hsts, buildMode); weak {
			add("audit_header_hsts_weak", "header:Strict-Transport-Security",
				fmt.Sprintf("Strict-Transport-Security is unsuitable: %s", reason),
				"Set max-age to at least 15552000 (180 days) and add includeSubDomains before deploying HSTS to production.")
		}
	}

	if reason, conflict := frameConflict(values); conflict {
		add("audit_header_frame_conflict", "header:X-Frame-Options",
			fmt.Sprintf("framing policy is inconsistent: %s", reason),
			"Make X-Frame-Options and the CSP frame-ancestors directive agree (DENY/SAMEORIGIN with a matching frame-ancestors).")
	}

	return findings
}

// parseCSP splits a Content-Security-Policy value into directive name -> sources,
// lowercased for stable comparison.
func parseCSP(value string) map[string][]string {
	directives := map[string][]string{}
	for _, part := range strings.Split(value, ";") {
		fields := strings.Fields(strings.ToLower(strings.TrimSpace(part)))
		if len(fields) == 0 {
			continue
		}
		directives[fields[0]] = fields[1:]
	}
	return directives
}

func cspWeakness(value string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return "policy is empty", true
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "unsafe-inline") {
		return "allows unsafe-inline scripts/styles", true
	}
	if strings.Contains(lower, "unsafe-eval") {
		return "allows unsafe-eval", true
	}
	directives := parseCSP(value)
	for _, name := range []string{"default-src", "script-src", "object-src", "base-uri"} {
		for _, src := range directives[name] {
			if isWildcardCSPSource(src) {
				return name + " allows a wildcard source", true
			}
		}
	}
	if _, hasDefault := directives["default-src"]; !hasDefault {
		if _, hasScript := directives["script-src"]; !hasScript {
			return "no default-src or script-src directive constrains script execution", true
		}
	}
	return "", false
}

func isWildcardCSPSource(src string) bool {
	switch src {
	case "*", "http:", "https:":
		return true
	}
	return strings.HasPrefix(src, "*.") || strings.Contains(src, "://*")
}

func referrerWeak(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "unsafe-url", "no-referrer-when-downgrade":
		return true
	default:
		return false
	}
}

func hstsWeakness(value, buildMode string) (string, bool) {
	maxAge, ok := hstsMaxAge(value)
	if !ok {
		return "max-age is missing or not a number", true
	}
	if maxAge == 0 {
		return "max-age=0 disables HSTS", true
	}
	const minProductionMaxAge = 15552000 // 180 days
	if strings.EqualFold(strings.TrimSpace(buildMode), "production") && maxAge < minProductionMaxAge {
		return fmt.Sprintf("max-age=%d is below the recommended 15552000 for production", maxAge), true
	}
	return "", false
}

func hstsMaxAge(value string) (int64, bool) {
	for _, part := range strings.Split(value, ";") {
		part = strings.ToLower(strings.TrimSpace(part))
		if !strings.HasPrefix(part, "max-age") {
			continue
		}
		_, raw, ok := strings.Cut(part, "=")
		if !ok {
			return 0, false
		}
		seconds, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return 0, false
		}
		return seconds, true
	}
	return 0, false
}

func frameConflict(values map[string]string) (string, bool) {
	xfo, hasXFO := values["x-frame-options"]
	if hasXFO {
		switch strings.ToUpper(strings.TrimSpace(xfo)) {
		case "DENY", "SAMEORIGIN":
		default:
			return fmt.Sprintf("X-Frame-Options has an unrecognized value %q", xfo), true
		}
	}

	frameAncestors, hasFA := parseCSP(values["content-security-policy"])["frame-ancestors"]
	if !hasXFO || !hasFA {
		return "", false
	}

	switch strings.ToUpper(strings.TrimSpace(xfo)) {
	case "DENY":
		if !frameAncestorsOnly(frameAncestors, "'none'") {
			return "X-Frame-Options DENY only matches CSP frame-ancestors 'none'", true
		}
	case "SAMEORIGIN":
		if !frameAncestorsOnly(frameAncestors, "'self'") {
			return "X-Frame-Options SAMEORIGIN only matches CSP frame-ancestors 'self'", true
		}
	}
	return "", false
}

func frameAncestorsOnly(frameAncestors []string, want string) bool {
	return len(frameAncestors) == 1 && frameAncestors[0] == want
}

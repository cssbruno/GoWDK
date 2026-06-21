package auditspec

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// EnrichFindings fills stable triage metadata on findings without changing the
// policy result itself. It is safe to call more than once.
func EnrichFindings(findings []Finding) []Finding {
	for index := range findings {
		if findings[index].Fingerprint == "" {
			findings[index].Fingerprint = findingFingerprint(findings[index])
		}
		if findings[index].Confidence == "" {
			findings[index].Confidence = confidenceFor(findings[index].Code)
		}
		if findings[index].Evidence == "" {
			findings[index].Evidence = evidenceFor(findings[index].Code)
		}
		if findings[index].CWE == nil {
			findings[index].CWE = cweFor(findings[index].Code)
		}
		if findings[index].OWASP == nil {
			findings[index].OWASP = owaspFor(findings[index].Code)
		}
	}
	return findings
}

func findingFingerprint(finding Finding) string {
	key := strings.Join([]string{
		finding.Code,
		finding.Target,
		finding.Policy,
		finding.Rule,
		sourceWithoutLine(finding.Source),
		stableMessage(finding.Message),
	}, "\x00")
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:16])
}

func sourceWithoutLine(source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		return ""
	}
	parts := strings.Split(source, ":")
	if len(parts) <= 1 {
		return source
	}
	last := parts[len(parts)-1]
	for _, char := range last {
		if char < '0' || char > '9' {
			return source
		}
	}
	return strings.Join(parts[:len(parts)-1], ":")
}

func stableMessage(message string) string {
	message = strings.TrimSpace(message)
	if len(message) > 160 {
		return message[:160]
	}
	return message
}

func confidenceFor(code string) string {
	switch {
	case strings.HasPrefix(code, "policy_"):
		return "high"
	case code == "audit_guard_unverified":
		return "medium"
	case code == "audit_test_failed":
		return "high"
	default:
		return "high"
	}
}

func evidenceFor(code string) string {
	switch {
	case strings.HasPrefix(code, "policy_"):
		return "policy-resolution"
	case code == "audit_test_failed":
		return "runtime-test"
	case code == "audit_guard_unverified":
		return "inferred-static"
	case strings.HasPrefix(code, "audit_observability_"):
		return "static-observability-posture"
	default:
		return "static-posture"
	}
}

func cweFor(code string) []string {
	switch code {
	case "audit_action_missing_csrf", "audit_api_missing_csrf", "audit_command_missing_csrf":
		return []string{"CWE-352"}
	case "audit_api_public_by_omission", "audit_guardless_endpoint_page", "audit_client_route_unguarded", "audit_public_not_allowed", "audit_required_guard_missing", "audit_guard_unverified":
		return []string{"CWE-862"}
	case "audit_contract_roleless":
		return []string{"CWE-863"}
	case "audit_bundle_secret":
		return []string{"CWE-798"}
	case "audit_raw_html_sink":
		return []string{"CWE-79"}
	case "audit_header_csp_weak":
		return []string{"CWE-1021", "CWE-79"}
	case "audit_header_frame_conflict":
		return []string{"CWE-1021"}
	case "audit_header_hsts_weak":
		return []string{"CWE-319"}
	case "audit_header_nosniff_missing":
		return []string{"CWE-693"}
	case "audit_header_referrer_weak":
		return []string{"CWE-200"}
	case "audit_max_body_exceeds_policy", "audit_observability_body_limit_missing", "audit_observability_batch_limit_missing":
		return []string{"CWE-770"}
	case "audit_observability_production_exposed", "audit_observability_origin_unchecked", "audit_observability_content_type_missing", "audit_observability_absolute_source":
		return []string{"CWE-200"}
	default:
		return nil
	}
}

func owaspFor(code string) []string {
	switch code {
	case "audit_action_missing_csrf", "audit_api_missing_csrf", "audit_command_missing_csrf":
		return []string{"A01:2021-Broken Access Control"}
	case "audit_api_public_by_omission", "audit_guardless_endpoint_page", "audit_client_route_unguarded", "audit_public_not_allowed", "audit_required_guard_missing", "audit_guard_unverified", "audit_contract_roleless":
		return []string{"A01:2021-Broken Access Control"}
	case "audit_bundle_secret", "audit_observability_production_exposed", "audit_observability_origin_unchecked", "audit_observability_content_type_missing", "audit_observability_absolute_source":
		return []string{"A02:2021-Cryptographic Failures"}
	case "audit_raw_html_sink":
		return []string{"A03:2021-Injection"}
	case "audit_max_body_exceeds_policy", "audit_observability_body_limit_missing", "audit_observability_batch_limit_missing":
		return []string{"A05:2021-Security Misconfiguration"}
	default:
		if strings.HasPrefix(code, "audit_header_") {
			return []string{"A05:2021-Security Misconfiguration"}
		}
		return nil
	}
}

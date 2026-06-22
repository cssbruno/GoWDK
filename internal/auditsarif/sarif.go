// Package auditsarif renders gowdk audit findings as SARIF 2.1.0, the format
// GitHub code scanning ingests. It maps each finding to a result keyed by its
// stable fingerprint (so an alert tracks across line movement), and each
// distinct diagnostic code to a reusable rule with explain text, CWE/OWASP
// taxonomy, and remediation. Waived findings are emitted as suppressed results
// so a justified suppression stays visible in the security dashboard rather than
// silently absent.
package auditsarif

import (
	"sort"
	"strings"

	"github.com/cssbruno/gowdk/internal/auditspec"
	"github.com/cssbruno/gowdk/internal/diagnostics"
)

// FingerprintKey names the partialFingerprints entry carrying the stable finding
// identity. The /v1 suffix lets the fingerprint algorithm evolve without
// colliding with historic alert tracking.
const FingerprintKey = "gowdkAuditFingerprint/v1"

// Options carries the tool identity stamped into the SARIF driver.
type Options struct {
	ToolName       string
	ToolVersion    string
	InformationURI string
}

// Document is the root SARIF 2.1.0 log.
type Document struct {
	Schema  string `json:"$schema"`
	Version string `json:"version"`
	Runs    []Run  `json:"runs"`
}

// Run is one analysis run.
type Run struct {
	Tool    Tool     `json:"tool"`
	Results []Result `json:"results"`
}

// Tool identifies the analyzer.
type Tool struct {
	Driver Driver `json:"driver"`
}

// Driver is the analyzer component and its rule catalog.
type Driver struct {
	Name           string `json:"name"`
	Version        string `json:"version,omitempty"`
	InformationURI string `json:"informationUri,omitempty"`
	Rules          []Rule `json:"rules"`
}

// Rule is the reusable metadata for one diagnostic code.
type Rule struct {
	ID                   string          `json:"id"`
	Name                 string          `json:"name,omitempty"`
	ShortDescription     *Message        `json:"shortDescription,omitempty"`
	FullDescription      *Message        `json:"fullDescription,omitempty"`
	Help                 *Message        `json:"help,omitempty"`
	DefaultConfiguration *RuleConfig     `json:"defaultConfiguration,omitempty"`
	Properties           *RuleProperties `json:"properties,omitempty"`
	HelpURI              string          `json:"helpUri,omitempty"`
}

// RuleConfig carries the rule's default level.
type RuleConfig struct {
	Level string `json:"level"`
}

// RuleProperties carries the security taxonomy GitHub surfaces as tags.
type RuleProperties struct {
	Tags      []string `json:"tags,omitempty"`
	Precision string   `json:"precision,omitempty"`
}

// Result is one finding instance.
type Result struct {
	RuleID              string            `json:"ruleId"`
	Level               string            `json:"level"`
	Message             Message           `json:"message"`
	Locations           []Location        `json:"locations,omitempty"`
	PartialFingerprints map[string]string `json:"partialFingerprints,omitempty"`
	Suppressions        []Suppression     `json:"suppressions,omitempty"`
	Properties          map[string]any    `json:"properties,omitempty"`
}

// Message is the SARIF text wrapper.
type Message struct {
	Text string `json:"text"`
}

// Location points at the source artifact and region.
type Location struct {
	PhysicalLocation PhysicalLocation `json:"physicalLocation"`
}

// PhysicalLocation is the artifact plus optional region.
type PhysicalLocation struct {
	ArtifactLocation ArtifactLocation `json:"artifactLocation"`
	Region           *Region          `json:"region,omitempty"`
}

// ArtifactLocation is a workspace-relative URI.
type ArtifactLocation struct {
	URI string `json:"uri"`
}

// Region is a 1-based line span.
type Region struct {
	StartLine int `json:"startLine"`
}

// Suppression records a justified waiver so the result stays visible but muted.
type Suppression struct {
	Kind          string `json:"kind"`
	Justification string `json:"justification,omitempty"`
}

// FromFindings renders findings as a SARIF document. Findings are assumed to
// already be enriched (fingerprint, severity, taxonomy) by auditspec.
func FromFindings(findings []auditspec.Finding, opts Options) Document {
	name := opts.ToolName
	if strings.TrimSpace(name) == "" {
		name = "gowdk audit"
	}
	rules := buildRules(findings)
	results := make([]Result, 0, len(findings))
	for _, finding := range findings {
		results = append(results, resultFor(finding))
	}
	return Document{
		Schema:  "https://json.schemastore.org/sarif-2.1.0.json",
		Version: "2.1.0",
		Runs: []Run{{
			Tool: Tool{Driver: Driver{
				Name:           name,
				Version:        opts.ToolVersion,
				InformationURI: opts.InformationURI,
				Rules:          rules,
			}},
			Results: results,
		}},
	}
}

func buildRules(findings []auditspec.Finding) []Rule {
	seen := map[string]bool{}
	codes := make([]string, 0, len(findings))
	taxonomy := map[string][]string{}
	for _, finding := range findings {
		if !seen[finding.Code] {
			seen[finding.Code] = true
			codes = append(codes, finding.Code)
		}
		if _, ok := taxonomy[finding.Code]; !ok {
			taxonomy[finding.Code] = finding.CWE
			taxonomy[finding.Code] = append(taxonomy[finding.Code], finding.OWASP...)
		}
	}
	sort.Strings(codes)
	rules := make([]Rule, 0, len(codes))
	for _, code := range codes {
		rules = append(rules, ruleFor(code, taxonomy[code]))
	}
	return rules
}

func ruleFor(code string, tags []string) Rule {
	rule := Rule{
		ID:   code,
		Name: ruleName(code),
		Help: &Message{Text: "Run: gowdk explain " + code},
	}
	if severity, ok := diagnostics.DefaultSeverity(code); ok {
		rule.DefaultConfiguration = &RuleConfig{Level: sarifLevel(severity)}
	}
	if explanation, ok := diagnostics.Explain(code); ok {
		if summary := strings.TrimSpace(explanation.Summary); summary != "" {
			rule.ShortDescription = &Message{Text: summary}
		}
		if details := strings.TrimSpace(explanation.Details); details != "" {
			rule.FullDescription = &Message{Text: details}
			rule.Help = &Message{Text: details + "\n\nRun: gowdk explain " + code}
		}
	}
	if len(tags) > 0 {
		rule.Properties = &RuleProperties{Tags: dedupeStrings(tags)}
	}
	return rule
}

// ruleName converts a snake_case code into a CamelCase rule name, which GitHub
// renders as the alert's rule label.
func ruleName(code string) string {
	parts := strings.Split(code, "_")
	for index, part := range parts {
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}
	return strings.Join(parts, "")
}

func resultFor(finding auditspec.Finding) Result {
	result := Result{
		RuleID:  finding.Code,
		Level:   sarifLevel(finding.Severity),
		Message: Message{Text: messageText(finding)},
	}
	if finding.Fingerprint != "" {
		result.PartialFingerprints = map[string]string{FingerprintKey: finding.Fingerprint}
	}
	if location, ok := locationFor(finding); ok {
		result.Locations = []Location{location}
	}
	if finding.Suppression != nil {
		result.Suppressions = []Suppression{{
			Kind:          "external",
			Justification: suppressionJustification(finding),
		}}
	}
	result.Properties = resultProperties(finding)
	return result
}

func messageText(finding auditspec.Finding) string {
	message := strings.TrimSpace(finding.Message)
	if message == "" {
		message = finding.Code
	}
	if remediation := strings.TrimSpace(finding.Remediation); remediation != "" {
		message += " Fix: " + remediation
	}
	return message
}

func resultProperties(finding auditspec.Finding) map[string]any {
	properties := map[string]any{}
	if finding.Policy != "" {
		properties["policy"] = finding.Policy
	}
	if finding.Rule != "" {
		properties["rule"] = finding.Rule
	}
	if finding.Confidence != "" {
		properties["confidence"] = finding.Confidence
	}
	if finding.Evidence != "" {
		properties["evidence"] = finding.Evidence
	}
	if finding.Target != "" {
		properties["target"] = finding.Target
	}
	if len(properties) == 0 {
		return nil
	}
	return properties
}

func suppressionJustification(finding auditspec.Finding) string {
	if finding.Suppression == nil {
		return ""
	}
	parts := make([]string, 0, 3)
	if owner := strings.TrimSpace(finding.Suppression.Owner); owner != "" {
		parts = append(parts, "owner "+owner)
	}
	if justification := strings.TrimSpace(finding.Suppression.Justification); justification != "" {
		parts = append(parts, justification)
	}
	if expires := strings.TrimSpace(finding.Suppression.Expires); expires != "" {
		parts = append(parts, "expires "+expires)
	}
	return strings.Join(parts, "; ")
}

// locationFor maps a finding source to a SARIF physical location. A file-shaped
// source of the form "path:line" yields an artifact plus region; a bare file
// path yields the artifact alone. Findings whose source is not a file (for
// example "runtime") fall back to the logical target so every result still
// carries a stable, human-readable location URI.
func locationFor(finding auditspec.Finding) (Location, bool) {
	uri, line := splitSourceLocation(finding.Source)
	if looksLikeFilePath(uri) {
		physical := PhysicalLocation{ArtifactLocation: ArtifactLocation{URI: uri}}
		if line > 0 {
			physical.Region = &Region{StartLine: line}
		}
		return Location{PhysicalLocation: physical}, true
	}
	fallback := strings.TrimSpace(finding.Target)
	if fallback == "" {
		fallback = uri
	}
	if fallback == "" {
		return Location{}, false
	}
	return Location{PhysicalLocation: PhysicalLocation{ArtifactLocation: ArtifactLocation{URI: fallback}}}, true
}

// looksLikeFilePath reports whether uri is plausibly a workspace file (so a SARIF
// region makes sense) rather than a logical token like "runtime".
func looksLikeFilePath(uri string) bool {
	uri = strings.TrimSpace(uri)
	if uri == "" {
		return false
	}
	if strings.ContainsAny(uri, "/\\") {
		return true
	}
	dot := strings.LastIndex(uri, ".")
	if dot <= 0 || dot == len(uri)-1 {
		return false
	}
	for _, char := range uri[dot+1:] {
		if !(char >= 'a' && char <= 'z') && !(char >= 'A' && char <= 'Z') {
			return false
		}
	}
	return true
}

func splitSourceLocation(source string) (string, int) {
	source = strings.TrimSpace(source)
	if source == "" {
		return "", 0
	}
	index := strings.LastIndex(source, ":")
	if index <= 0 || index == len(source)-1 {
		return source, 0
	}
	suffix := source[index+1:]
	line := 0
	for _, char := range suffix {
		if char < '0' || char > '9' {
			return source, 0
		}
		line = line*10 + int(char-'0')
	}
	return source[:index], line
}

func sarifLevel(severity diagnostics.Severity) string {
	switch severity {
	case diagnostics.SeverityError:
		return "error"
	case diagnostics.SeverityWarning:
		return "warning"
	default:
		return "note"
	}
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

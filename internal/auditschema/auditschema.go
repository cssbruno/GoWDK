// Package auditschema publishes the versioned JSON Schema contracts for the
// gowdk audit report (`gowdk audit --json`) and the security manifest
// (gowdk-security.json). The schema files are embedded so the CLI can print
// them (`gowdk audit --schema`) and tests can assert the emitted JSON stays
// compatible with the published contract.
package auditschema

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
)

//go:embed schema/audit-report.v1.json
var auditReportSchema []byte

//go:embed schema/security-manifest.v1.json
var securityManifestSchema []byte

// Name identifies one published schema.
type Name string

const (
	// AuditReport is the schema for `gowdk audit --json`.
	AuditReport Name = "report"
	// SecurityManifest is the schema for gowdk-security.json.
	SecurityManifest Name = "security"
)

// Names lists the published schema selectors in stable order.
func Names() []string {
	names := []string{string(AuditReport), string(SecurityManifest)}
	sort.Strings(names)
	return names
}

// Schema returns the embedded JSON Schema bytes for the named contract.
func Schema(name Name) ([]byte, error) {
	switch name {
	case AuditReport:
		return auditReportSchema, nil
	case SecurityManifest:
		return securityManifestSchema, nil
	default:
		return nil, fmt.Errorf("unknown schema %q (want one of %v)", name, Names())
	}
}

// AuditReportSchema returns the embedded audit report JSON Schema.
func AuditReportSchema() []byte { return auditReportSchema }

// SecurityManifestSchema returns the embedded security manifest JSON Schema.
func SecurityManifestSchema() []byte { return securityManifestSchema }

// DescribedKeys returns the property names declared under the given path of the
// named schema. An empty path targets the schema root; a non-empty path walks
// nested objects (for example "$defs", "finding"). Compatibility tests use it to
// assert every key the CLI emits is part of the published contract, so a new
// struct field cannot silently drift away from the schema.
func DescribedKeys(name Name, path ...string) (map[string]bool, error) {
	raw, err := Schema(name)
	if err != nil {
		return nil, err
	}
	var node map[string]any
	if err := json.Unmarshal(raw, &node); err != nil {
		return nil, fmt.Errorf("schema %q is not valid JSON: %w", name, err)
	}
	for _, segment := range path {
		next, ok := node[segment].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("schema %q has no object at %q", name, segment)
		}
		node = next
	}
	properties, ok := node["properties"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema %q has no properties at %v", name, path)
	}
	keys := make(map[string]bool, len(properties))
	for key := range properties {
		keys[key] = true
	}
	return keys, nil
}

package parser

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAuditFileGolden(t *testing.T) {
	path := filepath.Join("testdata", "golden", "security.audit.gwdk")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := ParseAuditFile(path, source)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.MarshalIndent(struct {
		Package  string `json:"package"`
		Policies any    `json:"policies"`
		Tests    any    `json:"tests"`
	}{
		Package:  spec.Package,
		Policies: spec.Policies,
		Tests:    spec.Tests,
	}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	payload = append(payload, '\n')
	goldenPath := filepath.Join("testdata", "golden", "audit.golden.json")
	golden, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) != string(golden) {
		t.Fatalf("audit golden mismatch\n got:\n%s\nwant:\n%s", payload, golden)
	}
}

func TestParseAuditSyntaxReportsUnsupportedPolicyLine(t *testing.T) {
	_, err := ParseAuditSyntax([]byte(`policy bad {
  surprise now
}
`))
	if err == nil {
		t.Fatal("expected unsupported policy line error")
	}
	if !strings.Contains(err.Error(), `unsupported policy syntax "surprise now"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

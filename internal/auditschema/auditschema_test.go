package auditschema

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPublishedSchemasAreValidJSONAndSelfConsistent(t *testing.T) {
	for _, name := range []Name{AuditReport, SecurityManifest} {
		raw, err := Schema(name)
		if err != nil {
			t.Fatalf("Schema(%q): %v", name, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("schema %q is not valid JSON: %v", name, err)
		}
		if _, ok := doc["$id"].(string); !ok {
			t.Fatalf("schema %q is missing $id", name)
		}
		defs, _ := doc["$defs"].(map[string]any)
		for _, ref := range collectRefs(doc) {
			def := strings.TrimPrefix(ref, "#/$defs/")
			if def == ref {
				t.Fatalf("schema %q has non-local $ref %q", name, ref)
			}
			if _, ok := defs[def]; !ok {
				t.Fatalf("schema %q references missing $defs/%s", name, def)
			}
		}
	}
}

func TestSchemaLookupRejectsUnknownName(t *testing.T) {
	if _, err := Schema(Name("nope")); err == nil {
		t.Fatal("expected an error for an unknown schema name")
	}
	if got := Names(); len(got) != 2 || got[0] != "report" || got[1] != "security" {
		t.Fatalf("Names() = %v, want [report security]", got)
	}
}

func TestDescribedKeysWalksDefs(t *testing.T) {
	keys, err := DescribedKeys(AuditReport, "$defs", "finding")
	if err != nil {
		t.Fatalf("DescribedKeys finding: %v", err)
	}
	for _, want := range []string{"code", "severity", "fingerprint", "message"} {
		if !keys[want] {
			t.Fatalf("finding schema is missing property %q", want)
		}
	}
}

// collectRefs returns every "$ref" string value found anywhere in the document.
func collectRefs(node any) []string {
	var refs []string
	switch typed := node.(type) {
	case map[string]any:
		for key, value := range typed {
			if key == "$ref" {
				if ref, ok := value.(string); ok {
					refs = append(refs, ref)
				}
				continue
			}
			refs = append(refs, collectRefs(value)...)
		}
	case []any:
		for _, item := range typed {
			refs = append(refs, collectRefs(item)...)
		}
	}
	return refs
}

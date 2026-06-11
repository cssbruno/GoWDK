package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/view"
)

// TestStabilityTableCoversConstructs guards against the published stability
// table drifting from the code registries: every metadata keyword and every
// supported g: directive must appear in docs/language/stability.md, so adding a
// construct in code without documenting its tier fails here.
func TestStabilityTableCoversConstructs(t *testing.T) {
	path := filepath.FromSlash("../../docs/language/stability.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stability table: %v", err)
	}
	doc := string(content)

	for _, keyword := range MetadataKeywords {
		if !strings.Contains(doc, "`"+keyword+"`") {
			t.Errorf("metadata keyword %q is missing from %s", keyword, path)
		}
	}

	for _, directive := range view.SupportedDirectiveNames() {
		if !strings.Contains(doc, directive) {
			t.Errorf("g: directive %q is missing from %s", directive, path)
		}
	}
}

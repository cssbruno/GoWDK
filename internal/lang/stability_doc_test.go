package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/view"
)

// TestStabilityRegistryCoversCodeConstructs guards the construct stability
// registry against the code: every metadata keyword and every supported g:
// directive must have a tier, so adding a construct without classifying it
// fails here.
func TestStabilityRegistryCoversCodeConstructs(t *testing.T) {
	registry := map[string]ConstructStability{}
	for _, construct := range ConstructStabilities() {
		registry[construct.Name] = construct
	}

	for _, keyword := range MetadataKeywords {
		construct, ok := registry[keyword]
		if !ok {
			t.Errorf("metadata keyword %q has no stability entry", keyword)
			continue
		}
		if construct.Kind != ConstructKeyword || construct.Tier != TierStable {
			t.Errorf("metadata keyword %q has unexpected entry %+v", keyword, construct)
		}
	}

	for _, directive := range view.SupportedDirectiveNames() {
		if _, ok := registry[directive]; !ok {
			t.Errorf("supported g: directive %q has no stability entry", directive)
		}
	}
}

// TestStabilityTableMatchesRegistry guards the published table against the
// registry: every construct (or, for planned/deprecated constructs, its
// diagnostic code) must appear in docs/language/stability.md, and every tier
// used must be documented as a status term.
func TestStabilityTableMatchesRegistry(t *testing.T) {
	path := filepath.FromSlash("../../docs/language/stability.md")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read stability table: %v", err)
	}
	doc := string(content)

	for _, construct := range ConstructStabilities() {
		// Planned/deprecated constructs are identified in the table by the
		// diagnostic code emitted on use; stable/partial constructs by name.
		if construct.DiagnosticCode != "" {
			if !strings.Contains(doc, construct.DiagnosticCode) {
				t.Errorf("construct %q references diagnostic %q which is missing from %s", construct.Name, construct.DiagnosticCode, path)
			}
			continue
		}
		if !strings.Contains(doc, construct.Name) {
			t.Errorf("construct %q (%s) is missing from %s", construct.Name, construct.Kind, path)
		}
	}

	for _, tier := range []StabilityTier{TierStable, TierPartial, TierPlanned, TierDeprecated} {
		if !strings.Contains(strings.ToLower(doc), string(tier)) {
			t.Errorf("stability tier %q is not documented in %s", tier, path)
		}
	}
}

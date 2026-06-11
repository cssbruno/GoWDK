package lang

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/view"
)

var backtickedToken = regexp.MustCompile("`([^`]+)`")

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

	// SupportedDirectiveNames excludes the accepted directive families, so guard
	// them explicitly to keep the registry a complete source of truth.
	for _, family := range []string{"g:on:*", "g:message:*"} {
		if _, ok := registry[family]; !ok {
			t.Errorf("supported g: directive family %q has no stability entry", family)
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

	// Collect the exact backticked tokens (construct names and codes) from the
	// table, so a match proves a real table cell rather than incidental prose:
	// short names like `go`, `act`, and `api` must not pass by appearing inside
	// words such as "diagnostics" or "contract".
	tokens := map[string]bool{}
	for _, match := range backtickedToken.FindAllStringSubmatch(doc, -1) {
		tokens[match[1]] = true
	}

	for _, construct := range ConstructStabilities() {
		// Constructs with a diagnostic code are identified in the table by that
		// code (planned blocks, deprecated endpoint forms); the rest by name.
		want := construct.Name
		if construct.DiagnosticCode != "" {
			want = construct.DiagnosticCode
		}
		if !tokens[want] {
			t.Errorf("construct %q (%s): expected backticked token %q in %s", construct.Name, construct.Kind, want, path)
		}
	}

	for _, tier := range []StabilityTier{TierStable, TierPartial, TierPlanned, TierDeprecated} {
		if !strings.Contains(strings.ToLower(doc), string(tier)) {
			t.Errorf("stability tier %q is not documented in %s", tier, path)
		}
	}
}

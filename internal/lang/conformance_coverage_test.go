package lang

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// corpusConstructs are the constructs the single-file conformance corpus
// exercises directly. TestConformanceCoversEveryConstruct verifies each one is
// actually present in a corpus file, so the claim cannot outlive its case.
var corpusConstructs = map[string]bool{
	"package":                     true,
	"use":                         true,
	"route":                       true,
	"title":                       true,
	"description":                 true,
	"build {}":                    true,
	"view {}":                     true,
	"style {}":                    true,
	"props":                       true,
	"component":                   true,
	"unsupported top-level block": true,
	"act block form":              true,
	"api block form":              true,
}

// integrationCoverage maps each construct the single-file corpus cannot exercise
// cleanly to a test file that does. These constructs need project context the
// corpus lacks: reactive g: directives reference a Go-typed state contract,
// endpoints need exported Go handlers, and several metadata keywords/blocks need
// sibling files or config. Paths are relative to this package and verified to
// exist by TestIntegrationCoverageFilesExist, so a renamed or deleted file
// breaks the gate instead of silently passing.
var integrationCoverage = map[string]string{
	// g: directives parsed by internal/view.
	"g:if":          "../view/view_test.go",
	"g:else-if":     "../view/view_test.go",
	"g:else":        "../view/view_test.go",
	"g:for":         "../view/server_list_test.go",
	"g:key":         "../view/view_test.go",
	"g:unsafe-html": "../view/view_test.go",
	"g:on:*":        "../view/view_test.go",

	// g: directives validated against component contracts by internal/compiler.
	"g:bind:value":   "../compiler/validate_test.go",
	"g:bind:checked": "../compiler/validate_test.go",
	"g:post":         "../compiler/validate_test.go",
	"g:target":       "../compiler/validate_test.go",
	"g:swap":         "../compiler/validate_test.go",
	"g:island":       "../compiler/validate_test.go",
	"g:event":        "../compiler/validate_test.go",
	"g:ref":          "../compiler/validate_test.go",
	"g:slot":         "../compiler/validate_test.go",
	"g:message:*":    "../compiler/validate_test.go",
	"g:command":      "../compiler/validate_contract_refs_test.go",
	"g:query":        "../compiler/validate_contract_refs_test.go",
	"g:subscribe":    "../compiler/validate_contract_refs_test.go",

	// Planned g: directives rejected by internal/view markup validation.
	"g:transition": "../view/view_test.go",
	"g:animate":    "../view/view_test.go",
	"g:window":     "../view/view_test.go",
	"g:document":   "../view/view_test.go",
	"g:body":       "../view/view_test.go",
	"g:head":       "../view/view_test.go",
	"g:await":      "../view/view_test.go",
	"g:async":      "../view/view_test.go",
	"g:use":        "../view/view_test.go",
	"g:action":     "../view/view_test.go",
	"g:attach":     "../view/view_test.go",

	// Blocks and keywords needing Go types, sibling files, or config.
	"import":     "../parser/syntax_test.go",
	"paths {}":   "../compiler/validate_test.go",
	"server {}":  "../compiler/validate_test.go",
	"client {}":  "../compiler/validate_test.go",
	"go {}":      "../compiler/backend_bindings_test.go",
	"store":      "../compiler/validate_test.go",
	"state":      "../compiler/validate_test.go",
	"emits":      "../compiler/validate_test.go",
	"page":       "../compiler/validate_test.go",
	"canonical":  "../compiler/validate_test.go",
	"image":      "../compiler/validate_test.go",
	"layout":     "../compiler/validate_test.go",
	"cache":      "../compiler/validate_test.go",
	"revalidate": "../compiler/validate_test.go",
	"error":      "../compiler/validate_test.go",
	"guard":      "../compiler/validate_test.go",
	"css":        "../compiler/validate_test.go",
	"wasm":       "../compiler/validate_test.go",
	"asset":      "../compiler/validate_test.go",
	"act":        "../compiler/backend_bindings_test.go",
	"api":        "../compiler/backend_bindings_test.go",
	"fragment":   "../compiler/backend_bindings_test.go",
}

// TestConformanceCoversEveryConstruct asserts every construct in the stability
// registry is accounted for — exercised by the single-file corpus or mapped to
// an integration test — the two sets do not overlap, and a corpus claim is
// backed by an actual corpus file. A new construct fails here until it is given
// real coverage.
func TestConformanceCoversEveryConstruct(t *testing.T) {
	corpus := readAllCorpus(t)

	for _, construct := range ConstructStabilities() {
		inCorpus := corpusConstructs[construct.Name]
		_, inIntegration := integrationCoverage[construct.Name]
		switch {
		case inCorpus && inIntegration:
			t.Errorf("construct %q is listed as both corpus and integration covered", construct.Name)
		case !inCorpus && !inIntegration:
			t.Errorf("construct %q (%s) has no corpus or integration coverage entry", construct.Name, construct.Kind)
		case inCorpus && !constructPresentInCorpus(construct, corpus):
			t.Errorf("construct %q claims corpus coverage but no corpus file exercises it", construct.Name)
		}
	}
}

// TestIntegrationCoverageFilesExist verifies every integration-coverage
// reference points at a real file, so a rename or deletion fails the gate.
func TestIntegrationCoverageFilesExist(t *testing.T) {
	for name, path := range integrationCoverage {
		if _, err := os.Stat(filepath.FromSlash(path)); err != nil {
			t.Errorf("integration coverage for %q points at missing file %q: %v", name, path, err)
		}
	}
}

// TestCoverageSetsHaveNoStaleEntries keeps the coverage maps honest: every entry
// must name a construct that still exists in the registry.
func TestCoverageSetsHaveNoStaleEntries(t *testing.T) {
	registry := map[string]bool{}
	for _, construct := range ConstructStabilities() {
		registry[construct.Name] = true
	}
	for name := range corpusConstructs {
		if !registry[name] {
			t.Errorf("corpusConstructs entry %q is not a registry construct", name)
		}
	}
	for name := range integrationCoverage {
		if !registry[name] {
			t.Errorf("integrationCoverage entry %q is not a registry construct", name)
		}
	}
}

func readAllCorpus(t *testing.T) string {
	t.Helper()
	var builder strings.Builder
	for _, dir := range []string{"testdata/conformance/accept", "testdata/conformance/reject"} {
		base := filepath.FromSlash(dir)
		for _, name := range conformanceFiles(t, base) {
			builder.Write(readConformanceFile(t, filepath.Join(base, name)))
			builder.WriteByte('\n')
		}
	}
	return builder.String()
}

// constructPresentInCorpus checks the corpus actually exercises a construct: by
// its diagnostic code (reject cases), its `name {` header (block constructs), or
// a line beginning with its name (keywords and endpoints).
func constructPresentInCorpus(construct ConstructStability, corpus string) bool {
	if construct.DiagnosticCode != "" {
		return strings.Contains(corpus, construct.DiagnosticCode)
	}
	if header := strings.TrimSuffix(construct.Name, " {}"); header != construct.Name {
		return regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(header) + `[ \t]*\{`).MatchString(corpus)
	}
	return regexp.MustCompile(`(?m)^[ \t]*` + regexp.QuoteMeta(construct.Name) + `([ \t]|"|$)`).MatchString(corpus)
}

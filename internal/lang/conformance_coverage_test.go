package lang

import "testing"

// corpusConstructs are the constructs the single-file conformance corpus
// exercises directly (in an accept case, or a reject case asserting the
// construct's diagnostic code).
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
// cleanly to the test surface that does. These constructs need project context
// the corpus lacks: reactive g: directives reference a Go-typed state contract,
// endpoints need exported Go handlers, and several metadata keywords/blocks need
// sibling files or config. Keeping the mapping here makes "full construct
// coverage" a verifiable gate.
var integrationCoverage = map[string]string{
	// g: directives: parsed by internal/view, validated against component
	// contracts by internal/compiler.
	"g:if":           "internal/view view_test.go; internal/compiler validate_component_view",
	"g:else-if":      "internal/view view_test.go",
	"g:else":         "internal/view view_test.go",
	"g:for":          "internal/view view_test.go",
	"g:key":          "internal/view view_test.go",
	"g:html":         "internal/view view_test.go",
	"g:bind:value":   "internal/compiler validate_component_client",
	"g:bind:checked": "internal/compiler validate_component_client",
	"g:post":         "internal/compiler validate_component_view (forms)",
	"g:target":       "internal/compiler validate_component_view (forms)",
	"g:swap":         "internal/compiler validate_component_view (forms)",
	"g:island":       "internal/compiler validate_component_client (islands)",
	"g:command":      "internal/compiler validate_contract_refs",
	"g:query":        "internal/compiler validate_contract_refs",
	"g:event":        "internal/view directives (domain-event explainer)",
	"g:ref":          "internal/compiler validate_component_client",
	"g:slot":         "internal/compiler validate_component_view (slots)",
	"g:on:*":         "internal/view view_test.go (event directives)",
	"g:message:*":    "internal/compiler validate_component_view (validation messages)",

	// Planned g: directives: rejected by internal/view markup validation.
	"g:transition": "internal/view directives (unsupported markup)",
	"g:animate":    "internal/view directives (unsupported markup)",
	"g:window":     "internal/view directives (unsupported markup)",
	"g:document":   "internal/view directives (unsupported markup)",
	"g:body":       "internal/view directives (unsupported markup)",
	"g:head":       "internal/view directives (unsupported markup)",
	"g:await":      "internal/view directives (unsupported markup)",
	"g:async":      "internal/view directives (unsupported markup)",
	"g:use":        "internal/view directives (unsupported markup)",
	"g:action":     "internal/view directives (unsupported markup)",
	"g:attach":     "internal/view directives (unsupported markup)",

	// Blocks and keywords needing Go types, sibling files, or config.
	"import":     "internal/parser syntax_test (Go imports)",
	"paths {}":   "internal/compiler routes_test (dynamic SPA paths)",
	"load {}":    "internal/compiler validate_page (SSR load)",
	"client {}":  "internal/compiler validate_component_client",
	"go {}":      "internal/compiler backend_bindings_test",
	"store":      "internal/compiler validate_page (page stores)",
	"state":      "internal/compiler validate_component_client",
	"emits":      "internal/compiler validate_identity (component emits)",
	"page":       "internal/compiler validate_identity (page id)",
	"canonical":  "internal/compiler validate_page (head metadata)",
	"image":      "internal/compiler validate_page (head metadata)",
	"layout":     "internal/compiler validate_identity (layouts)",
	"cache":      "internal/compiler validate_page (cache policy)",
	"revalidate": "internal/compiler validate_page (revalidate policy)",
	"error":      "internal/compiler validate_errors (error pages)",
	"guard":      "internal/compiler validate_page (guards)",
	"css":        "internal/compiler validate_page (css selection)",
	"wasm":       "internal/compiler validate_component_fingerprint (wasm islands)",
	"asset":      "internal/compiler validate_assets",
	"act":        "internal/compiler backend_bindings_test (actions)",
	"api":        "internal/compiler backend_bindings_test (apis)",
	"fragment":   "internal/compiler backend_bindings_test (fragments)",
}

// TestConformanceCoversEveryConstruct asserts every construct in the stability
// registry is accounted for — exercised by the single-file corpus or mapped to
// the integration test that covers it — and that the two coverage sets do not
// overlap. A new construct fails here until it is given coverage, so "full
// construct coverage" cannot silently regress.
func TestConformanceCoversEveryConstruct(t *testing.T) {
	for _, construct := range ConstructStabilities() {
		inCorpus := corpusConstructs[construct.Name]
		_, inIntegration := integrationCoverage[construct.Name]
		switch {
		case inCorpus && inIntegration:
			t.Errorf("construct %q is listed as both corpus and integration covered", construct.Name)
		case !inCorpus && !inIntegration:
			t.Errorf("construct %q (%s) has no corpus or integration coverage entry", construct.Name, construct.Kind)
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

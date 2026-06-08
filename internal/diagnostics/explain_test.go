package diagnostics

import "testing"

func TestExplainReturnsRegistryExplanation(t *testing.T) {
	explanation, ok := Explain("missing_ssr_addon")
	if !ok {
		t.Fatal("expected explanation")
	}
	if explanation.Code != "missing_ssr_addon" || explanation.Area != "rendering" || explanation.Stability != StabilityStable {
		t.Fatalf("unexpected explanation metadata: %#v", explanation)
	}
	if explanation.Details == "" || len(explanation.NextSteps) == 0 || explanation.Invalid == "" || explanation.Fixed == "" {
		t.Fatalf("expected detailed explanation, got %#v", explanation)
	}
}

func TestExplainFallsBackToRegistrySummary(t *testing.T) {
	explanation, ok := Explain("duplicate_route")
	if !ok {
		t.Fatal("expected explanation")
	}
	if explanation.Summary == "" || len(explanation.NextSteps) == 0 {
		t.Fatalf("expected fallback summary and next steps, got %#v", explanation)
	}
}

func TestSuggestionsReturnsClosestCodes(t *testing.T) {
	suggestions := Suggestions("missing_ssr_adon", 3)
	if len(suggestions) == 0 || suggestions[0] != "missing_ssr_addon" {
		t.Fatalf("expected missing_ssr_addon suggestion, got %#v", suggestions)
	}
}

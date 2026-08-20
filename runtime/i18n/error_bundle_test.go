package i18n

import "testing"

func TestErrorBundleReusesTypedCatalogFallbackAndFormatting(t *testing.T) {
	bundle := NewErrorBundle("en", map[string]Catalog[ErrorCode]{
		"en": NewCatalog("en", map[ErrorCode]string{"missing": "Missing {name}"}),
	})
	message := UserMessage{Code: "missing", Default: "Default {name}", Vars: map[string]string{"name": "record"}}
	if got := bundle.Resolve("en", message); got != "Missing record" {
		t.Fatalf("unexpected localized message: %q", got)
	}
	if got := bundle.Resolve("fr", UserMessage{Code: "other", Default: "Default {name}", Vars: message.Vars}); got != "Default record" {
		t.Fatalf("unexpected fallback message: %q", got)
	}
}

package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func persistedStorePageNamed(id, route, storeName, typeName, initName, scope string) gwdkir.Page {
	return gwdkir.Page{
		ID:     id,
		Route:  route,
		Source: "pages/" + id + ".page.gwdk",
		Guards: []string{"public"},
		Imports: []gwdkir.Import{{
			Alias: "ui",
			Path:  "github.com/cssbruno/gowdk/testfixture/islands",
		}},
		Stores: []gwdkir.Store{{
			Name:    storeName,
			Type:    gwdkir.GoRef{Alias: "ui", Name: typeName},
			Init:    gwdkir.GoRef{Alias: "ui", Name: initName},
			Persist: scope,
		}},
		Blocks: gwdkir.Blocks{View: true, ViewBody: `<main>Page</main>`},
	}
}

func persistedStorePage(storeName, typeName, initName, scope string) gwdkir.Page {
	return gwdkir.Page{
		ID:     "cart",
		Route:  "/cart",
		Source: "pages/cart.page.gwdk",
		Guards: []string{"public"},
		Imports: []gwdkir.Import{{
			Alias: "ui",
			Path:  "github.com/cssbruno/gowdk/testfixture/islands",
		}},
		Stores: []gwdkir.Store{{
			Name:    storeName,
			Type:    gwdkir.GoRef{Alias: "ui", Name: typeName},
			Init:    gwdkir.GoRef{Alias: "ui", Name: initName},
			Persist: scope,
		}},
		Blocks: gwdkir.Blocks{View: true, ViewBody: `<main>Cart</main>`},
	}
}

func TestValidatePageAcceptsPersistedStore(t *testing.T) {
	for _, scope := range []string{"local", "session"} {
		t.Run(scope, func(t *testing.T) {
			page := persistedStorePage("cart", "CounterState", "NewCounterState", scope)
			diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
			if ValidationErrors(diagnostics).HasErrors() {
				t.Fatalf("persist %q should be valid, got %#v", scope, diagnostics)
			}
		})
	}
}

func TestValidatePageRejectsInvalidPersistScope(t *testing.T) {
	page := persistedStorePage("cart", "CounterState", "NewCounterState", "disk")
	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "page_store_persist_scope_invalid")
	if diagnostic == nil {
		t.Fatalf("missing page_store_persist_scope_invalid diagnostic: %#v", diagnostics)
	}
	if diagnostic.Severity != SeverityError {
		t.Fatalf("invalid persist scope should be an error, got severity %v", diagnostic.Severity)
	}
}

func TestValidatePageWarnsOnPersistedSecretField(t *testing.T) {
	// SessionState has a Token field, which resembles a secret.
	page := persistedStorePage("session", "SessionState", "NewSessionState", "local")
	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "page_store_persist_secret_field")
	if diagnostic == nil {
		t.Fatalf("missing page_store_persist_secret_field diagnostic: %#v", diagnostics)
	}
	if diagnostic.Severity != SeverityWarning {
		t.Fatalf("secret field should be a warning, got severity %v", diagnostic.Severity)
	}
	// A resembling-secret field name must not fail the build.
	if ValidationErrors(diagnostics).HasErrors() {
		t.Fatalf("secret field warning should not fail the build: %#v", diagnostics)
	}
}

func TestValidatePageWarnsOnNestedPersistedSecretField(t *testing.T) {
	// ProfileState has no top-level secret-like field, but nests Account.Token.
	// Persistence writes the whole Account value, so the nested secret must be
	// flagged even though the top-level scan alone would miss it.
	page := persistedStorePage("profile", "ProfileState", "NewProfileState", "local")
	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "page_store_persist_secret_field")
	if diagnostic == nil {
		t.Fatalf("missing nested page_store_persist_secret_field diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(diagnostic.Message, "Account.Token") {
		t.Fatalf("expected the nested field path Account.Token in the message, got %q", diagnostic.Message)
	}
	if diagnostic.Severity != SeverityWarning {
		t.Fatalf("nested secret field should be a warning, got severity %v", diagnostic.Severity)
	}
	if ValidationErrors(diagnostics).HasErrors() {
		t.Fatalf("nested secret field warning should not fail the build: %#v", diagnostics)
	}
}

func TestValidateWarnsOnPersistedStoreScopeConflict(t *testing.T) {
	// Same store name and shape but different persist scopes share one storage
	// key; the effective scope then depends on navigation order.
	app := appFixture{Pages: []gwdkir.Page{
		persistedStorePageNamed("shop", "/shop", "cart", "CounterState", "NewCounterState", "local"),
		persistedStorePageNamed("checkout", "/checkout", "cart", "CounterState", "NewCounterState", "session"),
	}}
	diagnostics := ValidateProgramReport(gowdk.Config{}, app.program(gowdk.Config{}))
	d := firstDiagnostic(diagnostics, "page_store_persist_scope_conflict")
	if d == nil {
		t.Fatalf("missing page_store_persist_scope_conflict: %#v", diagnostics)
	}
	if d.Severity != SeverityWarning {
		t.Fatalf("scope conflict should be a warning, got %v", d.Severity)
	}
	// A mismatched-shape conflict is reported separately; same shape must not
	// also raise a key conflict.
	if other := firstDiagnostic(diagnostics, "page_store_persist_key_conflict"); other != nil {
		t.Fatalf("same-shape scope conflict must not also raise a key conflict: %#v", other)
	}
	if ValidationErrors(diagnostics).HasErrors() {
		t.Fatalf("scope conflict alone must not fail the build: %#v", diagnostics)
	}
}

func TestValidatePageDoesNotPersistCheckUnpersistedStore(t *testing.T) {
	// A non-persisted store with a secret-looking field must stay silent.
	page := persistedStorePage("session", "SessionState", "NewSessionState", "")
	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if d := firstDiagnostic(diagnostics, "page_store_persist_secret_field"); d != nil {
		t.Fatalf("unexpected secret-field warning on a non-persisted store: %#v", d)
	}
}

func TestValidateWarnsOnPersistedStoreKeyConflict(t *testing.T) {
	// Same store name, different shapes, both persisted -> shared key, divergent hash.
	app := appFixture{Pages: []gwdkir.Page{
		persistedStorePageNamed("shop", "/shop", "cart", "CounterState", "NewCounterState", "local"),
		persistedStorePageNamed("notes", "/notes", "cart", "TextState", "NewTextState", "local"),
	}}
	diagnostics := ValidateProgramReport(gowdk.Config{}, app.program(gowdk.Config{}))
	d := firstDiagnostic(diagnostics, "page_store_persist_key_conflict")
	if d == nil {
		t.Fatalf("missing page_store_persist_key_conflict: %#v", diagnostics)
	}
	if d.Severity != SeverityWarning {
		t.Fatalf("key conflict should be a warning, got %v", d.Severity)
	}
	if ValidationErrors(diagnostics).HasErrors() {
		t.Fatalf("key conflict alone must not fail the build: %#v", diagnostics)
	}
}

func TestValidateAllowsSharedPersistedStoreAcrossPages(t *testing.T) {
	// Same name AND same shape across pages is intentional cross-route sharing.
	app := appFixture{Pages: []gwdkir.Page{
		persistedStorePageNamed("shop", "/shop", "cart", "CounterState", "NewCounterState", "local"),
		persistedStorePageNamed("checkout", "/checkout", "cart", "CounterState", "NewCounterState", "local"),
	}}
	diagnostics := ValidateProgramReport(gowdk.Config{}, app.program(gowdk.Config{}))
	if d := firstDiagnostic(diagnostics, "page_store_persist_key_conflict"); d != nil {
		t.Fatalf("same-shape shared persisted store must not conflict: %#v", d)
	}
}

func TestLooksLikeSecretFieldName(t *testing.T) {
	secrets := []string{"Token", "Password", "APIKey", "api_key", "Secret", "Credential", "Authenticated", "SSN", "PrivateKey"}
	for _, name := range secrets {
		if !looksLikeSecretFieldName(name) {
			t.Errorf("expected %q to be flagged as secret-like", name)
		}
	}
	safe := []string{"Count", "Open", "Query", "Items", "Theme", "Density"}
	for _, name := range safe {
		if looksLikeSecretFieldName(name) {
			t.Errorf("did not expect %q to be flagged as secret-like", name)
		}
	}
}

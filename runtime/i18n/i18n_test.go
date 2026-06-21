package i18n

import "testing"

type messageKey string

const (
	messageGreeting messageKey = "greeting"
	messageCart     messageKey = "cart"
)

func TestCatalogLooksUpTypedMessages(t *testing.T) {
	catalog := NewCatalog("en", map[messageKey]string{
		messageGreeting: "Hello",
		messageCart:     "{count} items",
	})

	if got := catalog.Must(messageGreeting); got != "Hello" {
		t.Fatalf("unexpected message: %q", got)
	}
	formatted, ok := catalog.Format(messageCart, map[string]string{"count": "3"})
	if !ok || formatted != "3 items" {
		t.Fatalf("unexpected formatted message: %q ok=%v", formatted, ok)
	}
	if missing := catalog.MissingKeys([]messageKey{messageGreeting, messageKey("missing")}); len(missing) != 1 || missing[0] != "missing" {
		t.Fatalf("unexpected missing keys: %#v", missing)
	}
}

func TestBundleFallsBackToDefaultCatalog(t *testing.T) {
	bundle := NewBundle("en", map[string]Catalog[messageKey]{
		"en": NewCatalog("en", map[messageKey]string{messageGreeting: "Hello"}),
		"pt": NewCatalog("pt", map[messageKey]string{messageGreeting: "Ola"}),
	})

	catalog, ok := bundle.Catalog("pt")
	if !ok || catalog.Must(messageGreeting) != "Ola" {
		t.Fatalf("expected pt catalog, got %#v ok=%v", catalog, ok)
	}
	catalog, ok = bundle.Catalog("de")
	if !ok || catalog.Must(messageGreeting) != "Hello" {
		t.Fatalf("expected default catalog, got %#v ok=%v", catalog, ok)
	}
}

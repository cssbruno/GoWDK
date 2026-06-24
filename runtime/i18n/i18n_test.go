package i18n

import (
	"strings"
	"testing"
	"time"
)

type messageKey string

const (
	messageGreeting messageKey = "greeting"
	messageCart     messageKey = "cart"
	messageUnused   messageKey = "unused"
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

func TestBundleCheckReportsCatalogCompleteness(t *testing.T) {
	required := []MessageReference[messageKey]{
		Ref(messageCart, "home.page.gwdk", 12, 8),
		Ref(messageGreeting, "home.page.gwdk", 11, 8),
	}
	bundle := NewBundle("en", map[string]Catalog[messageKey]{
		"en": NewCatalog("en", map[messageKey]string{
			messageGreeting: "Hello",
			messageCart:     "{count} items",
			messageUnused:   "Stale",
		}),
		"pt": NewCatalog("pt", map[messageKey]string{
			messageGreeting: "Ola",
		}),
	})

	report := bundle.Check(required)
	if report.OK() {
		t.Fatalf("expected incomplete report")
	}
	if len(report.Catalogs) != 2 || report.Catalogs[0].Locale != "en" || report.Catalogs[1].Locale != "pt" {
		t.Fatalf("expected deterministic catalog order, got %#v", report.Catalogs)
	}
	if len(report.Catalogs[0].Unused) != 1 || report.Catalogs[0].Unused[0] != messageUnused {
		t.Fatalf("expected stale English key, got %#v", report.Catalogs[0])
	}
	if len(report.Catalogs[1].Missing) != 1 || report.Catalogs[1].Missing[0].Key != messageCart || report.Catalogs[1].Missing[0].Source != "home.page.gwdk" {
		t.Fatalf("expected missing Portuguese cart key with source, got %#v", report.Catalogs[1])
	}
	if text := report.Error(); !strings.Contains(text, `"missing"`) || !strings.Contains(text, `"unused"`) {
		t.Fatalf("expected JSON report text, got %s", text)
	}
}

func TestBundleCheckAcceptsCompleteCatalogsAndBuildsTemplate(t *testing.T) {
	required := []MessageReference[messageKey]{
		Ref(messageGreeting, "home.page.gwdk", 10, 3),
		Ref(messageCart, "home.page.gwdk", 11, 3),
	}
	bundle := NewBundle("en", map[string]Catalog[messageKey]{
		"en": NewCatalog("en", map[messageKey]string{
			messageGreeting: "Hello",
			messageCart:     "{count} items",
		}),
		"pt": NewCatalog("pt", map[messageKey]string{
			messageGreeting: "Ola",
			messageCart:     "{count} itens",
		}),
	})

	if report := bundle.Check(required); !report.OK() {
		t.Fatalf("expected complete catalogs, got %s", report.Error())
	}
	template := bundle.Template("pt", append(required, Key(messageGreeting)))
	if template.Locale != "pt" || len(template.Entries) != 2 {
		t.Fatalf("unexpected template: %#v", template)
	}
	if template.Entries[0].Key != messageCart || template.Entries[0].Value != "{count} itens" {
		t.Fatalf("expected sorted cart entry with existing value, got %#v", template.Entries[0])
	}
	if template.Entries[1].Key != messageGreeting || template.Entries[1].Value != "Ola" || template.Entries[1].Line != 10 {
		t.Fatalf("expected greeting entry with source metadata, got %#v", template.Entries[1])
	}
}

func TestPluralNumberAndDateTimeFormatting(t *testing.T) {
	forms := PluralForms{
		Zero:  "No items",
		One:   "{count} item",
		Other: "{count} items",
	}
	if got := FormatPlural("en", 0, forms, nil); got != "No items" {
		t.Fatalf("unexpected zero plural: %q", got)
	}
	if got := FormatPlural("en", 1, forms, nil); got != "1 item" {
		t.Fatalf("unexpected one plural: %q", got)
	}
	if got := FormatPlural("en", 2, forms, nil); got != "2 items" {
		t.Fatalf("unexpected other plural: %q", got)
	}
	if got := Cardinal("ja", 1); got != PluralOther {
		t.Fatalf("expected ja to use other in bounded formatter, got %q", got)
	}

	if got := FormatNumber("en", 1234.5, NumberFormatOptions{MaxFractionDigits: 2}); got != "1,234.5" {
		t.Fatalf("unexpected English number: %q", got)
	}
	if got := FormatNumber("pt-BR", 1234.5, NumberFormatOptions{MinFractionDigits: 2, MaxFractionDigits: 2}); got != "1.234,50" {
		t.Fatalf("unexpected Portuguese number: %q", got)
	}

	moment := time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)
	if got := FormatDate("en", moment, DateMedium); got != "Jan 2, 2026" {
		t.Fatalf("unexpected English date: %q", got)
	}
	if got := FormatDate("pt-BR", moment, DateShort); got != "02/01/2026" {
		t.Fatalf("unexpected Portuguese date: %q", got)
	}
	if got := FormatTime("en", moment, TimeShort); got != "3:04 PM" {
		t.Fatalf("unexpected English time: %q", got)
	}
	if got := FormatTime("pt-BR", moment, TimeMedium); got != "15:04:05" {
		t.Fatalf("unexpected Portuguese time: %q", got)
	}
}

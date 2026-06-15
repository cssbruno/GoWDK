package seo

import (
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestAddonRegistersSEOFeature(t *testing.T) {
	addon := Addon(Options{BaseURL: "https://example.com"})

	if addon.Name() != "seo" {
		t.Fatalf("unexpected addon name %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureSEO) {
		t.Fatal("expected SEO feature to be enabled")
	}
}

func TestAddonReturnsSEOOptionsAndExtraURLProvider(t *testing.T) {
	addon := Addon(Options{
		BaseURL:  "https://example.com",
		Disallow: []string{"/admin"},
		ExtraURLs: []URL{{
			Loc: "/rss.xml",
		}},
		ExtraURLProvider: func() []gowdk.SEOURL {
			return []gowdk.SEOURL{{Loc: "/feed.xml"}}
		},
	})

	provider, ok := addon.(gowdk.SEOProvider)
	if !ok {
		t.Fatalf("expected SEOProvider, got %T", addon)
	}
	options := provider.SEOOptions()
	if options.BaseURL != "https://example.com" || len(options.Disallow) != 1 || options.Disallow[0] != "/admin" {
		t.Fatalf("unexpected options: %#v", options)
	}
	if len(options.ExtraURLs) != 2 || options.ExtraURLs[0].Loc != "/rss.xml" || options.ExtraURLs[1].Loc != "/feed.xml" {
		t.Fatalf("unexpected extra URLs: %#v", options.ExtraURLs)
	}
	if options.ExtraURLProvider != nil {
		t.Fatal("expected provider function to be materialized and cleared")
	}
}

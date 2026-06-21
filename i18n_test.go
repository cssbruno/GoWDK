package gowdk

import (
	"strings"
	"testing"
)

func TestI18NConfigLocalizesRoutes(t *testing.T) {
	config := I18NConfig{
		Locales: []LocaleConfig{
			{Code: "en"},
			{Code: "pt-BR", PathPrefix: "/br"},
		},
		DefaultLocale: "en",
	}

	routes := config.LocalizedRoutes("/about")
	if len(routes) != 2 {
		t.Fatalf("expected two localized routes, got %#v", routes)
	}
	if routes[0].Locale != "en" || routes[0].Route != "/en/about" {
		t.Fatalf("unexpected first route: %#v", routes[0])
	}
	if routes[1].Locale != "pt-BR" || routes[1].Route != "/br/about" {
		t.Fatalf("unexpected second route: %#v", routes[1])
	}
	if root := config.LocalizeRoute("/", "en"); root != "/en/" {
		t.Fatalf("expected localized root, got %q", root)
	}
	caseRoute := I18NConfig{Locales: []LocaleConfig{{Code: "en-GB"}}}.LocalizedRoutes("/news")
	if len(caseRoute) != 1 || caseRoute[0].Locale != "en-GB" || caseRoute[0].Route != "/en-gb/news" {
		t.Fatalf("expected default prefix to lowercase locale code, got %#v", caseRoute)
	}
}

func TestI18NConfigCanOmitDefaultPrefix(t *testing.T) {
	config := I18NConfig{
		Locales:           []LocaleConfig{{Code: "en"}, {Code: "pt"}},
		OmitDefaultPrefix: true,
	}

	routes := config.LocalizedRoutes("/")
	if len(routes) != 2 || routes[0].Route != "/" || routes[1].Route != "/pt/" {
		t.Fatalf("unexpected localized root routes: %#v", routes)
	}
}

func TestI18NConfigOmittedDefaultPrefixDoesNotReserveSyntheticPrefix(t *testing.T) {
	config := I18NConfig{
		Locales: []LocaleConfig{
			{Code: "en"},
			{Code: "en-GB", PathPrefix: "/en"},
		},
		OmitDefaultPrefix: true,
	}

	if err := config.Validate(); err != nil {
		t.Fatalf("expected omitted default prefix to allow /en for another locale: %v", err)
	}
	routes := config.LocalizedRoutes("/")
	if len(routes) != 2 || routes[0].Route != "/" || routes[1].Route != "/en/" {
		t.Fatalf("unexpected routes: %#v", routes)
	}
}

func TestI18NConfigValidateRejectsUnsafePolicy(t *testing.T) {
	tests := []struct {
		name   string
		config I18NConfig
		want   string
	}{
		{
			name:   "missing code",
			config: I18NConfig{Locales: []LocaleConfig{{}}},
			want:   "Code is required",
		},
		{
			name:   "invalid code",
			config: I18NConfig{Locales: []LocaleConfig{{Code: "e"}}},
			want:   "not a supported locale code",
		},
		{
			name:   "duplicate code",
			config: I18NConfig{Locales: []LocaleConfig{{Code: "en"}, {Code: "EN"}}},
			want:   "duplicate locale",
		},
		{
			name:   "unsafe prefix",
			config: I18NConfig{Locales: []LocaleConfig{{Code: "en", PathPrefix: "/../en"}}},
			want:   "unsafe path segment",
		},
		{
			name:   "missing default",
			config: I18NConfig{Locales: []LocaleConfig{{Code: "en"}}, DefaultLocale: "pt"},
			want:   "not declared",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestBuildParamsLocaleCode(t *testing.T) {
	params := BuildParams{Locale: " pt-BR "}
	if params.LocaleCode() != "pt-BR" {
		t.Fatalf("unexpected locale code: %q", params.LocaleCode())
	}
}

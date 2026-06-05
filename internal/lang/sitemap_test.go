package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
)

func TestSiteMapJSONIncludesMovableSourceAndRoute(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "pages", "marketing", "home.page.gwdk")
	dashboard := filepath.Join(root, "anywhere", "dashboard.page.gwdk")
	writeSiteMapFile(t, home, `@page home
@route "/"
@layout root

view {
}
`)
	writeSiteMapFile(t, dashboard, `@page dashboard
@route "/dashboard"
@layout root, dashboard
@render ssr
@guard auth.required

load {
}

view {
}
`)

	payload, diagnostics := SiteMapJSON(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, []string{home, dashboard})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	output := string(payload)
	for _, expected := range []string{
		`"id": "home"`,
		`"route": "/"`,
		`"source": "` + home + `"`,
		`"id": "dashboard"`,
		`"render": "ssr"`,
		`"auth.required"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in sitemap JSON:\n%s", expected, output)
		}
	}
}

func TestSiteMapJSONRunsCompilerValidation(t *testing.T) {
	root := t.TempDir()
	dashboard := filepath.Join(root, "dashboard.page.gwdk")
	writeSiteMapFile(t, dashboard, `@page dashboard
@route "/dashboard"
@render ssr

view {
}
`)

	payload, diagnostics := SiteMapJSON(gowdk.Config{}, []string{dashboard})
	if !diagnostics.HasErrors() {
		t.Fatal("expected missing SSR addon diagnostics")
	}
	if payload != nil {
		t.Fatalf("expected no sitemap payload on validation errors, got %s", payload)
	}
	if got := diagnostics[0].Code; got != "missing_ssr_addon" {
		t.Fatalf("expected missing_ssr_addon diagnostic, got %q", got)
	}
}

func writeSiteMapFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

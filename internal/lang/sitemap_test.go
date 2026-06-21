package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	contractsaddon "github.com/cssbruno/gowdk/addons/contracts"
	"github.com/cssbruno/gowdk/addons/realtime"
	"github.com/cssbruno/gowdk/addons/ssr"
)

func TestSiteMapJSONIncludesMovableSourceAndRoute(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "pages", "marketing", "home.page.gwdk")
	dashboard := filepath.Join(root, "anywhere", "dashboard.page.gwdk")
	writeSiteMapFile(t, home, `package app

page home
route "/"
guard public
layout root

view {
}
`)
	writeSiteMapFile(t, dashboard, `package app

page dashboard
route "/dashboard"
layout root, dashboard
guard auth.required

server {
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
		`"routes": [`,
		`"kind": "spa"`,
		`"kind": "ssr"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in sitemap JSON:\n%s", expected, output)
		}
	}
}

func TestSiteMapJSONIncludesLocalizedRoutes(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home.page.gwdk")
	writeSiteMapFile(t, home, `package app

page home
route "/"
guard public

view {
}
`)

	payload, diagnostics := SiteMapJSON(gowdk.Config{I18N: gowdk.I18NConfig{
		Locales: []gowdk.LocaleConfig{{Code: "en"}, {Code: "pt"}},
	}}, []string{home})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	output := string(payload)
	for _, expected := range []string{
		`"route": "/en/"`,
		`"locale": "en"`,
		`"route": "/pt/"`,
		`"locale": "pt"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in sitemap JSON:\n%s", expected, output)
		}
	}
}

func TestSiteMapJSONIncludesEndpointGraph(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "contact.page.gwdk")
	writeSiteMapFile(t, page, `package app

page contact
route "/contact"
guard public

act Submit POST "/contact"
api Health GET "/api/health"
fragment Summary GET "/contact/summary" "#summary" {
  <section>Summary</section>
}

view {
  <main>Contact</main>
}
`)

	payload, diagnostics := SiteMapJSON(gowdk.Config{}, []string{page})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	output := string(payload)
	for _, expected := range []string{
		`"endpoints": [`,
		`"kind": "action"`,
		`"kind": "api"`,
		`"kind": "fragment"`,
		`"symbol": "Submit"`,
		`"symbol": "Health"`,
		`"symbol": "Summary"`,
		`"source": "` + page + `"`,
		`"sourceSpan": {`,
		`"handler": "fragments.ContactSummary"`,
		`"bindingStatus": "missing"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in sitemap JSON:\n%s", expected, output)
		}
	}
}

func TestSiteMapJSONIncludesContractReferenceEndpoints(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	page := filepath.Join(repoRoot, "examples", "contracts", "patients.page.gwdk")

	payload, diagnostics := SiteMapJSONWithOptions(
		gowdk.Config{Addons: []gowdk.Addon{contractsaddon.Addon(), realtime.Addon()}},
		[]string{page},
		CheckOptions{ProjectRoot: repoRoot},
	)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	output := string(payload)
	for _, expected := range []string{
		`"kind": "command"`,
		`"kind": "query"`,
		`"endpointSource": "contract"`,
		`"symbol": "patients.CreatePatient"`,
		`"symbol": "patients.GetPatientPage"`,
		`"source": "` + page + `"`,
		`"sourceSpan": {`,
		`"handler": "contracts.command.patients.CreatePatient"`,
		`"handler": "contracts.query.patients.GetPatientPage"`,
		`"contract": {`,
		`"status": "unknown"`,
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected %q in sitemap JSON:\n%s", expected, output)
		}
	}
}

func TestSiteMapJSONRunsCompilerValidation(t *testing.T) {
	root := t.TempDir()
	dashboard := filepath.Join(root, "dashboard.page.gwdk")
	writeSiteMapFile(t, dashboard, `package app

page dashboard
route "/dashboard"
guard public

go server {
}

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

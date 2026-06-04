package compiler

import (
	"strings"
	"testing"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/addons/ssr"
	"github.com/gowdk/gowdk/internal/manifest"
)

func TestValidatePageRejectsSSRWithoutAddon(t *testing.T) {
	page := manifest.Page{ID: "dashboard", Route: "/dashboard", Render: gowdk.SSR}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Code != "missing_ssr_addon" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
	if !strings.Contains(diagnostics[0].Message, "enable ssr.Addon()") {
		t.Fatalf("diagnostic should suggest enabling ssr addon: %s", diagnostics[0].Message)
	}
}

func TestValidatePageAllowsSSRWithAddon(t *testing.T) {
	page := manifest.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Blocks: manifest.Blocks{
			Load: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, page)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicatePageIDsAndComponentNames(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{
			{ID: "home", Route: "/", Source: "pages/home.page.gwdk"},
			{ID: "home", Route: "/again", Source: "pages/home-again.page.gwdk"},
		},
		Components: []manifest.Component{
			{Name: "Hero", Source: "components/hero.cmp.gwdk"},
			{Name: "Hero", Source: "components/hero-copy.cmp.gwdk"},
		},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate identity diagnostics")
	}
	diagnostics, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	codes := map[string]bool{}
	for _, diagnostic := range diagnostics {
		codes[diagnostic.Code] = true
		if diagnostic.Source == "" {
			t.Fatalf("expected source on duplicate diagnostic: %#v", diagnostic)
		}
	}
	if !codes["duplicate_page_id"] {
		t.Fatalf("missing duplicate_page_id diagnostic: %#v", diagnostics)
	}
	if !codes["duplicate_component_name"] {
		t.Fatalf("missing duplicate_component_name diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRequiresPathsForStaticDynamicRoutes(t *testing.T) {
	page := manifest.Page{ID: "patients.show", Route: "/patients/{id}", Render: gowdk.Static}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Code != "static_dynamic_route_missing_paths" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
	if !strings.Contains(diagnostics[0].Message, "add paths") {
		t.Fatalf("diagnostic should suggest paths block: %s", diagnostics[0].Message)
	}
}

func TestValidatePageAllowsStaticDynamicRoutesWithPaths(t *testing.T) {
	page := manifest.Page{ID: "blog.post", Route: "/blog/{slug}", Render: gowdk.Static, Paths: true}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageAllowsStaticActionsWithoutSSR(t *testing.T) {
	page := manifest.Page{
		ID:     "newsletter",
		Route:  "/newsletter",
		Render: gowdk.Static,
		Blocks: manifest.Blocks{
			Actions: []manifest.Action{{Name: "subscribe"}},
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageRejectsLoadOnStaticPage(t *testing.T) {
	page := manifest.Page{
		ID:     "newsletter",
		Route:  "/newsletter",
		Render: gowdk.Static,
		Blocks: manifest.Blocks{
			Load: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Code != "load_requires_request_render" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
}

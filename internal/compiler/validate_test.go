package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestValidatePageRejectsSSRWithoutAddon(t *testing.T) {
	page := manifest.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Blocks: manifest.Blocks{
			View: true,
		},
	}

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
			View: true,
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
			{ID: "home", Route: "/", Source: "pages/home.page.gwdk", Blocks: manifest.Blocks{View: true}},
			{ID: "home", Route: "/again", Source: "pages/home-again.page.gwdk", Blocks: manifest.Blocks{View: true}},
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

func TestValidateManifestResolvesLayoutsByID(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:      "dashboard",
			Route:   "/dashboard",
			Layouts: []string{"root", "missing"},
			Source:  "pages/dashboard.page.gwdk",
			Blocks:  manifest.Blocks{View: true},
		}},
		Layouts: []manifest.Layout{{
			ID:     "root",
			Source: "layouts/root.layout.gwdk",
		}},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown layout diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_layout_id") {
		t.Fatalf("missing unknown_layout_id diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicateLayoutIDs(t *testing.T) {
	app := manifest.Manifest{
		Layouts: []manifest.Layout{
			{ID: "root", Source: "layouts/root.layout.gwdk"},
			{ID: "root", Source: "layouts/root-copy.layout.gwdk"},
		},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate layout diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "duplicate_layout_id") {
		t.Fatalf("missing duplicate_layout_id diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicatePageRoutes(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{
			{ID: "blog.post", Route: "/blog/{slug}", Paths: true, Source: "pages/blog-post.page.gwdk", Blocks: manifest.Blocks{View: true}},
			{ID: "blog.entry", Route: "/blog/{id}", Paths: true, Source: "pages/blog-entry.page.gwdk", Blocks: manifest.Blocks{View: true}},
		},
	}

	err := ValidateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate route diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "duplicate_route") {
		t.Fatalf("missing duplicate_route diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsRouteMethodConflicts(t *testing.T) {
	t.Run("multiple actions on one route", func(t *testing.T) {
		app := manifest.Manifest{
			Pages: []manifest.Page{{
				ID:    "profile",
				Route: "/profile",
				Blocks: manifest.Blocks{
					View:    true,
					Actions: []manifest.Action{{Name: "save"}, {Name: "updateAvatar"}},
				},
			}},
		}

		err := ValidateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticCode(diagnostics, "route_method_conflict") {
			t.Fatalf("missing route_method_conflict diagnostic: %#v", diagnostics)
		}
	})

	t.Run("api default route conflicts with page get", func(t *testing.T) {
		app := manifest.Manifest{
			Pages: []manifest.Page{{
				ID:    "patients.index",
				Route: "/patients",
				Blocks: manifest.Blocks{
					View: true,
					APIs: []manifest.API{{Name: "index"}},
				},
			}},
		}

		err := ValidateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticCode(diagnostics, "route_method_conflict") {
			t.Fatalf("missing route_method_conflict diagnostic: %#v", diagnostics)
		}
	})
}

func TestValidateManifestAllowsSameRouteWithDifferentMethods(t *testing.T) {
	app := manifest.Manifest{
		Pages: []manifest.Page{{
			ID:    "newsletter",
			Route: "/newsletter",
			Blocks: manifest.Blocks{
				View:    true,
				Actions: []manifest.Action{{Name: "subscribe"}},
			},
		}},
	}

	if err := ValidateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected GET page plus POST action to be valid, got %v", err)
	}
}

func TestValidatePageRejectsMalformedRoutes(t *testing.T) {
	tests := []struct {
		name  string
		route string
	}{
		{name: "relative route", route: "patients"},
		{name: "query string", route: "/patients?page=1"},
		{name: "trailing slash", route: "/patients/"},
		{name: "dot segment", route: "/patients/../admin"},
		{name: "embedded param", route: "/blog/{slug}.html"},
		{name: "invalid param name", route: "/blog/{123}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := manifest.Page{ID: "patients", Route: test.route, Paths: true, Blocks: manifest.Blocks{View: true}}

			diagnostics := ValidatePage(gowdk.Config{}, page)
			if !hasDiagnosticCode(diagnostics, "malformed_route") {
				t.Fatalf("missing malformed_route diagnostic for %q: %#v", test.route, diagnostics)
			}
			if hasDiagnosticCode(diagnostics, "static_dynamic_route_missing_paths") {
				t.Fatalf("malformed route should not cascade into missing paths: %#v", diagnostics)
			}
		})
	}
}

func TestValidatePageRejectsDuplicateRouteParams(t *testing.T) {
	page := manifest.Page{ID: "blog.post", Route: "/blog/{slug}/{slug}", Paths: true, Blocks: manifest.Blocks{View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if !hasDiagnosticCode(diagnostics, "duplicate_route_param") {
		t.Fatalf("missing duplicate_route_param diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRequiresPathsForStaticDynamicRoutes(t *testing.T) {
	page := manifest.Page{ID: "patients.show", Route: "/patients/{id}", Render: gowdk.Static, Blocks: manifest.Blocks{View: true}}

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
	page := manifest.Page{ID: "blog.post", Route: "/blog/{slug}", Render: gowdk.Static, Paths: true, Blocks: manifest.Blocks{View: true}}

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
			View:    true,
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
			View: true,
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

func TestValidatePageRejectsAmbiguousHybridWithoutLoad(t *testing.T) {
	page := manifest.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %#v", diagnostics)
	}
	if diagnostics[0].Code != "hybrid_requires_explicit_request_policy" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
	if !strings.Contains(diagnostics[0].Message, "implicit SSR") {
		t.Fatalf("expected implicit SSR guidance: %s", diagnostics[0].Message)
	}
}

func TestValidatePageAllowsHybridWithExplicitLoadAndSSRAddon(t *testing.T) {
	page := manifest.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			Load: true,
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, page)
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageRejectsMissingViewBlock(t *testing.T) {
	page := manifest.Page{ID: "home", Route: "/", Render: gowdk.Static}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %#v", diagnostics)
	}
	if diagnostics[0].Code != "missing_view_block" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
}

func TestValidatePageRejectsInvalidCSSSelection(t *testing.T) {
	page := manifest.Page{
		ID:    "embed",
		Route: "/embed",
		CSS:   []string{"none", "forms"},
		Blocks: manifest.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if !hasDiagnosticCode(diagnostics, "invalid_css_selection") {
		t.Fatalf("missing invalid_css_selection diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRejectsDuplicateCSSSelection(t *testing.T) {
	page := manifest.Page{
		ID:    "home",
		Route: "/",
		CSS:   []string{"default", "forms", "forms"},
		Blocks: manifest.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if !hasDiagnosticCode(diagnostics, "duplicate_css_selection") {
		t.Fatalf("missing duplicate_css_selection diagnostic: %#v", diagnostics)
	}
}

func hasDiagnosticCode(diagnostics []ValidationError, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

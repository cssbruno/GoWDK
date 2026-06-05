package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestGenerateSSRPackageEmitsHandlerStubsForSSRAndAcceptedHybrid(t *testing.T) {
	source, err := GenerateSSRPackage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, manifest.Manifest{Pages: []manifest.Page{
		{
			ID:     "dashboard",
			Route:  "/dashboard",
			Render: gowdk.SSR,
			Blocks: manifest.Blocks{
				View: true,
			},
		},
		{
			ID:     "reports",
			Route:  "/reports",
			Render: gowdk.Hybrid,
			Blocks: manifest.Blocks{
				Load:     true,
				LoadBody: `=> { reports }`,
				View:     true,
			},
		},
		{
			ID:    "home",
			Route: "/",
			Blocks: manifest.Blocks{
				View: true,
			},
		},
	}}, SSRPackageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "ssr.go", source, parser.AllErrors); err != nil {
		t.Fatalf("generated SSR package is not valid Go: %v\n%s", err, source)
	}

	text := string(source)
	for _, want := range []string{
		`package ssr`,
		`"github.com/cssbruno/gowdk/addons/ssr"`,
		`"github.com/cssbruno/gowdk/runtime/render"`,
		`func RenderDashboard(w http.ResponseWriter, r *http.Request)`,
		`var _ ssr.LoadFunc = LoadReports`,
		`func LoadReports(ctx ssr.LoadContext) (map[string]any, error)`,
		`func RenderReports(w http.ResponseWriter, r *http.Request)`,
		`_, _ = LoadReports(ssr.NewLoadContext(r, nil))`,
		`_, _ = (render.Renderer{}).Render(r.Context())`,
		`http.StatusNotImplemented`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected generated source to contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "RenderHome") {
		t.Fatalf("expected app page omitted from SSR handlers:\n%s", text)
	}
}

func TestGenerateSSRPackageRejectsMissingSSRAddon(t *testing.T) {
	_, err := GenerateSSRPackage(gowdk.Config{}, manifest.Manifest{Pages: []manifest.Page{{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Blocks: manifest.Blocks{
			View: true,
		},
	}}}, SSRPackageOptions{})
	if err == nil {
		t.Fatal("expected missing SSR addon error")
	}
	if !strings.Contains(err.Error(), "SSR addon is not enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateSSRPackageRejectsAmbiguousHybrid(t *testing.T) {
	_, err := GenerateSSRPackage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, manifest.Manifest{Pages: []manifest.Page{{
		ID:     "reports",
		Route:  "/reports",
		Render: gowdk.Hybrid,
		Blocks: manifest.Blocks{
			View: true,
		},
	}}}, SSRPackageOptions{})
	if err == nil {
		t.Fatal("expected hybrid policy error")
	}
	if !strings.Contains(err.Error(), "implicit SSR") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateSSRPackageUsesCustomPackageName(t *testing.T) {
	source, err := GenerateSSRPackage(gowdk.Config{}, manifest.Manifest{}, SSRPackageOptions{PackageName: "requesttime"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(source)) != "package requesttime" {
		t.Fatalf("unexpected empty SSR package:\n%s", source)
	}
}

func TestGenerateSSRPackageRejectsInvalidPackageName(t *testing.T) {
	_, err := GenerateSSRPackage(gowdk.Config{}, manifest.Manifest{}, SSRPackageOptions{PackageName: "bad-name"})
	if err == nil {
		t.Fatal("expected invalid package name error")
	}
	if !strings.Contains(err.Error(), `invalid SSR package name "bad-name"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

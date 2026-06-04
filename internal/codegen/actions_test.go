package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestGenerateActionPackageEmitsRegistryBackedHTTPHandlers(t *testing.T) {
	source, err := GenerateActionPackage(gowdk.Config{}, manifest.Manifest{Pages: []manifest.Page{
		{
			ID:    "newsletter",
			Route: "/newsletter",
			Blocks: manifest.Blocks{
				View:    true,
				Actions: []manifest.Action{{Name: "subscribe"}},
			},
		},
		{
			ID:    "patients",
			Route: "/patients",
			Blocks: manifest.Blocks{
				View:    true,
				Actions: []manifest.Action{{Name: "refresh"}},
			},
		},
	}}, ActionPackageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "actions.go", source, parser.AllErrors); err != nil {
		t.Fatalf("generated action package is not valid Go: %v\n%s", err, source)
	}

	text := string(source)
	for _, want := range []string{
		`package actions`,
		`gowdkactions "github.com/cssbruno/gowdk/addons/actions"`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`var Registry = gowdkactions.Registry{}`,
		`func RegisterNewsletterSubscribe(handler gowdkactions.Handler)`,
		`func NewsletterSubscribe(w http.ResponseWriter, r *http.Request)`,
		`func PatientsRefresh(w http.ResponseWriter, r *http.Request)`,
		`values, err := gowdkactions.DecodeForm(r)`,
		`handler, ok := Registry["NewsletterSubscribe"]`,
		`result, err := handler(r.Context(), values)`,
		`gowdkresponse.WriteHTTP(w, result)`,
		`w.Header().Set("Cache-Control", "no-store")`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected generated source to contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateActionPackageUsesCustomPackageName(t *testing.T) {
	source, err := GenerateActionPackage(gowdk.Config{}, manifest.Manifest{}, ActionPackageOptions{PackageName: "handlers"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(source)) != "package handlers" {
		t.Fatalf("unexpected empty action package:\n%s", source)
	}
}

func TestGenerateActionPackageRejectsInvalidPackageName(t *testing.T) {
	_, err := GenerateActionPackage(gowdk.Config{}, manifest.Manifest{}, ActionPackageOptions{PackageName: "bad-name"})
	if err == nil {
		t.Fatal("expected invalid package name error")
	}
	if !strings.Contains(err.Error(), `invalid action package name "bad-name"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateActionPackageRejectsDuplicateGeneratedHandlerNames(t *testing.T) {
	_, err := GenerateActionPackage(gowdk.Config{}, manifest.Manifest{Pages: []manifest.Page{
		{
			ID:    "newsletter",
			Route: "/newsletter",
			Blocks: manifest.Blocks{
				View:    true,
				Actions: []manifest.Action{{Name: "subscribe"}},
			},
		},
		{
			ID:    "newsletter-subscribe",
			Route: "/newsletter-subscribe",
			Blocks: manifest.Blocks{
				View:    true,
				Actions: []manifest.Action{{Name: ""}},
			},
		},
	}}, ActionPackageOptions{})
	if err == nil {
		t.Fatal("expected duplicate handler error")
	}
	if !strings.Contains(err.Error(), `duplicate generated action handler name "NewsletterSubscribe"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

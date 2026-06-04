package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestGenerateAPIPackageEmitsHTTPHandlerStubs(t *testing.T) {
	source, err := GenerateAPIPackage(manifest.Manifest{Pages: []manifest.Page{
		{
			ID: "status",
			Blocks: manifest.Blocks{
				APIs: []manifest.API{{Name: "health", Method: "GET", Route: "/api/health"}},
			},
		},
		{
			ID: "patients.index",
			Blocks: manifest.Blocks{
				APIs: []manifest.API{{Name: "list", Method: "GET", Route: "/api/patients"}},
			},
		},
	}}, APIPackageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "api.go", source, parser.AllErrors); err != nil {
		t.Fatalf("generated API package is not valid Go: %v\n%s", err, source)
	}

	text := string(source)
	for _, want := range []string{
		`package api`,
		`import "net/http"`,
		`func PatientsIndexList(w http.ResponseWriter, r *http.Request)`,
		`func StatusHealth(w http.ResponseWriter, r *http.Request)`,
		`http.StatusNotImplemented`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected generated source to contain %q:\n%s", want, text)
		}
	}
	if strings.Index(text, "PatientsIndexList") > strings.Index(text, "StatusHealth") {
		t.Fatalf("expected API handlers sorted by generated name:\n%s", text)
	}
}

func TestGenerateAPIPackageUsesCustomPackageName(t *testing.T) {
	source, err := GenerateAPIPackage(manifest.Manifest{}, APIPackageOptions{PackageName: "handlers"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(source)) != "package handlers" {
		t.Fatalf("unexpected empty API package:\n%s", source)
	}
}

func TestGenerateAPIPackageRejectsInvalidPackageName(t *testing.T) {
	_, err := GenerateAPIPackage(manifest.Manifest{}, APIPackageOptions{PackageName: "bad-name"})
	if err == nil {
		t.Fatal("expected invalid package name error")
	}
	if !strings.Contains(err.Error(), `invalid API package name "bad-name"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateAPIPackageRejectsDuplicateGeneratedHandlerNames(t *testing.T) {
	_, err := GenerateAPIPackage(manifest.Manifest{Pages: []manifest.Page{
		{
			ID: "status",
			Blocks: manifest.Blocks{
				APIs: []manifest.API{{Name: "health"}},
			},
		},
		{
			ID: "status-health",
			Blocks: manifest.Blocks{
				APIs: []manifest.API{{}},
			},
		},
	}}, APIPackageOptions{})
	if err == nil {
		t.Fatal("expected duplicate handler error")
	}
	if !strings.Contains(err.Error(), `duplicate generated API handler name "StatusHealth"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

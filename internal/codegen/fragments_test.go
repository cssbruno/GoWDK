package codegen

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func TestGenerateFragmentPackageEmitsRenderFunctions(t *testing.T) {
	source, err := GenerateFragmentPackage(manifest.Manifest{Pages: []manifest.Page{{
		ID: "patients",
		Blocks: manifest.Blocks{
			Actions: []manifest.Action{{
				Name: "refresh",
				Fragments: []manifest.Fragment{{
					Target: "#patients",
					Body:   "<p>Updated</p>",
				}},
			}},
		},
	}}}, FragmentPackageOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "fragments.go", source, parser.AllErrors); err != nil {
		t.Fatalf("generated fragment package is not valid Go: %v\n%s", err, source)
	}

	text := string(source)
	for _, want := range []string{
		`package fragments`,
		`"net/http"`,
		`gowdkresponse "github.com/cssbruno/gowdk/runtime/response"`,
		`func RenderPatientsRefreshPatients() gowdkresponse.Response`,
		`return gowdkresponse.FragmentFor("#patients", "<p>Updated</p>")`,
		`func HandlePatientsRefreshPatients(w http.ResponseWriter, r *http.Request)`,
		`gowdkresponse.WriteHTTP(w, RenderPatientsRefreshPatients())`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("expected generated source to contain %q:\n%s", want, text)
		}
	}
}

func TestGenerateFragmentPackageUsesCustomPackageName(t *testing.T) {
	source, err := GenerateFragmentPackage(manifest.Manifest{}, FragmentPackageOptions{PackageName: "partials"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(source)) != "package partials" {
		t.Fatalf("unexpected empty fragment package:\n%s", source)
	}
}

func TestGenerateFragmentPackageRejectsInvalidPackageName(t *testing.T) {
	_, err := GenerateFragmentPackage(manifest.Manifest{}, FragmentPackageOptions{PackageName: "bad-name"})
	if err == nil {
		t.Fatal("expected invalid package name error")
	}
	if !strings.Contains(err.Error(), `invalid fragment package name "bad-name"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateFragmentPackageRejectsDuplicateGeneratedRendererNames(t *testing.T) {
	_, err := GenerateFragmentPackage(manifest.Manifest{Pages: []manifest.Page{{
		ID: "patients",
		Blocks: manifest.Blocks{
			Actions: []manifest.Action{{
				Name: "refresh",
				Fragments: []manifest.Fragment{
					{Target: "#patients-list"},
					{Target: "#patients_list"},
				},
			}},
		},
	}}}, FragmentPackageOptions{})
	if err == nil {
		t.Fatal("expected duplicate renderer error")
	}
	if !strings.Contains(err.Error(), `duplicate generated fragment renderer name "RenderPatientsRefreshPatientsList"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

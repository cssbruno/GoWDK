package compiler

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// sampleProgram returns an IR program that exercises the converter across pages,
// components, layouts, blocks, endpoints, and bindings.
func sampleProgram() gwdkir.Program {
	return gwdkir.Program{
		Version: gwdkir.Version,
		Pages: []gwdkir.Page{{
			Source:  "pages/home.page.gwdk",
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Render:  gowdk.SPA,
			Layouts: []string{"root"},
			Guards:  []string{"public"},
			Blocks: gwdkir.Blocks{
				Build:     true,
				BuildBody: `=> { title: "Home" }`,
				View:      true,
				ViewBody:  `<main>{title}</main>`,
				Fragments: []gwdkir.FragmentEndpoint{{
					Name:   "List",
					Method: "GET",
					Route:  "/home/list",
					Target: "#list",
					Body:   "<section>List</section>",
				}},
				Spans: gwdkir.BlockSpans{
					Fragments: []source.NamedSpan{{Name: "List"}},
				},
			},
		}},
		Components: []gwdkir.Component{{
			Source:  "components/counter.cmp.gwdk",
			Package: "components",
			Name:    "Counter",
			Props:   []gwdkir.Prop{{Name: "label", Type: "string"}},
		}},
		Layouts: []gwdkir.Layout{{
			Source:  "pages/root.layout.gwdk",
			Package: "pages",
			ID:      "root",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<html><slot /></html>`,
			},
		}},
		Endpoints: []gwdkir.Endpoint{{
			Kind:       gwdkir.EndpointAction,
			PageID:     "home",
			SourceFile: "pages/home.page.gwdk",
			Symbol:     "Subscribe",
			Method:     "POST",
			Path:       "/subscribe",
			Binding: gwdkir.Binding{
				ImportPath:   "example.com/app/handlers",
				PackageName:  "handlers",
				FunctionName: "Subscribe",
				Status:       source.BackendBindingBound,
			},
		}},
	}
}

// TestManifestFromIRReconstructsCoreRecords pins the converter's output shape so
// later validator-migration steps can rely on it as the IR->manifest seam.
func TestManifestFromIRReconstructsCoreRecords(t *testing.T) {
	app := ManifestFromIR(sampleProgram())

	if len(app.Pages) != 1 || app.Pages[0].ID != "home" || app.Pages[0].Route != "/" {
		t.Fatalf("unexpected pages: %#v", app.Pages)
	}
	if len(app.Pages[0].Blocks.Fragments) != 1 || app.Pages[0].Blocks.Fragments[0].Name != "List" {
		t.Fatalf("fragment endpoints not preserved: %#v", app.Pages[0].Blocks.Fragments)
	}
	if len(app.Components) != 1 || app.Components[0].Name != "Counter" {
		t.Fatalf("unexpected components: %#v", app.Components)
	}
	if len(app.Layouts) != 1 || app.Layouts[0].ID != "root" {
		t.Fatalf("unexpected layouts: %#v", app.Layouts)
	}
	if len(app.BackendBindings) != 1 || app.BackendBindings[0].FunctionName != "Subscribe" {
		t.Fatalf("unexpected backend bindings: %#v", app.BackendBindings)
	}
	if app.BackendBindings[0].Kind != "action" || app.BackendBindings[0].Status != source.BackendBindingBound {
		t.Fatalf("unexpected binding kind/status: %#v", app.BackendBindings[0])
	}
}

// TestValidateProgramMatchesManifestValidation is the equivalence guard for the
// IR-first validation path: ValidateProgram(ir) must produce the exact same
// result as running the manifest validator on the reconstructed manifest. While
// individual validators still read manifest, this is a tautology by
// construction; the test exists to catch a future change that makes
// ValidateProgram diverge from "validate the manifest produced from this IR".
func TestValidateProgramMatchesManifestValidation(t *testing.T) {
	config := gowdk.Config{}
	ir := sampleProgram()

	viaProgram := ValidateProgram(config, ir)
	viaManifest := ValidateManifest(config, ManifestFromIR(ir))

	if errString(viaProgram) != errString(viaManifest) {
		t.Fatalf("ValidateProgram diverged from manifest validation:\nprogram: %v\nmanifest: %v", viaProgram, viaManifest)
	}
	if viaProgram != nil {
		t.Fatalf("expected sample program to validate cleanly, got: %v", viaProgram)
	}
}

// TestValidateProgramReportsInvalidRoutes confirms the IR-first path still
// surfaces validation failures (here, a duplicate page identity).
func TestValidateProgramReportsInvalidRoutes(t *testing.T) {
	config := gowdk.Config{}
	ir := sampleProgram()
	ir.Pages = append(ir.Pages, ir.Pages[0]) // duplicate page id "home"

	if err := ValidateProgram(config, ir); err == nil {
		t.Fatal("expected duplicate page identity to fail IR-first validation")
	}
}

// TestValidateBackendBindingPolicyIRMatchesManifest is the equivalence guard for
// the production backend-binding policy on the IR-first path.
func TestValidateBackendBindingPolicyIRMatchesManifest(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{Mode: gowdk.Production}}
	ir := sampleProgram()
	// Force an unbound endpoint so the production policy has something to reject.
	ir.Endpoints[0].Binding.Status = source.BackendBindingMissing

	viaIR := ValidateBackendBindingPolicyIR(config, ir)
	viaManifest := ValidateBackendBindingPolicy(config, ManifestFromIR(ir))

	if errString(viaIR) != errString(viaManifest) {
		t.Fatalf("policy IR path diverged:\nir: %v\nmanifest: %v", viaIR, viaManifest)
	}
	if viaIR == nil {
		t.Fatal("expected production policy to reject a missing backend binding")
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// TestValidateProgramRunsStandaloneEndpointChecks guards the regression that
// motivated issue #145: standalone Go endpoints are lowered into IR losslessly
// via Program.GoEndpoints, and ManifestFromIR reconstructs manifest.Endpoints
// from them, so validateStandaloneEndpoints actually runs on the IR path. Before
// GoEndpoints existed, ManifestFromIR left Endpoints nil and this check was
// silently skipped.
func TestValidateProgramRunsStandaloneEndpointChecks(t *testing.T) {
	ir := gwdkir.Program{
		Version: gwdkir.Version,
		GoEndpoints: []gwdkir.GoEndpoint{{
			Kind:       "action",
			SourceKind: gwdkir.EndpointSourceGo,
			Package:    "handlers",
			Source:     "handlers/subscribe.go",
			Name:       "subscribe", // not an exported Go identifier -> invalid
			Method:     "POST",
			Route:      "/subscribe",
		}},
	}

	err := ValidateProgram(gowdk.Config{}, ir)
	if err == nil {
		t.Fatal("expected non-exported standalone endpoint handler to fail IR validation")
	}
	if !strings.Contains(err.Error(), "must be an exported Go identifier") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}

	// The same IR lowered to a manifest must produce the identical diagnostic —
	// proving ValidateProgram and ValidateManifest agree on endpoint checks.
	if got := errString(ValidateProgram(gowdk.Config{}, ir)); got != errString(ValidateManifest(gowdk.Config{}, ManifestFromIR(ir))) {
		t.Fatalf("IR and manifest endpoint validation diverged: %q", got)
	}
}

// TestGoEndpointsReconstructFaithfully checks that ManifestFromIR rebuilds the
// standalone endpoint declarations field-for-field from the IR mirror.
func TestGoEndpointsReconstructFaithfully(t *testing.T) {
	ir := gwdkir.Program{
		Version: gwdkir.Version,
		GoEndpoints: []gwdkir.GoEndpoint{{
			Kind:          "api",
			SourceKind:    gwdkir.EndpointSourceGo,
			Package:       "handlers",
			Source:        "handlers/list.go",
			Name:          "List",
			Method:        "GET",
			Route:         "/api/list/{id:int}",
			ErrorPage:     "500.html",
			RouteParams:   []source.NamedSpan{{Name: "id"}},
			RouteSpan:     source.SourceSpan{Start: source.SourcePosition{Line: 3, Column: 1}},
			ErrorPageSpan: source.SourceSpan{Start: source.SourcePosition{Line: 4, Column: 1}},
		}},
	}

	app := ManifestFromIR(ir)
	if len(app.Endpoints) != 1 {
		t.Fatalf("expected 1 reconstructed endpoint, got %d", len(app.Endpoints))
	}
	got := app.Endpoints[0]
	if got.Kind != "api" || got.Name != "List" || got.Method != "GET" || got.Route != "/api/list/{id:int}" {
		t.Fatalf("endpoint core fields not preserved: %#v", got)
	}
	if got.ErrorPage != "500.html" || got.SourceKind != "go" || got.Source != "handlers/list.go" {
		t.Fatalf("endpoint metadata not preserved: %#v", got)
	}
	if len(got.RouteParams) != 1 || got.RouteParams[0].Name != "id" {
		t.Fatalf("route params not preserved: %#v", got.RouteParams)
	}
	if got.RouteSpan.Start.Line != 3 || got.ErrorPageSpan.Start.Line != 4 {
		t.Fatalf("spans not preserved: %#v", got)
	}
}

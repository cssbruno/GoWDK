package compiler_test

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// TestValidateProgramScenarios exercises the IR-native validators end to end
// through both public entry points. ValidateManifest is now a thin adapter over
// ValidateProgram(BuildIR(app)), so both must surface the same diagnostic for
// each scenario — including the standalone-endpoint and route-conflict cases
// that motivated #145 (these read manifest.Endpoints, reconstructed from the
// lossless gwdkir.GoEndpoint mirror). Each case asserts the concrete expected
// diagnostic rather than just comparing the two paths, so the test guards real
// behavior and not a tautology.
func TestValidateProgramScenarios(t *testing.T) {
	view := func(body string) manifest.Blocks { return manifest.Blocks{View: true, ViewBody: body} }
	page := func(id, route string) manifest.Page {
		return manifest.Page{ID: id, Route: route, Package: "pages", Source: id + ".page.gwdk", Blocks: view("<main/>")}
	}

	cases := []struct {
		name string
		app  manifest.Manifest
		want string // expected diagnostic substring; "" means must validate cleanly
	}{
		{
			name: "valid program",
			app:  manifest.Manifest{Pages: []manifest.Page{page("home", "/"), page("about", "/about")}},
			want: "",
		},
		{
			name: "duplicate page identity",
			app:  manifest.Manifest{Pages: []manifest.Page{page("home", "/"), page("home", "/other")}},
			want: "home",
		},
		{
			name: "duplicate route pattern",
			app:  manifest.Manifest{Pages: []manifest.Page{page("home", "/dup"), page("other", "/dup")}},
			want: "/dup",
		},
		{
			name: "invalid standalone endpoint handler",
			app: manifest.Manifest{
				Pages: []manifest.Page{page("home", "/")},
				Endpoints: []manifest.EndpointDeclaration{{
					Kind: "action", SourceKind: manifest.EndpointSourceGo, Package: "h",
					Source: "h/x.go", Name: "lowercase", Method: "POST", Route: "/x",
				}},
			},
			want: "must be an exported Go identifier",
		},
		{
			name: "route method conflict between page and standalone endpoint",
			app: manifest.Manifest{
				Pages: []manifest.Page{page("home", "/clash")},
				Endpoints: []manifest.EndpointDeclaration{{
					Kind: "api", SourceKind: manifest.EndpointSourceGo, Package: "h",
					Source: "h/x.go", Name: "Clash", Method: "GET", Route: "/clash",
				}},
			},
			want: "conflicts with",
		},
		{
			name: "standalone action with unsupported method",
			app: manifest.Manifest{
				Endpoints: []manifest.EndpointDeclaration{{
					Kind: "action", SourceKind: manifest.EndpointSourceGo, Package: "h",
					Source: "h/x.go", Name: "Save", Method: "DELETE", Route: "/save",
				}},
			},
			want: "actions currently require POST",
		},
	}

	config := gowdk.Config{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viaManifest := errText(compiler.ValidateManifest(config, tc.app))
			viaProgram := errText(compiler.ValidateProgram(config, gwdkanalysis.BuildIR(config, tc.app)))

			// Both entry points must agree (ValidateManifest adapts to ValidateProgram).
			if viaManifest != viaProgram {
				t.Fatalf("entry points disagree:\nValidateManifest: %q\nValidateProgram:  %q", viaManifest, viaProgram)
			}
			// And the concrete diagnostic must match the scenario's expectation.
			if tc.want == "" {
				if viaProgram != "" {
					t.Fatalf("expected clean validation, got: %q", viaProgram)
				}
				return
			}
			if !strings.Contains(viaProgram, tc.want) {
				t.Fatalf("expected diagnostic containing %q, got: %q", tc.want, viaProgram)
			}
		})
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

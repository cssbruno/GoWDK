package compiler_test

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/manifest"
)

// TestValidateProgramMatchesValidateManifestDifferential is the safety proof for
// flipping the CLI to validate IR instead of the parsed manifest: for a corpus of
// valid and invalid programs, ValidateProgram(BuildIR(app)) must produce the exact
// same diagnostics as ValidateManifest(app). If this ever diverges, the IR round
// trip has lost or reordered something a validator depends on.
func TestValidateProgramMatchesValidateManifestDifferential(t *testing.T) {
	view := func(body string) manifest.Blocks { return manifest.Blocks{View: true, ViewBody: body} }
	page := func(id, route string) manifest.Page {
		return manifest.Page{ID: id, Route: route, Package: "pages", Source: id + ".page.gwdk", Blocks: view("<main/>")}
	}

	cases := []struct {
		name string
		app  manifest.Manifest
	}{
		{
			name: "valid program",
			app:  manifest.Manifest{Pages: []manifest.Page{page("home", "/"), page("about", "/about")}},
		},
		{
			name: "duplicate page identity",
			app:  manifest.Manifest{Pages: []manifest.Page{page("home", "/"), page("home", "/other")}},
		},
		{
			name: "duplicate route pattern",
			app:  manifest.Manifest{Pages: []manifest.Page{page("home", "/dup"), page("other", "/dup")}},
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
		},
		{
			name: "dynamic spa page with empty paths block",
			app: manifest.Manifest{Pages: []manifest.Page{{
				ID: "blog.post", Route: "/blog/{slug}", Package: "pages",
				Source: "blog.post.page.gwdk", Paths: true, Blocks: view("<main/>"),
			}}},
		},
		{
			name: "standalone action with unsupported method",
			app: manifest.Manifest{
				Endpoints: []manifest.EndpointDeclaration{{
					Kind: "action", SourceKind: manifest.EndpointSourceGo, Package: "h",
					Source: "h/x.go", Name: "Save", Method: "DELETE", Route: "/save",
				}},
			},
		},
	}

	config := gowdk.Config{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			viaManifest := errText(compiler.ValidateManifest(config, tc.app))
			ir := gwdkanalysis.BuildIR(config, tc.app)
			viaProgram := errText(compiler.ValidateProgram(config, ir))
			if viaManifest != viaProgram {
				t.Fatalf("diagnostic drift between manifest and IR validation:\nmanifest: %q\nIR:       %q", viaManifest, viaProgram)
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

package buildgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	view "github.com/cssbruno/gowdk/internal/viewrender"
)

// irComponent converts a gwdkir.Component fixture into the IR component the
// migrated render helpers now consume. It routes through the production
// IR builder so the test exercises the same conversion as the real pipeline.
func irComponent(component gwdkir.Component) gwdkir.Component {
	ir := gwdkanalysis.BuildProgram(gowdk.Config{}, gwdkanalysis.Sources{Components: []gwdkir.Component{component}})
	if len(ir.Components) == 0 {
		return gwdkir.Component{}
	}
	return ir.Components[0]
}

func analyzedIRFixture(t *testing.T, program gwdkir.Program) gwdkir.Program {
	t.Helper()
	parseBlocks := func(blocks *gwdkir.Blocks) {
		t.Helper()
		if strings.TrimSpace(blocks.ViewBody) == "" {
			return
		}
		blocks.View = true
		nodes, err := view.Parse(blocks.ViewBody)
		if err != nil {
			t.Fatal(err)
		}
		blocks.ViewNodes = nodes
	}
	for index := range program.Pages {
		parseBlocks(&program.Pages[index].Blocks)
		if program.Pages[index].Blocks.Server && len(program.Pages[index].Blocks.ServerFields) == 0 && strings.TrimSpace(program.Pages[index].Blocks.ServerBody) != "" {
			lowered := gwdkanalysis.BuildProgram(gowdk.Config{}, gwdkanalysis.Sources{Pages: []gwdkir.Page{program.Pages[index]}})
			if len(lowered.Pages) == 1 {
				program.Pages[index].Blocks.ServerFields = lowered.Pages[0].Blocks.ServerFields
			}
		}
	}
	for index := range program.Components {
		parseBlocks(&program.Components[index].Blocks)
	}
	for index := range program.Layouts {
		parseBlocks(&program.Layouts[index].Blocks)
	}
	for index := range program.Templates {
		if strings.TrimSpace(program.Templates[index].Body) == "" {
			continue
		}
		nodes, err := view.Parse(program.Templates[index].Body)
		if err != nil {
			t.Fatal(err)
		}
		program.Templates[index].Nodes = nodes
	}
	return program
}

type testRouteManifest struct {
	Version int `json:"version"`
	Routes  []struct {
		PageID string `json:"page"`
		Route  string `json:"route"`
		Path   string `json:"path"`
		Locale string `json:"locale"`
	} `json:"routes"`
	Endpoints []struct {
		Kind          string   `json:"kind"`
		Directive     string   `json:"directive"`
		Method        string   `json:"method"`
		Route         string   `json:"route"`
		PageID        string   `json:"page"`
		Symbol        string   `json:"symbol"`
		Handler       string   `json:"handler"`
		DynamicParams []string `json:"dynamicParams"`
		RouteParams   []struct {
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"routeParams"`
		Guards []string `json:"guards"`
		CSRF   bool     `json:"csrf"`
	} `json:"endpoints"`
}

func readRouteManifest(t *testing.T, outputDir string) testRouteManifest {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(outputDir, routeManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var routes testRouteManifest
	if err := json.Unmarshal(payload, &routes); err != nil {
		t.Fatal(err)
	}
	return routes
}

func writeFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

func readBytes(t *testing.T, path string) []byte {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func hasAssetArtifact(artifacts []AssetArtifact, path string) bool {
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return true
		}
	}
	return false
}

func cssArtifactByLogicalPath(t *testing.T, artifacts []CSSArtifact, logicalPath string) CSSArtifact {
	t.Helper()
	for _, artifact := range artifacts {
		if artifact.LogicalPath == logicalPath {
			return artifact
		}
	}
	t.Fatalf("expected css artifact with logical path %q, got %#v", logicalPath, artifacts)
	return CSSArtifact{}
}

func assetArtifactByLogicalPath(t *testing.T, artifacts []AssetArtifact, logicalPath string) AssetArtifact {
	t.Helper()
	for _, artifact := range artifacts {
		if artifact.LogicalPath == logicalPath {
			return artifact
		}
	}
	t.Fatalf("expected asset artifact with logical path %q, got %#v", logicalPath, artifacts)
	return AssetArtifact{}
}

func sharedIslandRuntimePath(outputDir string) string {
	return filepath.Join(outputDir, "assets", "gowdk", "islands", "island.js")
}

func readSharedIslandRuntime(t *testing.T, outputDir string) string {
	t.Helper()
	return readFile(t, sharedIslandRuntimePath(outputDir))
}

func mustRelativePath(t *testing.T, base string, path string) string {
	t.Helper()
	rel, err := filepath.Rel(base, path)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func counterComponent() gwdkir.Component {
	return gwdkir.Component{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Count++}>{Count}</button>`,
		},
	}
}

func taggedCounterComponent() gwdkir.Component {
	return gwdkir.Component{
		Name:    "TaggedCounter",
		Source:  "components/tagged-counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TaggedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTaggedState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<span>{Count}</span>`,
		},
	}
}

func textComponent() gwdkir.Component {
	return gwdkir.Component{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<input g:bind:value={Query} />`,
		},
	}
}

func nestedComponent() gwdkir.Component {
	return gwdkir.Component{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<section g:if={User.Open}>{Count}</section>`,
		},
	}
}

func filterComponent() gwdkir.Component {
	return gwdkir.Component{
		Name:    "Filter",
		Source:  "components/filter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "FilterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewFilterState"},
		},
		Blocks: gwdkir.Blocks{
			View: true,
		},
	}
}

func assertOutputMatchesFixture(t *testing.T, outputDir, relativePath string) {
	t.Helper()
	actual, err := os.ReadFile(filepath.Join(outputDir, filepath.FromSlash(relativePath)))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.Join("testdata", "full_fixture", "expected", filepath.FromSlash(relativePath)))
	if err != nil {
		t.Fatal(err)
	}
	if string(actual) != string(expected) {
		t.Fatalf("generated output mismatch for %s\nexpected:\n%s\nactual:\n%s", relativePath, expected, actual)
	}
}

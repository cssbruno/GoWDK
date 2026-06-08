package buildgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cssbruno/gowdk/internal/manifest"
)

type testRouteManifest struct {
	Version int `json:"version"`
	Routes  []struct {
		PageID string `json:"page"`
		Route  string `json:"route"`
		Path   string `json:"path"`
	} `json:"routes"`
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

func mustRelativePath(t *testing.T, base string, path string) string {
	t.Helper()
	rel, err := filepath.Rel(base, path)
	if err != nil {
		t.Fatal(err)
	}
	return rel
}

func counterComponent() manifest.Component {
	return manifest.Component{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "CounterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Count++}>{Count}</button>`,
		},
	}
}

func taggedCounterComponent() manifest.Component {
	return manifest.Component{
		Name:    "TaggedCounter",
		Source:  "components/tagged-counter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TaggedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTaggedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<span>{Count}</span>`,
		},
	}
}

func textComponent() manifest.Component {
	return manifest.Component{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "TextState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<input g:bind:value={Query} />`,
		},
	}
}

func nestedComponent() manifest.Component {
	return manifest.Component{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "NestedState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: manifest.Blocks{
			View:     true,
			ViewBody: `<section g:if={User.Open}>{Count}</section>`,
		},
	}
}

func filterComponent() manifest.Component {
	return manifest.Component{
		Name:    "Filter",
		Source:  "components/filter.cmp.gwdk",
		Imports: []manifest.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: manifest.StateContract{
			Type: manifest.GoTypeRef{Alias: "ui", Name: "FilterState"},
			Init: manifest.GoFuncRef{Alias: "ui", Name: "NewFilterState"},
		},
		Blocks: manifest.Blocks{
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

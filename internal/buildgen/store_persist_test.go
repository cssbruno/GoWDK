package buildgen

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gotypes"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestBuildEmitsPersistConfigOnStoreSeed(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:      "cart",
			Route:   "/cart",
			Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			Stores: []gwdkir.Store{{
				Name:    "cart",
				Type:    gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
				Init:    gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
				Persist: "local",
			}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<main>Cart</main>`},
		}},
	}

	result, err := Build(gowdk.Config{}, app, outputDir)
	if err != nil {
		t.Fatal(err)
	}
	storePath := filepath.Join(outputDir, "assets", "gowdk", "islands", "stores.js")
	if !hasAssetArtifact(result.AssetArtifacts, storePath) {
		t.Fatalf("expected stores.js asset, got %#v", result.AssetArtifacts)
	}

	html := readFile(t, filepath.Join(outputDir, "cart", "index.html"))
	for _, expected := range []string{
		`data-gowdk-store="cart"`,
		`data-gowdk-persist="local"`,
		`data-gowdk-persist-key="gowdk:store:cart"`,
		`data-gowdk-persist-version="`,
	} {
		if !strings.Contains(html, expected) {
			t.Fatalf("expected %q in persisted store page:\n%s", expected, html)
		}
	}
	// The version attribute must carry a non-empty hash.
	if strings.Contains(html, `data-gowdk-persist-version=""`) {
		t.Fatalf("persist version attribute is empty:\n%s", html)
	}

	storeJS := readFile(t, storePath)
	for _, expected := range []string{
		"registry.persist",
		"readPersisted",
		"writePersisted",
		"window.localStorage",
		"window.sessionStorage",
		"data-gowdk-persist",
	} {
		if !strings.Contains(storeJS, expected) {
			t.Fatalf("expected %q in store runtime:\n%s", expected, storeJS)
		}
	}
}

func TestBuildOmitsPersistConfigForUnpersistedStore(t *testing.T) {
	outputDir := t.TempDir()
	app := gwdkanalysis.Sources{
		Pages: []gwdkir.Page{{
			ID:      "cart",
			Route:   "/cart",
			Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			Stores: []gwdkir.Store{{
				Name: "cart",
				Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
				Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
			}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<main>Cart</main>`},
		}},
	}

	if _, err := Build(gowdk.Config{}, app, outputDir); err != nil {
		t.Fatal(err)
	}
	html := readFile(t, filepath.Join(outputDir, "cart", "index.html"))
	if strings.Contains(html, "data-gowdk-persist") {
		t.Fatalf("did not expect persist attributes on an unpersisted store:\n%s", html)
	}
}

func TestStoreSchemaHashIsStableAndShapeSensitive(t *testing.T) {
	base := gotypes.Struct{FieldTypes: map[string]string{"Count": "int", "Open": "bool"}}
	baseSeed := `{"count":0,"open":false}`
	// Same shape, different map iteration order must produce the same hash.
	same := gotypes.Struct{FieldTypes: map[string]string{"Open": "bool", "Count": "int"}}
	if storeSchemaHash(base, baseSeed) != storeSchemaHash(same, baseSeed) {
		t.Fatalf("schema hash should be order-independent: %q vs %q", storeSchemaHash(base, baseSeed), storeSchemaHash(same, baseSeed))
	}
	if storeSchemaHash(base, baseSeed) == "" {
		t.Fatal("schema hash should be non-empty")
	}
	// A retyped field must change the hash (stale storage would otherwise restore a wrong-typed value).
	retyped := gotypes.Struct{FieldTypes: map[string]string{"Count": "string", "Open": "bool"}}
	if storeSchemaHash(base, baseSeed) == storeSchemaHash(retyped, baseSeed) {
		t.Fatal("retyping a field should change the schema hash")
	}
	// A removed field must change the hash.
	removed := gotypes.Struct{FieldTypes: map[string]string{"Count": "int"}}
	if storeSchemaHash(base, `{"count":0}`) == storeSchemaHash(base, baseSeed) {
		t.Fatal("removing a field (fewer on-wire keys) should change the schema hash")
	}
	if storeSchemaHash(base, baseSeed) == storeSchemaHash(removed, `{"count":0}`) {
		t.Fatal("removing a field should change the schema hash")
	}
	// A json-tag-only rename (same Go fields, different on-wire key) must change the hash.
	renamedSeed := `{"qty":0,"open":false}`
	if storeSchemaHash(base, baseSeed) == storeSchemaHash(base, renamedSeed) {
		t.Fatal("a json-tag-only rename should change the schema hash")
	}
}

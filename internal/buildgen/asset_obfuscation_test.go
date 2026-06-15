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
	runtimeasset "github.com/cssbruno/gowdk/runtime/asset"
)

func TestBuildObfuscatesGeneratedRuntimeAssetsInProduction(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{
		Mode:            gowdk.Production,
		ObfuscateAssets: true,
	}}
	app := spaNavigationRuntimeFixture()

	firstDir := t.TempDir()
	first, err := Build(config, app, firstDir)
	if err != nil {
		t.Fatal(err)
	}
	secondDir := t.TempDir()
	second, err := Build(config, app, secondDir)
	if err != nil {
		t.Fatal(err)
	}

	firstAsset := assetArtifactByPath(t, first.AssetArtifacts, filepath.Join(firstDir, filepath.FromSlash(clientRuntimeAssetPath)))
	secondAsset := assetArtifactByPath(t, second.AssetArtifacts, filepath.Join(secondDir, filepath.FromSlash(clientRuntimeAssetPath)))
	if !firstAsset.Obfuscated {
		t.Fatalf("expected runtime asset to be marked obfuscated: %#v", firstAsset)
	}
	if firstAsset.Hash == "" || firstAsset.Hash != secondAsset.Hash {
		t.Fatalf("expected deterministic obfuscated hash, got first=%q second=%q", firstAsset.Hash, secondAsset.Hash)
	}

	runtimePath := filepath.Join(firstDir, filepath.FromSlash(clientRuntimeAssetPath))
	runtime := readFile(t, runtimePath)
	if strings.Contains(runtime, "\n  async function submitPartial") {
		t.Fatalf("expected runtime asset to be minified, got:\n%s", runtime)
	}

	manifest := readAssetManifest(t, firstDir)
	if manifest.Version != runtimeasset.ManifestVersion {
		t.Fatalf("expected asset manifest version %d, got %d", runtimeasset.ManifestVersion, manifest.Version)
	}
	if !manifest.IsObfuscated(clientRuntimeAssetPath) {
		t.Fatalf("expected obfuscation marker in asset manifest: %#v", manifest.Obfuscated)
	}

	summary := findBuildReportEvent(first.Report, "plan", "asset_obfuscation")
	if summary == nil || summary.Data["enabled"] != "true" || summary.Data["assets"] != "1" {
		t.Fatalf("unexpected asset obfuscation summary: %#v", summary)
	}
	event := findBuildReportEvent(first.Report, "plan", "asset_obfuscated")
	if event == nil || event.Path != clientRuntimeAssetPath || event.Data["changed"] != "true" || event.Data["beforeHash"] == event.Data["afterHash"] {
		t.Fatalf("unexpected asset obfuscation event: %#v", event)
	}
}

func TestBuildDoesNotObfuscateGeneratedRuntimeAssetsByDefault(t *testing.T) {
	outputDir := t.TempDir()
	result, err := Build(gowdk.Config{}, spaNavigationRuntimeFixture(), outputDir)
	if err != nil {
		t.Fatal(err)
	}
	asset := assetArtifactByPath(t, result.AssetArtifacts, filepath.Join(outputDir, filepath.FromSlash(clientRuntimeAssetPath)))
	if asset.Obfuscated {
		t.Fatalf("did not expect default runtime asset to be obfuscated: %#v", asset)
	}
	runtime := readFile(t, filepath.Join(outputDir, filepath.FromSlash(clientRuntimeAssetPath)))
	if !strings.Contains(runtime, "\n  async function submitPartial") {
		t.Fatalf("expected readable default runtime asset, got:\n%s", runtime)
	}
	manifest := readAssetManifest(t, outputDir)
	if manifest.IsObfuscated(clientRuntimeAssetPath) || len(manifest.Obfuscated) > 0 {
		t.Fatalf("did not expect obfuscation metadata by default: %#v", manifest.Obfuscated)
	}
	summary := findBuildReportEvent(result.Report, "plan", "asset_obfuscation")
	if summary == nil || summary.Data["enabled"] != "false" || summary.Data["assets"] != "0" {
		t.Fatalf("unexpected default obfuscation summary: %#v", summary)
	}
}

func TestBuildRejectsAssetObfuscationOutsideProductionMode(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{ObfuscateAssets: true}}
	_, err := Build(config, spaNavigationRuntimeFixture(), t.TempDir())
	if err == nil {
		t.Fatal("expected production-only obfuscation error")
	}
	if !strings.Contains(err.Error(), "Build.ObfuscateAssets requires Build.Mode: gowdk.Production") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func readAssetManifest(t *testing.T, outputDir string) runtimeasset.Manifest {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(outputDir, assetManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var manifest runtimeasset.Manifest
	if err := json.Unmarshal(payload, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}

func assetArtifactByPath(t *testing.T, artifacts []AssetArtifact, path string) AssetArtifact {
	t.Helper()
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return artifact
		}
	}
	t.Fatalf("expected asset artifact path %q, got %#v", path, artifacts)
	return AssetArtifact{}
}

func spaNavigationRuntimeFixture() gwdkanalysis.Sources {
	return gwdkanalysis.Sources{
		Pages: []gwdkir.Page{
			{
				ID:    "home",
				Route: "/",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<main><Nav /></main>`,
				},
			},
			{
				ID:    "docs",
				Route: "/docs",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<main><h1>Docs</h1></main>`,
				},
			},
		},
		Components: []gwdkir.Component{{
			Name: "Nav",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<nav><a href="/docs">Docs</a></nav>`,
			},
		}},
	}
}

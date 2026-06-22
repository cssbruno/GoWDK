package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestCleanTargetsCollectsConfiguredOutputs(t *testing.T) {
	config := gowdk.Config{
		Build: gowdk.BuildConfig{
			Output: "gowdk_cache",
			Targets: []gowdk.BuildTargetConfig{
				{Name: "site", Output: ".gowdk/output/site", App: ".gowdk/app/site", Binary: "bin/site", WASM: "bin/site.wasm", BackendApp: ".gowdk/backend/site", BackendBinary: "bin/site-backend"},
			},
		},
	}
	targets, err := cleanTargets(config, nil, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"gowdk_cache", ".gowdk/output/site", ".gowdk/app/site", "bin/site", "bin/site.wasm", ".gowdk/backend/site", "bin/site-backend"} {
		if !contains(targets, want) {
			t.Fatalf("expected %q in targets %v", want, targets)
		}
	}
}

func TestCleanTargetsFiltersByTargetName(t *testing.T) {
	config := gowdk.Config{
		Build: gowdk.BuildConfig{
			Output: "gowdk_cache",
			Targets: []gowdk.BuildTargetConfig{
				{Name: "site", Output: "out/site"},
				{Name: "admin", Output: "out/admin"},
			},
		},
	}
	targets, err := cleanTargets(config, []string{"admin"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if contains(targets, "out/site") || contains(targets, "gowdk_cache") {
		t.Fatalf("expected only the admin target, got %v", targets)
	}
	if !contains(targets, "out/admin") {
		t.Fatalf("expected the admin output, got %v", targets)
	}
}

func TestCleanTargetsUsesNormalizedTargetNames(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{Targets: []gowdk.BuildTargetConfig{{Name: " admin ", Output: "out/admin"}}}}
	targets, err := cleanTargets(config, []string{"admin"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 || targets[0] != "out/admin" {
		t.Fatalf("targets = %#v, want admin output", targets)
	}
}

func TestCleanTargetsRejectsDuplicateTargetNames(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{Targets: []gowdk.BuildTargetConfig{{Name: "site"}, {Name: " site "}}}}
	if _, err := cleanTargets(config, nil, ""); err == nil {
		t.Fatal("expected an error for duplicate target names")
	}
}

func TestCleanTargetsRejectsUnknownTarget(t *testing.T) {
	config := gowdk.Config{Build: gowdk.BuildConfig{Targets: []gowdk.BuildTargetConfig{{Name: "site"}}}}
	if _, err := cleanTargets(config, []string{"missing"}, ""); err == nil {
		t.Fatal("expected an error for an unknown target")
	}
}

func TestCleanTargetsAddsOutOverride(t *testing.T) {
	targets, err := cleanTargets(gowdk.Config{}, nil, "custom-out")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(targets, "custom-out") {
		t.Fatalf("expected the --out override, got %v", targets)
	}
}

func TestSafeRelativeTargetsRejectsRootAndEscapes(t *testing.T) {
	root := t.TempDir()
	safe := safeRelativeTargets(root, []string{".", "..", "../escape", root, "gowdk_cache", "gowdk_cache"})
	if len(safe) != 1 || safe[0] != "gowdk_cache" {
		t.Fatalf("expected only the in-project path, got %v", safe)
	}
}

func TestRunCleanRemovesExistingAndReportsAbsent(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "gowdk_cache")
	if err := os.MkdirAll(filepath.Join(outputDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := runClean(root, []string{"gowdk_cache", "bin/site"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(result.Removed, "gowdk_cache") {
		t.Fatalf("expected gowdk_cache removed, got %+v", result)
	}
	if !contains(result.Absent, "bin/site") {
		t.Fatalf("expected bin/site reported absent, got %+v", result)
	}
	if _, statErr := os.Stat(outputDir); !os.IsNotExist(statErr) {
		t.Fatalf("expected %s to be removed", outputDir)
	}
}

func TestRunCleanDryRunKeepsFiles(t *testing.T) {
	root := t.TempDir()
	outputDir := filepath.Join(root, "gowdk_cache")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatal(err)
	}

	result, err := runClean(root, []string{"gowdk_cache"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if !result.DryRun || !contains(result.Removed, "gowdk_cache") {
		t.Fatalf("expected dry-run to list gowdk_cache, got %+v", result)
	}
	if _, statErr := os.Stat(outputDir); statErr != nil {
		t.Fatalf("dry-run must not remove %s: %v", outputDir, statErr)
	}
}

func TestRunCleanDoesNotFollowSymlinkOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	external := filepath.Join(outside, "keep.txt")
	if err := os.WriteFile(external, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	// "link" inside the project points at the external directory; the
	// configured clean target reaches a file through that intermediate symlink.
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	result, err := runClean(root, []string{"link/keep.txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if contains(result.Removed, "link/keep.txt") {
		t.Fatalf("expected symlinked external path to be refused, got %+v", result)
	}
	if _, statErr := os.Stat(external); statErr != nil {
		t.Fatalf("external file must survive clean: %v", statErr)
	}
}

func TestCleanCommandRejectsUnknownFlag(t *testing.T) {
	if err := clean([]string{"--nope"}); err == nil {
		t.Fatal("expected an error for an unknown flag")
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

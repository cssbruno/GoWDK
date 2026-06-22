package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestConfiguredDiscoveryInputsResolveEmptyRoot(t *testing.T) {
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatal(err)
		}
	})

	inputs, err := configuredDiscoveryInputs(gowdk.Config{}, "", nil, "")
	if err != nil {
		t.Fatal(err)
	}
	if inputs.root != cwd {
		t.Fatalf("root = %q, want cwd %q", inputs.root, cwd)
	}
	if !reflect.DeepEqual(inputs.includes, defaultSourceIncludes) {
		t.Fatalf("includes = %#v, want %#v", inputs.includes, defaultSourceIncludes)
	}
}

func TestConfiguredDiscoveryInputsSelectedModulesAndExcludes(t *testing.T) {
	root := t.TempDir()
	config := gowdk.Config{
		Source: gowdk.SourceConfig{
			Include: []string{"pages/**/*.gwdk"},
			Exclude: []string{"pages/draft/**"},
		},
		Modules: []gowdk.ModuleConfig{
			{
				Name: "admin",
				Source: gowdk.SourceConfig{
					Include: []string{"admin/**/*.gwdk"},
					Exclude: []string{"admin/tmp/**"},
				},
			},
			{Name: "public"},
		},
	}

	inputs, err := configuredDiscoveryInputs(config, "dist", []string{"admin"}, root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(inputs.includes, []string{"admin/**/*.gwdk"}) {
		t.Fatalf("includes = %#v, want selected module include only", inputs.includes)
	}
	for _, want := range append(defaultSourceExcludes, "pages/draft/**", "admin/tmp/**", "dist/**") {
		if !hasString(inputs.excludes, want) {
			t.Fatalf("excludes = %#v, missing %q", inputs.excludes, want)
		}
	}
}

func TestDiscoverConfiguredFilesAndDirsShareInputs(t *testing.T) {
	root := t.TempDir()
	writeProjectInputFile(t, root, "pages/home.page.gwdk")
	writeProjectInputFile(t, root, "pages/draft/ignored.page.gwdk")
	writeProjectInputFile(t, root, "admin/dashboard.page.gwdk")
	writeProjectInputFile(t, root, "admin/tmp/ignored.page.gwdk")
	writeProjectInputFile(t, root, "dist/generated.page.gwdk")
	config := gowdk.Config{
		Source: gowdk.SourceConfig{
			Include: []string{"pages/**/*.gwdk"},
			Exclude: []string{"pages/draft/**"},
		},
		Modules: []gowdk.ModuleConfig{
			{
				Name: "admin",
				Source: gowdk.SourceConfig{
					Include: []string{"admin/**/*.gwdk"},
					Exclude: []string{"admin/tmp/**"},
				},
			},
		},
	}

	files, err := discoverConfiguredFiles(config, "dist", nil, root)
	if err != nil {
		t.Fatal(err)
	}
	filesWithDirs, dirs, err := discoverConfiguredFilesAndDirs(config, "dist", nil, root)
	if err != nil {
		t.Fatal(err)
	}
	wantFiles := []string{"admin/dashboard.page.gwdk", "pages/home.page.gwdk"}
	if got := projectInputRelPaths(t, root, files); !reflect.DeepEqual(got, wantFiles) {
		t.Fatalf("files = %#v, want %#v", got, wantFiles)
	}
	if got := projectInputRelPaths(t, root, filesWithDirs); !reflect.DeepEqual(got, wantFiles) {
		t.Fatalf("files from FilesAndDirs = %#v, want %#v", got, wantFiles)
	}
	for _, excluded := range []string{"pages/draft", "admin/tmp", "dist"} {
		if hasString(projectInputRelPaths(t, root, dirs), excluded) {
			t.Fatalf("dirs should exclude %q, got %#v", excluded, projectInputRelPaths(t, root, dirs))
		}
	}
}

func writeProjectInputFile(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("package pages\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func projectInputRelPaths(t *testing.T, root string, paths []string) []string {
	t.Helper()
	out := make([]string, 0, len(paths))
	for _, item := range paths {
		rel, err := filepath.Rel(root, item)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == want {
			return true
		}
	}
	return false
}

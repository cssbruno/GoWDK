package discover

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestConfiguredSelectionUsesSelectedModuleRules(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pages/home.page.gwdk")
	writeFile(t, root, "pages/draft/ignored.page.gwdk")
	writeFile(t, root, "admin/dashboard.page.gwdk")
	writeFile(t, root, "admin/tmp/ignored.page.gwdk")
	writeFile(t, root, "public/home.page.gwdk")
	writeFile(t, root, "dist/generated.page.gwdk")

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
			{
				Name: "public",
				Source: gowdk.SourceConfig{
					Include: []string{"public/**/*.gwdk"},
				},
			},
		},
	}

	selection, err := ConfiguredSelection(config, "dist", []string{"admin"}, root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selection.Includes, []string{"admin/**/*.gwdk"}) {
		t.Fatalf("includes = %#v, want selected module include only", selection.Includes)
	}
	for _, want := range append(DefaultSourceExcludes(), "pages/draft/**", "admin/tmp/**", "dist/**") {
		if !containsString(selection.Excludes, want) {
			t.Fatalf("excludes = %#v, missing %q", selection.Excludes, want)
		}
	}

	files, dirs, err := selection.FilesAndDirs()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := relFiles(t, root, files), []string{"admin/dashboard.page.gwdk"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %#v, want %#v", got, want)
	}
	for _, excluded := range []string{"admin/tmp", "dist"} {
		if containsString(relFiles(t, root, dirs), excluded) {
			t.Fatalf("dirs should exclude %q, got %#v", excluded, relFiles(t, root, dirs))
		}
	}

	if !selection.Matches(filepath.Join(root, "admin", "new.cmp.gwdk")) {
		t.Fatal("expected unsaved selected-module component path to match")
	}
	for _, path := range []string{
		filepath.Join(root, "admin", "tmp", "hidden.cmp.gwdk"),
		filepath.Join(root, "public", "hidden.cmp.gwdk"),
		filepath.Join(root, "dist", "generated.cmp.gwdk"),
		filepath.Join(filepath.Dir(root), "outside.cmp.gwdk"),
	} {
		if selection.Matches(path) {
			t.Fatalf("expected %q not to match", path)
		}
	}
}

func TestConfiguredSelectionIncludesProjectAndAllModulesByDefault(t *testing.T) {
	root := t.TempDir()
	config := gowdk.Config{
		Source: gowdk.SourceConfig{Include: []string{"pages/**/*.gwdk"}},
		Modules: []gowdk.ModuleConfig{
			{Name: "admin"},
			{Name: "public", Source: gowdk.SourceConfig{Include: []string{"site/**/*.gwdk"}}},
		},
	}

	selection, err := ConfiguredSelection(config, "", nil, root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"pages/**/*.gwdk", "admin/**/*.gwdk", "site/**/*.gwdk"}
	if !reflect.DeepEqual(selection.Includes, want) {
		t.Fatalf("includes = %#v, want %#v", selection.Includes, want)
	}
}

func TestConfiguredSelectionPrunesGeneratedAndTargetOutputs(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"pages/home.page.gwdk",
		".gowdk/copied.page.gwdk",
		".gowdk/output/admin/copied.page.gwdk",
		"bin/copied.page.gwdk",
		"dist/copied.page.gwdk",
		"gowdk_cache/copied.page.gwdk",
		"generated/admin/copied.page.gwdk",
		"generated/backend/copied.page.gwdk",
		"custom-output/copied.page.gwdk",
	} {
		writeFile(t, root, path)
	}
	config := gowdk.Config{
		Build: gowdk.BuildConfig{
			Targets: []gowdk.BuildTargetConfig{
				{
					Name:       "admin",
					App:        "generated/admin",
					BackendApp: "generated/backend",
				},
				{
					Name:   "custom",
					Output: "custom-output",
				},
			},
		},
	}

	selection, err := ConfiguredSelection(config, "", nil, root)
	if err != nil {
		t.Fatal(err)
	}
	files, dirs, err := selection.FilesAndDirs()
	if err != nil {
		t.Fatal(err)
	}
	if got, want := relFiles(t, root, files), []string{"pages/home.page.gwdk"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("files = %#v, want %#v", got, want)
	}
	for _, excluded := range []string{
		".gowdk",
		"bin",
		"dist",
		"gowdk_cache",
		"generated/admin",
		"generated/backend",
		"custom-output",
	} {
		if containsString(relFiles(t, root, dirs), excluded) {
			t.Fatalf("generated directory %q was traversed: %#v", excluded, relFiles(t, root, dirs))
		}
	}
}

func TestConfiguredSelectionRejectsUnknownModule(t *testing.T) {
	_, err := ConfiguredSelection(gowdk.Config{}, "", []string{"missing"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), `module "missing" is not configured`) {
		t.Fatalf("expected unknown module error, got %v", err)
	}
}

func TestSelectionFingerprintIsDeterministic(t *testing.T) {
	root := t.TempDir()
	first := Selection{
		Root:     root,
		Includes: []string{"pages/**/*.gwdk", "components/**/*.gwdk"},
		Excludes: []string{"dist/**", "vendor/**"},
	}
	second := Selection{
		Root:     root,
		Includes: []string{"components/**/*.gwdk", "pages/**/*.gwdk"},
		Excludes: []string{"vendor/**", "dist/**"},
	}
	if first.Fingerprint() != second.Fingerprint() {
		t.Fatalf("equivalent selections have different fingerprints: %q != %q", first.Fingerprint(), second.Fingerprint())
	}
	second.Excludes = append(second.Excludes, "draft/**")
	if first.Fingerprint() == second.Fingerprint() {
		t.Fatal("different selections have the same fingerprint")
	}
}

func TestDefaultSourcePatternsReturnCopies(t *testing.T) {
	includes := DefaultSourceIncludes()
	excludes := DefaultSourceExcludes()
	includes[0] = "changed"
	excludes[0] = "changed"
	if got := DefaultSourceIncludes()[0]; got != "**/*.gwdk" {
		t.Fatalf("default include mutated to %q", got)
	}
	if got := DefaultSourceExcludes()[0]; got != ".git/**" {
		t.Fatalf("default exclude mutated to %q", got)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

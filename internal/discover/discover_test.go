package discover

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFilesMatchesRecursiveGWDKIncludes(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/home.page.gwdk")
	writeFile(t, root, "src/nested/card.cmp.gwdk")
	writeFile(t, root, "modules/blog/post.page.gwdk")
	writeFile(t, root, "tmp/ignored.page.gwdk")
	writeFile(t, root, "src/readme.md")

	files, err := Files(root, []string{"src/**/*.gwdk", "modules/**/*.gwdk"}, []string{"src/**/card.cmp.gwdk"})
	if err != nil {
		t.Fatal(err)
	}

	got := relFiles(t, root, files)
	want := []string{"modules/blog/post.page.gwdk", "src/home.page.gwdk"}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected %v, got %v", want, got)
		}
	}
}

func TestFilesPrunesExcludedDirectories(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "src/home.page.gwdk")
	writeFile(t, root, "node_modules/pkg/dep.page.gwdk")
	writeFile(t, root, "vendor/lib/thing.page.gwdk")

	files, err := Files(root, []string{"**/*.gwdk"}, []string{"node_modules/**", "vendor/**"})
	if err != nil {
		t.Fatal(err)
	}

	got := relFiles(t, root, files)
	want := []string{"src/home.page.gwdk"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestMatchesExcludedDir(t *testing.T) {
	matchers := compileGlobs([]string{"node_modules/**", "vendor/**", "**/testdata/**", "src/**/card.cmp.gwdk"})
	tests := []struct {
		dir  string
		want bool
	}{
		{"node_modules", true},
		{"vendor", true},
		{"pkg/testdata", true},
		{"src", false},        // file-glob exclude must not prune the whole src tree
		{"src/nested", false}, // intermediate dir of a file exclude stays
	}
	for _, test := range tests {
		if got := matchesExcludedDir(matchers, test.dir); got != test.want {
			t.Fatalf("matchesExcludedDir(%q) = %v, want %v", test.dir, got, test.want)
		}
	}
}

func TestFilesToleratesUnreadableDirectory(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := t.TempDir()
	writeFile(t, root, "src/home.page.gwdk")
	blocked := filepath.Join(root, "blocked")
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "blocked/secret.page.gwdk")
	if err := os.Chmod(blocked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(blocked, 0o755) })

	files, err := Files(root, []string{"**/*.gwdk"}, nil)
	if err != nil {
		t.Fatalf("expected unreadable directory to be skipped, got error: %v", err)
	}
	got := relFiles(t, root, files)
	if len(got) != 1 || got[0] != "src/home.page.gwdk" {
		t.Fatalf("expected only src/home.page.gwdk, got %v", got)
	}
}

func TestGlobPatternMatch(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"src/**/*.gwdk", "src/home.page.gwdk", true},
		{"src/**/*.gwdk", "src/nested/card.cmp.gwdk", true},
		{"src/*.gwdk", "src/nested/card.cmp.gwdk", false},
		{"**/*.css", "theme.css", true},
		{"**/*.css", "assets/theme.css", true},
		{"src/file?.gwdk", "src/file1.gwdk", true},
		{"src/file?.gwdk", "src/file10.gwdk", false},
		{"dist/**", "dist/assets/app.js", true},
	}

	for _, test := range tests {
		if got := globPattern(test.pattern).match(test.value); got != test.want {
			t.Fatalf("globPattern(%q).match(%q) = %v, want %v", test.pattern, test.value, got, test.want)
		}
	}
}

func writeFile(t *testing.T, root, name string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func relFiles(t *testing.T, root string, files []string) []string {
	t.Helper()
	var rels []string
	for _, file := range files {
		rel, err := filepath.Rel(root, file)
		if err != nil {
			t.Fatal(err)
		}
		rels = append(rels, filepath.ToSlash(rel))
	}
	return rels
}

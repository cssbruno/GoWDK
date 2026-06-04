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

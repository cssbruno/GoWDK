package lang

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatNormalizesTopLevelBlocks(t *testing.T) {
	source := []byte("@page home\n\n@route \"/\"\n\n\nview {\n<h1>GOWDK</h1>\n}\n")
	got := string(Format(source))
	want := "@page home\n@route \"/\"\n\nview {\n  <h1>GOWDK</h1>\n}\n"
	if got != want {
		t.Fatalf("unexpected format:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestFormatGoldenPreservesCommentsAndNestedMarkup(t *testing.T) {
	source, err := os.ReadFile(filepath.FromSlash("testdata/format_golden/input.gwdk"))
	if err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.FromSlash("testdata/format_golden/expected.gwdk"))
	if err != nil {
		t.Fatal(err)
	}

	if got := string(Format(source)); got != string(expected) {
		t.Fatalf("format golden mismatch\nexpected:\n%s\nactual:\n%s", expected, got)
	}
}

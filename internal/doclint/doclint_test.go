package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestCheckFlagsBrokenLocalLink(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "guide.md", "See the [missing page](./does-not-exist.md).\n")

	problems, err := Check(Config{Root: dir, ExcludedDirs: defaultExclusions})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 {
		t.Fatalf("expected 1 problem, got %d: %v", len(problems), problems)
	}
	if problems[0].Reason != "missing local file" || problems[0].Target != "./does-not-exist.md" {
		t.Fatalf("unexpected problem: %+v", problems[0])
	}
}

func TestCheckAllowsValidLinksAndAnchors(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "Read [setup](sub/setup.md#install-steps) and [top](#overview).\n\n# Overview\n")
	writeFile(t, dir, "sub/setup.md", "# Setup\n\n## Install steps\n\nDone.\n")

	problems, err := Check(Config{Root: dir, ExcludedDirs: defaultExclusions})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("expected no problems, got %v", problems)
	}
}

func TestCheckFlagsMissingAnchor(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "a.md", "Jump to [other](b.md#no-such-heading).\n")
	writeFile(t, dir, "b.md", "# Real Heading\n")

	problems, err := Check(Config{Root: dir, ExcludedDirs: defaultExclusions})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || problems[0].Reason != "missing heading anchor" {
		t.Fatalf("expected one missing-anchor problem, got %v", problems)
	}
}

func TestCheckSkipsExcludedDirs(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "node_modules/dep/readme.md", "[broken](./nope.md)\n")
	writeFile(t, dir, ".gowdk/output/page.md", "[broken](./nope.md)\n")

	problems, err := Check(Config{Root: dir, ExcludedDirs: defaultExclusions})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("expected excluded dirs to be skipped, got %v", problems)
	}
}

func TestCheckSkipsExternalLinks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "ext.md", "[site](https://example.com/missing) and [mail](mailto:a@b.com)\n")

	problems, err := Check(Config{Root: dir, ExcludedDirs: defaultExclusions})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("expected external links to be skipped, got %v", problems)
	}
}

func TestCheckIgnoresLinksInCode(t *testing.T) {
	dir := t.TempDir()
	body := "Inline `[x](./missing.md)` is an example.\n\n" +
		"```\n[y](./also-missing.md)\n```\n"
	writeFile(t, dir, "code.md", body)

	problems, err := Check(Config{Root: dir, ExcludedDirs: defaultExclusions})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("expected code-span and fenced links to be ignored, got %v", problems)
	}
}

func TestCheckResolvesDirectoryLinks(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.md", "Browse the [utils folder](src/utils/).\n")
	writeFile(t, dir, "src/utils/keep.md", "# keep\n")

	problems, err := Check(Config{Root: dir, ExcludedDirs: defaultExclusions})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Fatalf("expected directory link to resolve, got %v", problems)
	}
}

func TestCheckReferenceDefinitions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "ref.md", "See [the spec][spec].\n\n[spec]: ./spec.md\n")

	problems, err := Check(Config{Root: dir, ExcludedDirs: defaultExclusions})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || problems[0].Reason != "missing local file" {
		t.Fatalf("expected the reference definition to be checked, got %v", problems)
	}
}

func TestSlugifyMatchesGitHubStyle(t *testing.T) {
	cases := map[string]string{
		"Install Steps":          "install-steps",
		"persist \"local\"":      "persist-local",
		"Go / WASM Interop":      "go--wasm-interop",
		"Section 1.2: Overview!": "section-12-overview",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHeadingAnchorsDeduplicate(t *testing.T) {
	anchors := headingAnchors("# Notes\n\n## Notes\n\n## Notes\n")
	for _, want := range []string{"notes", "notes-1", "notes-2"} {
		if !anchors[want] {
			t.Errorf("expected anchor %q in %v", want, anchors)
		}
	}
}

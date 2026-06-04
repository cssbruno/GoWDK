package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/addons/ssr"
)

func TestCheckFilesValidatesRenderRules(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "dashboard.page.gwdk")
	writeGWDK(t, path, `@page dashboard
@route "/dashboard"
@render ssr

load {
}

view {
}
`)

	_, diagnostics := CheckFiles(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected missing SSR addon diagnostic")
	}

	_, diagnostics = CheckFiles(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, []string{path})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
}

func TestCheckSourceValidatesUnsavedBuffer(t *testing.T) {
	_, diagnostics := CheckSource(gowdk.Config{}, "untitled.gwdk", []byte(`@page post
@route "/blog/{slug}"

view {
}
`))
	if !diagnostics.HasErrors() {
		t.Fatal("expected dynamic route diagnostic")
	}
	if diagnostics[0].File != "untitled.gwdk" {
		t.Fatalf("expected source path on diagnostic, got %#v", diagnostics[0])
	}
	if !strings.Contains(diagnostics[0].Message, "dynamic route params") {
		t.Fatalf("unexpected diagnostic: %#v", diagnostics[0])
	}
}

func TestCompletionsIncludeCoreLanguageKeywords(t *testing.T) {
	completions := Completions()
	if len(completions) == 0 {
		t.Fatal("expected completions")
	}
	var foundPage bool
	var foundPost bool
	for _, item := range completions {
		if item.Label == "@page" {
			foundPage = true
		}
		if item.Label == "g:post" {
			foundPost = true
		}
	}
	if !foundPage || !foundPost {
		t.Fatalf("missing expected completions: %#v", completions)
	}
}

func TestManifestJSONEmitsParsedPage(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "home.page.gwdk")
	writeGWDK(t, path, `@page home
@route "/"
@layout root

view {
}
`)

	payload, diagnostics := ManifestJSON(gowdk.Config{}, []string{path})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	if !strings.Contains(string(payload), `"home"`) || !strings.Contains(string(payload), `"render": "static"`) {
		t.Fatalf("unexpected manifest JSON: %s", payload)
	}
}

func TestCheckJSONReportsCompilerDiagnosticsWithFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "dashboard.page.gwdk")
	writeGWDK(t, path, `@page dashboard
@route "/dashboard"
@render ssr

load {
}

view {
}
`)

	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected diagnostics")
	}
	output := string(payload)
	if !strings.Contains(output, `"file": "`+path+`"`) {
		t.Fatalf("expected diagnostic file in JSON: %s", output)
	}
	if !strings.Contains(output, "SSR addon is not enabled") {
		t.Fatalf("expected SSR diagnostic in JSON: %s", output)
	}
}

func TestParseFileReportsParserDiagnosticLine(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.page.gwdk")
	writeGWDK(t, path, `@page bad
@route "/bad"
@render nope
`)

	_, diagnostics := ParseFile(path)
	if !diagnostics.HasErrors() {
		t.Fatal("expected parser diagnostic")
	}
	if diagnostics[0].Pos.Line != 3 || diagnostics[0].Pos.Column != 1 {
		t.Fatalf("expected line 3 diagnostic, got %#v", diagnostics[0].Pos)
	}
}

func writeGWDK(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

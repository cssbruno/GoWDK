package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
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

func TestManifestJSONGoldenFixture(t *testing.T) {
	paths := []string{
		filepath.FromSlash("testdata/manifest_golden/home.page.gwdk"),
		filepath.FromSlash("testdata/manifest_golden/hero.cmp.gwdk"),
	}

	payload, diagnostics := ManifestJSON(gowdk.Config{}, paths)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	expected, err := os.ReadFile(filepath.FromSlash("testdata/manifest_golden/manifest.golden.json"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(string(payload)) != strings.TrimSpace(string(expected)) {
		t.Fatalf("manifest golden mismatch\nexpected:\n%s\nactual:\n%s", expected, payload)
	}
}

func TestCheckJSONGoldenDiagnosticsFixture(t *testing.T) {
	path := filepath.FromSlash("testdata/diagnostics_golden/invalid.page.gwdk")
	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected diagnostics")
	}
	expected, err := os.ReadFile(filepath.FromSlash("testdata/diagnostics_golden/diagnostics.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(payload)) != strings.TrimSpace(string(expected)) {
		t.Fatalf("diagnostics golden mismatch\nexpected:\n%s\nactual:\n%s", expected, payload)
	}
}

func TestClassifySourceUsesCurrentFileKindRules(t *testing.T) {
	cases := []struct {
		path   string
		source string
		kind   FileKind
	}{
		{"home.page.gwdk", "@page home", FileKindPage},
		{"hero.cmp.gwdk", "@component Hero", FileKindComponent},
		{"hero.gwdk", "@component Hero", FileKindComponent},
		{"root.layout.gwdk", "@layout root", FileKindLayout},
		{"images.asset.gwdk", "@asset images", FileKindAsset},
		{"tailwind.plugin.gwdk", "@plugin tailwind", FileKindPlugin},
	}

	for _, tc := range cases {
		if got := ClassifySource(tc.path, []byte(tc.source)); got != tc.kind {
			t.Fatalf("ClassifySource(%q) = %q, want %q", tc.path, got, tc.kind)
		}
	}
}

func TestParseBuildFilesParsesLayoutFilesAndSkipsNonGWDKInputs(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "home.page.gwdk")
	layout := filepath.Join(root, "root.layout.gwdk")
	asset := filepath.Join(root, "images.asset.gwdk")
	plugin := filepath.Join(root, "tailwind.plugin.gwdk")
	writeGWDK(t, page, `@page home
@route "/"
@layout root

view {
}
`)
	writeGWDK(t, layout, `@layout root

view {
  <slot />
}
`)
	writeGWDK(t, asset, `@asset images
`)
	writeGWDK(t, plugin, `@plugin tailwind
`)

	app, diagnostics := ParseBuildFiles([]string{page, layout, asset, plugin})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	if len(app.Pages) != 1 || app.Pages[0].ID != "home" {
		t.Fatalf("expected one page, got %#v", app.Pages)
	}
	if len(app.Components) != 0 {
		t.Fatalf("expected no components, got %#v", app.Components)
	}
	if len(app.Layouts) != 1 || app.Layouts[0].ID != "root" {
		t.Fatalf("expected root layout, got %#v", app.Layouts)
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
	if !strings.Contains(output, `"code": "missing_ssr_addon"`) {
		t.Fatalf("expected diagnostic code in JSON: %s", output)
	}
	if diagnostics[0].Pos.Line != 3 || diagnostics[0].Pos.Column != 1 {
		t.Fatalf("expected compiler diagnostic at @render line, got %#v", diagnostics[0].Pos)
	}
	if diagnostics[0].Range == nil ||
		diagnostics[0].Range.Start.Line != 3 || diagnostics[0].Range.Start.Column != 1 ||
		diagnostics[0].Range.End.Line != 3 || diagnostics[0].Range.End.Column != 12 {
		t.Fatalf("expected compiler diagnostic range for @render, got %#v", diagnostics[0].Range)
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
	if diagnostics[0].Code != "parse_error" {
		t.Fatalf("expected parse_error code, got %#v", diagnostics[0])
	}
	if diagnostics[0].Range == nil {
		t.Fatalf("expected parse diagnostic range, got %#v", diagnostics[0])
	}
	if diagnostics[0].Range.Start.Line != 3 || diagnostics[0].Range.Start.Column != 1 ||
		diagnostics[0].Range.End.Line != 3 || diagnostics[0].Range.End.Column != 13 {
		t.Fatalf("unexpected parse diagnostic range: %#v", diagnostics[0].Range)
	}
}

func TestCheckJSONReportsParserDiagnosticRangeAndCode(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.page.gwdk")
	writeGWDK(t, path, `@page bad
@route "/bad"
@render nope
`)

	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected parser diagnostic")
	}
	output := string(payload)
	if !strings.Contains(output, `"code": "parse_error"`) {
		t.Fatalf("expected parser diagnostic code in JSON: %s", output)
	}
	if !strings.Contains(output, `"range"`) ||
		!strings.Contains(output, `"start": {`) ||
		!strings.Contains(output, `"end": {`) {
		t.Fatalf("expected parser diagnostic range in JSON: %s", output)
	}
}

func TestCheckJSONReportsClientStatementDiagnosticRange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "counter.cmp.gwdk")
	writeGWDK(t, path, `@component Counter

client {
  fn Bad() {
    Missing++
  }
}

view {
  <button g:on:click={Bad()}>Bad</button>
}
`)

	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected client diagnostic")
	}
	if len(diagnostics) != 1 {
		t.Fatalf("expected one client diagnostic, got %#v\n%s", diagnostics, payload)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "component_client_error" {
		t.Fatalf("expected component_client_error, got %#v", diagnostic)
	}
	if diagnostic.Pos.Line != 5 || diagnostic.Pos.Column != 1 {
		t.Fatalf("expected client statement diagnostic at line 5, got %#v\n%s", diagnostic.Pos, payload)
	}
	if diagnostic.Range == nil ||
		diagnostic.Range.Start.Line != 5 || diagnostic.Range.Start.Column != 1 ||
		diagnostic.Range.End.Line != 5 || diagnostic.Range.End.Column != 2 {
		t.Fatalf("unexpected client statement diagnostic range: %#v\n%s", diagnostic.Range, payload)
	}
	output := string(payload)
	if !strings.Contains(output, `"code": "component_client_error"`) ||
		!strings.Contains(output, `"line": 5`) ||
		!strings.Contains(output, `unknown island field \"Missing\"`) {
		t.Fatalf("expected client diagnostic JSON details, got: %s", output)
	}
}

func TestCheckJSONReportsClientExpressionDiagnosticRange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "counter.cmp.gwdk")
	writeGWDK(t, path, `import ui "github.com/cssbruno/gowdk/testfixture/islands"

@component Counter

state ui.CounterState = ui.NewCounterState()

client {
  fn Bad() {
    Count = Count && Open
  }
}

view {
  <button g:on:click={Bad()}>{Count}</button>
}
`)

	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected client expression diagnostic")
	}
	if len(diagnostics) != 1 {
		t.Fatalf("expected one client diagnostic, got %#v\n%s", diagnostics, payload)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "component_client_error" {
		t.Fatalf("expected component_client_error, got %#v", diagnostic)
	}
	if diagnostic.Pos.Line != 9 || diagnostic.Pos.Column != 9 {
		t.Fatalf("expected client expression diagnostic at line 9 column 9, got %#v\n%s", diagnostic.Pos, payload)
	}
	if diagnostic.Range == nil ||
		diagnostic.Range.Start.Line != 9 || diagnostic.Range.Start.Column != 9 ||
		diagnostic.Range.End.Line != 9 || diagnostic.Range.End.Column != 22 {
		t.Fatalf("unexpected client expression diagnostic range: %#v\n%s", diagnostic.Range, payload)
	}
	output := string(payload)
	if !strings.Contains(output, `"column": 9`) ||
		!strings.Contains(diagnostic.Message, `operator && requires bools`) {
		t.Fatalf("expected expression diagnostic JSON details, got: %s", output)
	}
}

func writeGWDK(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

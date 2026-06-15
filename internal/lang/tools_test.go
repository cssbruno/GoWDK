package lang

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestCheckFilesValidatesRenderRules(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "dashboard.page.gwdk")
	writeGWDK(t, path, `package app

page dashboard
route "/dashboard"
guard public

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

func TestCheckFilesRejectsReservedActiveComponentExport(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "toggle.cmp.gwdk")
	writeGWDK(t, path, `package components

component Toggle
props {
  active bool
}
exports {
  active bool
}

view {
  <p>{active}</p>
}
`)

	_, diagnostics := CheckFiles(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected reserved export diagnostic")
	}
	if len(diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "component_contract_error" {
		t.Fatalf("expected component_contract_error, got %#v", diagnostic)
	}
	if !strings.Contains(diagnostic.Message, `export "active" uses reserved name "active"`) {
		t.Fatalf("unexpected diagnostic message: %#v", diagnostic)
	}
	if diagnostic.Pos.Line != 8 || diagnostic.Pos.Column != 3 {
		t.Fatalf("expected export diagnostic at line 8, got %#v", diagnostic.Pos)
	}
}

func TestCheckFilesEnrichesIRBeforeValidation(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "newsletter.page.gwdk")
	moduleRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	writeGWDK(t, filepath.Join(root, "go.mod"), `module example.com/gowdk-lang-check

go 1.26.4

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => `+filepath.ToSlash(moduleRoot)+`
`)
	writeGWDK(t, path, `package app

page newsletter
route "/newsletter"
guard public

act Subscribe POST "/newsletter"

go {
  import (
    "context"

    "github.com/cssbruno/gowdk/runtime/response"
  )

  func Subscribe(context.Context) (response.Response, error) {
    return response.RedirectTo("/thanks"), nil
  }
}

view {
  <main>Newsletter</main>
}
`)

	result, diagnostics := CheckFiles(gowdk.Config{}, []string{path})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	for _, binding := range result.Bindings {
		if binding.BlockName == "Subscribe" {
			if binding.Status != source.BackendBindingBound {
				t.Fatalf("expected Subscribe to be bound, got %#v", binding)
			}
			if binding.Signature == "" {
				t.Fatalf("expected Subscribe signature metadata, got %#v", binding)
			}
			return
		}
	}
	t.Fatalf("missing Subscribe binding in %#v", result.Bindings)
}

func TestCheckSourceValidatesUnsavedBuffer(t *testing.T) {
	_, diagnostics := CheckSource(gowdk.Config{}, "untitled.gwdk", []byte(`package app

page post
route "/blog/{slug}"
guard public

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
	if diagnostics[0].Suggestion == "" || !strings.Contains(diagnostics[0].Suggestion, "paths") {
		t.Fatalf("expected dynamic route suggestion, got %#v", diagnostics[0])
	}
}

func TestCheckSourceWithOptionsReportsContractReferenceBinding(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "patients.page.gwdk")
	source := []byte(`package app

page patients
route "/patients"
guard public

view {
  <main>
    <form method="post" action="/patients" g:command="patients.CreatePatient">
      <input name="name" />
    </form>
  </main>
}
`)
	writeGWDK(t, filepath.Join(root, "gowdk.config.go"), "package app\n")

	_, diagnostics := CheckSourceWithOptions(gowdk.Config{}, path, source, CheckOptions{ProjectRoot: root})
	if !diagnostics.HasErrors() {
		t.Fatal("expected missing contract reference diagnostic")
	}
	if len(diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "contract_reference_missing" {
		t.Fatalf("expected contract_reference_missing, got %#v", diagnostic)
	}
	if diagnostic.File != path || diagnostic.Range == nil {
		t.Fatalf("expected source range on current document, got %#v", diagnostic)
	}
	if diagnostic.Range.Start.Line != 9 || diagnostic.Range.Start.Column == 0 {
		t.Fatalf("expected g:command source range, got %#v", diagnostic.Range)
	}
	if !strings.Contains(diagnostic.Message, "command patients.CreatePatient has no scanned Go registration") {
		t.Fatalf("unexpected diagnostic message: %#v", diagnostic)
	}
	if !strings.Contains(diagnostic.Suggestion, "gowdk contracts list") {
		t.Fatalf("expected contract list suggestion, got %#v", diagnostic)
	}
}

func TestCheckSourceValidatesUnsavedLayoutSlots(t *testing.T) {
	_, diagnostics := CheckSource(gowdk.Config{}, "root.layout.gwdk", []byte(`package app

view {
  <main></main>
}
`))
	if !diagnostics.HasErrors() {
		t.Fatal("expected layout slot diagnostic")
	}
	if diagnostics[0].Code != "layout_slot_count" {
		t.Fatalf("expected layout_slot_count diagnostic, got %#v", diagnostics[0])
	}
	if diagnostics[0].File != "root.layout.gwdk" {
		t.Fatalf("expected source path on diagnostic, got %#v", diagnostics[0])
	}
}

func TestCompletionsIncludeCoreLanguageKeywords(t *testing.T) {
	completions := Completions()
	if len(completions) == 0 {
		t.Fatal("expected completions")
	}
	labels := map[string]bool{}
	for _, item := range completions {
		labels[item.Label] = true
	}
	for _, expected := range []string{
		"page",
		"component",
		"layout",
		"package",
		"use",
		"store",
		"props",
		"state",
		"client",
		"computed",
		"emits",
		"g:post",
		"g:if",
		"g:for",
		"g:bind:value",
		"g:ref",
		`param("slug")`,
	} {
		if !labels[expected] {
			t.Fatalf("missing completion %q in %#v", expected, completions)
		}
	}
}

func TestManifestJSONEmitsParsedPage(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "home.page.gwdk")
	writeGWDK(t, path, `package app

page home
route "/"
guard public
layout root

view {
}
`)

	payload, diagnostics := ManifestJSON(gowdk.Config{}, []string{path})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	if !strings.Contains(string(payload), `"home"`) || !strings.Contains(string(payload), `"render": "spa"`) {
		t.Fatalf("unexpected manifest JSON: %s", payload)
	}
}

func TestParseSourceDerivesPageIDFromFilename(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "blog-post.page.gwdk")
	writeGWDK(t, path, `package app

route "/blog/{slug}"
guard public

paths {
  => { slug: "hello" }
}

view {
}
`)

	page, diagnostics := ParseFile(path)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	if page.ID != "blog-post" {
		t.Fatalf("expected derived page ID blog-post, got %q", page.ID)
	}
}

func TestParseSourceKeepsExplicitPageID(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "blog-post.page.gwdk")
	writeGWDK(t, path, `package app

page blog.post
route "/blog/{slug}"
guard public

paths {
  => { slug: "hello" }
}

view {
}
`)

	page, diagnostics := ParseFile(path)
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	if page.ID != "blog.post" {
		t.Fatalf("expected explicit page ID blog.post, got %q", page.ID)
	}
}

func TestManifestJSONUsesConfiguredDefaultRenderMode(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "home.page.gwdk")
	writeGWDK(t, path, `package app

page home
route "/"
guard public

view {
}
`)

	payload, diagnostics := ManifestJSON(gowdk.Config{Render: gowdk.RenderConfig{Default: gowdk.Action}}, []string{path})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
	}
	if !strings.Contains(string(payload), `"render": "action"`) {
		t.Fatalf("expected action render mode in manifest JSON: %s", payload)
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

func TestCheckFilesAcceptsGoInteropExample(t *testing.T) {
	path := filepath.FromSlash("../../examples/go-interop/imported-build.page.gwdk")

	_, diagnostics := CheckFiles(gowdk.Config{}, []string{path})
	if diagnostics.HasErrors() {
		t.Fatal(diagnostics)
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

func TestCheckFilesWarnsForImageWithoutAlt(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "gallery.page.gwdk")
	writeGWDK(t, path, `package app

page gallery
route "/gallery"
guard public

view {
  <main>
    <img src="/hero.png" />
  </main>
}
`)

	_, diagnostics := CheckFiles(gowdk.Config{}, []string{path})
	if diagnostics.HasErrors() {
		t.Fatalf("expected warning-only diagnostics, got %v", diagnostics)
	}
	if len(diagnostics) != 1 {
		t.Fatalf("expected one diagnostic, got %#v", diagnostics)
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "missing_img_alt" || diagnostic.Severity != "warning" {
		t.Fatalf("expected missing_img_alt warning, got %#v", diagnostic)
	}
	if diagnostic.File != path {
		t.Fatalf("expected diagnostic file %q, got %#v", path, diagnostic)
	}
	if diagnostic.Pos.Line != 9 || diagnostic.Pos.Column != 5 {
		t.Fatalf("expected diagnostic at img tag, got %#v", diagnostic.Pos)
	}
	if diagnostic.Range == nil ||
		diagnostic.Range.Start.Line != 9 || diagnostic.Range.Start.Column != 5 ||
		diagnostic.Range.End.Line != 9 || diagnostic.Range.End.Column <= diagnostic.Range.Start.Column {
		t.Fatalf("expected diagnostic range on img tag, got %#v", diagnostic.Range)
	}
	if !strings.Contains(diagnostic.Suggestion, `alt=""`) {
		t.Fatalf("expected alt suggestion, got %#v", diagnostic)
	}
}

func TestCheckFilesAcceptsImagesWithAlt(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "gallery.page.gwdk")
	writeGWDK(t, path, `package app

page gallery
route "/gallery"
guard public

view {
  <main>
    <img src="/hero.png" alt="Hero" />
    <img src="/decorative.png" alt="" />
  </main>
}
`)

	_, diagnostics := CheckFiles(gowdk.Config{}, []string{path})
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestCheckJSONIncludesAccessibilityWarning(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "gallery.page.gwdk")
	writeGWDK(t, path, `package app

page gallery
route "/gallery"
guard public

view {
  <img src="/hero.png" />
}
`)

	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if diagnostics.HasErrors() {
		t.Fatalf("expected warning-only diagnostics, got %v", diagnostics)
	}
	output := string(payload)
	if !strings.Contains(output, `"code": "missing_img_alt"`) ||
		!strings.Contains(output, `"severity": "warning"`) {
		t.Fatalf("expected accessibility warning in JSON: %s", output)
	}
}

func TestCheckFilesWarnsForImageWithoutAltInComponentsAndLayouts(t *testing.T) {
	root := t.TempDir()
	componentPath := filepath.Join(root, "hero.cmp.gwdk")
	layoutPath := filepath.Join(root, "root.layout.gwdk")
	writeGWDK(t, componentPath, `package app

component Hero

view {
  <section>
    <img src="/hero.png" />
  </section>
}
`)
	writeGWDK(t, layoutPath, `package app

view {
  <main>
    <img src="/logo.png" />
    <slot />
  </main>
}
`)

	_, diagnostics := CheckFiles(gowdk.Config{}, []string{componentPath, layoutPath})
	if diagnostics.HasErrors() {
		t.Fatalf("expected warning-only diagnostics, got %v", diagnostics)
	}
	if len(diagnostics) != 2 {
		t.Fatalf("expected two accessibility warnings, got %#v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != "missing_img_alt" || diagnostic.Severity != "warning" {
			t.Fatalf("expected missing_img_alt warning, got %#v", diagnostic)
		}
		if diagnostic.Pos.Column != 5 {
			t.Fatalf("expected diagnostic at img tag, got %#v", diagnostic)
		}
	}
}

func TestClassifySourceUsesCurrentFileKindRules(t *testing.T) {
	cases := []struct {
		path   string
		source string
		kind   FileKind
	}{
		{"home.page.gwdk", "page home", FileKindPage},
		{"hero.cmp.gwdk", "component Hero", FileKindComponent},
		{"hero.gwdk", "component Hero", FileKindComponent},
		{"home.page.gwdk", "// Mention component in docs\npage home", FileKindPage},
		{"root.gwdk", "layout root", FileKindLayout},
		{"root.layout.gwdk", "layout root", FileKindLayout},
		{"images.asset.gwdk", "asset images", FileKindAsset},
		{"security.audit.gwdk", "policy frontend {", FileKindAudit},
		{"security.gwdk", "policy frontend {", FileKindAudit},
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
	writeGWDK(t, page, `package app

page home
route "/"
guard public
layout root

view {
}
`)
	writeGWDK(t, layout, `package app

layout root

view {
  <slot />
}
`)
	writeGWDK(t, asset, `asset images
`)

	app, diagnostics := ParseBuildFiles([]string{page, layout, asset})
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
	writeGWDK(t, path, `package app

page dashboard
route "/dashboard"
guard public

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
	if diagnostics[0].Pos.Line != 7 || diagnostics[0].Pos.Column != 1 {
		t.Fatalf("expected compiler diagnostic at load line, got %#v", diagnostics[0].Pos)
	}
	if diagnostics[0].Range == nil ||
		diagnostics[0].Range.Start.Line != 7 || diagnostics[0].Range.Start.Column != 1 ||
		diagnostics[0].Range.End.Line != 7 || diagnostics[0].Range.End.Column != 7 {
		t.Fatalf("expected compiler diagnostic range for load block, got %#v", diagnostics[0].Range)
	}
	if !strings.Contains(output, "SSR addon is not enabled") {
		t.Fatalf("expected SSR diagnostic in JSON: %s", output)
	}
}

func TestParseFileReportsParserDiagnosticLine(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.page.gwdk")
	writeGWDK(t, path, `package app

page bad
route "/bad"
@unknown nope
`)

	_, diagnostics := ParseFile(path)
	if !diagnostics.HasErrors() {
		t.Fatal("expected parser diagnostic")
	}
	if diagnostics[0].Pos.Line != 5 || diagnostics[0].Pos.Column != 1 {
		t.Fatalf("expected line 5 diagnostic, got %#v", diagnostics[0].Pos)
	}
	if diagnostics[0].Code != "malformed_legacy_metadata" {
		t.Fatalf("expected malformed_legacy_metadata code, got %#v", diagnostics[0])
	}
	if diagnostics[0].Range == nil {
		t.Fatalf("expected parse diagnostic range, got %#v", diagnostics[0])
	}
	if diagnostics[0].Range.Start.Line != 5 || diagnostics[0].Range.Start.Column != 1 ||
		diagnostics[0].Range.End.Line != 5 || diagnostics[0].Range.End.Column != 14 {
		t.Fatalf("unexpected parse diagnostic range: %#v", diagnostics[0].Range)
	}
}

func TestParseSourceReportsRecoveredParserDiagnostics(t *testing.T) {
	_, diagnostics := ParseSource("bad.page.gwdk", []byte(`package app

page bad
use ui ui
@unknown nope
route "/bad"
`))
	if len(diagnostics) != 2 {
		t.Fatalf("expected two parser diagnostics, got %#v", diagnostics)
	}
	if diagnostics[0].Code != "malformed_gowdk_use" || diagnostics[0].Pos.Line != 4 {
		t.Fatalf("unexpected first diagnostic: %#v", diagnostics[0])
	}
	if diagnostics[1].Code != "malformed_legacy_metadata" || diagnostics[1].Pos.Line != 5 {
		t.Fatalf("unexpected second diagnostic: %#v", diagnostics[1])
	}
}

func TestCheckJSONReportsParserDiagnosticRangeAndCode(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad.page.gwdk")
	writeGWDK(t, path, `package app

page bad
route "/bad"
@unknown nope
`)

	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected parser diagnostic")
	}
	output := string(payload)
	if !strings.Contains(output, `"code": "malformed_legacy_metadata"`) {
		t.Fatalf("expected parser diagnostic code in JSON: %s", output)
	}
	if !strings.Contains(output, `"range"`) ||
		!strings.Contains(output, `"start": {`) ||
		!strings.Contains(output, `"end": {`) {
		t.Fatalf("expected parser diagnostic range in JSON: %s", output)
	}
}

func TestCheckJSONIncludesRegisteredFixMetadata(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "old.page.gwdk")
	writeGWDK(t, path, `package app

page old
route "/old"

act submit {
}

view {
}
`)

	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected old endpoint syntax diagnostic")
	}
	output := string(payload)
	if !strings.Contains(output, `"code": "old_action_block_syntax"`) ||
		!strings.Contains(output, `"fix": {`) ||
		!strings.Contains(output, `"rewriter": "endpoint_header_from_message"`) {
		t.Fatalf("expected registered fix metadata in JSON: %s", output)
	}
}

func TestCheckJSONReportsUnsupportedBuildStatementDiagnostic(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "bad-build.page.gwdk")
	writeGWDK(t, path, `package app

page bad
route "/bad"

build {
  title := "Bad"
}

view {
  <h1>Bad</h1>
}
`)

	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected unsupported build statement diagnostic")
	}
	if diagnostics[0].Code != "unsupported_literal_record_syntax" {
		t.Fatalf("expected unsupported_literal_record_syntax code, got %#v", diagnostics[0])
	}
	if diagnostics[0].Pos.Line != 7 || diagnostics[0].Pos.Column != 3 {
		t.Fatalf("expected build statement line diagnostic, got %#v", diagnostics[0].Pos)
	}
	if diagnostics[0].Range == nil ||
		diagnostics[0].Range.Start.Line != 7 || diagnostics[0].Range.Start.Column != 3 ||
		diagnostics[0].Range.End.Line != 7 || diagnostics[0].Range.End.Column != 17 {
		t.Fatalf("expected build statement diagnostic range, got %#v", diagnostics[0].Range)
	}
	output := string(payload)
	if !strings.Contains(output, "unsupported literal record syntax") || !strings.Contains(output, "title :=") {
		t.Fatalf("expected unsupported build syntax diagnostic in JSON: %s", output)
	}
}

func TestCheckJSONReportsClientStatementDiagnosticRange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "counter.cmp.gwdk")
	writeGWDK(t, path, `package app

component Counter

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
	if diagnostic.Pos.Line != 7 || diagnostic.Pos.Column != 1 {
		t.Fatalf("expected client statement diagnostic at line 7, got %#v\n%s", diagnostic.Pos, payload)
	}
	if diagnostic.Range == nil ||
		diagnostic.Range.Start.Line != 7 || diagnostic.Range.Start.Column != 1 ||
		diagnostic.Range.End.Line != 7 || diagnostic.Range.End.Column != 2 {
		t.Fatalf("unexpected client statement diagnostic range: %#v\n%s", diagnostic.Range, payload)
	}
	output := string(payload)
	if !strings.Contains(output, `"code": "component_client_error"`) ||
		!strings.Contains(output, `"line": 7`) ||
		!strings.Contains(output, `unknown island field \"Missing\"`) ||
		!strings.Contains(output, `"suggestion": "Use a field declared by the component props/state contract`) {
		t.Fatalf("expected client diagnostic JSON details, got: %s", output)
	}
}

func TestCheckJSONReportsBadGForSuggestion(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested.cmp.gwdk")
	writeGWDK(t, path, `package app

import ui "github.com/cssbruno/gowdk/testfixture/islands"

component Nested

state ui.NestedState = ui.NewNestedState()

view {
  <ul><li g:for={item of Items}>{item.Name}</li></ul>
}
`)

	payload, diagnostics := CheckJSON(gowdk.Config{}, []string{path})
	if !diagnostics.HasErrors() {
		t.Fatal("expected bad g:for diagnostic")
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "component_field_error" {
		t.Fatalf("expected component_field_error, got %#v\n%s", diagnostic, payload)
	}
	if !strings.Contains(diagnostic.Suggestion, `g:for={item in Items}`) {
		t.Fatalf("expected g:for suggestion, got %#v\n%s", diagnostic, payload)
	}
	if !strings.Contains(string(payload), `"suggestion": "Use g:for={item in Items}`) {
		t.Fatalf("expected suggestion in JSON payload, got: %s", payload)
	}
}

func TestParseLayoutSourceReportsTypedParserDiagnostic(t *testing.T) {
	_, diagnostics := ParseLayoutSource("root.layout.gwdk", []byte(`package layouts

page home

view {
  <slot />
}
`))
	if !diagnostics.HasErrors() {
		t.Fatal("expected layout parser diagnostic")
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "unsupported_layout_metadata" {
		t.Fatalf("expected unsupported_layout_metadata, got %#v", diagnostic)
	}
	if diagnostic.Pos.Line != 3 || diagnostic.Pos.Column != 1 {
		t.Fatalf("expected layout metadata diagnostic at line 3, got %#v", diagnostic.Pos)
	}
	if diagnostic.Range == nil ||
		diagnostic.Range.Start.Line != 3 || diagnostic.Range.Start.Column != 1 ||
		diagnostic.Range.End.Line != 3 || diagnostic.Range.End.Column != 10 {
		t.Fatalf("unexpected layout diagnostic range: %#v", diagnostic.Range)
	}
}

func TestParseComponentSourceReportsTypedParserDiagnostic(t *testing.T) {
	_, diagnostics := ParseComponentSource("badge.cmp.gwdk", []byte(`package components

component Badge
props {
  Count time
}

view {
}
`))
	if !diagnostics.HasErrors() {
		t.Fatal("expected component parser diagnostic")
	}
	diagnostic := diagnostics[0]
	if diagnostic.Code != "unsupported_component_prop_type" {
		t.Fatalf("expected unsupported_component_prop_type, got %#v", diagnostic)
	}
	if diagnostic.Pos.Line != 5 || diagnostic.Pos.Column != 3 {
		t.Fatalf("expected component prop diagnostic at line 5, got %#v", diagnostic.Pos)
	}
	if diagnostic.Range == nil ||
		diagnostic.Range.Start.Line != 5 || diagnostic.Range.Start.Column != 3 ||
		diagnostic.Range.End.Line != 5 || diagnostic.Range.End.Column != 13 {
		t.Fatalf("unexpected component diagnostic range: %#v", diagnostic.Range)
	}
}

func TestParserDiagnosticFallsBackToParseError(t *testing.T) {
	diagnostic := parserDiagnostic("bad.gwdk", []byte("not a typed parser error\n"), errors.New("unknown parser failure"))
	if diagnostic.Code != "parse_error" {
		t.Fatalf("expected parse_error fallback, got %#v", diagnostic)
	}
	if diagnostic.Pos.Line != 0 || diagnostic.Range != nil {
		t.Fatalf("expected no source position for untyped parser error, got %#v", diagnostic)
	}
}

func TestCheckJSONReportsClientExpressionDiagnosticRange(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "counter.cmp.gwdk")
	writeGWDK(t, path, `package app

import ui "github.com/cssbruno/gowdk/testfixture/islands"

component Counter

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
	if diagnostic.Pos.Line != 11 || diagnostic.Pos.Column != 9 {
		t.Fatalf("expected client expression diagnostic at line 11 column 9, got %#v\n%s", diagnostic.Pos, payload)
	}
	if diagnostic.Range == nil ||
		diagnostic.Range.Start.Line != 11 || diagnostic.Range.Start.Column != 9 ||
		diagnostic.Range.End.Line != 11 || diagnostic.Range.End.Column != 22 {
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

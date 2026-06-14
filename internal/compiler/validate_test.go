package compiler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

func TestValidateManifestRejectsMissingPackageDeclaration(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source: sourcePath,
		ID:     "home",
		Route:  "/",
		Blocks: gwdkir.Blocks{View: true},
	}

	err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}})
	if err == nil {
		t.Fatal("expected missing package diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "missing_package_declaration") {
		t.Fatalf("missing package diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsPackageMismatchWithSiblingGoFile(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package views\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(root, "handlers.go")
	if err := os.WriteFile(goFile, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  sourcePath,
		Package: "views",
		ID:      "home",
		Route:   "/",
		Blocks:  gwdkir.Blocks{View: true},
	}

	err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}})
	if err == nil {
		t.Fatal("expected package mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticMessage(diagnostics, "package_mismatch", "views", "app") {
		t.Fatalf("missing package mismatch diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAcceptsPackageMatchingSiblingGoFile(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(root, "handlers.go")
	if err := os.WriteFile(goFile, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  sourcePath,
		Package: "app",
		ID:      "home",
		Route:   "/",
		Guards:  []string{"public"},
		Blocks:  gwdkir.Blocks{View: true},
	}

	if err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}}); err != nil {
		t.Fatalf("expected matching package to validate, got %v", err)
	}
}

func TestValidateManifestIgnoresProjectConfigGoPackage(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "styled.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package css\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(root, "gowdk.config.go")
	if err := os.WriteFile(configFile, []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  sourcePath,
		Package: "css",
		ID:      "styled",
		Route:   "/styled",
		Guards:  []string{"public"},
		Blocks:  gwdkir.Blocks{View: true},
	}

	if err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}}); err != nil {
		t.Fatalf("expected project config package to be ignored, got %v", err)
	}
}

func TestValidateManifestReportsGoPackageParseErrors(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(root, "handlers.go")
	if err := os.WriteFile(goFile, []byte("package app\nfunc Bad("), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  sourcePath,
		Package: "app",
		ID:      "home",
		Route:   "/",
		Blocks:  gwdkir.Blocks{View: true},
	}

	err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}})
	if err == nil {
		t.Fatal("expected Go package error diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "go_package_error") {
		t.Fatalf("missing Go package error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestReportsGoPackageTypeErrors(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(root, "handlers.go")
	if err := os.WriteFile(goFile, []byte("package app\n\nfunc Broken() int { return missing }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  sourcePath,
		Package: "app",
		ID:      "home",
		Route:   "/",
		Blocks:  gwdkir.Blocks{View: true},
	}

	err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}})
	if err == nil {
		t.Fatal("expected Go package type-check diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	diagnostic := firstDiagnostic(diagnostics, "go_package_error")
	if diagnostic == nil {
		t.Fatalf("missing Go package error diagnostic: %#v", diagnostics)
	}
	if diagnostic.Source != goFile {
		t.Fatalf("expected diagnostic source %s, got %#v", goFile, diagnostic)
	}
	if !strings.Contains(diagnostic.Message, "undefined: missing") {
		t.Fatalf("expected undefined symbol in diagnostic, got %q", diagnostic.Message)
	}
	if diagnostic.Span.Start.Line == 0 || diagnostic.Span.Start.Column == 0 {
		t.Fatalf("expected diagnostic source span, got %#v", diagnostic.Span)
	}
}

func TestValidateManifestTypeChecksDefaultScriptWithSiblingGoFiles(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	goFile := filepath.Join(root, "copy.go")
	if err := os.WriteFile(goFile, []byte(`package app

type PageCopy struct {
	Title string
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  sourcePath,
		Package: "app",
		ID:      "home",
		Route:   "/",
		Guards:  []string{"public"},
		Blocks: gwdkir.Blocks{
			View: true,
			GoBlocks: []gwdkir.GoBlock{{
				Body: `func HomeCopy() PageCopy {
	return PageCopy{Title: "GOWDK ships apps"}
}`,
			}},
		},
	}

	if err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}}); err != nil {
		t.Fatalf("expected inline go block to type-check with sibling Go files, got %v", err)
	}
}

func TestValidateManifestTypeChecksDefaultScriptWithGOWDKImports(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  sourcePath,
		Package: "app",
		ID:      "home",
		Route:   "/",
		Guards:  []string{"public"},
		Imports: []gwdkir.Import{{Alias: "strings", Path: "strings"}},
		Blocks: gwdkir.Blocks{
			View: true,
			GoBlocks: []gwdkir.GoBlock{{
				Body: `func HomeSlug() string {
	return strings.ToLower("GOWDK Ships Apps")
}`,
			}},
		},
	}

	if err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}}); err != nil {
		t.Fatalf("expected inline go block to type-check with GOWDK imports, got %v", err)
	}
}

func TestValidateManifestReportsDefaultScriptTypeErrors(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  sourcePath,
		Package: "app",
		ID:      "home",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View: true,
			GoBlocks: []gwdkir.GoBlock{{
				Span: source.SourceSpan{Start: source.SourcePosition{Line: 8, Column: 1}},
				Body: `func BrokenCopy() string {
	return MissingTitle
}`,
			}},
		},
	}

	err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}})
	if err == nil {
		t.Fatal("expected inline go block type-check diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	diagnostic := firstDiagnostic(diagnostics, "go_package_error")
	if diagnostic == nil {
		t.Fatalf("missing Go package error diagnostic: %#v", diagnostics)
	}
	if diagnostic.Source != sourcePath {
		t.Fatalf("expected diagnostic source %s, got %#v", sourcePath, diagnostic)
	}
	if !strings.Contains(diagnostic.Message, "undefined: MissingTitle") {
		t.Fatalf("expected undefined inline go block symbol in diagnostic, got %q", diagnostic.Message)
	}
	if diagnostic.Span.Start.Line < 8 {
		t.Fatalf("expected diagnostic to map to go block source line, got %#v", diagnostic.Span)
	}
}

func TestGoBlockPackageSourceForValidationPreservesLineDirective(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "home.page.gwdk")
	payload, err := goBlockPackageSourceForValidation(packageDeclaration{
		Source:  sourcePath,
		Package: "app",
	}, gwdkir.GoBlock{
		Span: source.SourceSpan{Start: source.SourcePosition{Line: 8, Column: 1}},
		Body: `func BrokenCopy() string {
	return MissingTitle
}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload, "//line "+filepath.ToSlash(sourcePath)+":8") {
		t.Fatalf("missing line directive in validation source:\n%s", payload)
	}
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, sourcePath, payload, parser.ParseComments|parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	function := file.Decls[0].(*ast.FuncDecl)
	position := fileSet.PositionFor(function.Body.List[0].(*ast.ReturnStmt).Results[0].Pos(), true)
	if position.Line < 8 {
		t.Fatalf("line directive did not adjust parsed position to source line: %s:%d:%d\n%s", position.Filename, position.Line, position.Column, payload)
	}
}

func TestValidateManifestTypeChecksDefaultGoBlockAsStaticPackageGo(t *testing.T) {
	root := t.TempDir()
	sourcePath := filepath.Join(root, "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  sourcePath,
		Package: "app",
		ID:      "home",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View: true,
			GoBlocks: []gwdkir.GoBlock{{
				Body: `func StaticSeed() string {
	return MissingSeed
}`,
			}},
		},
	}

	err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}})
	if err == nil {
		t.Fatal("expected default go block type-check diagnostic")
	}
	if !strings.Contains(err.Error(), "undefined: MissingSeed") {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "request-time") {
		t.Fatalf("default go block should not imply request-time behavior: %v", err)
	}
}

func TestValidateManifestSkipsSiblingGoPackageForUnsavedAbsoluteSource(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "handlers.go"), []byte("package main\n\nfunc Broken() int { return missing }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  filepath.Join(root, "unsaved.page.gwdk"),
		Package: "app",
		ID:      "home",
		Route:   "/",
		Guards:  []string{"public"},
		Blocks:  gwdkir.Blocks{View: true},
	}

	if err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}}); err != nil {
		t.Fatalf("expected unsaved absolute source to skip sibling Go package validation, got %v", err)
	}
}

func TestValidateManifestTypeChecksGoPackagesWithModuleImports(t *testing.T) {
	root := t.TempDir()
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte(fmt.Sprintf(`module example.com/app

go 1.26.4

require github.com/cssbruno/gowdk v0.0.0

replace github.com/cssbruno/gowdk => %s
`, filepath.ToSlash(repoRoot))), 0o644); err != nil {
		t.Fatal(err)
	}
	sourceDir := filepath.Join(root, "features", "auth")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sourceDir, "handlers.go"), []byte(`package auth

import "github.com/cssbruno/gowdk/runtime/form"

func Email(values form.Values) string {
	return values.First("email")
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		Source:  filepath.Join(sourceDir, "login.page.gwdk"),
		Package: "auth",
		ID:      "login",
		Route:   "/login",
		Guards:  []string{"public"},
		Blocks:  gwdkir.Blocks{View: true},
	}

	if err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}}); err != nil {
		t.Fatalf("expected module imports to type-check, got %v", err)
	}
}

func TestValidateManifestAcceptsQualifiedComponentUse(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Uses:    []gwdkir.Use{{Alias: "ui", Package: "components"}},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><ui.Hero /></main>`,
			},
		}},
		Components: []gwdkir.Component{{
			Package: "components",
			Name:    "Hero",
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<section>Hero</section>`},
		}},
	}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected qualified component use to validate, got %v", err)
	}
}

func TestValidateManifestRejectsUnknownGOWDKUsePackage(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		Package: "pages",
		ID:      "home",
		Route:   "/",
		Uses:    []gwdkir.Use{{Alias: "ui", Package: "missing"}},
		Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main><ui.Hero /></main>`},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown use package diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_gowdk_use_package") {
		t.Fatalf("missing unknown package diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownGOWDKUseAlias(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main><ui.Hero /></main>`},
		}},
		Components: []gwdkir.Component{{Package: "components", Name: "Hero"}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown use alias diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_gowdk_use_alias") {
		t.Fatalf("missing unknown alias diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestUnknownGOWDKUseAliasPointsToComponentTag(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: "<main>\n  <ui.Hero />\n</main>",
				Spans: gwdkir.BlockSpans{
					View:          source.SourceSpan{Start: source.SourcePosition{Line: 8, Column: 1}, End: source.SourcePosition{Line: 8, Column: 7}},
					ViewBodyStart: source.SourcePosition{Line: 9, Column: 1},
				},
			},
		}},
		Components: []gwdkir.Component{{Package: "components", Name: "Hero"}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown use alias diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "unknown_gowdk_use_alias")
	if diagnostic == nil {
		t.Fatalf("missing unknown alias diagnostic: %#v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 10, 3, 10, 14)
}

func TestValidateManifestRejectsUnknownQualifiedComponent(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Uses:    []gwdkir.Use{{Alias: "ui", Package: "components"}},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main><ui.Missing /></main>`},
		}},
		Components: []gwdkir.Component{{Package: "components", Name: "Hero"}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown component diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_gowdk_component") {
		t.Fatalf("missing unknown component diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestUnknownQualifiedComponentPointsToComponentTag(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Uses:    []gwdkir.Use{{Alias: "ui", Package: "components"}},
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: "<main>\n  <ui.Missing />\n</main>",
				Spans: gwdkir.BlockSpans{
					View:          source.SourceSpan{Start: source.SourcePosition{Line: 12, Column: 1}, End: source.SourcePosition{Line: 12, Column: 7}},
					ViewBodyStart: source.SourcePosition{Line: 13, Column: 1},
				},
			},
		}},
		Components: []gwdkir.Component{{Package: "components", Name: "Hero"}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown component diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "unknown_gowdk_component")
	if diagnostic == nil {
		t.Fatalf("missing unknown component diagnostic: %#v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 14, 3, 14, 17)
}

func TestValidateManifestRejectsComponentRefToLayoutOnlyUsePackage(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Uses:    []gwdkir.Use{{Alias: "chrome", Package: "layouts"}},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main><chrome.Root /></main>`},
		}},
		Layouts: []gwdkir.Layout{{Package: "layouts", ID: "root"}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown component diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_gowdk_component") {
		t.Fatalf("missing unknown component diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsComponentScopedComponentRefToStoreOnlyUsePackage(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "stores",
			ID:      "cart",
			Route:   "/cart",
			Imports: []gwdkir.Import{{
				Alias: "ui",
				Path:  "github.com/cssbruno/gowdk/testfixture/islands",
			}},
			Stores: []gwdkir.Store{{
				Name: "cart",
				Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
				Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
			}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<main>Cart</main>`},
		}},
		Components: []gwdkir.Component{{
			Package: "marketing",
			Name:    "Hero",
			Uses:    []gwdkir.Use{{Alias: "stores", Package: "stores"}},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<section><stores.Cart /></section>`},
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown component diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_gowdk_component") {
		t.Fatalf("missing unknown component diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAcceptsComponentScopedGOWDKUse(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{
		{
			Package: "marketing",
			Name:    "Hero",
			Uses:    []gwdkir.Use{{Alias: "icons", Package: "icons"}},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<section><icons.Badge /></section>`},
		},
		{
			Package: "icons",
			Name:    "Badge",
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<strong>GOWDK</strong>`},
		},
	}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected component-scoped use to validate, got %v", err)
	}
}

func TestValidateManifestRejectsUnknownComponentScopedGOWDKUseAlias(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Package: "marketing",
		Name:    "Hero",
		Blocks:  gwdkir.Blocks{View: true, ViewBody: `<section><icons.Badge /></section>`},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown component use alias diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_gowdk_use_alias") {
		t.Fatalf("missing unknown alias diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownComponentScopedGOWDKUsePackage(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Package: "marketing",
		Name:    "Hero",
		Uses:    []gwdkir.Use{{Alias: "icons", Package: "icons"}},
		Blocks:  gwdkir.Blocks{View: true, ViewBody: `<section><icons.Badge /></section>`},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown component use package diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_gowdk_use_package") {
		t.Fatalf("missing unknown package diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRejectsSSRWithoutAddon(t *testing.T) {
	page := gwdkir.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Guards: []string{"auth.required"},
		Render: gowdk.SSR,
		Blocks: gwdkir.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Code != "missing_ssr_addon" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
	if !strings.Contains(diagnostics[0].Message, "enable ssr.Addon()") {
		t.Fatalf("diagnostic should suggest enabling ssr addon: %s", diagnostics[0].Message)
	}
}

func TestValidatePageWarnsOnMissingGuard(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		ID:     "home",
		Route:  "/",
		Source: sourcePath,
		Blocks: gwdkir.Blocks{View: true},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irGuardlessPage(page))
	var guard *ValidationError
	for i := range diagnostics {
		if diagnostics[i].Code == "missing_page_guard" {
			guard = &diagnostics[i]
		}
	}
	if guard == nil {
		t.Fatalf("missing missing_page_guard diagnostic: %#v", diagnostics)
	}
	if guard.Severity != SeverityWarning {
		t.Fatalf("missing_page_guard should be a warning, got severity %v", guard.Severity)
	}
	// A guardless page must not fail the build; the runtime denies the route.
	if ValidationErrors(diagnostics).HasErrors() {
		t.Fatalf("guardless page should not produce build errors: %#v", diagnostics)
	}
}

func TestValidatePageErrorsOnGuardlessBackendEndpoints(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "signup.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		ID:     "signup",
		Route:  "/signup",
		Source: sourcePath,
		Blocks: gwdkir.Blocks{
			View:    true,
			Actions: []gwdkir.Action{{Name: "Submit"}},
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irGuardlessPage(page))
	var guard *ValidationError
	for i := range diagnostics {
		if diagnostics[i].Code == "missing_page_guard" {
			guard = &diagnostics[i]
		}
	}
	if guard == nil {
		t.Fatalf("missing missing_page_guard diagnostic: %#v", diagnostics)
	}
	if guard.Severity != SeverityError {
		t.Fatalf("guardless page with endpoints should be an error, got severity %v", guard.Severity)
	}
	// The derived act/api/fragment endpoints would be public, so the build fails.
	if !ValidationErrors(diagnostics).HasErrors() {
		t.Fatalf("guardless page with endpoints should fail the build: %#v", diagnostics)
	}
}

func TestValidatePageErrorsOnGuardlessBackendEndpointsWithoutSourcePath(t *testing.T) {
	page := gwdkir.Page{
		ID:    "signup",
		Route: "/signup",
		Blocks: gwdkir.Blocks{
			View:    true,
			Actions: []gwdkir.Action{{Name: "Submit"}},
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, page)
	var guard *ValidationError
	for i := range diagnostics {
		if diagnostics[i].Code == "missing_page_guard" {
			guard = &diagnostics[i]
		}
	}
	if guard == nil {
		t.Fatalf("missing missing_page_guard diagnostic: %#v", diagnostics)
	}
	if guard.Severity != SeverityError {
		t.Fatalf("guardless page with endpoints should be an error, got severity %v", guard.Severity)
	}
}

func TestValidatePageRequiresPublicGuardToBeExclusive(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "home.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		ID:     "home",
		Route:  "/",
		Source: sourcePath,
		Guards: []string{"public", "auth.required"},
		Blocks: gwdkir.Blocks{View: true},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if !hasDiagnosticCode(diagnostics, "public_guard_exclusive") {
		t.Fatalf("missing public_guard_exclusive diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRejectsProtectedGuardOnBuildTimePage(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "dashboard.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Source: sourcePath,
		Guards: []string{"auth.required"},
		Render: gowdk.SPA,
		Blocks: gwdkir.Blocks{View: true},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if !hasDiagnosticCode(diagnostics, "guard_requires_request_render") {
		t.Fatalf("missing guard_requires_request_render diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRejectsProtectedGuardOnBuildTimePageWithMissingSourceFile(t *testing.T) {
	page := gwdkir.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Source: filepath.Join(t.TempDir(), "missing.page.gwdk"),
		Guards: []string{"auth.required"},
		Render: gowdk.SPA,
		Blocks: gwdkir.Blocks{View: true},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if !hasDiagnosticCode(diagnostics, "guard_requires_request_render") {
		t.Fatalf("missing guard_requires_request_render diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageAllowsProtectedGuardOnRequestTimePage(t *testing.T) {
	sourcePath := filepath.Join(t.TempDir(), "dashboard.page.gwdk")
	if err := os.WriteFile(sourcePath, []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	page := gwdkir.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Source: sourcePath,
		Guards: []string{"auth.required"},
		Render: gowdk.SSR,
		Blocks: gwdkir.Blocks{View: true},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, irPage(page))
	if hasDiagnosticCode(diagnostics, "guard_requires_request_render") {
		t.Fatalf("unexpected guard_requires_request_render diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageAllowsSSRWithAddon(t *testing.T) {
	page := gwdkir.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.SSR,
		Blocks: gwdkir.Blocks{
			Load: true,
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, irPage(page))
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicatePageIDsAndComponentNames(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{
			{ID: "home", Route: "/", Source: "pages/home.page.gwdk", Blocks: gwdkir.Blocks{View: true}},
			{ID: "home", Route: "/again", Source: "pages/home-again.page.gwdk", Blocks: gwdkir.Blocks{View: true}},
		},
		Components: []gwdkir.Component{
			{Name: "Hero", Source: "components/hero.cmp.gwdk"},
			{Name: "Hero", Source: "components/hero-copy.cmp.gwdk"},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate identity diagnostics")
	}
	diagnostics, ok := err.(ValidationErrors)
	if !ok {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	codes := map[string]bool{}
	for _, diagnostic := range diagnostics {
		codes[diagnostic.Code] = true
		if diagnostic.Source == "" {
			t.Fatalf("expected source on duplicate diagnostic: %#v", diagnostic)
		}
	}
	if !codes["duplicate_page_id"] {
		t.Fatalf("Missing duplicate_page_id diagnostic: %#v", diagnostics)
	}
	if !codes["duplicate_component_name"] {
		t.Fatalf("Missing duplicate_component_name diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsPageStoreDeclaration(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:     "cart",
		Route:  "/cart",
		Source: "pages/cart.page.gwdk",
		Guards: []string{"public"},
		Imports: []gwdkir.Import{{
			Alias: "ui",
			Path:  "github.com/cssbruno/gowdk/testfixture/islands",
		}},
		Stores: []gwdkir.Store{{
			Name: "cart",
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		}},
		Blocks: gwdkir.Blocks{View: true, ViewBody: `<main>Cart</main>`},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected valid store declaration, got %v", err)
	}
}

func TestValidateManifestRejectsDuplicatePageStore(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:     "cart",
		Route:  "/cart",
		Source: "pages/cart.page.gwdk",
		Stores: []gwdkir.Store{
			{
				Name: "cart",
				Span: source.SourceSpan{Start: source.SourcePosition{Line: 5, Column: 1}, End: source.SourcePosition{Line: 5, Column: 40}},
			},
			{
				Name: "cart",
				Span: source.SourceSpan{Start: source.SourcePosition{Line: 6, Column: 1}, End: source.SourcePosition{Line: 6, Column: 40}},
			},
		},
		Blocks: gwdkir.Blocks{View: true, ViewBody: `<main>Cart</main>`},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate store diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "duplicate_page_store")
	if diagnostic == nil {
		t.Fatalf("Missing duplicate_page_store diagnostic: %v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 6, 1, 6, 40)
}

func TestValidateManifestRejectsUnknownComponentStoreUse(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			ID:     "cart",
			Route:  "/cart",
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<main><CartButton /></main>`},
		}},
		Components: []gwdkir.Component{{
			Name:   "CartButton",
			Source: "components/cart-button.cmp.gwdk",
			Blocks: gwdkir.Blocks{
				Client:     true,
				ClientBody: "use cart",
				Spans: gwdkir.BlockSpans{
					Client: source.SourceSpan{Start: source.SourcePosition{Line: 4, Column: 1}, End: source.SourcePosition{Line: 4, Column: 9}},
				},
				View:     true,
				ViewBody: `<button>Cart</button>`,
			},
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown store diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "unknown_component_store")
	if diagnostic == nil {
		t.Fatalf("Missing unknown_component_store diagnostic: %v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 5, 1, 5, 2)
}

func TestValidateManifestAcceptsQualifiedComponentStoreUse(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "stores",
			ID:      "cart",
			Route:   "/cart",
			Imports: []gwdkir.Import{{
				Alias: "ui",
				Path:  "github.com/cssbruno/gowdk/testfixture/islands",
			}},
			Stores: []gwdkir.Store{{
				Name: "cart",
				Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
				Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
			}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<main>Cart</main>`},
		}},
		Components: []gwdkir.Component{{
			Package: "components",
			Name:    "CartButton",
			Uses:    []gwdkir.Use{{Alias: "stores", Package: "stores"}},
			Blocks: gwdkir.Blocks{
				Client:     true,
				ClientBody: "use stores.cart",
				View:       true,
				ViewBody:   `<button>Cart</button>`,
			},
		}},
	}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected qualified store use to validate, got %v", err)
	}
}

func TestValidateManifestRejectsUnknownQualifiedComponentStoreUseAlias(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "stores",
			ID:      "cart",
			Route:   "/cart",
			Stores:  []gwdkir.Store{{Name: "cart"}},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main>Cart</main>`},
		}},
		Components: []gwdkir.Component{{
			Package: "components",
			Name:    "CartButton",
			Blocks: gwdkir.Blocks{
				Client:     true,
				ClientBody: "use stores.cart",
				View:       true,
				ViewBody:   `<button>Cart</button>`,
			},
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown store alias diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_gowdk_use_alias") {
		t.Fatalf("missing unknown alias diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownQualifiedComponentStoreName(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "stores",
			ID:      "cart",
			Route:   "/cart",
			Stores:  []gwdkir.Store{{Name: "cart"}},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main>Cart</main>`},
		}},
		Components: []gwdkir.Component{{
			Package: "components",
			Name:    "CartButton",
			Uses:    []gwdkir.Use{{Alias: "stores", Package: "stores"}},
			Blocks: gwdkir.Blocks{
				Client:     true,
				ClientBody: "use stores.missing",
				View:       true,
				ViewBody:   `<button>Cart</button>`,
			},
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown store diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_component_store") {
		t.Fatalf("missing unknown store diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsRedundantComponentImplementations(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{
		{
			Name:   "Hero",
			Source: "components/hero.cmp.gwdk",
			Props:  []gwdkir.Prop{{Name: "title", Type: "string"}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<section><h1>{title}</h1></section>`},
		},
		{
			Name:   "Feature",
			Source: "components/feature.cmp.gwdk",
			Props:  []gwdkir.Prop{{Name: "title", Type: "string"}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<section>
  // ignored by fingerprint
  <h1>{title}</h1>
</section>`},
		},
	}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected redundant component diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "redundant_component_implementation") {
		t.Fatalf("Missing redundant component diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsRedundantComponentImplementationsWithNormalizedAttrs(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{
		{
			Name:   "PrimaryButton",
			Source: "components/primary-button.cmp.gwdk",
			Props:  []gwdkir.Prop{{Name: "label", Type: "string"}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<button id="save" class="primary large">{label}</button>`},
		},
		{
			Name:   "SaveButton",
			Source: "components/save-button.cmp.gwdk",
			Props:  []gwdkir.Prop{{Name: "label", Type: "string"}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<button class="large primary" id="save">{label}</button>`},
		},
	}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected redundant component diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "redundant_component_implementation") {
		t.Fatalf("Missing redundant component diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsRedundantTypedComponentsWithCanonicalImportsAndEvents(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{
		{
			Name:    "Counter",
			Source:  "components/counter.cmp.gwdk",
			Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			PropsType: gwdkir.GoRef{
				Alias: "ui",
				Name:  "CounterProps",
			},
			State: gwdkir.StateContract{
				Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
				Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
			},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<button g:on:click={Count=Count+1}>{Label}:{Count}</button>`},
		},
		{
			Name:    "Stepper",
			Source:  "components/stepper.cmp.gwdk",
			Imports: []gwdkir.Import{{Alias: "widgets", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			PropsType: gwdkir.GoRef{
				Alias: "widgets",
				Name:  "CounterProps",
			},
			State: gwdkir.StateContract{
				Type: gwdkir.GoRef{Alias: "widgets", Name: "CounterState"},
				Init: gwdkir.GoRef{Alias: "widgets", Name: "NewCounterState"},
			},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<button g:on:click={Count = Count + 1}>{Label}:{Count}</button>`},
		},
	}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected redundant component diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "redundant_component_implementation") {
		t.Fatalf("Missing redundant component diagnostic: %#v", diagnostics)
	}
	if !hasDiagnosticMessage(diagnostics, "redundant_component_implementation", "components/counter.cmp.gwdk", "components/stepper.cmp.gwdk") {
		t.Fatalf("redundant diagnostic should point to both component sources: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsSameViewWithDifferentContracts(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{
		{
			Name:   "Hero",
			Source: "components/hero.cmp.gwdk",
			Props:  []gwdkir.Prop{{Name: "title", Type: "string"}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<section>Same</section>`},
		},
		{
			Name:   "Feature",
			Source: "components/feature.cmp.gwdk",
			Props:  []gwdkir.Prop{{Name: "subtitle", Type: "string"}},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<section>Same</section>`},
		},
	}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected different contracts to be allowed, got %v", err)
	}
}

func TestValidateManifestAllowsSameViewWithDifferentTypedContracts(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{
		{
			Name:    "CounterShell",
			Source:  "components/counter-shell.cmp.gwdk",
			Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			State: gwdkir.StateContract{
				Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
				Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
			},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<section>Same</section>`},
		},
		{
			Name:    "OtherShell",
			Source:  "components/other-shell.cmp.gwdk",
			Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
			State: gwdkir.StateContract{
				Type: gwdkir.GoRef{Alias: "ui", Name: "OtherState"},
				Init: gwdkir.GoRef{Alias: "ui", Name: "NewOtherState"},
			},
			Blocks: gwdkir.Blocks{View: true, ViewBody: `<section>Same</section>`},
		},
	}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected different typed contracts to be allowed, got %v", err)
	}
}

func TestValidateManifestResolvesGoTypedComponentContracts(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: gwdkir.GoRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Count++}>{Label}: {Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected typed component contracts to validate, got %v", err)
	}
}

func TestValidateManifestAllowsEventModifiers(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button g:on:click.prevent.stop.once.capture.debounce(1s)={Count++}>{Count}</button><button g:on:input.throttle(250ms)={Count++}>{Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected event modifiers to validate, got %v", err)
	}
}

func TestValidateManifestRejectsBadEventModifier(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button g:on:click.passive={Count++}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unsupported event modifier diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsBadDebounceDuration(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button g:on:click.debounce(soon)={Count++}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected invalid debounce duration diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDebounceThrottleCombination(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button g:on:click.debounce(100ms).throttle(100ms)={Count++}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected debounce/throttle compatibility diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestResolvesUnaliasedGoTypedComponentImports(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: gwdkir.GoRef{
			Alias: "islands",
			Name:  "CounterProps",
		},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "islands", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "islands", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Count++}>{Label}: {Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected unaliased Go imports to validate, got %v", err)
	}
}

func TestValidateManifestRejectsMissingGoTypedComponentField(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Missing++}>{Missing}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected Missing field diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsMissingGoTypedComponentPackage(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/Missing"}},
		PropsType: gwdkir.GoRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<p>{Label}</p>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected Missing package diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_contract_error") {
		t.Fatalf("Missing component_contract_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsMissingGoTypedComponentType(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: gwdkir.GoRef{
			Alias: "ui",
			Name:  "MissingProps",
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<p>{Label}</p>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected Missing type diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_contract_error") {
		t.Fatalf("Missing component_contract_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsReservedActiveExportName(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Package: "components",
		Name:    "Toggle",
		Source:  "components/toggle.cmp.gwdk",
		Props:   []gwdkir.Prop{{Name: "active", Type: "bool"}},
		Exports: []gwdkir.Export{{
			Name: "active",
			Type: "bool",
			Span: testSourceSpan(8, 3, 8, 14),
		}},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<p>{active}</p>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected reserved export diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	diagnostic := firstDiagnostic(diagnostics, "component_contract_error")
	if diagnostic == nil {
		t.Fatalf("missing component_contract_error diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(diagnostic.Message, `export "active" uses reserved name "active"`) {
		t.Fatalf("unexpected reserved export diagnostic: %#v", diagnostic)
	}
	assertSourceSpan(t, diagnostic.Span, 8, 3, 8, 14)
}

func TestValidateManifestAllowsClientFunctionEventCall(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Increment() {
  Count++
}`,
			View:     true,
			ViewBody: `<button g:on:click={Increment()}>{Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected client function event call to validate, got %v", err)
	}
}

func TestValidateManifestAllowsDeclaredComponentEmit(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Emits: []gwdkir.Emit{{
			Name:   "select",
			Params: []gwdkir.EmitParam{{Name: "id", Type: "int"}},
		}},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Select() {
  emit select(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Select()}>{Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected declared component emit to validate, got %v", err)
	}
}

func TestValidateManifestRejectsDuplicateComponentEmitNames(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:   "Picker",
		Source: "components/picker.cmp.gwdk",
		Emits: []gwdkir.Emit{
			{
				Name: "select",
				Span: source.SourceSpan{
					Start: source.SourcePosition{Line: 4, Column: 3},
					End:   source.SourcePosition{Line: 4, Column: 16},
				},
			},
			{
				Name: "select",
				Span: source.SourceSpan{
					Start: source.SourcePosition{Line: 5, Column: 3},
					End:   source.SourcePosition{Line: 5, Column: 20},
				},
			},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate component emit diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "duplicate_component_emit")
	if diagnostic == nil {
		t.Fatalf("Missing duplicate_component_emit diagnostic: %v", err)
	}
	if !strings.Contains(diagnostic.Message, `duplicate emit "select"`) {
		t.Fatalf("unexpected diagnostic message: %s", diagnostic.Message)
	}
	assertSourceSpan(t, diagnostic.Span, 5, 3, 5, 20)
}

func TestValidateManifestRejectsUnknownComponentEmit(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Select() {
  emit select(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Select()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown component emit diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), `unknown component event "select"`) {
		t.Fatalf("unexpected diagnostics: %v", err)
	}
}

func TestValidateManifestClientParseErrorPointsToClientLine(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:   "Counter",
		Source: "components/counter.cmp.gwdk",
		Blocks: gwdkir.Blocks{
			Client:     true,
			ClientBody: "fn Bad() {\n  if Count {\n  }\n}",
			Spans: gwdkir.BlockSpans{
				Client: source.SourceSpan{
					Start: source.SourcePosition{Line: 10, Column: 1},
					End:   source.SourcePosition{Line: 14, Column: 1},
				},
			},
			View:     true,
			ViewBody: `<button>Bad</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected client parse diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "component_client_error")
	if diagnostic == nil {
		t.Fatalf("Missing component_client_error diagnostic: %v", err)
	}
	if !strings.Contains(diagnostic.Message, "unsupported syntax") {
		t.Fatalf("unexpected diagnostic message: %s", diagnostic.Message)
	}
	assertSourceSpan(t, diagnostic.Span, 12, 1, 12, 2)
}

func TestValidateManifestRejectsComponentEmitPayloadTypeMismatch(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Emits: []gwdkir.Emit{{
			Name:   "select",
			Params: []gwdkir.EmitParam{{Name: "id", Type: "string"}},
		}},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Select() {
  emit select(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Select()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected component emit payload type diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "component event select argument 1 expects string, got int") {
		t.Fatalf("unexpected diagnostics: %v", err)
	}
}

func TestValidateManifestAllowsClientFunctionParams(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Add(step int) {
  Count = Count + step
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add(Count + 1)}>{Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected client function params to validate, got %v", err)
	}
}

func TestValidateManifestAllowsClientHelperFunctionReturns(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Next(value int) int {
  return value + 1
}

fn Add() {
  Count = Next(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add()}>{Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected client helper function to validate, got %v", err)
	}
}

func TestValidateManifestAllowsClientBuiltins(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `computed ItemCount string {
  return string(len(Items))
}

fn SetCount() {
  Count = len(Items) + int("1")
}`,
			View:     true,
			ViewBody: `<button g:on:click={SetCount()}>{ItemCount}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected client built-ins to validate, got %v", err)
	}
}

func TestValidateManifestAllowsAsyncFetchJSONClientFunction(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `async fn Refresh() {
  Items = await fetchJSON[[]ui.Item]("/api/items")
}`,
			View:     true,
			ViewBody: `<button g:on:click={Refresh()}>{len(Items)}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected async fetchJSON function to validate, got %v", err)
	}
}

func TestValidateManifestRejectsAwaitOutsideAsyncClientFunction(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Refresh() {
  Items = await fetchJSON[[]ui.Item]("/api/items")
}`,
			View:     true,
			ViewBody: `<button g:on:click={Refresh()}>{len(Items)}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected await outside async diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "await is only supported inside async client functions") {
		t.Fatalf("Missing async await diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsAsyncFetchJSONNonStringURL(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `async fn Refresh() {
  Items = await fetchJSON[[]ui.Item](Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Refresh()}>{len(Items)}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected fetchJSON URL diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "fetchJSON url must be string") {
		t.Fatalf("Missing fetchJSON URL diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsBadClientBuiltinArg(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Bad() {
  Count = len(Count)
}`,
			View:     true,
			ViewBody: `<button g:on:click={Bad()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected Bad built-in argument diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsHelperAsEventHandler(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Next(value int) int {
  return value + 1
}`,
			View:     true,
			ViewBody: `<button g:on:click={Next(Count)}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected helper event handler diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsHelperReturnMismatch(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Bad() int {
  return Open
}

fn Add() {
  Count = Bad()
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected helper return mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsHelperCallCycle(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn A() int {
  return B()
}

fn B() int {
  return A()
}

fn Add() {
  Count = A()
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected helper call cycle diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsClientExpressionTypeMismatch(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Bad(step int) {
  Open = Count + step
}`,
			View:     true,
			ViewBody: `<button g:on:click={Bad(1)}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected expression type mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsClientFunctionArgumentMismatch(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Add(step int) {
  Count = step
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected argument mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownClientFunctionEventCall(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Increment() {
  Count++
}`,
			View:     true,
			ViewBody: `<button g:on:click={Missing()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown client function diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsClientFunctionUnknownStateField(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Increment() {
  Missing++
}`,
			View:     true,
			ViewBody: `<button g:on:click={Increment()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected client function field diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsClientFunctionMutatingProp(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: gwdkir.GoRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Rename() {
  Label = "changed"
}`,
			View:     true,
			ViewBody: `<button g:on:click={Rename()}>{Label}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected prop mutation diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsLifecycleAndEffects(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `on mount {
  Open = true
}

effect when Count {
  Open = false
  return {
    Open = true
  }
}

on destroy {
  Open = false
}`,
			View:     true,
			ViewBody: `<button g:on:click={Count++}>{Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected lifecycle/effects to validate, got %v", err)
	}
}

func TestValidateManifestRejectsEffectUnknownDependency(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `effect when Missing {
  Open = true
}`,
			View:     true,
			ViewBody: `<button>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown effect dependency diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsDOMRefFocusCall(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `ref searchInput HTMLInputElement

fn FocusSearch() {
  searchInput.Focus()
}`,
			View:     true,
			ViewBody: `<input g:ref={searchInput} /><button g:on:click={FocusSearch()}>Focus</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected DOM ref focus call to validate, got %v", err)
	}
}

func TestValidateManifestRejectsUnknownDOMRefBinding(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			Client:     true,
			ClientBody: `ref searchInput HTMLInputElement`,
			View:       true,
			ViewBody:   `<input g:ref={missingInput} />`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown DOM ref binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicateDOMRefBinding(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			Client:     true,
			ClientBody: `ref searchInput HTMLInputElement`,
			View:       true,
			ViewBody:   `<input g:ref={searchInput} /><input g:ref={searchInput} />`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate DOM ref binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnboundUsedDOMRef(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `ref searchInput HTMLInputElement

fn FocusSearch() {
  searchInput.Focus()
}`,
			View:     true,
			ViewBody: `<button g:on:click={FocusSearch()}>Focus</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unbound DOM ref diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsGIfBoolExpression(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<p g:if={Count > 0 && !Open}>{Count}</p>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected g:if bool expression to validate, got %v", err)
	}
}

func TestValidateManifestAllowsGElseIfExpression(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<p g:if={Open}>{Count}</p><p g:else-if={Count > 0}>{Count}</p><p g:else>Closed</p>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected g:else-if expression to validate, got %v", err)
	}
}

func TestValidateManifestRejectsGElseIfNonBoolExpression(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<p g:if={Open}>{Count}</p><p g:else-if={Count}>{Count}</p>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected g:else-if non-bool diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsGIfNonBoolExpression(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<p g:if={Count}>{Count}</p>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected g:if non-bool diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsNestedAndIndexExpressions(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<section g:if={User.Open && Items[0].Name == "first" && Flags[Count]}>{Count}</section>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected nested/index expressions to validate, got %v", err)
	}
}

func TestValidateManifestAllowsGForListRendering(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<ul><li g:for={item in Items} g:key={item.ID}>{item.Name}</li></ul>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected g:for list rendering to validate, got %v", err)
	}
}

func TestValidateManifestAllowsGForIndexVariable(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<ol><li aria-posinset={i} g:for={item, i in Items} g:key={item.ID}>{i}: {item.Name}</li></ol>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected g:for index variable to validate, got %v", err)
	}
}

func TestValidateManifestAllowsListMutationBuiltins(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn AddItem() {
  append(Items, { ID: "third", Name: "third", Done: false })
}

fn RemoveFirst() {
  remove(Items, 0)
}

fn SwapFirstTwo() {
  move(Items, 1, 0)
}`,
			View:     true,
			ViewBody: `<ul><li g:for={item, i in Items} g:key={item.ID}><button g:on:click={remove(Items, i)}>{item.Name}</button></li></ul><button g:on:click={AddItem()}>Add</button><button g:on:click={RemoveFirst()}>Remove</button><button g:on:click={SwapFirstTwo()}>Swap</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected list mutation built-ins to validate, got %v", err)
	}
}

func TestValidateManifestRejectsBadAppendItemField(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn AddItem() {
  append(Items, { Missing: "third" })
}`,
			View:     true,
			ViewBody: `<ul><li g:for={item in Items} g:key={item.ID}>{item.Name}</li></ul><button g:on:click={AddItem()}>Add</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected Bad append item diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("Missing Bad append field diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsGForWithoutKey(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<ul><li g:for={item in Items}>{item.Name}</li></ul>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected Missing g:key diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") || !strings.Contains(err.Error(), "g:for requires g:key") {
		t.Fatalf("Missing g:for Missing key diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownGForItemField(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<ul><li g:for={item in Items} g:key={item.ID}>{item.Missing}</li></ul>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown item field diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") || !strings.Contains(err.Error(), "item.Missing") {
		t.Fatalf("Missing unknown item field diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestViewEventDiagnosticPointsToExpression(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button g:on:click={Missing()}>{Count}</button>`,
			Spans: gwdkir.BlockSpans{
				View: source.SourceSpan{Start: source.SourcePosition{Line: 9, Column: 1}, End: source.SourcePosition{Line: 9, Column: 7}},
			},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected invalid event expression diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "component_field_error")
	if diagnostic == nil || !strings.Contains(diagnostic.Message, "Missing()") {
		t.Fatalf("Missing event expression diagnostic: %#v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 10, 21, 10, 30)
}

func TestValidateManifestUnknownViewFieldDiagnosticPointsToIdentifier(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:   "Counter",
		Source: "components/counter.cmp.gwdk",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button>{Missing}</button>`,
			Spans: gwdkir.BlockSpans{
				View: source.SourceSpan{Start: source.SourcePosition{Line: 4, Column: 1}, End: source.SourcePosition{Line: 4, Column: 7}},
			},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown field diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "component_field_error")
	if diagnostic == nil || !strings.Contains(diagnostic.Message, `"Missing"`) {
		t.Fatalf("Missing unknown field diagnostic: %#v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 5, 10, 5, 17)
}

func TestValidateManifestRepeatedViewExpressionDiagnosticPointsToOccurrence(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View: true,
			ViewBody: `<p>{Count}</p>
<button class:active={Count}>Increment</button>`,
			Spans: gwdkir.BlockSpans{
				View: source.SourceSpan{Start: source.SourcePosition{Line: 4, Column: 1}, End: source.SourcePosition{Line: 4, Column: 7}},
			},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected class toggle diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "component_field_error")
	if diagnostic == nil || !strings.Contains(diagnostic.Message, "class toggle") {
		t.Fatalf("Missing class toggle diagnostic: %#v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 6, 23, 6, 28)
}

func TestValidateManifestBadGForDiagnosticPointsToDirectiveValue(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:   "Nested",
		Source: "components/nested.cmp.gwdk",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<ul><li g:for={item of Items}>{item.Name}</li></ul>`,
			Spans: gwdkir.BlockSpans{
				View: source.SourceSpan{Start: source.SourcePosition{Line: 12, Column: 1}, End: source.SourcePosition{Line: 12, Column: 7}},
			},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected Bad g:for diagnostic")
	}
	diagnostic := firstDiagnostic(err.(ValidationErrors), "component_field_error")
	if diagnostic == nil || !strings.Contains(diagnostic.Message, `g:for must use`) {
		t.Fatalf("Missing Bad g:for diagnostic: %#v", err)
	}
	assertSourceSpan(t, diagnostic.Span, 13, 16, 13, 29)
}

func TestValidateManifestAllowsGoishConditionalExpressions(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn ToggleCount() {
  Count = if Open { Count + 1 } else { 0 }
}`,
			View:     true,
			ViewBody: `<section g:if={if Open { Count > 0 } else { false }}><button g:on:click={ToggleCount()}>{Count}</button></section>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected Go-ish conditional expressions to validate, got %v", err)
	}
}

func TestValidateManifestAllowsClientLocalVariables(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Add(step int) {
  let next int = Count + step
  Count = next
}`,
			View:     true,
			ViewBody: `<button g:on:click={Add(2)}>{Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected local variables to validate, got %v", err)
	}
}

func TestValidateManifestRejectsLocalVariableBeforeDeclaration(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Bad() {
  Count = next
  let next int = Count + 1
}`,
			View:     true,
			ViewBody: `<button g:on:click={Bad()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected local-before-declaration diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") || !strings.Contains(err.Error(), "next") {
		t.Fatalf("Missing local-before-declaration diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsGoishConditionalTypeMismatch(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `fn Bad() {
  Count = if Open { Count + 1 } else { "closed" }
}`,
			View:     true,
			ViewBody: `<button g:on:click={Bad()}>{Count}</button>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected conditional branch mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsComputedState(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `computed Label string {
  return if Open { "open" } else { "closed" }
}

computed Visible bool {
  return Label == "open"
}`,
			View:     true,
			ViewBody: `<section g:if={Visible}>{Label}</section>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected computed state to validate, got %v", err)
	}
}

func TestValidateManifestAllowsGoStyleComputedState(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `computed Label string {
  if Count == 0 {
    return "Start"
  }
  return string(Count)
}

func Increment() {
  Count++
}`,
			View:     true,
			ViewBody: `<button g:on:click={Increment()}>{Label}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected Go-style computed state to validate, got %v", err)
	}
}

func TestValidateManifestAllowsComputedOutOfOrderDependencies(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `computed Visible bool {
  return Label == "open"
}

computed Label string {
  return if Open { "open" } else { "closed" }
}`,
			View:     true,
			ViewBody: `<section g:if={Visible}>{Label}</section>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected out-of-order computed state to validate, got %v", err)
	}
}

func TestValidateManifestRejectsComputedCycle(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `computed First string {
  return Second
}

computed Second string {
  return First
}`,
			View:     true,
			ViewBody: `<section>{First}</section>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected computed cycle diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsComputedMutation(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			Client: true,
			ClientBody: `computed Label string {
  Count = Count + 1
}`,
			View:     true,
			ViewBody: `<section>{Count}</section>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected computed mutation diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_client_error") {
		t.Fatalf("Missing component_client_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownNestedField(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Nested",
		Source:  "components/nested.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "NestedState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewNestedState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<section g:if={User.Missing}>{Count}</section>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown nested field diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsValueBindingToStringState(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<input g:bind:value={Query} />`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected string state value binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsValueBindingToNonStringState(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<input g:bind:value={Count} />`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected non-string value binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsNumberInputValueBindingToNumericState(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<input type="number" g:bind:value={Count} />`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected numeric value binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsNumericValueBindingOutsideNumberInput(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<textarea g:bind:value={Count}></textarea>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected numeric value binding target diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsRadioValueBindingToStringState(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<input type="radio" value="initial" g:bind:value={Query} />`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected radio value binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsRadioValueBindingWithoutValue(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<input type="radio" g:bind:value={Query} />`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected radio Missing value diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsValueBindingToProp(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		PropsType: gwdkir.GoRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<input g:bind:value={Label} />`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected prop value binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsCheckedBindingToBoolState(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<input type="checkbox" g:bind:checked={Open} />`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected bool state checked binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsCheckedBindingToNonBoolState(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Search",
		Source:  "components/search.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<input type="checkbox" g:bind:checked={Query} />`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected non-bool checked binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsReactiveAttributes(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button disabled={Open} aria-expanded={Open}>{Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected reactive attributes to validate, got %v", err)
	}
}

func TestValidateManifestRejectsNonBoolReactiveBooleanAttribute(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button disabled={Count}>{Count}</button>`,
			Spans: gwdkir.BlockSpans{
				View: source.SourceSpan{Start: source.SourcePosition{Line: 30, Column: 1}, End: source.SourcePosition{Line: 30, Column: 7}},
			},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected non-bool boolean attr diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
	diagnostic := firstDiagnostic(diagnostics, "component_field_error")
	assertSourceSpan(t, diagnostic.Span, 31, 19, 31, 24)
}

func TestValidateManifestRejectsUnsafeReactiveURLAttribute(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Link",
		Source:  "components/link.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "TextState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewTextState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<a href={Query}>Link</a>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unsafe reactive URL attr diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsClassToggle(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button class:active={Open}>{Count}</button>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected class toggle to validate, got %v", err)
	}
}

func TestValidateManifestRejectsNonBoolClassToggle(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<button class:active={Count}>{Count}</button>`,
			Spans: gwdkir.BlockSpans{
				View: source.SourceSpan{Start: source.SourcePosition{Line: 40, Column: 1}, End: source.SourcePosition{Line: 40, Column: 7}},
			},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected non-bool class toggle diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
	diagnostic := firstDiagnostic(diagnostics, "component_field_error")
	assertSourceSpan(t, diagnostic.Span, 41, 23, 41, 28)
}

func TestValidateManifestAllowsStyleBinding(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<div style:height.px={Count}>{Count}</div>`,
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected style binding to validate, got %v", err)
	}
}

func TestValidateManifestRejectsBoolStyleBinding(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewCounterState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<div style:height.px={Open}>{Count}</div>`,
			Spans: gwdkir.BlockSpans{
				View: source.SourceSpan{Start: source.SourcePosition{Line: 50, Column: 1}, End: source.SourcePosition{Line: 50, Column: 7}},
			},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected bool style binding diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_field_error") {
		t.Fatalf("Missing component_field_error diagnostic: %#v", diagnostics)
	}
	diagnostic := firstDiagnostic(diagnostics, "component_field_error")
	assertSourceSpan(t, diagnostic.Span, 51, 23, 51, 27)
}

func TestValidateManifestRejectsRelativeGoTypedImportPath(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "./ui"}},
		PropsType: gwdkir.GoRef{
			Alias: "ui",
			Name:  "CounterProps",
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected invalid import diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "invalid_go_import") {
		t.Fatalf("Missing invalid_go_import diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsStateInitReturnMismatch(t *testing.T) {
	app := appFixture{Components: []gwdkir.Component{{
		Name:    "Counter",
		Source:  "components/counter.cmp.gwdk",
		Imports: []gwdkir.Import{{Alias: "ui", Path: "github.com/cssbruno/gowdk/testfixture/islands"}},
		State: gwdkir.StateContract{
			Type: gwdkir.GoRef{Alias: "ui", Name: "CounterState"},
			Init: gwdkir.GoRef{Alias: "ui", Name: "NewOtherState"},
		},
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<p>{Count}</p>`,
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected state init mismatch diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "component_contract_error") {
		t.Fatalf("Missing component_contract_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestResolvesLayoutsByID(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			ID:      "dashboard",
			Route:   "/dashboard",
			Layouts: []string{"root", "Missing"},
			Source:  "pages/dashboard.page.gwdk",
			Blocks:  gwdkir.Blocks{View: true},
		}},
		Layouts: []gwdkir.Layout{{
			ID:     "root",
			Source: "layouts/root.layout.gwdk",
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown layout diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_layout_id") {
		t.Fatalf("Missing unknown_layout_id diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAcceptsQualifiedLayoutUse(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Uses:    []gwdkir.Use{{Alias: "chrome", Package: "layouts"}},
			Layouts: []string{"chrome.root"},
			Blocks:  gwdkir.Blocks{View: true},
		}},
		Layouts: []gwdkir.Layout{{
			Package: "layouts",
			ID:      "root",
			Source:  "layouts/root.layout.gwdk",
			Blocks:  gwdkir.Blocks{View: true, ViewBody: "<slot />"},
		}},
	}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected qualified layout reference to validate, got %v", err)
	}
}

func TestValidateManifestRejectsUnqualifiedCrossPackageLayout(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Layouts: []string{"root"},
			Blocks:  gwdkir.Blocks{View: true},
		}},
		Layouts: []gwdkir.Layout{{
			Package: "layouts",
			ID:      "root",
			Source:  "layouts/root.layout.gwdk",
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown layout diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_layout_id") {
		t.Fatalf("Missing unknown_layout_id diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicateLayoutIDs(t *testing.T) {
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{ID: "root", Source: "layouts/root.layout.gwdk"},
			{ID: "root", Source: "layouts/root-copy.layout.gwdk"},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate layout diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "duplicate_layout_id") {
		t.Fatalf("Missing duplicate_layout_id diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsDuplicateLayoutIDsAcrossPackages(t *testing.T) {
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{Package: "pages", ID: "root", Source: "pages/root.layout.gwdk", Blocks: gwdkir.Blocks{View: true, ViewBody: "<slot />"}},
			{Package: "admin", ID: "root", Source: "admin/root.layout.gwdk", Blocks: gwdkir.Blocks{View: true, ViewBody: "<slot />"}},
		},
	}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected package-qualified layout IDs to be unique, got %v", err)
	}
}

func TestValidateManifestRejectsDuplicateLayoutIDsInSamePackage(t *testing.T) {
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{Package: "pages", ID: "root", Source: "pages/root.layout.gwdk"},
			{Package: "pages", ID: "root", Source: "pages/root-copy.layout.gwdk"},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate layout diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "duplicate_layout_id") {
		t.Fatalf("Missing duplicate_layout_id diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsLayoutSelfReference(t *testing.T) {
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{Package: "app", ID: "root", Layouts: []string{"root"}, Source: "app/root.layout.gwdk"},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected layout self-reference diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "layout_self_reference") {
		t.Fatalf("Missing layout_self_reference diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsCyclicLayoutInheritance(t *testing.T) {
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{Package: "app", ID: "a", Layouts: []string{"b"}, Source: "app/a.layout.gwdk"},
			{Package: "app", ID: "b", Layouts: []string{"a"}, Source: "app/b.layout.gwdk"},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected cyclic layout diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "cyclic_layout_reference") {
		t.Fatalf("Missing cyclic_layout_reference diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAcceptsLayoutInheritanceChain(t *testing.T) {
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{Package: "app", ID: "root", Source: "app/root.layout.gwdk", Blocks: gwdkir.Blocks{View: true, ViewBody: "<slot />"}},
			{Package: "app", ID: "docs", Layouts: []string{"root"}, Source: "app/docs.layout.gwdk", Blocks: gwdkir.Blocks{View: true, ViewBody: "<slot />"}},
		},
	}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected a valid layout inheritance chain to validate, got %v", err)
	}
}

func TestValidateManifestRejectsLayoutFileUseAndQualifiedParentLayout(t *testing.T) {
	useSpan := source.SourceSpan{Start: source.SourcePosition{Line: 2, Column: 1}, End: source.SourcePosition{Line: 2, Column: 20}}
	parentSpan := source.SourceSpan{Start: source.SourcePosition{Line: 3, Column: 8}, End: source.SourcePosition{Line: 3, Column: 19}}
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{
				Package:     "pages",
				ID:          "docs",
				Source:      "pages/docs.layout.gwdk",
				Uses:        []gwdkir.Use{{Alias: "chrome", Package: "layouts", Span: useSpan}},
				Layouts:     []string{"chrome.root"},
				LayoutSpans: []source.NamedSpan{{Name: "chrome.root", Span: parentSpan}},
				Blocks:      gwdkir.Blocks{View: true, ViewBody: "<slot />"},
			},
			{
				Package: "layouts",
				ID:      "root",
				Source:  "layouts/root.layout.gwdk",
				Blocks:  gwdkir.Blocks{View: true, ViewBody: "<slot />"},
			},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected layout use diagnostics")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unsupported_gowdk_use_scope") {
		t.Fatalf("Missing unsupported_gowdk_use_scope diagnostic: %#v", diagnostics)
	}
	if !hasDiagnosticCode(diagnostics, "unknown_layout_id") {
		t.Fatalf("Missing unknown_layout_id diagnostic: %#v", diagnostics)
	}
	var sawUseSpan, sawParentSpan bool
	for _, diagnostic := range diagnostics {
		switch diagnostic.Code {
		case "unsupported_gowdk_use_scope":
			sawUseSpan = diagnostic.Span == useSpan
		case "unknown_layout_id":
			sawParentSpan = diagnostic.Span == parentSpan && strings.Contains(diagnostic.Message, "layout files do not support use aliases")
		}
	}
	if !sawUseSpan {
		t.Fatalf("expected unsupported use diagnostic at use span, got %#v", diagnostics)
	}
	if !sawParentSpan {
		t.Fatalf("expected qualified parent diagnostic at parent layout span, got %#v", diagnostics)
	}
}

func TestValidateManifestRejectsUnknownLayoutParent(t *testing.T) {
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{Package: "app", ID: "docs", Layouts: []string{"missing"}, Source: "app/docs.layout.gwdk"},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown layout parent diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_layout_id") {
		t.Fatalf("Missing unknown_layout_id diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestReportsContractReferenceParseErrors(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Source:  "pages/home.page.gwdk",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<form g:command="CreatePatient"></form>`,
			},
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected contract reference parse diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "contract_reference_parse_error") {
		t.Fatalf("Missing contract_reference_parse_error diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsInvalidContractReferenceRoutes(t *testing.T) {
	tests := []struct {
		name     string
		viewBody string
		message  string
	}{
		{
			name:     "external command action",
			viewBody: `<form method="post" action="https://example.com/pay" g:command="patients.CreatePatient"></form>`,
			message:  "must be a local absolute path",
		},
		{
			name:     "dynamic command action",
			viewBody: `<form method="post" action="/patients/{id}" g:command="patients.CreatePatient"></form>`,
			message:  "without query, fragment, or params",
		},
		{
			name:     "trailing slash command action",
			viewBody: `<form method="post" action="/patients/" g:command="patients.CreatePatient"></form>`,
			message:  "clean absolute path",
		},
		{
			name:     "relative command action",
			viewBody: `<form method="post" action="patients" g:command="patients.CreatePatient"></form>`,
			message:  "local absolute path",
		},
		{
			name:     "unsupported command method",
			viewBody: `<form method="get" action="/patients" g:command="patients.CreatePatient"></form>`,
			message:  "command contract routes require POST",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := appFixture{
				Pages: []gwdkir.Page{{
					Package: "pages",
					ID:      "patients",
					Route:   "/patients",
					Source:  "pages/patients.page.gwdk",
					Blocks: gwdkir.Blocks{
						View:     true,
						ViewBody: test.viewBody,
					},
				}},
			}

			err := validateManifest(gowdk.Config{}, app)
			if err == nil {
				t.Fatal("expected invalid contract route diagnostic")
			}
			diagnostics := err.(ValidationErrors)
			if !hasDiagnosticCode(diagnostics, "contract_route_invalid") {
				t.Fatalf("Missing contract_route_invalid diagnostic: %#v", diagnostics)
			}
			if !strings.Contains(diagnostics[0].Message, test.message) {
				t.Fatalf("expected diagnostic message to contain %q, got %q", test.message, diagnostics[0].Message)
			}
			if diagnostics[0].Source != "pages/patients.page.gwdk" || diagnostics[0].Span.Start.Line == 0 {
				t.Fatalf("expected source span on contract route diagnostic, got %#v", diagnostics[0])
			}
		})
	}
}

func TestValidateManifestRejectsDefaultContractRouteOnDynamicPage(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "blog.show",
			Route:   "/blog/{slug}",
			Source:  "pages/blog-show.page.gwdk",
			Blocks: gwdkir.Blocks{
				Paths:    true,
				View:     true,
				ViewBody: `<form g:command="posts.CreateComment"></form>`,
			},
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected invalid dynamic default contract route diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticMessage(diagnostics, "contract_route_invalid", "dynamic page route", "/blog/{slug}") {
		t.Fatalf("Missing dynamic default contract_route_invalid diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDefaultQueryRouteWithDynamicParams(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			Package: "pages",
			ID:      "patients.show",
			Route:   "/patients/{id}",
			Source:  "pages/patients-show.page.gwdk",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<section g:query="patients.GetPatient"></section>`,
			},
		}},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected invalid contract query route diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "contract_route_invalid") {
		t.Fatalf("Missing contract_route_invalid diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(diagnostics[0].Message, "dynamic page route") {
		t.Fatalf("unexpected diagnostic message: %s", diagnostics[0].Message)
	}
}

func TestValidateManifestRejectsLayoutWithoutSlot(t *testing.T) {
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{Package: "app", ID: "root", Source: "app/root.layout.gwdk", Blocks: gwdkir.Blocks{View: true, ViewBody: "<main></main>"}},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected layout slot diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "layout_slot_count") {
		t.Fatalf("Missing layout_slot_count diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsLayoutWithMultipleSlots(t *testing.T) {
	app := appFixture{
		Layouts: []gwdkir.Layout{
			{Package: "app", ID: "root", Source: "app/root.layout.gwdk", Blocks: gwdkir.Blocks{View: true, ViewBody: "<slot />\n<slot />"}},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected layout slot diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "layout_slot_count") {
		t.Fatalf("Missing layout_slot_count diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsDuplicatePageRoutes(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{
			{ID: "blog.post", Route: "/blog/{slug}", Source: "pages/blog-post.page.gwdk", Blocks: gwdkir.Blocks{Paths: true, View: true}},
			{ID: "blog.entry", Route: "/blog/{id}", Source: "pages/blog-entry.page.gwdk", Blocks: gwdkir.Blocks{Paths: true, View: true}},
		},
	}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected duplicate route diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "duplicate_route") {
		t.Fatalf("Missing duplicate_route diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsAmbiguousDynamicPageRoutes(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
	}{
		{
			name:  "literal tail can also be a param",
			left:  "/blog/{category}/{slug}",
			right: "/blog/{slug}/edit",
		},
		{
			name:  "literal head can also be a param",
			left:  "/{section}/settings",
			right: "/admin/{page}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := appFixture{
				Pages: []gwdkir.Page{
					{ID: "left", Route: test.left, Blocks: gwdkir.Blocks{Paths: true, View: true}},
					{ID: "right", Route: test.right, Blocks: gwdkir.Blocks{Paths: true, View: true}},
				},
			}

			err := validateManifest(gowdk.Config{}, app)
			if err == nil {
				t.Fatal("expected ambiguous dynamic route diagnostic")
			}
			diagnostics := err.(ValidationErrors)
			if !hasDiagnosticCode(diagnostics, "ambiguous_dynamic_route") {
				t.Fatalf("Missing ambiguous_dynamic_route diagnostic: %#v", diagnostics)
			}
		})
	}
}

func TestValidateManifestAllowsConcreteRouteBesideDynamicPageRoute(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{
			{ID: "blog.about", Route: "/blog/about", Blocks: gwdkir.Blocks{View: true}},
			{ID: "blog.post", Route: "/blog/{slug}", Blocks: gwdkir.Blocks{Paths: true, View: true}},
		},
	}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected concrete and dynamic routes to be valid, got %v", err)
	}
}

func TestValidateManifestRejectsDynamicFragmentEndpointOverlap(t *testing.T) {
	tests := []struct {
		name      string
		fragment  gwdkir.FragmentEndpoint
		api       gwdkir.API
		expectMsg []string
	}{
		{
			name:     "dynamic fragment shadows concrete fragment",
			fragment: gwdkir.FragmentEndpoint{Name: "Summary", Method: "GET", Route: "/patients/summary/vitals", Target: "#vitals"},
			expectMsg: []string{
				"/patients/summary/vitals",
				"fragment patients.Summary",
				"fragment patients.Vitals",
			},
		},
		{
			name:      "concrete api shadows dynamic fragment",
			api:       gwdkir.API{Name: "Summary", Method: "GET", Route: "/patients/summary/vitals"},
			expectMsg: []string{"/patients/summary/vitals", "api patients.Summary", "fragment patients.Vitals"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := gwdkir.Page{
				ID:     "patients",
				Route:  "/patients",
				Blocks: gwdkir.Blocks{View: true},
			}
			page.Blocks.Fragments = []gwdkir.FragmentEndpoint{{
				Name:   "Vitals",
				Method: "GET",
				Route:  "/patients/{id:int}/vitals",
				Target: "#vitals",
			}}
			if test.fragment.Name != "" {
				page.Blocks.Fragments = append(page.Blocks.Fragments, test.fragment)
			}
			if test.api.Name != "" {
				page.Blocks.APIs = append(page.Blocks.APIs, test.api)
			}

			err := validateManifest(gowdk.Config{}, appFixture{Pages: []gwdkir.Page{page}})
			if err == nil {
				t.Fatal("expected ambiguous dynamic route diagnostic")
			}
			diagnostics := err.(ValidationErrors)
			if !hasDiagnosticMessage(diagnostics, "ambiguous_dynamic_route", test.expectMsg...) {
				t.Fatalf("Missing ambiguous_dynamic_route diagnostic: %#v", diagnostics)
			}
		})
	}
}

func TestValidateManifestRejectsDynamicFragmentContractOverlap(t *testing.T) {
	program := appFixture{Pages: []gwdkir.Page{{
		ID:    "patients",
		Route: "/patients",
		Blocks: gwdkir.Blocks{View: true, Fragments: []gwdkir.FragmentEndpoint{{
			Name:   "Vitals",
			Method: "GET",
			Route:  "/patients/{id:int}/vitals",
			Target: "#vitals",
		}}},
	}}}.program(gowdk.Config{})
	program.ContractRefs = append(program.ContractRefs, gwdkir.ContractReference{
		Kind:      gwdkir.ContractQuery,
		Name:      "patients.GetVitals",
		Method:    "GET",
		Path:      "/patients/42/vitals",
		OwnerKind: gwdkir.SourcePage,
		OwnerID:   "patients",
		Source:    "pages/patients.page.gwdk",
	})

	err := ValidateProgram(gowdk.Config{}, program)
	if err == nil {
		t.Fatal("expected ambiguous dynamic route diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticMessage(diagnostics, "ambiguous_dynamic_route", "/patients/42/vitals", "query contract patients.GetVitals", "fragment patients.Vitals") {
		t.Fatalf("Missing ambiguous_dynamic_route diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsDifferentMethodEndpointBesideDynamicFragment(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:    "patients",
		Route: "/patients",
		Blocks: gwdkir.Blocks{
			View: true,
			Fragments: []gwdkir.FragmentEndpoint{{
				Name:   "Vitals",
				Method: "GET",
				Route:  "/patients/{id:int}/vitals",
				Target: "#vitals",
			}},
			Actions: []gwdkir.Action{{
				Name:   "SaveVitals",
				Method: "POST",
				Route:  "/patients/summary/vitals",
			}},
		},
	}}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected different methods to be valid, got %v", err)
	}
}

func TestValidateManifestRejectsRouteMethodConflicts(t *testing.T) {
	t.Run("multiple actions on one route", func(t *testing.T) {
		app := appFixture{
			Pages: []gwdkir.Page{{
				ID:    "profile",
				Route: "/profile",
				Blocks: gwdkir.Blocks{
					View:    true,
					Actions: []gwdkir.Action{{Name: "save"}, {Name: "updateAvatar"}},
				},
			}},
		}

		err := validateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticCode(diagnostics, "route_method_conflict") {
			t.Fatalf("Missing route_method_conflict diagnostic: %#v", diagnostics)
		}
	})

	t.Run("api default route conflicts with page get", func(t *testing.T) {
		app := appFixture{
			Pages: []gwdkir.Page{{
				ID:    "patients.index",
				Route: "/patients",
				Blocks: gwdkir.Blocks{
					View: true,
					APIs: []gwdkir.API{{Name: "index"}},
				},
			}},
		}

		err := validateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticCode(diagnostics, "route_method_conflict") {
			t.Fatalf("Missing route_method_conflict diagnostic: %#v", diagnostics)
		}
	})

	t.Run("command route conflicts with action", func(t *testing.T) {
		app := appFixture{
			Pages: []gwdkir.Page{{
				ID:    "patients",
				Route: "/patients",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<main><form method="post" action="/patients" g:command="patients.CreatePatient"></form></main>`,
					Actions:  []gwdkir.Action{{Name: "CreatePatient"}},
				},
			}},
		}

		err := validateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected command/action route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticMessage(diagnostics, "route_method_conflict", "POST", "/patients", "command contract patients.CreatePatient", "action patients.CreatePatient") {
			t.Fatalf("Missing command/action route_method_conflict diagnostic: %#v", diagnostics)
		}
	})

	t.Run("default command route conflicts with inherited action route", func(t *testing.T) {
		app := appFixture{
			Pages: []gwdkir.Page{{
				ID:    "patients",
				Route: "/patients",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<main><form g:command="patients.CreatePatient"></form></main>`,
					Actions:  []gwdkir.Action{{Name: "Save"}},
				},
			}},
		}

		err := validateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected default command/action route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticMessage(diagnostics, "route_method_conflict", "POST", "/patients", "command contract patients.CreatePatient", "action patients.Save") {
			t.Fatalf("Missing default command/action route_method_conflict diagnostic: %#v", diagnostics)
		}
	})

	t.Run("duplicate command routes conflict", func(t *testing.T) {
		app := appFixture{
			Pages: []gwdkir.Page{{
				ID:    "patients",
				Route: "/patients",
				Blocks: gwdkir.Blocks{
					View: true,
					ViewBody: `<main>
  <form method="post" action="/patients" g:command="patients.CreatePatient"></form>
  <form method="post" action="/patients" g:command="patients.UpdatePatient"></form>
</main>`,
				},
			}},
		}

		err := validateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected duplicate command route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticMessage(diagnostics, "route_method_conflict", "POST", "/patients", "command contract patients.UpdatePatient", "command contract patients.CreatePatient") {
			t.Fatalf("Missing duplicate command route_method_conflict diagnostic: %#v", diagnostics)
		}
	})

	t.Run("identical command route references are allowed", func(t *testing.T) {
		app := appFixture{
			Pages: []gwdkir.Page{{
				ID:    "patients",
				Route: "/patients",
				Blocks: gwdkir.Blocks{
					View: true,
					ViewBody: `<main>
  <form method="post" action="/patients" g:command="patients.CreatePatient"></form>
  <form method="post" action="/patients" g:command="patients.CreatePatient"></form>
</main>`,
				},
			}},
		}

		if err := validateManifest(gowdk.Config{}, app); err != nil {
			t.Fatalf("expected identical command route references to be valid, got %v", err)
		}
	})

	t.Run("query route conflicts with api", func(t *testing.T) {
		app := appFixture{
			Pages: []gwdkir.Page{{
				ID:    "patients",
				Route: "/patients",
				Blocks: gwdkir.Blocks{
					View:     true,
					ViewBody: `<main><section g:query="patients.ListPatients"></section></main>`,
					APIs:     []gwdkir.API{{Name: "ListPatients"}},
				},
			}},
		}

		err := validateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected query/api route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticMessage(diagnostics, "route_method_conflict", "GET", "/patients", "query contract patients.ListPatients", "api patients.ListPatients") {
			t.Fatalf("Missing query/api route_method_conflict diagnostic: %#v", diagnostics)
		}
	})

	t.Run("duplicate query routes conflict", func(t *testing.T) {
		app := appFixture{
			Pages: []gwdkir.Page{{
				ID:    "patients",
				Route: "/patients",
				Blocks: gwdkir.Blocks{
					View: true,
					ViewBody: `<main>
  <section g:query="patients.ListPatients"></section>
  <section g:query="patients.SearchPatients"></section>
</main>`,
				},
			}},
		}

		err := validateManifest(gowdk.Config{}, app)
		if err == nil {
			t.Fatal("expected duplicate query route method conflict")
		}
		diagnostics := err.(ValidationErrors)
		if !hasDiagnosticMessage(diagnostics, "route_method_conflict", "GET", "/patients", "query contract patients.SearchPatients", "query contract patients.ListPatients") {
			t.Fatalf("Missing duplicate query route_method_conflict diagnostic: %#v", diagnostics)
		}
	})

	t.Run("identical query route references are allowed", func(t *testing.T) {
		app := appFixture{
			Pages: []gwdkir.Page{{
				ID:    "patients",
				Route: "/patients",
				Blocks: gwdkir.Blocks{
					View: true,
					ViewBody: `<main>
  <section g:query="patients.ListPatients"></section>
  <section g:query="patients.ListPatients"></section>
</main>`,
				},
			}},
		}

		if err := validateManifest(gowdk.Config{}, app); err != nil {
			t.Fatalf("expected identical query route references to be valid, got %v", err)
		}
	})
}

func TestValidateManifestAllowsSameRouteWithDifferentMethods(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			ID:    "newsletter",
			Route: "/newsletter",
			Blocks: gwdkir.Blocks{
				View:    true,
				Actions: []gwdkir.Action{{Name: "Subscribe"}},
			},
		}},
	}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected GET page plus POST action to be valid, got %v", err)
	}
}

func TestValidateManifestAllowsPageOwnedQueryOnPageRoute(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{{
			ID:    "patients",
			Route: "/patients",
			Blocks: gwdkir.Blocks{
				View:     true,
				ViewBody: `<main><section g:query="patients.GetPatientPage"></section></main>`,
			},
		}},
	}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected page-owned query to share the page GET route, got %v", err)
	}
}

func TestValidatePageRejectsMalformedRoutes(t *testing.T) {
	tests := []struct {
		name  string
		route string
	}{
		{name: "relative route", route: "patients"},
		{name: "query string", route: "/patients?page=1"},
		{name: "trailing slash", route: "/patients/"},
		{name: "dot segment", route: "/patients/../admin"},
		{name: "embedded param", route: "/blog/{slug}.html"},
		{name: "invalid param name", route: "/blog/{123}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := gwdkir.Page{ID: "patients", Route: test.route, Blocks: gwdkir.Blocks{Paths: true, View: true}}

			diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
			if !hasDiagnosticCode(diagnostics, "malformed_route") {
				t.Fatalf("Missing malformed_route diagnostic for %q: %#v", test.route, diagnostics)
			}
			if hasDiagnosticCode(diagnostics, "spa_dynamic_route_missing_paths") {
				t.Fatalf("malformed route should not cascade into Missing paths: %#v", diagnostics)
			}
		})
	}
}

func TestValidatePageRejectsDuplicateRouteParams(t *testing.T) {
	page := gwdkir.Page{
		ID:     "blog.post",
		Route:  "/blog/{slug}/{slug}",
		Blocks: gwdkir.Blocks{Paths: true, View: true},
		Spans: gwdkir.PageSpans{
			Route: testSourceSpan(3, 8, 3, 28),
			RouteParams: []source.NamedSpan{
				{Name: "slug", Span: testSourceSpan(3, 14, 3, 20)},
				{Name: "slug", Span: testSourceSpan(3, 21, 3, 27)},
			},
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "duplicate_route_param")
	if diagnostic == nil {
		t.Fatalf("Missing duplicate_route_param diagnostic: %#v", diagnostics)
	}
	assertSourceSpan(t, diagnostic.Span, 3, 21, 3, 27)
}

func TestValidatePageRouteDiagnosticsUseExactSpans(t *testing.T) {
	page := gwdkir.Page{
		ID:     "settings",
		Route:  "/settings",
		Blocks: gwdkir.Blocks{View: true},
	}

	t.Run("action malformed param type", func(t *testing.T) {
		page := page
		page.Blocks.Actions = []gwdkir.Action{{
			Name:        "Save",
			Method:      "POST",
			Route:       "/save/{id:uuid}",
			Span:        testSourceSpan(6, 1, 6, 32),
			RouteSpan:   testSourceSpan(6, 17, 6, 34),
			RouteParams: []source.NamedSpan{{Name: "id", Span: testSourceSpan(6, 24, 6, 33)}},
		}}

		diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
		diagnostic := firstDiagnostic(diagnostics, "malformed_route")
		if diagnostic == nil {
			t.Fatalf("Missing malformed_route diagnostic: %#v", diagnostics)
		}
		assertSourceSpan(t, diagnostic.Span, 6, 24, 6, 33)
	})

	t.Run("api malformed param type", func(t *testing.T) {
		page := page
		page.Blocks.APIs = []gwdkir.API{{
			Name:        "Lookup",
			Method:      "GET",
			Route:       "/api/{id:uuid}",
			Span:        testSourceSpan(9, 1, 9, 31),
			RouteSpan:   testSourceSpan(9, 16, 9, 31),
			RouteParams: []source.NamedSpan{{Name: "id", Span: testSourceSpan(9, 21, 9, 30)}},
		}}

		diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
		diagnostic := firstDiagnostic(diagnostics, "malformed_route")
		if diagnostic == nil {
			t.Fatalf("Missing malformed_route diagnostic: %#v", diagnostics)
		}
		assertSourceSpan(t, diagnostic.Span, 9, 21, 9, 30)
	})

	t.Run("fragment malformed param type", func(t *testing.T) {
		page := page
		page.Blocks.Fragments = []gwdkir.FragmentEndpoint{{
			Name:        "Preview",
			Method:      "GET",
			Route:       "/preview/{id:uuid}",
			Span:        testSourceSpan(12, 1, 12, 34),
			RouteSpan:   testSourceSpan(12, 20, 12, 40),
			RouteParams: []source.NamedSpan{{Name: "id", Span: testSourceSpan(12, 29, 12, 38)}},
		}}

		diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
		diagnostic := firstDiagnostic(diagnostics, "malformed_route")
		if diagnostic == nil {
			t.Fatalf("Missing malformed_route diagnostic: %#v", diagnostics)
		}
		assertSourceSpan(t, diagnostic.Span, 12, 29, 12, 38)
	})
}

func TestValidatePageAllowsDynamicFragmentRouteParams(t *testing.T) {
	page := gwdkir.Page{
		ID:     "patients",
		Route:  "/patients",
		Blocks: gwdkir.Blocks{View: true},
	}
	page.Blocks.Fragments = []gwdkir.FragmentEndpoint{{
		Name:        "Vitals",
		Method:      "GET",
		Route:       "/patients/{id:int}/vitals",
		Target:      "#vitals",
		RouteParams: []source.NamedSpan{{Name: "id", Span: testSourceSpan(6, 20, 6, 28)}},
	}}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if hasDiagnosticCode(diagnostics, "malformed_route") {
		t.Fatalf("dynamic fragment route should be valid, got %#v", diagnostics)
	}
}

func TestValidatePageRejectsRevalidateWithoutCache(t *testing.T) {
	page := gwdkir.Page{ID: "home", Route: "/", Revalidate: "60", Blocks: gwdkir.Blocks{View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if !hasDiagnosticCode(diagnostics, "revalidate_requires_cache") {
		t.Fatalf("Missing revalidate_requires_cache diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRejectsDuplicateRevalidatePolicy(t *testing.T) {
	page := gwdkir.Page{
		ID:         "home",
		Route:      "/",
		Cache:      "public, max-age=60, stale-while-revalidate=30",
		Revalidate: "60",
		Blocks:     gwdkir.Blocks{View: true},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if !hasDiagnosticCode(diagnostics, "duplicate_revalidate_policy") {
		t.Fatalf("Missing duplicate_revalidate_policy diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageAllowsTypedRouteParams(t *testing.T) {
	page := gwdkir.Page{ID: "patients.show", Route: "/patients/{id:int}", Blocks: gwdkir.Blocks{Paths: true, View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if hasDiagnosticCode(diagnostics, "malformed_route") {
		t.Fatalf("typed route params should be valid: %#v", diagnostics)
	}
}

func TestValidatePageAllowsRestRouteParamOnSSRPage(t *testing.T) {
	page := gwdkir.Page{
		ID:     "docs.page",
		Route:  "/docs/{path...}",
		Render: gowdk.SSR,
		Blocks: gwdkir.Blocks{View: true, Load: true},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, irPage(page))
	if len(diagnostics) != 0 {
		t.Fatalf("expected rest route param on SSR page to be valid, got %#v", diagnostics)
	}
}

func TestValidatePageRejectsRestRouteParamBeforeLastSegment(t *testing.T) {
	page := gwdkir.Page{ID: "docs.page", Route: "/docs/{path...}/edit", Blocks: gwdkir.Blocks{Paths: true, View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "malformed_route")
	if diagnostic == nil {
		t.Fatalf("Missing malformed_route diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(diagnostic.Message, "rest parameters must be the last segment") {
		t.Fatalf("diagnostic should explain rest params must be last: %s", diagnostic.Message)
	}
}

func TestValidatePageRejectsTypedRestRouteParam(t *testing.T) {
	page := gwdkir.Page{ID: "docs.page", Route: "/docs/{path...:int}", Blocks: gwdkir.Blocks{Paths: true, View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "malformed_route")
	if diagnostic == nil {
		t.Fatalf("Missing malformed_route diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(diagnostic.Message, "rest route parameters are always strings") {
		t.Fatalf("diagnostic should explain rest params are strings: %s", diagnostic.Message)
	}
}

func TestValidatePageRejectsMalformedRestRouteParamVariants(t *testing.T) {
	tests := []struct {
		name    string
		route   string
		message string
	}{
		{name: "missing name", route: "/docs/{...}", message: "declare it as {name...}"},
		{name: "two dots", route: "/docs/{path..}", message: "rest route parameters use exactly three dots"},
		{name: "four dots", route: "/docs/{path....}", message: "invalid route parameter name"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page := gwdkir.Page{ID: "docs.page", Route: test.route, Blocks: gwdkir.Blocks{Paths: true, View: true}}

			diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
			diagnostic := firstDiagnostic(diagnostics, "malformed_route")
			if diagnostic == nil {
				t.Fatalf("Missing malformed_route diagnostic for %q: %#v", test.route, diagnostics)
			}
			if !strings.Contains(diagnostic.Message, test.message) {
				t.Fatalf("diagnostic for %q should contain %q: %s", test.route, test.message, diagnostic.Message)
			}
		})
	}
}

func TestValidatePageRejectsOptionalRouteParams(t *testing.T) {
	page := gwdkir.Page{ID: "docs.page", Route: "/docs/{slug?}", Blocks: gwdkir.Blocks{Paths: true, View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "malformed_route")
	if diagnostic == nil {
		t.Fatalf("Missing malformed_route diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(diagnostic.Message, "optional route parameters are not supported; declare explicit routes for each shape (rest parameters {name...} are supported as the final segment)") {
		t.Fatalf("diagnostic should explain optional params are unsupported: %s", diagnostic.Message)
	}
	if hasDiagnosticCode(diagnostics, "spa_dynamic_route_missing_paths") {
		t.Fatalf("optional param should not cascade into missing paths: %#v", diagnostics)
	}
}

func TestValidatePageRejectsRestRouteParamOnSPAPage(t *testing.T) {
	page := gwdkir.Page{ID: "docs.page", Route: "/docs/{path...}", Render: gowdk.SPA, Blocks: gwdkir.Blocks{Paths: true, View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "malformed_route")
	if diagnostic == nil {
		t.Fatalf("Missing malformed_route diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(diagnostic.Message, "require SSR rendering") {
		t.Fatalf("diagnostic should explain rest params require SSR: %s", diagnostic.Message)
	}
}

func TestValidateManifestRejectsDuplicateRestRoutes(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{
			{ID: "docs.tree", Route: "/docs/{path...}", Render: gowdk.SSR, Source: "pages/docs-tree.page.gwdk", Blocks: gwdkir.Blocks{View: true, Load: true}},
			{ID: "docs.copy", Route: "/docs/{rest...}", Render: gowdk.SSR, Source: "pages/docs-copy.page.gwdk", Blocks: gwdkir.Blocks{View: true, Load: true}},
		},
	}

	err := validateManifest(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err == nil {
		t.Fatal("expected duplicate route diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "duplicate_route") {
		t.Fatalf("Missing duplicate_route diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestRejectsAmbiguousRestRoutes(t *testing.T) {
	tests := []struct {
		name  string
		other string
	}{
		{name: "single param route", other: "/docs/{slug}"},
		{name: "longer concrete route", other: "/docs/guides/intro"},
		{name: "longer dynamic route", other: "/docs/{section}/{slug}"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := appFixture{
				Pages: []gwdkir.Page{
					{ID: "docs.tree", Route: "/docs/{path...}", Render: gowdk.SSR, Source: "pages/docs-tree.page.gwdk", Blocks: gwdkir.Blocks{View: true, Load: true}},
					{ID: "docs.other", Route: test.other, Render: gowdk.SSR, Source: "pages/docs-other.page.gwdk", Blocks: gwdkir.Blocks{View: true, Load: true}},
				},
			}

			err := validateManifest(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
			if err == nil {
				t.Fatal("expected ambiguous dynamic route diagnostic")
			}
			diagnostics := err.(ValidationErrors)
			if !hasDiagnosticCode(diagnostics, "ambiguous_dynamic_route") {
				t.Fatalf("Missing ambiguous_dynamic_route diagnostic: %#v", diagnostics)
			}
		})
	}
}

func TestValidateManifestAllowsRestRouteBesideShorterRoutes(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{
			{ID: "docs.index", Route: "/docs", Render: gowdk.SSR, Source: "pages/docs-index.page.gwdk", Blocks: gwdkir.Blocks{View: true, Load: true}},
			{ID: "docs.tree", Route: "/docs/{path...}", Render: gowdk.SSR, Source: "pages/docs-tree.page.gwdk", Blocks: gwdkir.Blocks{View: true, Load: true}},
			{ID: "blog.post", Route: "/blog/{slug}", Render: gowdk.SSR, Source: "pages/blog-post.page.gwdk", Blocks: gwdkir.Blocks{View: true, Load: true}},
		},
	}

	if err := validateManifest(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app); err != nil {
		t.Fatalf("expected rest route beside non-overlapping routes to be valid, got %v", err)
	}
}

func TestValidatePageRejectsRestRouteParamOnActionAndAPIEndpoints(t *testing.T) {
	page := gwdkir.Page{
		ID:    "docs.page",
		Route: "/docs",
		Blocks: gwdkir.Blocks{
			View:    true,
			Actions: []gwdkir.Action{{Name: "Save", Method: "POST", Route: "/docs/{path...}"}},
			APIs:    []gwdkir.API{{Name: "Lookup", Method: "PUT", Route: "/api/docs/{path...}"}},
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "malformed_route")
	if diagnostic == nil {
		t.Fatalf("Missing malformed_route diagnostic: %#v", diagnostics)
	}
	count := 0
	for _, item := range diagnostics {
		if item.Code == "malformed_route" && strings.Contains(item.Message, "only supported on page routes") {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("expected rest param rejection for action and api endpoints, got %#v", diagnostics)
	}
}

func TestValidatePageRejectsInheritedRestRouteOnAPIEndpoints(t *testing.T) {
	page := gwdkir.Page{
		ID:     "docs.page",
		Route:  "/docs/{path...}",
		Render: gowdk.SSR,
		Blocks: gwdkir.Blocks{
			View: true,
			Load: true,
			APIs: []gwdkir.API{{Name: "Save", Method: "POST"}},
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, irPage(page))
	diagnostic := firstDiagnostic(diagnostics, "malformed_route")
	if diagnostic == nil {
		t.Fatalf("Missing malformed_route diagnostic: %#v", diagnostics)
	}
	if !strings.Contains(diagnostic.Message, "inherits page route") || !strings.Contains(diagnostic.Message, "only supported on page routes") {
		t.Fatalf("diagnostic should explain the API inherits the rest page route: %s", diagnostic.Message)
	}
}

func TestValidateManifestRejectsEndpointInsideRestRouteNamespace(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{
			{ID: "docs.tree", Route: "/docs/{path...}", Render: gowdk.SSR, Source: "pages/docs-tree.page.gwdk", Blocks: gwdkir.Blocks{View: true, Load: true}},
			{
				ID: "docs.lookup", Route: "/lookup", Render: gowdk.SSR, Source: "pages/docs-lookup.page.gwdk",
				Blocks: gwdkir.Blocks{
					View: true,
					Load: true,
					APIs: []gwdkir.API{{Name: "Lookup", Method: "GET", Route: "/docs/guides/intro"}},
				},
			},
		},
	}

	err := validateManifest(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app)
	if err == nil {
		t.Fatal("expected ambiguous dynamic route diagnostic for endpoint inside rest namespace")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "ambiguous_dynamic_route") {
		t.Fatalf("Missing ambiguous_dynamic_route diagnostic: %#v", diagnostics)
	}
}

func TestValidateManifestAllowsDifferentMethodEndpointBesideRestRoute(t *testing.T) {
	app := appFixture{
		Pages: []gwdkir.Page{
			{ID: "docs.tree", Route: "/docs/{path...}", Render: gowdk.SSR, Source: "pages/docs-tree.page.gwdk", Blocks: gwdkir.Blocks{View: true, Load: true}},
			{
				ID: "docs.save", Route: "/save", Render: gowdk.SSR, Source: "pages/docs-save.page.gwdk",
				Blocks: gwdkir.Blocks{
					View: true,
					Load: true,
					APIs: []gwdkir.API{{Name: "Save", Method: "POST", Route: "/docs/guides/intro"}},
				},
			},
		},
	}

	if err := validateManifest(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, app); err != nil {
		t.Fatalf("expected different-method endpoint beside rest route to be valid, got %v", err)
	}
}

func TestValidatePageRequiresPathsForSPADynamicRoutes(t *testing.T) {
	page := gwdkir.Page{ID: "patients.show", Route: "/patients/{id}", Render: gowdk.SPA, Blocks: gwdkir.Blocks{View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Code != "spa_dynamic_route_missing_paths" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
	if !strings.Contains(diagnostics[0].Message, "add paths") {
		t.Fatalf("diagnostic should suggest paths block: %s", diagnostics[0].Message)
	}
}

func TestValidatePageAllowsSPADynamicRoutesWithPaths(t *testing.T) {
	page := gwdkir.Page{ID: "blog.post", Route: "/blog/{slug}", Render: gowdk.SPA, Blocks: gwdkir.Blocks{Paths: true, View: true}}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageAllowsSPAActionsWithoutSSR(t *testing.T) {
	page := gwdkir.Page{
		ID:     "newsletter",
		Route:  "/newsletter",
		Render: gowdk.SPA,
		Blocks: gwdkir.Blocks{
			View:    true,
			Actions: []gwdkir.Action{{Name: "Subscribe"}},
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageRejectsLoadOnSPAPage(t *testing.T) {
	page := gwdkir.Page{
		ID:     "newsletter",
		Route:  "/newsletter",
		Render: gowdk.SPA,
		Blocks: gwdkir.Blocks{
			View: true,
			Load: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %d", len(diagnostics))
	}
	if diagnostics[0].Code != "load_requires_request_render" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
}

func TestValidatePageRequiresSSRAddonForHybridWithoutLoad(t *testing.T) {
	page := gwdkir.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: gwdkir.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %#v", diagnostics)
	}
	if diagnostics[0].Code != "missing_ssr_addon" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
}

func TestValidatePageAllowsDynamicHybridWithoutLoadAsRequestTime(t *testing.T) {
	page := gwdkir.Page{
		ID:     "dashboard",
		Route:  "/dashboard/{id}",
		Render: gowdk.Hybrid,
		Blocks: gwdkir.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, irPage(page))
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageAllowsHybridWithExplicitLoadAndSSRAddon(t *testing.T) {
	page := gwdkir.Page{
		ID:     "dashboard",
		Route:  "/dashboard",
		Render: gowdk.Hybrid,
		Blocks: gwdkir.Blocks{
			Load: true,
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{Addons: []gowdk.Addon{ssr.Addon()}}, irPage(page))
	if len(diagnostics) != 0 {
		t.Fatalf("expected no diagnostics, got %#v", diagnostics)
	}
}

func TestValidatePageRejectsMissingViewBlock(t *testing.T) {
	page := gwdkir.Page{ID: "home", Route: "/", Render: gowdk.SPA}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if len(diagnostics) != 1 {
		t.Fatalf("expected 1 diagnostic, got %#v", diagnostics)
	}
	if diagnostics[0].Code != "missing_view_block" {
		t.Fatalf("unexpected diagnostic code: %s", diagnostics[0].Code)
	}
}

func TestValidateManifestRejectsInvalidScriptGo(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:      "home",
		Package: "pages",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
			GoBlocks: []gwdkir.GoBlock{{
				Body: `func Broken( {`,
			}},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected invalid go block error")
	}
	if !strings.Contains(err.Error(), "invalid Go") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestRejectsSSRScriptWithoutAddon(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:      "home",
		Package: "pages",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
			GoBlocks: []gwdkir.GoBlock{{
				Target: "ssr",
				Body:   `func LoadHome() map[string]any { return nil }`,
			}},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected ssr addon error")
	}
	if !strings.Contains(err.Error(), "request-time page behavior") || !strings.Contains(err.Error(), "SSR addon is not enabled") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestRejectsUnknownAddonGoBlockTarget(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:      "home",
		Package: "pages",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
			GoBlocks: []gwdkir.GoBlock{{
				Target: "addon.contracts",
				Body:   `func RegisterContracts() {}`,
			}},
		},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown addon go block target error")
	}
	if !strings.Contains(err.Error(), `requires an enabled addon named "contracts"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestAllowsKnownAddonGoBlockTarget(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:      "home",
		Package: "pages",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
			GoBlocks: []gwdkir.GoBlock{{
				Target: "addon.contracts",
				Body:   `func RegisterContracts() {}`,
			}},
		},
	}}}

	err := validateManifest(gowdk.Config{Addons: []gowdk.Addon{compilerGoBlockAddon{}}}, app)
	if err != nil {
		t.Fatalf("expected known addon target to validate, got %v", err)
	}
}

func TestValidateManifestRejectsMarkerAddonGoBlockTarget(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:      "home",
		Package: "pages",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
			GoBlocks: []gwdkir.GoBlock{{
				Target: "addon.contracts",
				Body:   `func RegisterContracts() {}`,
			}},
		},
	}}}

	err := validateManifest(gowdk.Config{Addons: []gowdk.Addon{gowdk.NewAddon("contracts", gowdk.FeatureContracts)}}, app)
	if err == nil {
		t.Fatal("expected marker addon go block target error")
	}
	if !strings.Contains(err.Error(), `does not implement gowdk.GoBlockConsumer`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateManifestUsesAddonGoBlockConsumerDiagnostics(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		ID:      "home",
		Package: "pages",
		Route:   "/",
		Blocks: gwdkir.Blocks{
			View:     true,
			ViewBody: `<main>Home</main>`,
			GoBlocks: []gwdkir.GoBlock{{
				Target: "addon.contracts",
				Body:   `func RegisterContracts() {}`,
			}},
		},
	}}}

	err := validateManifest(gowdk.Config{Addons: []gowdk.Addon{compilerGoBlockAddon{diagnostic: "contract go block rejected"}}}, app)
	if err == nil {
		t.Fatal("expected addon go block diagnostic")
	}
	if !strings.Contains(err.Error(), "contract go block rejected") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type compilerGoBlockAddon struct {
	diagnostic string
}

func (addon compilerGoBlockAddon) Name() string {
	return "contracts"
}

func (addon compilerGoBlockAddon) Features() []gowdk.Feature {
	return []gowdk.Feature{gowdk.FeatureContracts}
}

func (addon compilerGoBlockAddon) GoBlockTargets() []string {
	return []string{"addon.contracts"}
}

func (addon compilerGoBlockAddon) ValidateGoBlock(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) []gowdk.GoBlockDiagnostic {
	if addon.diagnostic == "" {
		return nil
	}
	return []gowdk.GoBlockDiagnostic{{Code: "contract_script_rejected", Message: addon.diagnostic}}
}

func (addon compilerGoBlockAddon) GeneratedGo(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) ([]gowdk.GoBlockFile, error) {
	return nil, nil
}

func TestValidateManifestAcceptsQualifiedCSSAssetUse(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{
		{
			Package: "pages",
			ID:      "home",
			Route:   "/",
			Uses:    []gwdkir.Use{{Alias: "theme", Package: "assets"}},
			CSS:     []string{"theme.tokens"},
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main>Home</main>`},
		},
		{
			Package: "assets",
			ID:      "tokens",
			Route:   "/tokens",
			Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main>Tokens</main>`},
		},
	}}

	if err := validateManifest(gowdk.Config{}, app); err != nil {
		t.Fatalf("expected qualified CSS asset use to validate, got %v", err)
	}
}

func TestValidateManifestRejectsUnknownQualifiedCSSAssetUseAlias(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		Package: "pages",
		ID:      "home",
		Route:   "/",
		CSS:     []string{"theme.tokens"},
		Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main>Home</main>`},
	}}}

	err := validateManifest(gowdk.Config{}, app)
	if err == nil {
		t.Fatal("expected unknown CSS asset alias diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "unknown_gowdk_use_alias") {
		t.Fatalf("missing unknown alias diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRejectsInvalidCSSSelection(t *testing.T) {
	page := gwdkir.Page{
		ID:    "embed",
		Route: "/embed",
		CSS:   []string{"none", "forms"},
		Blocks: gwdkir.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if !hasDiagnosticCode(diagnostics, "invalid_css_selection") {
		t.Fatalf("Missing invalid_css_selection diagnostic: %#v", diagnostics)
	}
}

func TestValidatePageRejectsDuplicateCSSSelection(t *testing.T) {
	page := gwdkir.Page{
		ID:    "home",
		Route: "/",
		CSS:   []string{"default", "forms", "forms"},
		Blocks: gwdkir.Blocks{
			View: true,
		},
	}

	diagnostics := ValidatePage(gowdk.Config{}, irPage(page))
	if !hasDiagnosticCode(diagnostics, "duplicate_css_selection") {
		t.Fatalf("Missing duplicate_css_selection diagnostic: %#v", diagnostics)
	}
}

func hasDiagnosticCode(diagnostics []ValidationError, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return true
		}
	}
	return false
}

func firstDiagnostic(diagnostics []ValidationError, code string) *ValidationError {
	for index := range diagnostics {
		if diagnostics[index].Code == code {
			return &diagnostics[index]
		}
	}
	return nil
}

func assertSourceSpan(t *testing.T, span source.SourceSpan, startLine, startColumn, endLine, endColumn int) {
	t.Helper()
	if span.Start.Line != startLine || span.Start.Column != startColumn || span.End.Line != endLine || span.End.Column != endColumn {
		t.Fatalf("unexpected source span: got %#v, want %d:%d-%d:%d", span, startLine, startColumn, endLine, endColumn)
	}
}

func testSourceSpan(startLine, startColumn, endLine, endColumn int) source.SourceSpan {
	return source.SourceSpan{
		Start: source.SourcePosition{Line: startLine, Column: startColumn},
		End:   source.SourcePosition{Line: endLine, Column: endColumn},
	}
}

func hasDiagnosticMessage(diagnostics []ValidationError, code string, parts ...string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != code {
			continue
		}
		matches := true
		for _, part := range parts {
			if !strings.Contains(diagnostic.Message, part) {
				matches = false
				break
			}
		}
		if matches {
			return true
		}
	}
	return false
}

// appFixture mirrors the shape of the old manifest fixtures so the validator
// corpus stays declarative: parsed source records plus optional standalone
// endpoints and binding records, lowered through the production program
// assembly before validation.
type appFixture struct {
	Pages           []gwdkir.Page
	Components      []gwdkir.Component
	Layouts         []gwdkir.Layout
	Endpoints       []gwdkir.GoEndpoint
	BackendBindings []source.BackendBinding
}

func (app appFixture) program(config gowdk.Config) gwdkir.Program {
	pages := append([]gwdkir.Page(nil), app.Pages...)
	for index := range pages {
		pages[index] = publicTestPage(pages[index])
	}
	ir := gwdkanalysis.BuildProgram(config, gwdkanalysis.Sources{
		Pages:      pages,
		Components: app.Components,
		Layouts:    app.Layouts,
	})
	gwdkanalysis.AddStandaloneEndpoints(config, &ir, app.Endpoints)
	gwdkanalysis.AttachBackendBindings(&ir, app.BackendBindings)
	return ir
}

func validateManifest(config gowdk.Config, app appFixture) error {
	return ValidateProgram(config, app.program(config))
}

// irPage lowers a manifest page fixture through the production manifest->IR
// path so page-level validator tests assert against exactly what the build
// pipeline validates.
func irPage(page gwdkir.Page) gwdkir.Page {
	return lowerTestPage(publicTestPage(page))
}

func irGuardlessPage(page gwdkir.Page) gwdkir.Page {
	return lowerTestPage(page)
}

func publicTestPage(page gwdkir.Page) gwdkir.Page {
	if len(page.Guards) == 0 {
		page.Guards = []string{"public"}
	}
	return page
}

func lowerTestPage(page gwdkir.Page) gwdkir.Page {
	program := gwdkanalysis.BuildProgram(gowdk.Config{}, gwdkanalysis.Sources{Pages: []gwdkir.Page{page}})
	if len(program.Pages) != 1 {
		panic(fmt.Sprintf("irPage: expected 1 IR page, got %d", len(program.Pages)))
	}
	return program.Pages[0]
}

func TestValidateSourceProgramSkipsCrossFileUseChecks(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		Package: "pages",
		ID:      "home",
		Route:   "/",
		Uses:    []gwdkir.Use{{Alias: "ui", Package: "components"}},
		Blocks:  gwdkir.Blocks{View: true, ViewBody: `<main><ui.Hero /></main>`},
	}}}

	if err := ValidateSourceProgram(gowdk.Config{}, app.program(gowdk.Config{})); err != nil {
		t.Fatalf("expected single-file program to skip cross-file use checks, got %v", err)
	}
}

func TestValidateSourceProgramKeepsSingleFileUseChecks(t *testing.T) {
	app := appFixture{Pages: []gwdkir.Page{{
		Package: "pages",
		ID:      "home",
		Route:   "/",
		Uses: []gwdkir.Use{
			{Alias: "ui", Package: "components"},
			{Alias: "ui", Package: "widgets"},
		},
		Blocks: gwdkir.Blocks{View: true, ViewBody: `<main><ui.Hero /></main>`},
	}}}

	err := ValidateSourceProgram(gowdk.Config{}, app.program(gowdk.Config{}))
	if err == nil {
		t.Fatal("expected duplicate alias diagnostic")
	}
	diagnostics := err.(ValidationErrors)
	if !hasDiagnosticCode(diagnostics, "duplicate_gowdk_use_alias") {
		t.Fatalf("missing duplicate alias diagnostic: %#v", diagnostics)
	}
}

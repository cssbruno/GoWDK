package gwdkanalysis

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/parser"
)

func TestAnalyzeLowersASTIntoManifestAndIR(t *testing.T) {
	pageAST := mustParse(t, `package pages
@page home
@route "/"
@layout ui.root
@guard auth.required
@css "./home.css"

import auth "github.com/example/app/auth"
use ui "components"
store Session auth.SessionState = auth.NewSessionState()

build {
  => { title: "Home" }
}

act Login POST "/login"
api Session GET "/api/session"

view {
  <main>{title}</main>
}
`)
	if pageAST.Page == nil || pageAST.Page.ID != "home" {
		t.Fatalf("expected typed @page AST, got %#v", pageAST.Page)
	}
	if pageAST.Route == nil || pageAST.Route.Path != "/" {
		t.Fatalf("expected typed @route AST, got %#v", pageAST.Route)
	}
	if len(pageAST.Layouts) != 1 || pageAST.Layouts[0].ID != "ui.root" {
		t.Fatalf("expected typed layout refs, got %#v", pageAST.Layouts)
	}
	if len(pageAST.Guards) != 1 || pageAST.Guards[0].Name != "auth.required" {
		t.Fatalf("expected typed guard refs, got %#v", pageAST.Guards)
	}
	if len(pageAST.CSS) != 1 || pageAST.CSS[0].Path != "./home.css" {
		t.Fatalf("expected typed CSS refs, got %#v", pageAST.CSS)
	}

	result, err := Analyze(gowdk.Config{}, []SourceFile{
		{Path: "pages/home.page.gwdk", Kind: SourcePage, AST: pageAST},
		{Path: "components/hero.cmp.gwdk", Kind: SourceComponent, AST: mustParse(t, `package components
@component Hero
@wasm ./hero/browser

props {
  title string
}

client {
  fn Select() {
    title = title
  }
}

view {
  <section>{title}</section>
}
`)},
		{Path: "components/root.layout.gwdk", Kind: SourceLayout, AST: mustParse(t, `package components
@layout root

view {
  <slot />
}
`)},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Manifest.Pages) != 1 || result.Manifest.Pages[0].ID != "home" {
		t.Fatalf("unexpected manifest pages: %#v", result.Manifest.Pages)
	}
	page := result.Manifest.Pages[0]
	if page.Package != "pages" || page.Route != "/" || len(page.Stores) != 1 {
		t.Fatalf("unexpected lowered page: %#v", page)
	}
	if len(page.Blocks.Actions) != 1 || page.Blocks.Actions[0].Name != "Login" {
		t.Fatalf("unexpected actions: %#v", page.Blocks.Actions)
	}
	if len(page.Blocks.APIs) != 1 || page.Blocks.APIs[0].Name != "Session" {
		t.Fatalf("unexpected APIs: %#v", page.Blocks.APIs)
	}

	if result.IR.Version != gwdkir.Version {
		t.Fatalf("unexpected IR version: %d", result.IR.Version)
	}
	if len(result.IR.Packages) != 2 {
		t.Fatalf("expected two packages, got %#v", result.IR.Packages)
	}
	if len(result.IR.Routes) != 1 || result.IR.Routes[0].Kind != gwdkir.RouteSPA || result.IR.Routes[0].PageID != "home" {
		t.Fatalf("unexpected routes: %#v", result.IR.Routes)
	}
	if len(result.IR.Endpoints) != 2 {
		t.Fatalf("expected action and API endpoints, got %#v", result.IR.Endpoints)
	}
	if result.IR.Endpoints[0].Path != "/api/session" || result.IR.Endpoints[1].Path != "/login" {
		t.Fatalf("expected sorted endpoints, got %#v", result.IR.Endpoints)
	}
	if len(result.IR.Templates) != 3 {
		t.Fatalf("expected page, component, and layout templates, got %#v", result.IR.Templates)
	}
	if len(result.IR.ClientBehaviors) != 1 || result.IR.ClientBehaviors[0].Component != "Hero" {
		t.Fatalf("unexpected client behaviors: %#v", result.IR.ClientBehaviors)
	}
	if len(result.IR.Assets) != 2 {
		t.Fatalf("expected CSS and WASM assets, got %#v", result.IR.Assets)
	}
}

func TestBuildIRAttachesBackendBindings(t *testing.T) {
	ir := BuildIR(gowdk.Config{}, manifest.Manifest{
		Pages: []manifest.Page{{
			ID:      "newsletter",
			Route:   "/newsletter",
			Package: "newsletter",
			Blocks: manifest.Blocks{
				Actions: []manifest.Action{{Name: "Subscribe", Method: "POST", Route: "/newsletter"}},
			},
		}},
		BackendBindings: []manifest.BackendBinding{{
			Kind:         "action",
			PageID:       "newsletter",
			BlockName:    "Subscribe",
			Method:       "POST",
			Route:        "/newsletter",
			Status:       manifest.BackendBindingBound,
			ImportPath:   "example.com/app/newsletter",
			PackageName:  "newsletter",
			FunctionName: "Subscribe",
			Signature:    manifest.BackendSignatureAction0,
		}},
	})

	if len(ir.Endpoints) != 1 {
		t.Fatalf("expected one endpoint, got %#v", ir.Endpoints)
	}
	binding := ir.Endpoints[0].Binding
	if binding.Status != manifest.BackendBindingBound || binding.FunctionName != "Subscribe" {
		t.Fatalf("expected backend binding on IR endpoint, got %#v", binding)
	}
}

func TestBuildIRResolvesQualifiedCSSAssetUse(t *testing.T) {
	ir := BuildIR(gowdk.Config{}, manifest.Manifest{
		Pages: []manifest.Page{{
			ID:      "home",
			Route:   "/",
			Package: "pages",
			Uses:    []manifest.Use{{Alias: "theme", Package: "assets"}},
			CSS:     []string{"theme.tokens"},
			Blocks:  manifest.Blocks{View: true, ViewBody: `<main>Home</main>`},
		}},
	})

	if len(ir.Assets) != 1 {
		t.Fatalf("expected one asset, got %#v", ir.Assets)
	}
	asset := ir.Assets[0]
	if asset.Name != "tokens" || asset.UseAlias != "theme" || asset.UsePackage != "assets" {
		t.Fatalf("expected resolved qualified CSS asset, got %#v", asset)
	}
}

func TestAnalyzeRejectsWrongSourceKind(t *testing.T) {
	_, err := Analyze(gowdk.Config{}, []SourceFile{
		{Path: "pages/home.page.gwdk", Kind: "unknown", AST: mustParse(t, `package pages
@page home
@route "/"

view {
  <main>Home</main>
}
`)},
	})
	if err == nil {
		t.Fatal("expected unsupported source kind error")
	}
}

func mustParse(t *testing.T, source string) parser.SyntaxFile {
	t.Helper()
	file, err := parser.ParseSyntax([]byte(source))
	if err != nil {
		t.Fatal(err)
	}
	return file
}

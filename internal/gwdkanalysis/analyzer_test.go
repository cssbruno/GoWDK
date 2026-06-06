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
@cache "public, max-age=60"
@revalidate 60s
@error "/errors/home.html"
@layout ui.root
@guard auth.required
@css "./home.css"

import auth "github.com/example/app/auth"
use ui "components"
store Session auth.SessionState = auth.NewSessionState()

build {
  => { title: "Home" }
}

act Login POST "/login" @error "/errors/login.html"
api Session GET "/api/session" @error "/errors/session.html"

fragment Feed GET "/fragments/home-feed" "#feed" {
  <section>Feed</section>
}

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
	if pageAST.Cache == nil || pageAST.Cache.Policy != "public, max-age=60" {
		t.Fatalf("expected typed cache policy, got %#v", pageAST.Cache)
	}
	if pageAST.Revalidate == nil || pageAST.Revalidate.Seconds != "60" {
		t.Fatalf("expected typed revalidate policy, got %#v", pageAST.Revalidate)
	}
	if pageAST.ErrorPage == nil || pageAST.ErrorPage.Path != "errors/home.html" {
		t.Fatalf("expected typed error page, got %#v", pageAST.ErrorPage)
	}
	if len(pageAST.CSS) != 1 || pageAST.CSS[0].Path != "./home.css" {
		t.Fatalf("expected typed CSS refs, got %#v", pageAST.CSS)
	}

	result, err := Analyze(gowdk.Config{}, []SourceFile{
		{Path: "pages/home.page.gwdk", Kind: SourcePage, AST: pageAST},
		{Path: "components/hero.cmp.gwdk", Kind: SourceComponent, AST: mustParse(t, `package components
@component Hero
@wasm ./hero/browser
@css "./hero.css"
@asset "./hero.png"

props {
  title string
}

exports {
  selectedID string
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
	if page.Package != "pages" || page.Route != "/" || page.Cache != "public, max-age=60" || page.Revalidate != "60" || page.ErrorPage != "errors/home.html" || len(page.Stores) != 1 {
		t.Fatalf("unexpected lowered page: %#v", page)
	}
	if len(page.Blocks.Actions) != 1 || page.Blocks.Actions[0].Name != "Login" {
		t.Fatalf("unexpected actions: %#v", page.Blocks.Actions)
	}
	if page.Blocks.Actions[0].ErrorPage != "errors/login.html" {
		t.Fatalf("unexpected action error page: %#v", page.Blocks.Actions[0])
	}
	if len(page.Blocks.APIs) != 1 || page.Blocks.APIs[0].Name != "Session" {
		t.Fatalf("unexpected APIs: %#v", page.Blocks.APIs)
	}
	if page.Blocks.APIs[0].ErrorPage != "errors/session.html" {
		t.Fatalf("unexpected API error page: %#v", page.Blocks.APIs[0])
	}
	if len(page.Blocks.Fragments) != 1 || page.Blocks.Fragments[0].Name != "Feed" || page.Blocks.Fragments[0].Target != "#feed" {
		t.Fatalf("unexpected fragments: %#v", page.Blocks.Fragments)
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
	if result.IR.Routes[0].Cache != "public, max-age=60, stale-while-revalidate=60" {
		t.Fatalf("unexpected route cache policy: %#v", result.IR.Routes[0])
	}
	if len(result.IR.Pages) != 1 || result.IR.Pages[0].Revalidate != "60" || result.IR.Pages[0].ErrorPage != "errors/home.html" {
		t.Fatalf("unexpected IR page policies: %#v", result.IR.Pages)
	}
	if len(result.IR.Endpoints) != 3 {
		t.Fatalf("expected action, API, and fragment endpoints, got %#v", result.IR.Endpoints)
	}
	if result.IR.Endpoints[0].Path != "/api/session" || result.IR.Endpoints[1].Path != "/fragments/home-feed" || result.IR.Endpoints[1].Kind != gwdkir.EndpointFragment || result.IR.Endpoints[2].Path != "/login" {
		t.Fatalf("expected sorted endpoints, got %#v", result.IR.Endpoints)
	}
	if result.IR.Endpoints[0].ErrorPage != "errors/session.html" || result.IR.Endpoints[2].ErrorPage != "errors/login.html" {
		t.Fatalf("unexpected endpoint error pages: %#v", result.IR.Endpoints)
	}
	if len(result.IR.Pages[0].Blocks.Fragments) != 1 || result.IR.Pages[0].Blocks.Fragments[0].Body != "<section>Feed</section>" {
		t.Fatalf("expected fragment in IR page blocks, got %#v", result.IR.Pages[0].Blocks.Fragments)
	}
	if len(result.IR.Templates) != 3 {
		t.Fatalf("expected page, component, and layout templates, got %#v", result.IR.Templates)
	}
	if len(result.IR.ClientBehaviors) != 1 || result.IR.ClientBehaviors[0].Component != "Hero" {
		t.Fatalf("unexpected client behaviors: %#v", result.IR.ClientBehaviors)
	}
	if len(result.Manifest.Components) != 1 || len(result.Manifest.Components[0].Exports) != 1 || result.Manifest.Components[0].Exports[0].Name != "selectedID" {
		t.Fatalf("unexpected manifest component exports: %#v", result.Manifest.Components)
	}
	if len(result.IR.Components) != 1 || len(result.IR.Components[0].Exports) != 1 || result.IR.Components[0].Exports[0].Type != "string" {
		t.Fatalf("unexpected IR component exports: %#v", result.IR.Components)
	}
	if len(result.IR.Assets) != 4 {
		t.Fatalf("expected CSS and WASM assets, got %#v", result.IR.Assets)
	}
	var componentCSS, componentAsset bool
	for _, asset := range result.IR.Assets {
		if asset.OwnerID == "Hero" && asset.Kind == gwdkir.AssetCSS && asset.Path == "./hero.css" {
			if asset.HashKey != "component:components:Hero:components/hero.cmp.gwdk:./hero.css" || len(asset.ScopeID) != len("gwdk-000000000000") {
				t.Fatalf("unexpected component CSS scope metadata: %#v", asset)
			}
			componentCSS = true
		}
		if asset.OwnerID == "Hero" && asset.Kind == gwdkir.AssetFile && asset.Path == "./hero.png" {
			componentAsset = true
		}
	}
	if !componentCSS || !componentAsset {
		t.Fatalf("expected component CSS and asset IR entries, got %#v", result.IR.Assets)
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

func TestBuildIRIncludesStandaloneGoEndpointSource(t *testing.T) {
	ir := BuildIR(gowdk.Config{}, manifest.Manifest{
		Endpoints: []manifest.EndpointDeclaration{{
			Kind:       "api",
			SourceKind: manifest.EndpointSourceGo,
			Package:    "api",
			Source:     "api/handlers.go",
			Name:       "Session",
			Method:     "GET",
			Route:      "/api/session",
		}},
		BackendBindings: []manifest.BackendBinding{{
			Kind:         "api",
			PageID:       "api.Session",
			BlockName:    "Session",
			Method:       "GET",
			Route:        "/api/session",
			Status:       manifest.BackendBindingBound,
			ImportPath:   "example.com/app/api",
			PackageName:  "api",
			FunctionName: "Session",
			Signature:    manifest.BackendSignatureAPI,
		}},
	})
	if len(ir.Endpoints) != 1 {
		t.Fatalf("expected one endpoint, got %#v", ir.Endpoints)
	}
	endpoint := ir.Endpoints[0]
	if endpoint.Source != gwdkir.EndpointSourceGo || endpoint.PageID != "api.Session" || endpoint.Binding.Signature != manifest.BackendSignatureAPI {
		t.Fatalf("unexpected endpoint IR: %#v", endpoint)
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

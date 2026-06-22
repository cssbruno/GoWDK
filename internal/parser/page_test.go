package parser

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func TestParsePageReadsSPADynamicRouteWithPathsAndBuild(t *testing.T) {
	page, err := ParsePage([]byte(`
page blog.post
route "/blog/{slug}"
layout root, blog
css default page forms

import interop "github.com/cssbruno/gowdk/examples/go-interop"

paths {
  => { slug: "hello-gowdk" }
  => { slug: "compile-first" }
}

build {
  => { title: "SPA post" }
}

view {
  <main>
    <h1>Post</h1>
  </main>
}
`))
	if err != nil {
		t.Fatal(err)
	}

	if page.ID != "blog.post" {
		t.Fatalf("expected page ID blog.post, got %q", page.ID)
	}
	if page.Route != "/blog/{slug}" {
		t.Fatalf("expected route /blog/{slug}, got %q", page.Route)
	}
	if page.Render != "" {
		t.Fatalf("expected omitted render mode, got %q", page.Render)
	}
	if page.RenderMode(gowdk.SPA) != gowdk.SPA {
		t.Fatalf("expected effective spa render, got %q", page.RenderMode(gowdk.SPA))
	}
	if !page.Blocks.Paths || !page.Blocks.Build || !page.Blocks.View {
		t.Fatalf("expected paths/build/view blocks, got %#v", page)
	}
	if page.Blocks.PathsBody != `=> { slug: "hello-gowdk" }
  => { slug: "compile-first" }` {
		t.Fatalf("unexpected paths body: %q", page.Blocks.PathsBody)
	}
	if page.Blocks.BuildBody != `=> { title: "SPA post" }` {
		t.Fatalf("unexpected build body: %q", page.Blocks.BuildBody)
	}
	if page.Blocks.ViewBody != "<main>\n    <h1>Post</h1>\n  </main>" {
		t.Fatalf("unexpected view body: %q", page.Blocks.ViewBody)
	}
	if len(page.Blocks.PathsRecords) != 2 || page.Blocks.PathsRecords[0].Fields["slug"] != "hello-gowdk" || page.Blocks.PathsRecords[1].Fields["slug"] != "compile-first" {
		t.Fatalf("expected typed paths records, got %#v", page.Blocks.PathsRecords)
	}
	if len(page.Blocks.BuildRecords) != 1 || page.Blocks.BuildRecords[0].Expressions["title"] != `"SPA post"` || page.Blocks.BuildRecords[0].FieldOrder[0] != "title" {
		t.Fatalf("expected typed build record expressions, got %#v", page.Blocks.BuildRecords)
	}
	if len(page.Blocks.ViewNodes) != 1 {
		t.Fatalf("expected parsed view nodes, got %#v", page.Blocks.ViewNodes)
	}
	if page.Layouts[1] != "blog" {
		t.Fatalf("expected blog layout, got %#v", page.Layouts)
	}
	if strings.Join(page.CSS, ",") != "default,page,forms" {
		t.Fatalf("expected css selection, got %#v", page.CSS)
	}
	if len(page.Imports) != 1 || page.Imports[0].Alias != "interop" || page.Imports[0].Path != "github.com/cssbruno/gowdk/examples/go-interop" {
		t.Fatalf("expected page import, got %#v", page.Imports)
	}
	if page.Spans.Route.Start.Line != 3 || page.Spans.Route.Start.Column != 1 {
		t.Fatalf("expected route metadata declaration span, got %#v", page.Spans.Route)
	}
	if len(page.Spans.RouteParams) != 1 || page.Spans.RouteParams[0].Name != "slug" ||
		page.Spans.RouteParams[0].Span.Start.Line != 3 || page.Spans.RouteParams[0].Span.Start.Column != 14 {
		t.Fatalf("expected slug route param span, got %#v", page.Spans.RouteParams)
	}
	if page.Blocks.Spans.Paths.Start.Line != 9 || page.Blocks.Spans.Build.Start.Line != 14 || page.Blocks.Spans.View.Start.Line != 18 {
		t.Fatalf("expected block spans, got %#v", page.Blocks.Spans)
	}
}

func TestParsePageWithDefaultIDAllowsOmittedPageMetadata(t *testing.T) {
	page, err := ParsePageWithDefaultID([]byte(`
package pages

route "/"
guard public

view {
  <main>Home</main>
}
`), "home")
	if err != nil {
		t.Fatal(err)
	}
	if page.ID != "home" {
		t.Fatalf("expected default page ID home, got %q", page.ID)
	}
	if page.Route != "/" {
		t.Fatalf("expected route /, got %q", page.Route)
	}
}

func TestParsePageWithDefaultIDKeepsExplicitPageMetadata(t *testing.T) {
	page, err := ParsePageWithDefaultID([]byte(`
package pages

page marketing.home
route "/"
guard public

view {
  <main>Home</main>
}
`), "home")
	if err != nil {
		t.Fatal(err)
	}
	if page.ID != "marketing.home" {
		t.Fatalf("expected explicit page ID marketing.home, got %q", page.ID)
	}
}

func TestParsePageRejectsOldAtMetadata(t *testing.T) {
	_, err := ParsePage([]byte(`
page home
route "/"
@unknown nope

view {
  <main>Home</main>
}
`))
	if err == nil {
		t.Fatal("expected old metadata syntax to be rejected")
	}
	if !strings.Contains(err.Error(), `malformed legacy metadata "@unknown nope"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageReadsStoreDeclaration(t *testing.T) {
	page, err := ParsePage([]byte(`
page cart
route "/cart"

import ui "github.com/cssbruno/gowdk/testfixture/islands"

store cart ui.CounterState = ui.NewCounterState()

view {
  <main>Cart</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Stores) != 1 {
		t.Fatalf("expected one store, got %#v", page.Stores)
	}
	store := page.Stores[0]
	if store.Name != "cart" || store.Type.Alias != "ui" || store.Type.Name != "CounterState" ||
		store.Init.Alias != "ui" || store.Init.Name != "NewCounterState" {
		t.Fatalf("unexpected store: %#v", store)
	}
	if store.Span.Start.Line != 7 {
		t.Fatalf("unexpected store span: %#v", store.Span)
	}
}

func TestParsePageReadsStorePersistModifier(t *testing.T) {
	for _, tc := range []struct {
		name       string
		line       string
		scope      string
		persistSet bool
	}{
		{name: "local", line: `store cart ui.CounterState = ui.NewCounterState() persist "local"`, scope: "local", persistSet: true},
		{name: "session", line: `store cart ui.CounterState = ui.NewCounterState() persist "session"`, scope: "session", persistSet: true},
		{name: "no-persist", line: `store cart ui.CounterState = ui.NewCounterState()`, scope: "", persistSet: false},
		// An unknown scope still parses into a store so validation can emit a
		// precise diagnostic rather than a generic parse error.
		{name: "invalid-scope", line: `store cart ui.CounterState = ui.NewCounterState() persist "disk"`, scope: "disk", persistSet: true},
		// An explicit empty scope must be distinguishable from no persistence so it
		// is diagnosed rather than silently treated as unpersisted.
		{name: "empty-scope", line: `store cart ui.CounterState = ui.NewCounterState() persist ""`, scope: "", persistSet: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page, err := ParsePage([]byte("\npage cart\nroute \"/cart\"\n\nimport ui \"github.com/cssbruno/gowdk/testfixture/islands\"\n\n" + tc.line + "\n\nview {\n  <main>Cart</main>\n}\n"))
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Stores) != 1 {
				t.Fatalf("expected one store, got %#v", page.Stores)
			}
			if got := page.Stores[0].Persist; got != tc.scope {
				t.Fatalf("store persist = %q, want %q", got, tc.scope)
			}
			if got := page.Stores[0].PersistSet; got != tc.persistSet {
				t.Fatalf("store persistSet = %v, want %v", got, tc.persistSet)
			}
			if page.Stores[0].Name != "cart" || page.Stores[0].Init.Name != "NewCounterState" {
				t.Fatalf("persist modifier corrupted the store: %#v", page.Stores[0])
			}
		})
	}
}

func TestParsePageStorePersistRequiresStringScope(t *testing.T) {
	// `persist` without a string scope is not a valid store line.
	page, err := ParsePage([]byte("\npage cart\nroute \"/cart\"\n\nimport ui \"github.com/cssbruno/gowdk/testfixture/islands\"\n\nstore cart ui.CounterState = ui.NewCounterState() persist\n\nview {\n  <main>Cart</main>\n}\n"))
	if err == nil && len(page.Stores) == 1 {
		t.Fatalf("expected the bare persist keyword to be rejected, got store %#v", page.Stores[0])
	}
}

func TestParsePageReadsStyleBlockOutsideView(t *testing.T) {
	page, err := ParsePage([]byte(`
page styled
route "/styled"

view {
  <main class="hero">Styled</main>
}

style {
  .hero {
    color: red;
  }

  @media (min-width: 40rem) {
    .hero { color: blue; }
  }
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if page.Blocks.ViewBody != `<main class="hero">Styled</main>` {
		t.Fatalf("unexpected view body: %q", page.Blocks.ViewBody)
	}
	if !page.Blocks.Style || !strings.Contains(page.Blocks.StyleBody, ".hero {\n    color: red;\n  }") {
		t.Fatalf("expected style body, got %#v", page.Blocks)
	}
	if strings.Contains(page.Blocks.ViewBody, "style") {
		t.Fatalf("did not expect style block in view body: %q", page.Blocks.ViewBody)
	}
}

func TestParsePageNormalizesTypedRouteParams(t *testing.T) {
	page, err := ParsePage([]byte(`
page patient
route "/patients/{id:int}"
view {
  <h1>Patient</h1>
}
`))
	if err != nil {
		t.Fatalf("ParsePage() error = %v", err)
	}
	if page.Route != "/patients/{id}" {
		t.Fatalf("expected normalized route, got %q", page.Route)
	}
	if len(page.RouteParams) != 1 || page.RouteParams[0].Name != "id" || page.RouteParams[0].Type != "int" {
		t.Fatalf("expected typed route param, got %#v", page.RouteParams)
	}
}

func TestParsePageReadsGOWDKUseDeclaration(t *testing.T) {
	page, err := ParsePage([]byte(`
package pages

page home
route "/"

use ui "components"

view {
  <main><ui.Hero title="GOWDK" /></main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Uses) != 1 {
		t.Fatalf("expected one GOWDK use, got %#v", page.Uses)
	}
	use := page.Uses[0]
	if use.Alias != "ui" || use.Package != "components" {
		t.Fatalf("unexpected GOWDK use: %#v", use)
	}
	if use.Span.Start.Line != 7 {
		t.Fatalf("unexpected use span: %#v", use.Span)
	}
}

func TestParsePageReadsQualifiedLayoutReference(t *testing.T) {
	page, err := ParsePage([]byte(`
package pages

page home
route "/"
layout chrome.root, local

use chrome "layouts"

view {
  <main>Home</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(page.Layouts, ",") != "chrome.root,local" {
		t.Fatalf("unexpected layouts: %#v", page.Layouts)
	}
	if page.Spans.Layouts[0].Name != "chrome.root" || page.Spans.Layouts[0].Span.Start.Line != 6 {
		t.Fatalf("unexpected qualified layout span: %#v", page.Spans.Layouts)
	}
}

func TestParsePageReadsDocumentMetadata(t *testing.T) {
	page, err := ParsePage([]byte(`
page home
route "/"
title "GOWDK - Go-native web apps"
description "Portable .gwdk pages compiled into Go web output."
canonical "https://gowdk.com/"
image "https://gowdk.com/assets/wdk_logo.png"
robots "index,follow"
noindex false
preload "/assets/app.css" as "style"
prefetch "/docs"
jsonld Article

view {
  <main>Home</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if page.Metadata.Title != "GOWDK - Go-native web apps" {
		t.Fatalf("unexpected title metadata: %#v", page.Metadata)
	}
	if page.Metadata.Description != "Portable .gwdk pages compiled into Go web output." {
		t.Fatalf("unexpected description metadata: %#v", page.Metadata)
	}
	if page.Metadata.Canonical != "https://gowdk.com/" {
		t.Fatalf("unexpected canonical metadata: %#v", page.Metadata)
	}
	if page.Metadata.Image != "https://gowdk.com/assets/wdk_logo.png" {
		t.Fatalf("unexpected image metadata: %#v", page.Metadata)
	}
	if page.Metadata.Robots != "index,follow" || page.Metadata.NoIndex {
		t.Fatalf("unexpected robots metadata: %#v", page.Metadata)
	}
	if len(page.Metadata.Preload) != 1 || page.Metadata.Preload[0].Href != "/assets/app.css" || page.Metadata.Preload[0].As != "style" {
		t.Fatalf("unexpected preload metadata: %#v", page.Metadata.Preload)
	}
	if len(page.Metadata.Prefetch) != 1 || page.Metadata.Prefetch[0].Href != "/docs" || page.Metadata.Prefetch[0].As != "" {
		t.Fatalf("unexpected prefetch metadata: %#v", page.Metadata.Prefetch)
	}
	if len(page.Metadata.Structured) != 1 || page.Metadata.Structured[0].Kind != "Article" {
		t.Fatalf("unexpected structured metadata: %#v", page.Metadata.Structured)
	}
	if page.Spans.Title.Start.Line != 4 || page.Spans.Description.Start.Line != 5 || page.Spans.Canonical.Start.Line != 6 || page.Spans.Image.Start.Line != 7 ||
		page.Spans.Robots.Start.Line != 8 || page.Spans.NoIndex.Start.Line != 9 || page.Spans.Preload[0].Span.Start.Line != 10 || page.Spans.Prefetch[0].Span.Start.Line != 11 ||
		page.Spans.Structured[0].Span.Start.Line != 12 {
		t.Fatalf("unexpected metadata spans: %#v", page.Spans)
	}
}

func TestParsePageRejectsMalformedGOWDKUse(t *testing.T) {
	_, err := ParsePage([]byte(`
package pages

page home
route "/"

use ui components

view {
  <main>Home</main>
}
`))
	if err == nil {
		t.Fatal("expected malformed use error")
	}
	if !strings.Contains(err.Error(), `malformed use "use ui components"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageReadsSSRLoadGuardAndActionEndpoint(t *testing.T) {
	page, err := ParsePage([]byte(`
page dashboard
route "/dashboard"
layout root, dashboard
guard auth.required

server {
  user := session.User()
  => { user }
}

act Refresh POST "/dashboard"

view {
}
`))
	if err != nil {
		t.Fatal(err)
	}

	if page.Render != "" {
		t.Fatalf("expected omitted render, got %q", page.Render)
	}
	if page.RenderMode(gowdk.SPA) != gowdk.SSR {
		t.Fatalf("expected effective ssr render, got %q", page.RenderMode(gowdk.SPA))
	}
	if !page.Blocks.Server {
		t.Fatal("expected load block")
	}
	if page.Blocks.ServerBody != "user := session.User()\n  => { user }" {
		t.Fatalf("unexpected load body: %q", page.Blocks.ServerBody)
	}
	if page.Guards[0] != "auth.required" {
		t.Fatalf("expected auth guard, got %#v", page.Guards)
	}
	action := page.Blocks.Actions[0]
	if action.Name != "Refresh" || action.Method != "POST" || action.Route != "/dashboard" {
		t.Fatalf("expected refresh action, got %#v", page.Blocks.Actions)
	}
	if page.Spans.Guard[0].Name != "auth.required" || page.Spans.Guard[0].Span.Start.Line != 5 {
		t.Fatalf("expected guard span, got %#v", page.Spans.Guard)
	}
	if page.Blocks.Actions[0].Span.Start.Line != 12 {
		t.Fatalf("expected action span, got %#v", page.Blocks.Actions[0].Span)
	}
}

func TestParsePageReadsActionEndpointMetadata(t *testing.T) {
	page, err := ParsePage([]byte(`
page newsletter
route "/newsletter"

act Subscribe POST "/newsletter" error "/errors/subscribe.html"

view {
  <main>Newsletter</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Blocks.Actions) != 1 {
		t.Fatalf("expected one action, got %#v", page.Blocks.Actions)
	}
	action := page.Blocks.Actions[0]
	if action.Name != "Subscribe" || action.Method != "POST" || action.Route != "/newsletter" || action.ErrorPage != "errors/subscribe.html" {
		t.Fatalf("unexpected action endpoint metadata: %#v", action)
	}
	if action.Span.Start.Line != 5 || action.RouteSpan.Start.Line != 5 {
		t.Fatalf("expected action spans, got %#v", action)
	}
}

func TestParsePageReadsFragmentEndpointMetadata(t *testing.T) {
	page, err := ParsePage([]byte(`
page patients
route "/patients"

fragment List GET "/patients/list" "#patients" {
  <section>Patients</section>
}

view {
  <main>Patients</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Blocks.Fragments) != 1 {
		t.Fatalf("expected one fragment endpoint, got %#v", page.Blocks.Fragments)
	}
	fragment := page.Blocks.Fragments[0]
	if fragment.Name != "List" || fragment.Method != "GET" || fragment.Route != "/patients/list" || fragment.Target != "#patients" {
		t.Fatalf("unexpected fragment endpoint metadata: %#v", fragment)
	}
	if fragment.Body != "<section>Patients</section>" {
		t.Fatalf("unexpected fragment body: %q", fragment.Body)
	}
	if fragment.Span.Start.Line != 5 || fragment.RouteSpan.Start.Line != 5 || fragment.TargetSpan.Start.Line != 5 {
		t.Fatalf("expected fragment spans, got %#v", fragment)
	}
	if len(page.Blocks.Spans.Fragments) != 1 || page.Blocks.Spans.Fragments[0].Name != "List" {
		t.Fatalf("expected fragment block span, got %#v", page.Blocks.Spans.Fragments)
	}
}

func TestParsePageRejectsOldActionBlockSyntax(t *testing.T) {
	_, err := ParsePage([]byte(`
page patients
route "/patients"

act refresh {
  input := form FilterInput
  fragment "#patients" {
    <section>Updated</section>
  }
  -> "/patients"
}

view {
  <main>Patients</main>
}
`))
	if err == nil {
		t.Fatal("expected old action syntax error")
	}
	if !strings.Contains(err.Error(), "old action block syntax is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComponentReadsEmitsMetadata(t *testing.T) {
	component, err := ParseComponent([]byte(`
component Child

emits {
  select(id string, active bool)
}

view {
  <button>Select</button>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(component.Emits) != 1 {
		t.Fatalf("expected one emit, got %#v", component.Emits)
	}
	event := component.Emits[0]
	if event.Name != "select" || len(event.Params) != 2 {
		t.Fatalf("unexpected emit metadata: %#v", event)
	}
	if event.Params[0].Name != "id" || event.Params[0].Type != "string" || event.Params[1].Name != "active" || event.Params[1].Type != "bool" {
		t.Fatalf("unexpected emit params: %#v", event.Params)
	}
	if component.Blocks.Spans.Emits.Start.Line != 4 {
		t.Fatalf("expected emits span, got %#v", component.Blocks.Spans.Emits)
	}
}

func TestParsePageRejectsLowercaseActionEndpoint(t *testing.T) {
	_, err := ParsePage([]byte(`
page patients
route "/patients"

act refresh POST "/patients"

view {
  <main>Patients</main>
}
`))
	if err == nil {
		t.Fatal("expected invalid handler error")
	}
	if !strings.Contains(err.Error(), `action handler "refresh" must be an exported Go identifier`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseGoldenFixture(t *testing.T) {
	pageSource, err := os.ReadFile(filepath.FromSlash("testdata/golden/product.page.gwdk"))
	if err != nil {
		t.Fatal(err)
	}
	componentSource, err := os.ReadFile(filepath.FromSlash("testdata/golden/product-card.cmp.gwdk"))
	if err != nil {
		t.Fatal(err)
	}

	page, err := ParsePage(pageSource)
	if err != nil {
		t.Fatal(err)
	}
	component, err := ParseComponent(componentSource)
	if err != nil {
		t.Fatal(err)
	}

	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(parserGoldenSummary(page, component)); err != nil {
		t.Fatal(err)
	}
	expected, err := os.ReadFile(filepath.FromSlash("testdata/golden/parse.golden.json"))
	if err != nil {
		t.Fatal(err)
	}

	if strings.TrimSpace(buffer.String()) != strings.TrimSpace(string(expected)) {
		t.Fatalf("parser golden mismatch\nexpected:\n%s\nactual:\n%s", expected, buffer.String())
	}
}

func TestParsePageRejectsUnsupportedActionBodySyntax(t *testing.T) {
	_, err := ParsePage([]byte(`
page newsletter
route "/newsletter"

act subscribe {
  send(input)
}
`))
	if err == nil {
		t.Fatal("expected unsupported action syntax error")
	}
	if !strings.Contains(err.Error(), `old action block syntax is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsMalformedImport(t *testing.T) {
	_, err := ParsePage([]byte(`
page imported
route "/imported"

import interop github.com/cssbruno/gowdk/examples/go-interop

view {
  <main>Imported</main>
}
`))
	if err == nil {
		t.Fatal("expected malformed import error")
	}
	diagnostic, ok := ParserDiagnostic(err)
	if !ok {
		t.Fatalf("expected typed parser diagnostic, got %v", err)
	}
	if diagnostic.Code != DiagnosticMalformedGoImport {
		t.Fatalf("expected %s, got %#v", DiagnosticMalformedGoImport, diagnostic)
	}
	if diagnostic.Span.Start.Line != 5 {
		t.Fatalf("expected malformed import on line 5, got %#v", diagnostic.Span)
	}
}

type parserGolden struct {
	Page      parserGoldenPage      `json:"page"`
	Component parserGoldenComponent `json:"component"`
}

type parserGoldenPage struct {
	Package       string             `json:"package,omitempty"`
	ID            string             `json:"id"`
	Route         string             `json:"route"`
	Render        gowdk.RenderMode   `json:"render"`
	Layouts       []string           `json:"layouts,omitempty"`
	Guard         []string           `json:"guard,omitempty"`
	Paths         bool               `json:"paths"`
	DynamicParams []string           `json:"dynamicParams,omitempty"`
	Blocks        parserGoldenBlocks `json:"blocks"`
}

type parserGoldenBlocks struct {
	PathsBody string               `json:"pathsBody,omitempty"`
	Build     bool                 `json:"build"`
	BuildBody string               `json:"buildBody,omitempty"`
	Load      bool                 `json:"load,omitempty"`
	LoadBody  string               `json:"loadBody,omitempty"`
	View      bool                 `json:"view"`
	ViewBody  string               `json:"viewBody,omitempty"`
	Actions   []parserGoldenAction `json:"actions,omitempty"`
	APIs      []string             `json:"apis,omitempty"`
}

type parserGoldenAction struct {
	Name           string   `json:"name"`
	Method         string   `json:"method,omitempty"`
	Route          string   `json:"route,omitempty"`
	InputName      string   `json:"inputName,omitempty"`
	InputType      string   `json:"inputType,omitempty"`
	ValidatesInput bool     `json:"validatesInput,omitempty"`
	Redirect       string   `json:"redirect,omitempty"`
	Fragments      []string `json:"fragments,omitempty"`
}

type parserGoldenComponent struct {
	Package  string             `json:"package,omitempty"`
	Name     string             `json:"name"`
	Props    []parserGoldenProp `json:"props,omitempty"`
	ViewBody string             `json:"viewBody,omitempty"`
}

type parserGoldenProp struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func parserGoldenSummary(page gwdkir.Page, component gwdkir.Component) parserGolden {
	return parserGolden{
		Page: parserGoldenPage{
			Package:       page.Package,
			ID:            page.ID,
			Route:         page.Route,
			Render:        page.Render,
			Layouts:       page.Layouts,
			Guard:         page.Guards,
			Paths:         page.Blocks.Paths,
			DynamicParams: page.DynamicParams(),
			Blocks: parserGoldenBlocks{
				PathsBody: page.Blocks.PathsBody,
				Build:     page.Blocks.Build,
				BuildBody: page.Blocks.BuildBody,
				Load:      page.Blocks.Server,
				LoadBody:  page.Blocks.ServerBody,
				View:      page.Blocks.View,
				ViewBody:  page.Blocks.ViewBody,
				Actions:   parserGoldenActions(page.Blocks.Actions),
				APIs:      parserGoldenAPIs(page.Blocks.APIs),
			},
		},
		Component: parserGoldenComponent{
			Package:  component.Package,
			Name:     component.Name,
			Props:    parserGoldenProps(component.Props),
			ViewBody: component.Blocks.ViewBody,
		},
	}
}

func parserGoldenActions(actions []gwdkir.Action) []parserGoldenAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]parserGoldenAction, 0, len(actions))
	for _, action := range actions {
		out = append(out, parserGoldenAction{
			Name:           action.Name,
			Method:         action.Method,
			Route:          action.Route,
			InputName:      action.InputName,
			InputType:      action.InputType,
			ValidatesInput: action.ValidatesInput,
			Redirect:       action.Redirect,
			Fragments:      parserGoldenFragments(action.Fragments),
		})
	}
	return out
}

func parserGoldenFragments(fragments []gwdkir.Fragment) []string {
	if len(fragments) == 0 {
		return nil
	}
	out := make([]string, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, fragment.Target)
	}
	return out
}

func parserGoldenAPIs(apis []gwdkir.API) []string {
	if len(apis) == 0 {
		return nil
	}
	out := make([]string, 0, len(apis))
	for _, api := range apis {
		out = append(out, api.Name)
	}
	return out
}

func parserGoldenProps(props []gwdkir.Prop) []parserGoldenProp {
	if len(props) == 0 {
		return nil
	}
	out := make([]parserGoldenProp, 0, len(props))
	for _, prop := range props {
		out = append(out, parserGoldenProp{Name: prop.Name, Type: prop.Type})
	}
	return out
}

func TestParsePageRejectsUnsafeActionRedirect(t *testing.T) {
	_, err := ParsePage([]byte(`
page newsletter
route "/newsletter"

act subscribe {
  input := form SubscribeInput
  -> "https://example.com"
}
`))
	if err == nil {
		t.Fatal("expected unsafe redirect error")
	}
	if !strings.Contains(err.Error(), `old action block syntax is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsActionValidationForDifferentInput(t *testing.T) {
	_, err := ParsePage([]byte(`
page newsletter
route "/newsletter"

act subscribe {
  input := form SubscribeInput
  valid(other)?
}
`))
	if err == nil {
		t.Fatal("expected mismatched validation input error")
	}
	if !strings.Contains(err.Error(), `old action block syntax is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsUnclosedPathsBlock(t *testing.T) {
	_, err := ParsePage([]byte(`
page blog.post
route "/blog/{slug}"

paths {
  => { slug: "hello-gowdk" }
`))
	if err == nil {
		t.Fatal("expected unclosed paths block error")
	}
	if err.Error() != "paths block missing closing }" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsUnclosedBuildBlock(t *testing.T) {
	_, err := ParsePage([]byte(`
page home
route "/"

build {
  => { title: "Home" }
`))
	if err == nil {
		t.Fatal("expected unclosed build block error")
	}
	if err.Error() != "build block missing closing }" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsOldAtPageMetadata(t *testing.T) {
	_, err := ParsePage([]byte(`
page home
route "/"
@mode ssr
`))
	if err == nil {
		t.Fatal("expected old metadata syntax error")
	}
	if !strings.Contains(err.Error(), `malformed legacy metadata "@mode ssr"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageReadsAPIEndpointMetadata(t *testing.T) {
	page, err := ParsePage([]byte(`
page status
route "/status"

api Health GET "/api/health" error "/errors/api-health.html"

view {
  <main>Status</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Blocks.APIs) != 1 {
		t.Fatalf("expected one API, got %#v", page.Blocks.APIs)
	}
	api := page.Blocks.APIs[0]
	if api.Name != "Health" || api.Method != "GET" || api.Route != "/api/health" || api.ErrorPage != "errors/api-health.html" {
		t.Fatalf("unexpected API metadata: %#v", api)
	}
	if api.Span.Start.Line != 5 || api.RouteSpan.Start.Line != 5 {
		t.Fatalf("expected API spans, got api=%#v route=%#v", api.Span, api.RouteSpan)
	}
}

func TestParsePageRejectsUnsafeEndpointErrorPage(t *testing.T) {
	_, err := ParsePage([]byte(`
page newsletter
route "/newsletter"

act Subscribe POST "/newsletter" error "../secret.html"

view {
}
`))
	if err == nil {
		t.Fatal("expected unsafe endpoint error page path error")
	}
	if !strings.Contains(err.Error(), `error path must stay inside generated output`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsUnsupportedAPIBodySyntax(t *testing.T) {
	_, err := ParsePage([]byte(`
page status
route "/status"

api health {
  return JSON({})
}

view {
}
`))
	if err == nil {
		t.Fatal("expected unsupported API body syntax error")
	}
	if !strings.Contains(err.Error(), `old API block syntax is not supported`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageReadsCachePolicy(t *testing.T) {
	page, err := ParsePage([]byte(`
page home
route "/"
cache "public, max-age=60"
revalidate 5m

view {
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if page.Cache != "public, max-age=60" {
		t.Fatalf("unexpected cache policy: %#v", page)
	}
	if page.Revalidate != "300" {
		t.Fatalf("unexpected revalidate policy: %#v", page)
	}
	if page.Spans.Cache.Start.Line != 4 {
		t.Fatalf("unexpected cache span: %#v", page.Spans.Cache)
	}
	if page.Spans.Revalidate.Start.Line != 5 {
		t.Fatalf("unexpected revalidate span: %#v", page.Spans.Revalidate)
	}
}

func TestParsePageReadsErrorPage(t *testing.T) {
	page, err := ParsePage([]byte(`
page dashboard
route "/dashboard"
error "/errors/dashboard.html"

view {
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if page.ErrorPage != "errors/dashboard.html" {
		t.Fatalf("unexpected error page: %#v", page)
	}
	if page.Spans.ErrorPage.Start.Line != 4 {
		t.Fatalf("unexpected error page span: %#v", page.Spans.ErrorPage)
	}
}

func TestParsePageRejectsUnsafeErrorPage(t *testing.T) {
	_, err := ParsePage([]byte(`
page dashboard
route "/dashboard"
error "../secret.html"

view {
}
`))
	if err == nil {
		t.Fatal("expected unsafe error page path error")
	}
	if !strings.Contains(err.Error(), `error path must stay inside generated output`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsMalformedMetadata(t *testing.T) {
	_, err := ParsePage([]byte(`
page home
route "/"
@123

view {
}
`))
	if err == nil {
		t.Fatal("expected malformed legacy metadata error")
	}
	if err.Error() != `line 4: malformed legacy metadata "@123"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsUnsupportedTopLevelBlock(t *testing.T) {
	_, err := ParsePage([]byte(`
page home
route "/"

fragment "#target" {
}

view {
}
`))
	if err == nil {
		t.Fatal("expected unsupported top-level block error")
	}
	if err.Error() != `line 5: unsupported top-level block "fragment"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComponentReadsNamePropsAndViewBody(t *testing.T) {
	component, err := ParseComponent([]byte(`
component Hero

props {
  title string
  tagline string = "Portable"
  count int = 2
  ratio float = 1.5
  active bool = true
}

view {
  <section>
    <h1>{title}</h1>
  </section>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if component.Name != "Hero" {
		t.Fatalf("expected Hero, got %q", component.Name)
	}
	if len(component.Props) != 5 || component.Props[0].Name != "title" || component.Props[1].Default != "Portable" || !component.Props[1].DefaultSet || component.Props[2].Type != "int" || component.Props[2].Default != "2" || component.Props[3].Type != "float" || component.Props[3].Default != "1.5" || component.Props[4].Type != "bool" || component.Props[4].Default != "true" {
		t.Fatalf("unexpected props: %#v", component.Props)
	}
	if component.Blocks.ViewBody != "<section>\n    <h1>{title}</h1>\n  </section>" {
		t.Fatalf("unexpected view body: %q", component.Blocks.ViewBody)
	}
}

func TestParseComponentReadsGoTypedContracts(t *testing.T) {
	component, err := ParseComponent([]byte(`
import ui "github.com/cssbruno/gowdk/testfixture/islands"

component Counter

props ui.CounterProps
state ui.CounterState = ui.NewCounterState()

view {
  <button g:on:click={Count++}>{Count}</button>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(component.Imports) != 1 || component.Imports[0].Alias != "ui" || component.Imports[0].Path != "github.com/cssbruno/gowdk/testfixture/islands" {
		t.Fatalf("unexpected imports: %#v", component.Imports)
	}
	if component.PropsType.Alias != "ui" || component.PropsType.Name != "CounterProps" {
		t.Fatalf("unexpected props contract: %#v", component.PropsType)
	}
	if component.State.Type.Alias != "ui" || component.State.Type.Name != "CounterState" ||
		component.State.Init.Alias != "ui" || component.State.Init.Name != "NewCounterState" {
		t.Fatalf("unexpected state contract: %#v", component.State)
	}
}

func TestParseComponentReadsTypedExports(t *testing.T) {
	component, err := ParseComponent([]byte(`
component Counter

exports {
  selectedID string
  count int
  active bool
}

view {
  <button>{selectedID}</button>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(component.Exports) != 3 {
		t.Fatalf("expected typed exports, got %#v", component.Exports)
	}
	if component.Exports[0].Name != "selectedID" || component.Exports[0].Type != "string" ||
		component.Exports[1].Name != "count" || component.Exports[1].Type != "int" ||
		component.Exports[2].Name != "active" || component.Exports[2].Type != "bool" {
		t.Fatalf("unexpected exports: %#v", component.Exports)
	}
	if component.Blocks.Spans.Exports.Start.Line != 4 {
		t.Fatalf("expected exports span line 4, got %#v", component.Blocks.Spans.Exports)
	}
}

func TestParseComponentReadsScopedCSSAndAssets(t *testing.T) {
	component, err := ParseComponent([]byte(`
component Hero
css "./hero.css"
asset "./hero.png"

view {
  <section>Hero</section>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(component.CSS) != 1 || component.CSS[0] != "./hero.css" {
		t.Fatalf("unexpected component CSS: %#v", component.CSS)
	}
	if len(component.Assets) != 1 || component.Assets[0] != "./hero.png" {
		t.Fatalf("unexpected component assets: %#v", component.Assets)
	}
	if component.Spans.CSS[0].Span.Start.Line != 3 || component.Spans.Assets[0].Span.Start.Line != 4 {
		t.Fatalf("unexpected component asset spans: %#v %#v", component.Spans.CSS, component.Spans.Assets)
	}
}

func TestParseComponentReadsWASMContract(t *testing.T) {
	component, err := ParseComponent([]byte(`
component Counter
wasm ./browser/counter

view {
  <button>{Count}</button>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if component.WASM.Package != "./browser/counter" {
		t.Fatalf("unexpected wasm package: %#v", component.WASM)
	}
	if component.WASM.Span.Start.Line != 3 {
		t.Fatalf("unexpected wasm span: %#v", component.WASM.Span)
	}
}

func TestParseComponentReadsClientBlock(t *testing.T) {
	component, err := ParseComponent([]byte(`
component Counter

client {
  fn Increment() {
    Count++
  }
}

view {
  <button g:on:click={Increment()}>{Count}</button>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if !component.Blocks.Client {
		t.Fatal("expected client block")
	}
	if component.Blocks.ClientBody != "fn Increment() {\n    Count++\n  }" {
		t.Fatalf("unexpected client body: %q", component.Blocks.ClientBody)
	}
	if component.Blocks.Spans.Client.Start.Line != 4 {
		t.Fatalf("expected client span line 4, got %#v", component.Blocks.Spans.Client)
	}
}

func TestParsePageReadsGoBlocks(t *testing.T) {
	page, err := ParsePage([]byte(`
page home
route "/"

go {
func HomePageForBuild() PageCopy {
	return PageCopy{Title: "Home"}
}
}

go server {
func LoadHome() string {
	return "Home"
}
}

view {
  <main>Home</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Blocks.GoBlocks) != 2 {
		t.Fatalf("expected two go blocks, got %#v", page.Blocks.GoBlocks)
	}
	if page.Blocks.GoBlocks[0].Target != "" || !strings.Contains(page.Blocks.GoBlocks[0].Body, "HomePageForBuild") {
		t.Fatalf("unexpected default go block: %#v", page.Blocks.GoBlocks[0])
	}
	if page.Blocks.GoBlocks[1].Target != "server" || !strings.Contains(page.Blocks.GoBlocks[1].Body, "LoadHome") {
		t.Fatalf("unexpected ssr go block: %#v", page.Blocks.GoBlocks[1])
	}
	if len(page.Blocks.Spans.GoBlocks) != 2 || page.Blocks.Spans.GoBlocks[1].Name != "server" {
		t.Fatalf("unexpected go spans: %#v", page.Blocks.Spans.GoBlocks)
	}
}

func TestParseComponentReadsGoBlock(t *testing.T) {
	component, err := ParseComponent([]byte(`
component Counter

go addon.counter {
func RegisterCounter() {}
}

view {
  <button>Count</button>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(component.Blocks.GoBlocks) != 1 || component.Blocks.GoBlocks[0].Target != "addon.counter" {
		t.Fatalf("unexpected component go blocks: %#v", component.Blocks.GoBlocks)
	}
	if !strings.Contains(component.Blocks.GoBlocks[0].Body, "RegisterCounter") {
		t.Fatalf("unexpected component go block body: %q", component.Blocks.GoBlocks[0].Body)
	}
}

func TestParseLayoutReadsGoBlock(t *testing.T) {
	layout, err := ParseLayout("root.layout.gwdk", []byte(`
layout root

go server {
func LayoutData() string {
	return "root"
}
}

view {
  <slot />
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Blocks.GoBlocks) != 1 || layout.Blocks.GoBlocks[0].Target != "server" {
		t.Fatalf("unexpected layout go blocks: %#v", layout.Blocks.GoBlocks)
	}
}

func TestParseComponentRejectsUnsupportedPropType(t *testing.T) {
	_, err := ParseComponent([]byte(`
component Hero

props {
  items []string
}

view {
  <section>Count</section>
}
`))
	if err == nil {
		t.Fatal("expected unsupported prop type error")
	}
}

func TestParseComponentRejectsInvalidPropDefault(t *testing.T) {
	_, err := ParseComponent([]byte(`
component Hero

props {
  count int = true
}

view {
  <section>Count</section>
}
`))
	if err == nil {
		t.Fatal("expected invalid prop default error")
	}
	if !strings.Contains(err.Error(), "default must be an int literal") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComponentRejectsUnknownMetadata(t *testing.T) {
	_, err := ParseComponent([]byte(`
component Hero
route "/"

view {
}
`))
	if err == nil {
		t.Fatal("expected unknown metadata declaration error")
	}
	if err.Error() != `line 3: unsupported metadata route` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComponentRejectsMalformedMetadata(t *testing.T) {
	_, err := ParseComponent([]byte(`
component Hero
@

view {
}
`))
	if err == nil {
		t.Fatal("expected malformed legacy metadata error")
	}
	if err.Error() != `line 3: malformed legacy metadata "@"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComponentRejectsUnsupportedTopLevelBlock(t *testing.T) {
	_, err := ParseComponent([]byte(`
component Hero

server {
}

view {
}
`))
	if err == nil {
		t.Fatal("expected unsupported top-level block error")
	}
	if err.Error() != `line 4: unsupported top-level block "server"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseLayoutReadsIDAndViewBody(t *testing.T) {
	layout, err := ParseLayout("layouts/root.layout.gwdk", []byte(`
view {
  <slot />
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if layout.ID != "root" {
		t.Fatalf("expected identity from file name, got %q", layout.ID)
	}
	if len(layout.Layouts) != 0 {
		t.Fatalf("expected no parent layouts, got %#v", layout.Layouts)
	}
	if !layout.Blocks.View {
		t.Fatal("expected layout view block")
	}
	if layout.Blocks.ViewBody != "<slot />" {
		t.Fatalf("unexpected layout view body: %q", layout.Blocks.ViewBody)
	}
}

func TestParseLayoutTreatsLayoutMetadataAsParent(t *testing.T) {
	layout, err := ParseLayout("layouts/docs.layout.gwdk", []byte(`
layout root

view {
  <slot />
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if layout.ID != "docs" {
		t.Fatalf("expected identity from file name, got %q", layout.ID)
	}
	if len(layout.Layouts) != 1 || layout.Layouts[0] != "root" {
		t.Fatalf("expected layout to declare parent layout root, got %#v", layout.Layouts)
	}
}

func TestParseLayoutReadsErrorPage(t *testing.T) {
	layout, err := ParseLayout("layouts/docs.layout.gwdk", []byte(`
error "/errors/docs.html"

view {
  <slot />
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if layout.ErrorPage != "errors/docs.html" {
		t.Fatalf("expected layout error page, got %q", layout.ErrorPage)
	}
	if layout.ErrorPageSpan.Start.Line != 2 {
		t.Fatalf("unexpected error page span: %#v", layout.ErrorPageSpan)
	}
}

func TestParseLayoutPreservesUseForUnsupportedScopeDiagnostic(t *testing.T) {
	layout, err := ParseLayout("layouts/docs.layout.gwdk", []byte(`
package layouts
use chrome "chrome"
layout root

view {
  <slot />
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(layout.Uses) != 1 || layout.Uses[0].Alias != "chrome" || layout.Uses[0].Package != "chrome" {
		t.Fatalf("expected layout use to be preserved for validation, got %#v", layout.Uses)
	}
	if layout.Uses[0].Span.Start.Line != 3 {
		t.Fatalf("expected layout use span on line 3, got %#v", layout.Uses[0].Span)
	}
}

func TestParseLayoutRejectsPageMetadata(t *testing.T) {
	_, err := ParseLayout("root.layout.gwdk", []byte(`
layout root
page home

view {
}
`))
	if err == nil {
		t.Fatal("expected unsupported metadata error")
	}
	if err.Error() != `line 3: unsupported metadata page` {
		t.Fatalf("unexpected error: %v", err)
	}
}

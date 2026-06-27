package parser

import (
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/gwdkast"
)

func TestParseSyntaxBuildsTypedASTForCurrentSubset(t *testing.T) {
	file, err := ParseSyntax([]byte(`
page newsletter
route "/newsletter/{slug}"
guard auth.required
error "/errors/newsletter.html"
js "./newsletter.js"

js {
  console.log("newsletter")
}

import interop "github.com/cssbruno/gowdk/examples/go-interop"

use ui "components"

paths {
  => { slug: "hello" }
}

build {
  => { title: "Newsletter" }
}

act Subscribe POST "/newsletter/{slug}" error "/errors/subscribe.html"

api Health GET "/api/health" error "/errors/health.html"

fragment List GET "/newsletter/list" "#items" {
  <ul><li>{title}</li></ul>
}

view {
  <main><Panel><h1>{title}</h1></Panel></main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Metadata) != 4 || file.Metadata[1].Name != "route" || file.Metadata[1].Span.Start.Line != 3 {
		t.Fatalf("unexpected metadata: %#v", file.Metadata)
	}
	if file.ErrorPage == nil || file.ErrorPage.Path != "errors/newsletter.html" {
		t.Fatalf("unexpected error page declaration: %#v", file.ErrorPage)
	}
	if len(file.Imports) != 1 || file.Imports[0].Alias != "interop" || file.Imports[0].Path != "github.com/cssbruno/gowdk/examples/go-interop" {
		t.Fatalf("unexpected imports: %#v", file.Imports)
	}
	if len(file.Uses) != 1 || file.Uses[0].Alias != "ui" || file.Uses[0].Package != "components" {
		t.Fatalf("unexpected uses: %#v", file.Uses)
	}
	if len(file.JS) != 2 || file.JS[0].Path != "./newsletter.js" || file.JS[0].Span.Start.Line != 6 {
		t.Fatalf("unexpected scoped JS declarations: %#v", file.JS)
	}
	if file.JS[1].Path != "" || !strings.Contains(file.JS[1].Inline, `console.log("newsletter")`) || file.JS[1].Span.Start.Line != 8 {
		t.Fatalf("unexpected inline JS declaration: %#v", file.JS[1])
	}
	if len(file.Blocks) != 3 {
		t.Fatalf("expected three blocks, got %#v", file.Blocks)
	}
	paths := file.Blocks[0]
	if paths.Kind != "paths" || len(paths.Records) != 1 || paths.Records[0].Fields["slug"] != "hello" {
		t.Fatalf("unexpected paths AST: %#v", paths)
	}
	build := file.Blocks[1]
	if build.Kind != "build" || len(build.Records) != 1 || build.Records[0].Fields["title"] != "Newsletter" {
		t.Fatalf("unexpected build AST: %#v", build)
	}
	if len(file.Actions) != 1 || file.Actions[0].Name != "Subscribe" || file.Actions[0].Method != "POST" || file.Actions[0].Route != "/newsletter/{slug}" || file.Actions[0].ErrorPage != "errors/subscribe.html" {
		t.Fatalf("unexpected action endpoints: %#v", file.Actions)
	}
	if len(file.APIs) != 1 || file.APIs[0].Name != "Health" || file.APIs[0].Method != "GET" || file.APIs[0].Route != "/api/health" || file.APIs[0].ErrorPage != "errors/health.html" {
		t.Fatalf("unexpected api endpoints: %#v", file.APIs)
	}
	if len(file.Fragments) != 1 || file.Fragments[0].Name != "List" || file.Fragments[0].Route != "/newsletter/list" || file.Fragments[0].Target != "#items" {
		t.Fatalf("unexpected fragment endpoints: %#v", file.Fragments)
	}
	if file.Fragments[0].Body != "<ul><li>{title}</li></ul>" {
		t.Fatalf("unexpected fragment body: %q", file.Fragments[0].Body)
	}
	view := file.Blocks[2]
	if view.Kind != "view" || len(view.View) != 1 {
		t.Fatalf("expected parsed view AST, got %#v", view)
	}
}

func TestParseSyntaxReportsBodySyntaxLine(t *testing.T) {
	_, err := ParseSyntax([]byte(`page bad
route "/bad"

build {
  title = "Bad"
}
`))
	if err == nil {
		t.Fatal("expected literal record error")
	}
	if err.Error() != `line 5: unsupported literal record syntax "title = \"Bad\""` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseSyntaxReadsStyleBlockOutsideView(t *testing.T) {
	file := mustParseSyntax(t, []byte(`page styled
route "/styled"

view {
  <main class="hero">Styled</main>
}

style {
  .hero {
    color: red;
  }
}
`))
	if len(file.Blocks) != 2 {
		t.Fatalf("expected view and style blocks, got %#v", file.Blocks)
	}
	viewBlock := file.Blocks[0]
	if viewBlock.Kind != "view" || viewBlock.Body != `<main class="hero">Styled</main>` {
		t.Fatalf("unexpected view block: %#v", viewBlock)
	}
	if len(viewBlock.View) != 1 {
		t.Fatalf("expected parsed view AST, got %#v", viewBlock.View)
	}
	styleBlock := file.Blocks[1]
	if styleBlock.Kind != "style" || styleBlock.StyleBody != ".hero {\n    color: red;\n  }" {
		t.Fatalf("unexpected style block: %#v", styleBlock)
	}
}

func TestParseSyntaxReadsImportedBuildCall(t *testing.T) {
	file, err := ParseSyntax([]byte(`page imported
route "/imported"

import interop "github.com/cssbruno/gowdk/examples/go-interop"

build {
  => interop.FeaturedCopyForBuild()
}

view {
  <main>{title}</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Imports) != 1 || file.Imports[0].Alias != "interop" {
		t.Fatalf("expected import, got %#v", file.Imports)
	}
	if len(file.Blocks) != 2 || file.Blocks[0].Call == nil {
		t.Fatalf("expected build call block, got %#v", file.Blocks)
	}
	if file.Blocks[0].Call.Alias != "interop" || file.Blocks[0].Call.Function != "FeaturedCopyForBuild" {
		t.Fatalf("unexpected build call: %#v", file.Blocks[0].Call)
	}
}

func TestParseSyntaxRejectsDuplicateLiteralRecordField(t *testing.T) {
	_, err := ParseSyntax([]byte(`page duplicate
route "/duplicate"

build {
  => { title: "Home", title: "Again" }
}

view {
  <main>Duplicate</main>
}
`))
	if err == nil {
		t.Fatal("expected duplicate literal record field error")
	}
	if !strings.Contains(err.Error(), `duplicate literal record field "title"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseSyntaxReadsGoBlocks(t *testing.T) {
	file, err := ParseSyntax([]byte(`package pages
page home
route "/"

go {
func HomePageForBuild() PageCopy {
	return PageCopy{Title: "Home"}
}
}

go server {
func LoadDashboard() string {
	return "dashboard"
}
}

view {
  <main>Home</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Blocks) != 3 {
		t.Fatalf("expected go block, go server, and view blocks, got %#v", file.Blocks)
	}
	if file.Blocks[0].Kind != "go" || file.Blocks[0].Name != "" {
		t.Fatalf("unexpected default go block block: %#v", file.Blocks[0])
	}
	if !strings.Contains(file.Blocks[0].Body, "func HomePageForBuild") {
		t.Fatalf("unexpected default go block body: %q", file.Blocks[0].Body)
	}
	if file.Blocks[1].Kind != "go" || file.Blocks[1].Name != "server" {
		t.Fatalf("unexpected targeted go block block: %#v", file.Blocks[1])
	}
	if !strings.Contains(file.Blocks[1].Body, "func LoadDashboard") {
		t.Fatalf("unexpected targeted go block body: %q", file.Blocks[1].Body)
	}
}

func TestParseSyntaxRejectsDuplicateGoBlockTarget(t *testing.T) {
	_, err := ParseSyntax([]byte(`page home
route "/"

go server {
func One() {}
}

go server {
func Two() {}
}

view {
  <main>Home</main>
}
`))
	if err == nil {
		t.Fatal("expected duplicate go target error")
	}
	if !strings.Contains(err.Error(), "duplicate go server block") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseSyntaxGoBuildIsDefaultBuildLane(t *testing.T) {
	file, err := ParseSyntax([]byte(`page home
route "/"

go build {
func HomePageForBuild() string { return "home" }
}

view {
  <main>Home</main>
}
`))
	if err != nil {
		t.Fatal(err)
	}
	var goBlocks []gwdkast.Block
	for _, block := range file.Blocks {
		if block.Kind == "go" {
			goBlocks = append(goBlocks, block)
		}
	}
	if len(goBlocks) != 1 || goBlocks[0].Name != "" {
		t.Fatalf("go build {} should canonicalize to the default (empty) build target, got %#v", goBlocks)
	}
}

func TestParseSyntaxRejectsGoBuildDuplicatingDefault(t *testing.T) {
	_, err := ParseSyntax([]byte(`page home
route "/"

go {
func One() {}
}

go build {
func Two() {}
}

view {
  <main>Home</main>
}
`))
	if err == nil || !strings.Contains(err.Error(), "duplicate go block") {
		t.Fatalf("go {} and go build {} are the same lane and must collide, got %v", err)
	}
}

func TestParseSyntaxReturnsGOWDKAST(t *testing.T) {
	_ = mustParseSyntax(t, []byte(`package ui
component Counter
wasm ./counter/browser
css "./counter.css"
asset "./counter.svg"
js "./counter.js"

import ui "github.com/cssbruno/gowdk/examples/ui"

store Cart ui.CartState = ui.NewCartState()
props ui.CounterProps
state ui.CounterState = ui.NewCounterState()

props {
  title string
  subtitle string
}

exports {
  selectedID string
  count int
}

emits {
  select(id string)
}

client {
  fn Increment() {
    Count++
  }
}

view {
  <button>{title}</button>
}
`))
}

func TestParseSyntaxReadsStoresAndComponentContracts(t *testing.T) {
	file := mustParseSyntax(t, []byte(`package ui
component Counter
wasm ./counter/browser
css "./counter.css"
asset "./counter.svg"
js "./counter.js"

import ui "github.com/cssbruno/gowdk/examples/ui"

store Cart ui.CartState = ui.NewCartState()
props ui.CounterProps
state ui.CounterState = ui.NewCounterState()

props {
  title string
  subtitle string
}

exports {
  selectedID string
  count int
}

emits {
  select(id string)
}

client {
  fn Increment() {
    Count++
  }
}

view {
  <button>{title}</button>
}
`))

	if file.Package == nil || file.Package.Name != "ui" {
		t.Fatalf("unexpected package AST: %#v", file.Package)
	}
	if len(file.Stores) != 1 || file.Stores[0].Name != "Cart" || file.Stores[0].Type.Name != "CartState" || file.Stores[0].Init.Name != "NewCartState" {
		t.Fatalf("unexpected stores: %#v", file.Stores)
	}
	if file.PropsType == nil || file.PropsType.Name != "CounterProps" {
		t.Fatalf("unexpected props type: %#v", file.PropsType)
	}
	if file.State == nil || file.State.Type.Name != "CounterState" || file.State.Init.Name != "NewCounterState" {
		t.Fatalf("unexpected state contract: %#v", file.State)
	}
	if file.WASM == nil || file.WASM.Package != "./counter/browser" {
		t.Fatalf("unexpected wasm contract: %#v", file.WASM)
	}
	if len(file.CSS) != 1 || file.CSS[0].Path != "./counter.css" {
		t.Fatalf("unexpected component CSS assets: %#v", file.CSS)
	}
	if file.CSS[0].Scope.OwnerKind != "component" || file.CSS[0].Scope.OwnerID != "Counter" || file.CSS[0].Scope.Package != "ui" {
		t.Fatalf("unexpected component CSS scope owner: %#v", file.CSS[0].Scope)
	}
	if file.CSS[0].Scope.HashKey != "component:ui:Counter::./counter.css" || len(file.CSS[0].Scope.ScopeID) != len("gwdk-000000000000") {
		t.Fatalf("unexpected component CSS scope hash metadata: %#v", file.CSS[0].Scope)
	}
	if len(file.Assets) != 1 || file.Assets[0].Path != "./counter.svg" {
		t.Fatalf("unexpected component assets: %#v", file.Assets)
	}
	if len(file.JS) != 1 || file.JS[0].Path != "./counter.js" {
		t.Fatalf("unexpected component JS assets: %#v", file.JS)
	}
	props := file.Blocks[0]
	if props.Kind != "props" || len(props.Props) != 2 || props.Props[0].Name != "title" || props.Props[1].Name != "subtitle" {
		t.Fatalf("unexpected props block: %#v", props)
	}
	exports := file.Blocks[1]
	if exports.Kind != "exports" || len(exports.Exports) != 2 || exports.Exports[0].Name != "selectedID" || exports.Exports[1].Type != "int" {
		t.Fatalf("unexpected exports block: %#v", exports)
	}
	emits := file.Blocks[2]
	if emits.Kind != "emits" || len(emits.Emits) != 1 || emits.Emits[0].Name != "select" || emits.Emits[0].Params[0].Name != "id" {
		t.Fatalf("unexpected emits block: %#v", emits)
	}
	client := file.Blocks[3]
	if client.Kind != "client" || client.Body == "" {
		t.Fatalf("unexpected client block: %#v", client)
	}
	view := file.Blocks[4]
	if view.Kind != "view" || len(view.View) != 1 {
		t.Fatalf("unexpected view block: %#v", view)
	}
}

func mustParseSyntax(t *testing.T, source []byte) gwdkast.File {
	t.Helper()
	file, err := ParseSyntax(source)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

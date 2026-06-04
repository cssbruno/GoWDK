package parser

import (
	"testing"

	"github.com/gowdk/gowdk"
)

func TestParsePageReadsStaticDynamicRouteWithPathsAndBuild(t *testing.T) {
	page, err := ParsePage([]byte(`
@page blog.post
@route "/blog/{slug}"
@layout root, blog
@render static

paths {
  => { slug: "hello-gowdk" }
  => { slug: "static-first" }
}

build {
  => { title: "Static post" }
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
	if page.Render != gowdk.Static {
		t.Fatalf("expected static render, got %q", page.Render)
	}
	if !page.Paths || !page.Blocks.Build || !page.Blocks.View {
		t.Fatalf("expected paths/build/view blocks, got %#v", page)
	}
	if page.Blocks.PathsBody != `=> { slug: "hello-gowdk" }
  => { slug: "static-first" }` {
		t.Fatalf("unexpected paths body: %q", page.Blocks.PathsBody)
	}
	if page.Blocks.BuildBody != `=> { title: "Static post" }` {
		t.Fatalf("unexpected build body: %q", page.Blocks.BuildBody)
	}
	if page.Blocks.ViewBody != "<main>\n    <h1>Post</h1>\n  </main>" {
		t.Fatalf("unexpected view body: %q", page.Blocks.ViewBody)
	}
	if page.Layouts[1] != "blog" {
		t.Fatalf("expected blog layout, got %#v", page.Layouts)
	}
}

func TestParsePageReadsSSRLoadGuardAndAction(t *testing.T) {
	page, err := ParsePage([]byte(`
@page dashboard
@route "/dashboard"
@layout root, dashboard
@render ssr
@guard auth.required

load {
}

act refresh {
}

view {
}
`))
	if err != nil {
		t.Fatal(err)
	}

	if page.Render != gowdk.SSR {
		t.Fatalf("expected ssr render, got %q", page.Render)
	}
	if !page.Blocks.Load {
		t.Fatal("expected load block")
	}
	if page.Guard[0] != "auth.required" {
		t.Fatalf("expected auth guard, got %#v", page.Guard)
	}
	if page.Blocks.Actions[0].Name != "refresh" {
		t.Fatalf("expected refresh action, got %#v", page.Blocks.Actions)
	}
}

func TestParsePageReadsActionBodyMetadata(t *testing.T) {
	page, err := ParsePage([]byte(`
@page newsletter
@route "/newsletter"

act subscribe {
  input := form SubscribeInput
  valid(input)?
  -> "/newsletter?ok=1"
}

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
	if action.Name != "subscribe" || action.InputName != "input" || action.InputType != "SubscribeInput" {
		t.Fatalf("unexpected action input metadata: %#v", action)
	}
	if !action.ValidatesInput {
		t.Fatalf("expected action validation metadata: %#v", action)
	}
	if action.Redirect != "/newsletter?ok=1" {
		t.Fatalf("unexpected action redirect: %#v", action)
	}
}

func TestParsePageRejectsUnsupportedActionBodySyntax(t *testing.T) {
	_, err := ParsePage([]byte(`
@page newsletter
@route "/newsletter"

act subscribe {
  send(input)
}
`))
	if err == nil {
		t.Fatal("expected unsupported action syntax error")
	}
	if err.Error() != `line 7: action subscribe line 1 has unsupported syntax "send(input)"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsUnsafeActionRedirect(t *testing.T) {
	_, err := ParsePage([]byte(`
@page newsletter
@route "/newsletter"

act subscribe {
  input := form SubscribeInput
  -> "https://example.com"
}
`))
	if err == nil {
		t.Fatal("expected unsafe redirect error")
	}
	if err.Error() != `line 8: action subscribe line 2: redirect "https://example.com" must be a local absolute path` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsActionValidationForDifferentInput(t *testing.T) {
	_, err := ParsePage([]byte(`
@page newsletter
@route "/newsletter"

act subscribe {
  input := form SubscribeInput
  valid(other)?
}
`))
	if err == nil {
		t.Fatal("expected mismatched validation input error")
	}
	if err.Error() != `line 8: action subscribe line 2 validates "other" but input is "input"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsUnclosedPathsBlock(t *testing.T) {
	_, err := ParsePage([]byte(`
@page blog.post
@route "/blog/{slug}"

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
@page home
@route "/"

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

func TestParsePageRejectsUnknownRenderMode(t *testing.T) {
	_, err := ParsePage([]byte(`
@page home
@route "/"
@render server-first
`))
	if err == nil {
		t.Fatal("expected render mode error")
	}
}

func TestParseComponentReadsNamePropsAndViewBody(t *testing.T) {
	component, err := ParseComponent([]byte(`
@component Hero

props {
  title string
  tagline string
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
	if len(component.Props) != 2 || component.Props[0].Name != "title" || component.Props[1].Type != "string" {
		t.Fatalf("unexpected props: %#v", component.Props)
	}
	if component.Blocks.ViewBody != "<section>\n    <h1>{title}</h1>\n  </section>" {
		t.Fatalf("unexpected view body: %q", component.Blocks.ViewBody)
	}
}

func TestParseComponentRejectsUnsupportedPropType(t *testing.T) {
	_, err := ParseComponent([]byte(`
@component Hero

props {
  count int
}

view {
  <section>Count</section>
}
`))
	if err == nil {
		t.Fatal("expected unsupported prop type error")
	}
}

package parser

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/internal/manifest"
)

func TestParsePageReadsStaticDynamicRouteWithPathsAndBuild(t *testing.T) {
	page, err := ParsePage([]byte(`
@page blog.post
@route "/blog/{slug}"
@layout root, blog
@render static
@css default page forms

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
	if strings.Join(page.CSS, ",") != "default,page,forms" {
		t.Fatalf("expected css selection, got %#v", page.CSS)
	}
	if page.Spans.Route.Start.Line != 3 || page.Spans.Route.Start.Column != 1 {
		t.Fatalf("expected route annotation span, got %#v", page.Spans.Route)
	}
	if len(page.Spans.RouteParams) != 1 || page.Spans.RouteParams[0].Name != "slug" ||
		page.Spans.RouteParams[0].Span.Start.Line != 3 || page.Spans.RouteParams[0].Span.Start.Column != 15 {
		t.Fatalf("expected slug route param span, got %#v", page.Spans.RouteParams)
	}
	if page.Blocks.Spans.Paths.Start.Line != 8 || page.Blocks.Spans.Build.Start.Line != 13 || page.Blocks.Spans.View.Start.Line != 17 {
		t.Fatalf("expected block spans, got %#v", page.Blocks.Spans)
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
  user := session.User()
  => { user }
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
	if page.Blocks.LoadBody != "user := session.User()\n  => { user }" {
		t.Fatalf("unexpected load body: %q", page.Blocks.LoadBody)
	}
	if page.Guard[0] != "auth.required" {
		t.Fatalf("expected auth guard, got %#v", page.Guard)
	}
	if page.Blocks.Actions[0].Name != "refresh" {
		t.Fatalf("expected refresh action, got %#v", page.Blocks.Actions)
	}
	if page.Spans.Guard[0].Name != "auth.required" || page.Spans.Guard[0].Span.Start.Line != 6 {
		t.Fatalf("expected guard span, got %#v", page.Spans.Guard)
	}
	if page.Blocks.Actions[0].Span.Start.Line != 13 {
		t.Fatalf("expected action span, got %#v", page.Blocks.Actions[0].Span)
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
	if action.InputSpan.Start.Line != 6 || action.ValidationSpan.Start.Line != 7 || action.RedirectSpan.Start.Line != 8 {
		t.Fatalf("expected action body spans, got input=%#v validation=%#v redirect=%#v", action.InputSpan, action.ValidationSpan, action.RedirectSpan)
	}
}

func TestParsePageReadsActionFragmentMetadata(t *testing.T) {
	page, err := ParsePage([]byte(`
@page patients
@route "/patients"

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
	if err != nil {
		t.Fatal(err)
	}
	action := page.Blocks.Actions[0]
	if len(action.Fragments) != 1 {
		t.Fatalf("expected one fragment, got %#v", action.Fragments)
	}
	if action.Fragments[0].Target != "#patients" {
		t.Fatalf("unexpected fragment target: %#v", action.Fragments[0])
	}
	if action.Fragments[0].Body != "<section>Updated</section>" {
		t.Fatalf("unexpected fragment body: %q", action.Fragments[0].Body)
	}
	if action.Redirect != "/patients" {
		t.Fatalf("expected redirect after fragment, got %#v", action)
	}
}

func TestParsePageRejectsInvalidActionFragmentTarget(t *testing.T) {
	_, err := ParsePage([]byte(`
@page patients
@route "/patients"

act refresh {
  fragment "patients" {
    <section>Updated</section>
  }
}

view {
  <main>Patients</main>
}
`))
	if err == nil {
		t.Fatal("expected invalid fragment target error")
	}
	if !strings.Contains(err.Error(), `fragment target "patients" must be a static id selector`) {
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

type parserGolden struct {
	Page      parserGoldenPage      `json:"page"`
	Component parserGoldenComponent `json:"component"`
}

type parserGoldenPage struct {
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
	InputName      string   `json:"inputName,omitempty"`
	InputType      string   `json:"inputType,omitempty"`
	ValidatesInput bool     `json:"validatesInput,omitempty"`
	Redirect       string   `json:"redirect,omitempty"`
	Fragments      []string `json:"fragments,omitempty"`
}

type parserGoldenComponent struct {
	Name     string             `json:"name"`
	Props    []parserGoldenProp `json:"props,omitempty"`
	ViewBody string             `json:"viewBody,omitempty"`
}

type parserGoldenProp struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func parserGoldenSummary(page manifest.Page, component manifest.Component) parserGolden {
	return parserGolden{
		Page: parserGoldenPage{
			ID:            page.ID,
			Route:         page.Route,
			Render:        page.Render,
			Layouts:       page.Layouts,
			Guard:         page.Guard,
			Paths:         page.Paths,
			DynamicParams: page.DynamicParams(),
			Blocks: parserGoldenBlocks{
				PathsBody: page.Blocks.PathsBody,
				Build:     page.Blocks.Build,
				BuildBody: page.Blocks.BuildBody,
				Load:      page.Blocks.Load,
				LoadBody:  page.Blocks.LoadBody,
				View:      page.Blocks.View,
				ViewBody:  page.Blocks.ViewBody,
				Actions:   parserGoldenActions(page.Blocks.Actions),
				APIs:      parserGoldenAPIs(page.Blocks.APIs),
			},
		},
		Component: parserGoldenComponent{
			Name:     component.Name,
			Props:    parserGoldenProps(component.Props),
			ViewBody: component.Blocks.ViewBody,
		},
	}
}

func parserGoldenActions(actions []manifest.Action) []parserGoldenAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]parserGoldenAction, 0, len(actions))
	for _, action := range actions {
		out = append(out, parserGoldenAction{
			Name:           action.Name,
			InputName:      action.InputName,
			InputType:      action.InputType,
			ValidatesInput: action.ValidatesInput,
			Redirect:       action.Redirect,
			Fragments:      parserGoldenFragments(action.Fragments),
		})
	}
	return out
}

func parserGoldenFragments(fragments []manifest.Fragment) []string {
	if len(fragments) == 0 {
		return nil
	}
	out := make([]string, 0, len(fragments))
	for _, fragment := range fragments {
		out = append(out, fragment.Target)
	}
	return out
}

func parserGoldenAPIs(apis []manifest.API) []string {
	if len(apis) == 0 {
		return nil
	}
	out := make([]string, 0, len(apis))
	for _, api := range apis {
		out = append(out, api.Name)
	}
	return out
}

func parserGoldenProps(props []manifest.Prop) []parserGoldenProp {
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

func TestParsePageReadsAPIRouteMetadata(t *testing.T) {
	page, err := ParsePage([]byte(`
@page status
@route "/status"

api health {
  GET "/api/health"
}

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
	if api.Name != "health" || api.Method != "GET" || api.Route != "/api/health" {
		t.Fatalf("unexpected API metadata: %#v", api)
	}
	if api.Span.Start.Line != 5 || api.RouteSpan.Start.Line != 6 {
		t.Fatalf("expected API spans, got api=%#v route=%#v", api.Span, api.RouteSpan)
	}
}

func TestParsePageRejectsUnsupportedAPIBodySyntax(t *testing.T) {
	_, err := ParsePage([]byte(`
@page status
@route "/status"

api health {
  return JSON({})
}

view {
}
`))
	if err == nil {
		t.Fatal("expected unsupported API body syntax error")
	}
	if err.Error() != `line 7: api health line 1 has unsupported syntax "return JSON({})"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsUnknownAnnotation(t *testing.T) {
	_, err := ParsePage([]byte(`
@page home
@route "/"
@cache public

view {
}
`))
	if err == nil {
		t.Fatal("expected unknown annotation error")
	}
	if err.Error() != `line 4: unsupported annotation @cache` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsMalformedAnnotation(t *testing.T) {
	_, err := ParsePage([]byte(`
@page home
@route "/"
@123

view {
}
`))
	if err == nil {
		t.Fatal("expected malformed annotation error")
	}
	if err.Error() != `line 4: malformed annotation "@123"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParsePageRejectsUnsupportedTopLevelBlock(t *testing.T) {
	_, err := ParsePage([]byte(`
@page home
@route "/"

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

func TestParseComponentRejectsUnknownAnnotation(t *testing.T) {
	_, err := ParseComponent([]byte(`
@component Hero
@route "/"

view {
}
`))
	if err == nil {
		t.Fatal("expected unknown annotation error")
	}
	if err.Error() != `line 3: unsupported annotation @route` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComponentRejectsMalformedAnnotation(t *testing.T) {
	_, err := ParseComponent([]byte(`
@component Hero
@

view {
}
`))
	if err == nil {
		t.Fatal("expected malformed annotation error")
	}
	if err.Error() != `line 3: malformed annotation "@"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseComponentRejectsUnsupportedTopLevelBlock(t *testing.T) {
	_, err := ParseComponent([]byte(`
@component Hero

load {
}

view {
}
`))
	if err == nil {
		t.Fatal("expected unsupported top-level block error")
	}
	if err.Error() != `line 4: unsupported top-level block "load"` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseLayoutReadsIDAndViewBody(t *testing.T) {
	layout, err := ParseLayout([]byte(`
@layout root

view {
  <slot />
}
`))
	if err != nil {
		t.Fatal(err)
	}
	if layout.ID != "root" {
		t.Fatalf("expected root layout, got %q", layout.ID)
	}
	if !layout.Blocks.View {
		t.Fatal("expected layout view block")
	}
	if layout.Blocks.ViewBody != "<slot />" {
		t.Fatalf("unexpected layout view body: %q", layout.Blocks.ViewBody)
	}
}

func TestParseLayoutRejectsPageAnnotation(t *testing.T) {
	_, err := ParseLayout([]byte(`
@layout root
@page home

view {
}
`))
	if err == nil {
		t.Fatal("expected unsupported annotation error")
	}
	if err.Error() != `line 3: unsupported annotation @page` {
		t.Fatalf("unexpected error: %v", err)
	}
}

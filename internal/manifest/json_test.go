package manifest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/gowdk/gowdk"
)

func TestManifestJSONIncludesRenderModeAndPaths(t *testing.T) {
	app := Manifest{
		Pages: []Page{
			{
				Source:  "pages/blog.post.gwdk",
				ID:      "blog.post",
				Route:   "/blog/{slug}",
				Render:  gowdk.Static,
				Layouts: []string{"root", "blog"},
				Paths:   true,
				Blocks: Blocks{
					View:     true,
					ViewBody: `<main class="post lead" style="color: red;"><img src="/assets/post.png" /><Hero title="Blog" /><ArticleCard title="Post" /></main>`,
					APIs:     []API{{Name: "metadata", Method: "GET", Route: "/blog/{slug}/metadata"}},
				},
			},
			{
				ID:     "newsletter",
				Route:  "/newsletter",
				Render: gowdk.Action,
				Blocks: Blocks{
					Actions: []Action{{
						Name:           "subscribe",
						InputName:      "input",
						InputType:      "SubscribeInput",
						ValidatesInput: true,
						Redirect:       "/newsletter?ok=1",
						Fragments:      []Fragment{{Target: "#newsletter", Body: "<p>Saved</p>"}},
					}},
				},
			},
			{
				ID:      "dashboard",
				Route:   "/dashboard",
				Render:  gowdk.SSR,
				Layouts: []string{"root", "dashboard"},
				Guard:   []string{"auth.required"},
			},
		},
		Components: []Component{
			{
				Source: "components/hero.cmp.gwdk",
				Name:   "Hero",
				Props:  []Prop{{Name: "title", Type: "string"}},
			},
		},
		Layouts: []Layout{
			{
				Source: "layouts/root.layout.gwdk",
				ID:     "root",
			},
		},
	}

	payload, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		Version int `json:"version"`
		Pages   map[string]struct {
			Source          string           `json:"source"`
			Kind            string           `json:"kind"`
			Route           string           `json:"route"`
			Render          gowdk.RenderMode `json:"render"`
			Layouts         []string         `json:"layouts"`
			DynamicParams   []string         `json:"dynamicParams"`
			Paths           bool             `json:"paths"`
			Guard           []string         `json:"guard"`
			StaticAssets    []string         `json:"staticAssets"`
			CSSClasses      []string         `json:"cssClasses"`
			StyleAttributes []string         `json:"styleAttributes"`
			Artifacts       []struct {
				Kind string `json:"kind"`
				Path string `json:"path"`
			} `json:"artifacts"`
			Blocks struct {
				Paths   bool     `json:"paths"`
				Build   bool     `json:"build"`
				Load    bool     `json:"load"`
				View    bool     `json:"view"`
				Actions []string `json:"actions"`
				APIs    []string `json:"apis"`
			} `json:"blocks"`
			Actions []struct {
				Name           string `json:"name"`
				InputName      string `json:"inputName"`
				InputType      string `json:"inputType"`
				ValidatesInput bool   `json:"validatesInput"`
				Redirect       string `json:"redirect"`
				Fragments      []struct {
					Target string `json:"target"`
				} `json:"fragments"`
			} `json:"actions"`
			APIs []struct {
				Name   string `json:"name"`
				Method string `json:"method"`
				Route  string `json:"route"`
			} `json:"apis"`
			Components []string `json:"components"`
		} `json:"pages"`
		Components map[string]struct {
			Source string `json:"source"`
			Kind   string `json:"kind"`
			Props  []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"props"`
		} `json:"components"`
		Layouts map[string]struct {
			Source string `json:"source"`
			Kind   string `json:"kind"`
		} `json:"layouts"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Version != PublicSchemaVersion {
		t.Fatalf("expected manifest version %d, got %d", PublicSchemaVersion, decoded.Version)
	}
	if decoded.Pages["blog.post"].Render != gowdk.Static {
		t.Fatalf("expected blog.post render static, got %q", decoded.Pages["blog.post"].Render)
	}
	if decoded.Pages["blog.post"].Source != "pages/blog.post.gwdk" || decoded.Pages["blog.post"].Kind != "page" {
		t.Fatalf("unexpected blog.post identity: %#v", decoded.Pages["blog.post"])
	}
	if len(decoded.Pages["blog.post"].DynamicParams) != 1 || decoded.Pages["blog.post"].DynamicParams[0] != "slug" {
		t.Fatalf("expected blog.post dynamic param, got %#v", decoded.Pages["blog.post"].DynamicParams)
	}
	if !decoded.Pages["blog.post"].Paths {
		t.Fatal("expected blog.post paths flag")
	}
	if !decoded.Pages["blog.post"].Blocks.Paths || !decoded.Pages["blog.post"].Blocks.View {
		t.Fatalf("expected blog.post blocks, got %#v", decoded.Pages["blog.post"].Blocks)
	}
	if len(decoded.Pages["blog.post"].Blocks.APIs) != 1 || decoded.Pages["blog.post"].Blocks.APIs[0] != "metadata" {
		t.Fatalf("expected blog.post API block name, got %#v", decoded.Pages["blog.post"].Blocks.APIs)
	}
	if len(decoded.Pages["blog.post"].APIs) != 1 || decoded.Pages["blog.post"].APIs[0].Name != "metadata" || decoded.Pages["blog.post"].APIs[0].Method != "GET" || decoded.Pages["blog.post"].APIs[0].Route != "/blog/{slug}/metadata" {
		t.Fatalf("expected blog.post API metadata, got %#v", decoded.Pages["blog.post"].APIs)
	}
	if strings.Join(decoded.Pages["blog.post"].Components, ",") != "ArticleCard,Hero" {
		t.Fatalf("expected blog.post component references, got %#v", decoded.Pages["blog.post"].Components)
	}
	if strings.Join(decoded.Pages["blog.post"].StaticAssets, ",") != "/assets/post.png" {
		t.Fatalf("expected blog.post static assets, got %#v", decoded.Pages["blog.post"].StaticAssets)
	}
	if strings.Join(decoded.Pages["blog.post"].CSSClasses, ",") != "lead,post" {
		t.Fatalf("expected blog.post CSS classes, got %#v", decoded.Pages["blog.post"].CSSClasses)
	}
	if strings.Join(decoded.Pages["blog.post"].StyleAttributes, ",") != "color: red;" {
		t.Fatalf("expected blog.post style attributes, got %#v", decoded.Pages["blog.post"].StyleAttributes)
	}
	if len(decoded.Pages["blog.post"].Artifacts) != 1 || decoded.Pages["blog.post"].Artifacts[0].Kind != "html" || decoded.Pages["blog.post"].Artifacts[0].Path != "blog/{slug}/index.html" {
		t.Fatalf("expected blog.post generated artifact path, got %#v", decoded.Pages["blog.post"].Artifacts)
	}
	if len(decoded.Pages["newsletter"].Artifacts) != 1 || decoded.Pages["newsletter"].Artifacts[0].Kind != "html" || decoded.Pages["newsletter"].Artifacts[0].Path != "newsletter/index.html" {
		t.Fatalf("expected newsletter generated artifact path, got %#v", decoded.Pages["newsletter"].Artifacts)
	}
	if decoded.Pages["dashboard"].Render != gowdk.SSR {
		t.Fatalf("expected dashboard render ssr, got %q", decoded.Pages["dashboard"].Render)
	}
	if len(decoded.Pages["dashboard"].Artifacts) != 0 {
		t.Fatalf("expected no static artifact for SSR page, got %#v", decoded.Pages["dashboard"].Artifacts)
	}
	if decoded.Pages["dashboard"].Guard[0] != "auth.required" {
		t.Fatalf("expected dashboard guard, got %#v", decoded.Pages["dashboard"].Guard)
	}
	action := decoded.Pages["newsletter"].Actions[0]
	if action.Name != "subscribe" || action.InputName != "input" || action.InputType != "SubscribeInput" {
		t.Fatalf("unexpected action metadata: %#v", action)
	}
	if !action.ValidatesInput || action.Redirect != "/newsletter?ok=1" {
		t.Fatalf("unexpected action redirect metadata: %#v", action)
	}
	if len(action.Fragments) != 1 || action.Fragments[0].Target != "#newsletter" {
		t.Fatalf("unexpected action fragment metadata: %#v", action.Fragments)
	}
	component := decoded.Components["Hero"]
	if component.Kind != "component" || component.Source != "components/hero.cmp.gwdk" {
		t.Fatalf("unexpected component identity: %#v", component)
	}
	if len(component.Props) != 1 || component.Props[0].Name != "title" || component.Props[0].Type != "string" {
		t.Fatalf("unexpected component props: %#v", component.Props)
	}
	if decoded.Layouts["root"].Kind != "layout" || decoded.Layouts["root"].Source != "layouts/root.layout.gwdk" {
		t.Fatalf("unexpected layout metadata: %#v", decoded.Layouts)
	}
}

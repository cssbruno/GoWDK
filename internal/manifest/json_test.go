package manifest

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestManifestJSONIncludesRenderModeAndPaths(t *testing.T) {
	app := Manifest{
		Pages: []Page{
			{
				Source:  "pages/blog.post.gwdk",
				Package: "blog",
				ID:      "blog.post",
				Route:   "/blog/{slug}",
				Render:  gowdk.SPA,
				Metadata: PageMetadata{
					Title:       "Blog post",
					Description: "A generated blog post.",
					Canonical:   "https://gowdk.test/blog/hello-gowdk",
					Image:       "https://gowdk.test/assets/blog.png",
				},
				Uses:    []Use{{Alias: "ui", Package: "components"}},
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
				Source:  "components/hero.cmp.gwdk",
				Package: "components",
				Name:    "Hero",
				Props:   []Prop{{Name: "title", Type: "string"}},
				Emits: []Emit{{
					Name:   "select",
					Params: []EmitParam{{Name: "id", Type: "string"}, {Name: "active", Type: "bool"}},
				}},
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
			Source   string           `json:"source"`
			Kind     string           `json:"kind"`
			Package  string           `json:"package"`
			Route    string           `json:"route"`
			Render   gowdk.RenderMode `json:"render"`
			Metadata struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				Canonical   string `json:"canonical"`
				Image       string `json:"image"`
			} `json:"metadata"`
			Uses []struct {
				Alias   string `json:"alias"`
				Package string `json:"package"`
			} `json:"uses"`
			Layouts         []string `json:"layouts"`
			DynamicParams   []string `json:"dynamicParams"`
			Paths           bool     `json:"paths"`
			Guard           []string `json:"guard"`
			Assets          []string `json:"assets"`
			CSSClasses      []string `json:"cssClasses"`
			StyleAttributes []string `json:"styleAttributes"`
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
			Source  string `json:"source"`
			Kind    string `json:"kind"`
			Package string `json:"package"`
			Props   []struct {
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"props"`
			Emits []struct {
				Name   string `json:"name"`
				Params []struct {
					Name string `json:"name"`
					Type string `json:"type"`
				} `json:"params"`
			} `json:"emits"`
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
	if decoded.Pages["blog.post"].Render != gowdk.SPA {
		t.Fatalf("expected blog.post render spa, got %q", decoded.Pages["blog.post"].Render)
	}
	if decoded.Pages["blog.post"].Source != "pages/blog.post.gwdk" || decoded.Pages["blog.post"].Kind != "page" {
		t.Fatalf("unexpected blog.post identity: %#v", decoded.Pages["blog.post"])
	}
	if decoded.Pages["blog.post"].Package != "blog" {
		t.Fatalf("expected blog.post package metadata, got %#v", decoded.Pages["blog.post"])
	}
	if decoded.Pages["blog.post"].Metadata.Title != "Blog post" ||
		decoded.Pages["blog.post"].Metadata.Description != "A generated blog post." ||
		decoded.Pages["blog.post"].Metadata.Canonical != "https://gowdk.test/blog/hello-gowdk" ||
		decoded.Pages["blog.post"].Metadata.Image != "https://gowdk.test/assets/blog.png" {
		t.Fatalf("expected blog.post document metadata, got %#v", decoded.Pages["blog.post"].Metadata)
	}
	if len(decoded.Pages["blog.post"].Uses) != 1 || decoded.Pages["blog.post"].Uses[0].Alias != "ui" || decoded.Pages["blog.post"].Uses[0].Package != "components" {
		t.Fatalf("expected blog.post uses metadata, got %#v", decoded.Pages["blog.post"].Uses)
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
	if strings.Join(decoded.Pages["blog.post"].Assets, ",") != "/assets/post.png" {
		t.Fatalf("expected blog.post app assets, got %#v", decoded.Pages["blog.post"].Assets)
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
		t.Fatalf("expected no app artifact for SSR page, got %#v", decoded.Pages["dashboard"].Artifacts)
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
	if component.Package != "components" {
		t.Fatalf("unexpected component package metadata: %#v", component)
	}
	if len(component.Props) != 1 || component.Props[0].Name != "title" || component.Props[0].Type != "string" {
		t.Fatalf("unexpected component props: %#v", component.Props)
	}
	if len(component.Emits) != 1 || component.Emits[0].Name != "select" {
		t.Fatalf("unexpected component emits: %#v", component.Emits)
	}
	if len(component.Emits[0].Params) != 2 || component.Emits[0].Params[0].Name != "id" || component.Emits[0].Params[0].Type != "string" || component.Emits[0].Params[1].Name != "active" || component.Emits[0].Params[1].Type != "bool" {
		t.Fatalf("unexpected component emit params: %#v", component.Emits[0].Params)
	}
	if decoded.Layouts["root"].Kind != "layout" || decoded.Layouts["root"].Source != "layouts/root.layout.gwdk" {
		t.Fatalf("unexpected layout metadata: %#v", decoded.Layouts)
	}
}

func TestManifestJSONIncludesBackendBindingSignatureMetadata(t *testing.T) {
	app := Manifest{BackendBindings: []BackendBinding{{
		Kind:         "action",
		PageID:       "login",
		Source:       "features/auth/login.page.gwdk",
		BlockName:    "login",
		Method:       "POST",
		Route:        "/login",
		ImportPath:   "example.com/app/features/auth",
		PackageName:  "auth",
		FunctionName: "Login",
		Signature:    BackendSignatureActionFormPtr,
		InputType:    "LoginInput",
		InputPointer: true,
		InputFields: []BackendInputField{{
			FieldName: "Email",
			FormName:  "email",
			Type:      "string",
		}},
		Status:  BackendBindingBound,
		Message: "bound",
	}}}

	payload, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		BackendBindings []struct {
			Kind         string               `json:"kind"`
			PageID       string               `json:"pageId"`
			Source       string               `json:"source"`
			BlockName    string               `json:"blockName"`
			Method       string               `json:"method"`
			Route        string               `json:"route"`
			ImportPath   string               `json:"importPath"`
			PackageName  string               `json:"packageName"`
			FunctionName string               `json:"functionName"`
			Signature    BackendSignatureKind `json:"signature"`
			InputType    string               `json:"inputType"`
			InputPointer bool                 `json:"inputPointer"`
			InputFields  []struct {
				FieldName string `json:"fieldName"`
				FormName  string `json:"formName"`
				Type      string `json:"type"`
			} `json:"inputFields"`
			Status  BackendBindingStatus `json:"status"`
			Message string               `json:"message"`
		} `json:"backendBindings"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.BackendBindings) != 1 {
		t.Fatalf("expected one backend binding, got %#v", decoded.BackendBindings)
	}
	binding := decoded.BackendBindings[0]
	if binding.Kind != "action" ||
		binding.PageID != "login" ||
		binding.Source != "features/auth/login.page.gwdk" ||
		binding.BlockName != "login" ||
		binding.Method != "POST" ||
		binding.Route != "/login" ||
		binding.ImportPath != "example.com/app/features/auth" ||
		binding.PackageName != "auth" ||
		binding.FunctionName != "Login" ||
		binding.Signature != BackendSignatureActionFormPtr ||
		binding.InputType != "LoginInput" ||
		!binding.InputPointer ||
		len(binding.InputFields) != 1 ||
		binding.InputFields[0].FieldName != "Email" ||
		binding.InputFields[0].FormName != "email" ||
		binding.InputFields[0].Type != "string" ||
		binding.Status != BackendBindingBound ||
		binding.Message != "bound" {
		t.Fatalf("unexpected backend binding metadata: %#v", binding)
	}
}

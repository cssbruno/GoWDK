package manifest

import (
	"encoding/json"
	"testing"

	"github.com/gowdk/gowdk"
)

func TestManifestJSONIncludesRenderModeAndPaths(t *testing.T) {
	app := Manifest{
		Pages: []Page{
			{
				ID:      "blog.post",
				Route:   "/blog/{slug}",
				Render:  gowdk.Static,
				Layouts: []string{"root", "blog"},
				Paths:   true,
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
	}

	payload, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}

	var decoded struct {
		Pages map[string]struct {
			Route   string           `json:"route"`
			Render  gowdk.RenderMode `json:"render"`
			Layouts []string         `json:"layouts"`
			Paths   bool             `json:"paths"`
			Guard   []string         `json:"guard"`
			Actions []struct {
				Name           string `json:"name"`
				InputName      string `json:"inputName"`
				InputType      string `json:"inputType"`
				ValidatesInput bool   `json:"validatesInput"`
				Redirect       string `json:"redirect"`
			} `json:"actions"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Pages["blog.post"].Render != gowdk.Static {
		t.Fatalf("expected blog.post render static, got %q", decoded.Pages["blog.post"].Render)
	}
	if !decoded.Pages["blog.post"].Paths {
		t.Fatal("expected blog.post paths flag")
	}
	if decoded.Pages["dashboard"].Render != gowdk.SSR {
		t.Fatalf("expected dashboard render ssr, got %q", decoded.Pages["dashboard"].Render)
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
}

package spa

import (
	"testing"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/runtime/response"
)

func TestAddonRegistersSPAFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "spa" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureSPA) {
		t.Fatal("expected spa feature")
	}
}

func TestPathSetStoresRouteParams(t *testing.T) {
	paths := PathSet{{"slug": "hello-gowdk"}}
	if paths[0]["slug"] != "hello-gowdk" {
		t.Fatalf("unexpected path set: %#v", paths)
	}
}

func TestPrerenderedPageStoresRouteOutput(t *testing.T) {
	page := PrerenderedPage{
		Route: "/",
		Path:  "index.html",
		HTML:  response.HTMLBody(200, "<h1>Home</h1>"),
	}

	if page.HTML.Kind != response.HTML || page.HTML.Body != "<h1>Home</h1>" {
		t.Fatalf("unexpected prerendered page: %#v", page)
	}
}

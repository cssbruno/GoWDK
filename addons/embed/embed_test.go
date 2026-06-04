package embed

import (
	"net/http"
	"testing"

	"github.com/cssbruno/gowdk"
)

func TestAddonRegistersEmbedFeature(t *testing.T) {
	addon := Addon()
	if addon.Name() != "embed" {
		t.Fatalf("unexpected addon name: %q", addon.Name())
	}
	if !(gowdk.Config{Addons: []gowdk.Addon{addon}}).HasFeature(gowdk.FeatureEmbed) {
		t.Fatal("expected embed feature")
	}
}

func TestAssetsFileServerReturnsHandler(t *testing.T) {
	handler := (Assets{FS: http.Dir(".")}).FileServer()
	if handler == nil {
		t.Fatal("expected file server handler")
	}
}

func TestManifestAliasResolvesAssets(t *testing.T) {
	manifest := Manifest{Version: 1, Files: map[string]string{"app.css": "assets/app.css"}}
	if got := manifest.Resolve("app.css"); got != "assets/app.css" {
		t.Fatalf("unexpected asset path: %q", got)
	}
}

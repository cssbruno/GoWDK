package asset

import "testing"

func TestManifestResolve(t *testing.T) {
	manifest := Manifest{
		Version: 1,
		Files: map[string]string{
			"assets/app.css": "assets/app.css",
		},
	}

	if got := manifest.Resolve("assets/app.css"); got != "assets/app.css" {
		t.Fatalf("expected asset path, got %q", got)
	}
	if got := manifest.Resolve("missing.css"); got != "" {
		t.Fatalf("expected missing asset to resolve empty, got %q", got)
	}
	if got := (Manifest{}).Resolve("assets/app.css"); got != "" {
		t.Fatalf("expected nil manifest to resolve empty, got %q", got)
	}
}

func TestManifestURL(t *testing.T) {
	manifest := Manifest{
		Version: 1,
		Files: map[string]string{
			"app.css":      "assets/app.css",
			"absolute.css": "/assets/absolute.css",
		},
	}

	if got := manifest.URL("app.css"); got != "/assets/app.css" {
		t.Fatalf("expected asset URL, got %q", got)
	}
	if got := manifest.URL("absolute.css"); got != "/assets/absolute.css" {
		t.Fatalf("expected absolute asset URL to be preserved, got %q", got)
	}
	if got := manifest.URL("missing.css"); got != "" {
		t.Fatalf("expected missing asset URL to be empty, got %q", got)
	}
}

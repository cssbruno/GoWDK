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

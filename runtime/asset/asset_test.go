package asset

import "testing"

func TestManifestResolve(t *testing.T) {
	manifest := Manifest{
		Version: ManifestVersion,
		Files: map[string]string{
			"assets/app.css": "assets/app.css",
		},
		Hashes: map[string]string{
			"assets/app.css": "sha256:abc",
		},
		Cache: map[string]string{
			"assets/app.css": "public, max-age=31536000, immutable",
		},
		Sizes: map[string]int64{
			"assets/app.css": 42,
		},
		Obfuscated: map[string]bool{
			"assets/app.css": true,
		},
	}

	if got := manifest.Resolve("assets/app.css"); got != "assets/app.css" {
		t.Fatalf("expected asset path, got %q", got)
	}
	if got := manifest.Resolve("missing.css"); got != "" {
		t.Fatalf("expected missing asset to resolve empty, got %q", got)
	}
	if got := manifest.Hash("assets/app.css"); got != "sha256:abc" {
		t.Fatalf("expected asset hash, got %q", got)
	}
	if got := manifest.CachePolicy("assets/app.css"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("expected asset cache policy, got %q", got)
	}
	if got := manifest.SizeBytes("assets/app.css"); got != 42 {
		t.Fatalf("expected asset size, got %d", got)
	}
	if got := manifest.SizeBytes("missing.css"); got != 0 {
		t.Fatalf("expected missing asset size to be zero, got %d", got)
	}
	if !manifest.IsObfuscated("assets/app.css") {
		t.Fatal("expected obfuscated asset marker")
	}
	if (Manifest{}).IsObfuscated("assets/app.css") {
		t.Fatal("expected nil manifest obfuscation marker to be false")
	}
	if got := (Manifest{}).Resolve("assets/app.css"); got != "" {
		t.Fatalf("expected nil manifest to resolve empty, got %q", got)
	}
}

func TestManifestURL(t *testing.T) {
	manifest := Manifest{
		Version: ManifestVersion,
		Files: map[string]string{
			"app.css":                "assets/app.css",
			"absolute.css":           "/assets/absolute.css",
			"protocol-relative.css":  "//assets/protocol.css",
			"slash-backslash.css":    `/\assets\slash.css`,
			"backslash-relative.css": `\assets\relative.css`,
		},
	}

	if got := manifest.URL("app.css"); got != "/assets/app.css" {
		t.Fatalf("expected asset URL, got %q", got)
	}
	if got := manifest.URL("absolute.css"); got != "/assets/absolute.css" {
		t.Fatalf("expected absolute asset URL to be preserved, got %q", got)
	}
	if got := manifest.URL("protocol-relative.css"); got != "/assets/protocol.css" {
		t.Fatalf("expected protocol-relative asset URL to be normalized, got %q", got)
	}
	if got := manifest.URL("slash-backslash.css"); got != "/assets/slash.css" {
		t.Fatalf("expected slash-backslash asset URL to be normalized, got %q", got)
	}
	if got := manifest.URL("backslash-relative.css"); got != "/assets/relative.css" {
		t.Fatalf("expected backslash-relative asset URL to be normalized, got %q", got)
	}
	if got := manifest.URL("missing.css"); got != "" {
		t.Fatalf("expected missing asset URL to be empty, got %q", got)
	}
}

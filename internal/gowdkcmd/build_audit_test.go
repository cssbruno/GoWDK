package gowdkcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk/internal/buildgen"
)

func TestFinalBuildArtifactSecurityFindingsClassifiesBinaryAsNonText(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "app.wasm")
	if err := os.WriteFile(binary, []byte("\x00token=live_secret_1234567890"), 0o644); err != nil {
		t.Fatal(err)
	}

	findings := finalBuildArtifactSecurityFindings(buildgen.Result{
		AssetArtifacts: []buildgen.AssetArtifact{{Path: binary}},
	})
	if len(findings) != 0 {
		t.Fatalf("expected binary artifact to be skipped, got %#v", findings)
	}
}

func TestFinalBuildArtifactSecurityFindingsScansTextAndWarnsOnLargeText(t *testing.T) {
	root := t.TempDir()
	secret := filepath.Join(root, "index.html")
	if err := os.WriteFile(secret, []byte(`<main data-token="token=live_secret_1234567890"></main>`), 0o644); err != nil {
		t.Fatal(err)
	}
	large := filepath.Join(root, "large.js")
	payload := strings.Repeat("a", finalArtifactMaxTextBytes+1)
	if err := os.WriteFile(large, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}

	findings := finalBuildArtifactSecurityFindings(buildgen.Result{
		Artifacts:      []buildgen.Artifact{{Path: secret}},
		AssetArtifacts: []buildgen.AssetArtifact{{Path: large}},
	})
	if len(findings) != 2 {
		t.Fatalf("expected secret and large-text findings, got %#v", findings)
	}
	if !strings.Contains(findings[0].Message, "secret-shaped") {
		t.Fatalf("expected secret finding first, got %#v", findings)
	}
	if !strings.Contains(findings[1].Message, "exceeds the bundled-secret scan limit") {
		t.Fatalf("expected large text warning, got %#v", findings)
	}
}

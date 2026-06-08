package main

import (
	"os"
	"strings"
	"testing"
)

func TestEditorReleaseWorkflowCoverage(t *testing.T) {
	releaseText := readWorkflow(t, "../../.github/workflows/release.yml")
	publishText := readWorkflow(t, "../../.github/workflows/vscode-extension-publish.yml")

	for workflow, text := range map[string]string{
		"release.yml":                  releaseText,
		"vscode-extension-publish.yml": publishText,
	} {
		for _, expected := range []string{"node --check", "node --test", "editors/vscode"} {
			if !strings.Contains(text, expected) {
				t.Fatalf("expected %q in %s:\n%s", expected, workflow, text)
			}
		}
	}

	if !strings.Contains(releaseText, "node scripts/package-vsix.js") {
		t.Fatalf("expected local VSIX packager in release.yml:\n%s", releaseText)
	}
	for _, forbidden := range []string{"npm install", "vsce package", "npx "} {
		if strings.Contains(releaseText, forbidden) {
			t.Fatalf("did not expect %q in release workflow:\n%s", forbidden, releaseText)
		}
	}
	for _, expected := range []string{
		"npm install -g @vscode/vsce",
		"vsce package",
		"--packagePath",
		"vsce \"${publish_args[@]}\"",
		"--pre-release",
	} {
		if !strings.Contains(publishText, expected) {
			t.Fatalf("expected %q in publish workflow:\n%s", expected, publishText)
		}
	}
}

func TestReleaseTrustWorkflowCoverage(t *testing.T) {
	ciText := readWorkflow(t, "../../.github/workflows/ci.yml")
	releaseText := readWorkflow(t, "../../.github/workflows/release.yml")
	smokeText := readWorkflow(t, "../../.github/workflows/release-smoke.yml")
	cacheText := readWorkflow(t, "../../.github/workflows/cache-maintenance.yml")

	for workflow, text := range map[string]string{
		"ci.yml":      ciText,
		"release.yml": releaseText,
	} {
		for _, expected := range []string{
			"scripts/check-release-policy.sh",
			"scripts/validate-release-notes.sh",
			".github/release-note-template.md",
			"docs/engineering/release-notes-v0.2.md",
		} {
			if !strings.Contains(text, expected) {
				t.Fatalf("expected %q in %s:\n%s", expected, workflow, text)
			}
		}
	}

	for _, expected := range []string{
		"go version",
		"go env GOVERSION",
		"version --json",
		"sha256sum -c checksums.txt",
		"actions/upload-artifact",
		"if-no-files-found: error",
		"Validate selected release notes",
		"fail_on_unmatched_files: true",
		"draft: false",
		"prerelease: true",
		"Verify release assets",
	} {
		if !strings.Contains(releaseText, expected) {
			t.Fatalf("expected %q in release.yml:\n%s", expected, releaseText)
		}
	}

	for _, expected := range []string{
		"workflow_dispatch",
		"scripts/smoke-release-artifact.sh",
		"gowdk-linux-amd64",
		"gowdk-darwin-amd64",
		"gowdk-darwin-arm64",
		"gowdk-windows-amd64.exe",
	} {
		if !strings.Contains(smokeText, expected) {
			t.Fatalf("expected %q in release-smoke.yml:\n%s", expected, smokeText)
		}
	}

	for _, expected := range []string{
		"workflow_dispatch",
		"schedule:",
		"actions: write",
		"scripts/prune-github-caches.sh",
		"GOWDK_CACHE_PRUNE_KEEP",
	} {
		if !strings.Contains(cacheText, expected) {
			t.Fatalf("expected %q in cache-maintenance.yml:\n%s", expected, cacheText)
		}
	}
}

func readWorkflow(t *testing.T, path string) string {
	t.Helper()
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

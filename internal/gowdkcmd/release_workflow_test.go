package gowdkcmd

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
	if !strings.Contains(releaseText, "npm ci") {
		t.Fatalf("expected locked npm install in release.yml:\n%s", releaseText)
	}
	for _, forbidden := range []string{"npm install -g", "vsce package", "npx --yes"} {
		if strings.Contains(releaseText, forbidden) {
			t.Fatalf("did not expect %q in release workflow:\n%s", forbidden, releaseText)
		}
	}
	for _, expected := range []string{
		"npm ci",
		"npx --no-install vsce package",
		"--packagePath",
		"npx --no-install vsce \"${publish_args[@]}\"",
		"--pre-release",
	} {
		if !strings.Contains(publishText, expected) {
			t.Fatalf("expected %q in publish workflow:\n%s", expected, publishText)
		}
	}
}

func TestReleaseTrustWorkflowCoverage(t *testing.T) {
	releaseText := readWorkflow(t, "../../.github/workflows/release.yml")
	exampleReportText := readWorkflow(t, "../../scripts/check-example-reports.sh")

	for _, expected := range []string{
		"workflow_dispatch",
		"go version",
		"go env GOVERSION",
		"version --json",
		"scripts/check-supply-chain-pins.sh",
		"sha256sum -c checksums.txt",
		"actions/upload-artifact",
		"if-no-files-found: error",
		"fail_on_unmatched_files: true",
		"draft: false",
		"prerelease: false",
		"Verify release assets",
		"scripts/check-example-reports.sh --extended",
	} {
		if !strings.Contains(releaseText, expected) {
			t.Fatalf("expected %q in release.yml:\n%s", expected, releaseText)
		}
	}

	for _, expected := range []string{
		"gowdk endpoints",
		"inspect tree",
		"inspect endpoint-graph",
		"inspect asset-graph",
	} {
		if !strings.Contains(exampleReportText, expected) {
			t.Fatalf("expected %q in check-example-reports.sh:\n%s", expected, exampleReportText)
		}
	}

	for _, expected := range []string{
		"gowdk-linux-amd64",
		"gowdk-darwin-amd64",
		"gowdk-darwin-arm64",
		"gowdk-windows-amd64.exe",
	} {
		if !strings.Contains(releaseText, expected) {
			t.Fatalf("expected %q in release.yml:\n%s", expected, releaseText)
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

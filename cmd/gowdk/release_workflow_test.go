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
		for _, expected := range []string{
			"node --check",
			"node --test",
			"node scripts/package-vsix.js",
			"editors/vscode",
		} {
			if !strings.Contains(text, expected) {
				t.Fatalf("expected %q in %s:\n%s", expected, workflow, text)
			}
		}
	}

	for _, forbidden := range []string{
		"npm install",
		"vsce package",
		"npx ",
	} {
		if strings.Contains(releaseText, forbidden) {
			t.Fatalf("did not expect %q in release workflow:\n%s", forbidden, releaseText)
		}
	}

	for _, expected := range []string{
		"npm install -g @vscode/vsce",
		"--packagePath",
		"vsce \"${publish_args[@]}\"",
	} {
		if !strings.Contains(publishText, expected) {
			t.Fatalf("expected %q in publish workflow:\n%s", expected, publishText)
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

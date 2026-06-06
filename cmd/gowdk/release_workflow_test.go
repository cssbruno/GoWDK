package main

import (
	"os"
	"strings"
	"testing"
)

func TestEditorReleaseWorkflowCoverage(t *testing.T) {
	for _, workflow := range []string{
		"../../.github/workflows/release.yml",
		"../../.github/workflows/vscode-extension-publish.yml",
	} {
		payload, err := os.ReadFile(workflow)
		if err != nil {
			t.Fatal(err)
		}
		text := string(payload)
		for _, expected := range []string{
			"node --check",
			"node --test",
			"vsce package",
			"editors/vscode",
		} {
			if !strings.Contains(text, expected) {
				t.Fatalf("expected %q in %s:\n%s", expected, workflow, text)
			}
		}
	}
}

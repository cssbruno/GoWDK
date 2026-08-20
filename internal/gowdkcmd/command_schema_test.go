package gowdkcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandSchemaOwnsRecursiveHelpFlagsAndCompletions(t *testing.T) {
	seen := map[string]bool{}
	for _, record := range commandRecords() {
		path := strings.Join(record.Path, " ")
		if seen[path] {
			t.Fatalf("duplicate command path %q", path)
		}
		seen[path] = true
		if !strings.HasPrefix(record.Spec.Usage(), "usage: gowdk "+path) && len(record.Path) > 1 && record.Path[0] != "list" && record.Path[0] != "playground" {
			t.Fatalf("usage for %q drifted from its path: %q", path, record.Spec.Usage())
		}
		for _, flag := range record.Flags {
			if !strings.Contains(record.Spec.Usage(), flag.Name) {
				t.Fatalf("flag %q for %q is absent from canonical usage", flag.Name, path)
			}
			if flag.Group == "" {
				t.Fatalf("flag %q for %q has no group", flag.Name, path)
			}
		}
	}

	outputs := bashCompletion() + zshCompletion() + fishCompletion()
	for _, spec := range topLevelCommands {
		if !strings.Contains(outputs, spec.Name) {
			t.Fatalf("completion output omits command %q", spec.Name)
		}
		for _, flag := range commandFlags(spec) {
			if !strings.Contains(outputs, flag.Name) && !strings.Contains(outputs, strings.TrimPrefix(flag.Name, "--")) {
				t.Fatalf("completion output omits %s flag %q", spec.Name, flag.Name)
			}
		}
	}
}

func TestPublishedCommandSchemaIsCurrent(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "reference", "cli-schema.md")
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(payload), commandDocumentationMarkdown(); got != want {
		t.Fatalf("%s is stale; run scripts/generate-cli-schema.sh", path)
	}
}

package diagnostics

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

var diagnosticLiteralPatterns = []*regexp.Regexp{
	regexp.MustCompile(`Code:\s*"([a-z][a-z0-9_]*[a-z0-9])"`),
	regexp.MustCompile(`return\s+"(old_action_block_syntax|old_api_block_syntax|package_must_be_first|malformed_gowdk_use|parse_error)"`),
	regexp.MustCompile(`contractDiagnostic\([^,\n]+,\s*"([a-z][a-z0-9_]*[a-z0-9])"`),
	regexp.MustCompile(`contractReferenceDiagnostic\([^,\n]+,\s*"([a-z][a-z0-9_]*[a-z0-9])"`),
	regexp.MustCompile(`goEndpointDiagnostic\([^,\n]+,\s*[^,\n]+,\s*[^,\n]+,\s*"([a-z][a-z0-9_]*[a-z0-9])"`),
	regexp.MustCompile(`clientGoBlockDiagnosticError\([^,\n]+,\s*"([a-z][a-z0-9_]*[a-z0-9])"`),
	regexp.MustCompile(`wasmIslandDiagnosticError\([^,\n]+,\s*"([a-z][a-z0-9_]*[a-z0-9])"`),
}

func TestRegistryIsSortedUniqueAndComplete(t *testing.T) {
	seen := map[string]bool{}
	var previous string
	for _, entry := range Registry {
		if entry.Code == "" || entry.Area == "" || entry.Summary == "" {
			t.Fatalf("registry entry must include code, area, and summary: %#v", entry)
		}
		switch entry.Stability {
		case StabilityStable, StabilityExperimental, StabilityAddon:
		default:
			t.Fatalf("unknown stability for %s: %q", entry.Code, entry.Stability)
		}
		if seen[entry.Code] {
			t.Fatalf("duplicate diagnostic code %q", entry.Code)
		}
		seen[entry.Code] = true
		if previous != "" && entry.Code < previous {
			t.Fatalf("registry must be sorted by code: %q appears after %q", entry.Code, previous)
		}
		previous = entry.Code
	}

	emitted := emittedDiagnosticCodes(t)
	var missing []string
	for code := range emitted {
		if !seen[code] {
			missing = append(missing, code)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("diagnostic codes emitted by source are missing from registry: %s", strings.Join(missing, ", "))
	}
}

func emittedDiagnosticCodes(t *testing.T) map[string]bool {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", ".."))
	codes := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "dist", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(source)
		for _, pattern := range diagnosticLiteralPatterns {
			for _, match := range pattern.FindAllStringSubmatch(text, -1) {
				codes[match[1]] = true
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return codes
}

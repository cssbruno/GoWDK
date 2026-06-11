package lang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cssbruno/gowdk"
)

// The conformance corpus pins the .gwdk language contract: every file under
// testdata/conformance/accept must check clean (no error-severity
// diagnostics), and every file under testdata/conformance/reject must produce
// the stable diagnostic codes named in its leading `// expect: <code>[, code]`
// directive. New language syntax should add a corpus case here. See
// docs/language/conformance.md.

const conformanceExpectPrefix = "// expect:"

func TestConformanceCorpusAccept(t *testing.T) {
	dir := filepath.FromSlash("testdata/conformance/accept")
	for _, name := range conformanceFiles(t, dir) {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			source := readConformanceFile(t, path)
			_, diagnostics := CheckSource(gowdk.Config{}, path, source)
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == "error" {
					t.Errorf("accept case produced error %q: %s", diagnostic.Code, diagnostic.Message)
				}
			}
		})
	}
}

func TestConformanceCorpusReject(t *testing.T) {
	dir := filepath.FromSlash("testdata/conformance/reject")
	for _, name := range conformanceFiles(t, dir) {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			source := readConformanceFile(t, path)
			expected := conformanceExpectedCodes(source)
			if len(expected) == 0 {
				t.Fatalf("reject case %s is missing a %q directive", name, conformanceExpectPrefix)
			}
			_, diagnostics := CheckSource(gowdk.Config{}, path, source)
			got := conformanceCodes(diagnostics)
			for _, code := range expected {
				if !containsCode(got, code) {
					t.Errorf("reject case %s expected diagnostic %q; got %v", name, code, got)
				}
			}
		})
	}
}

// TestConformanceCorpusCoversRejectionContracts fails when a rejection contract
// that surfaces a specific stable code through the single-file check loses its
// reject case. Markup directive/syntax rejections currently surface as
// parse_error (a tracked limitation), so they are pinned by their own reject
// cases rather than listed here.
func TestConformanceCorpusCoversRejectionContracts(t *testing.T) {
	required := []string{
		"unsupported_top_level_block",
		"old_action_block_syntax",
		"old_api_block_syntax",
		"malformed_legacy_metadata",
		"malformed_gowdk_use",
	}

	covered := map[string]bool{}
	dir := filepath.FromSlash("testdata/conformance/reject")
	for _, name := range conformanceFiles(t, dir) {
		source := readConformanceFile(t, filepath.Join(dir, name))
		for _, code := range conformanceExpectedCodes(source) {
			covered[code] = true
		}
	}

	for _, code := range required {
		if !covered[code] {
			t.Errorf("no reject corpus case expects required rejection code %q", code)
		}
	}
}

func conformanceFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read corpus dir %s: %v", dir, err)
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".gwdk") {
			continue
		}
		names = append(names, entry.Name())
	}
	if len(names) == 0 {
		t.Fatalf("corpus dir %s has no .gwdk cases", dir)
	}
	return names
}

func readConformanceFile(t *testing.T, path string) []byte {
	t.Helper()
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func conformanceExpectedCodes(source []byte) []string {
	for _, line := range strings.Split(string(source), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, conformanceExpectPrefix) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(trimmed, conformanceExpectPrefix))
		var codes []string
		for _, field := range strings.FieldsFunc(rest, func(r rune) bool { return r == ',' || r == ' ' }) {
			if field != "" {
				codes = append(codes, field)
			}
		}
		return codes
	}
	return nil
}

func conformanceCodes(diagnostics Diagnostics) []string {
	codes := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	return codes
}

func containsCode(codes []string, want string) bool {
	for _, code := range codes {
		if code == want {
			return true
		}
	}
	return false
}

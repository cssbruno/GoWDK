package envfile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseFileReadsDotenvAssignments(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(`
# comment
PLAIN=value
export EXPORTED=from-export
SPACED = value with spaces # comment
HASH=abc#def
SINGLE='quoted # value'
DOUBLE="line\nvalue"
EMPTY=
`), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := ParseFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, entry := range entries {
		got[entry.Name] = entry.Value
	}
	want := map[string]string{
		"PLAIN":    "value",
		"EXPORTED": "from-export",
		"SPACED":   "value with spaces",
		"HASH":     "abc#def",
		"SINGLE":   "quoted # value",
		"DOUBLE":   "line\nvalue",
		"EMPTY":    "",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected entries:\n got %#v\nwant %#v", got, want)
	}
}

func TestParseFileRejectsInvalidLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("BAD LINE\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := ParseFile(path)
	if err == nil || !strings.Contains(err.Error(), "expected NAME=value") {
		t.Fatalf("expected parse error, got %v", err)
	}
}

func TestLoadIntoEnvPreservesProcessValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("GOWDK_TEST_FILE_ONLY=file\nGOWDK_TEST_PROCESS=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWDK_TEST_PROCESS", "from-process")
	t.Cleanup(func() {
		_ = os.Unsetenv("GOWDK_TEST_FILE_ONLY")
	})

	result, err := LoadIntoEnv(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("GOWDK_TEST_FILE_ONLY") != "file" {
		t.Fatalf("expected file value to be applied")
	}
	if os.Getenv("GOWDK_TEST_PROCESS") != "from-process" {
		t.Fatalf("expected process value to win")
	}
	if !reflect.DeepEqual(result.Applied, []string{"GOWDK_TEST_FILE_ONLY"}) ||
		!reflect.DeepEqual(result.Skipped, []string{"GOWDK_TEST_PROCESS"}) {
		t.Fatalf("unexpected load result: %#v", result)
	}
}

func TestLoadIntoEnvUpdatesValuesItPreviouslyApplied(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	name := "GOWDK_TEST_FILE_RELOAD"
	_ = os.Unsetenv(name)
	t.Cleanup(func() {
		_ = os.Unsetenv(name)
		appliedMu.Lock()
		delete(appliedValues, name)
		appliedMu.Unlock()
	})

	if err := os.WriteFile(path, []byte(name+"=first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIntoEnv(path, false); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(name+"=second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIntoEnv(path, false); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(name); got != "second" {
		t.Fatalf("expected file-applied value to update, got %q", got)
	}
}

func TestLoadIntoEnvDoesNotOverrideManualValueAfterLoad(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".env")
	name := "GOWDK_TEST_MANUAL_OVERRIDE"
	_ = os.Unsetenv(name)
	t.Cleanup(func() {
		_ = os.Unsetenv(name)
		appliedMu.Lock()
		delete(appliedValues, name)
		appliedMu.Unlock()
	})

	if err := os.WriteFile(path, []byte(name+"=from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIntoEnv(path, false); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(name); got != "from-file" {
		t.Fatalf("expected initial file value, got %q", got)
	}

	// Override the value manually after the initial file load.
	if err := os.Setenv(name, "manual"); err != nil {
		t.Fatal(err)
	}

	result, err := LoadIntoEnv(path, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(name); got != "manual" {
		t.Fatalf("expected manual override to survive reload, got %q", got)
	}
	if !reflect.DeepEqual(result.Skipped, []string{name}) {
		t.Fatalf("expected %q skipped on reload, got %#v", name, result)
	}
}

func TestLookupPathUsesExplicitThenGOWDKEnvThenDefault(t *testing.T) {
	root := t.TempDir()
	write := func(name string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte("A=B\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(".env")
	write(".env.dev")
	t.Setenv("GOWDK_ENV", "dev")

	path, explicit, err := LookupPath(root, "")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, ".env.dev") || explicit {
		t.Fatalf("expected .env.dev discovery, got path=%q explicit=%v", path, explicit)
	}

	path, explicit, err = LookupPath(root, "custom.env")
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(root, "custom.env") || !explicit {
		t.Fatalf("expected explicit path, got path=%q explicit=%v", path, explicit)
	}
}

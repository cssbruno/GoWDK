package contractscan

import (
	"strings"
	"testing"
)

func TestScanGoListExportFilesSurfacesStderr(t *testing.T) {
	// A temp dir outside any Go module makes `go list` fail with a clear reason
	// (no go.mod), which must reach the caller instead of an opaque exit status.
	dir := t.TempDir()
	_, err := scanGoListExportFiles(dir, []string{"example.com/does-not-exist"})
	if err == nil {
		t.Fatal("expected go list to fail outside a Go module")
	}
	if !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("expected the underlying go list stderr (go.mod not found) to be surfaced, got: %v", err)
	}
}

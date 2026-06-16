//go:build linux

package playground

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestTmpfsSizeOptions(t *testing.T) {
	if got := tmpfsSizeOptions(0); got != "" {
		t.Fatalf("zero size must yield no options, got %q", got)
	}
	got := tmpfsSizeOptions(1 << 20)
	if !strings.Contains(got, "size=1048576") || !strings.Contains(got, "nr_inodes=") {
		t.Fatalf("expected size and nr_inodes options, got %q", got)
	}
}

func TestIsInitialUserNS(t *testing.T) {
	cases := []struct {
		name   string
		uidMap string
		want   bool
	}{
		{"initial identity mapping", "         0          0 4294967295\n", true},
		{"sandbox single-uid mapping", "         0       1000          1\n", false},
		{"empty", "", false},
		{"multi-line", "0 0 1000\n1000 1000 1\n", false},
	}
	for _, tc := range cases {
		if got := isInitialUserNS(tc.uidMap); got != tc.want {
			t.Errorf("%s: isInitialUserNS(%q) = %v, want %v", tc.name, tc.uidMap, got, tc.want)
		}
	}
}

// ConfineToSandbox must refuse when invoked outside the dedicated namespaces the
// launcher creates. The test process is not PID 1, so the gate rejects it before
// any mount is attempted — proving a direct call to the re-exec target cannot
// perform privileged mounts in the caller's namespace.
func TestConfineRefusesWhenNotLaunchedBySandbox(t *testing.T) {
	if os.Getpid() == 1 {
		t.Skip("test runner is PID 1; the PID-namespace gate cannot be exercised here")
	}
	err := ConfineToSandbox(SandboxSpec{})
	if err == nil {
		t.Fatal("expected ConfineToSandbox to refuse outside the sandbox namespaces")
	}
	if errors.Is(err, ErrSandboxUnsupported) {
		t.Fatalf("refusal must be a hard error, not a soft unsupported skip: %v", err)
	}
}

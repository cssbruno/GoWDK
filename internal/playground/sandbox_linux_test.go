//go:build linux

package playground

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestMain lets the test binary re-execute itself as the sandboxed child. When
// GOWDK_SANDBOX_CHILD is set it runs the isolation probe inside the namespaces
// LaunchSandbox created, instead of the normal test suite.
func TestMain(m *testing.M) {
	if os.Getenv("GOWDK_SANDBOX_CHILD") == "1" {
		os.Exit(runSandboxProbe())
	}
	os.Exit(m.Run())
}

// runSandboxProbe confines itself, then asserts that the sandbox actually denies
// network, host-file reads, and privilege escalation while allowing the staged
// workspace. It prints SANDBOX_OK and exits 0 only when every property holds.
func runSandboxProbe() int {
	spec, err := DecodeSandboxSpec(os.Getenv("GOWDK_SANDBOX_SPEC"))
	if err != nil {
		fmt.Println("FAIL decode spec:", err)
		return 1
	}
	if err := ConfineToSandbox(spec); err != nil {
		// The environment created the namespaces but denied confinement (e.g. a
		// blocked mount). That is "sandbox unavailable", not a test failure: exit
		// with the sentinel so the parent skips instead of failing.
		if errors.Is(err, ErrSandboxUnsupported) {
			fmt.Println("SANDBOX_UNSUPPORTED:", err)
			return SandboxUnsupportedExitCode
		}
		fmt.Println("FAIL confine:", err)
		return 1
	}

	// Network must be unreachable: a fresh network namespace has no route out.
	if conn, err := net.DialTimeout("tcp", "1.1.1.1:80", 750*time.Millisecond); err == nil {
		conn.Close()
		fmt.Println("FAIL network reachable")
		return 1
	}

	// The host filesystem is gone after pivot_root, so a host-only secret path
	// must not resolve to the secret.
	secretPath := os.Getenv("GOWDK_SANDBOX_SECRET")
	if data, err := os.ReadFile(secretPath); err == nil {
		fmt.Printf("FAIL read host secret %q: %q\n", secretPath, string(data))
		return 1
	}
	if _, err := os.ReadFile("/etc/hostname"); err == nil {
		fmt.Println("FAIL host /etc/hostname readable")
		return 1
	}

	// The staged workspace is the one writable host-derived path.
	if err := os.WriteFile(filepath.Join(SandboxWorkspacePath, "probe.txt"), []byte("ok"), 0o600); err != nil {
		fmt.Println("FAIL workspace not writable:", err)
		return 1
	}

	// no_new_privs must be set (prctl PR_GET_NO_NEW_PRIVS == 1).
	if got := getNoNewPrivs(); got != 1 {
		fmt.Println("FAIL no_new_privs not set, got", got)
		return 1
	}

	fmt.Println("SANDBOX_OK")
	return 0
}

func TestSandboxIsolation(t *testing.T) {
	if ok, reason := SandboxSupported(); !ok {
		t.Skip("sandbox unsupported here: " + reason)
	}

	dir := t.TempDir()
	workspace := filepath.Join(dir, "workspace")
	output := filepath.Join(dir, "out")
	modcache := filepath.Join(dir, "modcache")
	for _, d := range []string{workspace, output, modcache} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// A host-only secret the sandbox must not be able to read.
	secret := filepath.Join(dir, "host-secret.txt")
	if err := os.WriteFile(secret, []byte("TOP-SECRET"), 0o600); err != nil {
		t.Fatal(err)
	}

	spec := SandboxSpec{
		WorkspaceRoot: workspace,
		OutputDir:     output,
		GoRoot:        runtime.GOROOT(),
		GoModCache:    modcache,
		MaxOpenFiles:  4096,
		MaxTmpfsBytes: 64 << 20, // exercise the size-bounded tmpfs mount path
	}
	encoded, err := EncodeSandboxSpec(spec)
	if err != nil {
		t.Fatal(err)
	}

	var out strings.Builder
	env := []string{
		"GOWDK_SANDBOX_CHILD=1",
		"GOWDK_SANDBOX_SPEC=" + encoded,
		"GOWDK_SANDBOX_SECRET=" + secret,
	}
	// Run the probe with no test selected; TestMain exits before the suite runs.
	err = LaunchSandbox(spec, os.Args[0], []string{"-test.run=^$"}, env, &out, &out, 30*time.Second)
	// Some environments (e.g. CI runners with AppArmor's
	// kernel.apparmor_restrict_unprivileged_userns) report a positive
	// max_user_namespaces yet still deny the namespace clone at launch. That is a
	// "sandbox unavailable" condition, not a test failure: skip rather than fail.
	if errors.Is(err, ErrSandboxUnsupported) {
		t.Skip("sandbox cannot be created in this environment: " + err.Error())
	}
	if err != nil {
		t.Fatalf("sandbox probe failed: %v\noutput:\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "SANDBOX_OK") {
		t.Fatalf("expected SANDBOX_OK, got:\n%s", out.String())
	}

	// The probe wrote into the staged workspace; that write must be visible on
	// the host bind source and nowhere else.
	if _, err := os.Stat(filepath.Join(workspace, "probe.txt")); err != nil {
		t.Fatalf("expected workspace write to surface on host: %v", err)
	}
}

func TestSandboxSupportedReportsReason(t *testing.T) {
	ok, reason := SandboxSupported()
	if !ok && reason == "" {
		t.Fatal("unsupported sandbox must explain why")
	}
}

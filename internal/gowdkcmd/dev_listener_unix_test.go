//go:build unix

package gowdkcmd

import (
	"net"
	"path/filepath"
	"testing"
)

// TestDevServeStateHandsListenerToRuntimeChild verifies that the dev server
// reserves the runtime port up front, keeps it bound for the session, and hands
// the bound socket to the generated child via GOWDK_LISTENER_FD. This is the
// behavior that closes the port-allocation TOCTOU window (#543).
func TestDevServeStateHandsListenerToRuntimeChild(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	writeCLIFile(t, filepath.Join(appDir, "go.mod"), "module example.com/devapp\n\ngo 1.24\n")
	// Sleep instead of blocking on a bare select so the helper stays alive
	// without tripping Go's deadlock detector before the test observes it.
	writeCLIFile(t, filepath.Join(appDir, "cmd", "server", "main.go"), `package main

import "time"

func main() {
	time.Sleep(time.Minute)
}
`)
	binaryPath := filepath.Join(root, "bin", "devapp")

	serve := newDevServeState("127.0.0.1:0")
	t.Cleanup(serve.close)

	runtime := devRuntime{Enabled: true, AppDir: appDir, BinaryPath: binaryPath}
	if err := serve.useRuntime(runtime); err != nil {
		t.Fatal(err)
	}

	if serve.process == nil || serve.process.listener == nil {
		t.Fatal("expected the dev runtime to reserve a held listener on unix")
	}
	if got, want := serve.process.listener.Addr().String(), serve.process.addr; got != want {
		t.Fatalf("reserved listener addr %q != runtime addr %q", got, want)
	}

	command := activeDevRuntimeCommand(t, serve.process)
	if len(command.ExtraFiles) != 1 {
		t.Fatalf("expected exactly one inherited descriptor passed to the child, got %d", len(command.ExtraFiles))
	}
	if !hasEnvValue(command.Env, "GOWDK_LISTENER_FD=3") {
		t.Fatalf("expected child env GOWDK_LISTENER_FD=3, env=%#v", command.Env)
	}
	if !hasEnvValue(command.Env, "GOWDK_ADDR="+serve.process.addr) {
		t.Fatalf("expected child env GOWDK_ADDR=%q, env=%#v", serve.process.addr, command.Env)
	}

	// The reserved port stays bound for the whole session, so an independent
	// bind on it must fail — there is no window in which it is free.
	if listener, err := net.Listen("tcp", serve.process.addr); err == nil {
		listener.Close()
		t.Fatalf("expected reserved port %q to stay held by the dev session", serve.process.addr)
	}
}

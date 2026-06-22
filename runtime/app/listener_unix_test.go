//go:build unix

package app

import (
	"net"
	"strconv"
	"syscall"
	"testing"
)

// TestInheritedListenerRoundTrip verifies that a descriptor passed via
// GOWDK_LISTENER_FD yields a working listener bound to the same socket, which is
// how the dev server hands a pre-bound port to the generated child.
func TestInheritedListenerRoundTrip(t *testing.T) {
	base, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()

	file, err := base.(*net.TCPListener).File()
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	// Hand inheritedListener its own duplicate so the descriptor it consumes and
	// closes is independent of the ones this test owns.
	dupFD, err := syscall.Dup(int(file.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOWDK_LISTENER_FD", strconv.Itoa(dupFD))

	got, err := inheritedListener()
	if err != nil {
		t.Fatalf("inheritedListener: %v", err)
	}
	if got == nil {
		t.Fatal("expected a listener from an inherited descriptor")
	}
	defer got.Close()

	if got.Addr().String() != base.Addr().String() {
		t.Fatalf("inherited listener addr = %q, want %q", got.Addr(), base.Addr())
	}

	accepted := make(chan error, 1)
	go func() {
		conn, err := got.Accept()
		if err != nil {
			accepted <- err
			return
		}
		conn.Close()
		accepted <- nil
	}()

	client, err := net.Dial("tcp", got.Addr().String())
	if err != nil {
		t.Fatalf("dial inherited listener: %v", err)
	}
	client.Close()
	if err := <-accepted; err != nil {
		t.Fatalf("accept on inherited listener: %v", err)
	}
}

package app

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// inheritedListenerEnv names the environment variable a parent process uses to
// hand a pre-bound listening socket to the generated binary. When set, it holds
// the file descriptor (numbered by the parent, e.g. via exec.Cmd.ExtraFiles)
// of an already-bound, already-listening socket.
const inheritedListenerEnv = "GOWDK_LISTENER_FD"

// inheritedListener returns a listener wrapping a file descriptor handed down by
// a parent process via GOWDK_LISTENER_FD, or (nil, nil) when no descriptor was
// provided. The dev server uses this to keep a probed port reserved across
// restarts: the parent binds the socket once and keeps it open, closing the
// window in which the port could be reclaimed between probing and binding.
//
// The descriptor is consumed at most once per process; the variable is cleared
// so a child spawned by this process does not attempt to reuse the same fd.
func inheritedListener() (net.Listener, error) {
	raw := strings.TrimSpace(os.Getenv(inheritedListenerEnv))
	if raw == "" {
		return nil, nil
	}
	os.Unsetenv(inheritedListenerEnv)

	fd, err := strconv.Atoi(raw)
	if err != nil || fd < 0 {
		return nil, fmt.Errorf("invalid %s %q", inheritedListenerEnv, raw)
	}

	file := os.NewFile(uintptr(fd), "gowdk-inherited-listener")
	if file == nil {
		return nil, fmt.Errorf("invalid %s %q", inheritedListenerEnv, raw)
	}
	// net.FileListener duplicates the descriptor, so the wrapper File is no
	// longer needed once the listener is built.
	listener, err := net.FileListener(file)
	if closeErr := file.Close(); closeErr != nil && err == nil {
		// The listener is usable; only surface the close failure if listening
		// itself succeeded.
		err = fmt.Errorf("close inherited listener fd: %w", closeErr)
		if listener != nil {
			_ = listener.Close()
			listener = nil
		}
	}
	if err != nil {
		return nil, fmt.Errorf("listen on inherited fd %d: %w", fd, err)
	}
	return listener, nil
}

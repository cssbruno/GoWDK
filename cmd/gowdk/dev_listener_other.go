//go:build !unix

package main

import (
	"fmt"
	"net"
	"os"
)

// acquireDevRuntimeListener is a no-op off Unix because exec.Cmd.ExtraFiles
// descriptor inheritance (used to hand the bound socket to the child) is not
// supported there, notably on Windows. Returning (nil, nil) makes the dev
// server fall back to probing a free port via freeDevRuntimeAddr.
func acquireDevRuntimeListener() (net.Listener, error) {
	return nil, nil
}

// listenerInheritFile is never reached off Unix because acquireDevRuntimeListener
// returns no listener; it exists so the dev server compiles on every platform.
func listenerInheritFile(net.Listener) (*os.File, error) {
	return nil, fmt.Errorf("listener descriptor inheritance is not supported on this platform")
}

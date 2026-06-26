//go:build unix

package gowdkcmd

import (
	"fmt"
	"net"
	"os"
)

// acquireDevRuntimeListener binds a loopback listener and keeps it open for the
// lifetime of the dev session so the OS-assigned port stays reserved. The bound
// socket is handed to each generated child process through a duplicated file
// descriptor (see listenerInheritFile), eliminating the TOCTOU window that a
// probe-then-close approach leaves between choosing a port and binding it.
//
// It returns (nil, nil) only on platforms without descriptor inheritance; on
// Unix a non-nil listener is always returned on success.
func acquireDevRuntimeListener() (net.Listener, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	return listener, nil
}

// listenerInheritFile returns a duplicated *os.File for the listener's bound
// socket so it can be passed to a child process via exec.Cmd.ExtraFiles. The
// caller owns the returned file and must close it after starting the child; the
// child inherits its own duplicate.
func listenerInheritFile(listener net.Listener) (*os.File, error) {
	tcp, ok := listener.(*net.TCPListener)
	if !ok {
		return nil, fmt.Errorf("dev runtime listener is %T, want *net.TCPListener", listener)
	}
	return tcp.File()
}

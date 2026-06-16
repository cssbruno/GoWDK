//go:build !linux

package playground

import (
	"fmt"
	"io"
	"time"
)

// SandboxSupported always reports false off Linux: the namespace, mount, and
// pivot_root primitives this sandbox relies on are Linux-specific.
func SandboxSupported() (bool, string) {
	return false, "the playground sandbox requires Linux namespaces"
}

// LaunchSandbox fails closed on non-Linux platforms.
func LaunchSandbox(_ SandboxSpec, _ string, _ []string, _ []string, _, _ io.Writer, _ time.Duration) error {
	return fmt.Errorf("%w: only Linux is supported", ErrSandboxUnsupported)
}

// ConfineToSandbox fails closed on non-Linux platforms.
func ConfineToSandbox(_ SandboxSpec) error {
	return fmt.Errorf("%w: only Linux is supported", ErrSandboxUnsupported)
}

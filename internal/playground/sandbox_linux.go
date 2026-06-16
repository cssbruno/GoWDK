//go:build linux

package playground

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// prSetNoNewPrivs is the prctl option that prevents a process and its children
// from ever gaining privileges through execve (setuid binaries, file
// capabilities). It is not exported by the standard syscall package.
const prSetNoNewPrivs = 38

// prGetNoNewPrivs reads back the no_new_privs bit.
const prGetNoNewPrivs = 39

// rlimitNProc is RLIMIT_NPROC. The standard syscall package does not export it
// (it is not POSIX), so we define it here, matching the value used by Linux on
// the architectures gowdk targets (amd64, arm64, 386, arm, ppc64, riscv64,
// s390x). A handful of legacy architectures (mips, sparc, alpha, parisc) number
// the resource limits differently; gowdk does not ship for them.
const rlimitNProc = 6

// getNoNewPrivs returns the current no_new_privs bit (1 once it has been set).
func getNoNewPrivs() int {
	value, _, _ := syscall.Syscall(syscall.SYS_PRCTL, prGetNoNewPrivs, 0, 0)
	return int(value)
}

// SandboxSupported reports whether the kernel offers the namespaces this sandbox
// needs. It is intentionally conservative: a false result must make hosted
// execution fail closed rather than run unconfined.
//
// Reading max_user_namespaces alone is not enough: containers (notably the CI
// runners) report a positive limit yet still deny the unprivileged clone via
// seccomp/AppArmor, so the value lies about what actually works. We therefore
// follow the cheap sysfs check with a real clone probe and only report support
// when the namespaces can genuinely be created.
func SandboxSupported() (bool, string) {
	data, err := os.ReadFile("/proc/sys/user/max_user_namespaces")
	if err != nil {
		return false, "unprivileged user namespaces are unavailable (cannot read max_user_namespaces)"
	}
	max, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || max <= 0 {
		return false, "unprivileged user namespaces are disabled (max_user_namespaces=0)"
	}
	if err := probeNamespaces(); err != nil {
		return false, "the kernel denied creating the sandbox namespaces: " + err.Error()
	}
	return true, ""
}

// probeNamespaces verifies that the unprivileged clone the sandbox depends on is
// actually permitted, without running a build. It launches a child with the same
// namespace clone flags pointed at a path that cannot exist: the kernel creates
// the namespaces *before* execve, so a "no such file" failure proves the clone
// succeeded, whereas an EPERM (or any other clone-stage failure) means the
// environment forbids it. The child never runs any code.
func probeNamespaces() error {
	cmd := exec.Command(noNamespaceProbeBinary)
	cmd.SysProcAttr = sandboxSysProcAttr()
	err := cmd.Run()
	if err == nil || errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOENT) {
		// The clone (and uid/gid map writes) succeeded; only the exec failed.
		return nil
	}
	return err
}

// noNamespaceProbeBinary is an absolute path that is guaranteed not to exist, so
// the probe's exec always fails after the namespaces have been created.
const noNamespaceProbeBinary = "/nonexistent/gowdk-sandbox-namespace-probe"

// sandboxSysProcAttr returns the namespace clone configuration shared by the real
// sandbox launch and the support probe, so the probe tests exactly what the
// launch will attempt.
func sandboxSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWIPC |
			syscall.CLONE_NEWUTS,
		// Keep mounts from propagating back to the host namespace.
		Unshareflags: syscall.CLONE_NEWNS,
		// Map the caller to uid/gid 0 *inside* the user namespace so the child
		// can mount and pivot_root, while remaining unprivileged on the host.
		UidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings:                []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
		GidMappingsEnableSetgroups: false,
	}
}

// LaunchSandbox runs childArgs (the re-executed gowdk build) inside fresh user,
// mount, PID, network, IPC, and UTS namespaces. The network namespace has no
// configured interface, so the child has no network. The wall-clock timeout
// kills the child; because it is PID 1 of its namespace, the kernel reaps the
// whole process tree with it.
func LaunchSandbox(spec SandboxSpec, childPath string, childArgs []string, env []string, stdout, stderr io.Writer, timeout time.Duration) error {
	if ok, reason := SandboxSupported(); !ok {
		return fmt.Errorf("%w: %s", ErrSandboxUnsupported, reason)
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, childPath, childArgs...)
	cmd.Env = env
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.SysProcAttr = sandboxSysProcAttr()

	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("playground sandbox exceeded %s wall-clock limit", timeout)
	}
	// A clone-stage EPERM means the kernel refused the namespaces after our
	// support check passed (e.g. a policy change between probe and launch). Fail
	// closed on the documented error rather than surfacing a raw "operation not
	// permitted" that callers cannot classify.
	if err != nil && errors.Is(err, syscall.EPERM) {
		return fmt.Errorf("%w: the kernel denied creating the sandbox namespaces", ErrSandboxUnsupported)
	}
	// The child exits with SandboxUnsupportedExitCode when the namespaces were
	// created but confinement (a required mount, pivot_root, etc.) was denied
	// inside them. Map it back so callers fail closed on the documented error.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == SandboxUnsupportedExitCode {
		return fmt.Errorf("%w: the kernel denied establishing confinement inside the namespaces", ErrSandboxUnsupported)
	}
	return err
}

// ConfineToSandbox is run by the re-executed child, already inside the new
// namespaces. It builds a minimal root that exposes only the toolchain, a
// throwaway module-cache overlay, the staged workspace, and the output
// directory, then pivot_roots into it so the host filesystem becomes
// unreachable. Finally it applies resource limits and no-new-privileges. Any
// failure is returned so the caller aborts; partial confinement never runs the
// build.
//
// Some environments create the namespaces but then deny a required mount inside
// them (a container with a positive max_user_namespaces but a restrictive mount
// policy reports exactly this: "mount proc: operation not permitted"). Such a
// failure means the sandbox cannot be established here, not that the build is
// broken, so it is reported as ErrSandboxUnsupported and the child exits with
// SandboxUnsupportedExitCode so the parent fails closed cleanly.
func ConfineToSandbox(spec SandboxSpec) error {
	if err := confine(spec); err != nil {
		if isUnsupportedErrno(err) {
			return fmt.Errorf("%w: %v", ErrSandboxUnsupported, err)
		}
		return err
	}
	return nil
}

// isUnsupportedErrno reports whether a confinement error is the environment
// refusing a privileged operation (rather than a genuine bug). These are the
// errnos the kernel returns when namespaces, mounts, or pivot_root are blocked
// by policy or unavailable.
func isUnsupportedErrno(err error) bool {
	return errors.Is(err, syscall.EPERM) ||
		errors.Is(err, syscall.EACCES) ||
		errors.Is(err, syscall.ENOSYS) ||
		errors.Is(err, syscall.EOPNOTSUPP) ||
		errors.Is(err, syscall.ENODEV)
}

func confine(spec SandboxSpec) error {
	// 1. Make every mount in this namespace private so nothing propagates out.
	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("make mounts private: %w", err)
	}

	// 2. A fresh tmpfs becomes the new root. Mounting it makes it a mount point,
	// which pivot_root requires.
	newRoot, err := os.MkdirTemp("", "gowdk-sandbox-root-")
	if err != nil {
		return err
	}
	if err := syscall.Mount("tmpfs", newRoot, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, ""); err != nil {
		return fmt.Errorf("mount sandbox root tmpfs: %w", err)
	}

	in := func(p string) string { return filepath.Join(newRoot, p) }
	for _, dir := range []string{
		SandboxGoRootPath, SandboxGoModCachePath, SandboxWorkspacePath,
		SandboxOutputPath, SandboxGoCachePath, SandboxTmpPath, "/proc", "/dev", "/oldroot",
	} {
		if err := os.MkdirAll(in(dir), 0o755); err != nil {
			return err
		}
	}

	// 3. Read-only toolchain.
	if err := bindMount(spec.GoRoot, in(SandboxGoRootPath), true); err != nil {
		return fmt.Errorf("bind GOROOT: %w", err)
	}

	// 4. Module cache as a throwaway overlay: reads fall through to the host
	// cache (read-only lower), writes land on a tmpfs upper that is discarded
	// with the sandbox. This lets offline builds resolve cached modules without
	// failing on lock-file writes and without persisting anything to the host.
	// The overlay's writable upper/work dirs must live on the sandbox tmpfs
	// (newRoot), never the host's /tmp: at this point pivot_root has not run, so
	// os.MkdirTemp("") would land on the real host filesystem and the build's
	// cache writes would persist there after the sandbox exits.
	if err := mountModCacheOverlay(spec.GoModCache, in(SandboxGoModCachePath), newRoot); err != nil {
		// Fall back to a read-only bind: isolation is preserved; a build that
		// needs to write the cache will fail, which is safe.
		if bindErr := bindMount(spec.GoModCache, in(SandboxGoModCachePath), true); bindErr != nil {
			return fmt.Errorf("mount module cache: overlay: %v; bind: %w", err, bindErr)
		}
	}

	// 5. Writable workspace, output, build cache, and tmp.
	if err := bindMount(spec.WorkspaceRoot, in(SandboxWorkspacePath), false); err != nil {
		return fmt.Errorf("bind workspace: %w", err)
	}
	if err := bindMount(spec.OutputDir, in(SandboxOutputPath), false); err != nil {
		return fmt.Errorf("bind output: %w", err)
	}
	if err := mountTmpfs(in(SandboxGoCachePath)); err != nil {
		return err
	}
	if err := mountTmpfs(in(SandboxTmpPath)); err != nil {
		return err
	}

	// 6. A private proc for the new PID namespace, and a minimal /dev.
	if err := syscall.Mount("proc", in("/proc"), "proc", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, ""); err != nil {
		return fmt.Errorf("mount proc: %w", err)
	}
	if err := mountMinimalDev(in("/dev")); err != nil {
		return fmt.Errorf("mount /dev: %w", err)
	}

	// 7. Pivot into the new root and detach the old one so the host filesystem
	// is gone from this mount namespace.
	if err := os.Chdir(newRoot); err != nil {
		return err
	}
	if err := syscall.PivotRoot(".", "oldroot"); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return err
	}
	if err := syscall.Unmount("/oldroot", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("detach old root: %w", err)
	}
	if err := os.Remove("/oldroot"); err != nil {
		return fmt.Errorf("remove old root mountpoint: %w", err)
	}

	// 8. Resource limits and no-new-privileges.
	if err := applyRlimits(spec); err != nil {
		return err
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0); errno != 0 {
		return fmt.Errorf("set no_new_privs: %v", errno)
	}
	return nil
}

func bindMount(source, target string, readOnly bool) error {
	if err := syscall.Mount(source, target, "", syscall.MS_BIND|syscall.MS_REC, ""); err != nil {
		return err
	}
	if readOnly {
		// A read-only bind requires a second remount; the flag is ignored on the
		// initial bind.
		const flags = syscall.MS_BIND | syscall.MS_REC | syscall.MS_REMOUNT | syscall.MS_RDONLY | syscall.MS_NOSUID
		if err := syscall.Mount("", target, "", flags, ""); err != nil {
			return err
		}
	}
	return nil
}

func mountTmpfs(target string) error {
	return syscall.Mount("tmpfs", target, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "")
}

// mountModCacheOverlay mounts the host module cache (read-only lower) under a
// throwaway writable overlay. The upper and work dirs are created under
// scratchRoot, which must be the sandbox's tmpfs root: writes are then discarded
// with the namespace and never touch the host filesystem.
func mountModCacheOverlay(lower, target, scratchRoot string) error {
	if strings.TrimSpace(lower) == "" {
		return fmt.Errorf("empty module cache lowerdir")
	}
	scratch := filepath.Join(scratchRoot, ".modcache-overlay")
	upper := filepath.Join(scratch, "upper")
	work := filepath.Join(scratch, "work")
	for _, dir := range []string{upper, work} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work)
	return syscall.Mount("overlay", target, "overlay", syscall.MS_NOSUID|syscall.MS_NODEV, opts)
}

// mountMinimalDev provides only the character devices the Go toolchain needs.
// No block devices, no host devices, nothing that exposes data.
func mountMinimalDev(target string) error {
	if err := syscall.Mount("tmpfs", target, "tmpfs", syscall.MS_NOSUID, "mode=0755"); err != nil {
		return err
	}
	for _, node := range []string{"null", "zero", "full", "random", "urandom", "tty"} {
		hostNode := "/dev/" + node
		if _, err := os.Stat(hostNode); err != nil {
			continue
		}
		targetNode := filepath.Join(target, node)
		if file, err := os.OpenFile(targetNode, os.O_CREATE, 0o666); err == nil {
			file.Close()
		}
		if err := syscall.Mount(hostNode, targetNode, "", syscall.MS_BIND, ""); err != nil {
			return fmt.Errorf("bind /dev/%s: %w", node, err)
		}
	}
	return nil
}

func applyRlimits(spec SandboxSpec) error {
	limits := []struct {
		resource int
		value    uint64
	}{
		{syscall.RLIMIT_AS, spec.MaxAddressSpaceBytes},
		{syscall.RLIMIT_CPU, spec.MaxCPUSeconds},
		{syscall.RLIMIT_FSIZE, spec.MaxFileSizeBytes},
		{syscall.RLIMIT_NOFILE, spec.MaxOpenFiles},
		// RLIMIT_NPROC is counted per (user namespace, uid); the child runs in a
		// fresh user namespace, so this caps only processes the build spawns, not
		// the host. A cgroup v2 pids.max remains the stronger follow-up (see #459).
		{rlimitNProc, spec.MaxProcesses},
	}
	for _, limit := range limits {
		if limit.value == 0 {
			continue
		}
		rlimit := &syscall.Rlimit{Cur: limit.value, Max: limit.value}
		if err := syscall.Setrlimit(limit.resource, rlimit); err != nil {
			return fmt.Errorf("set rlimit %d: %w", limit.resource, err)
		}
	}
	return nil
}

package gowdkcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cssbruno/gowdk/internal/playground"
)

const playgroundUsage = "usage: gowdk playground policy [--json] | gowdk playground export --dir <project> --out <project.zip> [--json] | gowdk playground run --dir <project> --out <dir> --allow-hosted-execution (--module-cache <dir> | --allow-shared-module-cache)"

// sandboxBuildSubcommand is the hidden re-exec target. gowdk launches itself
// with this argument inside the sandbox namespaces; it is intentionally absent
// from usage and must not be run directly.
const sandboxBuildSubcommand = "__sandbox-build"

func playgroundCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(playgroundUsage)
	}
	switch args[0] {
	case "policy":
		return playgroundPolicy(args[1:])
	case "export":
		return playgroundExport(args[1:])
	case "run":
		return playgroundRun(args[1:])
	case sandboxBuildSubcommand:
		return playgroundSandboxBuild(args[1:])
	default:
		return fmt.Errorf(playgroundUsage)
	}
}

func playgroundPolicy(args []string) error {
	jsonOutput := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			return fmt.Errorf(playgroundUsage)
		}
	}
	policy := playground.DefaultPolicy()
	if jsonOutput {
		payload, err := playground.PolicyJSON(policy)
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	fmt.Println("Playground hosted execution: disabled by default")
	fmt.Println("Workspace:", policy.Workspace)
	fmt.Println("Network:", policy.Network)
	fmt.Println("Filesystem:", policy.Filesystem)
	fmt.Printf("Limits: %d files, %d bytes per file, %d bytes total, %d seconds\n", policy.Limits.MaxFiles, policy.Limits.MaxFileBytes, policy.Limits.MaxTotalBytes, policy.Limits.WallClockSeconds)
	fmt.Println("Export:", policy.Export)
	return nil
}

func playgroundExport(args []string) error {
	options, err := parsePlaygroundFileOptions(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(options.Output) == "" {
		return fmt.Errorf(playgroundUsage)
	}
	result, err := playground.ExportArchive(options.Dir, options.Output, playground.Options{})
	if err != nil {
		return err
	}
	if options.JSON {
		payload, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	fmt.Println(result.Archive)
	for _, file := range result.Files {
		fmt.Println(file.Path)
	}
	return nil
}

func playgroundRun(args []string) error {
	options, err := parsePlaygroundFileOptions(args)
	if err != nil {
		return err
	}
	if options.JSON {
		return fmt.Errorf("--json is not supported by playground run")
	}
	if !options.AllowExecution {
		return fmt.Errorf("hosted playground execution is disabled by default; pass --allow-hosted-execution only inside the documented sandbox")
	}
	if strings.TrimSpace(options.Output) == "" {
		return fmt.Errorf(playgroundUsage)
	}

	// Fail closed: hosted execution only runs inside the OS-level sandbox. If
	// the kernel cannot provide it, refuse rather than run the build unconfined.
	if ok, reason := playground.SandboxSupported(); !ok {
		return fmt.Errorf("hosted playground execution requires the OS-level sandbox, which is unavailable here: %s", reason)
	}

	workspace, cleanup, err := playground.StageWorkspace(options.Dir, playground.Options{})
	if err != nil {
		return err
	}
	defer cleanup()

	outputDir, err := filepath.Abs(options.Output)
	if err != nil {
		return err
	}
	if err := validateOutputDir(outputDir); err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	goRoot, err := resolveGoRoot()
	if err != nil {
		return err
	}
	goModCache, err := resolveSandboxModuleCache(options, goRoot)
	if err != nil {
		return err
	}

	spec := playground.SandboxSpec{
		WorkspaceRoot:        workspace.Root,
		OutputDir:            outputDir,
		GoRoot:               goRoot,
		GoModCache:           goModCache,
		MaxAddressSpaceBytes: 2 << 30,   // 2 GiB
		MaxCPUSeconds:        60,        // 60 CPU-seconds
		MaxFileSizeBytes:     256 << 20, // 256 MiB per file
		MaxOpenFiles:         4096,
		// RLIMIT_NPROC caps the per-uid process count. It is enforced against the
		// build's exec'd subprocesses (which run capless after the bounding-set
		// drop) only when gowdk runs as a non-root host user; when gowdk runs as
		// host root the build maps to global uid 0, which the kernel exempts, so a
		// hosted runner must add an outer pids cgroup. See the threat model.
		MaxProcesses:  256,
		MaxTmpfsBytes: 2 << 30, // 2 GiB per writable tmpfs; outer cgroup bounds total memory
	}
	encoded, err := playground.EncodeSandboxSpec(spec)
	if err != nil {
		return err
	}

	// Re-execute this binary inside the sandbox namespaces. The synthesized
	// environment carries no host variables, so no secrets leak in.
	return playground.LaunchSandbox(
		spec,
		"/proc/self/exe",
		[]string{"playground", sandboxBuildSubcommand, encoded},
		playground.SandboxEnvironment(),
		os.Stdout,
		os.Stderr,
		playground.ExecutionTimeout(),
	)
}

// playgroundSandboxBuild runs inside the sandbox namespaces. It confines itself
// (no network, no host filesystem, dropped privileges, resource limits) and then
// builds the staged workspace against the fixed in-sandbox paths.
func playgroundSandboxBuild(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("internal: %s requires one encoded sandbox spec", sandboxBuildSubcommand)
	}
	spec, err := playground.DecodeSandboxSpec(args[0])
	if err != nil {
		return err
	}
	if err := playground.ConfineToSandbox(spec); err != nil {
		// Confinement was denied by the environment (e.g. a blocked mount inside
		// the namespace). Signal the parent with the sentinel exit code so it
		// fails closed as "sandbox unavailable" rather than an opaque error; the
		// build never runs unconfined because it is sequenced after this point.
		if errors.Is(err, playground.ErrSandboxUnsupported) {
			os.Exit(playground.SandboxUnsupportedExitCode)
		}
		return fmt.Errorf("sandbox confinement failed: %w", err)
	}

	os.Clearenv()
	for _, item := range playground.SandboxEnvironment() {
		name, value, ok := strings.Cut(item, "=")
		if ok {
			_ = os.Setenv(name, value)
		}
	}

	// Root the build at the workspace so source/CSS/asset discovery stays inside
	// the project and never walks the toolchain or module-cache mounts.
	if err := os.Chdir(playground.SandboxWorkspacePath); err != nil {
		return fmt.Errorf("enter sandbox workspace: %w", err)
	}

	return build([]string{
		"--config", "gowdk.config.go",
		"--out", playground.SandboxOutputPath,
	})
}

// resolveGoRoot returns the GOROOT to expose read-only inside the sandbox.
//
// It comes from runtime.GOROOT(), which is baked into this binary at build time
// and so cannot be redirected by an attacker-controlled PATH; the toolchain is
// then addressed by absolute path rather than a PATH lookup, since this runs
// before any namespace/pivot confinement and a hosted wrapper may have an
// attacker-writable directory on PATH.
func resolveGoRoot() (string, error) {
	goRoot := strings.TrimSpace(runtime.GOROOT())
	if goRoot == "" {
		return "", fmt.Errorf("could not resolve GOROOT for the sandbox toolchain")
	}
	if _, err := os.Stat(filepath.Join(goRoot, "bin", "go")); err != nil {
		return "", fmt.Errorf("go toolchain not found at %s: %w", filepath.Join(goRoot, "bin", "go"), err)
	}
	return goRoot, nil
}

// resolveSandboxModuleCache decides which module cache to expose. The sandbox
// mounts the lower layer readable, so submitted build code can read every module
// in it. On a hosted runner that reuses the shared host GOMODCACHE across
// sessions, that would leak other tenants' cached (possibly private) modules.
// We therefore require an explicit choice: --module-cache <dir> names a
// per-session cache the caller has scoped, and --allow-shared-module-cache is the
// opt-in for local use of the host cache. With neither, hosted execution fails
// closed rather than silently exposing the shared cache.
func resolveSandboxModuleCache(options playgroundFileOptions, goRoot string) (string, error) {
	if dir := strings.TrimSpace(options.ModuleCache); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("module cache %q is not accessible: %w", dir, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("module cache %q is not a directory", dir)
		}
		return abs, nil
	}
	if !options.AllowSharedModuleCache {
		return "", fmt.Errorf("hosted playground execution will not mount the shared host module cache, because submitted build code could read every cached module: pass --module-cache <dir> with a per-session cache, or --allow-shared-module-cache to deliberately expose the host GOMODCACHE")
	}
	out, err := exec.Command(filepath.Join(goRoot, "bin", "go"), "env", "GOMODCACHE").Output()
	if err != nil {
		return "", fmt.Errorf("could not resolve GOMODCACHE for the sandbox: %w", err)
	}
	cache := strings.TrimSpace(string(out))
	if cache == "" {
		return "", fmt.Errorf("GOMODCACHE is empty; the sandbox needs a module cache to build offline")
	}
	fmt.Fprintln(os.Stderr, "warning: exposing the shared host GOMODCACHE to the sandbox; submitted build code can read every cached module. Use --module-cache with a per-session cache on shared runners.")
	return cache, nil
}

// validateOutputDir rejects output destinations broad enough to expose host data
// through the writable /out bind: the filesystem root, or any directory that
// already contains files (which could be the project root, /tmp, or a home
// directory). Hosted runners should pass a fresh, empty, service-owned directory.
func validateOutputDir(outputDir string) error {
	clean := filepath.Clean(outputDir)
	if clean == filepath.Dir(clean) {
		return fmt.Errorf("refusing to use the filesystem root %q as the playground output directory; use a dedicated empty directory", clean)
	}
	entries, err := os.ReadDir(clean)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) > 0 {
		return fmt.Errorf("playground output directory %q must be empty (found %d existing entries); use a fresh per-run directory", clean, len(entries))
	}
	return nil
}

type playgroundFileOptions struct {
	Dir                    string
	Output                 string
	JSON                   bool
	AllowExecution         bool
	ModuleCache            string
	AllowSharedModuleCache bool
}

func parsePlaygroundFileOptions(args []string) (playgroundFileOptions, error) {
	options := playgroundFileOptions{Dir: "."}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if value, next, ok, missing := consumeValueFlag(args, index, "--dir", true); ok {
			if missing {
				return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
			}
			options.Dir = value
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--out", true); ok {
			if missing {
				return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
			}
			options.Output = value
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--module-cache", true); ok {
			if missing {
				return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
			}
			options.ModuleCache = value
			index = next
			continue
		}
		switch {
		case arg == "--json":
			options.JSON = true
		case arg == "--allow-hosted-execution":
			options.AllowExecution = true
		case arg == "--allow-shared-module-cache":
			options.AllowSharedModuleCache = true
		default:
			return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
		}
	}
	if strings.TrimSpace(options.Dir) == "" {
		return playgroundFileOptions{}, fmt.Errorf("playground source directory is required")
	}
	return options, nil
}

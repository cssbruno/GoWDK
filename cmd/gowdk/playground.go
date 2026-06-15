package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cssbruno/gowdk/internal/playground"
)

const playgroundUsage = "usage: gowdk playground policy [--json] | gowdk playground export --dir <project> --out <project.zip> [--json] | gowdk playground run --dir <project> --out <dir> --allow-hosted-execution"

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
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}

	goRoot, goModCache, err := resolveGoToolchainPaths()
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

// resolveGoToolchainPaths returns the host GOROOT and module cache to expose
// (read-only / overlay) inside the sandbox so the toolchain can run offline.
func resolveGoToolchainPaths() (string, string, error) {
	goRoot := runtime.GOROOT()
	if strings.TrimSpace(goRoot) == "" {
		if out, err := exec.Command("go", "env", "GOROOT").Output(); err == nil {
			goRoot = strings.TrimSpace(string(out))
		}
	}
	if strings.TrimSpace(goRoot) == "" {
		return "", "", fmt.Errorf("could not resolve GOROOT for the sandbox toolchain")
	}
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		return "", "", fmt.Errorf("could not resolve GOMODCACHE for the sandbox: %w", err)
	}
	goModCache := strings.TrimSpace(string(out))
	if goModCache == "" {
		return "", "", fmt.Errorf("GOMODCACHE is empty; the sandbox needs a module cache to build offline")
	}
	return goRoot, goModCache, nil
}

type playgroundFileOptions struct {
	Dir            string
	Output         string
	JSON           bool
	AllowExecution bool
}

func parsePlaygroundFileOptions(args []string) (playgroundFileOptions, error) {
	options := playgroundFileOptions{Dir: "."}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--dir":
			index++
			if index >= len(args) {
				return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
			}
			options.Dir = args[index]
		case strings.HasPrefix(arg, "--dir="):
			options.Dir = strings.TrimPrefix(arg, "--dir=")
		case arg == "--out":
			index++
			if index >= len(args) {
				return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
			}
			options.Output = args[index]
		case strings.HasPrefix(arg, "--out="):
			options.Output = strings.TrimPrefix(arg, "--out=")
		case arg == "--json":
			options.JSON = true
		case arg == "--allow-hosted-execution":
			options.AllowExecution = true
		default:
			return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
		}
	}
	if strings.TrimSpace(options.Dir) == "" {
		return playgroundFileOptions{}, fmt.Errorf("playground source directory is required")
	}
	return options, nil
}

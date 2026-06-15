package playground

import (
	"encoding/json"
	"errors"
)

// ErrSandboxUnsupported is returned when OS-level confinement is not available
// on this platform or kernel. Hosted execution must fail closed on this error,
// never fall back to running the build unconfined.
var ErrSandboxUnsupported = errors.New("playground sandbox is not supported on this platform")

// SandboxSpec describes a confined build execution. All paths are absolute host
// paths; the sandbox remaps them to fixed locations inside the confined root.
type SandboxSpec struct {
	// WorkspaceRoot is the staged, disposable project copy (writable inside).
	WorkspaceRoot string `json:"workspaceRoot"`
	// OutputDir receives build output (writable inside).
	OutputDir string `json:"outputDir"`
	// GoRoot is the host GOROOT exposed read-only so the toolchain can run.
	GoRoot string `json:"goRoot"`
	// GoModCache is the host module cache exposed through a throwaway writable
	// overlay so offline builds resolve cached modules without persisting writes.
	GoModCache string `json:"goModCache"`
	// MaxAddressSpaceBytes caps the virtual address space (RLIMIT_AS). Zero
	// leaves the limit unset.
	MaxAddressSpaceBytes uint64 `json:"maxAddressSpaceBytes"`
	// MaxCPUSeconds caps CPU time (RLIMIT_CPU). Zero leaves it unset.
	MaxCPUSeconds uint64 `json:"maxCPUSeconds"`
	// MaxFileSizeBytes caps any single written file (RLIMIT_FSIZE). Zero leaves
	// it unset.
	MaxFileSizeBytes uint64 `json:"maxFileSizeBytes"`
	// MaxOpenFiles caps open descriptors (RLIMIT_NOFILE). Zero leaves it unset.
	MaxOpenFiles uint64 `json:"maxOpenFiles"`
	// MaxProcesses caps processes for the sandboxed user (RLIMIT_NPROC). Zero
	// leaves it unset.
	MaxProcesses uint64 `json:"maxProcesses"`
}

// Fixed in-sandbox mount points. The build always runs against these paths after
// confinement, regardless of where the host staged the inputs.
const (
	SandboxWorkspacePath  = "/workspace"
	SandboxOutputPath     = "/out"
	SandboxGoRootPath     = "/goroot"
	SandboxGoModCachePath = "/gomodcache"
	SandboxGoCachePath    = "/cache"
	SandboxTmpPath        = "/tmp"
)

// EncodeSandboxSpec serializes a spec for handoff to the re-executed child.
func EncodeSandboxSpec(spec SandboxSpec) (string, error) {
	data, err := json.Marshal(spec)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DecodeSandboxSpec parses a spec produced by EncodeSandboxSpec.
func DecodeSandboxSpec(encoded string) (SandboxSpec, error) {
	var spec SandboxSpec
	if err := json.Unmarshal([]byte(encoded), &spec); err != nil {
		return SandboxSpec{}, err
	}
	return spec, nil
}

// SandboxEnvironment returns the environment the build runs with inside the
// confined root: the toolchain and caches point at the fixed in-sandbox paths,
// the module proxy and checksum database are disabled, and no host environment
// leaks in. It is deliberately built from constants, not os.Environ().
func SandboxEnvironment() []string {
	return []string{
		"PATH=" + SandboxGoRootPath + "/bin",
		"HOME=" + SandboxTmpPath,
		"TMPDIR=" + SandboxTmpPath,
		"GOROOT=" + SandboxGoRootPath,
		"GOCACHE=" + SandboxGoCachePath,
		"GOMODCACHE=" + SandboxGoModCachePath,
		"GOPATH=" + SandboxTmpPath + "/gopath",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOFLAGS=-mod=mod",
		"GOTOOLCHAIN=local",
		"CGO_ENABLED=0",
	}
}

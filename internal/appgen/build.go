package appgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// BuildBinary compiles the generated app into binaryPath.
func BuildBinary(appDir, binaryPath string) (string, error) {
	if strings.TrimSpace(appDir) == "" {
		return "", fmt.Errorf("generated app directory is required")
	}
	if strings.TrimSpace(binaryPath) == "" {
		return "", fmt.Errorf("binary output path is required")
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return "", err
	}
	absBinary, err := filepath.Abs(binaryPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absBinary), 0o755); err != nil {
		return "", err
	}

	command := exec.Command("go", "build", "-buildvcs=false", "-o", absBinary, "./cmd/server")
	command.Dir = absApp
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build generated app failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return absBinary, nil
}

// BuildWASM compiles the generated app into a Go js/wasm artifact.
func BuildWASM(appDir, wasmPath string) (string, error) {
	if strings.TrimSpace(appDir) == "" {
		return "", fmt.Errorf("generated app directory is required")
	}
	if strings.TrimSpace(wasmPath) == "" {
		return "", fmt.Errorf("wasm output path is required")
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return "", err
	}
	absWASM, err := filepath.Abs(wasmPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(absWASM), 0o755); err != nil {
		return "", err
	}

	command := exec.Command("go", "build", "-buildvcs=false", "-o", absWASM, "./cmd/server")
	command.Dir = absApp
	command.Env = append(buildEnvWithout(os.Environ(), "GOOS", "GOARCH"), "GOOS=js", "GOARCH=wasm")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build generated wasm failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return absWASM, nil
}

func buildEnvWithout(env []string, names ...string) []string {
	blocked := map[string]bool{}
	for _, name := range names {
		blocked[name] = true
	}
	var filtered []string
	for _, entry := range env {
		name, _, ok := strings.Cut(entry, "=")
		if ok && blocked[name] {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

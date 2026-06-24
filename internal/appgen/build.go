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
	return buildGeneratedCommand(appDir, binaryPath, "./cmd/server", "generated app", "binary output path is required")
}

// BuildWorkerBinary compiles the generated contract worker app.
func BuildWorkerBinary(appDir, binaryPath string) (string, error) {
	return buildGeneratedCommand(appDir, binaryPath, "./cmd/worker", "generated worker app", "worker binary output path is required")
}

// BuildCronBinary compiles the generated contract cron app.
func BuildCronBinary(appDir, binaryPath string) (string, error) {
	return buildGeneratedCommand(appDir, binaryPath, "./cmd/cron", "generated cron app", "cron binary output path is required")
}

func buildGeneratedCommand(appDir, binaryPath string, packagePath string, label string, emptyBinaryMessage string) (string, error) {
	if strings.TrimSpace(appDir) == "" {
		return "", fmt.Errorf("%s directory is required", label)
	}
	if strings.TrimSpace(binaryPath) == "" {
		return "", fmt.Errorf("%s", emptyBinaryMessage)
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
	goEnv := generatedAppGoEnv(nil)
	if err := tidyGeneratedApp(absApp, goEnv); err != nil {
		return "", err
	}

	command := exec.Command("go", "build", "-buildvcs=false", "-o", absBinary, packagePath)
	command.Dir = absApp
	command.Env = goEnv
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build %s failed: %w\n%s", label, err, strings.TrimSpace(string(output)))
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
	wasmEnv := append(generatedAppGoEnv(buildEnvWithout(os.Environ(), "GOOS", "GOARCH")), "GOOS=js", "GOARCH=wasm")
	if err := tidyGeneratedApp(absApp, wasmEnv); err != nil {
		return "", err
	}

	command := exec.Command("go", "build", "-buildvcs=false", "-o", absWASM, "./cmd/server")
	command.Dir = absApp
	command.Env = wasmEnv
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("go build generated wasm failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return absWASM, nil
}

func tidyGeneratedApp(appDir string, env []string) error {
	command := exec.Command("go", "mod", "tidy")
	command.Dir = appDir
	if env != nil {
		command.Env = env
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod tidy generated app failed: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func generatedAppGoEnv(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	return append(buildEnvWithout(env, "GOWORK"), "GOWORK=off")
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

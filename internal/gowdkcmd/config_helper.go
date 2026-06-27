package gowdkcmd

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	goformat "go/format"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/cssbruno/gowdk/internal/project"
)

const (
	helperProtocolMin = 1
	helperProtocolMax = 1
	helperActiveEnv   = "GOWDK_HELPER_ACTIVE"
)

type helperPackage struct {
	ImportPath string `json:"ImportPath"`
	Name       string `json:"Name"`
	Dir        string `json:"Dir"`
	Module     *struct {
		Dir string `json:"Dir"`
	} `json:"Module"`
}

func runProjectHelperIfNeeded(args []string) (bool, error) {
	if os.Getenv(helperActiveEnv) == "1" || len(args) == 0 {
		return false, nil
	}
	if !projectHelperCommand(args[0]) {
		return false, nil
	}
	configPath, delegate, err := projectHelperConfigPath(args)
	if err != nil || !delegate {
		return delegate, err
	}
	helperPath, projectRoot, err := ensureProjectHelper(configPath)
	if err != nil {
		if helperUnavailable(err) {
			return false, nil
		}
		return true, err
	}
	helperArgs, err := normalizeProjectHelperArgs(args)
	if err != nil {
		return true, err
	}
	cmd := exec.Command(helperPath, helperArgs...)
	cmd.Dir = projectRoot
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		helperActiveEnv+"=1",
		fmt.Sprintf("GOWDK_HELPER_PROTOCOL_MIN=%d", helperProtocolMin),
		fmt.Sprintf("GOWDK_HELPER_PROTOCOL_MAX=%d", helperProtocolMax),
		"GOWDK_CLI_VERSION="+version,
	)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return true, helperExitError{code: exitErr.ExitCode()}
		}
		return true, err
	}
	return true, nil
}

func normalizeProjectHelperArgs(args []string) ([]string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	out := append([]string(nil), args...)
	switch out[0] {
	case "build", "check":
		return normalizeProjectCommandPathArgs(out, cwd, 1), nil
	case "dev":
		return normalizeDevHelperArgs(out, cwd), nil
	default:
		return out, nil
	}
}

func normalizeDevHelperArgs(args []string, cwd string) []string {
	out := append([]string(nil), args...)
	for index := 1; index < len(out); index++ {
		if _, next, ok, _ := consumeValueFlag(out, index, "--addr", true); ok {
			index = next
			continue
		}
		if _, next, ok, _ := consumeValueFlag(out, index, "--interval", true); ok {
			index = next
			continue
		}
		buildArgs := append([]string{"build"}, out[index:]...)
		buildArgs = normalizeProjectCommandPathArgs(buildArgs, cwd, 1)
		copy(out[index:], buildArgs[1:])
		return out
	}
	return out
}

func normalizeProjectCommandPathArgs(args []string, cwd string, start int) []string {
	out := append([]string(nil), args...)
	for index := start; index < len(out); index++ {
		if next, ok := normalizeHelperValueFlag(out, index, cwd, "--config"); ok {
			index = next
			continue
		}
		if next, ok := normalizeHelperValueFlag(out, index, cwd, "--env-file"); ok {
			index = next
			continue
		}
		if next, ok := skipProjectHelperValueFlag(out, index); ok {
			index = next
			continue
		}
		if skipProjectHelperBooleanFlag(out[index]) {
			continue
		}
		arg := out[index]
		if strings.HasPrefix(arg, "-") {
			continue
		}
		out[index] = absoluteExistingPath(cwd, arg)
	}
	return out
}

func normalizeHelperValueFlag(args []string, index int, cwd string, name string) (int, bool) {
	value, next, ok, missing := consumeValueFlag(args, index, name, true)
	if !ok || missing {
		return next, ok
	}
	normalized := absoluteExistingPath(cwd, value)
	if strings.HasPrefix(args[index], name+"=") {
		args[index] = name + "=" + normalized
	} else if next < len(args) {
		args[next] = normalized
	}
	return next, true
}

func skipProjectHelperValueFlag(args []string, index int) (int, bool) {
	for _, name := range []string{
		"--out",
		"--app",
		"--bin",
		"--docker-base",
		"--deploy-recipe",
		"--wasm",
		"--backend-app",
		"--backend-bin",
		"--worker-app",
		"--worker-bin",
		"--cron-app",
		"--cron-bin",
		"--target",
		"--module",
	} {
		if _, next, ok, _ := consumeValueFlag(args, index, name, name == "--deploy-recipe" || name == "--docker-base"); ok {
			return next, true
		}
	}
	return index, false
}

func skipProjectHelperBooleanFlag(arg string) bool {
	switch arg {
	case "--ssr",
		"--debug",
		"--timings",
		"--allow-missing-backend",
		"--allow-insecure",
		"--obfuscate-assets",
		"--docker",
		"--json",
		"--warnings-as-errors",
		"--standalone":
		return true
	}
	return strings.HasPrefix(arg, "--timings=") ||
		strings.HasPrefix(arg, "--allow-insecure=")
}

func absoluteExistingPath(cwd string, value string) string {
	if strings.TrimSpace(value) == "" || filepath.IsAbs(value) {
		return value
	}
	candidate := filepath.Join(cwd, value)
	if _, err := os.Stat(candidate); err != nil {
		return value
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return value
	}
	return absolute
}

func projectHelperCommand(command string) bool {
	return command == "build" || command == "check" || command == "dev"
}

func projectHelperConfigPath(args []string) (string, bool, error) {
	switch args[0] {
	case "build":
		plan, err := parseBuildOptions(args[1:])
		if err != nil {
			return "", true, err
		}
		return plan.ConfigPath, true, nil
	case "dev":
		options, err := parseDevOptions(args[1:])
		if err != nil {
			return "", true, err
		}
		plan, err := parseBuildOptions(options.BuildArgs)
		if err != nil {
			return "", true, err
		}
		return plan.ConfigPath, true, nil
	case "check":
		options, configPath, moduleNames, paths, err := parseProjectOptions(args[1:], "check", true)
		if err != nil {
			return "", true, err
		}
		standalone, err := shouldRunStandaloneCheck(options, configPath, moduleNames, paths)
		if err != nil {
			return "", true, err
		}
		if standalone {
			return "", false, nil
		}
		return configPath, true, nil
	default:
		return "", false, nil
	}
}

type helperExitError struct {
	code int
}

func (err helperExitError) Error() string {
	return fmt.Sprintf("project helper exited with code %d", err.code)
}

func (err helperExitError) ExitCode() int {
	return err.code
}

func (err helperExitError) SilentCLIError() {}

type helperUnavailableError struct {
	err error
}

func (err helperUnavailableError) Error() string {
	return err.err.Error()
}

func (err helperUnavailableError) Unwrap() error {
	return err.err
}

func helperUnavailable(err error) bool {
	_, ok := err.(helperUnavailableError)
	return ok
}

func ensureProjectHelper(configPath string) (helperPath string, projectRoot string, err error) {
	packageInfo, err := loadHelperConfigPackage(configPath)
	if err != nil {
		return "", "", err
	}
	source, err := projectHelperSource(packageInfo.ImportPath, packageInfo.Dir)
	if err != nil {
		return "", "", err
	}
	key, err := projectHelperCacheKey(packageInfo, configPath, source)
	if err != nil {
		return "", "", err
	}
	cacheDir := filepath.Join(packageInfo.Module.Dir, ".gowdk", "helper", key)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return "", "", err
	}
	mainPath := filepath.Join(cacheDir, "main.go")
	if err := os.WriteFile(mainPath, []byte(source), 0o644); err != nil {
		return "", "", err
	}
	binPath := filepath.Join(packageInfo.Module.Dir, ".gowdk", "helper", "gowdk-helper-"+key)
	if os.PathSeparator == '\\' {
		binPath += ".exe"
	}
	if _, err := os.Stat(binPath); err == nil {
		return binPath, packageInfo.Dir, nil
	} else if !os.IsNotExist(err) {
		return "", "", err
	}
	rel, err := filepath.Rel(packageInfo.Module.Dir, cacheDir)
	if err != nil {
		return "", "", err
	}
	cmd := exec.Command("go", "build", "-mod=mod", "-o", binPath, "./"+filepath.ToSlash(rel))
	cmd.Dir = packageInfo.Module.Dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("build GOWDK project helper: %w", err)
	}
	return binPath, packageInfo.Dir, nil
}

func loadHelperConfigPackage(configPath string) (helperPackage, error) {
	if strings.TrimSpace(configPath) == "" {
		configPath = project.DefaultConfigFile
	}
	absolute, err := filepath.Abs(configPath)
	if err != nil {
		return helperPackage{}, err
	}
	if _, err := os.Stat(absolute); err != nil {
		if os.IsNotExist(err) {
			return helperPackage{}, fmt.Errorf("%s is required; run \"gowdk init\" or pass --config <file>", project.DefaultConfigFile)
		}
		return helperPackage{}, err
	}
	if hasBuildConstraint, err := configFileHasBuildConstraint(absolute); err != nil {
		return helperPackage{}, err
	} else if hasBuildConstraint {
		return helperPackage{}, helperUnavailableError{err: fmt.Errorf("%s has build constraints and cannot be imported by the project helper", configPath)}
	}
	cmd := exec.Command("go", "list", "-json", ".")
	cmd.Dir = filepath.Dir(absolute)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		if strings.Contains(message, "go.mod file not found") {
			return helperPackage{}, helperUnavailableError{err: fmt.Errorf("go list config package: %s", message)}
		}
		return helperPackage{}, fmt.Errorf("go list config package: %s", message)
	}
	var packageInfo helperPackage
	if err := json.Unmarshal(output, &packageInfo); err != nil {
		return helperPackage{}, err
	}
	if packageInfo.ImportPath == "" {
		return helperPackage{}, fmt.Errorf("config package has no import path")
	}
	if packageInfo.Name == "main" {
		return helperPackage{}, fmt.Errorf("config package %s is package main and cannot be imported", packageInfo.ImportPath)
	}
	if packageInfo.Module == nil || packageInfo.Module.Dir == "" {
		return helperPackage{}, helperUnavailableError{err: fmt.Errorf("config package %s is not inside a Go module", packageInfo.ImportPath)}
	}
	return packageInfo, nil
}

func configFileHasBuildConstraint(path string) (bool, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(string(payload), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "//go:build ") || strings.HasPrefix(trimmed, "// +build "):
			return true, nil
		case strings.HasPrefix(trimmed, "//"):
			continue
		default:
			return false, nil
		}
	}
	return false, nil
}

func projectHelperSource(configImportPath string, projectRoot string) (string, error) {
	source := fmt.Sprintf(`package main

import (
	configpkg %q

	"github.com/cssbruno/gowdk/helperruntime"
)

func main() {
	helperruntime.Main(helperruntime.Options{
		Config: &configpkg.Config,
		ProjectRoot: %q,
	})
}
`, configImportPath, projectRoot)
	formatted, err := goformat.Source([]byte(source))
	if err != nil {
		return "", fmt.Errorf("format project helper source: %w", err)
	}
	return string(formatted), nil
}

func projectHelperCacheKey(packageInfo helperPackage, configPath string, source string) (string, error) {
	hash := sha256.New()
	writeHash := func(value string) {
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
	}
	writeHash(source)
	writeHash(packageInfo.ImportPath)
	writeHash(fmt.Sprintf("protocol=%d..%d", helperProtocolMin, helperProtocolMax))
	writeHash(version)
	writeHash(runtime.GOOS)
	writeHash(runtime.GOARCH)
	if goVersion, err := helperGoVersion(packageInfo.Module.Dir); err == nil {
		writeHash(goVersion)
	}
	for _, path := range helperCacheInputPaths(packageInfo, configPath) {
		writeHash(filepath.ToSlash(path))
		payload, err := os.ReadFile(path)
		if err == nil {
			_, _ = hash.Write(payload)
		}
		_, _ = hash.Write([]byte{0})
	}
	sum := hash.Sum(nil)
	return hex.EncodeToString(sum[:12]), nil
}

func helperGoVersion(dir string) (string, error) {
	cmd := exec.Command("go", "env", "GOVERSION")
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("go env GOVERSION: %s", message)
	}
	return strings.TrimSpace(string(output)), nil
}

func helperCacheInputPaths(packageInfo helperPackage, configPath string) []string {
	if strings.TrimSpace(configPath) == "" {
		configPath = filepath.Join(packageInfo.Dir, project.DefaultConfigFile)
	}
	var paths []string
	add := func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return
		}
		paths = append(paths, absolute)
	}
	add(configPath)
	add(filepath.Join(packageInfo.Module.Dir, "go.mod"))
	add(filepath.Join(packageInfo.Module.Dir, "go.sum"))
	return paths
}

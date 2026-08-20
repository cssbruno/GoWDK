package appgen

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// PackagingOptions contains every caller-controlled input to a generated Go
// artifact build. Environment is copied to the go command instead of being
// read or changed through process-global state.
type PackagingOptions struct {
	Environment []string
	Tags        []string
}

// PackagingMetadata is the reproducibility envelope recorded for a generated
// binary or WASM artifact.
type PackagingMetadata struct {
	GoVersion      string
	GOOS           string
	GOARCH         string
	CGOEnabled     string
	Trimpath       bool
	BuildVCS       bool
	ModuleMode     string
	Tags           []string
	ArtifactSHA256 string
}

// Data returns stable string fields suitable for a build-report event.
func (metadata PackagingMetadata) Data() map[string]string {
	return map[string]string{
		"artifactSHA256": metadata.ArtifactSHA256,
		"buildVCS":       "false",
		"cgoEnabled":     metadata.CGOEnabled,
		"goarch":         metadata.GOARCH,
		"goos":           metadata.GOOS,
		"goVersion":      metadata.GoVersion,
		"moduleMode":     metadata.ModuleMode,
		"tags":           strings.Join(metadata.Tags, ","),
		"trimpath":       "true",
	}
}

// PackagingResult describes an atomically published generated artifact.
type PackagingResult struct {
	Path     string
	Metadata PackagingMetadata
}

// BuildBinary compiles the generated app into binaryPath.
func BuildBinary(appDir, binaryPath string) (string, error) {
	result, err := BuildBinaryWithOptions(appDir, binaryPath, PackagingOptions{})
	return result.Path, err
}

// BuildBinaryWithOptions compiles and atomically publishes the generated app.
func BuildBinaryWithOptions(appDir, binaryPath string, options PackagingOptions) (PackagingResult, error) {
	return buildGeneratedCommand(appDir, binaryPath, "./cmd/server", "generated app", "binary output path is required", options, "", "")
}

// BuildWorkerBinary compiles the generated contract worker app.
func BuildWorkerBinary(appDir, binaryPath string) (string, error) {
	result, err := BuildWorkerBinaryWithOptions(appDir, binaryPath, PackagingOptions{})
	return result.Path, err
}

// BuildWorkerBinaryWithOptions compiles and atomically publishes a worker.
func BuildWorkerBinaryWithOptions(appDir, binaryPath string, options PackagingOptions) (PackagingResult, error) {
	return buildGeneratedCommand(appDir, binaryPath, "./cmd/worker", "generated worker app", "worker binary output path is required", options, "", "")
}

// BuildCronBinary compiles the generated contract cron app.
func BuildCronBinary(appDir, binaryPath string) (string, error) {
	result, err := BuildCronBinaryWithOptions(appDir, binaryPath, PackagingOptions{})
	return result.Path, err
}

// BuildCronBinaryWithOptions compiles and atomically publishes a cron role.
func BuildCronBinaryWithOptions(appDir, binaryPath string, options PackagingOptions) (PackagingResult, error) {
	return buildGeneratedCommand(appDir, binaryPath, "./cmd/cron", "generated cron app", "cron binary output path is required", options, "", "")
}

func buildGeneratedCommand(appDir, artifactPath, packagePath, label, emptyArtifactMessage string, options PackagingOptions, goos, goarch string) (PackagingResult, error) {
	if strings.TrimSpace(appDir) == "" {
		return PackagingResult{}, fmt.Errorf("%s directory is required", label)
	}
	if strings.TrimSpace(artifactPath) == "" {
		return PackagingResult{}, fmt.Errorf("%s", emptyArtifactMessage)
	}
	absApp, err := filepath.Abs(appDir)
	if err != nil {
		return PackagingResult{}, err
	}
	absArtifact, err := filepath.Abs(artifactPath)
	if err != nil {
		return PackagingResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(absArtifact), 0o755); err != nil {
		return PackagingResult{}, err
	}

	context := resolveGeneratedModuleContext(absApp)
	if packagePath != "./cmd/server" {
		context = generatedModuleContext{Nested: true, ImportBase: legacyGeneratedAppModulePath, BuildDir: absApp}
	}
	baseEnvironment := options.Environment
	if baseEnvironment == nil {
		baseEnvironment = os.Environ()
	}
	goEnvironment := generatedAppGoEnv(buildEnvWithout(baseEnvironment, "GOFLAGS", "GOOS", "GOARCH"), context.Nested)
	if goos != "" {
		goEnvironment = append(goEnvironment, "GOOS="+goos)
	}
	if goarch != "" {
		goEnvironment = append(goEnvironment, "GOARCH="+goarch)
	}
	buildDir := context.BuildDir
	buildPackage := packagePath
	if !context.Nested {
		buildPackage = "./" + pathJoinSlash(context.AppRel, strings.TrimPrefix(packagePath, "./"))
	}
	moduleMode := "readonly"
	if info, err := os.Stat(filepath.Join(buildDir, "vendor")); err == nil && info.IsDir() {
		moduleMode = "vendor"
	}
	tags := cleanPackagingTags(options.Tags)

	temporary, err := os.CreateTemp(filepath.Dir(absArtifact), ".gowdk-artifact-*")
	if err != nil {
		return PackagingResult{}, err
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		_ = os.Remove(temporaryPath)
		return PackagingResult{}, err
	}
	defer os.Remove(temporaryPath)

	args := []string{"build", "-trimpath", "-buildvcs=false", "-mod=" + moduleMode}
	if len(tags) > 0 {
		args = append(args, "-tags="+strings.Join(tags, ","))
	}
	args = append(args, "-o", temporaryPath, buildPackage)
	command, err := commandWithEnvironment("go", goEnvironment, args...)
	if err != nil {
		return PackagingResult{}, err
	}
	command.Dir = buildDir
	command.Env = goEnvironment
	output, err := command.CombinedOutput()
	if err != nil {
		return PackagingResult{}, fmt.Errorf("go build %s failed: %w\n%s", label, err, strings.TrimSpace(string(output)))
	}
	digest, err := artifactDigest(temporaryPath)
	if err != nil {
		return PackagingResult{}, err
	}
	metadata, err := packagingMetadata(buildDir, goEnvironment)
	if err != nil {
		return PackagingResult{}, err
	}
	metadata.Trimpath = true
	metadata.BuildVCS = false
	metadata.ModuleMode = moduleMode
	metadata.Tags = tags
	metadata.ArtifactSHA256 = digest
	if err := publishArtifact(temporaryPath, absArtifact); err != nil {
		return PackagingResult{}, err
	}
	return PackagingResult{Path: absArtifact, Metadata: metadata}, nil
}

// BuildWASM compiles the generated app into a Go js/wasm artifact.
func BuildWASM(appDir, wasmPath string) (string, error) {
	result, err := BuildWASMWithOptions(appDir, wasmPath, PackagingOptions{})
	return result.Path, err
}

// BuildWASMWithOptions compiles and atomically publishes a Go js/wasm artifact.
func BuildWASMWithOptions(appDir, wasmPath string, options PackagingOptions) (PackagingResult, error) {
	return buildGeneratedCommand(appDir, wasmPath, "./cmd/server", "generated wasm", "wasm output path is required", options, "js", "wasm")
}

func packagingMetadata(buildDir string, environment []string) (PackagingMetadata, error) {
	command, err := commandWithEnvironment("go", environment, "env", "-json", "GOVERSION", "GOOS", "GOARCH", "CGO_ENABLED")
	if err != nil {
		return PackagingMetadata{}, err
	}
	command.Dir = buildDir
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		return PackagingMetadata{}, fmt.Errorf("read Go packaging environment: %w\n%s", err, strings.TrimSpace(string(output)))
	}
	var values struct {
		GoVersion  string `json:"GOVERSION"`
		GOOS       string `json:"GOOS"`
		GOARCH     string `json:"GOARCH"`
		CGOEnabled string `json:"CGO_ENABLED"`
	}
	if err := json.Unmarshal(output, &values); err != nil {
		return PackagingMetadata{}, fmt.Errorf("parse Go packaging environment: %w", err)
	}
	return PackagingMetadata{GoVersion: values.GoVersion, GOOS: values.GOOS, GOARCH: values.GOARCH, CGOEnabled: values.CGOEnabled}, nil
}

func artifactDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func publishArtifact(temporaryPath, artifactPath string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(temporaryPath, artifactPath)
	}
	backupPath := artifactPath + ".gowdk-previous"
	_ = os.Remove(backupPath)
	hadPrevious := false
	if _, err := os.Stat(artifactPath); err == nil {
		if err := os.Rename(artifactPath, backupPath); err != nil {
			return fmt.Errorf("preserve previous artifact before publish: %w", err)
		}
		hadPrevious = true
	}
	if err := os.Rename(temporaryPath, artifactPath); err != nil {
		if hadPrevious {
			_ = os.Rename(backupPath, artifactPath)
		}
		return fmt.Errorf("publish generated artifact: %w", err)
	}
	if hadPrevious {
		_ = os.Remove(backupPath)
	}
	return nil
}

func cleanPackagingTags(tags []string) []string {
	seen := map[string]bool{}
	var cleaned []string
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || seen[tag] {
			continue
		}
		seen[tag] = true
		cleaned = append(cleaned, tag)
	}
	sort.Strings(cleaned)
	return cleaned
}

func commandWithEnvironment(name string, environment []string, args ...string) (*exec.Cmd, error) {
	pathValue := ""
	pathExtensions := ""
	for _, entry := range environment {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		switch strings.ToUpper(key) {
		case "PATH":
			pathValue = value
		case "PATHEXT":
			pathExtensions = value
		}
	}
	if pathValue == "" {
		resolved, err := exec.LookPath(name)
		if err != nil {
			return nil, err
		}
		command := exec.Command(resolved, args...)
		command.Env = environment
		return command, nil
	}
	extensions := []string{""}
	if runtime.GOOS == "windows" {
		extensions = filepath.SplitList(pathExtensions)
		if len(extensions) == 0 {
			extensions = []string{".com", ".exe", ".bat", ".cmd"}
		}
	}
	for _, directory := range filepath.SplitList(pathValue) {
		if directory == "" {
			directory = "."
		}
		for _, extension := range extensions {
			candidate := filepath.Join(directory, name+extension)
			info, err := os.Stat(candidate)
			if err == nil && !info.IsDir() && (runtime.GOOS == "windows" || info.Mode()&0o111 != 0) {
				command := exec.Command(candidate, args...)
				command.Env = environment
				return command, nil
			}
		}
	}
	return nil, fmt.Errorf("executable %q not found in packaging PATH", name)
}

func generatedAppGoEnv(env []string, disableWorkspace bool) []string {
	if env == nil {
		env = os.Environ()
	}
	if !disableWorkspace {
		return env
	}
	return append(buildEnvWithout(env, "GOWORK"), "GOWORK=off")
}

func pathJoinSlash(parts ...string) string {
	var cleaned []string
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(filepath.ToSlash(part)), "/")
		if part != "" && part != "." {
			cleaned = append(cleaned, part)
		}
	}
	return strings.Join(cleaned, "/")
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

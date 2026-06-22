package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/addons/ssr"
)

const testUsage = "usage: gowdk test [--config <file>] [--env-file <file>] [--module <name>] [--target <name>] [--stage <unit|app|binary|browser>] [--run <pattern>] [--timeout <duration>] [--count <n>] [--cover] [--json] [--keep-workdir] [--browser-command <command>] [--ssr] [files...]"

const (
	testStageUnit    = "unit"
	testStageApp     = "app"
	testStageBinary  = "binary"
	testStageBrowser = "browser"
)

type testOptions struct {
	ConfigPath     string
	EnvFilePath    string
	ModuleNames    []string
	TargetNames    []string
	Stages         []string
	Paths          []string
	RunPattern     string
	Timeout        string
	Count          string
	Cover          bool
	JSON           bool
	KeepWorkdir    bool
	BrowserCommand string
	SSR            bool
}

type testWorkdir struct {
	Root       string
	OutputDir  string
	AppDir     string
	BinaryPath string
}

type testBinaryProcess struct {
	command *exec.Cmd
	addr    string
	output  *boundedBuffer
}

func gowdkTest(args []string) error {
	options, err := parseTestOptions(args)
	if err != nil {
		return err
	}
	if len(options.Stages) == 0 {
		options.Stages = []string{testStageBinary}
	}

	cli := cliOptions{EnvFilePath: options.EnvFilePath}
	if options.SSR {
		cli.Config.Addons = append(cli.Config.Addons, ssr.Addon())
	}
	if err := loadProjectConfig(&cli, options.ConfigPath); err != nil {
		return err
	}
	modules, err := testSelectedModules(cli, options)
	if err != nil {
		return err
	}

	var work *testWorkdir
	var cleanup func()
	defer func() {
		if cleanup != nil {
			cleanup()
		}
	}()

	for _, stage := range options.Stages {
		switch stage {
		case testStageUnit:
			if err := runGoTestStage(cli.ProjectRoot, stage, options, nil); err != nil {
				return err
			}
		case testStageApp:
			if work == nil {
				work, cleanup, err = buildTestWorkdir(cli, options, modules)
				if err != nil {
					return err
				}
			}
			if err := runGoTestStage(cli.ProjectRoot, stage, options, testEnv(work, stage, "")); err != nil {
				return err
			}
		case testStageBinary:
			if work == nil {
				work, cleanup, err = buildTestWorkdir(cli, options, modules)
				if err != nil {
					return err
				}
			}
			if err := runBinaryTestStage(cli.ProjectRoot, work, stage, options); err != nil {
				return err
			}
		case testStageBrowser:
			if strings.TrimSpace(options.BrowserCommand) == "" {
				return fmt.Errorf("gowdk test --stage browser requires --browser-command <command>")
			}
			if work == nil {
				work, cleanup, err = buildTestWorkdir(cli, options, modules)
				if err != nil {
					return err
				}
			}
			if err := runBrowserTestStage(cli.ProjectRoot, work, options); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown test stage %q", stage)
		}
	}
	return nil
}

func parseTestOptions(args []string) (testOptions, error) {
	var options testOptions
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if value, next, ok, missing := consumeValueFlag(args, index, "--config", true); ok {
			if missing {
				return testOptions{}, fmt.Errorf(testUsage)
			}
			options.ConfigPath = value
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--env-file", true); ok {
			if missing {
				return testOptions{}, fmt.Errorf(testUsage)
			}
			options.EnvFilePath = value
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--module", true); ok {
			if missing {
				return testOptions{}, fmt.Errorf(testUsage)
			}
			options.ModuleNames = appendModuleNames(options.ModuleNames, value)
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--target", true); ok {
			if missing {
				return testOptions{}, fmt.Errorf(testUsage)
			}
			options.TargetNames = appendNames(options.TargetNames, value)
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--stage", true); ok {
			if missing {
				return testOptions{}, fmt.Errorf(testUsage)
			}
			options.Stages = appendTestStages(options.Stages, value)
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--run", true); ok {
			if missing {
				return testOptions{}, fmt.Errorf(testUsage)
			}
			options.RunPattern = value
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--timeout", true); ok {
			if missing {
				return testOptions{}, fmt.Errorf(testUsage)
			}
			timeout, err := normalizeTestTimeout(value)
			if err != nil {
				return testOptions{}, err
			}
			options.Timeout = timeout
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--count", true); ok {
			if missing {
				return testOptions{}, fmt.Errorf(testUsage)
			}
			count, err := normalizeTestCount(value)
			if err != nil {
				return testOptions{}, err
			}
			options.Count = count
			index = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, index, "--browser-command", true); ok {
			if missing {
				return testOptions{}, fmt.Errorf(testUsage)
			}
			options.BrowserCommand = value
			index = next
			continue
		}
		switch {
		case arg == "-h" || arg == "--help":
			return testOptions{}, fmt.Errorf(testUsage)
		case arg == "--ssr":
			options.SSR = true
		case arg == "--cover":
			options.Cover = true
		case arg == "--json":
			options.JSON = true
		case arg == "--keep-workdir":
			options.KeepWorkdir = true
		case strings.HasPrefix(arg, "-"):
			return testOptions{}, fmt.Errorf("unknown test flag %q", arg)
		default:
			options.Paths = append(options.Paths, arg)
		}
	}
	for _, stage := range options.Stages {
		if !validTestStage(stage) {
			return testOptions{}, fmt.Errorf("unknown test stage %q", stage)
		}
	}
	if len(options.TargetNames) > 0 && len(options.ModuleNames) > 0 {
		return testOptions{}, fmt.Errorf("gowdk test --target cannot be combined with --module")
	}
	return options, nil
}

func appendTestStages(stages []string, value string) []string {
	for _, stage := range strings.Split(value, ",") {
		stage = strings.TrimSpace(stage)
		if stage != "" {
			stages = append(stages, stage)
		}
	}
	return stages
}

func validTestStage(stage string) bool {
	switch stage {
	case testStageUnit, testStageApp, testStageBinary, testStageBrowser:
		return true
	default:
		return false
	}
}

func normalizeTestTimeout(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("test timeout is required")
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return "", fmt.Errorf("test timeout must be a positive duration")
	}
	return value, nil
}

func normalizeTestCount(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("test count is required")
	}
	count, err := strconv.Atoi(value)
	if err != nil || count < 0 {
		return "", fmt.Errorf("test count must be a non-negative integer")
	}
	return value, nil
}

func testSelectedModules(cli cliOptions, options testOptions) ([]string, error) {
	if len(options.TargetNames) == 0 {
		return cleanNames(options.ModuleNames), nil
	}
	targets, err := selectBuildTargets(cli.Config.Build.Targets, options.TargetNames)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var modules []string
	for _, target := range targets {
		for _, module := range target.Modules {
			if !seen[module] {
				seen[module] = true
				modules = append(modules, module)
			}
		}
	}
	return modules, nil
}

func buildTestWorkdir(cli cliOptions, options testOptions, modules []string) (*testWorkdir, func(), error) {
	root, err := os.MkdirTemp("", "gowdk-test-*")
	if err != nil {
		return nil, nil, err
	}
	work := &testWorkdir{
		Root:       root,
		OutputDir:  filepath.Join(root, "output"),
		AppDir:     filepath.Join(root, "app"),
		BinaryPath: filepath.Join(root, "bin", testBinaryName()),
	}
	cleanup := func() {
		if options.KeepWorkdir {
			fmt.Fprintf(os.Stderr, "gowdk test workdir preserved: %s\n", work.Root)
			return
		}
		if err := os.RemoveAll(work.Root); err != nil {
			fmt.Fprintf(os.Stderr, "remove gowdk test workdir: %v\n", err)
		}
	}

	paths, err := resolveExplicitTestPaths(options.Paths)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	fmt.Fprintf(os.Stderr, "gowdk test [build]: %s\n", work.Root)
	request := buildRequest{
		OutputDir:  work.OutputDir,
		AppDir:     work.AppDir,
		BinaryPath: work.BinaryPath,
		Modules:    modules,
		Paths:      paths,
	}
	if err := runInWorkingDir(cli.ProjectRoot, func() error {
		return runTestBuildOnce(cli, request, options.JSON)
	}); err != nil {
		cleanup()
		return nil, nil, err
	}
	return work, cleanup, nil
}

func resolveExplicitTestPaths(paths []string) ([]string, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		if filepath.IsAbs(path) {
			resolved = append(resolved, filepath.Clean(path))
			continue
		}
		resolved = append(resolved, filepath.Clean(filepath.Join(cwd, path)))
	}
	return resolved, nil
}

func runTestBuildOnce(cli cliOptions, request buildRequest, quietStdout bool) error {
	if !quietStdout {
		return buildOnce(cli, request, newBuildTimingRecorder(false))
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	previousStdout := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = previousStdout
	}()
	drained := make(chan error, 1)
	go func() {
		_, err := io.Copy(io.Discard, reader)
		drained <- err
	}()

	buildErr := buildOnce(cli, request, newBuildTimingRecorder(false))
	closeErr := writer.Close()
	drainErr := <-drained
	readerErr := reader.Close()
	if buildErr != nil {
		return buildErr
	}
	if closeErr != nil {
		return closeErr
	}
	if drainErr != nil {
		return drainErr
	}
	return readerErr
}

func testBinaryName() string {
	if runtime.GOOS == "windows" {
		return "app.exe"
	}
	return "app"
}

func runBinaryTestStage(projectRoot string, work *testWorkdir, stage string, options testOptions) error {
	process, err := startTestBinary(work.BinaryPath)
	if err != nil {
		return err
	}
	defer process.stop()
	return runGoTestStage(projectRoot, stage, options, testEnv(work, stage, "http://"+process.addr))
}

func runBrowserTestStage(projectRoot string, work *testWorkdir, options testOptions) error {
	process, err := startTestBinary(work.BinaryPath)
	if err != nil {
		return err
	}
	defer process.stop()
	env := testEnv(work, testStageBrowser, "http://"+process.addr)
	artifactDir := filepath.Join(work.Root, "browser-artifacts")
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return err
	}
	env = append(env, "GOWDK_TEST_ARTIFACT_DIR="+artifactDir)
	fmt.Fprintf(os.Stderr, "gowdk test [browser]: %s\n", options.BrowserCommand)
	command := shellCommand(options.BrowserCommand)
	command.Dir = projectRoot
	command.Env = append(os.Environ(), env...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("gowdk test browser stage failed: %w", err)
	}
	return nil
}

func runGoTestStage(projectRoot string, stage string, options testOptions, env []string) error {
	args := []string{"test"}
	if options.JSON {
		args = append(args, "-json")
	}
	if options.Cover {
		args = append(args, "-cover")
	}
	if options.Count != "" {
		args = append(args, "-count="+options.Count)
	}
	if options.Timeout != "" {
		args = append(args, "-timeout="+options.Timeout)
	}
	if options.RunPattern != "" {
		args = append(args, "-run", options.RunPattern)
	}
	args = append(args, "./...")

	fmt.Fprintf(os.Stderr, "gowdk test [%s]: go %s\n", stage, strings.Join(args, " "))
	command := exec.Command("go", args...)
	command.Dir = projectRoot
	command.Env = append(os.Environ(), env...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("gowdk test stage %q failed: %w; reproduce with `go %s` from %s", stage, err, strings.Join(args, " "), projectRoot)
	}
	return nil
}

func testEnv(work *testWorkdir, stage string, baseURL string) []string {
	if work == nil {
		return nil
	}
	env := []string{
		"GOWDK_TEST_STAGE=" + stage,
		"GOWDK_TEST_WORKDIR=" + work.Root,
		"GOWDK_TEST_OUTPUT_DIR=" + work.OutputDir,
		"GOWDK_TEST_APP_DIR=" + work.AppDir,
		"GOWDK_TEST_BINARY=" + work.BinaryPath,
	}
	if baseURL != "" {
		env = append(env, "GOWDK_TEST_BASE_URL="+baseURL)
	}
	return env
}

func startTestBinary(binaryPath string) (*testBinaryProcess, error) {
	addr, err := freeTestAddr()
	if err != nil {
		return nil, err
	}
	output := &boundedBuffer{limit: defaultAuditRunOutputLimit}
	command := exec.Command(binaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+addr)
	command.Stdout = output
	command.Stderr = output
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("start generated app binary: %w", err)
	}
	process := &testBinaryProcess{command: command, addr: addr, output: output}
	if err := waitForTestHealth("http://" + addr + "/_gowdk/health"); err != nil {
		process.stop()
		return nil, fmt.Errorf("%w\n%s", err, strings.TrimSpace(output.String()))
	}
	return process, nil
}

func (process *testBinaryProcess) stop() {
	if process == nil || process.command == nil || process.command.Process == nil {
		return
	}
	_ = process.command.Process.Kill()
	_, _ = process.command.Process.Wait()
}

func waitForTestHealth(url string) error {
	deadline := time.Now().Add(10 * time.Second)
	client := http.Client{Timeout: 500 * time.Millisecond}
	var lastErr error
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("health status %d", response.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(50 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = context.DeadlineExceeded
	}
	return fmt.Errorf("generated app did not become ready at %s: %w", url, lastErr)
}

func freeTestAddr() (string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		return "", err
	}
	return addr, nil
}

func shellCommand(command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.Command("cmd", "/C", command)
	}
	return exec.Command("sh", "-c", command)
}

func runInWorkingDir(dir string, fn func() error) error {
	if strings.TrimSpace(dir) == "" {
		return fn()
	}
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := os.Chdir(dir); err != nil {
		return err
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			fmt.Fprintf(os.Stderr, "restore working directory: %v\n", err)
		}
	}()
	return fn()
}

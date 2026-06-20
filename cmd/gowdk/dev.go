package main

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cssbruno/gowdk/internal/appgen"
)

const defaultDevOutputDir = "gowdk_cache"

func dev(args []string) error {
	options, err := parseDevOptions(args)
	if err != nil {
		return err
	}
	rawBuildArgs := append([]string(nil), options.BuildArgs...)
	state, err := newDevBuildState(rawBuildArgs)
	if err != nil {
		return err
	}
	options.BuildArgs = state.buildArgs
	absDir, err := filepath.Abs(state.outputDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return err
	}
	previous, err := state.snapshot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	lastSuccessfulBuild := time.Now()
	if state.runtime.Enabled || previous == nil || !devInputCacheFresh(absDir, previous) {
		if err := buildLoaded(state.plan, 0); err != nil {
			return err
		}
		lastSuccessfulBuild = time.Now()
		if tracker, err := newDevInputTracker(state.plan); err == nil {
			state.tracker = tracker
			previous, err = state.snapshot()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		if previous != nil {
			if err := writeDevInputCache(absDir, previous); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	} else {
		fmt.Printf("Dev cache hit: inputs unchanged for %s\n", absDir)
		lastSuccessfulBuild = devLastSuccessfulBuildTime(absDir, lastSuccessfulBuild)
	}

	serve := newDevServeState(options.Addr)
	if err := serve.apply(state, absDir); err != nil {
		return err
	}
	defer serve.close()

	fmt.Printf("Dev server polling GOWDK inputs every %s\n", options.Interval)
	fmt.Println(devStartupLine(state, absDir, options.Addr, serve.runtimeAddr()))
	notifyBuildError := func(err error, change inputChange) {
		serve.notifyBuildError(err, change, lastSuccessfulBuild)
	}
	for {
		time.Sleep(options.Interval)
		buildAbsDir, err := filepath.Abs(state.outputDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			notifyBuildError(err, inputChange{})
			continue
		}
		current, err := state.snapshot()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			notifyBuildError(err, inputChange{})
			continue
		}
		if current.same(previous) {
			continue
		}
		change := current.diff(previous)
		if state.configChanged(change) {
			next, err := newDevBuildState(rawBuildArgs)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				notifyBuildError(err, change)
				continue
			}
			nextAbsDir, err := filepath.Abs(next.outputDir)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				notifyBuildError(err, change)
				continue
			}
			if err := os.MkdirAll(nextAbsDir, 0o755); err != nil {
				fmt.Fprintln(os.Stderr, err)
				notifyBuildError(err, change)
				continue
			}
			state = next
			options.BuildArgs = state.buildArgs
			buildAbsDir = nextAbsDir
			current, err = state.snapshot()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				notifyBuildError(err, change)
				continue
			}
			change = current.diff(previous)
		}
		previous = current
		fmt.Printf("Change detected at %s: %s\n", time.Now().Format(time.RFC3339), change.summary())
		for _, detail := range change.details() {
			fmt.Printf("  %s\n", detail)
		}
		_, err = buildDevChangeLoaded(state.plan, change, true)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			notifyBuildError(err, change)
			continue
		}
		if err := serve.apply(state, buildAbsDir); err != nil {
			fmt.Fprintln(os.Stderr, err)
			notifyBuildError(err, change)
			continue
		}
		absDir = buildAbsDir
		lastSuccessfulBuild = time.Now()
		fmt.Println(devRebuildCompleteLine(state, absDir, options.Addr, serve.runtimeAddr()))
		if tracker, err := newDevInputTracker(state.plan); err == nil {
			state.tracker = tracker
			if refreshed, err := state.snapshot(); err == nil {
				previous = refreshed
			} else {
				fmt.Fprintln(os.Stderr, err)
			}
		} else {
			fmt.Fprintln(os.Stderr, err)
		}
		if err := writeDevInputCache(absDir, previous); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		serve.notifyReloadForChange(state, change)
	}
}

type devOptions struct {
	BuildArgs []string
	Addr      string
	Interval  time.Duration
}

type devRuntime struct {
	Enabled    bool
	AppDir     string
	BinaryPath string
}

type devBuildState struct {
	buildArgs []string
	plan      buildOptions
	outputDir string
	runtime   devRuntime
	tracker   devInputTracker
}

func newDevBuildState(args []string) (devBuildState, error) {
	state, err := newDevBuildPlan(args)
	if err != nil {
		return devBuildState{}, err
	}
	tracker, err := newDevInputTracker(state.plan)
	if err != nil {
		return devBuildState{}, err
	}
	state.tracker = tracker
	return state, nil
}

func newDevBuildPlan(args []string) (devBuildState, error) {
	plan, err := loadBuildOptions(args)
	if err != nil {
		return devBuildState{}, err
	}
	outputDir, err := devOutputDirLoaded(plan)
	if err != nil {
		return devBuildState{}, err
	}
	buildArgs := append([]string(nil), args...)
	if !devArgsHaveOutput(args) && !devArgsHaveTarget(args) {
		buildArgs = append(buildArgs, "--out", outputDir)
		plan.OutputDir = outputDir
	}
	runtime, err := devRuntimePlanLoaded(plan, outputDir)
	if err != nil {
		return devBuildState{}, err
	}
	return devBuildState{
		buildArgs: buildArgs,
		plan:      plan,
		outputDir: outputDir,
		runtime:   runtime,
	}, nil
}

func (state devBuildState) snapshot() (inputSnapshot, error) {
	return state.tracker.snapshot()
}

func (state devBuildState) configChanged(change inputChange) bool {
	return inputChangeTouchesConfig(change, state.plan.ConfigPath) || inputChangeTouchesEnvFile(change, state.plan.Options.EnvFilePath)
}

type devServeState struct {
	addr      string
	reload    *liveReloadBroker
	server    *http.Server
	staticDir string
	process   *devRuntimeProcess
}

func newDevServeState(addr string) *devServeState {
	return &devServeState{
		addr:   addr,
		reload: newLiveReloadBroker(),
	}
}

func (serve *devServeState) apply(state devBuildState, absDir string) error {
	if state.runtime.Enabled {
		return serve.useRuntime(state.runtime)
	}
	serve.useStatic(absDir)
	return nil
}

func (serve *devServeState) useStatic(absDir string) {
	if serve.process != nil {
		serve.process.stop()
		serve.process = nil
	}
	if serve.server != nil && serve.staticDir == absDir {
		return
	}
	if serve.server != nil {
		stopDevStaticServer(serve.server)
	}
	serve.server = startDevStaticServer(serve.addr, absDir, serve.reload)
	serve.staticDir = absDir
}

func (serve *devServeState) useRuntime(runtime devRuntime) error {
	if _, err := appgen.BuildBinary(runtime.AppDir, runtime.BinaryPath); err != nil {
		return err
	}
	if serve.server != nil && serve.process == nil {
		stopDevStaticServer(serve.server)
		serve.server = nil
		serve.staticDir = ""
	}
	if serve.server == nil {
		targetAddr, err := freeDevRuntimeAddr()
		if err != nil {
			return err
		}
		serve.server = startDevRuntimeProxy(serve.addr, targetAddr, serve.reload)
		serve.staticDir = ""
		if serve.process == nil {
			serve.process = &devRuntimeProcess{addr: targetAddr}
		} else {
			serve.process.addr = targetAddr
		}
	}
	if serve.process == nil {
		return fmt.Errorf("dev runtime proxy did not initialize")
	} else if serve.process.plan != runtime {
		serve.process.stop()
		serve.process.plan = runtime
	}
	return serve.process.restart()
}

func (serve *devServeState) notifyBuildError(err error, change inputChange, lastSuccessfulBuild time.Time) {
	serve.reload.notifyData("build-error", devOverlayErrorEventData(err, change, lastSuccessfulBuild))
}

func (serve *devServeState) notifyReload() {
	serve.reload.notify("reload")
}

func (serve *devServeState) notifyReloadForChange(state devBuildState, change inputChange) {
	if state.runtime.Enabled {
		serve.notifyReload()
		return
	}
	if payload, ok := devComponentHMRPayloadLoaded(state.plan, change); ok {
		serve.reload.notifyData("component-hmr", payload)
		return
	}
	serve.notifyReload()
}

func (serve *devServeState) runtimeAddr() string {
	if serve.process == nil {
		return ""
	}
	serve.process.mu.Lock()
	defer serve.process.mu.Unlock()
	return serve.process.addr
}

func (serve *devServeState) close() {
	if serve.process != nil {
		serve.process.stop()
		serve.process = nil
	}
	if serve.server != nil {
		stopDevStaticServer(serve.server)
		serve.server = nil
		serve.staticDir = ""
	}
}

func startDevStaticServer(addr, absDir string, reload *liveReloadBroker) *http.Server {
	server := &http.Server{
		Addr:              addr,
		Handler:           liveReloadFileHandler(absDir, reload),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	return server
}

func startDevRuntimeProxy(addr, targetAddr string, reload *liveReloadBroker) *http.Server {
	server := &http.Server{
		Addr:              addr,
		Handler:           devRuntimeProxyHandler(targetAddr, reload),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	return server
}

func stopDevStaticServer(server *http.Server) {
	if err := server.Close(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fmt.Fprintln(os.Stderr, err)
	}
}

func freeDevRuntimeAddr() (string, error) {
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

type devRuntimeProcess struct {
	plan     devRuntime
	addr     string
	mu       sync.Mutex
	cmd      *exec.Cmd
	waitDone chan error
}

func (process *devRuntimeProcess) restart() error {
	process.stop()
	command := exec.Command(process.plan.BinaryPath)
	command.Env = append(os.Environ(), "GOWDK_ADDR="+process.addr)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		return fmt.Errorf("start generated app: %w", err)
	}
	waitDone := make(chan error, 1)
	process.mu.Lock()
	process.cmd = command
	process.waitDone = waitDone
	process.mu.Unlock()
	go process.wait(command, waitDone)
	return nil
}

func (process *devRuntimeProcess) stop() {
	command, waitDone := process.activeCommand()
	if command == nil || command.Process == nil {
		return
	}
	if err := command.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		fmt.Fprintln(os.Stderr, err)
	}
	if waitDone != nil {
		<-waitDone
	}
}

func (process *devRuntimeProcess) activeCommand() (*exec.Cmd, <-chan error) {
	process.mu.Lock()
	defer process.mu.Unlock()
	command := process.cmd
	waitDone := process.waitDone
	process.cmd = nil
	process.waitDone = nil
	return command, waitDone
}

func (process *devRuntimeProcess) wait(command *exec.Cmd, waitDone chan<- error) {
	err := command.Wait()
	waitDone <- err

	process.mu.Lock()
	active := process.cmd == command
	if active {
		process.cmd = nil
		process.waitDone = nil
	}
	process.mu.Unlock()

	if err != nil && active {
		fmt.Fprintln(os.Stderr, err)
	}
}

func devStartupLine(state devBuildState, absDir string, publicAddr string, runtimeAddr string) string {
	if state.runtime.Enabled {
		return devRuntimeProxyLine("Generated app runtime", state.runtime, publicAddr, runtimeAddr)
	}
	return fmt.Sprintf("Static dev server: serving %s at http://%s", absDir, publicAddr)
}

func devRebuildCompleteLine(state devBuildState, absDir string, publicAddr string, runtimeAddr string) string {
	if state.runtime.Enabled {
		return devRuntimeProxyLine("Dev rebuild complete: generated app restarted", state.runtime, publicAddr, runtimeAddr)
	}
	return fmt.Sprintf("Dev rebuild complete: static output refreshed at %s", absDir)
}

func devRuntimeProxyLine(prefix string, runtime devRuntime, publicAddr string, runtimeAddr string) string {
	if runtimeAddr == "" {
		return fmt.Sprintf("%s: proxy http://%s -> generated app (binary %s)", prefix, publicAddr, runtime.BinaryPath)
	}
	return fmt.Sprintf("%s: proxy http://%s -> http://%s (binary %s)", prefix, publicAddr, runtimeAddr, runtime.BinaryPath)
}

func parseDevOptions(args []string) (devOptions, error) {
	options := devOptions{
		Addr:     "127.0.0.1:8080",
		Interval: time.Second,
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--addr":
			i++
			if i >= len(args) {
				return devOptions{}, errors.New(devUsage())
			}
			options.Addr = args[i]
		case strings.HasPrefix(arg, "--addr="):
			options.Addr = strings.TrimPrefix(arg, "--addr=")
		case arg == "--interval":
			i++
			if i >= len(args) {
				return devOptions{}, errors.New(devUsage())
			}
			interval, err := parseDevInterval(args[i])
			if err != nil {
				return devOptions{}, err
			}
			options.Interval = interval
		case strings.HasPrefix(arg, "--interval="):
			interval, err := parseDevInterval(strings.TrimPrefix(arg, "--interval="))
			if err != nil {
				return devOptions{}, err
			}
			options.Interval = interval
		default:
			options.BuildArgs = append(options.BuildArgs, arg)
		}
	}
	if strings.TrimSpace(options.Addr) == "" {
		return devOptions{}, fmt.Errorf("dev address is required")
	}
	return options, nil
}

func devUsage() string {
	return "usage: gowdk dev [--addr <addr>] [--interval <duration>] [build flags...]"
}

func devRuntimePlan(args []string, outputDir string) (devRuntime, []string, error) {
	plan, err := loadBuildOptions(args)
	if err != nil {
		return devRuntime{}, args, err
	}
	runtime, err := devRuntimePlanLoaded(plan, outputDir)
	return runtime, args, err
}

func devRuntimePlanLoaded(plan buildOptions, outputDir string) (devRuntime, error) {
	appDir, binaryPath, err := devAppAndBinaryLoaded(plan)
	if err != nil || strings.TrimSpace(appDir) == "" {
		return devRuntime{}, err
	}
	if strings.TrimSpace(binaryPath) != "" {
		return devRuntime{Enabled: true, AppDir: appDir, BinaryPath: binaryPath}, nil
	}
	binaryPath = filepath.Join(outputDir, ".gowdk", "dev", "app")
	if os.PathSeparator == '\\' {
		binaryPath += ".exe"
	}
	return devRuntime{Enabled: true, AppDir: appDir, BinaryPath: binaryPath}, nil
}

func devAppAndBinaryLoaded(plan buildOptions) (string, string, error) {
	if len(plan.TargetNames) > 0 {
		targets, err := selectBuildTargets(plan.Options.Config.Build.Targets, plan.TargetNames)
		if err != nil {
			return "", "", err
		}
		if len(targets) != 1 {
			return "", "", fmt.Errorf("dev runtime requires exactly one build target")
		}
		return targets[0].App, targets[0].Binary, nil
	}
	return plan.AppDir, plan.BinaryPath, nil
}

func devOutputDir(args []string) (string, error) {
	plan, err := loadBuildOptions(args)
	if err != nil {
		return "", err
	}
	return devOutputDirLoaded(plan)
}

func devOutputDirLoaded(plan buildOptions) (string, error) {
	if strings.TrimSpace(plan.OutputDir) != "" {
		return plan.OutputDir, nil
	}
	if len(plan.TargetNames) > 0 {
		targets, err := selectBuildTargets(plan.Options.Config.Build.Targets, plan.TargetNames)
		if err != nil {
			return "", err
		}
		if len(targets) != 1 {
			return "", fmt.Errorf("dev requires exactly one build target with Output")
		}
		if strings.TrimSpace(targets[0].Output) == "" {
			return "", fmt.Errorf("dev target %q is missing Output", targets[0].Name)
		}
		return targets[0].Output, nil
	}
	return defaultDevOutputDir, nil
}

func devBuildArgs(args []string) ([]string, string, error) {
	state, err := newDevBuildPlan(args)
	if err != nil {
		return nil, "", err
	}
	return state.buildArgs, state.outputDir, nil
}

func devArgsHaveOutput(args []string) bool {
	for _, arg := range args {
		if arg == "--out" || strings.HasPrefix(arg, "--out=") {
			return true
		}
	}
	return false
}

func devArgsHaveTarget(args []string) bool {
	for _, arg := range args {
		if arg == "--target" || strings.HasPrefix(arg, "--target=") {
			return true
		}
	}
	return false
}

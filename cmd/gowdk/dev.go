package main

import (
	"errors"
	"fmt"
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
	if state.runtime.Enabled || previous == nil || !devInputCacheFresh(absDir, previous) {
		if err := buildLoaded(state.plan, 0); err != nil {
			return err
		}
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
	}

	var reload *liveReloadBroker
	var server *http.Server
	var process *devRuntimeProcess
	if state.runtime.Enabled {
		fmt.Println("Live reload is not available for app targets; reload the browser manually after each rebuild.")
		if _, err := appgen.BuildBinary(state.runtime.AppDir, state.runtime.BinaryPath); err != nil {
			return err
		}
		process = &devRuntimeProcess{plan: state.runtime, addr: options.Addr}
		if err := process.restart(); err != nil {
			return err
		}
		defer process.stop()
	} else {
		reload = newLiveReloadBroker()
		server = &http.Server{
			Addr:              options.Addr,
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
	}

	fmt.Printf("Dev server polling GOWDK inputs every %s\n", options.Interval)
	if state.runtime.Enabled {
		fmt.Printf("Running generated app %s at http://%s\n", state.runtime.BinaryPath, options.Addr)
	} else {
		fmt.Printf("Serving %s at http://%s\n", absDir, options.Addr)
	}
	for {
		time.Sleep(options.Interval)
		current, err := state.snapshot()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			reload.notifyData("build-error", err.Error())
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
				reload.notifyData("build-error", err.Error())
				continue
			}
			state = next
			options.BuildArgs = state.buildArgs
			current, err = state.snapshot()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				reload.notifyData("build-error", err.Error())
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
			reload.notifyData("build-error", err.Error())
			continue
		}
		if process != nil {
			if !state.runtime.Enabled {
				process.stop()
				process = nil
				continue
			}
			if process.plan != state.runtime {
				process.stop()
				process.plan = state.runtime
			}
			if _, err := appgen.BuildBinary(state.runtime.AppDir, state.runtime.BinaryPath); err != nil {
				fmt.Fprintln(os.Stderr, err)
				reload.notifyData("build-error", err.Error())
				continue
			}
			if err := process.restart(); err != nil {
				fmt.Fprintln(os.Stderr, err)
				reload.notifyData("build-error", err.Error())
				continue
			}
		}
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
		reload.notify("reload")
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
	return inputChangeTouchesConfig(change, state.plan.ConfigPath)
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

func devAppAndBinary(args []string, _ string) (string, string, error) {
	plan, err := loadBuildOptions(args)
	if err != nil {
		return "", "", err
	}
	return devAppAndBinaryLoaded(plan)
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

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/internal/appgen"
)

const defaultDevOutputDir = "gowdk_cache"

func dev(args []string) error {
	options, err := parseDevOptions(args)
	if err != nil {
		return err
	}
	buildArgs, outputDir, err := devBuildArgs(options.BuildArgs)
	if err != nil {
		return err
	}
	options.BuildArgs = buildArgs
	runtime, buildArgs, err := devRuntimePlan(buildArgs, outputDir)
	if err != nil {
		return err
	}
	options.BuildArgs = buildArgs
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return err
	}
	previous, err := buildInputSnapshot(options.BuildArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	if runtime.Enabled || previous == nil || !devInputCacheFresh(absDir, previous) {
		if err := build(options.BuildArgs); err != nil {
			return err
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
	if runtime.Enabled {
		fmt.Println("Live reload is not available for app targets; reload the browser manually after each rebuild.")
		if _, err := appgen.BuildBinary(runtime.AppDir, runtime.BinaryPath); err != nil {
			return err
		}
		process = &devRuntimeProcess{plan: runtime, addr: options.Addr}
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
	if runtime.Enabled {
		fmt.Printf("Running generated app %s at http://%s\n", runtime.BinaryPath, options.Addr)
	} else {
		fmt.Printf("Serving %s at http://%s\n", absDir, options.Addr)
	}
	for {
		time.Sleep(options.Interval)
		current, err := buildInputSnapshot(options.BuildArgs)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			reload.notifyData("build-error", err.Error())
			continue
		}
		if current.same(previous) {
			continue
		}
		change := current.diff(previous)
		previous = current
		fmt.Printf("Change detected at %s: %s\n", time.Now().Format(time.RFC3339), change.summary())
		for _, detail := range change.details() {
			fmt.Printf("  %s\n", detail)
		}
		_, err = buildDevChange(options.BuildArgs, change, true)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			reload.notifyData("build-error", err.Error())
			continue
		}
		if process != nil {
			if _, err := appgen.BuildBinary(runtime.AppDir, runtime.BinaryPath); err != nil {
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
		if err := writeDevInputCache(absDir, current); err != nil {
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

type devRuntimeProcess struct {
	plan devRuntime
	addr string
	cmd  *exec.Cmd
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
	process.cmd = command
	go func() {
		if err := command.Wait(); err != nil && process.cmd == command {
			fmt.Fprintln(os.Stderr, err)
		}
	}()
	return nil
}

func (process *devRuntimeProcess) stop() {
	if process.cmd == nil || process.cmd.Process == nil {
		return
	}
	if err := process.cmd.Process.Kill(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	process.cmd = nil
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
	appDir, binaryPath, err := devAppAndBinary(args, outputDir)
	if err != nil || strings.TrimSpace(appDir) == "" {
		return devRuntime{}, args, err
	}
	if strings.TrimSpace(binaryPath) != "" {
		return devRuntime{Enabled: true, AppDir: appDir, BinaryPath: binaryPath}, args, nil
	}
	binaryPath = filepath.Join(outputDir, ".gowdk", "dev", "app")
	if os.PathSeparator == '\\' {
		binaryPath += ".exe"
	}
	return devRuntime{Enabled: true, AppDir: appDir, BinaryPath: binaryPath}, args, nil
}

func devAppAndBinary(args []string, _ string) (string, string, error) {
	options, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, configPath, targetNames, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return "", "", err
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return "", "", err
	}
	if len(targetNames) > 0 && hasAdHocBuildArgs(outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, moduleNames, paths) {
		return "", "", fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --wasm, --backend-app, --backend-bin, or explicit files")
	}
	if len(targetNames) > 0 {
		targets, err := selectBuildTargets(options.Config.Build.Targets, targetNames)
		if err != nil {
			return "", "", err
		}
		if len(targets) != 1 {
			return "", "", fmt.Errorf("dev runtime requires exactly one build target")
		}
		return targets[0].App, targets[0].Binary, nil
	}
	return appDir, binaryPath, nil
}

func devOutputDir(args []string) (string, error) {
	options, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, configPath, targetNames, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return "", err
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return "", err
	}
	if len(targetNames) > 0 && hasAdHocBuildArgs(outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, moduleNames, paths) {
		return "", fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --wasm, --backend-app, --backend-bin, or explicit files")
	}
	if strings.TrimSpace(outputDir) != "" {
		return outputDir, nil
	}
	if len(targetNames) > 0 {
		targets, err := selectBuildTargets(options.Config.Build.Targets, targetNames)
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
	if strings.TrimSpace(outputDir) != "" {
		return outputDir, nil
	}
	return defaultDevOutputDir, nil
}

func devBuildArgs(args []string) ([]string, string, error) {
	outputDir, err := devOutputDir(args)
	if err != nil {
		return nil, "", err
	}
	if devArgsHaveOutput(args) || devArgsHaveTarget(args) {
		return append([]string(nil), args...), outputDir, nil
	}
	next := append([]string(nil), args...)
	next = append(next, "--out", outputDir)
	return next, outputDir, nil
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

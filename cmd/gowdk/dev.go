package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	if err := build(options.BuildArgs); err != nil {
		return err
	}
	absDir, err := filepath.Abs(outputDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return err
	}

	reload := newLiveReloadBroker()
	server := &http.Server{
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

	fmt.Printf("Dev server polling GOWDK inputs every %s\n", options.Interval)
	fmt.Printf("Serving %s at http://%s\n", absDir, options.Addr)
	previous, err := buildInputSnapshot(options.BuildArgs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	for {
		time.Sleep(options.Interval)
		current, err := buildInputSnapshot(options.BuildArgs)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if current.same(previous) {
			continue
		}
		change := current.diff(previous)
		previous = current
		fmt.Printf("Change detected at %s: %s\n", time.Now().Format(time.RFC3339), change.summary())
		_, err = buildDevChange(options.BuildArgs, change, true)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		reload.notify("reload")
	}
}

type devOptions struct {
	BuildArgs []string
	Addr      string
	Interval  time.Duration
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

func devOutputDir(args []string) (string, error) {
	options, outputDir, appDir, binaryPath, wasmPath, configPath, targetNames, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return "", err
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return "", err
	}
	if len(targetNames) > 0 && hasAdHocBuildArgs(outputDir, appDir, binaryPath, wasmPath, moduleNames, paths) {
		return "", fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --wasm, or explicit files")
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

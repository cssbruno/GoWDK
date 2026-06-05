package main

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/staticgen"
)

func watch(args []string) error {
	options, err := parseWatchOptions(args)
	if err != nil {
		return err
	}
	if options.Once && options.Restart {
		return fmt.Errorf("watch --restart cannot be used with --once")
	}
	if options.Once {
		return build(options.BuildArgs)
	}

	var process *watchProcess
	if options.Restart {
		binaryPath, err := watchRestartBinaryPath(options.BuildArgs)
		if err != nil {
			return err
		}
		process = &watchProcess{Path: binaryPath}
		defer func() {
			if err := process.stop(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}()
	}

	fmt.Printf("Watching GOWDK inputs every %s\n", options.Interval)
	if err := build(options.BuildArgs); err != nil {
		fmt.Fprintln(os.Stderr, err)
	} else if process != nil {
		if err := process.restart(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
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
		fmt.Printf("Change detected at %s\n", time.Now().Format(time.RFC3339))
		restart, err := buildWatchChange(options.BuildArgs, change, process == nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			continue
		}
		if restart && process != nil {
			if err := process.restart(); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
}

func buildWatchChange(args []string, change inputChange, allowIncremental bool) (bool, error) {
	if allowIncremental {
		incremental, err := buildIncrementalStatic(args, change)
		if incremental || err != nil {
			return false, err
		}
	}
	return true, build(args)
}

func buildIncrementalStatic(args []string, change inputChange) (bool, error) {
	if len(change.Added) > 0 || len(change.Removed) > 0 || len(change.Changed) == 0 {
		return false, nil
	}

	options, outputDir, appDir, binaryPath, wasmPath, configPath, targetNames, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return true, err
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return true, err
	}
	if len(targetNames) > 0 && hasAdHocBuildArgs(outputDir, appDir, binaryPath, wasmPath, moduleNames, paths) {
		return true, fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --wasm, or explicit files")
	}
	if shouldBuildConfiguredTargets(options.Config, targetNames, outputDir, appDir, binaryPath, wasmPath, moduleNames, paths) {
		return false, nil
	}
	if strings.TrimSpace(appDir) != "" || strings.TrimSpace(binaryPath) != "" || strings.TrimSpace(wasmPath) != "" {
		return false, nil
	}
	if inputChangeTouchesConfig(change, configPath) {
		return false, nil
	}
	if outputDir == "" {
		outputDir = options.Config.Build.Output
	}
	if outputDir == "" {
		return true, fmt.Errorf(buildUsage)
	}
	options.Config.Build.Output = outputDir
	if len(paths) == 0 {
		discovered, err := discoverBuildFiles(options.Config, outputDir, moduleNames)
		if err != nil {
			return true, err
		}
		if len(discovered) == 0 {
			return true, fmt.Errorf("no .gwdk files found")
		}
		paths = discovered
	}

	app, diagnostics := lang.ParseBuildFiles(paths)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return true, fmt.Errorf("build failed")
	}

	pageSources, incremental := changedPageSources(app, change.Changed)
	if !incremental {
		return false, nil
	}
	result, err := staticgen.BuildIncremental(options.Config, app, outputDir, pageSources)
	if err != nil {
		printStaticgenBuildErrorReport(err, options.Debug)
		return true, err
	}
	for _, artifact := range result.Artifacts {
		if pageIDChanged(artifact.PageID, pageSources, app.Pages) {
			fmt.Println(artifact.Path)
		}
	}
	for _, artifact := range result.CSSArtifacts {
		fmt.Println(artifact.Path)
	}
	for _, artifact := range result.AssetArtifacts {
		fmt.Println(artifact.Path)
	}
	if result.RouteManifestPath != "" {
		fmt.Println(result.RouteManifestPath)
	}
	if result.AssetManifestPath != "" {
		fmt.Println(result.AssetManifestPath)
	}
	if result.BuildReportPath != "" {
		fmt.Println(result.BuildReportPath)
	}
	printStaticgenBuildReport(result.Report, options.Debug)
	return true, nil
}

func inputChangeTouchesConfig(change inputChange, configPath string) bool {
	configAbs, ok := watchedConfigPath(configPath)
	if !ok {
		return false
	}
	for _, changedPath := range change.Changed {
		if samePath(changedPath, configAbs) {
			return true
		}
	}
	return false
}

func watchedConfigPath(configPath string) (string, bool) {
	if strings.TrimSpace(configPath) != "" {
		abs, err := filepath.Abs(configPath)
		return filepath.Clean(abs), err == nil
	}
	if _, err := os.Stat("gowdk.config.go"); err != nil {
		return "", false
	}
	abs, err := filepath.Abs("gowdk.config.go")
	return filepath.Clean(abs), err == nil
}

func changedPageSources(app manifest.Manifest, changedPaths []string) ([]string, bool) {
	pageSources := map[string]string{}
	for _, page := range app.Pages {
		abs, ok := cleanAbs(page.Source)
		if ok {
			pageSources[abs] = page.Source
		}
	}

	var changedPages []string
	for _, changedPath := range changedPaths {
		abs, ok := cleanAbs(changedPath)
		if !ok {
			return nil, false
		}
		source, ok := pageSources[abs]
		if !ok {
			return nil, false
		}
		changedPages = append(changedPages, source)
	}
	return changedPages, len(changedPages) > 0
}

func pageIDChanged(pageID string, changedSources []string, pages []manifest.Page) bool {
	changed := map[string]bool{}
	for _, source := range changedSources {
		abs, ok := cleanAbs(source)
		if ok {
			changed[abs] = true
		}
	}
	for _, page := range pages {
		if page.ID != pageID {
			continue
		}
		abs, ok := cleanAbs(page.Source)
		return ok && changed[abs]
	}
	return false
}

func samePath(left, right string) bool {
	leftAbs, leftOK := cleanAbs(left)
	rightAbs, rightOK := cleanAbs(right)
	return leftOK && rightOK && leftAbs == rightAbs
}

func cleanAbs(path string) (string, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	return filepath.Clean(abs), true
}

type watchProcess struct {
	Path        string
	command     *exec.Cmd
	stopTimeout time.Duration
}

func (process *watchProcess) restart() error {
	if strings.TrimSpace(process.Path) == "" {
		return fmt.Errorf("watch restart binary path is required")
	}
	if err := process.stop(); err != nil {
		return err
	}
	command := exec.Command(process.Path)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	command.Env = os.Environ()
	if err := command.Start(); err != nil {
		return fmt.Errorf("start %s: %w", process.Path, err)
	}
	process.command = command
	fmt.Printf("Started %s pid=%d\n", process.Path, command.Process.Pid)
	return nil
}

func (process *watchProcess) stop() error {
	if process.command == nil || process.command.Process == nil {
		process.command = nil
		return nil
	}

	command := process.command
	process.command = nil
	if err := command.Process.Signal(os.Interrupt); err != nil {
		_ = command.Process.Kill()
	}

	done := make(chan error, 1)
	go func() {
		done <- command.Wait()
	}()

	timeout := process.stopTimeout
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		_ = command.Process.Kill()
		<-done
		return nil
	}
}

type watchOptions struct {
	BuildArgs []string
	Once      bool
	Interval  time.Duration
	Restart   bool
}

func parseWatchOptions(args []string) (watchOptions, error) {
	options := watchOptions{Interval: time.Second}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--once":
			options.Once = true
		case arg == "--restart":
			options.Restart = true
		case arg == "--interval":
			i++
			if i >= len(args) {
				return watchOptions{}, errors.New(watchUsage())
			}
			interval, err := parseWatchInterval(args[i])
			if err != nil {
				return watchOptions{}, err
			}
			options.Interval = interval
		case strings.HasPrefix(arg, "--interval="):
			interval, err := parseWatchInterval(strings.TrimPrefix(arg, "--interval="))
			if err != nil {
				return watchOptions{}, err
			}
			options.Interval = interval
		default:
			options.BuildArgs = append(options.BuildArgs, arg)
		}
	}
	return options, nil
}

func watchUsage() string {
	return "usage: gowdk watch [--once] [--restart] [--interval <duration>] [build flags...]"
}

func watchRestartBinaryPath(args []string) (string, error) {
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
	if strings.TrimSpace(binaryPath) != "" {
		return binaryPath, nil
	}
	if shouldBuildConfiguredTargets(options.Config, targetNames, outputDir, appDir, binaryPath, wasmPath, moduleNames, paths) {
		targets, err := selectBuildTargets(options.Config.Build.Targets, targetNames)
		if err != nil {
			return "", err
		}
		if len(targets) != 1 {
			return "", fmt.Errorf("watch --restart requires exactly one build target with Binary")
		}
		if strings.TrimSpace(targets[0].Binary) == "" {
			return "", fmt.Errorf("watch --restart target %q is missing Binary", targets[0].Name)
		}
		return targets[0].Binary, nil
	}
	return "", fmt.Errorf("watch --restart requires --bin <file> or one Build.Targets entry with Binary")
}

func parseWatchInterval(value string) (time.Duration, error) {
	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid watch interval %q: %w", value, err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("watch interval must be positive")
	}
	return interval, nil
}

type inputSnapshot map[string]string

type inputChange struct {
	Changed []string
	Added   []string
	Removed []string
}

func (change inputChange) summary() string {
	var parts []string
	if len(change.Changed) > 0 {
		parts = append(parts, fmt.Sprintf("%d changed", len(change.Changed)))
	}
	if len(change.Added) > 0 {
		parts = append(parts, fmt.Sprintf("%d added", len(change.Added)))
	}
	if len(change.Removed) > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", len(change.Removed)))
	}
	if len(parts) == 0 {
		return "no file changes"
	}
	return strings.Join(parts, ", ")
}

func buildInputSnapshot(args []string) (inputSnapshot, error) {
	options, outputDir, appDir, binaryPath, wasmPath, configPath, targetNames, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return nil, err
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return nil, err
	}
	if len(targetNames) > 0 && hasAdHocBuildArgs(outputDir, appDir, binaryPath, wasmPath, moduleNames, paths) {
		return nil, fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --wasm, or explicit files")
	}
	if shouldBuildConfiguredTargets(options.Config, targetNames, outputDir, appDir, binaryPath, wasmPath, moduleNames, paths) {
		targets, err := selectBuildTargets(options.Config.Build.Targets, targetNames)
		if err != nil {
			return nil, err
		}
		for _, target := range targets {
			discovered, err := discoverBuildFiles(options.Config, target.Output, target.Modules)
			if err != nil {
				return nil, err
			}
			paths = append(paths, discovered...)
		}
	} else if outputDir == "" {
		outputDir = options.Config.Build.Output
		if len(paths) == 0 {
			discovered, err := discoverBuildFiles(options.Config, outputDir, moduleNames)
			if err != nil {
				return nil, err
			}
			paths = discovered
		}
	} else if len(paths) == 0 {
		discovered, err := discoverBuildFiles(options.Config, outputDir, moduleNames)
		if err != nil {
			return nil, err
		}
		paths = discovered
	}
	if strings.TrimSpace(configPath) != "" {
		paths = append(paths, configPath)
	} else if _, err := os.Stat("gowdk.config.go"); err == nil {
		paths = append(paths, "gowdk.config.go")
	}
	snapshot := inputSnapshot{}
	for _, item := range paths {
		info, err := os.Stat(item)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			continue
		}
		abs, err := filepath.Abs(item)
		if err != nil {
			return nil, err
		}
		payload, err := os.ReadFile(item)
		if err != nil {
			return nil, err
		}
		sum := sha256.Sum256(payload)
		snapshot[abs] = fmt.Sprintf("%x", sum)
	}
	return snapshot, nil
}

func (snapshot inputSnapshot) same(other inputSnapshot) bool {
	if len(snapshot) != len(other) {
		return false
	}
	for path, hash := range snapshot {
		otherHash, ok := other[path]
		if !ok || hash != otherHash {
			return false
		}
	}
	return true
}

func (snapshot inputSnapshot) diff(previous inputSnapshot) inputChange {
	var change inputChange
	for path, hash := range snapshot {
		previousHash, ok := previous[path]
		if !ok {
			change.Added = append(change.Added, path)
			continue
		}
		if hash != previousHash {
			change.Changed = append(change.Changed, path)
		}
	}
	for path := range previous {
		if _, ok := snapshot[path]; !ok {
			change.Removed = append(change.Removed, path)
		}
	}
	sort.Strings(change.Changed)
	sort.Strings(change.Added)
	sort.Strings(change.Removed)
	return change
}

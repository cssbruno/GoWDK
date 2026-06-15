package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk/internal/playground"
)

const playgroundUsage = "usage: gowdk playground policy [--json] | gowdk playground export --dir <project> --out <project.zip> [--json] | gowdk playground run --dir <project> --out <dir> --allow-hosted-execution"

func playgroundCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf(playgroundUsage)
	}
	switch args[0] {
	case "policy":
		return playgroundPolicy(args[1:])
	case "export":
		return playgroundExport(args[1:])
	case "run":
		return playgroundRun(args[1:])
	default:
		return fmt.Errorf(playgroundUsage)
	}
}

func playgroundPolicy(args []string) error {
	jsonOutput := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOutput = true
		default:
			return fmt.Errorf(playgroundUsage)
		}
	}
	policy := playground.DefaultPolicy()
	if jsonOutput {
		payload, err := playground.PolicyJSON(policy)
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	fmt.Println("Playground hosted execution: disabled by default")
	fmt.Println("Workspace:", policy.Workspace)
	fmt.Println("Network:", policy.Network)
	fmt.Println("Filesystem:", policy.Filesystem)
	fmt.Printf("Limits: %d files, %d bytes per file, %d bytes total, %d seconds\n", policy.Limits.MaxFiles, policy.Limits.MaxFileBytes, policy.Limits.MaxTotalBytes, policy.Limits.WallClockSeconds)
	fmt.Println("Export:", policy.Export)
	return nil
}

func playgroundExport(args []string) error {
	options, err := parsePlaygroundFileOptions(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(options.Output) == "" {
		return fmt.Errorf(playgroundUsage)
	}
	result, err := playground.ExportArchive(options.Dir, options.Output, playground.Options{})
	if err != nil {
		return err
	}
	if options.JSON {
		payload, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	fmt.Println(result.Archive)
	for _, file := range result.Files {
		fmt.Println(file.Path)
	}
	return nil
}

func playgroundRun(args []string) error {
	options, err := parsePlaygroundFileOptions(args)
	if err != nil {
		return err
	}
	if options.JSON {
		return fmt.Errorf("--json is not supported by playground run")
	}
	if !options.AllowExecution {
		return fmt.Errorf("hosted playground execution is disabled by default; pass --allow-hosted-execution only inside the documented sandbox")
	}
	if strings.TrimSpace(options.Output) == "" {
		return fmt.Errorf(playgroundUsage)
	}
	workspace, cleanup, err := playground.StageWorkspace(options.Dir, playground.Options{})
	if err != nil {
		return err
	}
	defer cleanup()
	cacheRoot, err := os.MkdirTemp("", "gowdk-playground-cache-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(cacheRoot)
	env := playground.SanitizedEnvironment(cacheRoot)
	if err := playground.RejectSecretEnvironment(env); err != nil {
		return err
	}
	outputDir, err := filepath.Abs(options.Output)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	return withPlaygroundEnvironment(env, func() error {
		return build([]string{"--config", filepath.Join(workspace.Root, "gowdk.config.go"), "--out", outputDir})
	})
}

type playgroundFileOptions struct {
	Dir            string
	Output         string
	JSON           bool
	AllowExecution bool
}

func parsePlaygroundFileOptions(args []string) (playgroundFileOptions, error) {
	options := playgroundFileOptions{Dir: "."}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--dir":
			index++
			if index >= len(args) {
				return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
			}
			options.Dir = args[index]
		case strings.HasPrefix(arg, "--dir="):
			options.Dir = strings.TrimPrefix(arg, "--dir=")
		case arg == "--out":
			index++
			if index >= len(args) {
				return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
			}
			options.Output = args[index]
		case strings.HasPrefix(arg, "--out="):
			options.Output = strings.TrimPrefix(arg, "--out=")
		case arg == "--json":
			options.JSON = true
		case arg == "--allow-hosted-execution":
			options.AllowExecution = true
		default:
			return playgroundFileOptions{}, fmt.Errorf(playgroundUsage)
		}
	}
	if strings.TrimSpace(options.Dir) == "" {
		return playgroundFileOptions{}, fmt.Errorf("playground source directory is required")
	}
	return options, nil
}

func withPlaygroundEnvironment(env []string, fn func() error) error {
	previous := os.Environ()
	os.Clearenv()
	for _, item := range env {
		name, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		if err := os.Setenv(name, value); err != nil {
			restoreEnvironment(previous)
			return err
		}
	}
	defer restoreEnvironment(previous)
	return fn()
}

func restoreEnvironment(env []string) {
	os.Clearenv()
	for _, item := range env {
		name, value, ok := strings.Cut(item, "=")
		if !ok {
			continue
		}
		_ = os.Setenv(name, value)
	}
}

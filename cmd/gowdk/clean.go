package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
)

const cleanUsage = "usage: gowdk clean [--config <file>] [--target <name>] [--out <dir>] [--dry-run] [--json]"

// cleanResult is the machine-readable outcome of a clean run.
type cleanResult struct {
	Version int      `json:"version"`
	DryRun  bool     `json:"dryRun"`
	Removed []string `json:"removed"`
	Absent  []string `json:"absent"`
}

// clean removes the generated build outputs declared by the project config
// (Build.Output and every configured target's output, app, binary, and WASM
// paths) plus any explicit --out directory. It only ever touches configured
// output paths inside the project root, never the source tree.
func clean(args []string) error {
	var (
		configPath  string
		targetNames []string
		outDir      string
		dryRun      bool
		jsonOutput  bool
	)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dry-run":
			dryRun = true
		case arg == "--json":
			jsonOutput = true
		case arg == "--config":
			i++
			if i >= len(args) {
				return errors.New(cleanUsage)
			}
			configPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--target":
			i++
			if i >= len(args) {
				return errors.New(cleanUsage)
			}
			targetNames = appendNames(targetNames, args[i])
		case strings.HasPrefix(arg, "--target="):
			targetNames = appendNames(targetNames, strings.TrimPrefix(arg, "--target="))
		case arg == "--out":
			i++
			if i >= len(args) {
				return errors.New(cleanUsage)
			}
			outDir = args[i]
		case strings.HasPrefix(arg, "--out="):
			outDir = strings.TrimPrefix(arg, "--out=")
		default:
			return fmt.Errorf("unknown clean flag %q\n%s", arg, cleanUsage)
		}
	}

	var options cliOptions
	if err := loadProjectConfig(&options, configPath); err != nil {
		return err
	}

	candidates, err := cleanTargets(options.Config, targetNames, outDir)
	if err != nil {
		return err
	}

	result, err := runClean(options.ProjectRoot, candidates, dryRun)
	if err != nil {
		return err
	}
	return reportClean(result, jsonOutput)
}

// runClean resolves candidates against root, removes the ones that exist (or
// reports them under DryRun), and records the rest as absent. Paths that are
// the project root or escape it are silently dropped by safeRelativeTargets.
func runClean(root string, candidates []string, dryRun bool) (cleanResult, error) {
	result := cleanResult{Version: 1, DryRun: dryRun}
	for _, candidate := range safeRelativeTargets(root, candidates) {
		absolute := candidate
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(root, candidate)
		}
		if _, statErr := os.Lstat(absolute); statErr != nil {
			if os.IsNotExist(statErr) {
				result.Absent = append(result.Absent, candidate)
				continue
			}
			return cleanResult{}, statErr
		}
		if !dryRun {
			if removeErr := os.RemoveAll(absolute); removeErr != nil {
				return cleanResult{}, fmt.Errorf("clean %s: %w", candidate, removeErr)
			}
		}
		result.Removed = append(result.Removed, candidate)
	}
	return result, nil
}

// cleanTargets gathers the generated output paths from the config and the
// optional --out override, restricted to --target when given.
func cleanTargets(config gowdk.Config, targetNames []string, outDir string) ([]string, error) {
	var targets []string
	add := func(path string) {
		if strings.TrimSpace(path) != "" {
			targets = append(targets, path)
		}
	}

	selected := cleanNames(targetNames)
	if len(selected) == 0 {
		add(config.Build.Output)
	}

	wanted := map[string]bool{}
	for _, name := range selected {
		wanted[name] = true
	}
	seen := map[string]bool{}
	for _, target := range config.Build.Targets {
		if len(wanted) > 0 && !wanted[target.Name] {
			continue
		}
		seen[target.Name] = true
		add(target.Output)
		add(target.App)
		add(target.Binary)
		add(target.WASM)
		add(target.BackendApp)
		add(target.BackendBinary)
	}
	for _, name := range selected {
		if !seen[name] {
			return nil, fmt.Errorf("target %q is not configured", name)
		}
	}

	add(outDir)
	return targets, nil
}

// safeRelativeTargets resolves candidates against root, drops anything that is
// the project root itself or escapes it, and de-duplicates while preserving a
// stable sorted order.
func safeRelativeTargets(root string, candidates []string) []string {
	seen := map[string]bool{}
	var safe []string
	for _, candidate := range candidates {
		absolute := candidate
		if !filepath.IsAbs(absolute) {
			absolute = filepath.Join(root, candidate)
		}
		absolute = filepath.Clean(absolute)
		rel, err := filepath.Rel(root, absolute)
		if err != nil {
			continue
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
			// Refuse to remove the project root or anything outside it.
			continue
		}
		if seen[rel] {
			continue
		}
		seen[rel] = true
		safe = append(safe, rel)
	}
	sort.Strings(safe)
	return safe
}

func reportClean(result cleanResult, jsonOutput bool) error {
	if jsonOutput {
		payload, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	if len(result.Removed) == 0 {
		fmt.Println("nothing to clean")
		return nil
	}
	verb := "removed"
	if result.DryRun {
		verb = "would remove"
	}
	for _, path := range result.Removed {
		fmt.Printf("%s %s\n", verb, path)
	}
	return nil
}

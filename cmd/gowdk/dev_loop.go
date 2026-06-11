package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/discover"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/lang"
)

func buildDevChange(args []string, change inputChange, allowIncremental bool) (bool, error) {
	if allowIncremental {
		incremental, err := buildIncrementalSPA(args, change)
		if incremental || err != nil {
			return false, err
		}
	}
	return true, build(args)
}

func buildIncrementalSPA(args []string, change inputChange) (bool, error) {
	if len(change.Added) > 0 || len(change.Removed) > 0 || len(change.Changed) == 0 {
		return false, nil
	}

	options, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, configPath, targetNames, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return true, err
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return true, err
	}
	if len(targetNames) > 0 && hasAdHocBuildArgs(outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, moduleNames, paths) {
		return true, fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --wasm, --backend-app, --backend-bin, or explicit files")
	}
	if shouldBuildConfiguredTargets(options.Config, targetNames, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, moduleNames, paths) {
		return false, nil
	}
	if strings.TrimSpace(appDir) != "" || strings.TrimSpace(binaryPath) != "" || strings.TrimSpace(wasmPath) != "" || strings.TrimSpace(backendAppDir) != "" || strings.TrimSpace(backendBinaryPath) != "" {
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
	ir := gwdkanalysis.BuildProgram(options.Config, app)
	result, err := buildgen.BuildIncrementalFromIR(options.Config, ir, outputDir, pageSources)
	if err != nil {
		printBuildgenBuildErrorReport(err, options.Debug)
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
	printBuildgenBuildReport(result.Report, options.Debug)
	return true, nil
}

func inputChangeTouchesConfig(change inputChange, configPath string) bool {
	configAbs, ok := devConfigPath(configPath)
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

func devConfigPath(configPath string) (string, bool) {
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

func changedPageSources(app gwdkanalysis.Sources, changedPaths []string) ([]string, bool) {
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

func pageIDChanged(pageID string, changedSources []string, pages []gwdkir.Page) bool {
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

func parseDevInterval(value string) (time.Duration, error) {
	interval, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid dev interval %q: %w", value, err)
	}
	if interval <= 0 {
		return 0, fmt.Errorf("dev interval must be positive")
	}
	return interval, nil
}

type inputSnapshot map[string]string

type inputChange struct {
	Changed []string
	Added   []string
	Removed []string
}

func devInputCacheFresh(outputDir string, snapshot inputSnapshot) bool {
	if len(snapshot) == 0 || !devOutputHasFiles(outputDir) {
		return false
	}
	cached, err := readDevInputCache(outputDir)
	return err == nil && cached.same(snapshot)
}

func devOutputHasFiles(outputDir string) bool {
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.Name() == ".gowdk" {
			continue
		}
		return true
	}
	return false
}

func readDevInputCache(outputDir string) (inputSnapshot, error) {
	payload, err := os.ReadFile(devInputCachePath(outputDir))
	if err != nil {
		return nil, err
	}
	var snapshot inputSnapshot
	if err := json.Unmarshal(payload, &snapshot); err != nil {
		return nil, err
	}
	return snapshot, nil
}

func writeDevInputCache(outputDir string, snapshot inputSnapshot) error {
	payload, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	path := devInputCachePath(outputDir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, payload, 0o644)
}

func devInputCachePath(outputDir string) string {
	return filepath.Join(outputDir, ".gowdk", "dev", "inputs.json")
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

func (change inputChange) details() []string {
	var details []string
	for _, path := range change.Changed {
		details = append(details, "changed: "+displayInputPath(path))
	}
	for _, path := range change.Added {
		details = append(details, "added: "+displayInputPath(path))
	}
	for _, path := range change.Removed {
		details = append(details, "removed: "+displayInputPath(path))
	}
	return details
}

func displayInputPath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	if rel, ok := relativeInputPath(cwd, path); ok {
		return rel
	}
	canonicalCWD, canonicalPath, ok := canonicalDisplayPaths(cwd, path)
	if ok {
		if rel, ok := relativeInputPath(canonicalCWD, canonicalPath); ok {
			return rel
		}
	}
	return path
}

func relativeInputPath(base, path string) (string, bool) {
	rel, err := filepath.Rel(base, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return rel, true
}

func canonicalDisplayPaths(base, path string) (string, string, bool) {
	canonicalBase, err := filepath.EvalSymlinks(base)
	if err != nil {
		return "", "", false
	}
	canonicalDir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return "", "", false
	}
	return canonicalBase, filepath.Join(canonicalDir, filepath.Base(path)), true
}

func buildInputSnapshot(args []string) (inputSnapshot, error) {
	options, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, configPath, targetNames, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return nil, err
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return nil, err
	}
	if len(targetNames) > 0 && hasAdHocBuildArgs(outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, moduleNames, paths) {
		return nil, fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --wasm, --backend-app, --backend-bin, or explicit files")
	}
	if shouldBuildConfiguredTargets(options.Config, targetNames, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, moduleNames, paths) {
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
			css, err := discoverBuildCSSFiles(options.Config, target.Output)
			if err != nil {
				return nil, err
			}
			paths = append(paths, css...)
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
		css, err := discoverBuildCSSFiles(options.Config, outputDir)
		if err != nil {
			return nil, err
		}
		paths = append(paths, css...)
	} else if len(paths) == 0 {
		discovered, err := discoverBuildFiles(options.Config, outputDir, moduleNames)
		if err != nil {
			return nil, err
		}
		paths = discovered
		css, err := discoverBuildCSSFiles(options.Config, outputDir)
		if err != nil {
			return nil, err
		}
		paths = append(paths, css...)
	} else {
		css, err := discoverBuildCSSFiles(options.Config, outputDir)
		if err != nil {
			return nil, err
		}
		paths = append(paths, css...)
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

func discoverBuildCSSFiles(config gowdk.Config, outputDir string) ([]string, error) {
	includes := appendPatterns(nil, config.CSS.Include)
	if len(includes) == 1 && includes[0] == buildgen.DisableCSSDiscovery {
		return nil, nil
	}
	if len(includes) == 0 {
		includes = []string{"**/*.css"}
	}

	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	excludes := []string{".git/**", "**/.git/**", "vendor/**", "**/vendor/**", "node_modules/**", "**/node_modules/**", ".gowdk/**", "**/.gowdk/**", "dist/**", "**/dist/**"}
	excludes = appendPatterns(excludes, config.CSS.Exclude)
	if pattern := outputExcludePattern(root, outputDir); pattern != "" {
		excludes = append(excludes, pattern)
	}
	return discover.Files(root, includes, excludes)
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

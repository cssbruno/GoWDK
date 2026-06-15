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
	"github.com/cssbruno/gowdk/internal/view"
)

func buildDevChange(args []string, change inputChange, allowIncremental bool) (bool, error) {
	plan, err := loadBuildOptions(args)
	if err != nil {
		return false, err
	}
	return buildDevChangeLoaded(plan, change, allowIncremental)
}

func buildDevChangeLoaded(plan buildOptions, change inputChange, allowIncremental bool) (bool, error) {
	if allowIncremental {
		incremental, err := buildIncrementalSPALoaded(plan, change)
		if incremental || err != nil {
			return false, err
		}
	}
	return true, buildLoaded(plan, 0)
}

func buildIncrementalSPA(args []string, change inputChange) (bool, error) {
	plan, err := loadBuildOptions(args)
	if err != nil {
		return true, err
	}
	return buildIncrementalSPALoaded(plan, change)
}

func buildIncrementalSPALoaded(plan buildOptions, change inputChange) (bool, error) {
	if len(change.Added) > 0 || len(change.Removed) > 0 || len(change.Changed) == 0 {
		return false, nil
	}

	timings := newBuildTimingRecorder(plan.Timings)
	if plan.shouldBuildConfiguredTargets() {
		return false, nil
	}
	if strings.TrimSpace(plan.AppDir) != "" || strings.TrimSpace(plan.BinaryPath) != "" || strings.TrimSpace(plan.WASMPath) != "" || strings.TrimSpace(plan.BackendAppDir) != "" || strings.TrimSpace(plan.BackendBinaryPath) != "" {
		return false, nil
	}
	if inputChangeTouchesConfig(change, plan.ConfigPath) {
		return false, nil
	}
	options := plan.Options
	outputDir := plan.OutputDir
	paths := append([]string(nil), plan.Paths...)
	if outputDir == "" {
		outputDir = options.Config.Build.Output
	}
	if outputDir == "" {
		return true, fmt.Errorf(buildUsage)
	}
	options.Config.Build.Output = outputDir
	if len(paths) == 0 {
		discovered, err := discoverBuildFiles(options.Config, outputDir, plan.ModuleNames, options.ProjectRoot)
		if err != nil {
			return true, err
		}
		if len(discovered) == 0 {
			return true, fmt.Errorf("no .gwdk files found")
		}
		paths = discovered
	}

	timings.counter("incremental_input_changes", len(change.Changed))
	var app gwdkanalysis.Sources
	var diagnostics lang.Diagnostics
	timings.measure("parse_lower", func() error {
		app, diagnostics = lang.ParseBuildFiles(paths)
		return nil
	})
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return true, newDevDiagnosticError("build failed", devOverlayDiagnosticsFromLang(diagnostics))
	}

	incrementalPlan, incremental := changedIncrementalSPAPages(app, change.Changed)
	if !incremental {
		return false, nil
	}
	timings.counter("incremental_page_changes", incrementalPlan.PageChanges)
	timings.counter("incremental_component_changes", incrementalPlan.ComponentChanges)
	timings.counter("incremental_layout_changes", incrementalPlan.LayoutChanges)
	timings.counter("incremental_affected_pages", len(incrementalPlan.PageSources))
	var ir gwdkir.Program
	timings.measure("ir_assembly", func() error {
		ir = gwdkanalysis.BuildProgram(options.Config, app)
		return nil
	})
	var result buildgen.Result
	if err := timings.measure("output_plan_writes", func() error {
		var buildErr error
		result, buildErr = buildgen.BuildIncrementalFromIR(options.Config, ir, outputDir, incrementalPlan.PageSources)
		return buildErr
	}); err != nil {
		printBuildgenBuildErrorReport(err, options.Debug)
		return true, err
	}
	timings.counter("files_written", result.WriteStats.FilesWritten)
	timings.counter("identical_writes_skipped", result.WriteStats.IdenticalWritesSkipped)
	for _, artifact := range result.Artifacts {
		if pageIDChanged(artifact.PageID, incrementalPlan.PageSources, app.Pages) {
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
	if result.SitemapPath != "" {
		fmt.Println(result.SitemapPath)
	}
	if result.RobotsPath != "" {
		fmt.Println(result.RobotsPath)
	}
	if result.SecurityManifestPath != "" {
		fmt.Println(result.SecurityManifestPath)
	}
	if result.BuildReportPath != "" {
		fmt.Println(result.BuildReportPath)
	}
	printBuildgenBuildReport(result.Report, options.Debug)
	if _, err := timings.write(outputDir, plan.TimingsPath); err != nil {
		return true, err
	}
	return true, nil
}

func inputChangeTouchesConfig(change inputChange, configPath string) bool {
	configAbs, ok := devConfigPath(configPath)
	if !ok {
		return false
	}
	for _, paths := range [][]string{change.Changed, change.Added, change.Removed} {
		for _, changedPath := range paths {
			if samePath(changedPath, configAbs) {
				return true
			}
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

type incrementalSPAChangePlan struct {
	PageSources      []string
	PageChanges      int
	ComponentChanges int
	LayoutChanges    int
}

func changedIncrementalSPAPages(app gwdkanalysis.Sources, changedPaths []string) (incrementalSPAChangePlan, bool) {
	index, ok := newIncrementalDependencyIndex(app)
	if !ok {
		return incrementalSPAChangePlan{}, false
	}
	affected := map[string]bool{}
	plan := incrementalSPAChangePlan{}
	for _, changedPath := range changedPaths {
		abs, ok := cleanAbs(changedPath)
		if !ok {
			return incrementalSPAChangePlan{}, false
		}
		if source, ok := index.pagesBySource[abs]; ok {
			affected[source] = true
			plan.PageChanges++
			continue
		}
		if key, ok := index.componentsBySource[abs]; ok {
			for _, source := range index.pagesByComponent[key] {
				affected[source] = true
			}
			plan.ComponentChanges++
			continue
		}
		if key, ok := index.layoutsBySource[abs]; ok {
			for _, source := range index.pagesByLayout[key] {
				affected[source] = true
			}
			plan.LayoutChanges++
			continue
		}
		return incrementalSPAChangePlan{}, false
	}
	plan.PageSources = sortedKeys(affected)
	return plan, true
}

type incrementalDependencyIndex struct {
	pagesBySource      map[string]string
	componentsBySource map[string]string
	layoutsBySource    map[string]string
	pagesByComponent   map[string][]string
	pagesByLayout      map[string][]string
}

func newIncrementalDependencyIndex(app gwdkanalysis.Sources) (incrementalDependencyIndex, bool) {
	index := incrementalDependencyIndex{
		pagesBySource:      map[string]string{},
		componentsBySource: map[string]string{},
		layoutsBySource:    map[string]string{},
		pagesByComponent:   map[string][]string{},
		pagesByLayout:      map[string][]string{},
	}
	componentsByKey := map[string]gwdkir.Component{}
	for _, page := range app.Pages {
		abs, ok := cleanAbs(page.Source)
		if !ok {
			return incrementalDependencyIndex{}, false
		}
		index.pagesBySource[abs] = page.Source
	}
	for _, component := range app.Components {
		key := sourceComponentKey(component.Package, component.Name)
		componentsByKey[key] = component
		abs, ok := cleanAbs(component.Source)
		if !ok {
			return incrementalDependencyIndex{}, false
		}
		index.componentsBySource[abs] = key
	}
	layoutsByKey := map[string]gwdkir.Layout{}
	for _, layout := range app.Layouts {
		key := sourceLayoutKey(layout.Package, layout.ID)
		layoutsByKey[key] = layout
		abs, ok := cleanAbs(layout.Source)
		if !ok {
			return incrementalDependencyIndex{}, false
		}
		index.layoutsBySource[abs] = key
	}
	for _, page := range app.Pages {
		for key := range pageComponentDependencies(page, componentsByKey, layoutsByKey) {
			index.pagesByComponent[key] = append(index.pagesByComponent[key], page.Source)
		}
		for key := range pageLayoutDependencies(page, layoutsByKey) {
			index.pagesByLayout[key] = append(index.pagesByLayout[key], page.Source)
		}
	}
	sortDependencyIndex(index.pagesByComponent)
	sortDependencyIndex(index.pagesByLayout)
	return index, true
}

func pageComponentDependencies(page gwdkir.Page, components map[string]gwdkir.Component, layouts map[string]gwdkir.Layout) map[string]bool {
	seen := map[string]bool{}
	collectComponentDependenciesFromView(page.Package, page.Uses, page.Blocks.ViewBody, page.Blocks.ViewNodes, components, seen)
	for _, ref := range page.Layouts {
		if layout, ok := resolvePageLayoutDependency(page.Package, page.Uses, ref, layouts); ok {
			collectLayoutComponentDependencies(layout, layouts, components, map[string]bool{}, seen)
		}
	}
	return seen
}

func collectComponentDependenciesFromView(ownerPackage string, uses []gwdkir.Use, viewBody string, viewNodes []view.Node, components map[string]gwdkir.Component, seen map[string]bool) {
	var refs []string
	if len(viewNodes) > 0 {
		refs = view.ComponentReferencesFromNodes(viewNodes)
	} else {
		var err error
		refs, err = view.ComponentReferences(viewBody)
		if err != nil {
			return
		}
	}
	for _, ref := range refs {
		if component, ok := resolveComponentRef(ownerPackage, uses, ref, components); ok {
			collectComponentDependencies(component, components, seen)
		}
	}
}

func collectLayoutComponentDependencies(layout gwdkir.Layout, layouts map[string]gwdkir.Layout, components map[string]gwdkir.Component, seenLayouts map[string]bool, seenComponents map[string]bool) {
	key := sourceLayoutKey(layout.Package, layout.ID)
	if seenLayouts[key] {
		return
	}
	seenLayouts[key] = true
	collectComponentDependenciesFromView(layout.Package, layout.Uses, layout.Blocks.ViewBody, layout.Blocks.ViewNodes, components, seenComponents)
	for _, ref := range layout.Layouts {
		if parent, ok := resolveLayoutDependency(layout.Package, layout.Uses, ref, layouts); ok {
			collectLayoutComponentDependencies(parent, layouts, components, seenLayouts, seenComponents)
		}
	}
}

func collectComponentDependencies(component gwdkir.Component, components map[string]gwdkir.Component, seen map[string]bool) {
	key := sourceComponentKey(component.Package, component.Name)
	if seen[key] {
		return
	}
	seen[key] = true
	var refs []string
	if len(component.Blocks.ViewNodes) > 0 {
		refs = view.ComponentReferencesFromNodes(component.Blocks.ViewNodes)
	} else {
		var err error
		refs, err = view.ComponentReferences(component.Blocks.ViewBody)
		if err != nil {
			return
		}
	}
	for _, ref := range refs {
		if child, ok := resolveComponentRef(component.Package, component.Uses, ref, components); ok {
			collectComponentDependencies(child, components, seen)
		}
	}
}

func resolveComponentRef(ownerPackage string, uses []gwdkir.Use, ref string, components map[string]gwdkir.Component) (gwdkir.Component, bool) {
	if alias, name, ok := strings.Cut(ref, "."); ok {
		for _, use := range uses {
			if use.Alias == alias {
				component, exists := components[sourceComponentKey(use.Package, name)]
				return component, exists
			}
		}
		return gwdkir.Component{}, false
	}
	if ownerPackage != "" {
		if component, ok := components[sourceComponentKey(ownerPackage, ref)]; ok {
			return component, true
		}
	}
	component, ok := components[sourceComponentKey("", ref)]
	return component, ok
}

func pageLayoutDependencies(page gwdkir.Page, layouts map[string]gwdkir.Layout) map[string]bool {
	seen := map[string]bool{}
	for _, ref := range page.Layouts {
		if layout, ok := resolvePageLayoutDependency(page.Package, page.Uses, ref, layouts); ok {
			collectLayoutDependencies(layout, layouts, seen)
		}
	}
	return seen
}

func collectLayoutDependencies(layout gwdkir.Layout, layouts map[string]gwdkir.Layout, seen map[string]bool) {
	key := sourceLayoutKey(layout.Package, layout.ID)
	if seen[key] {
		return
	}
	seen[key] = true
	for _, ref := range layout.Layouts {
		if parent, ok := resolveLayoutDependency(layout.Package, layout.Uses, ref, layouts); ok {
			collectLayoutDependencies(parent, layouts, seen)
		}
	}
}

func resolvePageLayoutDependency(ownerPackage string, uses []gwdkir.Use, ref string, layouts map[string]gwdkir.Layout) (gwdkir.Layout, bool) {
	return resolveLayoutDependency(ownerPackage, uses, ref, layouts)
}

func resolveLayoutDependency(ownerPackage string, uses []gwdkir.Use, ref string, layouts map[string]gwdkir.Layout) (gwdkir.Layout, bool) {
	if alias, id, ok := strings.Cut(ref, "."); ok {
		for _, use := range uses {
			if use.Alias == alias {
				layout, exists := layouts[sourceLayoutKey(use.Package, id)]
				return layout, exists
			}
		}
		return gwdkir.Layout{}, false
	}
	if ownerPackage != "" {
		if layout, ok := layouts[sourceLayoutKey(ownerPackage, ref)]; ok {
			return layout, true
		}
	}
	layout, ok := layouts[sourceLayoutKey("", ref)]
	return layout, ok
}

func sourceComponentKey(packageName string, name string) string {
	return packageName + "\x00" + name
}

func sourceLayoutKey(packageName string, id string) string {
	return packageName + "\x00" + id
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortDependencyIndex(index map[string][]string) {
	for key, values := range index {
		sort.Strings(values)
		index[key] = values
	}
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
	canonicalCWD, err := canonicalInputPath(cwd)
	if err != nil {
		return path
	}
	canonicalPath, err := canonicalInputPath(path)
	if err != nil {
		return path
	}
	if rel, ok := relativeInputPath(canonicalCWD, canonicalPath); ok {
		return rel
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

func canonicalInputPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if evaluated, err := filepath.EvalSymlinks(abs); err == nil {
		return evaluated, nil
	}
	var suffix []string
	for current := abs; ; current = filepath.Dir(current) {
		evaluated, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				evaluated = filepath.Join(evaluated, suffix[i])
			}
			return evaluated, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return abs, nil
		}
		suffix = append(suffix, filepath.Base(current))
	}
}

type devInputTracker struct {
	files []string
	dirs  []string
}

type devInputPaths struct {
	files []string
	dirs  []string
}

func newDevInputTracker(plan buildOptions) (devInputTracker, error) {
	paths, err := buildInputPaths(plan)
	if err != nil {
		return devInputTracker{}, err
	}
	return devInputTracker{
		files: uniqueInputPaths(paths.files),
		dirs:  uniqueInputPaths(paths.dirs),
	}, nil
}

func (tracker devInputTracker) snapshot() (inputSnapshot, error) {
	return snapshotInputPaths(tracker.files, tracker.dirs)
}

func buildInputSnapshot(args []string) (inputSnapshot, error) {
	plan, err := loadBuildOptions(args)
	if err != nil {
		return nil, err
	}
	return buildInputSnapshotLoaded(plan)
}

func buildInputSnapshotLoaded(plan buildOptions) (inputSnapshot, error) {
	tracker, err := newDevInputTracker(plan)
	if err != nil {
		return nil, err
	}
	return tracker.snapshot()
}

func buildInputPaths(plan buildOptions) (devInputPaths, error) {
	options := plan.Options
	outputDir := plan.OutputDir
	paths := append([]string(nil), plan.Paths...)
	inputs := devInputPaths{}
	if plan.shouldBuildConfiguredTargets() {
		targets, err := selectBuildTargets(options.Config.Build.Targets, plan.TargetNames)
		if err != nil {
			return devInputPaths{}, err
		}
		for _, target := range targets {
			discovered, dirs, err := discoverBuildFilesAndDirs(options.Config, target.Output, target.Modules, options.ProjectRoot)
			if err != nil {
				return devInputPaths{}, err
			}
			inputs.addFiles(discovered...)
			inputs.addDirs(dirs...)
			css, cssDirs, err := discoverBuildCSSFilesAndDirs(options.Config, target.Output, options.ProjectRoot)
			if err != nil {
				return devInputPaths{}, err
			}
			inputs.addFiles(css...)
			inputs.addDirs(cssDirs...)
		}
	} else if outputDir == "" {
		outputDir = options.Config.Build.Output
		if len(paths) == 0 {
			discovered, dirs, err := discoverBuildFilesAndDirs(options.Config, outputDir, plan.ModuleNames, options.ProjectRoot)
			if err != nil {
				return devInputPaths{}, err
			}
			inputs.addFiles(discovered...)
			inputs.addDirs(dirs...)
		} else {
			inputs.addFiles(paths...)
			inputs.addParentDirs(paths...)
		}
		css, cssDirs, err := discoverBuildCSSFilesAndDirs(options.Config, outputDir, options.ProjectRoot)
		if err != nil {
			return devInputPaths{}, err
		}
		inputs.addFiles(css...)
		inputs.addDirs(cssDirs...)
	} else if len(paths) == 0 {
		discovered, dirs, err := discoverBuildFilesAndDirs(options.Config, outputDir, plan.ModuleNames, options.ProjectRoot)
		if err != nil {
			return devInputPaths{}, err
		}
		inputs.addFiles(discovered...)
		inputs.addDirs(dirs...)
		css, cssDirs, err := discoverBuildCSSFilesAndDirs(options.Config, outputDir, options.ProjectRoot)
		if err != nil {
			return devInputPaths{}, err
		}
		inputs.addFiles(css...)
		inputs.addDirs(cssDirs...)
	} else {
		inputs.addFiles(paths...)
		inputs.addParentDirs(paths...)
		css, cssDirs, err := discoverBuildCSSFilesAndDirs(options.Config, outputDir, options.ProjectRoot)
		if err != nil {
			return devInputPaths{}, err
		}
		inputs.addFiles(css...)
		inputs.addDirs(cssDirs...)
	}
	if strings.TrimSpace(plan.ConfigPath) != "" {
		inputs.addFiles(plan.ConfigPath)
		inputs.addParentDirs(plan.ConfigPath)
	} else {
		configPath := filepath.Join(options.ProjectRoot, "gowdk.config.go")
		if _, err := os.Stat(configPath); err == nil {
			inputs.addFiles(configPath)
			inputs.addParentDirs(configPath)
		}
	}
	return inputs, nil
}

func (paths *devInputPaths) addFiles(files ...string) {
	paths.files = append(paths.files, files...)
}

func (paths *devInputPaths) addDirs(dirs ...string) {
	paths.dirs = append(paths.dirs, dirs...)
}

func (paths *devInputPaths) addParentDirs(files ...string) {
	for _, file := range files {
		if strings.TrimSpace(file) == "" {
			continue
		}
		paths.dirs = append(paths.dirs, filepath.Dir(file))
	}
}

func snapshotInputPaths(files, dirs []string) (inputSnapshot, error) {
	snapshot := inputSnapshot{}
	for _, item := range files {
		info, err := os.Stat(item)
		if os.IsNotExist(err) {
			continue
		}
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
	for _, item := range dirs {
		info, err := os.Stat(item)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		abs, err := filepath.Abs(item)
		if err != nil {
			return nil, err
		}
		hash, err := snapshotDirectoryEntries(item)
		if err != nil {
			return nil, err
		}
		snapshot[abs] = hash
	}
	return snapshot, nil
}

func snapshotDirectoryEntries(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}
	parts := make([]string, 0, len(entries))
	for _, entry := range entries {
		kind := "file"
		switch {
		case entry.IsDir():
			kind = "dir"
		case entry.Type()&os.ModeSymlink != 0:
			kind = "symlink"
		}
		parts = append(parts, kind+"\x00"+entry.Name())
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x01")))
	return fmt.Sprintf("dir:%x", sum), nil
}

func uniqueInputPaths(paths []string) []string {
	seen := map[string]bool{}
	var unique []string
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		abs = filepath.Clean(abs)
		if seen[abs] {
			continue
		}
		seen[abs] = true
		unique = append(unique, abs)
	}
	sort.Strings(unique)
	return unique
}

func discoverBuildCSSFiles(config gowdk.Config, outputDir string, root string) ([]string, error) {
	files, _, err := discoverBuildCSSFilesAndDirs(config, outputDir, root)
	return files, err
}

func discoverBuildCSSFilesAndDirs(config gowdk.Config, outputDir string, root string) ([]string, []string, error) {
	includes := appendPatterns(nil, config.CSS.Include)
	if len(includes) == 1 && includes[0] == buildgen.DisableCSSDiscovery {
		return nil, nil, nil
	}
	if len(includes) == 0 {
		includes = []string{"**/*.css"}
	}

	if strings.TrimSpace(root) == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, nil, err
		}
	}
	excludes := []string{".git/**", "**/.git/**", "vendor/**", "**/vendor/**", "node_modules/**", "**/node_modules/**", ".gowdk/**", "**/.gowdk/**", "dist/**", "**/dist/**"}
	excludes = appendPatterns(excludes, config.CSS.Exclude)
	if pattern := outputExcludePattern(root, outputDir); pattern != "" {
		excludes = append(excludes, pattern)
	}
	return discover.FilesAndDirs(root, includes, excludes)
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

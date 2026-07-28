package discover

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
)

var (
	defaultSourceIncludes = []string{"**/*.gwdk"}
	defaultSourceExcludes = []string{
		".git/**",
		".gowdk/**",
		"bin/**",
		"dist/**",
		"gowdk_cache/**",
		"vendor/**",
		"node_modules/**",
		"**/testdata/**",
	}
)

// Selection is one resolved, configured GOWDK source set.
type Selection struct {
	Root     string
	Includes []string
	Excludes []string
}

// DefaultSourceIncludes returns the default portable source include patterns.
func DefaultSourceIncludes() []string {
	return append([]string(nil), defaultSourceIncludes...)
}

// DefaultSourceExcludes returns the default portable source exclude patterns.
func DefaultSourceExcludes() []string {
	return append([]string(nil), defaultSourceExcludes...)
}

// ConfiguredSelection resolves project and module source rules into one
// reusable source selection.
func ConfiguredSelection(config gowdk.Config, outputDir string, moduleNames []string, root string) (Selection, error) {
	if strings.TrimSpace(root) == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return Selection{}, err
		}
	}
	modules, err := selectedModules(config.Modules, moduleNames)
	if err != nil {
		return Selection{}, err
	}

	includes := sourceIncludes(config, modules, len(moduleNames) > 0)
	excludes := sourceExcludes(config, modules)
	for _, output := range configuredGeneratedDirs(config, outputDir) {
		if pattern := OutputExcludePattern(root, output); pattern != "" {
			excludes = appendPatternOnce(excludes, pattern)
		}
	}
	return Selection{
		Root:     root,
		Includes: includes,
		Excludes: excludes,
	}, nil
}

// SelectModules resolves explicit module names, or returns every configured
// module when no names are selected.
func SelectModules(modules []gowdk.ModuleConfig, moduleNames []string) ([]gowdk.ModuleConfig, error) {
	return selectedModules(modules, moduleNames)
}

// Files returns the files in the selection.
func (selection Selection) Files() ([]string, error) {
	return Files(selection.Root, selection.Includes, selection.Excludes)
}

// FilesAndDirs returns matching files and traversed, non-excluded directories.
func (selection Selection) FilesAndDirs() ([]string, []string, error) {
	return FilesAndDirs(selection.Root, selection.Includes, selection.Excludes)
}

// Matches reports whether path belongs to the configured source selection.
// The path does not need to exist, so editor overlays can use this for unsaved
// documents.
func (selection Selection) Matches(path string) bool {
	if strings.TrimSpace(selection.Root) == "" || strings.TrimSpace(path) == "" {
		return false
	}
	root, err := filepath.Abs(selection.Root)
	if err != nil {
		return false
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, absolute)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return false
	}
	rel = filepath.ToSlash(rel)
	return matchesAny(compileGlobs(selection.Includes), rel) &&
		!matchesAny(compileGlobs(selection.Excludes), rel)
}

// Fingerprint returns a deterministic identity for the selection rules.
func (selection Selection) Fingerprint() string {
	parts := []string{"root:" + filepath.ToSlash(filepath.Clean(selection.Root))}
	for _, pattern := range selection.Includes {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			parts = append(parts, "include:"+filepath.ToSlash(pattern))
		}
	}
	for _, pattern := range selection.Excludes {
		if pattern = strings.TrimSpace(pattern); pattern != "" {
			parts = append(parts, "exclude:"+filepath.ToSlash(pattern))
		}
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%x", sum)
}

// OutputExcludePattern returns a project-relative exclusion for generated
// output inside root.
func OutputExcludePattern(root, outputDir string) string {
	if strings.TrimSpace(outputDir) == "" {
		return ""
	}
	absOutput := outputDir
	if !filepath.IsAbs(absOutput) {
		absOutput = filepath.Join(root, outputDir)
	}
	rel, err := filepath.Rel(root, absOutput)
	if err != nil {
		return ""
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(rel)) + "/**"
}

func selectedModules(modules []gowdk.ModuleConfig, moduleNames []string) ([]gowdk.ModuleConfig, error) {
	if len(moduleNames) == 0 {
		return modules, nil
	}

	byName := make(map[string]gowdk.ModuleConfig)
	for _, module := range modules {
		byName[module.Name] = module
	}

	selected := make([]gowdk.ModuleConfig, 0, len(moduleNames))
	for _, name := range moduleNames {
		module, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("module %q is not configured", name)
		}
		selected = append(selected, module)
	}
	return selected, nil
}

func sourceIncludes(config gowdk.Config, modules []gowdk.ModuleConfig, modulesOnly bool) []string {
	var includes []string
	if !modulesOnly {
		includes = appendPatterns(includes, config.Source.Include)
	}
	for _, module := range modules {
		if hasPatterns(module.Source.Include) {
			includes = appendPatterns(includes, module.Source.Include)
			continue
		}
		if pattern := defaultModuleInclude(module.Name); pattern != "" {
			includes = append(includes, pattern)
		}
	}
	if len(includes) > 0 {
		return includes
	}
	return DefaultSourceIncludes()
}

func sourceExcludes(config gowdk.Config, modules []gowdk.ModuleConfig) []string {
	excludes := DefaultSourceExcludes()
	excludes = appendPatterns(excludes, config.Source.Exclude)
	for _, module := range modules {
		excludes = appendPatterns(excludes, module.Source.Exclude)
	}
	return excludes
}

func hasPatterns(patterns []string) bool {
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) != "" {
			return true
		}
	}
	return false
}

func appendPatterns(values, patterns []string) []string {
	for _, pattern := range patterns {
		if strings.TrimSpace(pattern) == "" {
			continue
		}
		values = append(values, pattern)
	}
	return values
}

func appendPatternOnce(patterns []string, pattern string) []string {
	for _, existing := range patterns {
		if existing == pattern {
			return patterns
		}
	}
	return append(patterns, pattern)
}

func configuredGeneratedDirs(config gowdk.Config, outputDir string) []string {
	dirs := []string{outputDir, config.Build.Output}
	for _, target := range config.Build.Targets {
		output := strings.TrimSpace(target.Output)
		if output == "" {
			if name := strings.Trim(strings.TrimSpace(target.Name), "/"); name != "" {
				output = filepath.Join(".gowdk", "output", name)
			}
		}
		dirs = append(dirs,
			output,
			target.App,
			target.BackendApp,
			target.WorkerApp,
			target.CronApp,
		)
	}
	return dirs
}

func defaultModuleInclude(name string) string {
	name = strings.Trim(strings.TrimSpace(name), "/")
	if name == "" {
		return ""
	}
	name = filepath.ToSlash(filepath.Clean(name))
	if name == "." {
		return ""
	}
	return name + "/**/*.gwdk"
}

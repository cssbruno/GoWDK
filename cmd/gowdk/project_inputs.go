package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/discover"
	"github.com/cssbruno/gowdk/internal/project"
	"github.com/cssbruno/gowdk/runtime/envfile"
)

func loadBuildConfig(options *cliOptions, configPath string) error {
	return loadProjectConfig(options, configPath)
}

func loadProjectConfig(options *cliOptions, configPath string) error {
	allowMissingBackend := options.AllowMissingBackend
	obfuscateAssets := options.ObfuscateAssets
	projectRoot, err := resolveProjectRoot(configPath)
	if err != nil {
		return err
	}
	if err := loadProjectEnvFile(options, projectRoot); err != nil {
		return err
	}
	config, err := project.LoadConfig(configPath)
	if err != nil {
		return err
	}
	config.Addons = append(config.Addons, options.Config.Addons...)
	if allowMissingBackend {
		config.Build.AllowMissingBackend = true
	}
	if obfuscateAssets {
		config.Build.ObfuscateAssets = true
		config.Build.Mode = gowdk.Production
	}
	options.Config = config
	options.ProjectRoot = projectRoot
	options.AllowMissingBackend = allowMissingBackend
	options.ObfuscateAssets = obfuscateAssets
	return nil
}

func loadProjectEnvFile(options *cliOptions, projectRoot string) error {
	path, explicit, err := envfile.LookupPath(projectRoot, options.EnvFilePath)
	if err != nil {
		return err
	}
	result, err := envfile.LoadIntoEnv(path, explicit)
	if err != nil {
		if explicit {
			return fmt.Errorf("load env file %q: %w", path, err)
		}
		return fmt.Errorf("load discovered env file %q: %w", path, err)
	}
	options.EnvFilePath = result.Path
	options.EnvFileLoaded = result.Loaded
	options.EnvFileExplicit = result.Explicit
	options.EnvFileApplied = append([]string(nil), result.Applied...)
	options.EnvFileSkipped = append([]string(nil), result.Skipped...)
	return nil
}

func resolveProjectRoot(configPath string) (string, error) {
	if strings.TrimSpace(configPath) != "" {
		absolute, err := filepath.Abs(configPath)
		if err != nil {
			return "", err
		}
		return filepath.Dir(absolute), nil
	}
	return os.Getwd()
}

func discoverBuildFiles(config gowdk.Config, outputDir string, moduleNames []string, root string) ([]string, error) {
	return discoverConfiguredFiles(config, outputDir, moduleNames, root)
}

func discoverBuildFilesAndDirs(config gowdk.Config, outputDir string, moduleNames []string, root string) ([]string, []string, error) {
	return discoverConfiguredFilesAndDirs(config, outputDir, moduleNames, root)
}

func discoverProjectFiles(config gowdk.Config, moduleNames []string, root string) ([]string, error) {
	return discoverConfiguredFiles(config, config.Build.Output, moduleNames, root)
}

func discoverConfiguredFiles(config gowdk.Config, outputDir string, moduleNames []string, root string) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	modules, err := buildModules(config.Modules, moduleNames)
	if err != nil {
		return nil, err
	}

	includes := buildSourceIncludes(config, modules, len(moduleNames) > 0)
	excludes := buildSourceExcludes(config, modules)
	if pattern := outputExcludePattern(root, outputDir); pattern != "" {
		excludes = append(excludes, pattern)
	}
	return discover.Files(root, includes, excludes)
}

func discoverConfiguredFilesAndDirs(config gowdk.Config, outputDir string, moduleNames []string, root string) ([]string, []string, error) {
	if strings.TrimSpace(root) == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return nil, nil, err
		}
	}
	modules, err := buildModules(config.Modules, moduleNames)
	if err != nil {
		return nil, nil, err
	}

	includes := buildSourceIncludes(config, modules, len(moduleNames) > 0)
	excludes := buildSourceExcludes(config, modules)
	if pattern := outputExcludePattern(root, outputDir); pattern != "" {
		excludes = append(excludes, pattern)
	}
	return discover.FilesAndDirs(root, includes, excludes)
}

func loadCommandInputs(args []string, command string, allowJSON bool) (cliOptions, []string, error) {
	options, configPath, moduleNames, paths, err := parseProjectOptions(args, command, allowJSON)
	if err != nil {
		return options, nil, err
	}
	if err := loadProjectConfig(&options, configPath); err != nil {
		return options, nil, err
	}
	if len(paths) == 0 {
		discovered, err := discoverProjectFiles(options.Config, moduleNames, options.ProjectRoot)
		if err != nil {
			return options, nil, err
		}
		if len(discovered) == 0 {
			return options, nil, fmt.Errorf("no .gwdk files found")
		}
		paths = discovered
	}
	return options, paths, nil
}

func buildModules(modules []gowdk.ModuleConfig, moduleNames []string) ([]gowdk.ModuleConfig, error) {
	if len(moduleNames) == 0 {
		return modules, nil
	}

	byName := make(map[string]gowdk.ModuleConfig)
	for _, module := range modules {
		byName[module.Name] = module
	}

	var selected []gowdk.ModuleConfig
	for _, name := range moduleNames {
		module, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("module %q is not configured", name)
		}
		selected = append(selected, module)
	}
	return selected, nil
}

func buildSourceIncludes(config gowdk.Config, modules []gowdk.ModuleConfig, modulesOnly bool) []string {
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

	return defaultSourceIncludes
}

func buildSourceExcludes(config gowdk.Config, modules []gowdk.ModuleConfig) []string {
	excludes := append([]string{}, defaultSourceExcludes...)
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

func outputExcludePattern(root, outputDir string) string {
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
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(rel)) + "/**"
}

func parseProjectOptions(args []string, command string, allowJSON bool) (cliOptions, string, []string, []string, error) {
	var options cliOptions
	var configPath string
	var envFilePath string
	var moduleNames []string
	var paths []string
	usage := projectCommandUsage(command, allowJSON)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-h" || arg == "--help":
			return options, "", nil, nil, errors.New(usage)
		case arg == "--ssr":
			options.Config.Addons = append(options.Config.Addons, ssr.Addon())
		case arg == "--json" && allowJSON:
			options.JSON = true
		case arg == "--json":
			return options, "", nil, nil, fmt.Errorf("unknown %s flag %q", command, arg)
		case arg == "--warnings-as-errors":
			if command != "check" {
				return options, "", nil, nil, fmt.Errorf("unknown %s flag %q", command, arg)
			}
			options.WarningsAsErrors = true
		case arg == "--config":
			i++
			if i >= len(args) {
				return options, "", nil, nil, errors.New(usage)
			}
			configPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case arg == "--env-file":
			i++
			if i >= len(args) {
				return options, "", nil, nil, errors.New(usage)
			}
			envFilePath = args[i]
			options.EnvFilePath = envFilePath
		case strings.HasPrefix(arg, "--env-file="):
			envFilePath = strings.TrimPrefix(arg, "--env-file=")
			options.EnvFilePath = envFilePath
		case arg == "--module":
			i++
			if i >= len(args) {
				return options, "", nil, nil, errors.New(usage)
			}
			moduleNames = appendModuleNames(moduleNames, args[i])
		case strings.HasPrefix(arg, "--module="):
			moduleNames = appendModuleNames(moduleNames, strings.TrimPrefix(arg, "--module="))
		case strings.HasPrefix(arg, "-"):
			return options, "", nil, nil, fmt.Errorf("unknown %s flag %q", command, arg)
		default:
			paths = append(paths, arg)
		}
	}
	options.EnvFilePath = envFilePath
	return options, configPath, moduleNames, paths, nil
}

func projectCommandUsage(command string, allowJSON bool) string {
	if allowJSON {
		warningsFlag := ""
		if command == "check" {
			warningsFlag = " [--warnings-as-errors]"
		}
		return fmt.Sprintf("usage: gowdk %s [--config <file>] [--env-file <file>] [--module <name>] [--json]%s [--ssr] [files...]", command, warningsFlag)
	}
	return fmt.Sprintf("usage: gowdk %s [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...]", command)
}

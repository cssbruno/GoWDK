package gowdkcmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/discover"
	"github.com/cssbruno/gowdk/internal/project"
	"github.com/cssbruno/gowdk/runtime/envfile"
)

func loadBuildConfig(options *cliOptions, configPath string, paths []string) error {
	return loadProjectConfigWithPaths(options, configPath, paths)
}

var nativeConfigExecution struct {
	mu          sync.Mutex
	active      bool
	config      *gowdk.Config
	projectRoot string
}

// RunWithConfig executes a command using a native project config already loaded
// in the current process. It is used by generated project helpers so
// gowdk.Config never has to cross a process boundary.
func RunWithConfig(args []string, config *gowdk.Config, projectRoot string) error {
	nativeConfigExecution.mu.Lock()
	previousActive := nativeConfigExecution.active
	previousConfig := nativeConfigExecution.config
	previousProjectRoot := nativeConfigExecution.projectRoot
	nativeConfigExecution.active = true
	nativeConfigExecution.config = config
	nativeConfigExecution.projectRoot = projectRoot
	nativeConfigExecution.mu.Unlock()
	defer func() {
		nativeConfigExecution.mu.Lock()
		nativeConfigExecution.active = previousActive
		nativeConfigExecution.config = previousConfig
		nativeConfigExecution.projectRoot = previousProjectRoot
		nativeConfigExecution.mu.Unlock()
	}()
	return Run(args)
}

func loadProjectConfig(options *cliOptions, configPath string) error {
	return loadProjectConfigWithPaths(options, configPath, nil)
}

func loadProjectConfigWithPaths(options *cliOptions, configPath string, paths []string) error {
	allowMissingBackend := options.AllowMissingBackend
	obfuscateAssets := options.ObfuscateAssets
	projectRoot, resolvedConfigPath, err := resolveProjectRoot(configPath, options.ProjectRoot, paths)
	if err != nil {
		return err
	}
	nativeConfigExecution.mu.Lock()
	nativeActive := nativeConfigExecution.active
	nativeConfig := nativeConfigExecution.config
	nativeProjectRoot := nativeConfigExecution.projectRoot
	nativeConfigExecution.mu.Unlock()
	if nativeActive {
		if strings.TrimSpace(nativeProjectRoot) != "" {
			projectRoot = nativeProjectRoot
		}
		if nativeConfig == nil {
			return fmt.Errorf("native project config is missing")
		}
		if err := loadProjectEnvFile(options, projectRoot); err != nil {
			return err
		}
		config := *nativeConfig
		if err := validateNativeProjectConfigStructure(resolvedConfigPath, config); err != nil {
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
	if err := loadProjectEnvFile(options, projectRoot); err != nil {
		return err
	}
	config, err := project.LoadConfigStructural(resolvedConfigPath)
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

func validateNativeProjectConfigStructure(path string, config gowdk.Config) error {
	if strings.TrimSpace(path) == "" {
		path = project.DefaultConfigFile
	}
	if err := config.Env.Validate(nil); err != nil {
		return fmt.Errorf("%s env contract: %w", path, err)
	}
	if err := config.Lifecycle.Validate(); err != nil {
		return fmt.Errorf("%s lifecycle contract: %w", path, err)
	}
	if err := config.I18N.Validate(); err != nil {
		return fmt.Errorf("%s i18n policy: %w", path, err)
	}
	if err := config.Build.CORS.Validate(); err != nil {
		return fmt.Errorf("%s CORS policy: %w", path, err)
	}
	if err := gowdk.ValidateAddons(config.Addons); err != nil {
		return fmt.Errorf("%s addons: %w", path, err)
	}
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

func resolveProjectRoot(configPath string, projectRoot string, paths []string) (string, string, error) {
	if strings.TrimSpace(configPath) != "" {
		absolute, err := filepath.Abs(configPath)
		if err != nil {
			return "", "", err
		}
		root := filepath.Dir(absolute)
		if strings.TrimSpace(projectRoot) != "" {
			explicitRoot, err := filepath.Abs(projectRoot)
			if err != nil {
				return "", "", err
			}
			if !pathWithinRoot(absolute, explicitRoot) {
				return "", "", fmt.Errorf("--config %q is outside --project-root %q", configPath, projectRoot)
			}
			root = explicitRoot
		}
		if err := validateProjectInputPaths(root, paths); err != nil {
			return "", "", err
		}
		return root, absolute, nil
	}
	if strings.TrimSpace(projectRoot) != "" {
		root, err := filepath.Abs(projectRoot)
		if err != nil {
			return "", "", err
		}
		config := filepath.Join(root, project.DefaultConfigFile)
		if _, err := os.Stat(config); err != nil {
			if os.IsNotExist(err) {
				return "", "", fmt.Errorf("%s is required under --project-root %q; run \"gowdk init\" or pass --config <file>", project.DefaultConfigFile, projectRoot)
			}
			return "", "", err
		}
		if err := validateProjectInputPaths(root, paths); err != nil {
			return "", "", err
		}
		return root, config, nil
	}
	root, config, err := discoverProjectRoot(paths)
	if err != nil {
		return "", "", err
	}
	if err := validateProjectInputPaths(root, paths); err != nil {
		return "", "", err
	}
	return root, config, nil
}

func discoverProjectRoot(paths []string) (string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	if root, config, ok, err := findProjectRootUpward(cwd); err != nil || ok {
		if err != nil {
			return "", "", err
		}
		return root, config, nil
	}
	roots := map[string]string{}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return "", "", err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			continue
		}
		start := absolute
		if !info.IsDir() {
			start = filepath.Dir(absolute)
		}
		root, config, ok, err := findProjectRootUpward(start)
		if err != nil {
			return "", "", err
		}
		if ok {
			roots[root] = config
		}
	}
	if len(roots) == 1 {
		for root, config := range roots {
			return root, config, nil
		}
	}
	if len(roots) > 1 {
		var names []string
		for root := range roots {
			names = append(names, root)
		}
		sort.Strings(names)
		return "", "", fmt.Errorf("explicit files resolve to multiple GOWDK project roots: %s; pass --project-root or --config", strings.Join(names, ", "))
	}
	return "", "", fmt.Errorf("%s is required; run \"gowdk init\", pass --config <file>, or pass --project-root <dir>", project.DefaultConfigFile)
}

func findProjectRootUpward(start string) (string, string, bool, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", "", false, err
	}
	for {
		config := filepath.Join(dir, project.DefaultConfigFile)
		if _, err := os.Stat(config); err == nil {
			return dir, config, true, nil
		} else if err != nil && !os.IsNotExist(err) {
			return "", "", false, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", false, nil
		}
		dir = parent
	}
}

func validateProjectInputPaths(root string, paths []string) error {
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if _, err := os.Stat(absolute); err != nil {
			continue
		}
		if !pathWithinRoot(absolute, root) {
			return fmt.Errorf("input file %q is outside project root %q; pass --project-root or --config for the owning project", path, root)
		}
	}
	return nil
}

func pathWithinRoot(path string, root string) bool {
	if realPath, err := filepath.EvalSymlinks(path); err == nil {
		path = realPath
	}
	if realRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = realRoot
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
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
	inputs, err := configuredDiscoveryInputs(config, outputDir, moduleNames, root)
	if err != nil {
		return nil, err
	}
	return discover.Files(inputs.root, inputs.includes, inputs.excludes)
}

func discoverConfiguredFilesAndDirs(config gowdk.Config, outputDir string, moduleNames []string, root string) ([]string, []string, error) {
	inputs, err := configuredDiscoveryInputs(config, outputDir, moduleNames, root)
	if err != nil {
		return nil, nil, err
	}
	return discover.FilesAndDirs(inputs.root, inputs.includes, inputs.excludes)
}

type discoveryInputs struct {
	root     string
	includes []string
	excludes []string
}

func configuredDiscoveryInputs(config gowdk.Config, outputDir string, moduleNames []string, root string) (discoveryInputs, error) {
	if strings.TrimSpace(root) == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return discoveryInputs{}, err
		}
	}
	modules, err := buildModules(config.Modules, moduleNames)
	if err != nil {
		return discoveryInputs{}, err
	}

	includes := buildSourceIncludes(config, modules, len(moduleNames) > 0)
	excludes := buildSourceExcludes(config, modules)
	if pattern := outputExcludePattern(root, outputDir); pattern != "" {
		excludes = append(excludes, pattern)
	}
	return discoveryInputs{root: root, includes: includes, excludes: excludes}, nil
}

func loadCommandInputs(args []string, command string, allowJSON bool) (cliOptions, []string, error) {
	options, configPath, moduleNames, paths, err := parseProjectOptions(args, command, allowJSON)
	if err != nil {
		return options, nil, err
	}
	if command == "check" {
		standalone, standaloneErr := shouldRunStandaloneCheck(options, configPath, moduleNames, paths)
		if standaloneErr != nil {
			return options, nil, standaloneErr
		}
		if standalone {
			options.Standalone = true
			return options, paths, nil
		}
	}
	if err := loadProjectConfigWithPaths(&options, configPath, paths); err != nil {
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

func shouldRunStandaloneCheck(options cliOptions, configPath string, moduleNames []string, paths []string) (bool, error) {
	if options.Standalone {
		if err := validateStandaloneCheckOptions(options, configPath, moduleNames, paths); err != nil {
			return false, err
		}
		return true, nil
	}
	if configPath != "" || options.ProjectRoot != "" || len(paths) == 0 {
		return false, nil
	}
	found, err := projectRootVisible(paths)
	if err != nil {
		return false, err
	}
	if found {
		return false, nil
	}
	if err := validateStandaloneCheckOptions(options, configPath, moduleNames, paths); err != nil {
		return false, err
	}
	return true, nil
}

func projectRootVisible(paths []string) (bool, error) {
	if _, _, found, err := findProjectRootUpward("."); err != nil || found {
		return found, err
	}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return false, err
		}
		info, err := os.Stat(absolute)
		if err != nil {
			continue
		}
		start := absolute
		if !info.IsDir() {
			start = filepath.Dir(absolute)
		}
		if _, _, found, err := findProjectRootUpward(start); err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func validateStandaloneCheckOptions(options cliOptions, configPath string, moduleNames []string, paths []string) error {
	switch {
	case len(paths) == 0:
		return fmt.Errorf("standalone check requires at least one explicit .gwdk file")
	case configPath != "":
		return fmt.Errorf("--standalone cannot be combined with --config")
	case options.ProjectRoot != "":
		return fmt.Errorf("--standalone cannot be combined with --project-root")
	case options.EnvFilePath != "":
		return fmt.Errorf("--standalone cannot be combined with --env-file")
	case len(moduleNames) > 0:
		return fmt.Errorf("--standalone cannot be combined with --module")
	case len(options.Config.Addons) > 0:
		return fmt.Errorf("--standalone cannot be combined with --ssr")
	}
	return nil
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
		if value, next, ok, missing := consumeValueFlag(args, i, "--config", true); ok {
			if missing {
				return options, "", nil, nil, errors.New(usage)
			}
			configPath = value
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--project-root", true); ok {
			if missing {
				return options, "", nil, nil, errors.New(usage)
			}
			options.ProjectRoot = value
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--env-file", true); ok {
			if missing {
				return options, "", nil, nil, errors.New(usage)
			}
			envFilePath = value
			options.EnvFilePath = envFilePath
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--module", true); ok {
			if missing {
				return options, "", nil, nil, errors.New(usage)
			}
			moduleNames = appendModuleNames(moduleNames, value)
			i = next
			continue
		}
		switch {
		case arg == "-h" || arg == "--help":
			return options, "", nil, nil, errors.New(usage)
		case arg == "--standalone":
			if command != "check" {
				return options, "", nil, nil, fmt.Errorf("unknown %s flag %q", command, arg)
			}
			options.Standalone = true
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
	standaloneFlag := ""
	if command == "check" {
		standaloneFlag = " [--standalone]"
	}
	if allowJSON {
		warningsFlag := ""
		if command == "check" {
			warningsFlag = " [--warnings-as-errors]"
		}
		return fmt.Sprintf("usage: gowdk %s [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json]%s%s [--ssr] [files...]", command, warningsFlag, standaloneFlag)
	}
	return fmt.Sprintf("usage: gowdk %s [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>]%s [--ssr] [files...]", command, standaloneFlag)
}

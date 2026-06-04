package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/gowdk/gowdk"
	"github.com/gowdk/gowdk/addons/ssr"
	"github.com/gowdk/gowdk/internal/appgen"
	"github.com/gowdk/gowdk/internal/codegen"
	"github.com/gowdk/gowdk/internal/discover"
	"github.com/gowdk/gowdk/internal/lang"
	"github.com/gowdk/gowdk/internal/lsp"
	"github.com/gowdk/gowdk/internal/project"
	"github.com/gowdk/gowdk/internal/staticgen"
)

const version = "0.1.0-dev"

var (
	defaultSourceIncludes = []string{"**/*.gwdk"}
	defaultSourceExcludes = []string{".git/**", "vendor/**", "node_modules/**"}
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}

	switch args[0] {
	case "version":
		fmt.Println(version)
	case "init":
		return initProject(args[1:])
	case "tokens":
		return tokens(args[1:])
	case "fmt":
		return format(args[1:])
	case "check":
		return check(args[1:])
	case "manifest":
		return manifestJSON(args[1:])
	case "sitemap":
		return siteMapJSON(args[1:])
	case "routes":
		return routesJSON(args[1:])
	case "build":
		return build(args[1:])
	case "watch":
		return watch(args[1:])
	case "serve":
		return serve(args[1:])
	case "lsp":
		return languageServer(args[1:])
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

func usage() {
	fmt.Println("gowdk " + version)
	fmt.Println("compile-first Go web kit: static/action-first, SSR optional")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  version                  print CLI version")
	fmt.Println("  init [--force] [dir]     scaffold a starter GOWDK project")
	fmt.Println("  tokens <file.gwdk>       print language tokens")
	fmt.Println("  fmt [--write] <files>    format .gwdk files")
	fmt.Println("  check [--config <file>] [--module <name>] [--json] [--ssr] [files...] parse and validate .gwdk files")
	fmt.Println("  manifest [--config <file>] [--module <name>] [--ssr] [files...] print validated manifest JSON")
	fmt.Println("  sitemap [--config <file>] [--module <name>] [--ssr] [files...] print editor site-map JSON")
	fmt.Println("  routes [--config <file>] [--module <name>] [--ssr] [files...] print generated route bindings JSON")
	fmt.Println("  build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...] emit static output")
	fmt.Println("  watch [--once] [--interval <duration>] [build flags...] rebuild static output when inputs change")
	fmt.Println("  serve --dir <dir> [--addr <addr>] serve generated static output locally")
	fmt.Println("  lsp [--ssr]              start the language server over stdio")
}

func initProject(args []string) error {
	options, err := parseInitOptions(args)
	if err != nil {
		return err
	}
	root, err := filepath.Abs(options.Dir)
	if err != nil {
		return err
	}
	files := []initFile{
		{
			Path: "gowdk.config.go",
			Body: `package app

import "github.com/gowdk/gowdk"

var Config = gowdk.Config{
	AppName: "GOWDK App",
	Source: gowdk.SourceConfig{
		Include: []string{"src/**/*.gwdk"},
	},
	Build: gowdk.BuildConfig{
		Output: "dist/site",
	},
	CSS: gowdk.CSSConfig{
		Include: []string{"styles/**/*.css"},
		Default: []string{"global"},
	},
}
`,
		},
		{
			Path: "src/pages/home.page.gwdk",
			Body: `@page home
@route "/"
@css default page

build {
  => { title: "Hello from GOWDK" }
}

view {
  <main class="home">
    <Hero title="{title}">
      <p>Compile-first Go web output.</p>
    </Hero>
  </main>
}
`,
		},
		{
			Path: "src/components/hero.cmp.gwdk",
			Body: `@component Hero

props {
  title string
}

view {
  <section class="hero">
    <h1>{title}</h1>
    <slot />
  </section>
}
`,
		},
		{
			Path: "styles/global.css",
			Body: `:root {
  color-scheme: light;
  font-family: system-ui, sans-serif;
}

body {
  margin: 0;
}

.home {
  max-width: 64rem;
  margin: 0 auto;
  padding: 4rem 1.5rem;
}

.hero {
  display: grid;
  gap: 1rem;
}
`,
		},
	}
	for _, file := range files {
		target := filepath.Join(root, filepath.FromSlash(file.Path))
		if err := writeInitFile(target, file.Body, options.Force); err != nil {
			return err
		}
		fmt.Println(target)
	}
	fmt.Println("Run: gowdk build")
	return nil
}

type initOptions struct {
	Dir   string
	Force bool
}

type initFile struct {
	Path string
	Body string
}

func parseInitOptions(args []string) (initOptions, error) {
	options := initOptions{Dir: "."}
	for _, arg := range args {
		switch arg {
		case "--force":
			options.Force = true
		case "-h", "--help":
			return initOptions{}, fmt.Errorf("usage: gowdk init [--force] [dir]")
		default:
			if strings.HasPrefix(arg, "-") {
				return initOptions{}, fmt.Errorf("unknown init flag %q", arg)
			}
			if options.Dir != "." {
				return initOptions{}, fmt.Errorf("usage: gowdk init [--force] [dir]")
			}
			options.Dir = arg
		}
	}
	if strings.TrimSpace(options.Dir) == "" {
		return initOptions{}, fmt.Errorf("init directory is required")
	}
	return options, nil
}

func writeInitFile(path string, body string, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists; rerun with --force to overwrite starter files", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func tokens(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gowdk tokens <file.gwdk>")
	}
	source, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	tokens, diagnostics := lang.Lex(string(source))
	for _, diagnostic := range diagnostics {
		diagnostic.File = args[0]
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	for _, token := range tokens {
		fmt.Printf("%d:%d\t%s\t%q\n", token.Pos.Line, token.Pos.Column, token.Kind, token.Lexeme)
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("tokenization failed")
	}
	return nil
}

func format(args []string) error {
	write := false
	var paths []string
	for _, arg := range args {
		if arg == "--write" {
			write = true
			continue
		}
		paths = append(paths, arg)
	}
	if len(paths) == 0 {
		return fmt.Errorf("usage: gowdk fmt [--write] <files>")
	}

	for _, path := range paths {
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		formatted := lang.Format(source)
		if write {
			if err := os.WriteFile(path, formatted, 0o644); err != nil {
				return err
			}
			continue
		}
		if len(paths) > 1 {
			fmt.Printf("==> %s <==\n", path)
		}
		fmt.Print(string(formatted))
	}
	return nil
}

func check(args []string) error {
	options, paths, err := loadCommandInputs(args, "check", true)
	if err != nil {
		return err
	}

	if options.JSON {
		payload, diagnostics := lang.CheckJSON(options.Config, paths)
		if len(payload) > 0 {
			fmt.Print(string(payload))
		}
		if diagnostics.HasErrors() {
			return fmt.Errorf("check failed")
		}
		return nil
	}

	_, diagnostics := lang.CheckFiles(options.Config, paths)
	if len(diagnostics) == 0 {
		fmt.Println("ok")
		return nil
	}
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("check failed")
	}
	return nil
}

func manifestJSON(args []string) error {
	options, paths, err := loadCommandInputs(args, "manifest", false)
	if err != nil {
		return err
	}

	payload, diagnostics := lang.ManifestJSON(options.Config, paths)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("manifest failed")
	}
	fmt.Print(string(payload))
	return nil
}

func siteMapJSON(args []string) error {
	options, paths, err := loadCommandInputs(args, "sitemap", false)
	if err != nil {
		return err
	}

	payload, diagnostics := lang.SiteMapJSON(options.Config, paths)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("sitemap failed")
	}
	fmt.Print(string(payload))
	return nil
}

func routesJSON(args []string) error {
	options, paths, err := loadCommandInputs(args, "routes", false)
	if err != nil {
		return err
	}

	app, diagnostics := lang.CheckFiles(options.Config, paths)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("routes failed")
	}

	bindings, err := codegen.BuildRouteBindings(options.Config, app)
	if err != nil {
		return err
	}
	payload, err := json.MarshalIndent(routeBindingsJSON(bindings), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(payload))
	return nil
}

func build(args []string) error {
	options, outputDir, appDir, binaryPath, configPath, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(binaryPath) != "" && strings.TrimSpace(appDir) == "" {
		return fmt.Errorf("gowdk build --bin requires --app <dir>")
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return err
	}
	if outputDir == "" {
		outputDir = options.Config.Build.Output
	}
	if outputDir == "" {
		return fmt.Errorf("usage: gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]")
	}
	if len(paths) == 0 {
		discovered, err := discoverBuildFiles(options.Config, outputDir, moduleNames)
		if err != nil {
			return err
		}
		if len(discovered) == 0 {
			return fmt.Errorf("no .gwdk files found")
		}
		paths = discovered
	}

	app, diagnostics := lang.ParseBuildFiles(paths)
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(os.Stderr, diagnostic.String())
	}
	if diagnostics.HasErrors() {
		return fmt.Errorf("build failed")
	}

	result, err := staticgen.Build(options.Config, app, outputDir)
	if err != nil {
		return err
	}
	for _, artifact := range result.Artifacts {
		fmt.Println(artifact.Path)
	}
	for _, artifact := range result.CSSArtifacts {
		fmt.Println(artifact.Path)
	}
	if result.RouteManifestPath != "" {
		fmt.Println(result.RouteManifestPath)
	}
	if result.AssetManifestPath != "" {
		fmt.Println(result.AssetManifestPath)
	}
	if strings.TrimSpace(appDir) != "" {
		actions, err := appgen.ActionRoutes(app)
		if err != nil {
			return err
		}
		app, err := appgen.GenerateWithOptions(outputDir, appDir, appgen.Options{Actions: actions})
		if err != nil {
			return err
		}
		fmt.Println(app.ModulePath)
		fmt.Println(app.MainPath)
		if strings.TrimSpace(binaryPath) != "" {
			built, err := appgen.BuildBinary(app.AppDir, binaryPath)
			if err != nil {
				return err
			}
			fmt.Println(built)
		}
	}
	return nil
}

func watch(args []string) error {
	options, err := parseWatchOptions(args)
	if err != nil {
		return err
	}
	if options.Once {
		return build(options.BuildArgs)
	}

	fmt.Printf("Watching GOWDK inputs every %s\n", options.Interval)
	if err := build(options.BuildArgs); err != nil {
		fmt.Fprintln(os.Stderr, err)
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
		previous = current
		fmt.Printf("Change detected at %s\n", time.Now().Format(time.RFC3339))
		if err := build(options.BuildArgs); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func serve(args []string) error {
	dir, addr, err := parseServeOptions(args)
	if err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("serve directory %q is not a directory", dir)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           staticFileHandler(absDir),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	fmt.Printf("Serving %s at http://%s\n", absDir, addr)
	return server.ListenAndServe()
}

type watchOptions struct {
	BuildArgs []string
	Once      bool
	Interval  time.Duration
}

func parseWatchOptions(args []string) (watchOptions, error) {
	options := watchOptions{Interval: time.Second}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--once":
			options.Once = true
		case arg == "--interval":
			i++
			if i >= len(args) {
				return watchOptions{}, fmt.Errorf("usage: gowdk watch [--once] [--interval <duration>] [build flags...]")
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

type inputSnapshot map[string]time.Time

func buildInputSnapshot(args []string) (inputSnapshot, error) {
	options, outputDir, _, _, configPath, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return nil, err
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return nil, err
	}
	if outputDir == "" {
		outputDir = options.Config.Build.Output
	}
	if len(paths) == 0 {
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
		snapshot[abs] = info.ModTime()
	}
	return snapshot, nil
}

func (snapshot inputSnapshot) same(other inputSnapshot) bool {
	if len(snapshot) != len(other) {
		return false
	}
	for path, modTime := range snapshot {
		if !modTime.Equal(other[path]) {
			return false
		}
	}
	return true
}

func loadBuildConfig(options *cliOptions, configPath string) error {
	return loadProjectConfig(options, configPath)
}

func loadProjectConfig(options *cliOptions, configPath string) error {
	config, loaded, err := project.LoadOptionalConfig(configPath)
	if err != nil {
		return err
	}
	if !loaded {
		return nil
	}
	config.Addons = append(config.Addons, options.Config.Addons...)
	options.Config = config
	return nil
}

func discoverBuildFiles(config gowdk.Config, outputDir string, moduleNames []string) ([]string, error) {
	return discoverConfiguredFiles(config, outputDir, moduleNames)
}

func discoverProjectFiles(config gowdk.Config, moduleNames []string) ([]string, error) {
	return discoverConfiguredFiles(config, config.Build.Output, moduleNames)
}

func discoverConfiguredFiles(config gowdk.Config, outputDir string, moduleNames []string) ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
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

func loadCommandInputs(args []string, command string, allowJSON bool) (cliOptions, []string, error) {
	options, configPath, moduleNames, paths, err := parseProjectOptions(args, command, allowJSON)
	if err != nil {
		return options, nil, err
	}
	if err := loadProjectConfig(&options, configPath); err != nil {
		return options, nil, err
	}
	if len(paths) == 0 {
		discovered, err := discoverProjectFiles(options.Config, moduleNames)
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

func parseServeOptions(args []string) (string, string, error) {
	addr := "127.0.0.1:8080"
	var dir string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--dir":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("usage: gowdk serve --dir <dir> [--addr <addr>]")
			}
			dir = args[i]
		case strings.HasPrefix(arg, "--dir="):
			dir = strings.TrimPrefix(arg, "--dir=")
		case arg == "--addr":
			i++
			if i >= len(args) {
				return "", "", fmt.Errorf("usage: gowdk serve --dir <dir> [--addr <addr>]")
			}
			addr = args[i]
		case strings.HasPrefix(arg, "--addr="):
			addr = strings.TrimPrefix(arg, "--addr=")
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("unknown serve flag %q", arg)
		default:
			return "", "", fmt.Errorf("unexpected serve argument %q", arg)
		}
	}
	if strings.TrimSpace(dir) == "" {
		return "", "", fmt.Errorf("usage: gowdk serve --dir <dir> [--addr <addr>]")
	}
	if strings.TrimSpace(addr) == "" {
		return "", "", fmt.Errorf("serve address is required")
	}
	return dir, addr, nil
}

func staticFileHandler(root string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		filePath, ok := staticFilePath(root, request.URL.Path)
		if !ok {
			http.NotFound(w, request)
			return
		}
		http.ServeFile(w, request, filePath)
	})
}

func staticFilePath(root, requestPath string) (string, bool) {
	clean := path.Clean("/" + requestPath)
	candidates := []string{clean}
	if strings.HasSuffix(requestPath, "/") {
		candidates = []string{path.Join(clean, "index.html")}
	} else if path.Ext(clean) == "" {
		candidates = append(candidates, path.Join(clean, "index.html"))
	}

	for _, candidate := range candidates {
		filePath, ok := staticCandidatePath(root, candidate)
		if ok {
			return filePath, true
		}
	}
	return "", false
}

func staticCandidatePath(root, candidate string) (string, bool) {
	rel := strings.TrimPrefix(path.Clean("/"+candidate), "/")
	filePath := filepath.Join(root, filepath.FromSlash(rel))
	relative, err := filepath.Rel(root, filePath)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", false
	}
	info, err := os.Stat(filePath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		indexPath := filepath.Join(filePath, "index.html")
		if indexInfo, err := os.Stat(indexPath); err == nil && !indexInfo.IsDir() {
			return indexPath, true
		}
		return "", false
	}
	return filePath, true
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

func languageServer(args []string) error {
	options, paths := parseOptions(args)
	if len(paths) > 0 {
		return fmt.Errorf("usage: gowdk lsp [--ssr]")
	}
	return lsp.NewServer(options.Config).Serve(os.Stdin, os.Stdout)
}

func parseBuildOptions(args []string) (cliOptions, string, string, string, string, []string, []string, error) {
	var options cliOptions
	var outputDir string
	var appDir string
	var binaryPath string
	var configPath string
	var moduleNames []string
	var paths []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--ssr":
			options.Config.Addons = append(options.Config.Addons, ssr.Addon())
		case arg == "--out":
			i++
			if i >= len(args) {
				return options, "", "", "", "", nil, nil, fmt.Errorf("usage: gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]")
			}
			outputDir = args[i]
		case len(arg) > len("--out=") && arg[:len("--out=")] == "--out=":
			outputDir = arg[len("--out="):]
		case arg == "--app":
			i++
			if i >= len(args) {
				return options, "", "", "", "", nil, nil, fmt.Errorf("usage: gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]")
			}
			appDir = args[i]
			if strings.TrimSpace(appDir) == "" {
				return options, "", "", "", "", nil, nil, fmt.Errorf("generated app directory is required")
			}
		case len(arg) > len("--app=") && arg[:len("--app=")] == "--app=":
			appDir = arg[len("--app="):]
			if strings.TrimSpace(appDir) == "" {
				return options, "", "", "", "", nil, nil, fmt.Errorf("generated app directory is required")
			}
		case arg == "--bin":
			i++
			if i >= len(args) {
				return options, "", "", "", "", nil, nil, fmt.Errorf("usage: gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]")
			}
			binaryPath = args[i]
			if strings.TrimSpace(binaryPath) == "" {
				return options, "", "", "", "", nil, nil, fmt.Errorf("binary output path is required")
			}
		case len(arg) > len("--bin=") && arg[:len("--bin=")] == "--bin=":
			binaryPath = arg[len("--bin="):]
			if strings.TrimSpace(binaryPath) == "" {
				return options, "", "", "", "", nil, nil, fmt.Errorf("binary output path is required")
			}
		case arg == "--config":
			i++
			if i >= len(args) {
				return options, "", "", "", "", nil, nil, fmt.Errorf("usage: gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]")
			}
			configPath = args[i]
		case len(arg) > len("--config=") && arg[:len("--config=")] == "--config=":
			configPath = arg[len("--config="):]
		case arg == "--module":
			i++
			if i >= len(args) {
				return options, "", "", "", "", nil, nil, fmt.Errorf("usage: gowdk build [--config <file>] [--ssr] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [files...]")
			}
			moduleNames = appendModuleNames(moduleNames, args[i])
		case len(arg) > len("--module=") && arg[:len("--module=")] == "--module=":
			moduleNames = appendModuleNames(moduleNames, arg[len("--module="):])
		case len(arg) > 0 && arg[0] == '-':
			return options, "", "", "", "", nil, nil, fmt.Errorf("unknown build flag %q", arg)
		default:
			paths = append(paths, arg)
		}
	}

	return options, outputDir, appDir, binaryPath, configPath, moduleNames, paths, nil
}

func appendModuleNames(moduleNames []string, value string) []string {
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		moduleNames = append(moduleNames, name)
	}
	return moduleNames
}

type cliOptions struct {
	Config gowdk.Config
	JSON   bool
}

type routeBindingsReport struct {
	Version int                `json:"version"`
	Routes  []routeBindingJSON `json:"routes"`
}

type routeBindingJSON struct {
	Kind    codegen.RouteKind `json:"kind"`
	Method  string            `json:"method"`
	Route   string            `json:"route"`
	PageID  string            `json:"pageId"`
	Handler string            `json:"handler"`
}

func routeBindingsJSON(bindings []codegen.RouteBinding) routeBindingsReport {
	routes := make([]routeBindingJSON, 0, len(bindings))
	for _, binding := range bindings {
		routes = append(routes, routeBindingJSON{
			Kind:    binding.Kind,
			Method:  binding.Method,
			Route:   binding.Route,
			PageID:  binding.PageID,
			Handler: binding.Handler,
		})
	}
	return routeBindingsReport{
		Version: 1,
		Routes:  routes,
	}
}

func parseOptions(args []string) (cliOptions, []string) {
	var options cliOptions
	var paths []string
	for _, arg := range args {
		switch arg {
		case "--ssr":
			options.Config.Addons = append(options.Config.Addons, ssr.Addon())
		case "--json":
			options.JSON = true
		default:
			paths = append(paths, arg)
			continue
		}
	}
	return options, paths
}

func parseProjectOptions(args []string, command string, allowJSON bool) (cliOptions, string, []string, []string, error) {
	var options cliOptions
	var configPath string
	var moduleNames []string
	var paths []string
	usage := projectCommandUsage(command, allowJSON)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--ssr":
			options.Config.Addons = append(options.Config.Addons, ssr.Addon())
		case arg == "--json" && allowJSON:
			options.JSON = true
		case arg == "--json":
			return options, "", nil, nil, fmt.Errorf("unknown %s flag %q", command, arg)
		case arg == "--config":
			i++
			if i >= len(args) {
				return options, "", nil, nil, errors.New(usage)
			}
			configPath = args[i]
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
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
	return options, configPath, moduleNames, paths, nil
}

func projectCommandUsage(command string, allowJSON bool) string {
	if allowJSON {
		return fmt.Sprintf("usage: gowdk %s [--config <file>] [--module <name>] [--json] [--ssr] [files...]", command)
	}
	return fmt.Sprintf("usage: gowdk %s [--config <file>] [--module <name>] [--ssr] [files...]", command)
}

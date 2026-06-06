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
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/appgen"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/lsp"
	"github.com/cssbruno/gowdk/internal/manifest"
)

const (
	version    = "0.1.5"
	buildUsage = "usage: gowdk build [--config <file>] [--debug] [--ssr] [--allow-missing-backend] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...]"
)

var (
	defaultSourceIncludes = []string{"**/*.gwdk"}
	defaultSourceExcludes = []string{".git/**", "vendor/**", "node_modules/**", "**/testdata/**"}
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
	case "dev":
		return dev(args[1:])
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
	fmt.Println("compile-first Go web kit: build-time output, backend actions, SSR optional")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  version                  print CLI version")
	fmt.Println("  init [--force] [dir]     scaffold a starter GOWDK project")
	fmt.Println("  tokens <file.gwdk>       print language tokens")
	fmt.Println("  fmt [--write] <files>    format .gwdk files")
	fmt.Println("  check [--config <file>] [--module <name>] [--json] [--ssr] [files...] parse and validate .gwdk files")
	fmt.Println("  manifest [--config <file>] [--module <name>] [--ssr] [files...] print validated manifest JSON")
	fmt.Println("  sitemap [--config <file>] [--module <name>] [--ssr] [files...] print editor site-map JSON")
	fmt.Println("  routes [--config <file>] [--module <name>] [--ssr] [files...] print route and endpoint metadata JSON")
	fmt.Println("  build [--config <file>] [--debug] [--ssr] [--allow-missing-backend] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...] compile .gwdk files into build output")
	fmt.Println("  dev [--addr <addr>] [--interval <duration>] [build flags...] build, serve, rebuild, and live reload")
	fmt.Println("  serve --dir <dir> [--addr <addr>] serve generated build output locally")
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

import "github.com/cssbruno/gowdk"

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
			Path: ".gitignore",
			Body: `gowdk_cache/
`,
		},
		{
			Path: "src/pages/home.page.gwdk",
			Body: `package app

@page home
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
			Body: `package app

@component Hero

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

	metadata, err := compiler.BuildRouteMetadata(options.Config, app)
	if err != nil {
		return err
	}
	printRouteInfos(metadata.Info)
	payload, err := json.MarshalIndent(routeMetadataJSON(metadata), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(payload))
	return nil
}

func printRouteInfos(infos []compiler.RouteInfo) {
	for _, info := range infos {
		fmt.Fprintf(os.Stderr, "info: %s: %s\n", info.Code, info.Message)
	}
}

func build(args []string) error {
	options, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, configPath, targetNames, moduleNames, paths, err := parseBuildOptions(args)
	if err != nil {
		return err
	}
	if err := loadBuildConfig(&options, configPath); err != nil {
		return err
	}
	if len(targetNames) > 0 && hasAdHocBuildArgs(outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, moduleNames, paths) {
		return fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --wasm, --backend-app, --backend-bin, or explicit files")
	}
	if shouldBuildConfiguredTargets(options.Config, targetNames, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, moduleNames, paths) {
		return buildConfiguredTargets(options, targetNames)
	}
	return buildOnce(options, buildRequest{
		OutputDir:         outputDir,
		AppDir:            appDir,
		BinaryPath:        binaryPath,
		WASMPath:          wasmPath,
		BackendAppDir:     backendAppDir,
		BackendBinaryPath: backendBinaryPath,
		Modules:           moduleNames,
		Paths:             paths,
	})
}

type buildRequest struct {
	OutputDir         string
	AppDir            string
	BinaryPath        string
	WASMPath          string
	BackendAppDir     string
	BackendBinaryPath string
	Modules           []string
	Paths             []string
}

func buildOnce(options cliOptions, request buildRequest) error {
	outputDir := request.OutputDir
	if strings.TrimSpace(request.BinaryPath) != "" && strings.TrimSpace(request.AppDir) == "" {
		return fmt.Errorf("gowdk build --bin requires --app <dir>")
	}
	if strings.TrimSpace(request.WASMPath) != "" && strings.TrimSpace(request.AppDir) == "" {
		return fmt.Errorf("gowdk build --wasm requires --app <dir>")
	}
	if strings.TrimSpace(request.BackendBinaryPath) != "" && strings.TrimSpace(request.BackendAppDir) == "" {
		return fmt.Errorf("gowdk build --backend-bin requires --backend-app <dir>")
	}
	if outputDir == "" {
		outputDir = options.Config.Build.Output
	}
	if outputDir == "" {
		return fmt.Errorf(buildUsage)
	}
	options.Config.Build.Output = outputDir
	paths := append([]string(nil), request.Paths...)
	if len(paths) == 0 {
		discovered, err := discoverBuildFiles(options.Config, outputDir, request.Modules)
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
	app = compiler.BindBackendHandlers(app)

	result, err := buildgen.Build(options.Config, app, outputDir)
	if err != nil {
		printBuildgenBuildErrorReport(err, options.Debug)
		return err
	}
	for _, artifact := range result.Artifacts {
		fmt.Println(artifact.Path)
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
	appDir := request.AppDir
	binaryPath := request.BinaryPath
	wasmPath := request.WASMPath
	backendAppDir := request.BackendAppDir
	backendBinaryPath := request.BackendBinaryPath
	if strings.TrimSpace(appDir) != "" {
		app, err := appgen.GenerateWithOptions(outputDir, appDir, appgen.Options{
			AutoRoutes:   true,
			Config:       options.Config,
			Manifest:     &app,
			ProxyBackend: strings.TrimSpace(backendAppDir) != "",
		})
		if err != nil {
			return err
		}
		fmt.Println(app.ModulePath)
		fmt.Println(app.PackagePath)
		fmt.Println(app.MainPath)
		if strings.TrimSpace(binaryPath) != "" {
			built, err := appgen.BuildBinary(app.AppDir, binaryPath)
			if err != nil {
				return err
			}
			fmt.Println(built)
		}
		if strings.TrimSpace(wasmPath) != "" {
			built, err := appgen.BuildWASM(app.AppDir, wasmPath)
			if err != nil {
				return err
			}
			fmt.Println(built)
		}
	}
	if strings.TrimSpace(backendAppDir) != "" {
		app, err := appgen.GenerateBackendWithOptions(backendAppDir, appgen.Options{
			AutoRoutes: true,
			Config:     options.Config,
			Manifest:   &app,
		})
		if err != nil {
			return err
		}
		fmt.Println(app.ModulePath)
		fmt.Println(app.PackagePath)
		fmt.Println(app.MainPath)
		if strings.TrimSpace(backendBinaryPath) != "" {
			built, err := appgen.BuildBinary(app.AppDir, backendBinaryPath)
			if err != nil {
				return err
			}
			fmt.Println(built)
		}
	}
	return nil
}

func shouldBuildConfiguredTargets(config gowdk.Config, targetNames []string, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath string, moduleNames, paths []string) bool {
	if len(targetNames) > 0 {
		return true
	}
	if len(config.Build.Targets) == 0 {
		return false
	}
	return strings.TrimSpace(outputDir) == "" &&
		strings.TrimSpace(appDir) == "" &&
		strings.TrimSpace(binaryPath) == "" &&
		strings.TrimSpace(wasmPath) == "" &&
		strings.TrimSpace(backendAppDir) == "" &&
		strings.TrimSpace(backendBinaryPath) == "" &&
		len(moduleNames) == 0 &&
		len(paths) == 0
}

func hasAdHocBuildArgs(outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath string, moduleNames, paths []string) bool {
	return strings.TrimSpace(outputDir) != "" ||
		strings.TrimSpace(appDir) != "" ||
		strings.TrimSpace(binaryPath) != "" ||
		strings.TrimSpace(wasmPath) != "" ||
		strings.TrimSpace(backendAppDir) != "" ||
		strings.TrimSpace(backendBinaryPath) != "" ||
		len(moduleNames) > 0 ||
		len(paths) > 0
}

func buildConfiguredTargets(options cliOptions, targetNames []string) error {
	targets, err := selectBuildTargets(options.Config.Build.Targets, targetNames)
	if err != nil {
		return err
	}
	for _, target := range targets {
		targetOptions := options
		targetOptions.Config.Build.Output = target.Output
		if err := buildOnce(targetOptions, buildRequest{
			OutputDir:         target.Output,
			AppDir:            target.App,
			BinaryPath:        target.Binary,
			WASMPath:          target.WASM,
			BackendAppDir:     target.BackendApp,
			BackendBinaryPath: target.BackendBinary,
			Modules:           target.Modules,
		}); err != nil {
			return fmt.Errorf("build target %q: %w", target.Name, err)
		}
	}
	return nil
}

func selectBuildTargets(targets []gowdk.BuildTargetConfig, targetNames []string) ([]gowdk.BuildTargetConfig, error) {
	byName := map[string]gowdk.BuildTargetConfig{}
	var normalized []gowdk.BuildTargetConfig
	for _, target := range targets {
		name := strings.TrimSpace(target.Name)
		if name == "" {
			return nil, fmt.Errorf("build target is missing name")
		}
		if _, exists := byName[name]; exists {
			return nil, fmt.Errorf("build target %q is configured more than once", name)
		}
		target.Name = name
		target.Modules = cleanNames(target.Modules)
		if strings.TrimSpace(target.Output) == "" {
			return nil, fmt.Errorf("build target %q is missing output", name)
		}
		if strings.TrimSpace(target.Binary) != "" && strings.TrimSpace(target.App) == "" {
			return nil, fmt.Errorf("build target %q binary requires app", name)
		}
		if strings.TrimSpace(target.WASM) != "" && strings.TrimSpace(target.App) == "" {
			return nil, fmt.Errorf("build target %q wasm requires app", name)
		}
		if strings.TrimSpace(target.BackendBinary) != "" && strings.TrimSpace(target.BackendApp) == "" {
			return nil, fmt.Errorf("build target %q backend binary requires backend app", name)
		}
		target.Output = strings.TrimSpace(target.Output)
		target.App = strings.TrimSpace(target.App)
		target.Binary = strings.TrimSpace(target.Binary)
		target.WASM = strings.TrimSpace(target.WASM)
		target.BackendApp = strings.TrimSpace(target.BackendApp)
		target.BackendBinary = strings.TrimSpace(target.BackendBinary)
		byName[name] = target
		normalized = append(normalized, target)
	}

	if len(targetNames) == 0 {
		return normalized, nil
	}
	var selected []gowdk.BuildTargetConfig
	for _, name := range cleanNames(targetNames) {
		target, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("build target %q is not configured", name)
		}
		selected = append(selected, target)
	}
	return selected, nil
}

func printBuildgenBuildErrorReport(err error, debug bool) {
	if !debug {
		return
	}
	var buildErr *buildgen.BuildError
	if errors.As(err, &buildErr) {
		printBuildgenBuildReport(buildErr.Report, true)
	}
}

func printBuildgenBuildReport(report buildgen.BuildReport, debug bool) {
	if !debug || report.Version == 0 {
		return
	}
	mode := strings.TrimSpace(report.Mode)
	if mode == "" {
		mode = "build"
	}
	fmt.Fprintf(os.Stderr, "gowdk build report (%s):\n", mode)
	for _, event := range report.Events {
		stage := event.Stage
		if event.Kind != "" {
			stage += "/" + event.Kind
		}
		details := buildgenBuildEventDetails(event)
		if details != "" {
			details = " (" + details + ")"
		}
		fmt.Fprintf(os.Stderr, "  [%s] %s: %s%s\n", event.Level, stage, event.Message, details)
	}
}

func buildgenBuildEventDetails(event buildgen.BuildEvent) string {
	var details []string
	if event.PageID != "" {
		details = append(details, "page="+event.PageID)
	}
	if event.Route != "" {
		details = append(details, "route="+event.Route)
	}
	if event.Path != "" {
		details = append(details, "path="+event.Path)
	}
	if len(event.Data) > 0 {
		keys := make([]string, 0, len(event.Data))
		for key := range event.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			details = append(details, key+"="+event.Data[key])
		}
	}
	return strings.Join(details, ", ")
}

func languageServer(args []string) error {
	options, paths := parseOptions(args)
	if len(paths) > 0 {
		return fmt.Errorf("usage: gowdk lsp [--ssr]")
	}
	return lsp.NewServer(options.Config).Serve(os.Stdin, os.Stdout)
}

func parseBuildOptions(args []string) (cliOptions, string, string, string, string, string, string, string, []string, []string, []string, error) {
	var options cliOptions
	var outputDir string
	var appDir string
	var binaryPath string
	var wasmPath string
	var backendAppDir string
	var backendBinaryPath string
	var configPath string
	var targetNames []string
	var moduleNames []string
	var paths []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--ssr":
			options.Config.Addons = append(options.Config.Addons, ssr.Addon())
		case arg == "--debug":
			options.Debug = true
		case arg == "--allow-missing-backend":
			options.AllowMissingBackend = true
			options.Config.Build.AllowMissingBackend = true
		case arg == "--out":
			i++
			if i >= len(args) {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf(buildUsage)
			}
			outputDir = args[i]
		case len(arg) > len("--out=") && arg[:len("--out=")] == "--out=":
			outputDir = arg[len("--out="):]
		case arg == "--app":
			i++
			if i >= len(args) {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf(buildUsage)
			}
			appDir = args[i]
			if strings.TrimSpace(appDir) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("generated app directory is required")
			}
		case len(arg) > len("--app=") && arg[:len("--app=")] == "--app=":
			appDir = arg[len("--app="):]
			if strings.TrimSpace(appDir) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("generated app directory is required")
			}
		case arg == "--bin":
			i++
			if i >= len(args) {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf(buildUsage)
			}
			binaryPath = args[i]
			if strings.TrimSpace(binaryPath) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("binary output path is required")
			}
		case len(arg) > len("--bin=") && arg[:len("--bin=")] == "--bin=":
			binaryPath = arg[len("--bin="):]
			if strings.TrimSpace(binaryPath) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("binary output path is required")
			}
		case arg == "--wasm":
			i++
			if i >= len(args) {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf(buildUsage)
			}
			wasmPath = args[i]
			if strings.TrimSpace(wasmPath) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("wasm output path is required")
			}
		case len(arg) > len("--wasm=") && arg[:len("--wasm=")] == "--wasm=":
			wasmPath = arg[len("--wasm="):]
			if strings.TrimSpace(wasmPath) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("wasm output path is required")
			}
		case arg == "--backend-app":
			i++
			if i >= len(args) {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf(buildUsage)
			}
			backendAppDir = args[i]
			if strings.TrimSpace(backendAppDir) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("generated backend app directory is required")
			}
		case len(arg) > len("--backend-app=") && arg[:len("--backend-app=")] == "--backend-app=":
			backendAppDir = arg[len("--backend-app="):]
			if strings.TrimSpace(backendAppDir) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("generated backend app directory is required")
			}
		case arg == "--backend-bin":
			i++
			if i >= len(args) {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf(buildUsage)
			}
			backendBinaryPath = args[i]
			if strings.TrimSpace(backendBinaryPath) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("backend binary output path is required")
			}
		case len(arg) > len("--backend-bin=") && arg[:len("--backend-bin=")] == "--backend-bin=":
			backendBinaryPath = arg[len("--backend-bin="):]
			if strings.TrimSpace(backendBinaryPath) == "" {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("backend binary output path is required")
			}
		case arg == "--config":
			i++
			if i >= len(args) {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf(buildUsage)
			}
			configPath = args[i]
		case len(arg) > len("--config=") && arg[:len("--config=")] == "--config=":
			configPath = arg[len("--config="):]
		case arg == "--target":
			i++
			if i >= len(args) {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf(buildUsage)
			}
			targetNames = appendNames(targetNames, args[i])
		case len(arg) > len("--target=") && arg[:len("--target=")] == "--target=":
			targetNames = appendNames(targetNames, arg[len("--target="):])
		case arg == "--module":
			i++
			if i >= len(args) {
				return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf(buildUsage)
			}
			moduleNames = appendNames(moduleNames, args[i])
		case len(arg) > len("--module=") && arg[:len("--module=")] == "--module=":
			moduleNames = appendNames(moduleNames, arg[len("--module="):])
		case len(arg) > 0 && arg[0] == '-':
			return options, "", "", "", "", "", "", "", nil, nil, nil, fmt.Errorf("unknown build flag %q", arg)
		default:
			paths = append(paths, arg)
		}
	}

	return options, outputDir, appDir, binaryPath, wasmPath, backendAppDir, backendBinaryPath, configPath, targetNames, moduleNames, paths, nil
}

func appendModuleNames(moduleNames []string, value string) []string {
	return appendNames(moduleNames, value)
}

func appendNames(names []string, value string) []string {
	for _, name := range strings.Split(value, ",") {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	return names
}

func cleanNames(names []string) []string {
	var cleaned []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		cleaned = append(cleaned, name)
	}
	return cleaned
}

type cliOptions struct {
	Config              gowdk.Config
	JSON                bool
	Debug               bool
	AllowMissingBackend bool
}

type routeMetadataReport struct {
	Version   int                   `json:"version"`
	Routes    []routeBindingJSON    `json:"routes"`
	Endpoints []endpointBindingJSON `json:"endpoints,omitempty"`
	Info      []routeInfoJSON       `json:"info,omitempty"`
}

type routeBindingJSON struct {
	Kind    compiler.RouteKind `json:"kind"`
	Method  string             `json:"method"`
	Route   string             `json:"route"`
	PageID  string             `json:"pageId"`
	Handler string             `json:"handler"`
}

type endpointBindingJSON struct {
	Kind           compiler.EndpointKind `json:"kind"`
	EndpointSource string                `json:"endpointSource,omitempty"`
	Source         string                `json:"source,omitempty"`
	SourceSpan     *sourceSpanJSON       `json:"sourceSpan,omitempty"`
	Package        string                `json:"package,omitempty"`
	PackagePath    string                `json:"packagePath,omitempty"`
	PackageName    string                `json:"packageName,omitempty"`
	Symbol         string                `json:"symbol,omitempty"`
	Method         string                `json:"method"`
	Route          string                `json:"route"`
	PageID         string                `json:"pageId"`
	Handler        string                `json:"handler"`
	BindingStatus  string                `json:"bindingStatus,omitempty"`
	Signature      string                `json:"signature,omitempty"`
	InputType      string                `json:"inputType,omitempty"`
	BackendBinding *backendBindingJSON   `json:"backendBinding,omitempty"`
}

type sourceSpanJSON struct {
	Start sourcePositionJSON `json:"start"`
	End   sourcePositionJSON `json:"end"`
}

type sourcePositionJSON struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type backendBindingJSON struct {
	Status       string `json:"status"`
	PackageName  string `json:"packageName,omitempty"`
	ImportPath   string `json:"importPath,omitempty"`
	FunctionName string `json:"functionName,omitempty"`
	Signature    string `json:"signature,omitempty"`
	InputType    string `json:"inputType,omitempty"`
	Message      string `json:"message,omitempty"`
}

type routeInfoJSON struct {
	Code    string `json:"code"`
	PageID  string `json:"pageId"`
	Route   string `json:"route"`
	Message string `json:"message"`
}

func routeMetadataJSON(metadata compiler.RouteMetadata) routeMetadataReport {
	routes := make([]routeBindingJSON, 0, len(metadata.Routes))
	for _, binding := range metadata.Routes {
		routes = append(routes, routeBindingJSON{
			Kind:    binding.Kind,
			Method:  binding.Method,
			Route:   binding.Route,
			PageID:  binding.PageID,
			Handler: binding.Handler,
		})
	}
	endpoints := make([]endpointBindingJSON, 0, len(metadata.Endpoints))
	for _, binding := range metadata.Endpoints {
		item := endpointBindingJSON{
			Kind:           binding.Kind,
			EndpointSource: binding.EndpointSource,
			Source:         binding.Source,
			SourceSpan:     endpointSourceSpanJSON(binding.SourceSpan),
			Package:        binding.Package,
			PackagePath:    binding.PackagePath,
			PackageName:    binding.PackageName,
			Symbol:         binding.Symbol,
			Method:         binding.Method,
			Route:          binding.Route,
			PageID:         binding.PageID,
			Handler:        binding.Handler,
			BindingStatus:  string(binding.BindingStatus),
			Signature:      string(binding.BindingSignature),
			InputType:      binding.BindingInputType,
		}
		if binding.BindingStatus != "" {
			item.BackendBinding = &backendBindingJSON{
				Status:       string(binding.BindingStatus),
				PackageName:  binding.BindingPackage,
				ImportPath:   binding.BindingImportPath,
				FunctionName: binding.BindingFunction,
				Signature:    string(binding.BindingSignature),
				InputType:    binding.BindingInputType,
				Message:      binding.BindingMessage,
			}
		}
		endpoints = append(endpoints, item)
	}
	info := make([]routeInfoJSON, 0, len(metadata.Info))
	for _, item := range metadata.Info {
		info = append(info, routeInfoJSON{
			Code:    item.Code,
			PageID:  item.PageID,
			Route:   item.Route,
			Message: item.Message,
		})
	}
	return routeMetadataReport{
		Version:   1,
		Routes:    routes,
		Endpoints: endpoints,
		Info:      info,
	}
}

func endpointSourceSpanJSON(span manifest.SourceSpan) *sourceSpanJSON {
	if span.Start.Line <= 0 || span.Start.Column <= 0 || span.End.Line <= 0 || span.End.Column <= 0 {
		return nil
	}
	return &sourceSpanJSON{
		Start: sourcePositionJSON{Line: span.Start.Line, Column: span.Start.Column},
		End:   sourcePositionJSON{Line: span.End.Line, Column: span.End.Column},
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

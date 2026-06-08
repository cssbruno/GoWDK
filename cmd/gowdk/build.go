package main

import (
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
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/lang"
)

const buildUsage = "usage: gowdk build [--config <file>] [--debug] [--ssr] [--allow-missing-backend] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...]"

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
	app, err := compiler.DiscoverGoEndpointComments(app)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fmt.Errorf("build failed")
	}
	if err := compiler.ValidateManifest(options.Config, app); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fmt.Errorf("build failed")
	}
	app = compiler.BindBackendHandlers(app)
	ir := gwdkanalysis.BuildIR(options.Config, app)
	if err := linkIRContractReferences(&ir, "."); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fmt.Errorf("build failed")
	}
	if err := compiler.ValidateContractReferences(ir.ContractRefs); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fmt.Errorf("build failed")
	}

	result, err := buildgen.BuildFromIR(options.Config, ir, outputDir)
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
			IR:           &ir,
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
			IR:         &ir,
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
			target.Output = defaultBuildTargetOutput(name)
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

func defaultBuildTargetOutput(name string) string {
	return filepath.ToSlash(filepath.Join(".gowdk", "output", name))
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

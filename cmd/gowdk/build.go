package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/appgen"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/contractscan"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/source"
)

const buildUsage = "usage: gowdk build [--config <file>] [--debug] [--timings[=<file>]] [--ssr] [--allow-missing-backend] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...]"

func build(args []string) error {
	started := time.Now()
	plan, err := loadBuildOptions(args)
	if err != nil {
		return err
	}
	timings := newBuildTimingRecorder(plan.Timings)
	timings.addDuration("config_load", time.Since(started))
	if plan.shouldBuildConfiguredTargets() {
		return buildConfiguredTargets(plan, timings)
	}
	return buildOnce(plan.Options, plan.request(), timings)
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
	TimingsPath       string
}

type buildOptions struct {
	Options           cliOptions
	OutputDir         string
	AppDir            string
	BinaryPath        string
	WASMPath          string
	BackendAppDir     string
	BackendBinaryPath string
	ConfigPath        string
	TargetNames       []string
	ModuleNames       []string
	Paths             []string
	Timings           bool
	TimingsPath       string
}

func loadBuildOptions(args []string) (buildOptions, error) {
	plan, err := parseBuildOptions(args)
	if err != nil {
		return buildOptions{}, err
	}
	if err := loadBuildConfig(&plan.Options, plan.ConfigPath); err != nil {
		return buildOptions{}, err
	}
	if len(plan.TargetNames) > 0 && plan.hasAdHocArgs() {
		return buildOptions{}, fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --wasm, --backend-app, --backend-bin, or explicit files")
	}
	return plan, nil
}

func (plan buildOptions) request() buildRequest {
	return buildRequest{
		OutputDir:         plan.OutputDir,
		AppDir:            plan.AppDir,
		BinaryPath:        plan.BinaryPath,
		WASMPath:          plan.WASMPath,
		BackendAppDir:     plan.BackendAppDir,
		BackendBinaryPath: plan.BackendBinaryPath,
		Modules:           plan.ModuleNames,
		Paths:             plan.Paths,
		TimingsPath:       plan.TimingsPath,
	}
}

func (plan buildOptions) hasAdHocArgs() bool {
	return hasAdHocBuildArgs(plan.OutputDir, plan.AppDir, plan.BinaryPath, plan.WASMPath, plan.BackendAppDir, plan.BackendBinaryPath, plan.ModuleNames, plan.Paths)
}

func (plan buildOptions) shouldBuildConfiguredTargets() bool {
	return shouldBuildConfiguredTargets(plan.Options.Config, plan.TargetNames, plan.OutputDir, plan.AppDir, plan.BinaryPath, plan.WASMPath, plan.BackendAppDir, plan.BackendBinaryPath, plan.ModuleNames, plan.Paths)
}

func buildOnce(options cliOptions, request buildRequest, timings *buildTimingRecorder) error {
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
		var discovered []string
		if err := timings.measure("source_discovery", func() error {
			var discoverErr error
			discovered, discoverErr = discoverBuildFiles(options.Config, outputDir, request.Modules, options.ProjectRoot)
			return discoverErr
		}); err != nil {
			return err
		}
		if len(discovered) == 0 {
			return fmt.Errorf("no .gwdk files found")
		}
		paths = discovered
	}
	timings.counter("source_files", len(paths))

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
		return fmt.Errorf("build failed")
	}
	var ir gwdkir.Program
	timings.measure("ir_assembly", func() error {
		ir = gwdkanalysis.BuildProgram(options.Config, app)
		return nil
	})
	timings.counter("pages", len(ir.Pages))
	timings.counter("components", len(ir.Components))
	timings.counter("layouts", len(ir.Layouts))
	timings.counter("endpoints", len(ir.Endpoints))
	var bindings []source.BackendBinding
	if err := timings.measure("go_binding", func() error {
		if err := compiler.DiscoverGoEndpoints(options.Config, &ir); err != nil {
			return err
		}
		bindings = compiler.BindBackendHandlers(&ir)
		return nil
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fmt.Errorf("build failed")
	}
	var report compiler.ValidationErrors
	timings.measure("ir_validation", func() error {
		report = compiler.ValidateProgramReport(options.Config, ir)
		report = append(report, compiler.BackendBindingDiagnostics(bindings)...)
		return nil
	})
	for _, diagnostic := range report {
		prefix := ""
		if diagnostic.Severity == compiler.SeverityWarning {
			prefix = "warning: "
		}
		fmt.Fprintln(os.Stderr, prefix+diagnostic.Error())
	}
	if report.HasErrors() {
		return fmt.Errorf("build failed")
	}
	var contractReport contractscan.Report
	if err := timings.measure("contract_validation", func() error {
		scanned, err := scanContractReport(options.ProjectRoot)
		if err != nil {
			return err
		}
		contractReport = scanned
		linkIRContractReferencesFromReport(&ir, contractReport)
		return compiler.ValidateContractReferences(ir.ContractRefs)
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return fmt.Errorf("build failed")
	}

	var result buildgen.Result
	if err := timings.measure("output_plan_writes", func() error {
		var buildErr error
		result, buildErr = buildgen.BuildFromValidatedIR(options.Config, ir, outputDir)
		return buildErr
	}); err != nil {
		printBuildgenBuildErrorReport(err, options.Debug)
		return err
	}
	timings.counter("artifacts", len(result.Artifacts))
	timings.counter("css_artifacts", len(result.CSSArtifacts))
	timings.counter("asset_artifacts", len(result.AssetArtifacts))
	timings.counter("files_written", result.WriteStats.FilesWritten)
	timings.counter("identical_writes_skipped", result.WriteStats.IdenticalWritesSkipped)
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
	if result.OpenAPIPath != "" {
		fmt.Println(result.OpenAPIPath)
	}
	if result.SecurityManifestPath != "" {
		fmt.Println(result.SecurityManifestPath)
	}
	var asyncAPIPath string
	if err := timings.measure("asyncapi_report", func() error {
		var writeErr error
		asyncAPIPath, writeErr = contractscan.WriteAsyncAPI(outputDir, contractReport, contractscan.AsyncAPIOptions{})
		return writeErr
	}); err != nil {
		return err
	}
	if asyncAPIPath != "" {
		fmt.Println(asyncAPIPath)
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
		var app appgen.Result
		if err := timings.measure("app_generation", func() error {
			var appErr error
			app, appErr = appgen.GenerateWithOptions(outputDir, appDir, appgen.Options{
				AutoRoutes:   true,
				Config:       options.Config,
				IR:           &ir,
				ProxyBackend: strings.TrimSpace(backendAppDir) != "",
			})
			return appErr
		}); err != nil {
			return err
		}
		fmt.Println(app.ModulePath)
		fmt.Println(app.PackagePath)
		fmt.Println(app.MainPath)
		if strings.TrimSpace(binaryPath) != "" {
			var built string
			if err := timings.measure("binary_build", func() error {
				var buildErr error
				built, buildErr = appgen.BuildBinary(app.AppDir, binaryPath)
				return buildErr
			}); err != nil {
				return err
			}
			fmt.Println(built)
		}
		if strings.TrimSpace(wasmPath) != "" {
			var built string
			if err := timings.measure("wasm_build", func() error {
				var buildErr error
				built, buildErr = appgen.BuildWASM(app.AppDir, wasmPath)
				return buildErr
			}); err != nil {
				return err
			}
			fmt.Println(built)
		}
	}
	if strings.TrimSpace(backendAppDir) != "" {
		var app appgen.Result
		if err := timings.measure("backend_app_generation", func() error {
			var appErr error
			app, appErr = appgen.GenerateBackendWithOptions(backendAppDir, appgen.Options{
				AutoRoutes: true,
				Config:     options.Config,
				IR:         &ir,
			})
			return appErr
		}); err != nil {
			return err
		}
		fmt.Println(app.ModulePath)
		fmt.Println(app.PackagePath)
		fmt.Println(app.MainPath)
		if strings.TrimSpace(backendBinaryPath) != "" {
			var built string
			if err := timings.measure("backend_binary_build", func() error {
				var buildErr error
				built, buildErr = appgen.BuildBinary(app.AppDir, backendBinaryPath)
				return buildErr
			}); err != nil {
				return err
			}
			fmt.Println(built)
		}
	}
	if _, err := timings.write(outputDir, request.TimingsPath); err != nil {
		return err
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

func buildConfiguredTargets(plan buildOptions, timings *buildTimingRecorder) error {
	targets, err := selectBuildTargets(plan.Options.Config.Build.Targets, plan.TargetNames)
	if err != nil {
		return err
	}
	if plan.Timings && strings.TrimSpace(plan.TimingsPath) != "" && len(targets) > 1 {
		return fmt.Errorf("--timings=<file> cannot be used when building multiple configured targets")
	}
	for _, target := range targets {
		targetOptions := plan.Options
		targetOptions.Config.Build.Output = target.Output
		if err := buildOnce(targetOptions, buildRequest{
			OutputDir:         target.Output,
			AppDir:            target.App,
			BinaryPath:        target.Binary,
			WASMPath:          target.WASM,
			BackendAppDir:     target.BackendApp,
			BackendBinaryPath: target.BackendBinary,
			Modules:           target.Modules,
			TimingsPath:       plan.TimingsPath,
		}, timings.clone()); err != nil {
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

func parseBuildOptions(args []string) (buildOptions, error) {
	var plan buildOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--ssr":
			plan.Options.Config.Addons = append(plan.Options.Config.Addons, ssr.Addon())
		case arg == "--debug":
			plan.Options.Debug = true
		case arg == "--timings":
			plan.Timings = true
		case strings.HasPrefix(arg, "--timings="):
			path, err := parseBuildTimingFlagValue(strings.TrimPrefix(arg, "--timings="))
			if err != nil {
				return buildOptions{}, err
			}
			plan.Timings = true
			plan.TimingsPath = path
		case arg == "--allow-missing-backend":
			plan.Options.AllowMissingBackend = true
			plan.Options.Config.Build.AllowMissingBackend = true
		case arg == "--out":
			i++
			if i >= len(args) {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.OutputDir = args[i]
		case len(arg) > len("--out=") && arg[:len("--out=")] == "--out=":
			plan.OutputDir = arg[len("--out="):]
		case arg == "--app":
			i++
			if i >= len(args) {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.AppDir = args[i]
			if strings.TrimSpace(plan.AppDir) == "" {
				return buildOptions{}, fmt.Errorf("generated app directory is required")
			}
		case len(arg) > len("--app=") && arg[:len("--app=")] == "--app=":
			plan.AppDir = arg[len("--app="):]
			if strings.TrimSpace(plan.AppDir) == "" {
				return buildOptions{}, fmt.Errorf("generated app directory is required")
			}
		case arg == "--bin":
			i++
			if i >= len(args) {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.BinaryPath = args[i]
			if strings.TrimSpace(plan.BinaryPath) == "" {
				return buildOptions{}, fmt.Errorf("binary output path is required")
			}
		case len(arg) > len("--bin=") && arg[:len("--bin=")] == "--bin=":
			plan.BinaryPath = arg[len("--bin="):]
			if strings.TrimSpace(plan.BinaryPath) == "" {
				return buildOptions{}, fmt.Errorf("binary output path is required")
			}
		case arg == "--wasm":
			i++
			if i >= len(args) {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.WASMPath = args[i]
			if strings.TrimSpace(plan.WASMPath) == "" {
				return buildOptions{}, fmt.Errorf("wasm output path is required")
			}
		case len(arg) > len("--wasm=") && arg[:len("--wasm=")] == "--wasm=":
			plan.WASMPath = arg[len("--wasm="):]
			if strings.TrimSpace(plan.WASMPath) == "" {
				return buildOptions{}, fmt.Errorf("wasm output path is required")
			}
		case arg == "--backend-app":
			i++
			if i >= len(args) {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.BackendAppDir = args[i]
			if strings.TrimSpace(plan.BackendAppDir) == "" {
				return buildOptions{}, fmt.Errorf("generated backend app directory is required")
			}
		case len(arg) > len("--backend-app=") && arg[:len("--backend-app=")] == "--backend-app=":
			plan.BackendAppDir = arg[len("--backend-app="):]
			if strings.TrimSpace(plan.BackendAppDir) == "" {
				return buildOptions{}, fmt.Errorf("generated backend app directory is required")
			}
		case arg == "--backend-bin":
			i++
			if i >= len(args) {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.BackendBinaryPath = args[i]
			if strings.TrimSpace(plan.BackendBinaryPath) == "" {
				return buildOptions{}, fmt.Errorf("backend binary output path is required")
			}
		case len(arg) > len("--backend-bin=") && arg[:len("--backend-bin=")] == "--backend-bin=":
			plan.BackendBinaryPath = arg[len("--backend-bin="):]
			if strings.TrimSpace(plan.BackendBinaryPath) == "" {
				return buildOptions{}, fmt.Errorf("backend binary output path is required")
			}
		case arg == "--config":
			i++
			if i >= len(args) {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.ConfigPath = args[i]
		case len(arg) > len("--config=") && arg[:len("--config=")] == "--config=":
			plan.ConfigPath = arg[len("--config="):]
		case arg == "--target":
			i++
			if i >= len(args) {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.TargetNames = appendNames(plan.TargetNames, args[i])
		case len(arg) > len("--target=") && arg[:len("--target=")] == "--target=":
			plan.TargetNames = appendNames(plan.TargetNames, arg[len("--target="):])
		case arg == "--module":
			i++
			if i >= len(args) {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.ModuleNames = appendNames(plan.ModuleNames, args[i])
		case len(arg) > len("--module=") && arg[:len("--module=")] == "--module=":
			plan.ModuleNames = appendNames(plan.ModuleNames, arg[len("--module="):])
		case len(arg) > 0 && arg[0] == '-':
			return buildOptions{}, fmt.Errorf("unknown build flag %q", arg)
		default:
			plan.Paths = append(plan.Paths, arg)
		}
	}

	return plan, nil
}

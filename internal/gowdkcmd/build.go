package gowdkcmd

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

const buildUsage = "usage: gowdk build [--config <file>] [--env-file <file>] [--debug] [--timings[=<file>]] [--ssr] [--allow-missing-backend] [--allow-insecure] [--obfuscate-assets] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--docker] [--docker-base <distroless|scratch>] [--deploy-recipe <caddy|nginx|split|static|systemd>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [--worker-app <dir>] [--worker-bin <file>] [--cron-app <dir>] [--cron-bin <file>] [files...]"

func build(args []string) error {
	started := time.Now()
	plan, err := loadBuildOptions(args)
	if err != nil {
		return err
	}
	return buildLoaded(plan, time.Since(started))
}

func buildLoaded(plan buildOptions, configLoad time.Duration) error {
	timings := newBuildTimingRecorder(plan.Timings)
	timings.addDuration("config_load", configLoad)
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
	WorkerAppDir      string
	WorkerBinaryPath  string
	Worker            gowdk.ContractWorkerConfig
	CronAppDir        string
	CronBinaryPath    string
	Cron              gowdk.ContractCronConfig
	Docker            bool
	DockerBase        string
	DeployRecipes     []string
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
	WorkerAppDir      string
	WorkerBinaryPath  string
	CronAppDir        string
	CronBinaryPath    string
	Docker            bool
	DockerBase        string
	DeployRecipes     []string
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
		return buildOptions{}, fmt.Errorf("--target cannot be combined with --module, --out, --app, --bin, --docker, --docker-base, --deploy-recipe, --wasm, --backend-app, --backend-bin, --worker-app, --worker-bin, --cron-app, --cron-bin, or explicit files")
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
		WorkerAppDir:      plan.WorkerAppDir,
		WorkerBinaryPath:  plan.WorkerBinaryPath,
		Worker:            plan.Options.Config.Build.Worker,
		CronAppDir:        plan.CronAppDir,
		CronBinaryPath:    plan.CronBinaryPath,
		Cron:              plan.Options.Config.Build.Cron,
		Docker:            plan.Docker,
		DockerBase:        plan.DockerBase,
		DeployRecipes:     plan.DeployRecipes,
		Modules:           plan.ModuleNames,
		Paths:             plan.Paths,
		TimingsPath:       plan.TimingsPath,
	}
}

func (plan buildOptions) hasAdHocArgs() bool {
	return plan.request().hasAdHocArgs()
}

func (plan buildOptions) shouldBuildConfiguredTargets() bool {
	if len(plan.TargetNames) > 0 {
		return true
	}
	if len(plan.Options.Config.Build.Targets) == 0 {
		return false
	}
	return !plan.request().hasAdHocArgs()
}

func (request buildRequest) hasAdHocArgs() bool {
	return strings.TrimSpace(request.OutputDir) != "" ||
		strings.TrimSpace(request.AppDir) != "" ||
		strings.TrimSpace(request.BinaryPath) != "" ||
		strings.TrimSpace(request.WASMPath) != "" ||
		strings.TrimSpace(request.BackendAppDir) != "" ||
		strings.TrimSpace(request.BackendBinaryPath) != "" ||
		strings.TrimSpace(request.WorkerAppDir) != "" ||
		strings.TrimSpace(request.WorkerBinaryPath) != "" ||
		strings.TrimSpace(request.CronAppDir) != "" ||
		strings.TrimSpace(request.CronBinaryPath) != "" ||
		request.Docker ||
		strings.TrimSpace(request.DockerBase) != "" ||
		len(request.DeployRecipes) > 0 ||
		len(request.Modules) > 0 ||
		len(request.Paths) > 0
}

func (request buildRequest) hasRoleArtifacts() bool {
	return strings.TrimSpace(request.WorkerAppDir) != "" ||
		strings.TrimSpace(request.WorkerBinaryPath) != "" ||
		strings.TrimSpace(request.CronAppDir) != "" ||
		strings.TrimSpace(request.CronBinaryPath) != ""
}

func mergeContractWorkerConfig(defaults, override gowdk.ContractWorkerConfig) gowdk.ContractWorkerConfig {
	if !roleServiceRefConfigured(override.EventSource) {
		override.EventSource = defaults.EventSource
	}
	if !roleServiceRefConfigured(override.SeenStore) {
		override.SeenStore = defaults.SeenStore
	}
	if !roleServiceRefConfigured(override.Backoff) {
		override.Backoff = defaults.Backoff
	}
	return override
}

func roleServiceRefConfigured(ref gowdk.ServiceRef) bool {
	return strings.TrimSpace(ref.ImportPath) != "" || strings.TrimSpace(ref.Function) != ""
}

func buildOnce(options cliOptions, request buildRequest, timings *buildTimingRecorder) error {
	outputDir := request.OutputDir
	if strings.TrimSpace(request.DockerBase) != "" && !request.Docker {
		return fmt.Errorf("gowdk build --docker-base requires --docker")
	}
	if request.Docker && strings.TrimSpace(request.BinaryPath) == "" {
		return fmt.Errorf("gowdk build --docker requires --bin <file>")
	}
	if request.Docker {
		base, err := normalizeDockerBase(request.DockerBase)
		if err != nil {
			return err
		}
		request.DockerBase = base
	}
	recipes, err := normalizeDeploymentRecipes(request.DeployRecipes)
	if err != nil {
		return err
	}
	request.DeployRecipes = recipes
	if strings.TrimSpace(request.BinaryPath) != "" && strings.TrimSpace(request.AppDir) == "" {
		return fmt.Errorf("gowdk build --bin requires --app <dir>")
	}
	if strings.TrimSpace(request.WASMPath) != "" && strings.TrimSpace(request.AppDir) == "" {
		return fmt.Errorf("gowdk build --wasm requires --app <dir>")
	}
	if strings.TrimSpace(request.BackendBinaryPath) != "" && strings.TrimSpace(request.BackendAppDir) == "" {
		return fmt.Errorf("gowdk build --backend-bin requires --backend-app <dir>")
	}
	if strings.TrimSpace(request.WorkerBinaryPath) != "" && strings.TrimSpace(request.WorkerAppDir) == "" {
		return fmt.Errorf("gowdk build --worker-bin requires --worker-app <dir>")
	}
	if strings.TrimSpace(request.CronBinaryPath) != "" && strings.TrimSpace(request.CronAppDir) == "" {
		return fmt.Errorf("gowdk build --cron-bin requires --cron-app <dir>")
	}
	if outputDir == "" {
		outputDir = options.Config.Build.Output
	}
	if outputDir == "" {
		if request.hasRoleArtifacts() {
			outputDir = filepath.ToSlash(filepath.Join("gowdk_cache", "roles"))
		} else {
			return fmt.Errorf(buildUsage)
		}
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
		if len(discovered) == 0 && !request.hasRoleArtifacts() {
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
		return newDevDiagnosticError("build failed", devOverlayDiagnosticsFromLang(diagnostics))
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
		var bindErr error
		bindings, bindErr = compiler.EnrichProgram(options.Config, &ir)
		return bindErr
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
		return newDevDiagnosticError("build failed", devOverlayDiagnosticsFromCompiler(report))
	}
	var contractReport contractscan.Report
	if err := timings.measure("contract_validation", func() error {
		scanned, err := scanContractReport(options.ProjectRoot)
		if err != nil {
			return err
		}
		contractReport = scanned
		linkIRContractReferencesFromReport(&ir, contractReport)
		if err := compiler.ValidateContractReferences(ir.ContractRefs); err != nil {
			return err
		}
		if err := compiler.ValidateRealtimeSubscriptionBindings(ir.RealtimeSubscriptions); err != nil {
			return err
		}
		return compiler.ValidateQueryInvalidations(options.Config, ir.QueryInvalidations)
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		var report compiler.ValidationErrors
		if errors.As(err, &report) {
			return newDevDiagnosticError("build failed", devOverlayDiagnosticsFromCompiler(report))
		}
		return fmt.Errorf("build failed")
	}

	if err := timings.measure("security_audit", func() error {
		return enforceBuildSecurityAudit(options, ir)
	}); err != nil {
		return err
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
	if err := timings.measure("final_security_audit", func() error {
		return enforceFinalBuildArtifactSecurityAudit(options, result)
	}); err != nil {
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
	if result.SitemapPath != "" {
		fmt.Println(result.SitemapPath)
	}
	if result.RobotsPath != "" {
		fmt.Println(result.RobotsPath)
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
	workerAppDir := request.WorkerAppDir
	workerBinaryPath := request.WorkerBinaryPath
	cronAppDir := request.CronAppDir
	cronBinaryPath := request.CronBinaryPath
	var buildReportEvents []buildgen.BuildEvent
	if strings.TrimSpace(appDir) != "" {
		var app appgen.Result
		if err := timings.measure("app_generation", func() error {
			var appErr error
			appOptions := appgen.OptionsFromIR(options.Config, &ir)
			appOptions.ProxyBackend = strings.TrimSpace(backendAppDir) != ""
			app, appErr = appgen.GenerateWithOptions(outputDir, appDir, appOptions)
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
			buildReportEvents = append(buildReportEvents, buildgen.BuildEvent{
				Level:   buildgen.BuildEventInfo,
				Stage:   "package",
				Kind:    "binary_built",
				Message: "compiled generated app binary",
				Path:    filepath.ToSlash(built),
			})
			if request.Docker {
				var artifacts dockerArtifacts
				if err := timings.measure("docker_artifacts", func() error {
					var dockerErr error
					artifacts, dockerErr = writeDockerArtifacts(built, request.DockerBase)
					return dockerErr
				}); err != nil {
					return err
				}
				fmt.Println(artifacts.DockerfilePath)
				fmt.Println(artifacts.DockerignorePath)
				buildReportEvents = append(buildReportEvents,
					buildgen.BuildEvent{
						Level:   buildgen.BuildEventInfo,
						Stage:   "package",
						Kind:    "dockerfile_written",
						Message: "wrote Dockerfile for generated app binary",
						Path:    filepath.ToSlash(artifacts.DockerfilePath),
						Data: map[string]string{
							"base":          artifacts.Base,
							"runtimeBinary": artifacts.RuntimeBinaryPath,
						},
					},
					buildgen.BuildEvent{
						Level:   buildgen.BuildEventInfo,
						Stage:   "package",
						Kind:    "dockerignore_written",
						Message: "wrote Docker build context ignore file",
						Path:    filepath.ToSlash(artifacts.DockerignorePath),
					},
				)
			}
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
			app, appErr = appgen.GenerateBackendWithOptions(backendAppDir, appgen.OptionsFromIR(options.Config, &ir))
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
	if strings.TrimSpace(workerAppDir) != "" {
		var app appgen.Result
		if err := timings.measure("worker_app_generation", func() error {
			var appErr error
			app, appErr = appgen.GenerateContractWorker(workerAppDir, contractReport, request.Worker)
			return appErr
		}); err != nil {
			return err
		}
		fmt.Println(app.ModulePath)
		fmt.Println(app.PackagePath)
		fmt.Println(app.MainPath)
		buildReportEvents = append(buildReportEvents, contractRoleBuildEvents("worker", app.Contracts, nil, app.MainPath)...)
		if strings.TrimSpace(workerBinaryPath) != "" {
			var built string
			if err := timings.measure("worker_binary_build", func() error {
				var buildErr error
				built, buildErr = appgen.BuildWorkerBinary(app.AppDir, workerBinaryPath)
				return buildErr
			}); err != nil {
				return err
			}
			fmt.Println(built)
			buildReportEvents = append(buildReportEvents, buildgen.BuildEvent{
				Level:   buildgen.BuildEventInfo,
				Stage:   "package",
				Kind:    "contract_role_binary_built",
				Message: "compiled generated contract worker binary",
				Path:    filepath.ToSlash(built),
				Data: map[string]string{
					"role": "worker",
				},
			})
		}
	}
	if strings.TrimSpace(cronAppDir) != "" {
		var app appgen.Result
		if err := timings.measure("cron_app_generation", func() error {
			var appErr error
			app, appErr = appgen.GenerateContractCron(cronAppDir, contractReport, request.Cron)
			return appErr
		}); err != nil {
			return err
		}
		fmt.Println(app.ModulePath)
		fmt.Println(app.PackagePath)
		fmt.Println(app.MainPath)
		buildReportEvents = append(buildReportEvents, contractRoleBuildEvents("cron", nil, app.Jobs, app.MainPath)...)
		if strings.TrimSpace(cronBinaryPath) != "" {
			var built string
			if err := timings.measure("cron_binary_build", func() error {
				var buildErr error
				built, buildErr = appgen.BuildCronBinary(app.AppDir, cronBinaryPath)
				return buildErr
			}); err != nil {
				return err
			}
			fmt.Println(built)
			buildReportEvents = append(buildReportEvents, buildgen.BuildEvent{
				Level:   buildgen.BuildEventInfo,
				Stage:   "package",
				Kind:    "contract_role_binary_built",
				Message: "compiled generated contract cron binary",
				Path:    filepath.ToSlash(built),
				Data: map[string]string{
					"role": "cron",
				},
			})
		}
	}
	if len(request.DeployRecipes) > 0 {
		var recipeArtifacts []deploymentRecipeArtifact
		if err := timings.measure("deploy_recipes", func() error {
			var recipeErr error
			recipeArtifacts, recipeErr = writeDeploymentRecipes(deploymentRecipeRequest{
				OutputDir:         outputDir,
				BinaryPath:        binaryPath,
				BackendBinaryPath: backendBinaryPath,
				WorkerBinaryPath:  workerBinaryPath,
				CronBinaryPath:    cronBinaryPath,
				Recipes:           request.DeployRecipes,
			})
			return recipeErr
		}); err != nil {
			return err
		}
		for _, artifact := range recipeArtifacts {
			fmt.Println(artifact.Path)
		}
		buildReportEvents = append(buildReportEvents, deploymentRecipeBuildEvents(recipeArtifacts)...)
	}
	if err := appendBuildReportEvents(result.BuildReportPath, buildReportEvents...); err != nil {
		return err
	}
	if _, err := timings.write(outputDir, request.TimingsPath); err != nil {
		return err
	}
	return nil
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
		worker := mergeContractWorkerConfig(plan.Options.Config.Build.Worker, target.Worker)
		cron := target.Cron
		if len(cron.Jobs) == 0 {
			cron = plan.Options.Config.Build.Cron
		}
		if err := buildOnce(targetOptions, buildRequest{
			OutputDir:         target.Output,
			AppDir:            target.App,
			BinaryPath:        target.Binary,
			WASMPath:          target.WASM,
			BackendAppDir:     target.BackendApp,
			BackendBinaryPath: target.BackendBinary,
			WorkerAppDir:      target.WorkerApp,
			WorkerBinaryPath:  target.WorkerBinary,
			Worker:            worker,
			CronAppDir:        target.CronApp,
			CronBinaryPath:    target.CronBinary,
			Cron:              cron,
			DeployRecipes:     target.DeployRecipes,
			Modules:           target.Modules,
			TimingsPath:       plan.TimingsPath,
		}, timings.clone()); err != nil {
			return fmt.Errorf("build target %q: %w", target.Name, err)
		}
	}
	return nil
}

func selectBuildTargets(targets []gowdk.BuildTargetConfig, targetNames []string) ([]gowdk.BuildTargetConfig, error) {
	selected, err := resolveConfiguredBuildTargets(targets, targetNames)
	if err != nil {
		return nil, err
	}
	normalized := make([]gowdk.BuildTargetConfig, 0, len(selected))
	for _, target := range selected {
		target.Modules = cleanNames(target.Modules)
		recipes, err := normalizeDeploymentRecipes(target.DeployRecipes)
		if err != nil {
			return nil, fmt.Errorf("build target %q: %w", target.Name, err)
		}
		target.DeployRecipes = recipes
		if strings.TrimSpace(target.Output) == "" {
			target.Output = defaultBuildTargetOutput(target.Name)
		}
		if strings.TrimSpace(target.Binary) != "" && strings.TrimSpace(target.App) == "" {
			return nil, fmt.Errorf("build target %q binary requires app", target.Name)
		}
		if strings.TrimSpace(target.WASM) != "" && strings.TrimSpace(target.App) == "" {
			return nil, fmt.Errorf("build target %q wasm requires app", target.Name)
		}
		if strings.TrimSpace(target.BackendBinary) != "" && strings.TrimSpace(target.BackendApp) == "" {
			return nil, fmt.Errorf("build target %q backend binary requires backend app", target.Name)
		}
		if strings.TrimSpace(target.WorkerBinary) != "" && strings.TrimSpace(target.WorkerApp) == "" {
			return nil, fmt.Errorf("build target %q worker binary requires worker app", target.Name)
		}
		if strings.TrimSpace(target.CronBinary) != "" && strings.TrimSpace(target.CronApp) == "" {
			return nil, fmt.Errorf("build target %q cron binary requires cron app", target.Name)
		}
		target.Output = strings.TrimSpace(target.Output)
		target.App = strings.TrimSpace(target.App)
		target.Binary = strings.TrimSpace(target.Binary)
		target.WASM = strings.TrimSpace(target.WASM)
		target.BackendApp = strings.TrimSpace(target.BackendApp)
		target.BackendBinary = strings.TrimSpace(target.BackendBinary)
		target.WorkerApp = strings.TrimSpace(target.WorkerApp)
		target.WorkerBinary = strings.TrimSpace(target.WorkerBinary)
		target.CronApp = strings.TrimSpace(target.CronApp)
		target.CronBinary = strings.TrimSpace(target.CronBinary)
		normalized = append(normalized, target)
	}
	return normalized, nil
}

func resolveConfiguredBuildTargets(targets []gowdk.BuildTargetConfig, targetNames []string) ([]gowdk.BuildTargetConfig, error) {
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

func contractRoleBuildEvents(role string, contracts []string, jobs []string, mainPath string) []buildgen.BuildEvent {
	var events []buildgen.BuildEvent
	events = append(events, buildgen.BuildEvent{
		Level:   buildgen.BuildEventInfo,
		Stage:   "package",
		Kind:    "contract_role_app_generated",
		Message: "generated standalone contract " + role + " app",
		Path:    filepath.ToSlash(mainPath),
		Data: map[string]string{
			"role": role,
		},
	})
	for _, contract := range contracts {
		events = append(events, buildgen.BuildEvent{
			Level:   buildgen.BuildEventInfo,
			Stage:   "package",
			Kind:    "contract_role_included",
			Message: "included contract in generated " + role + " role",
			Data: map[string]string{
				"role":     role,
				"contract": contract,
			},
		})
	}
	for _, job := range jobs {
		events = append(events, buildgen.BuildEvent{
			Level:   buildgen.BuildEventInfo,
			Stage:   "package",
			Kind:    "contract_role_included",
			Message: "included job in generated " + role + " role",
			Data: map[string]string{
				"role": role,
				"job":  job,
			},
		})
	}
	return events
}

func parseBuildOptions(args []string) (buildOptions, error) {
	var plan buildOptions
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if value, next, ok, missing := consumeValueFlag(args, i, "--out", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.OutputDir = value
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--app", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.AppDir = value
			if strings.TrimSpace(plan.AppDir) == "" {
				return buildOptions{}, fmt.Errorf("generated app directory is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--bin", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.BinaryPath = value
			if strings.TrimSpace(plan.BinaryPath) == "" {
				return buildOptions{}, fmt.Errorf("binary output path is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--docker-base", true); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.DockerBase = value
			if strings.TrimSpace(plan.DockerBase) == "" {
				return buildOptions{}, fmt.Errorf("Docker base is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--deploy-recipe", true); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.DeployRecipes = appendNames(plan.DeployRecipes, value)
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--wasm", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.WASMPath = value
			if strings.TrimSpace(plan.WASMPath) == "" {
				return buildOptions{}, fmt.Errorf("wasm output path is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--backend-app", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.BackendAppDir = value
			if strings.TrimSpace(plan.BackendAppDir) == "" {
				return buildOptions{}, fmt.Errorf("generated backend app directory is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--backend-bin", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.BackendBinaryPath = value
			if strings.TrimSpace(plan.BackendBinaryPath) == "" {
				return buildOptions{}, fmt.Errorf("backend binary output path is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--worker-app", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.WorkerAppDir = value
			if strings.TrimSpace(plan.WorkerAppDir) == "" {
				return buildOptions{}, fmt.Errorf("generated worker app directory is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--worker-bin", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.WorkerBinaryPath = value
			if strings.TrimSpace(plan.WorkerBinaryPath) == "" {
				return buildOptions{}, fmt.Errorf("worker binary output path is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--cron-app", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.CronAppDir = value
			if strings.TrimSpace(plan.CronAppDir) == "" {
				return buildOptions{}, fmt.Errorf("generated cron app directory is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--cron-bin", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.CronBinaryPath = value
			if strings.TrimSpace(plan.CronBinaryPath) == "" {
				return buildOptions{}, fmt.Errorf("cron binary output path is required")
			}
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--config", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.ConfigPath = value
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--env-file", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.Options.EnvFilePath = value
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--target", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.TargetNames = appendNames(plan.TargetNames, value)
			i = next
			continue
		}
		if value, next, ok, missing := consumeValueFlag(args, i, "--module", false); ok {
			if missing {
				return buildOptions{}, fmt.Errorf(buildUsage)
			}
			plan.ModuleNames = appendNames(plan.ModuleNames, value)
			i = next
			continue
		}
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
		case arg == "--allow-insecure":
			plan.Options.AllowInsecure = true
		case strings.HasPrefix(arg, "--allow-insecure="):
			codes, err := parseAllowInsecureCodes(strings.TrimPrefix(arg, "--allow-insecure="))
			if err != nil {
				return buildOptions{}, err
			}
			plan.Options.AllowInsecureCodes = codes
		case arg == "--obfuscate-assets":
			plan.Options.ObfuscateAssets = true
			plan.Options.Config.Build.ObfuscateAssets = true
			plan.Options.Config.Build.Mode = gowdk.Production
		case arg == "--docker":
			plan.Docker = true
		case len(arg) > 0 && arg[0] == '-':
			return buildOptions{}, fmt.Errorf("unknown build flag %q", arg)
		default:
			plan.Paths = append(plan.Paths, arg)
		}
	}

	return plan, nil
}

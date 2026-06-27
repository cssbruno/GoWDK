package gowdkcmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/lang"
)

const doctorUsage = "usage: gowdk doctor [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [--json] [files...]"

type doctorReport struct {
	Version     int               `json:"version"`
	Status      string            `json:"status"`
	Summary     doctorSummary     `json:"summary"`
	Environment doctorEnvironment `json:"environment"`
	Checks      []doctorCheck     `json:"checks"`
}

type doctorSummary struct {
	OK       int `json:"ok"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
	Skipped  int `json:"skipped"`
}

type doctorEnvironment struct {
	GOWDKVersion string   `json:"gowdkVersion"`
	WorkingDir   string   `json:"workingDir"`
	ConfigPath   string   `json:"configPath,omitempty"`
	EnvFilePath  string   `json:"envFilePath,omitempty"`
	EnvFile      string   `json:"envFile,omitempty"`
	EnvFileVars  []string `json:"envFileVars,omitempty"`
	ProcessVars  []string `json:"processVars,omitempty"`
	GoVersion    string   `json:"goVersion,omitempty"`
	GoEnvVersion string   `json:"goEnvVersion,omitempty"`
}

type doctorCheck struct {
	ID        string   `json:"id"`
	Status    string   `json:"status"`
	Severity  string   `json:"severity"`
	Message   string   `json:"message"`
	NextSteps []string `json:"nextSteps,omitempty"`
}

func doctor(args []string) error {
	report, jsonOutput := runDoctor(args)
	if jsonOutput {
		payload, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
	} else {
		printDoctorReport(report)
	}
	if report.Summary.Errors > 0 {
		return doctorExitError{errors: report.Summary.Errors}
	}
	return nil
}

type doctorExitError struct {
	errors int
}

func (err doctorExitError) Error() string {
	return fmt.Sprintf("doctor found %d error(s)", err.errors)
}

func (doctorExitError) SilentCLIError() {}

func runDoctor(args []string) (doctorReport, bool) {
	report := doctorReport{
		Version: 1,
		Status:  "ok",
		Environment: doctorEnvironment{
			GOWDKVersion: version,
		},
	}
	if workingDir, err := os.Getwd(); err == nil {
		report.Environment.WorkingDir = workingDir
	}

	options, configPath, moduleNames, paths, err := parseProjectOptions(args, "doctor", true)
	if err != nil {
		report.addCheck(doctorCheck{
			ID:       "arguments",
			Status:   "error",
			Severity: "error",
			Message:  err.Error(),
			NextSteps: []string{
				doctorUsage,
			},
		})
		report.finalize()
		// parseProjectOptions stops at the first bad argument, so honor a
		// requested --json output format regardless of flag order.
		return report, options.JSON || argsRequestJSON(args)
	}

	report.Environment.ConfigPath = doctorConfigDisplayPath(configPath)
	report.runGOWDKCLICheck()
	report.runGoToolchainCheck()

	configOK := report.runConfigCheck(&options, configPath)
	if !configOK {
		report.addSkipped("sources", "source discovery skipped because config did not load")
		report.addSkipped("language_check", "language check skipped because config did not load")
		report.addSkipped("routes", "route metadata skipped because config did not load")
		report.addSkipped("optional_tools", "optional tool checks skipped because config did not load")
		report.finalize()
		return report, options.JSON
	}

	if report.Environment.ConfigPath == "" {
		report.Environment.ConfigPath = doctorConfigDisplayPath(configPath)
	}

	resolvedPaths, sourcesOK := report.runSourcesCheck(options.Config, moduleNames, paths, options.ProjectRoot)
	if !sourcesOK {
		report.addSkipped("language_check", "language check skipped because no .gwdk sources were found")
		report.addSkipped("routes", "route metadata skipped because no .gwdk sources were found")
		report.runOptionalToolsCheck(options.Config)
		report.finalize()
		return report, options.JSON
	}

	app, languageOK := report.runLanguageCheck(options.Config, resolvedPaths, options.ProjectRoot)
	if !languageOK {
		report.addSkipped("routes", "route metadata skipped because language check failed")
		report.runOptionalToolsCheck(options.Config)
		report.finalize()
		return report, options.JSON
	}

	report.runRoutesCheck(options.Config, app, options.ProjectRoot)
	report.runOptionalToolsCheck(options.Config)
	report.finalize()
	return report, options.JSON
}

// argsRequestJSON reports whether the raw arguments include --json, so error
// reports can honor the requested format even when parsing stops early.
func argsRequestJSON(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}

func (report *doctorReport) runGOWDKCLICheck() {
	report.addCheck(doctorCheck{
		ID:       "gowdk_cli",
		Status:   "ok",
		Severity: "info",
		Message:  "gowdk " + version + " is running",
	})
}

func (report *doctorReport) runGoToolchainCheck() {
	goPath, err := exec.LookPath("go")
	if err != nil {
		report.addCheck(doctorCheck{
			ID:       "go_toolchain",
			Status:   "error",
			Severity: "error",
			Message:  "go executable was not found on PATH",
			NextSteps: []string{
				"Install Go and make the go executable available on PATH.",
			},
		})
		return
	}

	goVersion, versionErr := commandOutput(goPath, "version")
	goEnvVersion, envErr := commandOutput(goPath, "env", "GOVERSION")
	report.Environment.GoVersion = goVersion
	report.Environment.GoEnvVersion = goEnvVersion
	if versionErr != nil || envErr != nil {
		var messages []string
		if versionErr != nil {
			messages = append(messages, versionErr.Error())
		}
		if envErr != nil {
			messages = append(messages, envErr.Error())
		}
		report.addCheck(doctorCheck{
			ID:       "go_toolchain",
			Status:   "error",
			Severity: "error",
			Message:  "go executable is present but version checks failed: " + strings.Join(messages, "; "),
			NextSteps: []string{
				"Run go version and go env GOVERSION from the project root.",
			},
		})
		return
	}
	report.addCheck(doctorCheck{
		ID:       "go_toolchain",
		Status:   "ok",
		Severity: "info",
		Message:  goVersion + " (" + goEnvVersion + ")",
	})
}

func (report *doctorReport) runConfigCheck(options *cliOptions, configPath string) bool {
	if err := loadProjectConfig(options, configPath); err != nil {
		report.addCheck(doctorCheck{
			ID:       "config",
			Status:   "error",
			Severity: "error",
			Message:  err.Error(),
			NextSteps: []string{
				"Run gowdk init to create gowdk.config.go, or pass --config <file>.",
			},
		})
		return false
	}
	report.Environment.ConfigPath = doctorConfigDisplayPath(configPath)
	report.Environment.EnvFilePath = doctorEnvFileDisplayPath(*options)
	report.addCheck(doctorCheck{
		ID:       "config",
		Status:   "ok",
		Severity: "info",
		Message:  "loaded " + report.Environment.ConfigPath,
	})
	report.addEnvFileCheck(options)
	return true
}

func (report *doctorReport) addEnvFileCheck(options *cliOptions) {
	if options.EnvFileLoaded {
		report.Environment.EnvFilePath = options.EnvFilePath
		report.Environment.EnvFile = "loaded"
		report.Environment.EnvFileVars = append([]string(nil), options.EnvFileApplied...)
		report.Environment.ProcessVars = append([]string(nil), options.EnvFileSkipped...)
		report.addCheck(doctorCheck{
			ID:       "env_file",
			Status:   "ok",
			Severity: "info",
			Message:  doctorEnvFileLoadedMessage(options),
		})
		return
	}
	if options.EnvFileExplicit {
		report.Environment.EnvFilePath = options.EnvFilePath
		report.Environment.EnvFile = "missing"
		report.addCheck(doctorCheck{
			ID:        "env_file",
			Status:    "error",
			Severity:  "error",
			Message:   "env file was requested but did not load",
			NextSteps: []string{"Check the --env-file path."},
		})
		return
	}
	report.Environment.EnvFile = "not_found"
	report.addCheck(doctorCheck{
		ID:       "env_file",
		Status:   "ok",
		Severity: "info",
		Message:  "no .env file discovered",
	})
}

func (report *doctorReport) runSourcesCheck(config gowdk.Config, moduleNames, paths []string, projectRoot string) ([]string, bool) {
	if len(paths) == 0 {
		discovered, err := discoverProjectFiles(config, moduleNames, projectRoot)
		if err != nil {
			report.addCheck(doctorCheck{
				ID:       "sources",
				Status:   "error",
				Severity: "error",
				Message:  err.Error(),
			})
			return nil, false
		}
		paths = discovered
	}
	if len(paths) == 0 {
		report.addCheck(doctorCheck{
			ID:       "sources",
			Status:   "error",
			Severity: "error",
			Message:  "no .gwdk files found",
			NextSteps: []string{
				"Add .gwdk files, update Source.Include, or pass explicit file paths.",
			},
		})
		return nil, false
	}
	report.addCheck(doctorCheck{
		ID:       "sources",
		Status:   "ok",
		Severity: "info",
		Message:  fmt.Sprintf("found %d .gwdk source file(s)", len(paths)),
	})
	return paths, true
}

func (report *doctorReport) runLanguageCheck(config gowdk.Config, paths []string, projectRoot string) (lang.CheckResult, bool) {
	app, diagnostics := lang.CheckFilesWithOptions(config, paths, lang.CheckOptions{ProjectRoot: projectRoot})
	if diagnostics.HasErrors() {
		report.addCheck(doctorCheck{
			ID:       "language_check",
			Status:   "error",
			Severity: "error",
			Message:  fmt.Sprintf("language validation failed with %d diagnostic(s)", len(diagnostics)),
			NextSteps: []string{
				firstDiagnosticString(diagnostics),
			},
		})
		return app, false
	}
	if len(diagnostics) > 0 {
		report.addCheck(doctorCheck{
			ID:       "language_check",
			Status:   "warning",
			Severity: "warning",
			Message:  fmt.Sprintf("language validation completed with %d warning(s)", len(diagnostics)),
			NextSteps: []string{
				firstDiagnosticString(diagnostics),
			},
		})
		return app, true
	}
	report.addCheck(doctorCheck{
		ID:       "language_check",
		Status:   "ok",
		Severity: "info",
		Message:  "language validation passed",
	})
	return app, true
}

func (report *doctorReport) runRoutesCheck(config gowdk.Config, checked lang.CheckResult, projectRoot string) {
	ir := checked.IR
	if err := linkIRContractReferences(&ir, projectRoot); err != nil {
		report.addCheck(doctorCheck{
			ID:       "routes",
			Status:   "error",
			Severity: "error",
			Message:  "route metadata failed: " + err.Error(),
		})
		return
	}
	metadata := compiler.BuildRouteMetadataFromIR(config, ir)
	if len(metadata.Info) > 0 {
		report.addCheck(doctorCheck{
			ID:       "routes",
			Status:   "ok",
			Severity: "info",
			Message:  fmt.Sprintf("built %d route(s) and %d endpoint(s) with %d note(s)", len(metadata.Routes), len(metadata.Endpoints), len(metadata.Info)),
		})
		return
	}
	report.addCheck(doctorCheck{
		ID:       "routes",
		Status:   "ok",
		Severity: "info",
		Message:  fmt.Sprintf("built %d route(s) and %d endpoint(s)", len(metadata.Routes), len(metadata.Endpoints)),
	})
}

func (report *doctorReport) runOptionalToolsCheck(config gowdk.Config) {
	var warnings []string
	for _, addon := range config.Addons {
		if addon != nil && addon.Name() == "tailwind" {
			if _, err := exec.LookPath("tailwindcss"); err != nil {
				warnings = append(warnings, "tailwindcss is not available on PATH")
			}
			break
		}
	}
	if doctorNodeLooksRelevant() {
		if _, err := exec.LookPath("node"); err != nil {
			warnings = append(warnings, "node is not available on PATH")
		}
	}
	if len(warnings) > 0 {
		sort.Strings(warnings)
		report.addCheck(doctorCheck{
			ID:       "optional_tools",
			Status:   "warning",
			Severity: "warning",
			Message:  strings.Join(warnings, "; "),
			NextSteps: []string{
				"Install the optional tool, or remove the config/files that require it.",
			},
		})
		return
	}
	report.addCheck(doctorCheck{
		ID:       "optional_tools",
		Status:   "ok",
		Severity: "info",
		Message:  "no missing relevant optional tools detected",
	})
}

func (report *doctorReport) addCheck(check doctorCheck) {
	report.Checks = append(report.Checks, check)
}

func (report *doctorReport) addSkipped(id, message string) {
	report.addCheck(doctorCheck{
		ID:       id,
		Status:   "skipped",
		Severity: "info",
		Message:  message,
	})
}

func (report *doctorReport) finalize() {
	var summary doctorSummary
	for _, check := range report.Checks {
		switch check.Status {
		case "error":
			summary.Errors++
		case "warning":
			summary.Warnings++
		case "skipped":
			summary.Skipped++
		default:
			summary.OK++
		}
	}
	report.Summary = summary
	switch {
	case summary.Errors > 0:
		report.Status = "error"
	case summary.Warnings > 0:
		report.Status = "warning"
	default:
		report.Status = "ok"
	}
}

func printDoctorReport(report doctorReport) {
	fmt.Printf("GOWDK doctor: %s\n", strings.ToUpper(report.Status))
	fmt.Printf("Summary: %d ok, %d warning(s), %d error(s), %d skipped\n", report.Summary.OK, report.Summary.Warnings, report.Summary.Errors, report.Summary.Skipped)
	for _, status := range []string{"error", "warning", "ok", "skipped"} {
		for _, check := range report.Checks {
			if check.Status != status {
				continue
			}
			fmt.Printf("[%s] %s: %s\n", strings.ToUpper(check.Status), check.ID, check.Message)
			for _, next := range check.NextSteps {
				if strings.TrimSpace(next) != "" {
					fmt.Printf("  next: %s\n", next)
				}
			}
		}
	}
}

func commandOutput(name string, args ...string) (string, error) {
	command := exec.Command(name, args...)
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("%s %s: %s", filepath.Base(name), strings.Join(args, " "), message)
	}
	return strings.TrimSpace(string(output)), nil
}

func doctorConfigDisplayPath(configPath string) string {
	if strings.TrimSpace(configPath) != "" {
		return configPath
	}
	return "gowdk.config.go"
}

func doctorEnvFileDisplayPath(options cliOptions) string {
	if strings.TrimSpace(options.EnvFilePath) == "" {
		return ""
	}
	return options.EnvFilePath
}

func doctorEnvFileLoadedMessage(options *cliOptions) string {
	message := "loaded " + options.EnvFilePath
	if len(options.EnvFileApplied) == 0 && len(options.EnvFileSkipped) == 0 {
		return message
	}
	var details []string
	if len(options.EnvFileApplied) > 0 {
		details = append(details, fmt.Sprintf("from file: %s", strings.Join(options.EnvFileApplied, ", ")))
	}
	if len(options.EnvFileSkipped) > 0 {
		details = append(details, fmt.Sprintf("process kept: %s", strings.Join(options.EnvFileSkipped, ", ")))
	}
	return message + " (" + strings.Join(details, ", ") + ")"
}

func doctorNodeLooksRelevant() bool {
	for _, path := range []string{"package.json", filepath.Join("editors", "vscode", "package.json")} {
		if _, err := os.Stat(path); err == nil {
			return true
		}
	}
	return false
}

func firstDiagnosticString(diagnostics lang.Diagnostics) string {
	if len(diagnostics) == 0 {
		return ""
	}
	return diagnostics[0].String()
}

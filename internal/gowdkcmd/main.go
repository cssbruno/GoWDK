package gowdkcmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cssbruno/gowdk"
)

const version = "0.12.3" // x-release-please-version

var (
	defaultSourceIncludes = []string{"**/*.gwdk"}
	defaultSourceExcludes = []string{".git/**", "vendor/**", "node_modules/**", "**/testdata/**"}
)

// Main runs the gowdk command-line program and exits with the documented code.
func Main() {
	if err := Run(os.Args[1:]); err != nil {
		if _, silent := err.(interface{ SilentCLIError() }); !silent {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(ExitCodeFor(err))
	}
}

// exitCodeFor maps an error to its process exit code. Errors carrying an
// explicit ExitCode() (the documented gowdk audit contract) use it; every other
// error is a generic failure (1).
func ExitCodeFor(err error) int {
	if coded, ok := err.(interface{ ExitCode() int }); ok {
		if code := coded.ExitCode(); code != 0 {
			return code
		}
	}
	return 1
}

func exitCodeFor(err error) int {
	return ExitCodeFor(err)
}

// Run executes a gowdk command without exiting the current process.
func Run(args []string) error {
	if len(args) == 0 {
		usage()
		return nil
	}
	if len(args) == 1 && (args[0] == "-h" || args[0] == "--help") {
		usage()
		return nil
	}
	if help, ok := nestedCommandUsage(args); ok {
		fmt.Println(help)
		return nil
	}
	if commandHelpRequested(args) {
		if help, ok := commandUsage(args[0]); ok {
			fmt.Println(help)
			return nil
		}
	}

	if delegated, err := runProjectHelperIfNeeded(args); delegated || err != nil {
		return err
	}

	if command, ok := topLevelCommand(args[0]); ok {
		return command.Handler(args[1:])
	}
	usage()
	return fmt.Errorf("unknown command %q", args[0])
}

func run(args []string) error {
	return Run(args)
}

func commandHelpRequested(args []string) bool {
	return len(args) == 2 && (args[1] == "-h" || args[1] == "--help")
}

func nestedCommandUsage(args []string) (string, bool) {
	if len(args) != 3 || (args[2] != "-h" && args[2] != "--help") {
		return "", false
	}
	switch args[0] {
	case "inspect":
		switch args[1] {
		case "ir", "tree", "endpoint-graph", "asset-graph", "go-bindings":
			return fmt.Sprintf("usage: gowdk inspect %s [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...]", args[1]), true
		}
	case "generate":
		if args[1] == "stubs" {
			return generateUsage, true
		}
	case "env":
		if args[1] == "check" {
			return envUsage, true
		}
	case "list":
		switch args[1] {
		case "commands", "queries", "events", "jobs":
			return "usage: gowdk list commands|queries|events|jobs [--json] [dir]", true
		}
	case "playground":
		switch args[1] {
		case "policy", "export", "run":
			return playgroundUsage, true
		}
	}
	return "", false
}

func commandUsage(command string) (string, bool) {
	if descriptor, ok := topLevelCommand(command); ok {
		return descriptor.Usage(), true
	}
	return "", false
}

type topLevelCommandDescriptor struct {
	Name       string
	Handler    func([]string) error
	Usage      func() string
	ListSuffix string
}

func staticCommandUsage(value string) func() string {
	return func() string { return value }
}

var topLevelCommands = []topLevelCommandDescriptor{
	{Name: "version", Handler: printVersion, Usage: staticCommandUsage("usage: gowdk version [--json]"), ListSuffix: " [--json]         print CLI version"},
	{Name: "init", Handler: initProject, Usage: staticCommandUsage(initUsage), ListSuffix: " [--force] [--tests] [--template <site|minimal>] [dir] scaffold a starter GOWDK project"},
	{Name: "add", Handler: addAddon, Usage: staticCommandUsage(addUsage), ListSuffix: " <addon> [--config <file>] [--base-url <url>] | add --list [--registry] [--json]  wire or list addons"},
	{Name: "tokens", Handler: tokens, Usage: staticCommandUsage("usage: gowdk tokens <file.gwdk>"), ListSuffix: " <file.gwdk>       print language tokens"},
	{Name: "fmt", Handler: format, Usage: staticCommandUsage("usage: gowdk fmt [--write] [--check] <files>"), ListSuffix: " [--write] [--check] <files>  format .gwdk files (--check reports files that need formatting)"},
	{Name: "check", Handler: check, Usage: func() string { return projectCommandUsage("check", true) }, ListSuffix: " [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--warnings-as-errors] [--standalone] [--ssr] [files...] parse and validate .gwdk files"},
	{Name: "env", Handler: envCommand, Usage: staticCommandUsage(envUsage), ListSuffix: " check [--config <file>] [--env-file <file>] [--json] validate deployment environment values"},
	{Name: "fix", Handler: fixCommand, Usage: staticCommandUsage(fixUsage), ListSuffix: " [--dry-run] [--code <diagnostic-code>] [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...] apply registered safe diagnostic fixes"},
	{Name: "manifest", Handler: manifestJSON, Usage: func() string { return projectCommandUsage("manifest", false) }, ListSuffix: " [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...] print validated manifest JSON"},
	{Name: "sitemap", Handler: siteMapJSON, Usage: func() string { return projectCommandUsage("sitemap", false) }, ListSuffix: " [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...] print editor site-map JSON"},
	{Name: "routes", Handler: routesJSON, Usage: func() string { return projectCommandUsage("routes", false) }, ListSuffix: " [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...] print route and endpoint metadata JSON"},
	{Name: "endpoints", Handler: endpointsJSONCommand, Usage: func() string { return projectCommandUsage("endpoints", false) }, ListSuffix: " [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...] print endpoint metadata JSON"},
	{Name: "inspect", Handler: inspect, Usage: staticCommandUsage(inspectUsage), ListSuffix: " ir|tree|endpoint-graph|asset-graph|go-bindings [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...] print validated compiler inspection JSON"},
	{Name: "generate", Handler: generate, Usage: staticCommandUsage(generateUsage), ListSuffix: " stubs [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [files...] write missing action/API Go handler stubs"},
	{Name: "explain", Handler: explainDiagnostic, Usage: staticCommandUsage("usage: gowdk explain [--json] <diagnostic-code>"), ListSuffix: " [--json] <diagnostic-code> explain a diagnostic code and next steps"},
	{Name: "doctor", Handler: doctor, Usage: staticCommandUsage(doctorUsage), ListSuffix: " [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [--json] [files...] check local GOWDK environment and project health"},
	{Name: "test", Handler: gowdkTest, Usage: staticCommandUsage(testUsage), ListSuffix: " [--config <file>] [--env-file <file>] [--module <name>] [--target <name>] [--stage <unit|app|binary|browser>] [--run <pattern>] [--timeout <duration>] [--count <n>] [--cover] [--json] [--keep-workdir] [--browser-command <command>] [--ssr] [files...] run Go tests against generated app artifacts"},
	{Name: "audit", Handler: audit, Usage: staticCommandUsage(auditUsage), ListSuffix: " [--config <file>] [--project-root <dir>] [--env-file <file>] [--module <name>] [--ssr] [--json] [--sarif[=<file>]] [--diff <previous-report>] [--schema[=report|security]] [--emit-tests[=<file>]] [--force] [--run] [files...] check security posture, emit SARIF/JSON-Schema, diff against a previous report, and run optional runtime tests"},
	{Name: "contracts", Handler: contractsReport, Usage: staticCommandUsage("usage: gowdk contracts [--json] [dir]"), ListSuffix: " [--json] [dir]  print Go contract registration metadata"},
	{Name: "graph", Handler: contractGraph, Usage: staticCommandUsage("usage: gowdk graph [--json] [dir]"), ListSuffix: " [--json] [dir]      print command/event contract graph"},
	{Name: "trace", Handler: contractTrace, Usage: staticCommandUsage("usage: gowdk trace <contract> [--json] [dir]"), ListSuffix: " <contract> [--json] [dir] print one command/query/event/job contract trace"},
	{Name: "list", Handler: listContracts, Usage: staticCommandUsage("usage: gowdk list commands|queries|events|jobs [--json] [dir]"), ListSuffix: " commands|queries|events|jobs [--json] [dir] print filtered contract metadata"},
	{Name: "build", Handler: build, Usage: staticCommandUsage(buildUsage), ListSuffix: " [--config <file>] [--project-root <dir>] [--env-file <file>] [--debug] [--timings[=<file>]] [--ssr] [--allow-missing-backend] [--allow-insecure] [--obfuscate-assets] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--docker] [--docker-base <distroless|scratch>] [--deploy-recipe <caddy|nginx|split|static|systemd>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...] compile .gwdk files into build output"},
	{Name: "clean", Handler: clean, Usage: staticCommandUsage(cleanUsage), ListSuffix: " [--config <file>] [--target <name>] [--out <dir>] [--dry-run] [--json] remove configured build outputs"},
	{Name: "dev", Handler: dev, Usage: devUsage, ListSuffix: " [--addr <addr>] [--interval <duration>] [build flags...] build, serve, rebuild, and live reload"},
	{Name: "preview", Handler: preview, Usage: previewUsage, ListSuffix: " [--addr <addr>] [--hot] [build flags...] build and serve a local deploy preview"},
	{Name: "playground", Handler: playgroundCommand, Usage: staticCommandUsage(playgroundUsage), ListSuffix: " policy|export|run inspect sandbox policy, export projects, or run an opt-in sandbox build"},
	{Name: "serve", Handler: serve, Usage: staticCommandUsage("usage: gowdk serve --dir <dir> [--addr <addr>]"), ListSuffix: " --dir <dir> [--addr <addr>] serve generated build output locally"},
	{Name: "lsp", Handler: languageServer, Usage: staticCommandUsage(lspUsage), ListSuffix: " [--config <file>] [--project-root <dir>] [--ssr] start the language server over stdio"},
}

func topLevelCommand(name string) (topLevelCommandDescriptor, bool) {
	for _, descriptor := range topLevelCommands {
		if descriptor.Name == name {
			return descriptor, true
		}
	}
	return topLevelCommandDescriptor{}, false
}

func printVersion(args []string) error {
	if len(args) == 0 {
		fmt.Println(version)
		return nil
	}
	if len(args) == 1 && args[0] == "--json" {
		payload, err := json.MarshalIndent(struct {
			Version string `json:"version"`
		}{Version: version}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(payload))
		return nil
	}
	return errors.New("usage: gowdk version [--json]")
}

func usage() {
	fmt.Println("gowdk " + version)
	fmt.Println("compile-first Go web kit: build-time output, backend actions, SSR optional")
	fmt.Println()
	fmt.Println("Commands:")
	for _, descriptor := range topLevelCommands {
		fmt.Println("  " + descriptor.Name + descriptor.ListSuffix)
	}
}

type cliOptions struct {
	Config              gowdk.Config
	ProjectRoot         string
	JSON                bool
	Standalone          bool
	Debug               bool
	AllowMissingBackend bool
	// AllowInsecure downgrades every production security error to a warning. It is
	// the blanket bypass; prefer the scoped AllowInsecureCodes form.
	AllowInsecure bool
	// AllowInsecureCodes scopes the bypass to specific diagnostic codes
	// (--allow-insecure=CODE1,CODE2). Findings outside the set still block.
	AllowInsecureCodes map[string]bool
	ObfuscateAssets    bool
	WarningsAsErrors   bool
	EnvFilePath        string
	EnvFileLoaded      bool
	EnvFileExplicit    bool
	EnvFileApplied     []string
	EnvFileSkipped     []string
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

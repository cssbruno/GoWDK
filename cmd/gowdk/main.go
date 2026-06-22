package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cssbruno/gowdk"
)

const version = "0.9.0" // x-release-please-version

var (
	defaultSourceIncludes = []string{"**/*.gwdk"}
	defaultSourceExcludes = []string{".git/**", "vendor/**", "node_modules/**", "**/testdata/**"}
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if _, silent := err.(interface{ SilentCLIError() }); !silent {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(exitCodeFor(err))
	}
}

// exitCodeFor maps an error to its process exit code. Errors carrying an
// explicit ExitCode() (the documented gowdk audit contract) use it; every other
// error is a generic failure (1).
func exitCodeFor(err error) int {
	if coded, ok := err.(interface{ ExitCode() int }); ok {
		if code := coded.ExitCode(); code != 0 {
			return code
		}
	}
	return 1
}

func run(args []string) error {
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

	switch args[0] {
	case "version":
		return printVersion(args[1:])
	case "init":
		return initProject(args[1:])
	case "add":
		return addAddon(args[1:])
	case "tokens":
		return tokens(args[1:])
	case "fmt":
		return format(args[1:])
	case "check":
		return check(args[1:])
	case "fix":
		return fixCommand(args[1:])
	case "manifest":
		return manifestJSON(args[1:])
	case "sitemap":
		return siteMapJSON(args[1:])
	case "routes":
		return routesJSON(args[1:])
	case "endpoints":
		return endpointsJSONCommand(args[1:])
	case "inspect":
		return inspect(args[1:])
	case "generate":
		return generate(args[1:])
	case "explain":
		return explainDiagnostic(args[1:])
	case "doctor":
		return doctor(args[1:])
	case "audit":
		return audit(args[1:])
	case "contracts":
		return contractsReport(args[1:])
	case "graph":
		return contractGraph(args[1:])
	case "trace":
		return contractTrace(args[1:])
	case "list":
		return listContracts(args[1:])
	case "build":
		return build(args[1:])
	case "clean":
		return clean(args[1:])
	case "dev":
		return dev(args[1:])
	case "preview":
		return preview(args[1:])
	case "playground":
		return playgroundCommand(args[1:])
	case "serve":
		return serve(args[1:])
	case "lsp":
		return languageServer(args[1:])
	default:
		usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
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
			return fmt.Sprintf("usage: gowdk inspect %s [--config <file>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...]", args[1]), true
		}
	case "generate":
		if args[1] == "stubs" {
			return generateUsage, true
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
	switch command {
	case "version":
		return "usage: gowdk version [--json]", true
	case "init":
		return initUsage, true
	case "add":
		return addUsage, true
	case "tokens":
		return "usage: gowdk tokens <file.gwdk>", true
	case "fmt":
		return "usage: gowdk fmt [--write] [--check] <files>", true
	case "check":
		return projectCommandUsage("check", true), true
	case "fix":
		return fixUsage, true
	case "manifest":
		return projectCommandUsage("manifest", false), true
	case "sitemap":
		return projectCommandUsage("sitemap", false), true
	case "routes":
		return projectCommandUsage("routes", false), true
	case "endpoints":
		return projectCommandUsage("endpoints", false), true
	case "inspect":
		return inspectUsage, true
	case "generate":
		return generateUsage, true
	case "explain":
		return "usage: gowdk explain [--json] <diagnostic-code>", true
	case "doctor":
		return doctorUsage, true
	case "audit":
		return auditUsage, true
	case "contracts":
		return "usage: gowdk contracts [--json] [dir]", true
	case "graph":
		return "usage: gowdk graph [--json] [dir]", true
	case "trace":
		return "usage: gowdk trace <contract> [--json] [dir]", true
	case "list":
		return "usage: gowdk list commands|queries|events|jobs [--json] [dir]", true
	case "build":
		return buildUsage, true
	case "clean":
		return cleanUsage, true
	case "dev":
		return devUsage(), true
	case "preview":
		return previewUsage(), true
	case "playground":
		return playgroundUsage, true
	case "serve":
		return "usage: gowdk serve --dir <dir> [--addr <addr>]", true
	case "lsp":
		return lspUsage, true
	default:
		return "", false
	}
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
	fmt.Println("  version [--json]         print CLI version")
	fmt.Println("  init [--force] [--tests] [--template <site|minimal>] [dir] scaffold a starter GOWDK project")
	fmt.Println("  add <addon> [--config <file>] [--base-url <url>] | add --list [--registry] [--json]  wire or list addons")
	fmt.Println("  tokens <file.gwdk>       print language tokens")
	fmt.Println("  fmt [--write] [--check] <files>  format .gwdk files (--check reports files that need formatting)")
	fmt.Println("  check [--config <file>] [--env-file <file>] [--module <name>] [--json] [--warnings-as-errors] [--ssr] [files...] parse and validate .gwdk files")
	fmt.Println("  fix [--dry-run] [--code <diagnostic-code>] [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] apply registered safe diagnostic fixes")
	fmt.Println("  manifest [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] print validated manifest JSON")
	fmt.Println("  sitemap [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] print editor site-map JSON")
	fmt.Println("  routes [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] print route and endpoint metadata JSON")
	fmt.Println("  endpoints [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] print endpoint metadata JSON")
	fmt.Println("  inspect ir|tree|endpoint-graph|asset-graph|go-bindings [--config <file>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...] print validated compiler inspection JSON")
	fmt.Println("  generate stubs [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] write missing action/API Go handler stubs")
	fmt.Println("  explain [--json] <diagnostic-code> explain a diagnostic code and next steps")
	fmt.Println("  doctor [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [--json] [files...] check local GOWDK environment and project health")
	fmt.Println("  audit [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [--json] [--sarif[=<file>]] [--diff <previous-report>] [--schema[=report|security]] [--emit-tests[=<file>]] [--force] [--run] [files...] check security posture, emit SARIF/JSON-Schema, diff against a previous report, and run optional runtime tests")
	fmt.Println("  contracts [--json] [dir]  print Go contract registration metadata")
	fmt.Println("  graph [--json] [dir]      print command/event contract graph")
	fmt.Println("  trace <contract> [--json] [dir] print one command/query/event/job contract trace")
	fmt.Println("  list commands|queries|events|jobs [--json] [dir] print filtered contract metadata")
	fmt.Println("  build [--config <file>] [--env-file <file>] [--debug] [--timings[=<file>]] [--ssr] [--allow-missing-backend] [--allow-insecure] [--obfuscate-assets] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--docker] [--docker-base <distroless|scratch>] [--deploy-recipe <caddy|nginx|split|static|systemd>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...] compile .gwdk files into build output")
	fmt.Println("  clean [--config <file>] [--target <name>] [--out <dir>] [--dry-run] [--json] remove configured build outputs")
	fmt.Println("  dev [--addr <addr>] [--interval <duration>] [build flags...] build, serve, rebuild, and live reload")
	fmt.Println("  preview [--addr <addr>] [--hot] [build flags...] build and serve a local deploy preview")
	fmt.Println("  playground policy|export|run inspect sandbox policy, export projects, or run an opt-in sandbox build")
	fmt.Println("  serve --dir <dir> [--addr <addr>] serve generated build output locally")
	fmt.Println("  lsp [--config <file>] [--ssr] start the language server over stdio")
}

type cliOptions struct {
	Config              gowdk.Config
	ProjectRoot         string
	JSON                bool
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

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
)

const version = "0.6.1"

var (
	defaultSourceIncludes = []string{"**/*.gwdk"}
	defaultSourceExcludes = []string{".git/**", "vendor/**", "node_modules/**", "**/testdata/**"}
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		if _, silent := err.(interface{ SilentCLIError() }); !silent {
			fmt.Fprintln(os.Stderr, err)
		}
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
	fmt.Println("  fmt [--write] <files>    format .gwdk files")
	fmt.Println("  check [--config <file>] [--env-file <file>] [--module <name>] [--json] [--warnings-as-errors] [--ssr] [files...] parse and validate .gwdk files")
	fmt.Println("  fix [--dry-run] [--code <diagnostic-code>] [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] apply registered safe diagnostic fixes")
	fmt.Println("  manifest [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] print validated manifest JSON")
	fmt.Println("  sitemap [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] print editor site-map JSON")
	fmt.Println("  routes [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] print route and endpoint metadata JSON")
	fmt.Println("  endpoints [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] print endpoint metadata JSON")
	fmt.Println("  inspect ir|tree|endpoint-graph|go-bindings [--config <file>] [--env-file <file>] [--module <name>] [--json] [--ssr] [files...] print validated compiler inspection JSON")
	fmt.Println("  generate stubs [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [files...] write missing action/API Go handler stubs")
	fmt.Println("  explain [--json] <diagnostic-code> explain a diagnostic code and next steps")
	fmt.Println("  doctor [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [--json] [files...] check local GOWDK environment and project health")
	fmt.Println("  audit [--config <file>] [--env-file <file>] [--module <name>] [--ssr] [--json] [--emit-tests[=<file>]] [--run] [files...] check security posture and optional runtime tests")
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
	AllowInsecure       bool
	ObfuscateAssets     bool
	WarningsAsErrors    bool
	EnvFilePath         string
	EnvFileLoaded       bool
	EnvFileExplicit     bool
	EnvFileApplied      []string
	EnvFileSkipped      []string
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

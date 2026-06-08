package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	goformat "go/format"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/addons/ssr"
	"github.com/cssbruno/gowdk/internal/appgen"
	"github.com/cssbruno/gowdk/internal/buildgen"
	"github.com/cssbruno/gowdk/internal/compiler"
	"github.com/cssbruno/gowdk/internal/gwdkanalysis"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/lsp"
	"github.com/cssbruno/gowdk/internal/manifest"
)

const (
	version    = "0.2.3"
	buildUsage = "usage: gowdk build [--config <file>] [--debug] [--ssr] [--allow-missing-backend] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...]"
	initUsage  = "usage: gowdk init [--force] [--tests] [--template <site|minimal>] [dir]"
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
		return printVersion(args[1:])
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
	case "explain":
		return explainDiagnostic(args[1:])
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
	case "dev":
		return dev(args[1:])
	case "preview":
		return preview(args[1:])
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
	fmt.Println("  tokens <file.gwdk>       print language tokens")
	fmt.Println("  fmt [--write] <files>    format .gwdk files")
	fmt.Println("  check [--config <file>] [--module <name>] [--json] [--ssr] [files...] parse and validate .gwdk files")
	fmt.Println("  manifest [--config <file>] [--module <name>] [--ssr] [files...] print validated manifest JSON")
	fmt.Println("  sitemap [--config <file>] [--module <name>] [--ssr] [files...] print editor site-map JSON")
	fmt.Println("  routes [--config <file>] [--module <name>] [--ssr] [files...] print route and endpoint metadata JSON")
	fmt.Println("  explain [--json] <diagnostic-code> explain a diagnostic code and next steps")
	fmt.Println("  contracts [--json] [dir]  print Go contract registration metadata")
	fmt.Println("  graph [--json] [dir]      print command/event contract graph")
	fmt.Println("  trace <contract> [--json] [dir] print one command/query/event/job contract trace")
	fmt.Println("  list commands|queries|events|jobs [--json] [dir] print filtered contract metadata")
	fmt.Println("  build [--config <file>] [--debug] [--ssr] [--allow-missing-backend] [--target <name>] [--module <name>] [--out <dir>] [--app <dir>] [--bin <file>] [--wasm <file>] [--backend-app <dir>] [--backend-bin <file>] [files...] compile .gwdk files into build output")
	fmt.Println("  dev [--addr <addr>] [--interval <duration>] [build flags...] build, serve, rebuild, and live reload")
	fmt.Println("  preview [--addr <addr>] [--hot] [build flags...] build and serve a local deploy preview")
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
	files, err := initTemplateFiles(options.Template)
	if err != nil {
		return err
	}
	if options.Tests {
		files = append(files, initTestFiles()...)
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

func initTemplateFiles(template string) ([]initFile, error) {
	switch template {
	case "", "site":
		return siteInitTemplateFiles(), nil
	case "minimal":
		return minimalInitTemplateFiles(), nil
	default:
		return nil, fmt.Errorf("unknown init template %q", template)
	}
}

func siteInitTemplateFiles() []initFile {
	return []initFile{
		{
			Path: "gowdk.config.go",
			Body: initConfigSource(),
		},
		{
			Path: ".gitignore",
			Body: `gowdk_cache/
.gowdk/
bin/
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
}

func minimalInitTemplateFiles() []initFile {
	return []initFile{
		{
			Path: "gowdk.config.go",
			Body: initConfigSource(),
		},
		{
			Path: ".gitignore",
			Body: `gowdk_cache/
.gowdk/
bin/
`,
		},
		{
			Path: "src/pages/home.page.gwdk",
			Body: `package app

@page home
@route "/"

view {
  <main>
    <h1>GOWDK</h1>
  </main>
}
`,
		},
		{
			Path: "styles/global.css",
			Body: `body {
  font-family: system-ui, sans-serif;
  margin: 2rem;
}
`,
		},
	}
}

func initTestFiles() []initFile {
	return []initFile{{
		Path: "tests/gowdk_smoke_test.go",
		Body: initSmokeTestSource(),
	}}
}

type initOptions struct {
	Dir      string
	Force    bool
	Template string
	Tests    bool
}

type initFile struct {
	Path string
	Body string
}

func initConfigSource() string {
	file := &ast.File{
		Name: ast.NewIdent("app"),
		Decls: []ast.Decl{
			&ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{
				&ast.ImportSpec{Path: initStringLit("github.com/cssbruno/gowdk")},
			}},
			&ast.GenDecl{Tok: token.VAR, Specs: []ast.Spec{&ast.ValueSpec{
				Names:  []*ast.Ident{ast.NewIdent("Config")},
				Values: []ast.Expr{initConfigExpr()},
			}}},
		},
	}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, token.NewFileSet(), file); err != nil {
		panic(err)
	}
	formatted, err := goformat.Source(buffer.Bytes())
	if err != nil {
		panic(err)
	}
	return string(formatted)
}

func initSmokeTestSource() string {
	file := &ast.File{
		Name: ast.NewIdent("gowdktest"),
		Decls: []ast.Decl{
			&ast.GenDecl{Tok: token.IMPORT, Specs: []ast.Spec{
				&ast.ImportSpec{Path: initStringLit("os")},
				&ast.ImportSpec{Path: initStringLit("os/exec")},
				&ast.ImportSpec{Path: initStringLit("path/filepath")},
				&ast.ImportSpec{Path: initStringLit("testing")},
			}},
			&ast.FuncDecl{
				Name: ast.NewIdent("TestGOWDKBuildSmoke"),
				Type: &ast.FuncType{Params: &ast.FieldList{List: []*ast.Field{{
					Names: []*ast.Ident{ast.NewIdent("t")},
					Type:  &ast.StarExpr{X: initSel("testing", "T")},
				}}}},
				Body: initBlock(
					initDefine([]ast.Expr{ast.NewIdent("gowdkBin")}, initCall(initSel("os", "Getenv"), initStringLit("GOWDK_BIN"))),
					&ast.IfStmt{
						Cond: &ast.BinaryExpr{X: ast.NewIdent("gowdkBin"), Op: token.EQL, Y: initStringLit("")},
						Body: initBlock(initExprStmt(initCall(initSel("t", "Skip"), initStringLit("set GOWDK_BIN=/path/to/gowdk to run generated app smoke tests")))),
					},
					initDefine([]ast.Expr{ast.NewIdent("cwd"), ast.NewIdent("err")}, initCall(initSel("os", "Getwd"))),
					&ast.IfStmt{
						Cond: initNotNil("err"),
						Body: initBlock(initExprStmt(initCall(initSel("t", "Fatal"), ast.NewIdent("err")))),
					},
					initDefine([]ast.Expr{ast.NewIdent("projectRoot")}, initCall(initSel("filepath", "Dir"), ast.NewIdent("cwd"))),
					initDefine([]ast.Expr{ast.NewIdent("cmd")}, initCall(initSel("exec", "Command"), ast.NewIdent("gowdkBin"), initStringLit("build"))),
					initAssign([]ast.Expr{initSel("cmd", "Dir")}, ast.NewIdent("projectRoot")),
					initDefine([]ast.Expr{ast.NewIdent("payload"), ast.NewIdent("err")}, initCall(initSel("cmd", "CombinedOutput"))),
					&ast.IfStmt{
						Cond: initNotNil("err"),
						Body: initBlock(initExprStmt(initCall(initSel("t", "Fatalf"), initStringLit("gowdk build failed: %v\n%s"), ast.NewIdent("err"), ast.NewIdent("payload")))),
					},
					&ast.IfStmt{
						Init: initDefine([]ast.Expr{ast.NewIdent("_"), ast.NewIdent("err")}, initCall(initSel("os", "Stat"), initCall(initSel("filepath", "Join"), ast.NewIdent("projectRoot"), initStringLit(".gowdk"), initStringLit("output"), initStringLit("site"), initStringLit("index.html")))),
						Cond: initNotNil("err"),
						Body: initBlock(initExprStmt(initCall(initSel("t", "Fatalf"), initStringLit("expected generated index.html: %v"), ast.NewIdent("err")))),
					},
					&ast.IfStmt{
						Init: initDefine([]ast.Expr{ast.NewIdent("_"), ast.NewIdent("err")}, initCall(initSel("os", "Stat"), initCall(initSel("filepath", "Join"), ast.NewIdent("projectRoot"), initStringLit("bin"), initStringLit("site")))),
						Cond: initNotNil("err"),
						Body: initBlock(initExprStmt(initCall(initSel("t", "Fatalf"), initStringLit("expected generated app binary: %v"), ast.NewIdent("err")))),
					},
				),
			},
		},
	}
	var buffer bytes.Buffer
	if err := printer.Fprint(&buffer, token.NewFileSet(), file); err != nil {
		panic(err)
	}
	formatted, err := goformat.Source(buffer.Bytes())
	if err != nil {
		panic(err)
	}
	return string(formatted)
}

func initConfigExpr() ast.Expr {
	return &ast.CompositeLit{
		Type: initSel("gowdk", "Config"),
		Elts: []ast.Expr{
			initKeyValue("AppName", initStringLit("GOWDK App")),
			initKeyValue("Source", &ast.CompositeLit{
				Type: initSel("gowdk", "SourceConfig"),
				Elts: []ast.Expr{
					initKeyValue("Include", initStringSlice("src/**/*.gwdk")),
				},
			}),
			initKeyValue("Build", &ast.CompositeLit{
				Type: initSel("gowdk", "BuildConfig"),
				Elts: []ast.Expr{
					initKeyValue("Targets", &ast.CompositeLit{
						Type: &ast.ArrayType{Elt: initSel("gowdk", "BuildTargetConfig")},
						Elts: []ast.Expr{&ast.CompositeLit{
							Type: initSel("gowdk", "BuildTargetConfig"),
							Elts: []ast.Expr{
								initKeyValue("Name", initStringLit("site")),
								initKeyValue("App", initStringLit(".gowdk/site")),
								initKeyValue("Binary", initStringLit("bin/site")),
							},
						}},
					}),
				},
			}),
			initKeyValue("CSS", &ast.CompositeLit{
				Type: initSel("gowdk", "CSSConfig"),
				Elts: []ast.Expr{
					initKeyValue("Include", initStringSlice("styles/**/*.css")),
					initKeyValue("Default", initStringSlice("global")),
				},
			}),
		},
	}
}

func initStringSlice(values ...string) ast.Expr {
	elts := make([]ast.Expr, 0, len(values))
	for _, value := range values {
		elts = append(elts, initStringLit(value))
	}
	return &ast.CompositeLit{Type: &ast.ArrayType{Elt: ast.NewIdent("string")}, Elts: elts}
}

func initKeyValue(key string, value ast.Expr) ast.Expr {
	return &ast.KeyValueExpr{Key: ast.NewIdent(key), Value: value}
}

func initSel(pkg string, name string) ast.Expr {
	return &ast.SelectorExpr{X: ast.NewIdent(pkg), Sel: ast.NewIdent(name)}
}

func initStringLit(value string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)}
}

func initBlock(stmts ...ast.Stmt) *ast.BlockStmt {
	return &ast.BlockStmt{List: stmts}
}

func initExprStmt(expr ast.Expr) ast.Stmt {
	return &ast.ExprStmt{X: expr}
}

func initCall(fun ast.Expr, args ...ast.Expr) *ast.CallExpr {
	return &ast.CallExpr{Fun: fun, Args: args}
}

func initDefine(names []ast.Expr, values ...ast.Expr) ast.Stmt {
	return &ast.AssignStmt{Lhs: names, Tok: token.DEFINE, Rhs: values}
}

func initAssign(names []ast.Expr, values ...ast.Expr) ast.Stmt {
	return &ast.AssignStmt{Lhs: names, Tok: token.ASSIGN, Rhs: values}
}

func initNotNil(name string) ast.Expr {
	return &ast.BinaryExpr{X: ast.NewIdent(name), Op: token.NEQ, Y: ast.NewIdent("nil")}
}

func parseInitOptions(args []string) (initOptions, error) {
	options := initOptions{Dir: ".", Template: "site"}
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--force":
			options.Force = true
		case "--tests":
			options.Tests = true
		case "--template":
			index++
			if index >= len(args) {
				return initOptions{}, fmt.Errorf(initUsage)
			}
			options.Template = args[index]
		case "-h", "--help":
			return initOptions{}, fmt.Errorf(initUsage)
		default:
			if strings.HasPrefix(arg, "--template=") {
				options.Template = strings.TrimPrefix(arg, "--template=")
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return initOptions{}, fmt.Errorf("unknown init flag %q", arg)
			}
			if options.Dir != "." {
				return initOptions{}, fmt.Errorf(initUsage)
			}
			options.Dir = arg
		}
	}
	if strings.TrimSpace(options.Dir) == "" {
		return initOptions{}, fmt.Errorf("init directory is required")
	}
	if _, err := initTemplateFiles(options.Template); err != nil {
		return initOptions{}, err
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

	ir := gwdkanalysis.BuildIR(options.Config, app)
	if err := linkIRContractReferences(&ir, "."); err != nil {
		return err
	}
	metadata := compiler.BuildRouteMetadataFromIR(options.Config, ir)
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
	Contract       *contractBindingJSON  `json:"contract,omitempty"`
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

type contractBindingJSON struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Status      string   `json:"status"`
	Message     string   `json:"message,omitempty"`
	ImportAlias string   `json:"importAlias,omitempty"`
	ImportPath  string   `json:"importPath,omitempty"`
	Type        string   `json:"type,omitempty"`
	Result      string   `json:"result,omitempty"`
	Roles       []string `json:"roles,omitempty"`
	Handler     string   `json:"handler,omitempty"`
	Register    string   `json:"register,omitempty"`
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
		if binding.Contract.Name != "" {
			item.Contract = &contractBindingJSON{
				Name:        binding.Contract.Name,
				Kind:        string(binding.Contract.Kind),
				Status:      string(binding.Contract.Status),
				Message:     binding.Contract.Message,
				ImportAlias: binding.Contract.ImportAlias,
				ImportPath:  binding.Contract.ImportPath,
				Type:        binding.Contract.Type,
				Result:      binding.Contract.Result,
				Roles:       append([]string(nil), binding.Contract.Roles...),
				Handler:     binding.Contract.Handler,
				Register:    binding.Contract.Register,
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

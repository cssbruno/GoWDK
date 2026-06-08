package main

import (
	"bytes"
	"fmt"
	"go/ast"
	goformat "go/format"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const initUsage = "usage: gowdk init [--force] [--tests] [--template <site|minimal>] [dir]"

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

@route "/"
@guard public
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

@route "/"
@guard public

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

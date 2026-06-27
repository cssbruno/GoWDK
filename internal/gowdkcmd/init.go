package gowdkcmd

import (
	"bytes"
	"errors"
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

route "/"
guard public
css default page

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

component Hero

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

route "/"
guard public

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
	return []initFile{
		{
			Path: "go.mod",
			Body: initGoModSource(),
		},
		{
			Path: "tests/gowdk_smoke_test.go",
			Body: initSmokeTestSource(),
		},
	}
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

func initGoModSource() string {
	release := version
	if !strings.HasPrefix(release, "v") {
		release = "v" + release
	}
	return "module gowdk-starter\n\n" +
		"go 1.26.4\n\n" +
		"require github.com/cssbruno/gowdk " + release + "\n"
}

func initSmokeTestSource() string {
	source := []byte(`package gowdktest

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGOWDKGeneratedApp(t *testing.T) {
	outputDir := requiredEnv(t, "GOWDK_TEST_OUTPUT_DIR")
	appDir := requiredEnv(t, "GOWDK_TEST_APP_DIR")
	binaryPath := requiredEnv(t, "GOWDK_TEST_BINARY")

	assertFile(t, filepath.Join(outputDir, "index.html"))
	assertFile(t, filepath.Join(outputDir, "gowdk-routes.json"))
	assertFile(t, filepath.Join(outputDir, "gowdk-assets.json"))
	assertDir(t, appDir)
	assertFile(t, binaryPath)

	baseURL := strings.TrimSpace(os.Getenv("GOWDK_TEST_BASE_URL"))
	if baseURL == "" {
		return
	}

	assertGET(t, baseURL+"/_gowdk/health", http.StatusOK, "\"status\":\"ok\"")
	assertGET(t, baseURL+"/", http.StatusOK, "<main")
	assertGET(t, baseURL+"/missing", http.StatusNotFound, "")
}

func requiredEnv(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Fatalf("%s is not set; run generated app tests with gowdk test", name)
	}
	return value
}

func assertFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected file %s: %v", path, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file %s, got directory", path)
	}
}

func assertDir(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("expected directory %s: %v", path, err)
	}
	if !info.IsDir() {
		t.Fatalf("expected directory %s, got file", path)
	}
}

func assertGET(t *testing.T, target string, status int, contains string) {
	t.Helper()
	client := http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read %s response body: %v", target, err)
	}
	if response.StatusCode != status {
		t.Fatalf("GET %s status = %d, want %d with body %s", target, response.StatusCode, status, string(body))
	}
	if contains != "" && !strings.Contains(string(body), contains) {
		t.Fatalf("GET %s body does not contain %q: %s", target, contains, string(body))
	}
}
`)
	formatted, err := goformat.Source(source)
	if err != nil {
		panic(err)
	}
	return string(formatted)
}

func initConfigExpr() ast.Expr {
	return &ast.CompositeLit{
		Type: initSel("Config"),
		Elts: []ast.Expr{
			initKeyValue("AppName", initStringLit("GOWDK App")),
			initKeyValue("Source", &ast.CompositeLit{
				Type: initSel("SourceConfig"),
				Elts: []ast.Expr{
					initKeyValue("Include", initStringSlice("src/**/*.gwdk")),
				},
			}),
			initKeyValue("Build", &ast.CompositeLit{
				Type: initSel("BuildConfig"),
				Elts: []ast.Expr{
					initKeyValue("Targets", &ast.CompositeLit{
						Type: &ast.ArrayType{Elt: initSel("BuildTargetConfig")},
						Elts: []ast.Expr{&ast.CompositeLit{
							Type: initSel("BuildTargetConfig"),
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
				Type: initSel("CSSConfig"),
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

func initSel(name string) ast.Expr {
	return &ast.SelectorExpr{X: ast.NewIdent("gowdk"), Sel: ast.NewIdent(name)}
}

func initStringLit(value string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(value)}
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
				return initOptions{}, errors.New(initUsage)
			}
			options.Template = args[index]
		case "-h", "--help":
			return initOptions{}, errors.New(initUsage)
		default:
			if strings.HasPrefix(arg, "--template=") {
				options.Template = strings.TrimPrefix(arg, "--template=")
				continue
			}
			if strings.HasPrefix(arg, "-") {
				return initOptions{}, fmt.Errorf("unknown init flag %q", arg)
			}
			if options.Dir != "." {
				return initOptions{}, errors.New(initUsage)
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

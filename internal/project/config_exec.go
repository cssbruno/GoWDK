package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
)

type executableConfig struct {
	AppName string                   `json:"appName"`
	Source  gowdk.SourceConfig       `json:"source"`
	Modules []gowdk.ModuleConfig     `json:"modules"`
	Render  gowdk.RenderConfig       `json:"render"`
	Build   gowdk.BuildConfig        `json:"build"`
	CSS     gowdk.CSSConfig          `json:"css"`
	Addons  []executableAddonDetails `json:"addons"`
}

type executableAddonDetails struct {
	Index        int             `json:"index"`
	Name         string          `json:"name"`
	Features     []gowdk.Feature `json:"features"`
	CSSProcessor bool            `json:"cssProcessor"`
}

type executableCSSResponse struct {
	Result gowdk.CSSResult `json:"result"`
	Error  string          `json:"error"`
}

type goListPackage struct {
	ImportPath string `json:"ImportPath"`
	Name       string `json:"Name"`
	Dir        string `json:"Dir"`
	Module     *struct {
		Dir string `json:"Dir"`
	} `json:"Module"`
}

type executableAddon struct {
	configPath string
	index      int
	name       string
	features   []gowdk.Feature
}

type executableCSSAddon struct {
	executableAddon
}

func loadExecutableConfig(configPath string) (gowdk.Config, error) {
	payload, err := runConfigHelper(configPath, "config", nil)
	if err != nil {
		return gowdk.Config{}, err
	}
	var wire executableConfig
	if err := json.Unmarshal(payload, &wire); err != nil {
		return gowdk.Config{}, fmt.Errorf("decode executable config: %w", err)
	}

	config := gowdk.Config{
		AppName: wire.AppName,
		Source:  wire.Source,
		Modules: wire.Modules,
		Render:  wire.Render,
		Build:   wire.Build,
		CSS:     wire.CSS,
	}
	for _, addon := range wire.Addons {
		proxy := executableAddon{
			configPath: configPath,
			index:      addon.Index,
			name:       addon.Name,
			features:   append([]gowdk.Feature(nil), addon.Features...),
		}
		if addon.CSSProcessor {
			config.Addons = append(config.Addons, executableCSSAddon{executableAddon: proxy})
			continue
		}
		config.Addons = append(config.Addons, proxy)
	}
	return config, nil
}

func (addon executableAddon) Name() string {
	return addon.name
}

func (addon executableAddon) Features() []gowdk.Feature {
	return append([]gowdk.Feature(nil), addon.features...)
}

func (addon executableCSSAddon) ProcessCSS(context gowdk.CSSContext) (gowdk.CSSResult, error) {
	input, err := json.Marshal(context)
	if err != nil {
		return gowdk.CSSResult{}, err
	}
	payload, err := runConfigHelper(addon.configPath, "css", input, strconv.Itoa(addon.index))
	if err != nil {
		return gowdk.CSSResult{}, err
	}
	var response executableCSSResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return gowdk.CSSResult{}, fmt.Errorf("decode css processor response: %w", err)
	}
	if response.Error != "" {
		return gowdk.CSSResult{}, fmt.Errorf("%s", response.Error)
	}
	return response.Result, nil
}

func runConfigHelper(configPath string, command string, input []byte, args ...string) ([]byte, error) {
	packageInfo, err := loadConfigPackage(configPath)
	if err != nil {
		return nil, err
	}
	helperDir, err := makeConfigHelperDir(packageInfo)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(helperDir)

	source, err := configHelperSource(packageInfo.ImportPath)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(helperDir, "main.go"), []byte(source), 0o644); err != nil {
		return nil, err
	}

	cmdArgs := append([]string{"run", ".", command}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = helperDir
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("run executable config helper: %s", message)
	}
	return output, nil
}

func loadConfigPackage(configPath string) (goListPackage, error) {
	absolute, err := filepath.Abs(configPath)
	if err != nil {
		return goListPackage{}, err
	}
	cmd := exec.Command("go", "list", "-json", ".")
	cmd.Dir = filepath.Dir(absolute)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return goListPackage{}, fmt.Errorf("go list config package: %s", message)
	}
	var packageInfo goListPackage
	if err := json.Unmarshal(output, &packageInfo); err != nil {
		return goListPackage{}, err
	}
	if packageInfo.ImportPath == "" {
		return goListPackage{}, fmt.Errorf("config package has no import path")
	}
	if packageInfo.Name == "main" {
		return goListPackage{}, fmt.Errorf("config package %s is package main and cannot be imported", packageInfo.ImportPath)
	}
	if packageInfo.Module == nil || packageInfo.Module.Dir == "" {
		return goListPackage{}, fmt.Errorf("config package %s is not inside a Go module", packageInfo.ImportPath)
	}
	return packageInfo, nil
}

func makeConfigHelperDir(packageInfo goListPackage) (string, error) {
	cacheRoot := filepath.Join(packageInfo.Module.Dir, ".gowdk")
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return "", err
	}
	return os.MkdirTemp(cacheRoot, "config-loader-*")
}

const configHelperImportPlaceholder = "gowdk.local/config-placeholder"

func configHelperSource(configImportPath string) (string, error) {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "gowdk-config-helper.go", configHelperSourceTemplate, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return "", fmt.Errorf("parse config helper source: %w", err)
	}
	replaced := false
	for _, item := range file.Imports {
		if item.Name == nil || item.Name.Name != "configpkg" {
			continue
		}
		importPath, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			return "", fmt.Errorf("parse config helper import path: %w", err)
		}
		if importPath != configHelperImportPlaceholder {
			continue
		}
		item.Path.Value = strconv.Quote(configImportPath)
		replaced = true
	}
	if !replaced {
		return "", fmt.Errorf("config helper source is missing config package import placeholder")
	}
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fileSet, file); err != nil {
		return "", fmt.Errorf("format config helper source: %w", err)
	}
	return buffer.String(), nil
}

const configHelperSourceTemplate = `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/cssbruno/gowdk"
	configpkg "gowdk.local/config-placeholder"
)

type executableConfig struct {
	AppName string                    ` + "`json:\"appName\"`" + `
	Source  gowdk.SourceConfig       ` + "`json:\"source\"`" + `
	Modules []gowdk.ModuleConfig     ` + "`json:\"modules\"`" + `
	Render  gowdk.RenderConfig       ` + "`json:\"render\"`" + `
	Build   gowdk.BuildConfig        ` + "`json:\"build\"`" + `
	CSS     gowdk.CSSConfig          ` + "`json:\"css\"`" + `
	Addons  []executableAddonDetails ` + "`json:\"addons\"`" + `
}

type executableAddonDetails struct {
	Index        int             ` + "`json:\"index\"`" + `
	Name         string          ` + "`json:\"name\"`" + `
	Features     []gowdk.Feature ` + "`json:\"features\"`" + `
	CSSProcessor bool            ` + "`json:\"cssProcessor\"`" + `
}

type executableCSSResponse struct {
	Result gowdk.CSSResult ` + "`json:\"result\"`" + `
	Error  string          ` + "`json:\"error\"`" + `
}

func main() {
	if len(os.Args) < 2 {
		fail("missing command")
	}
	switch os.Args[1] {
	case "config":
		writeConfig()
	case "css":
		if len(os.Args) < 3 {
			fail("missing addon index")
		}
		index, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fail(err.Error())
		}
		processCSS(index)
	default:
		fail("unknown command " + os.Args[1])
	}
}

func writeConfig() {
	config := configpkg.Config
	wire := executableConfig{
		AppName: config.AppName,
		Source:  config.Source,
		Modules: config.Modules,
		Render:  config.Render,
		Build:   config.Build,
		CSS:     config.CSS,
	}
	for index, addon := range config.Addons {
		_, cssProcessor := addon.(gowdk.CSSProcessor)
		wire.Addons = append(wire.Addons, executableAddonDetails{
			Index:        index,
			Name:         addon.Name(),
			Features:     addon.Features(),
			CSSProcessor: cssProcessor,
		})
	}
	writeJSON(wire)
}

func processCSS(index int) {
	config := configpkg.Config
	if index < 0 || index >= len(config.Addons) {
		writeJSON(executableCSSResponse{Error: fmt.Sprintf("addon index %%d is out of range", index)})
		return
	}
	processor, ok := config.Addons[index].(gowdk.CSSProcessor)
	if !ok {
		writeJSON(executableCSSResponse{Error: fmt.Sprintf("addon %%s does not implement CSSProcessor", config.Addons[index].Name())})
		return
	}
	var context gowdk.CSSContext
	if err := json.NewDecoder(os.Stdin).Decode(&context); err != nil {
		writeJSON(executableCSSResponse{Error: err.Error()})
		return
	}
	result, err := processor.ProcessCSS(context)
	if err != nil {
		writeJSON(executableCSSResponse{Error: err.Error()})
		return
	}
	writeJSON(executableCSSResponse{Result: result})
}

func writeJSON(value any) {
	if err := json.NewEncoder(os.Stdout).Encode(value); err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
`

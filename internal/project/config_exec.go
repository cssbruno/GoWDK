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
	Env     gowdk.EnvConfig          `json:"env"`
	Build   gowdk.BuildConfig        `json:"build"`
	CSS     gowdk.CSSConfig          `json:"css"`
	Addons  []executableAddonDetails `json:"addons"`
}

type executableAddonDetails struct {
	Index              int                      `json:"index"`
	Name               string                   `json:"name"`
	Features           []gowdk.Feature          `json:"features"`
	CSSProcessor       bool                     `json:"cssProcessor"`
	GoBlockConsumer    bool                     `json:"goBlockConsumer"`
	GoBlockTargets     []string                 `json:"goBlockTargets,omitempty"`
	SEOProvider        bool                     `json:"seoProvider"`
	SEOOptions         gowdk.SEOOptions         `json:"seoOptions,omitempty"`
	AuthSession        bool                     `json:"authSession"`
	AuthSessionOptions gowdk.AuthSessionOptions `json:"authSessionOptions,omitempty"`
}

type executableCSSResponse struct {
	Result gowdk.CSSResult `json:"result"`
	Error  string          `json:"error"`
}

type executableGoBlockRequest struct {
	Target  gowdk.GoBlockTarget  `json:"target"`
	Context gowdk.GoBlockContext `json:"context"`
}

type executableGoBlockResponse struct {
	Diagnostics []gowdk.GoBlockDiagnostic `json:"diagnostics,omitempty"`
	Files       []gowdk.GoBlockFile       `json:"files,omitempty"`
	Error       string                    `json:"error,omitempty"`
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
	configPath         string
	index              int
	name               string
	features           []gowdk.Feature
	goBlockTargets     []string
	seoProvider        bool
	seoOptions         gowdk.SEOOptions
	authSessionOptions gowdk.AuthSessionOptions
}

type executableCSSAddon struct {
	executableAddon
}

type executableGoBlockAddon struct {
	executableAddon
}

type executableCSSGoBlockAddon struct {
	executableAddon
}

type executableAuthAddon struct {
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
		Env:     wire.Env,
		Build:   wire.Build,
		CSS:     wire.CSS,
	}
	for _, addon := range wire.Addons {
		proxy := executableAddon{
			configPath:         configPath,
			index:              addon.Index,
			name:               addon.Name,
			features:           append([]gowdk.Feature(nil), addon.Features...),
			goBlockTargets:     append([]string(nil), addon.GoBlockTargets...),
			seoProvider:        addon.SEOProvider,
			seoOptions:         cloneExecutableSEOOptions(addon.SEOOptions),
			authSessionOptions: addon.AuthSessionOptions,
		}
		switch {
		case addon.AuthSession:
			config.Addons = append(config.Addons, executableAuthAddon{executableAddon: proxy})
		case addon.CSSProcessor && addon.GoBlockConsumer:
			config.Addons = append(config.Addons, executableCSSGoBlockAddon{executableAddon: proxy})
		case addon.CSSProcessor:
			config.Addons = append(config.Addons, executableCSSAddon{executableAddon: proxy})
		case addon.GoBlockConsumer:
			config.Addons = append(config.Addons, executableGoBlockAddon{executableAddon: proxy})
		default:
			config.Addons = append(config.Addons, proxy)
		}
	}
	return config, nil
}

func (addon executableAddon) Name() string {
	return addon.name
}

func (addon executableAddon) Features() []gowdk.Feature {
	return append([]gowdk.Feature(nil), addon.features...)
}

func (addon executableAddon) SEOOptions() gowdk.SEOOptions {
	return cloneExecutableSEOOptions(addon.seoOptions)
}

func (addon executableAuthAddon) AuthSessionOptions() gowdk.AuthSessionOptions {
	return addon.authSessionOptions
}

func cloneExecutableSEOOptions(options gowdk.SEOOptions) gowdk.SEOOptions {
	options.Disallow = append([]string(nil), options.Disallow...)
	options.ExtraURLs = append([]gowdk.SEOURL(nil), options.ExtraURLs...)
	options.ExtraURLProvider = nil
	return options
}

func (addon executableCSSAddon) ProcessCSS(context gowdk.CSSContext) (gowdk.CSSResult, error) {
	return addon.executableAddon.processCSS(context)
}

func (addon executableCSSGoBlockAddon) ProcessCSS(context gowdk.CSSContext) (gowdk.CSSResult, error) {
	return addon.executableAddon.processCSS(context)
}

func (addon executableAddon) processCSS(context gowdk.CSSContext) (gowdk.CSSResult, error) {
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

func (addon executableGoBlockAddon) GoBlockTargets() []string {
	return addon.executableAddon.goBlockTargetsCopy()
}

func (addon executableCSSGoBlockAddon) GoBlockTargets() []string {
	return addon.executableAddon.goBlockTargetsCopy()
}

func (addon executableAddon) goBlockTargetsCopy() []string {
	return append([]string(nil), addon.goBlockTargets...)
}

func (addon executableGoBlockAddon) ValidateGoBlock(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) []gowdk.GoBlockDiagnostic {
	return addon.executableAddon.validateGoBlock(target, context)
}

func (addon executableCSSGoBlockAddon) ValidateGoBlock(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) []gowdk.GoBlockDiagnostic {
	return addon.executableAddon.validateGoBlock(target, context)
}

func (addon executableAddon) validateGoBlock(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) []gowdk.GoBlockDiagnostic {
	input, err := json.Marshal(executableGoBlockRequest{Target: target, Context: context})
	if err != nil {
		return addon.goBlockProxyDiagnostics(target, fmt.Sprintf("encode addon go block validation request: %v", err))
	}
	payload, err := runConfigHelper(addon.configPath, "go-block-validate", input, strconv.Itoa(addon.index))
	if err != nil {
		return addon.goBlockProxyDiagnostics(target, fmt.Sprintf("run addon go block validation: %v", err))
	}
	var response executableGoBlockResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return addon.goBlockProxyDiagnostics(target, fmt.Sprintf("decode addon go block validation response: %v", err))
	}
	if response.Error != "" {
		return addon.goBlockProxyDiagnostics(target, response.Error)
	}
	return response.Diagnostics
}

func (addon executableGoBlockAddon) GeneratedGo(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) ([]gowdk.GoBlockFile, error) {
	return addon.executableAddon.generatedGo(target, context)
}

func (addon executableCSSGoBlockAddon) GeneratedGo(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) ([]gowdk.GoBlockFile, error) {
	return addon.executableAddon.generatedGo(target, context)
}

func (addon executableAddon) generatedGo(target gowdk.GoBlockTarget, context gowdk.GoBlockContext) ([]gowdk.GoBlockFile, error) {
	input, err := json.Marshal(executableGoBlockRequest{Target: target, Context: context})
	if err != nil {
		return nil, err
	}
	payload, err := runConfigHelper(addon.configPath, "go-block-generate", input, strconv.Itoa(addon.index))
	if err != nil {
		return nil, err
	}
	var response executableGoBlockResponse
	if err := json.Unmarshal(payload, &response); err != nil {
		return nil, fmt.Errorf("decode addon go block generation response: %w", err)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("%s", response.Error)
	}
	return response.Files, nil
}

func (addon executableAddon) goBlockProxyDiagnostics(target gowdk.GoBlockTarget, message string) []gowdk.GoBlockDiagnostic {
	return []gowdk.GoBlockDiagnostic{{
		Code:    "addon_go_block_diagnostic",
		Message: fmt.Sprintf("addon %q go block proxy failed: %s", addon.name, message),
		Span:    target.Span,
	}}
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
	Env     gowdk.EnvConfig          ` + "`json:\"env\"`" + `
	Build   gowdk.BuildConfig        ` + "`json:\"build\"`" + `
	CSS     gowdk.CSSConfig          ` + "`json:\"css\"`" + `
	Addons  []executableAddonDetails ` + "`json:\"addons\"`" + `
}

type executableAddonDetails struct {
	Index              int                      ` + "`json:\"index\"`" + `
	Name               string                   ` + "`json:\"name\"`" + `
	Features           []gowdk.Feature          ` + "`json:\"features\"`" + `
	CSSProcessor       bool                     ` + "`json:\"cssProcessor\"`" + `
	GoBlockConsumer    bool                     ` + "`json:\"goBlockConsumer\"`" + `
	GoBlockTargets     []string                 ` + "`json:\"goBlockTargets,omitempty\"`" + `
	SEOProvider        bool                     ` + "`json:\"seoProvider\"`" + `
	SEOOptions         gowdk.SEOOptions         ` + "`json:\"seoOptions,omitempty\"`" + `
	AuthSession        bool                     ` + "`json:\"authSession\"`" + `
	AuthSessionOptions gowdk.AuthSessionOptions ` + "`json:\"authSessionOptions,omitempty\"`" + `
}

type executableCSSResponse struct {
	Result gowdk.CSSResult ` + "`json:\"result\"`" + `
	Error  string          ` + "`json:\"error\"`" + `
}

type executableGoBlockRequest struct {
	Target  gowdk.GoBlockTarget  ` + "`json:\"target\"`" + `
	Context gowdk.GoBlockContext ` + "`json:\"context\"`" + `
}

type executableGoBlockResponse struct {
	Diagnostics []gowdk.GoBlockDiagnostic ` + "`json:\"diagnostics,omitempty\"`" + `
	Files       []gowdk.GoBlockFile       ` + "`json:\"files,omitempty\"`" + `
	Error       string                    ` + "`json:\"error,omitempty\"`" + `
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
	case "go-block-validate":
		if len(os.Args) < 3 {
			fail("missing addon index")
		}
		index, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fail(err.Error())
		}
		validateGoBlock(index)
	case "go-block-generate":
		if len(os.Args) < 3 {
			fail("missing addon index")
		}
		index, err := strconv.Atoi(os.Args[2])
		if err != nil {
			fail(err.Error())
		}
		generateGoBlock(index)
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
		Env:     config.Env,
		Build:   config.Build,
		CSS:     config.CSS,
	}
	for index, addon := range config.Addons {
		_, cssProcessor := addon.(gowdk.CSSProcessor)
		goBlockConsumer, hasGoBlockConsumer := addon.(gowdk.GoBlockConsumer)
		seoProvider, hasSEOProvider := addon.(gowdk.SEOProvider)
		authSessionProvider, hasAuthSessionProvider := addon.(gowdk.AuthSessionProvider)
		var goBlockTargets []string
		if hasGoBlockConsumer {
			goBlockTargets = goBlockConsumer.GoBlockTargets()
		}
		var seoOptions gowdk.SEOOptions
		if hasSEOProvider {
			seoOptions = seoProvider.SEOOptions()
		}
		var authSessionOptions gowdk.AuthSessionOptions
		if hasAuthSessionProvider {
			authSessionOptions = authSessionProvider.AuthSessionOptions()
		}
		wire.Addons = append(wire.Addons, executableAddonDetails{
			Index:              index,
			Name:               addon.Name(),
			Features:           addon.Features(),
			CSSProcessor:       cssProcessor,
			GoBlockConsumer:    hasGoBlockConsumer,
			GoBlockTargets:     goBlockTargets,
			SEOProvider:        hasSEOProvider,
			SEOOptions:         seoOptions,
			AuthSession:        hasAuthSessionProvider,
			AuthSessionOptions: authSessionOptions,
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

func validateGoBlock(index int) {
	config := configpkg.Config
	consumer, err := goBlockConsumer(config, index)
	if err != nil {
		writeJSON(executableGoBlockResponse{Error: err.Error()})
		return
	}
	var request executableGoBlockRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		writeJSON(executableGoBlockResponse{Error: err.Error()})
		return
	}
	writeJSON(executableGoBlockResponse{
		Diagnostics: consumer.ValidateGoBlock(request.Target, request.Context),
	})
}

func generateGoBlock(index int) {
	config := configpkg.Config
	consumer, err := goBlockConsumer(config, index)
	if err != nil {
		writeJSON(executableGoBlockResponse{Error: err.Error()})
		return
	}
	var request executableGoBlockRequest
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		writeJSON(executableGoBlockResponse{Error: err.Error()})
		return
	}
	files, err := consumer.GeneratedGo(request.Target, request.Context)
	if err != nil {
		writeJSON(executableGoBlockResponse{Error: err.Error()})
		return
	}
	writeJSON(executableGoBlockResponse{Files: files})
}

func goBlockConsumer(config gowdk.Config, index int) (gowdk.GoBlockConsumer, error) {
	if index < 0 || index >= len(config.Addons) {
		return nil, fmt.Errorf("addon index %d is out of range", index)
	}
	consumer, ok := config.Addons[index].(gowdk.GoBlockConsumer)
	if !ok {
		return nil, fmt.Errorf("addon %s does not implement GoBlockConsumer", config.Addons[index].Name())
	}
	return consumer, nil
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

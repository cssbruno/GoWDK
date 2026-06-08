package buildgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/clientrt"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

func clientRuntimeArtifacts(config gowdk.Config, pages []manifest.Page, outputDir string, layouts map[string]manifest.Layout, components map[string]view.Component) []plannedAssetArtifact {
	for _, page := range pages {
		viewSource := page.Blocks.ViewBody
		if source, err := composePageViewSource(page, layouts); err == nil {
			viewSource = source
		}
		if pageUsesPartialRuntime(page, viewSource) || pageUsesSPANavigationRuntime(config, page, viewSource, components) {
			return []plannedAssetArtifact{{
				AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(clientRuntimeAssetPath))},
				contents:      clientrt.Source(),
			}}
		}
	}
	return nil
}

var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d}

func runtimeArtifacts(config gowdk.Config, app manifest.Manifest, outputDir string, layouts map[string]manifest.Layout, components map[string]view.Component) ([]plannedAssetArtifact, error) {
	var artifacts []plannedAssetArtifact
	artifacts = append(artifacts, clientRuntimeArtifacts(config, app.Pages, outputDir, layouts, components)...)
	artifacts = append(artifacts, storeRuntimeArtifacts(app.Pages, outputDir)...)
	islands, err := islandRuntimeArtifacts(config, app, outputDir, layouts)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, islands...)
	clientGoBlocks, err := clientGoBlockRuntimeArtifacts(app.Pages, outputDir)
	if err != nil {
		return nil, err
	}
	artifacts = append(artifacts, clientGoBlocks...)
	return dedupeAssetArtifacts(artifacts), nil
}

func storeRuntimeArtifacts(pages []manifest.Page, outputDir string) []plannedAssetArtifact {
	for _, page := range pages {
		if len(page.Stores) > 0 {
			return []plannedAssetArtifact{{
				AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(storeRuntimeAssetPath))},
				contents:      []byte(storeRuntimeSource()),
			}}
		}
	}
	return nil
}

func storeRuntimeSource() string {
	return compactGeneratedJSSource(`(() => {
  const registry = window.__gowdkStores || (window.__gowdkStores = {
    stores: Object.create(null),
    listeners: Object.create(null)
  });

  registry.init = (name, state) => {
    if (!name || registry.stores[name]) return;
    registry.stores[name] = Object.assign({}, state || {});
  };

  registry.get = (name) => {
    return Object.assign({}, registry.stores[name] || {});
  };

  registry.set = (name, next) => {
    if (!name) return;
    registry.stores[name] = Object.assign({}, registry.stores[name] || {}, next || {});
    (registry.listeners[name] || []).slice().forEach((listener) => listener(registry.get(name)));
  };

  registry.subscribe = (name, listener) => {
    if (!name || typeof listener !== "function") return () => {};
    if (!registry.listeners[name]) registry.listeners[name] = [];
    registry.listeners[name].push(listener);
    return () => {
      registry.listeners[name] = (registry.listeners[name] || []).filter((item) => item !== listener);
    };
  };

  document.querySelectorAll("script[type=\"application/json\"][data-gowdk-store]").forEach((node) => {
    const name = node.getAttribute("data-gowdk-store");
    try {
      registry.init(name, JSON.parse(node.textContent || "{}"));
    } catch (error) {
      registry.init(name, {});
    }
  });
})();
`)
}

func islandRuntimeArtifacts(config gowdk.Config, app manifest.Manifest, outputDir string, layouts map[string]manifest.Layout) ([]plannedAssetArtifact, error) {
	components := componentsByName(app.Components)
	includeSourceMaps := config.Build.DebugAssets()
	planned := map[string]plannedAssetArtifact{}
	for _, page := range app.Pages {
		source, err := composePageViewSource(page, layouts)
		if err != nil {
			source = page.Blocks.ViewBody
		}
		usages, err := recursiveManifestComponentCallUsages(source, components, page.Package, componentUses(page.Uses))
		if err != nil {
			continue
		}
		for _, usage := range usages {
			component := usage.component
			switch manifestComponentRuntimeMode(usage.call.Island, component) {
			case "wasm":
				if _, exists := planned[filepath.Join(outputDir, filepath.FromSlash(islandWASMAssetPath(component.Name)))]; !exists {
					artifact, err := islandWASMArtifact(outputDir, component)
					if err != nil {
						return nil, err
					}
					addAsset(planned, artifact)
				}
				if strings.TrimSpace(component.WASM.Package) != "" {
					artifact, err := islandWASMExecArtifact(outputDir)
					if err != nil {
						return nil, err
					}
					addAsset(planned, artifact)
				}
				addAsset(planned, islandWASMLoaderArtifact(outputDir, component.Name))
			case "":
				if componentNeedsJSIsland(component) || usage.call.ReactiveProps {
					addAsset(planned, islandJSArtifact(outputDir, component.Name, includeSourceMaps))
					if includeSourceMaps {
						addAsset(planned, islandJSSourceMapArtifact(outputDir, component))
					}
				}
			}
		}
	}
	if len(planned) == 0 {
		return nil, nil
	}
	paths := make([]string, 0, len(planned))
	for path := range planned {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	artifacts := make([]plannedAssetArtifact, 0, len(paths))
	for _, path := range paths {
		artifacts = append(artifacts, planned[path])
	}
	return artifacts, nil
}

func islandScriptHrefs(source string, components map[string]view.Component, ownerPackage string, uses map[string]string) []string {
	usages, err := recursiveViewComponentCallUsages(source, components, ownerPackage, uses)
	if err != nil {
		return nil
	}
	seen := map[string]bool{}
	var scripts []string
	for _, usage := range usages {
		href := ""
		component := usage.component
		switch viewComponentRuntimeMode(usage.call.Island, component) {
		case "wasm":
			href = "/" + islandWASMLoaderAssetPath(component.Name)
		case "":
			if component.StateJSON != "" || component.HandlersJSON != "" || len(component.Emits) > 0 || usage.call.ReactiveProps {
				href = "/" + islandJSAssetPath(component.Name)
			}
		}
		if href == "" || seen[href] {
			continue
		}
		seen[href] = true
		scripts = append(scripts, href)
	}
	sort.Strings(scripts)
	return scripts
}

func clientGoBlockRuntimeArtifacts(pages []manifest.Page, outputDir string) ([]plannedAssetArtifact, error) {
	planned := map[string]plannedAssetArtifact{}
	for _, page := range pages {
		script, ok := clientGoBlock(page)
		if !ok {
			continue
		}
		wasm, err := clientGoBlockWASMArtifact(outputDir, page, script)
		if err != nil {
			return nil, err
		}
		addAsset(planned, wasm)
		execArtifact, err := islandWASMExecArtifact(outputDir)
		if err != nil {
			return nil, err
		}
		addAsset(planned, execArtifact)
		addAsset(planned, clientGoBlockWASMLoaderArtifact(outputDir, page))
	}
	if len(planned) == 0 {
		return nil, nil
	}
	paths := make([]string, 0, len(planned))
	for path := range planned {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	artifacts := make([]plannedAssetArtifact, 0, len(paths))
	for _, path := range paths {
		artifacts = append(artifacts, planned[path])
	}
	return artifacts, nil
}

func clientGoBlockHrefs(page manifest.Page) []string {
	if _, ok := clientGoBlock(page); !ok {
		return nil
	}
	return []string{"/" + clientGoBlockWASMLoaderAssetPath(page)}
}

func clientGoBlock(page manifest.Page) (manifest.GoBlock, bool) {
	required := clientGoBlockMountExportName(page)
	for _, script := range page.Blocks.GoBlocks {
		if script.Target != "client" {
			continue
		}
		if strings.Contains(script.Body, "//go:wasmexport "+required) || strings.Contains(script.Body, "//go:wasmexport\t"+required) {
			return script, true
		}
	}
	return manifest.GoBlock{}, false
}

func manifestComponentRuntimeMode(explicit string, component manifest.Component) string {
	if explicit != "" {
		return explicit
	}
	if strings.TrimSpace(component.WASM.Package) != "" {
		return "wasm"
	}
	return ""
}

func viewComponentRuntimeMode(explicit string, component view.Component) string {
	if explicit != "" {
		return explicit
	}
	return component.DefaultIsland
}

type resolvedViewComponentCallUsage struct {
	call      view.ComponentCallUsage
	component view.Component
}

func recursiveViewComponentCallUsages(source string, components map[string]view.Component, ownerPackage string, uses map[string]string) ([]resolvedViewComponentCallUsage, error) {
	var usages []resolvedViewComponentCallUsage
	visiting := map[string]bool{}
	var walk func(string, string, map[string]string) error
	walk = func(source string, ownerPackage string, uses map[string]string) error {
		direct, err := view.ComponentCallUsages(source)
		if err != nil {
			return err
		}
		for _, usage := range direct {
			component, ok := lookupViewComponent(components, usage.Component, ownerPackage, uses)
			if !ok {
				continue
			}
			usages = append(usages, resolvedViewComponentCallUsage{call: usage, component: component})
			identity := component.Identity()
			if visiting[identity] {
				continue
			}
			visiting[identity] = true
			if err := walk(component.Body, component.Package, component.Uses); err != nil {
				return err
			}
			delete(visiting, identity)
		}
		return nil
	}
	if err := walk(source, ownerPackage, uses); err != nil {
		return nil, err
	}
	return usages, nil
}

func lookupViewComponent(components map[string]view.Component, name string, ownerPackage string, uses map[string]string) (view.Component, bool) {
	if strings.Contains(name, ".") {
		if component, ok := components[name]; ok {
			return component, true
		}
		alias, componentName, _ := strings.Cut(name, ".")
		packageName := uses[alias]
		if packageName == "" {
			return view.Component{}, false
		}
		component, ok := components[componentRegistryKey(packageName, componentName)]
		return component, ok
	}
	if ownerPackage != "" {
		component, ok := components[componentRegistryKey(ownerPackage, name)]
		return component, ok
	}
	component, ok := components[name]
	return component, ok
}

type resolvedManifestComponentCallUsage struct {
	call      view.ComponentCallUsage
	component manifest.Component
}

func recursiveManifestComponentCallUsages(source string, components map[string]manifest.Component, ownerPackage string, uses map[string]string) ([]resolvedManifestComponentCallUsage, error) {
	var usages []resolvedManifestComponentCallUsage
	visiting := map[string]bool{}
	var walk func(string, string, map[string]string) error
	walk = func(source string, ownerPackage string, uses map[string]string) error {
		direct, err := view.ComponentCallUsages(source)
		if err != nil {
			return err
		}
		for _, usage := range direct {
			component, ok := lookupManifestComponent(components, usage.Component, ownerPackage, uses)
			if !ok {
				continue
			}
			usages = append(usages, resolvedManifestComponentCallUsage{call: usage, component: component})
			identity := manifestComponentIdentity(component)
			if visiting[identity] {
				continue
			}
			visiting[identity] = true
			if err := walk(component.Blocks.ViewBody, component.Package, componentUses(component.Uses)); err != nil {
				return err
			}
			delete(visiting, identity)
		}
		return nil
	}
	if err := walk(source, ownerPackage, uses); err != nil {
		return nil, err
	}
	return usages, nil
}

func lookupManifestComponent(components map[string]manifest.Component, name string, ownerPackage string, uses map[string]string) (manifest.Component, bool) {
	if strings.Contains(name, ".") {
		if component, ok := components[name]; ok {
			return component, true
		}
		alias, componentName, _ := strings.Cut(name, ".")
		packageName := uses[alias]
		if packageName == "" {
			return manifest.Component{}, false
		}
		component, ok := components[componentRegistryKey(packageName, componentName)]
		return component, ok
	}
	if ownerPackage != "" {
		component, ok := components[componentRegistryKey(ownerPackage, name)]
		return component, ok
	}
	component, ok := components[name]
	return component, ok
}

func statefulComponentNames(components []manifest.Component) map[string]bool {
	out := map[string]bool{}
	for _, component := range components {
		if componentNeedsJSIsland(component) {
			out[component.Name] = true
			if component.Package != "" {
				out[component.Package+"."+component.Name] = true
			}
		}
	}
	return out
}

func componentNeedsJSIsland(component manifest.Component) bool {
	return component.State.Type.Name != "" || component.Blocks.Client || len(component.Emits) > 0
}

func componentsByName(components []manifest.Component) map[string]manifest.Component {
	out := map[string]manifest.Component{}
	for _, component := range components {
		key := componentRegistryKey(component.Package, component.Name)
		out[key] = component
		if component.Package == "" {
			out[component.Name] = component
		}
	}
	return out
}

func addAsset(artifacts map[string]plannedAssetArtifact, artifact plannedAssetArtifact) {
	artifacts[artifact.Path] = artifact
}

func dedupeAssetArtifacts(artifacts []plannedAssetArtifact) []plannedAssetArtifact {
	if len(artifacts) < 2 {
		return artifacts
	}
	seen := map[string]plannedAssetArtifact{}
	for _, artifact := range artifacts {
		seen[artifact.Path] = artifact
	}
	paths := make([]string, 0, len(seen))
	for path := range seen {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	out := make([]plannedAssetArtifact, 0, len(paths))
	for _, path := range paths {
		out = append(out, seen[path])
	}
	return out
}

func islandJSArtifact(outputDir, componentName string, includeSourceMap bool) plannedAssetArtifact {
	assetPath := islandJSAssetPath(componentName)
	source := islandJSSource(componentName, includeSourceMap)
	if !includeSourceMap {
		source = compactGeneratedJSSource(source)
	}
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(source),
	}
}

func compactGeneratedJSSource(source string) string {
	var lines []string
	for _, line := range strings.Split(source, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n") + "\n"
}

func islandJSSourceMapArtifact(outputDir string, component manifest.Component) plannedAssetArtifact {
	assetPath := islandJSSourceMapAssetPath(component.Name)
	source := islandJSSource(component.Name, true)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      islandJSSourceMap(component, source),
	}
}

func islandWASMArtifact(outputDir string, component manifest.Component) (plannedAssetArtifact, error) {
	assetPath := islandWASMAssetPath(component.Name)
	contents := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}
	if strings.TrimSpace(component.WASM.Package) != "" {
		wasm, err := buildWASMIslandPackage(component)
		if err != nil {
			return plannedAssetArtifact{}, err
		}
		contents = wasm
	}
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      contents,
	}, nil
}

func buildWASMIslandPackage(component manifest.Component) ([]byte, error) {
	packagePath := strings.TrimSpace(component.WASM.Package)
	temp, err := os.CreateTemp("", "gowdk-"+componentAssetName(component.Name)+"-*.wasm")
	if err != nil {
		return nil, wasmIslandDiagnosticError(component, "wasm_package_build_error", packagePath, fmt.Errorf("create temp output: %w", err))
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, wasmIslandDiagnosticError(component, "wasm_package_build_error", packagePath, fmt.Errorf("close temp output: %w", err))
	}
	defer os.Remove(tempPath)

	dir, buildPackage, err := wasmIslandBuildContext(packagePath)
	if err != nil {
		return nil, wasmIslandDiagnosticError(component, "wasm_package_build_error", packagePath, err)
	}
	if err := validateWASMIslandPackageImports(component, dir, buildPackage, packagePath); err != nil {
		return nil, err
	}
	command := exec.Command("go", "build", "-buildvcs=false", "-o", tempPath, buildPackage)
	if dir != "" {
		command.Dir = dir
	}
	command.Env = append(envWithout(os.Environ(), "GOOS", "GOARCH"), "GOOS=js", "GOARCH=wasm")
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, wasmIslandDiagnosticError(component, "wasm_package_build_error", packagePath, fmt.Errorf("failed to build with GOOS=js GOARCH=wasm: %w\n%s", err, strings.TrimSpace(string(output))))
	}
	contents, err := os.ReadFile(tempPath)
	if err != nil {
		return nil, wasmIslandDiagnosticError(component, "wasm_package_build_error", packagePath, fmt.Errorf("read built artifact: %w", err))
	}
	if !bytes.HasPrefix(contents, wasmMagic) {
		return nil, wasmIslandDiagnosticError(component, "wasm_package_entrypoint_error", packagePath, fmt.Errorf("did not produce a browser WASM module; declare a package main with a main function"))
	}
	if err := validateWASMIslandExports(component, packagePath, contents); err != nil {
		return nil, err
	}
	return contents, nil
}

func clientGoBlockWASMArtifact(outputDir string, page manifest.Page, script manifest.GoBlock) (plannedAssetArtifact, error) {
	assetPath := clientGoBlockWASMAssetPath(page)
	contents, err := buildClientGoBlockWASM(page, script)
	if err != nil {
		return plannedAssetArtifact{}, err
	}
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      contents,
	}, nil
}

func buildClientGoBlockWASM(page manifest.Page, script manifest.GoBlock) ([]byte, error) {
	temp, err := os.CreateTemp(sourceDir(page.Source), ".gowdk-"+clientGoBlockAssetName(page)+"-*.wasm")
	if err != nil {
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_build_error", fmt.Errorf("create temp output: %w", err))
	}
	tempPath := temp.Name()
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_build_error", fmt.Errorf("close temp output: %w", err))
	}
	defer os.Remove(tempPath)

	source, err := clientGoBlockWASMSource(page, script)
	if err != nil {
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_source_error", err)
	}
	sourceFile, err := os.CreateTemp(sourceDir(page.Source), "gowdk-"+clientGoBlockAssetName(page)+"-*.go")
	if err != nil {
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_build_error", fmt.Errorf("create temp source: %w", err))
	}
	sourcePath := sourceFile.Name()
	if _, err := sourceFile.WriteString(source); err != nil {
		sourceFile.Close()
		_ = os.Remove(sourcePath)
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_build_error", fmt.Errorf("write temp source: %w", err))
	}
	if err := sourceFile.Close(); err != nil {
		_ = os.Remove(sourcePath)
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_build_error", fmt.Errorf("close temp source: %w", err))
	}
	defer os.Remove(sourcePath)
	if err := validateClientGoBlockWASMImports(page, sourcePath); err != nil {
		return nil, err
	}

	command := exec.Command("go", "build", "-buildvcs=false", "-o", tempPath, sourcePath)
	command.Dir = sourceDir(page.Source)
	command.Env = append(envWithout(os.Environ(), "GOOS", "GOARCH"), "GOOS=js", "GOARCH=wasm")
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_build_error", fmt.Errorf("failed to build with GOOS=js GOARCH=wasm: %w\n%s", err, strings.TrimSpace(string(output))))
	}
	contents, err := os.ReadFile(tempPath)
	if err != nil {
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_build_error", fmt.Errorf("read built artifact: %w", err))
	}
	if !bytes.HasPrefix(contents, wasmMagic) {
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_entrypoint_error", fmt.Errorf("did not produce a client-side WASM module"))
	}
	if err := validateClientGoBlockWASMExports(page, contents); err != nil {
		return nil, err
	}
	return contents, nil
}

func clientGoBlockWASMSource(page manifest.Page, script manifest.GoBlock) (string, error) {
	body := strings.TrimSpace(script.Body)
	sourceWithoutImports := "package main\n" + body + "\n"
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "go_client.gwdk.go", sourceWithoutImports, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return "", fmt.Errorf("go client block has invalid client-side Go: %w", err)
	}

	decls := make([]ast.Decl, 0, len(file.Decls)+2)
	if generatedImports := clientGoBlockGOWDKImportDecl(page.Imports, file); generatedImports != nil {
		decls = append(decls, generatedImports)
	}
	decls = append(decls, file.Decls...)
	if !fileDeclaresMain(file) {
		decls = append(decls, &ast.FuncDecl{
			Name: ast.NewIdent("main"),
			Type: &ast.FuncType{Params: &ast.FieldList{}},
			Body: &ast.BlockStmt{},
		})
	}

	generated := &ast.File{
		Name:     ast.NewIdent("main"),
		Decls:    decls,
		Comments: file.Comments,
	}
	var buffer bytes.Buffer
	if err := format.Node(&buffer, fileSet, generated); err != nil {
		return "", fmt.Errorf("format generated client-side Go: %w", err)
	}
	return buffer.String(), nil
}

func clientGoBlockGOWDKImportDecl(imports []manifest.Import, file *ast.File) ast.Decl {
	used := usedIdentifiers(file)
	localImports := map[string]bool{}
	for _, spec := range importSpecs(file) {
		alias, importPath := importSpecAliasPath(spec)
		if alias == "" {
			alias = path.Base(importPath)
		}
		if alias != "" {
			localImports[alias] = true
		}
	}
	var specs []ast.Spec
	for _, item := range imports {
		importPath := strings.TrimSpace(item.Path)
		if importPath == "" {
			continue
		}
		alias := clientGoBlockImportAlias(item)
		if !used[alias] || localImports[alias] {
			continue
		}
		specs = append(specs, importSpec(item.Alias, importPath))
	}
	if len(specs) == 0 {
		return nil
	}
	sort.Slice(specs, func(i, j int) bool {
		leftAlias, leftPath := importSpecAliasPath(specs[i].(*ast.ImportSpec))
		rightAlias, rightPath := importSpecAliasPath(specs[j].(*ast.ImportSpec))
		if leftAlias == rightAlias {
			return leftPath < rightPath
		}
		return leftAlias < rightAlias
	})
	return &ast.GenDecl{Tok: token.IMPORT, Specs: specs}
}

func importSpec(alias string, importPath string) ast.Spec {
	spec := &ast.ImportSpec{
		Path: &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(importPath)},
	}
	if strings.TrimSpace(alias) != "" {
		spec.Name = ast.NewIdent(strings.TrimSpace(alias))
	}
	return spec
}

func validateClientGoBlockWASMImports(page manifest.Page, sourcePath string) error {
	file, err := parser.ParseFile(token.NewFileSet(), sourcePath, nil, parser.ImportsOnly)
	if err != nil {
		return clientGoBlockDiagnosticError(page, "client_go_block_wasm_import_error", fmt.Errorf("parse imports: %w", err))
	}
	for _, item := range file.Imports {
		importPath, err := strconv.Unquote(item.Path.Value)
		if err != nil {
			return clientGoBlockDiagnosticError(page, "client_go_block_wasm_import_error", fmt.Errorf("parse import path: %w", err))
		}
		reason, forbidden := forbiddenWASMIslandImports[importPath]
		if !forbidden {
			continue
		}
		return clientGoBlockDiagnosticError(page, "unsupported_wasm_import", fmt.Errorf("imports unsupported client-side package %q: %s", importPath, reason))
	}
	return nil
}

func validateClientGoBlockWASMExports(page manifest.Page, contents []byte) error {
	exports, err := wasmExportNames(contents)
	if err != nil {
		return clientGoBlockDiagnosticError(page, "client_go_block_wasm_export_error", err)
	}
	required := clientGoBlockMountExportName(page)
	if !exports[required] {
		return clientGoBlockDiagnosticError(page, "client_go_block_wasm_export_error", fmt.Errorf("missing required WASM export: %s", required))
	}
	return nil
}

func validateWASMIslandExports(component manifest.Component, packagePath string, contents []byte) error {
	exports, err := wasmExportNames(contents)
	if err != nil {
		return wasmIslandDiagnosticError(component, "wasm_package_export_error", packagePath, err)
	}
	required := []string{
		"GOWDKMount" + component.Name,
		"GOWDKHandle" + component.Name,
		"GOWDKDestroy" + component.Name,
	}
	var missing []string
	for _, name := range required {
		if !exports[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return wasmIslandDiagnosticError(component, "wasm_package_export_error", packagePath, fmt.Errorf("missing required WASM exports: %s", strings.Join(missing, ", ")))
	}
	return nil
}

func wasmExportNames(contents []byte) (map[string]bool, error) {
	if len(contents) < 8 || !bytes.Equal(contents[:4], wasmMagic) {
		return nil, fmt.Errorf("invalid WASM module")
	}
	offset := 8
	for offset < len(contents) {
		sectionID := contents[offset]
		offset++
		sectionSize, next, ok := readWASMVarUint32(contents, offset)
		if !ok {
			return nil, fmt.Errorf("invalid WASM section size")
		}
		offset = next
		sectionEnd := offset + int(sectionSize)
		if sectionEnd < offset || sectionEnd > len(contents) {
			return nil, fmt.Errorf("invalid WASM section length")
		}
		if sectionID != 7 {
			offset = sectionEnd
			continue
		}
		exports := map[string]bool{}
		count, cursor, ok := readWASMVarUint32(contents, offset)
		if !ok {
			return nil, fmt.Errorf("invalid WASM export count")
		}
		for range count {
			nameLen, next, ok := readWASMVarUint32(contents, cursor)
			if !ok {
				return nil, fmt.Errorf("invalid WASM export name length")
			}
			cursor = next
			nameEnd := cursor + int(nameLen)
			if nameEnd < cursor || nameEnd > sectionEnd {
				return nil, fmt.Errorf("invalid WASM export name")
			}
			name := string(contents[cursor:nameEnd])
			cursor = nameEnd
			if cursor >= sectionEnd {
				return nil, fmt.Errorf("invalid WASM export descriptor")
			}
			cursor++
			_, next, ok = readWASMVarUint32(contents, cursor)
			if !ok {
				return nil, fmt.Errorf("invalid WASM export index")
			}
			cursor = next
			exports[name] = true
		}
		return exports, nil
	}
	return map[string]bool{}, nil
}

func readWASMVarUint32(contents []byte, offset int) (uint32, int, bool) {
	var value uint32
	var shift uint
	for i := 0; i < 5 && offset < len(contents); i++ {
		b := contents[offset]
		offset++
		value |= uint32(b&0x7f) << shift
		if b&0x80 == 0 {
			return value, offset, true
		}
		shift += 7
	}
	return 0, offset, false
}

type wasmIslandBuildDiagnosticError struct {
	err        error
	diagnostic BuildDiagnostic
}

func (err *wasmIslandBuildDiagnosticError) Error() string {
	if err == nil || err.err == nil {
		return ""
	}
	return err.err.Error()
}

func (err *wasmIslandBuildDiagnosticError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.err
}

func (err *wasmIslandBuildDiagnosticError) BuildDiagnostics() []BuildDiagnostic {
	if err == nil {
		return nil
	}
	return []BuildDiagnostic{err.diagnostic}
}

func wasmIslandDiagnosticError(component manifest.Component, code, packagePath string, cause error) error {
	message := fmt.Sprintf("component %s wasm package %q %v", component.Name, packagePath, cause)
	return &wasmIslandBuildDiagnosticError{
		err: fmt.Errorf("%s", message),
		diagnostic: BuildDiagnostic{
			Code:          code,
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSourceSpan(component.WASM.Span, component.Span),
			Message:       message,
		},
	}
}

type clientGoBlockBuildDiagnosticError struct {
	err        error
	diagnostic BuildDiagnostic
}

func (err *clientGoBlockBuildDiagnosticError) Error() string {
	if err == nil || err.err == nil {
		return ""
	}
	return err.err.Error()
}

func (err *clientGoBlockBuildDiagnosticError) Unwrap() error {
	if err == nil {
		return nil
	}
	return err.err
}

func (err *clientGoBlockBuildDiagnosticError) BuildDiagnostics() []BuildDiagnostic {
	if err == nil {
		return nil
	}
	return []BuildDiagnostic{err.diagnostic}
}

func clientGoBlockDiagnosticError(page manifest.Page, code string, cause error) error {
	message := fmt.Sprintf("page %s go client WASM: %v", page.ID, cause)
	return &clientGoBlockBuildDiagnosticError{
		err: fmt.Errorf("%s", message),
		diagnostic: BuildDiagnostic{
			Code:    code,
			Source:  page.Source,
			Span:    goBlockTargetSpan(page.Blocks.Spans.GoBlocks, "client"),
			Message: message,
		},
	}
}

func goBlockTargetSpan(spans []manifest.NamedSpan, target string) manifest.SourceSpan {
	for _, span := range spans {
		if span.Name == target {
			return span.Span
		}
	}
	return manifest.SourceSpan{}
}

var forbiddenWASMIslandImports = map[string]string{
	"net":          "network listeners belong in server api {} or act {} handlers",
	"net/http":     "HTTP clients and servers belong in server api {} or act {} handlers",
	"os/exec":      "process execution is not available in browser WASM islands",
	"plugin":       "Go plugins are not available in browser WASM islands",
	"syscall":      "use syscall/js for browser interop instead of syscall",
	"unsafe":       "unsafe is not allowed in browser WASM island packages",
	"database/sql": "database access belongs in server api {} or act {} handlers",
}

func importSpecs(file *ast.File) []*ast.ImportSpec {
	if file == nil {
		return nil
	}
	var specs []*ast.ImportSpec
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		for _, spec := range gen.Specs {
			importSpec, ok := spec.(*ast.ImportSpec)
			if ok {
				specs = append(specs, importSpec)
			}
		}
	}
	return specs
}

func importSpecAliasPath(spec *ast.ImportSpec) (string, string) {
	importPath := strings.Trim(spec.Path.Value, `"`)
	if unquoted, err := strconv.Unquote(spec.Path.Value); err == nil {
		importPath = unquoted
	}
	alias := ""
	if spec.Name != nil {
		alias = spec.Name.Name
	}
	return alias, importPath
}

func usedIdentifiers(file *ast.File) map[string]bool {
	used := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		base, ok := selector.X.(*ast.Ident)
		if ok {
			used[base.Name] = true
		}
		return true
	})
	return used
}

func fileDeclaresMain(file *ast.File) bool {
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if ok && fn.Name != nil && fn.Name.Name == "main" {
			return true
		}
	}
	return false
}

func clientGoBlockImportAlias(item manifest.Import) string {
	if strings.TrimSpace(item.Alias) != "" {
		return item.Alias
	}
	importPath := strings.TrimSpace(item.Path)
	if importPath == "" {
		return ""
	}
	return path.Base(importPath)
}

func validateWASMIslandPackageImports(component manifest.Component, dir, buildPackage, packagePath string) error {
	sourceDir, ok := wasmIslandLocalSourceDir(dir, buildPackage, packagePath)
	if !ok {
		return nil
	}
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("component %s wasm package %q: read package source: %w", component.Name, packagePath, err)
	}
	fileset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		filePath := filepath.Join(sourceDir, entry.Name())
		file, err := parser.ParseFile(fileset, filePath, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("component %s wasm package %q: parse imports in %s: %w", component.Name, packagePath, entry.Name(), err)
		}
		for _, item := range file.Imports {
			importPath, err := strconv.Unquote(item.Path.Value)
			if err != nil {
				return fmt.Errorf("component %s wasm package %q: parse import path in %s: %w", component.Name, packagePath, entry.Name(), err)
			}
			reason, forbidden := forbiddenWASMIslandImports[importPath]
			if !forbidden {
				continue
			}
			position := fileset.Position(item.Pos())
			return wasmIslandDiagnosticError(component, "unsupported_wasm_import", packagePath, fmt.Errorf("imports unsupported browser package %q at %s:%d: %s", importPath, filepath.ToSlash(filePath), position.Line, reason))
		}
	}
	return nil
}

func wasmIslandLocalSourceDir(dir, buildPackage, packagePath string) (string, bool) {
	if filepath.IsAbs(packagePath) {
		return packagePath, true
	}
	if strings.HasPrefix(packagePath, ".") {
		abs, err := filepath.Abs(packagePath)
		if err != nil {
			return "", false
		}
		return abs, true
	}
	if dir != "" && strings.HasPrefix(buildPackage, "./") {
		return filepath.Join(dir, filepath.FromSlash(strings.TrimPrefix(buildPackage, "./"))), true
	}
	return "", false
}

func wasmIslandBuildContext(packagePath string) (string, string, error) {
	if !filepath.IsAbs(packagePath) {
		return "", packagePath, nil
	}
	info, err := os.Stat(packagePath)
	if err != nil {
		return "", "", err
	}
	if !info.IsDir() {
		return "", "", fmt.Errorf("package path is not a directory")
	}
	moduleRoot := packagePath
	for {
		if _, err := os.Stat(filepath.Join(moduleRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(moduleRoot)
		if parent == moduleRoot {
			return "", "", fmt.Errorf("no go.mod found for local package")
		}
		moduleRoot = parent
	}
	rel, err := filepath.Rel(moduleRoot, packagePath)
	if err != nil {
		return "", "", err
	}
	return moduleRoot, "./" + filepath.ToSlash(rel), nil
}

func envWithout(env []string, names ...string) []string {
	blocked := map[string]bool{}
	for _, name := range names {
		blocked[name+"="] = true
	}
	out := make([]string, 0, len(env))
	for _, entry := range env {
		skip := false
		for prefix := range blocked {
			if strings.HasPrefix(entry, prefix) {
				skip = true
				break
			}
		}
		if !skip {
			out = append(out, entry)
		}
	}
	return out
}

func islandWASMLoaderArtifact(outputDir, componentName string) plannedAssetArtifact {
	assetPath := islandWASMLoaderAssetPath(componentName)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(islandWASMLoaderSource(componentName)),
	}
}

func clientGoBlockWASMLoaderArtifact(outputDir string, page manifest.Page) plannedAssetArtifact {
	assetPath := clientGoBlockWASMLoaderAssetPath(page)
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      []byte(clientGoBlockWASMLoaderSource(page)),
	}
}

func islandWASMExecArtifact(outputDir string) (plannedAssetArtifact, error) {
	assetPath := islandWASMExecAssetPath()
	contents, err := os.ReadFile(filepath.Join(runtime.GOROOT(), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return plannedAssetArtifact{}, fmt.Errorf("read Go wasm_exec.js runtime: %w", err)
	}
	return plannedAssetArtifact{
		AssetArtifact: AssetArtifact{Path: filepath.Join(outputDir, filepath.FromSlash(assetPath))},
		contents:      contents,
	}, nil
}

func islandJSAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".js")
}

func islandWASMAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".wasm")
}

func islandWASMLoaderAssetPath(componentName string) string {
	return path.Join(islandRuntimeDir, componentAssetName(componentName)+".wasm.js")
}

func clientGoBlockWASMAssetPath(page manifest.Page) string {
	return path.Join(islandRuntimeDir, "pages", clientGoBlockAssetName(page)+".wasm")
}

func clientGoBlockWASMLoaderAssetPath(page manifest.Page) string {
	return path.Join(islandRuntimeDir, "pages", clientGoBlockAssetName(page)+".wasm.js")
}

func islandWASMExecAssetPath() string {
	return path.Join(islandRuntimeDir, "wasm_exec.js")
}

func islandJSSourceMapAssetPath(componentName string) string {
	return islandJSAssetPath(componentName) + ".map"
}

func componentAssetName(componentName string) string {
	name := exportedSafe(componentName)
	if name == "" {
		return "component"
	}
	return name
}

func clientGoBlockAssetName(page manifest.Page) string {
	name := exportedPascalSafe(page.ID)
	if name == "" {
		return "Page"
	}
	return name
}

func clientGoBlockMountExportName(page manifest.Page) string {
	return "GOWDKMount" + clientGoBlockAssetName(page)
}

func exportedPascalSafe(value string) string {
	var builder strings.Builder
	upperNext := true
	for _, char := range value {
		validLower := char >= 'a' && char <= 'z'
		validUpper := char >= 'A' && char <= 'Z'
		validDigit := char >= '0' && char <= '9'
		if !validLower && !validUpper && !validDigit {
			upperNext = true
			continue
		}
		if builder.Len() == 0 && validDigit {
			builder.WriteByte('P')
		}
		if upperNext && validLower {
			char -= 'a' - 'A'
		}
		builder.WriteRune(char)
		upperNext = false
	}
	return builder.String()
}

func islandJSSource(componentName string, includeSourceMap bool) string {
	component := strconv.Quote(componentName)
	name := componentAssetName(componentName)
	mountFunction := "mount" + name + "Island"
	destroyFunction := "destroy" + name + "Island"
	source := fmt.Sprintf(`(() => {
  const component = %s;
  const selector = "gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"js\"]";
  const booleanAttrs = new Set(["allowfullscreen", "async", "autofocus", "autoplay", "checked", "controls", "default", "defer", "disabled", "formnovalidate", "hidden", "inert", "ismap", "loop", "multiple", "muted", "nomodule", "novalidate", "open", "readonly", "required", "reversed", "selected"]);
  const staleAsyncResult = Symbol("gowdk stale async result");
  const bindingTable = Object.freeze([
    { kind: "text", selector: "[data-gowdk-binding-text]", id: "data-gowdk-binding-text", field: "data-gowdk-bind" },
    { kind: "value", selector: "[data-gowdk-binding-value]", id: "data-gowdk-binding-value", field: "data-gowdk-bind-value" },
    { kind: "checked", selector: "[data-gowdk-binding-checked]", id: "data-gowdk-binding-checked", field: "data-gowdk-bind-checked" },
    { kind: "conditional", selector: "[data-gowdk-binding-if]", id: "data-gowdk-binding-if" },
    { kind: "list", selector: "[data-gowdk-binding-list]", id: "data-gowdk-binding-list" },
    { kind: "class", attrPrefix: "data-gowdk-binding-class-", valuePrefix: "data-gowdk-class-" },
    { kind: "style", attrPrefix: "data-gowdk-binding-style-", valuePrefix: "data-gowdk-style-", unitPrefix: "data-gowdk-style-unit-" },
    { kind: "attr", attrPrefix: "data-gowdk-binding-attr-", valuePrefix: "data-gowdk-attr-" }
  ]);
  const registry = window.__gowdkIslandRegistry || (window.__gowdkIslandRegistry = { components: Object.create(null), roots: new WeakMap() });
  window.__gowdkMountIslands = () => {
    Object.keys(registry.components).forEach((name) => registry.components[name](document));
  };
  window.__gowdkDestroyIslands = (scope, includeRoot) => {
    scope = scope || document;
    const roots = [];
    if (includeRoot && scope.matches && scope.matches("gowdk-island")) roots.push(scope);
    if (scope.querySelectorAll) scope.querySelectorAll("gowdk-island").forEach((root) => roots.push(root));
    roots.forEach((root) => {
      const destroy = registry.roots.get(root);
      if (destroy) destroy();
    });
  };

  function callHelper(name, args, state, helpers, stack) {
    const helper = helpers && helpers[name];
    if (!helper) return null;
    stack = stack || [];
    if (stack.indexOf(name) >= 0) throw new Error("recursive GOWDK helper " + name);
    const nextScope = Object.create(null);
    (helper.params || []).forEach((param, index) => {
      nextScope[param] = args[index];
    });
    return valueOf(helper.return || "", state, nextScope, helpers, stack.concat([name]));
  }

  const builtins = Object.freeze({
    len(value) {
      if (value == null) return 0;
      if (typeof value === "string" || Array.isArray(value)) return value.length;
      return 0;
    },
    string(value) {
      if (value == null) return "";
      return String(value);
    },
    lower(value) {
      return String(value == null ? "" : value).toLowerCase();
    },
    upper(value) {
      return String(value == null ? "" : value).toUpperCase();
    },
    contains(value, query) {
      return String(value == null ? "" : value).includes(String(query == null ? "" : query));
    },
    int(value) {
      const next = Number.parseInt(value, 10);
      return Number.isNaN(next) ? 0 : next;
    },
    float(value) {
      const next = Number.parseFloat(value);
      return Number.isNaN(next) ? 0 : next;
    }
  });

  const expressionCache = Object.create(null);

  function parseExpression(source) {
    source = String(source || "").trim();
    if (!expressionCache[source]) expressionCache[source] = expressionParser(tokenizeExpression(source)).parse();
    return expressionCache[source];
  }

  function tokenizeExpression(source) {
    const tokens = [];
    let index = 0;
    while (index < source.length) {
      const char = source[index];
      if (/\s/.test(char)) {
        index++;
        continue;
      }
      if (/[A-Za-z_]/.test(char)) {
        const start = index++;
        while (index < source.length && /[A-Za-z0-9_]/.test(source[index])) index++;
        tokens.push({ kind: "ident", value: source.slice(start, index) });
        continue;
      }
      if (/[0-9]/.test(char)) {
        const start = index++;
        while (index < source.length && /[0-9]/.test(source[index])) index++;
        if (source[index] === ".") {
          index++;
          while (index < source.length && /[0-9]/.test(source[index])) index++;
        }
        tokens.push({ kind: "number", value: source.slice(start, index) });
        continue;
      }
      if (char === "\"") {
        const start = index++;
        let escaped = false;
        while (index < source.length) {
          const next = source[index++];
          if (escaped) {
            escaped = false;
            continue;
          }
          if (next === "\\") escaped = true;
          else if (next === "\"") break;
        }
        tokens.push({ kind: "string", value: source.slice(start, index) });
        continue;
      }
      const pair = source.slice(index, index + 2);
      if (["==", "!=", "<=", ">=", "&&", "||"].indexOf(pair) >= 0) {
        tokens.push({ kind: "op", value: pair });
        index += 2;
        continue;
      }
      if ("+-*/%%!<>".indexOf(char) >= 0) {
        tokens.push({ kind: "op", value: char });
        index++;
        continue;
      }
      if ("()[]{}.,:".indexOf(char) >= 0) {
        tokens.push({ kind: char, value: char });
        index++;
        continue;
      }
      throw new Error("unsupported GOWDK expression token " + char);
    }
    tokens.push({ kind: "eof", value: "" });
    return tokens;
  }

  function expressionParser(tokens) {
    let index = 0;
    const parser = {
      parse() {
        const expr = this.parseConditional();
        this.expect("eof");
        return expr;
      },
      peek() {
        return tokens[index] || { kind: "eof", value: "" };
      },
      match(kind, value) {
        const token = this.peek();
        if (token.kind !== kind || (value != null && token.value !== value)) return false;
        index++;
        return true;
      },
      expect(kind, value) {
        const token = this.peek();
        if (!this.match(kind, value)) throw new Error("expected " + (value || kind) + " in GOWDK expression");
        return token;
      },
      parseConditional() {
        if (this.match("ident", "if")) {
          const cond = this.parseOr();
          this.expect("{");
          const thenExpr = this.parseConditional();
          this.expect("}");
          this.expect("ident", "else");
          this.expect("{");
          const elseExpr = this.parseConditional();
          this.expect("}");
          return { kind: "if", cond, thenExpr, elseExpr };
        }
        return this.parseOr();
      },
      parseOr() {
        let expr = this.parseAnd();
        while (this.match("op", "||")) expr = { kind: "binary", op: "||", left: expr, right: this.parseAnd() };
        return expr;
      },
      parseAnd() {
        let expr = this.parseEquality();
        while (this.match("op", "&&")) expr = { kind: "binary", op: "&&", left: expr, right: this.parseEquality() };
        return expr;
      },
      parseEquality() {
        let expr = this.parseCompare();
        while (this.peek().kind === "op" && (this.peek().value === "==" || this.peek().value === "!=")) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseCompare() };
        }
        return expr;
      },
      parseCompare() {
        let expr = this.parseTerm();
        while (this.peek().kind === "op" && ["<", "<=", ">", ">="].indexOf(this.peek().value) >= 0) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseTerm() };
        }
        return expr;
      },
      parseTerm() {
        let expr = this.parseFactor();
        while (this.peek().kind === "op" && (this.peek().value === "+" || this.peek().value === "-")) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseFactor() };
        }
        return expr;
      },
      parseFactor() {
        let expr = this.parseUnary();
        while (this.peek().kind === "op" && ["*", "/", "%%"].indexOf(this.peek().value) >= 0) {
          const op = this.expect("op").value;
          expr = { kind: "binary", op, left: expr, right: this.parseUnary() };
        }
        return expr;
      },
      parseUnary() {
        if (this.peek().kind === "op" && (this.peek().value === "!" || this.peek().value === "-")) {
          const op = this.expect("op").value;
          return { kind: "unary", op, expr: this.parseUnary() };
        }
        return this.parsePostfix();
      },
      parsePostfix() {
        let expr = this.parsePrimary();
        for (;;) {
          if (this.match(".")) {
            expr = { kind: "member", target: expr, name: this.expect("ident").value };
            continue;
          }
          if (this.match("[")) {
            const item = this.parseConditional();
            this.expect("]");
            expr = { kind: "index", target: expr, index: item };
            continue;
          }
          if (this.match("(")) {
            const args = [];
            if (!this.match(")")) {
              do {
                args.push(this.parseConditional());
              } while (this.match(","));
              this.expect(")");
            }
            if (expr.kind !== "ident") throw new Error("only named GOWDK helpers can be called");
            expr = { kind: "call", name: expr.name, args };
            continue;
          }
          return expr;
        }
      },
      parsePrimary() {
        const token = this.peek();
        if (this.match("number")) return { kind: "literal", value: Number(token.value) };
        if (this.match("string")) return { kind: "literal", value: JSON.parse(token.value) };
        if (this.match("ident", "true")) return { kind: "literal", value: true };
        if (this.match("ident", "false")) return { kind: "literal", value: false };
        if (this.match("ident", "nil") || this.match("ident", "null")) return { kind: "literal", value: null };
        if (this.match("ident")) return { kind: "ident", name: token.value };
        if (this.match("(")) {
          const expr = this.parseConditional();
          this.expect(")");
          return expr;
        }
        if (this.match("{")) {
          const fields = [];
          if (!this.match("}")) {
            do {
              const name = this.expect("ident").value;
              this.expect(":");
              fields.push([name, this.parseConditional()]);
            } while (this.match(","));
            this.expect("}");
          }
          return { kind: "object", fields };
        }
        throw new Error("unsupported GOWDK expression");
      }
    };
    return parser;
  }

  async function fetchJSON(url, signal) {
    const response = await fetch(String(url), { headers: { "Accept": "application/json" }, signal });
    if (!response.ok) throw new Error("GOWDK fetchJSON failed with HTTP " + response.status);
    const contentType = response.headers.get("content-type") || "";
    if (!/\bapplication\/json\b|\+json\b/i.test(contentType)) {
      throw new Error("GOWDK fetchJSON expected JSON response");
    }
    const text = await response.text();
    if (text.trim() === "") return null;
    try {
      return JSON.parse(text);
    } catch (_error) {
      throw new Error("GOWDK fetchJSON received invalid JSON");
    }
  }

  function clearAsyncError(state) {
    if (Object.prototype.hasOwnProperty.call(state, "Error")) state.Error = "";
  }

  function recordAsyncError(state, error) {
    if (Object.prototype.hasOwnProperty.call(state, "Error")) {
      state.Error = error && error.message ? error.message : String(error || "async error");
    }
    if (Object.prototype.hasOwnProperty.call(state, "Loading")) {
      state.Loading = false;
    }
  }

  function valueOf(token, state, scope, helpers, stack) {
    return evalExpression(parseExpression(token), state, scope || null, helpers || {}, stack || []);
  }

  function evalExpression(expr, state, scope, helpers, stack) {
    switch (expr.kind) {
      case "literal":
        return expr.value;
      case "ident":
        if (scope && Object.prototype.hasOwnProperty.call(scope, expr.name)) return scope[expr.name];
        if (Object.prototype.hasOwnProperty.call(state, expr.name)) return state[expr.name];
        return undefined;
      case "member": {
        const target = evalExpression(expr.target, state, scope, helpers, stack);
        return target == null ? undefined : target[expr.name];
      }
      case "index": {
        const target = evalExpression(expr.target, state, scope, helpers, stack);
        const index = evalExpression(expr.index, state, scope, helpers, stack);
        return target == null ? undefined : target[index];
      }
      case "object": {
        const out = {};
        expr.fields.forEach((field) => {
          out[field[0]] = evalExpression(field[1], state, scope, helpers, stack);
        });
        return out;
      }
      case "call": {
        const args = expr.args.map((arg) => evalExpression(arg, state, scope, helpers, stack));
        if (Object.prototype.hasOwnProperty.call(builtins, expr.name)) return builtins[expr.name].apply(null, args);
        return callHelper(expr.name, args, state, helpers, stack);
      }
      case "unary": {
        const value = evalExpression(expr.expr, state, scope, helpers, stack);
        if (expr.op === "!") return !Boolean(value);
        if (expr.op === "-") return -Number(value);
        return undefined;
      }
      case "binary":
        return evalBinaryExpression(expr, state, scope, helpers, stack);
      case "if":
        return Boolean(evalExpression(expr.cond, state, scope, helpers, stack))
          ? evalExpression(expr.thenExpr, state, scope, helpers, stack)
          : evalExpression(expr.elseExpr, state, scope, helpers, stack);
      default:
        return undefined;
    }
  }

  function evalBinaryExpression(expr, state, scope, helpers, stack) {
    if (expr.op === "&&") return Boolean(evalExpression(expr.left, state, scope, helpers, stack)) && Boolean(evalExpression(expr.right, state, scope, helpers, stack));
    if (expr.op === "||") return Boolean(evalExpression(expr.left, state, scope, helpers, stack)) || Boolean(evalExpression(expr.right, state, scope, helpers, stack));
    const left = evalExpression(expr.left, state, scope, helpers, stack);
    const right = evalExpression(expr.right, state, scope, helpers, stack);
    switch (expr.op) {
      case "==":
        return left === right;
      case "!=":
        return left !== right;
      case "<":
        return left < right;
      case "<=":
        return left <= right;
      case ">":
        return left > right;
      case ">=":
        return left >= right;
      case "+":
        return left + right;
      case "-":
        return Number(left) - Number(right);
      case "*":
        return Number(left) * Number(right);
      case "/":
        return Number(left) / Number(right);
      case "%%":
        return Number(left) %% Number(right);
      default:
        return undefined;
    }
  }

  function recomputeComputed(state, computeds, helpers) {
    (computeds || []).forEach((computed) => {
      state[computed.name] = valueOf(computed.expr, state, null, helpers);
    });
  }

  function splitArgs(source) {
    source = source.trim();
    if (!source) return [];
    const args = [];
    let start = 0;
    let depth = 0;
    let inString = false;
    let escaped = false;
    for (let i = 0; i < source.length; i++) {
      const char = source[i];
      if (escaped) {
        escaped = false;
        continue;
      }
      if (inString) {
        if (char === "\\") escaped = true;
        else if (char === "\"") inString = false;
        continue;
      }
      if (char === "\"") inString = true;
      else if (char === "(" || char === "[" || char === "{") depth++;
      else if (char === ")" || char === "]" || char === "}") depth--;
      else if (char === ",") {
        if (depth > 0) continue;
        args.push(source.slice(start, i).trim());
        start = i + 1;
      }
    }
    args.push(source.slice(start).trim());
    return args;
  }

  function emitComponentEvent(root, emitEvents, name, args, state, scope, helpers) {
    const event = emitEvents && emitEvents[name];
    if (!event) return;
    const payload = Object.create(null);
    (event.params || []).forEach((param, index) => {
      payload[param] = valueOf(args[index] || "", state, scope, helpers);
    });
    root.dispatchEvent(new CustomEvent(name, { detail: payload, bubbles: true }));
  }

  async function applyExpression(expr, state, handlers, helpers, scope, refs, computeds, asyncTokens, root, emitEvents) {
    expr = expr.trim();
    let local = expr.match(/^let\s+([A-Za-z_][A-Za-z0-9_]*)\s+[A-Za-z_][A-Za-z0-9_]*\s*=\s*(.+)$/);
    if (local) {
      if (!scope) return;
      scope[local[1]] = valueOf(local[2], state, scope, helpers);
      return;
    }
    let awaitedFetch = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*await\s+fetchJSON\[(.*)\]\((.*)\)$/);
    if (awaitedFetch) {
      const target = awaitedFetch[1];
      const token = (asyncTokens[target] || 0) + 1;
      const controllers = asyncTokens.__controllers || (asyncTokens.__controllers = Object.create(null));
      if (controllers[target] && typeof controllers[target].abort === "function") controllers[target].abort();
      const controller = typeof AbortController === "undefined" ? null : new AbortController();
      controllers[target] = controller;
      asyncTokens[target] = token;
      clearAsyncError(state);
      let next;
      try {
        next = await fetchJSON(valueOf(awaitedFetch[3], state, scope, helpers), controller ? controller.signal : undefined);
      } catch (error) {
        if (asyncTokens[target] !== token || (error && error.name === "AbortError")) throw staleAsyncResult;
        delete asyncTokens[target];
        if (controllers[target] === controller) delete controllers[target];
        throw error;
      }
      if (asyncTokens[target] !== token) throw staleAsyncResult;
      state[target] = next;
      delete asyncTokens[target];
      if (controllers[target] === controller) delete controllers[target];
      return;
    }
    let call = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\((.*)\)$/);
    if (call) {
      if (call[1] === "append" || call[1] === "remove" || call[1] === "move") {
        const args = splitArgs(call[2]);
        const field = (args[0] || "").trim();
        if (!Array.isArray(state[field])) return;
        if (call[1] === "append" && args.length === 2) {
          state[field] = state[field].concat([valueOf(args[1], state, scope, helpers)]);
          return;
        }
        if (call[1] === "remove" && args.length === 2) {
          const index = Number(valueOf(args[1], state, scope, helpers));
          if (!Number.isInteger(index) || index < 0 || index >= state[field].length) return;
          state[field] = state[field].slice(0, index).concat(state[field].slice(index + 1));
          return;
        }
        if (call[1] === "move" && args.length === 3) {
          const from = Number(valueOf(args[1], state, scope, helpers));
          const to = Number(valueOf(args[2], state, scope, helpers));
          if (!Number.isInteger(from) || !Number.isInteger(to) || from < 0 || from >= state[field].length || to < 0 || to >= state[field].length || from === to) return;
          const next = state[field].slice();
          const item = next.splice(from, 1)[0];
          next.splice(to, 0, item);
          state[field] = next;
          return;
        }
        return;
      }
      const handler = handlers[call[1]];
      if (!handler) return;
      const params = handler.params || [];
      const args = splitArgs(call[2]);
      const nextScope = Object.create(null);
      params.forEach((param, index) => {
        nextScope[param] = valueOf(args[index] || "", state, scope, helpers);
      });
      for (const statement of (handler.statements || handler)) {
        await applyExpression(statement, state, handlers, helpers, nextScope, refs, computeds, asyncTokens, root, emitEvents);
        recomputeComputed(state, computeds, helpers);
      }
      return;
    }
    let emit = expr.match(/^emit\s+([A-Za-z_][A-Za-z0-9_]*)\((.*)\)$/);
    if (emit) {
      emitComponentEvent(root, emitEvents, emit[1], splitArgs(emit[2]), state, scope, helpers);
      return;
    }
    let refCall = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\.(Focus|Blur|ScrollIntoView)\(\)$/);
    if (refCall) {
      const node = refs && refs[refCall[1]];
      if (!node) return;
      if (refCall[2] === "Focus" && typeof node.focus === "function") node.focus();
      else if (refCall[2] === "Blur" && typeof node.blur === "function") node.blur();
      else if (refCall[2] === "ScrollIntoView" && typeof node.scrollIntoView === "function") node.scrollIntoView();
      return;
    }
    let match = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)(\+\+|--)$/);
    if (match) {
      const current = Number(state[match[1]] || 0);
      state[match[1]] = match[2] === "++" ? current + 1 : current - 1;
      return;
    }
    match = expr.match(/^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*(.+)$/);
    if (match) {
      const target = match[1];
      const value = match[2].trim();
      const toggle = value.match(/^!\s*([A-Za-z_][A-Za-z0-9_]*)$/);
      state[target] = toggle ? !Boolean(valueOf(toggle[1], state, scope, helpers)) : valueOf(value, state, scope, helpers);
    }
  }

  async function applyStatements(statements, state, handlers, helpers, scope, refs, computeds, asyncTokens, root, emitEvents) {
    for (const statement of (statements || [])) {
      await applyExpression(statement, state, handlers, helpers, scope || null, refs, computeds, asyncTokens, root, emitEvents);
      recomputeComputed(state, computeds, helpers);
    }
  }

  function escapeHTML(value) {
    return String(value).replace(/[&<>"']/g, (char) => {
      if (char === "&") return "&amp;";
      if (char === "<") return "&lt;";
      if (char === ">") return "&gt;";
      if (char === "\"") return "&quot;";
      return "&#39;";
    });
  }

  function interpolateTemplate(template, state, scope, helpers) {
    return template.replace(/\{\{([^{}]+)\}\}/g, (_match, expr) => {
      const value = valueOf(expr, state, scope, helpers);
      return value == null ? "" : escapeHTML(value);
    });
  }

  function firstTemplateElement(html) {
    const holder = document.createElement("template");
    holder.innerHTML = html.trim();
    return holder.content.firstElementChild;
  }

  function syncElement(target, source) {
    Array.from(target.attributes).forEach((attr) => {
      if (!source.hasAttribute(attr.name)) target.removeAttribute(attr.name);
    });
    Array.from(source.attributes).forEach((attr) => {
      if (target.getAttribute(attr.name) !== attr.value) target.setAttribute(attr.name, attr.value);
    });
    if (target.innerHTML !== source.innerHTML) target.innerHTML = source.innerHTML;
  }

  function matchingNodes(container, selector) {
    const nodes = [];
    if (container.matches && container.matches(selector)) nodes.push(container);
    container.querySelectorAll(selector).forEach((node) => nodes.push(node));
    return nodes;
  }

  function emptyBindings() {
    return { text: [], value: [], checked: [], classes: [], styles: [], attrs: [], conditionals: [], lists: [] };
  }

  function collectDirectBinding(bindings, spec, root) {
    matchingNodes(root, spec.selector).forEach((node) => {
      if (!ownsNode(root, node)) return;
      const id = node.getAttribute(spec.id);
      if (spec.kind === "text") bindings.text.push({ id, node, field: node.getAttribute(spec.field) });
      else if (spec.kind === "value") bindings.value.push({ id, node, field: node.getAttribute(spec.field) });
      else if (spec.kind === "checked") bindings.checked.push({ id, node, field: node.getAttribute(spec.field) });
      else if (spec.kind === "conditional") bindings.conditionals.push({ id, node });
      else if (spec.kind === "list") bindings.lists.push({ id, node });
    });
  }

  function collectPrefixBinding(bindings, spec, node, attr) {
    if (!attr.name.startsWith(spec.attrPrefix)) return;
    const name = attr.name.slice(spec.attrPrefix.length);
    if (spec.kind === "class") {
      bindings.classes.push({ id: attr.value, node, name, expression: node.getAttribute(spec.valuePrefix + name) });
    } else if (spec.kind === "style") {
      bindings.styles.push({ id: attr.value, node, name, expression: node.getAttribute(spec.valuePrefix + name), unit: node.getAttribute(spec.unitPrefix + name) || "" });
    } else if (spec.kind === "attr") {
      bindings.attrs.push({ id: attr.value, node, name, expression: node.getAttribute(spec.valuePrefix + name) });
    }
  }

  function collectBindings(root) {
    const bindings = emptyBindings();
    bindingTable.filter((spec) => spec.selector).forEach((spec) => collectDirectBinding(bindings, spec, root));
    const prefixSpecs = bindingTable.filter((spec) => spec.attrPrefix);
    root.querySelectorAll("*").forEach((node) => {
      if (!ownsNode(root, node)) return;
      Array.from(node.attributes).forEach((attr) => {
        prefixSpecs.forEach((spec) => collectPrefixBinding(bindings, spec, node, attr));
      });
    });
    return bindings;
  }

  function conditionalRecords(root, options) {
    options = options || {};
    const records = root.__gowdkConditionalRecords || (root.__gowdkConditionalRecords = new Map());
    const shouldSkip = (node) => {
      if (options.owner && !ownsNode(options.owner, node)) return true;
      if (options.skipLoopItems && node.closest("[data-gowdk-for-item]")) return true;
      return false;
    };
    matchingNodes(root, "[data-gowdk-binding-if]").forEach((node) => {
      if (shouldSkip(node)) return;
      const id = node.getAttribute("data-gowdk-binding-if");
      if (!id || records.has(id)) return;
      const marker = document.createComment("gowdk-if:" + id);
      const template = node.cloneNode(true);
      node.parentNode.insertBefore(marker, node);
      records.set(id, {
        id,
        marker,
        template,
        current: node,
        group: node.getAttribute("data-gowdk-if-group") || "",
        index: Number(node.getAttribute("data-gowdk-if-index") || "0")
      });
    });
    return Array.from(records.values()).filter((record) => record.marker && record.marker.isConnected);
  }

  function mountConditional(record) {
    if (record.current && record.current.isConnected) return record.current;
    const node = record.template.cloneNode(true);
    node.hidden = false;
    record.marker.parentNode.insertBefore(node, record.marker.nextSibling);
    record.current = node;
    return node;
  }

  function unmountConditional(record) {
    if (!record.current || !record.current.isConnected) return;
    record.current.parentNode.removeChild(record.current);
    record.current = null;
  }

  function renderConditionals(container, state, scope, helpers, options) {
    options = options || {};
    const records = conditionalRecords(container, options);
    records.forEach((record) => {
      if (record.group) return;
      const condition = record.template.getAttribute("data-gowdk-if");
      const visible = condition == null || Boolean(valueOf(condition, state, scope, helpers));
      if (visible) mountConditional(record);
      else unmountConditional(record);
    });
    const conditionalGroups = new Map();
    records.forEach((record) => {
      if (!record.group) return;
      if (!conditionalGroups.has(record.group)) conditionalGroups.set(record.group, []);
      conditionalGroups.get(record.group).push(record);
    });
    conditionalGroups.forEach((groupRecords) => {
      groupRecords.sort((left, right) => left.index - right.index);
      let matched = false;
      groupRecords.forEach((record) => {
        const condition = record.template.getAttribute("data-gowdk-if");
        const visible = !matched && (condition == null || Boolean(valueOf(condition, state, scope, helpers)));
        if (visible) mountConditional(record);
        else unmountConditional(record);
        if (visible) matched = true;
      });
    });
  }

  function renderListLoops(root, state, helpers, bindings) {
    const markers = bindings ? bindings.lists.map((binding) => binding.node).filter((node) => node.isConnected) : Array.from(root.querySelectorAll("template[data-gowdk-for]"));
    markers.forEach((marker) => {
      if (!ownsNode(root, marker)) return;
      const group = marker.getAttribute("data-gowdk-for");
      const itemName = marker.getAttribute("data-gowdk-for-var");
      const indexName = marker.getAttribute("data-gowdk-for-index-var");
      const source = marker.getAttribute("data-gowdk-for-source");
      const keyExpr = marker.getAttribute("data-gowdk-for-key");
      const template = marker.getAttribute("data-gowdk-for-template") || "";
      const items = valueOf(source, state, null, helpers);
      const existing = new Map();
      let cursor = marker.nextSibling;
      while (cursor) {
        const next = cursor.nextSibling;
        if (cursor.nodeType !== 1 || cursor.getAttribute("data-gowdk-for-item") !== group) break;
        existing.set(cursor.getAttribute("data-gowdk-key-value") || "", cursor);
        cursor = next;
      }
      if (!Array.isArray(items)) return;
      const fragment = document.createDocumentFragment();
      const used = new Set();
      items.forEach((item, index) => {
        const scope = Object.create(null);
        scope[itemName] = item;
        scope.index = index;
        if (indexName) scope[indexName] = index;
        const key = String(valueOf(keyExpr, state, scope, helpers) ?? "");
        const fresh = firstTemplateElement(interpolateTemplate(template, state, scope, helpers));
        if (!fresh) return;
        renderConditionals(fresh, state, scope, helpers);
        const reused = existing.get(key);
        if (reused && !used.has(key)) {
          syncElement(reused, fresh);
          fragment.appendChild(reused);
          used.add(key);
          return;
        }
        fragment.appendChild(fresh);
        used.add(key);
      });
      marker.parentNode.insertBefore(fragment, marker.nextSibling);
      existing.forEach((node, key) => {
        if (!used.has(key) && node.parentNode) node.parentNode.removeChild(node);
      });
    });
  }

  function eventModifiers(source) {
    const modifiers = { prevent: false, stop: false, once: false, capture: false, debounce: 0, throttle: 0 };
    (source || "").split(/\s+/).filter(Boolean).forEach((item) => {
      if (item === "prevent") modifiers.prevent = true;
      else if (item === "stop") modifiers.stop = true;
      else if (item === "once") modifiers.once = true;
      else if (item === "capture") modifiers.capture = true;
      else if (item.startsWith("debounce:")) modifiers.debounce = Number(item.slice("debounce:".length)) || 0;
      else if (item.startsWith("throttle:")) modifiers.throttle = Number(item.slice("throttle:".length)) || 0;
    });
    return modifiers;
  }

  function domEventScope(domEvent) {
    const target = domEvent && domEvent.target ? domEvent.target : {};
    return {
      event: {
        value: target.value == null ? "" : String(target.value),
        checked: Boolean(target.checked),
        key: domEvent && domEvent.key ? String(domEvent.key) : "",
        code: domEvent && domEvent.code ? String(domEvent.code) : "",
        clientX: domEvent && typeof domEvent.clientX === "number" ? domEvent.clientX : 0,
        clientY: domEvent && typeof domEvent.clientY === "number" ? domEvent.clientY : 0
      }
    };
  }

  function ownsNode(root, node) {
    return node.closest("gowdk-island") === root;
  }

  function syncChildProps(root, state, helpers) {
    root.querySelectorAll("gowdk-island[data-gowdk-props]").forEach((node) => {
      const props = JSON.parse(node.getAttribute("data-gowdk-props") || "{}");
      const payload = Object.create(null);
      Object.keys(props).forEach((name) => {
        payload[name] = valueOf(props[name], state, null, helpers);
      });
      node.dispatchEvent(new CustomEvent("gowdk:props", { detail: payload }));
    });
  }

  function updateTextBindings(bindings, state) {
    bindings.text.forEach(({ node, field }) => {
      node.textContent = state[field] == null ? "" : String(state[field]);
    });
  }

  function updateValueBindings(bindings, state) {
    bindings.value.forEach(({ node, field }) => {
      if (node.type === "radio") {
        node.checked = String(state[field] == null ? "" : state[field]) === node.value;
        return;
      }
      const value = state[field] == null ? "" : String(state[field]);
      if (document.activeElement !== node && node.value !== value) node.value = value;
    });
  }

  function updateCheckedBindings(bindings, state) {
    bindings.checked.forEach(({ node, field }) => {
      const checked = Boolean(state[field]);
      if (node.checked !== checked) node.checked = checked;
    });
  }

  function updateClassBindings(bindings, state, helpers) {
    bindings.classes.forEach(({ node, name, expression }) => {
      node.classList.toggle(name, Boolean(valueOf(expression, state, null, helpers)));
    });
  }

  function updateStyleBindings(bindings, state, helpers) {
    bindings.styles.forEach(({ node, name, expression, unit }) => {
      const value = valueOf(expression, state, null, helpers);
      if (value == null || value === false || value === "") node.style.removeProperty(name);
      else node.style.setProperty(name, String(value) + unit);
    });
  }

  function updateAttrBindings(bindings, state, helpers) {
    bindings.attrs.forEach(({ node, name, expression }) => {
      const value = valueOf(expression, state, null, helpers);
      if (booleanAttrs.has(name)) {
        if (Boolean(value)) node.setAttribute(name, "");
        else node.removeAttribute(name);
        return;
      }
      if (value == null || value === false) node.removeAttribute(name);
      else node.setAttribute(name, String(value));
    });
  }

  function updateBindings(root, state, helpers, bindings) {
    updateTextBindings(bindings, state);
    updateValueBindings(bindings, state);
    updateCheckedBindings(bindings, state);
    updateClassBindings(bindings, state, helpers);
    updateStyleBindings(bindings, state, helpers);
    updateAttrBindings(bindings, state, helpers);
  }

  function render(root, state, helpers, bindings) {
    renderListLoops(root, state, helpers, bindings);
    bindings = collectBindings(root);
    renderConditionals(root, state, null, helpers, { owner: root, skipLoopItems: true });
    bindings = collectBindings(root);
    updateBindings(root, state, helpers, bindings);
    root.setAttribute("data-gowdk-state", JSON.stringify(state));
    return bindings;
  }

  async function %s(scope) {
    scope = scope || document;
    scope.querySelectorAll(selector).forEach(async (root) => {
    if (root.getAttribute("data-gowdk-mounted") === "js") return;
    root.setAttribute("data-gowdk-mounted", "js");
    const state = JSON.parse(root.getAttribute("data-gowdk-state") || "{}");
    const client = JSON.parse(root.getAttribute("data-gowdk-client") || "{}");
    const hasEnvelope = Boolean(client.handlers || client.helpers || client.emits || client.stores || client.mount || client.destroy || client.effects || client.computed);
    const handlers = hasEnvelope ? (client.handlers || {}) : client;
    const helpers = client.helpers || {};
    const emitEvents = client.emits || {};
    const storeNames = Array.isArray(client.stores) ? client.stores : [];
    const storeRegistry = window.__gowdkStores;
    const mountStatements = client.mount || [];
    const destroyStatements = client.destroy || [];
    const effects = client.effects || [];
    const computeds = client.computed || [];
    const refs = Object.create(null);
    const asyncTokens = Object.create(null);
    storeNames.forEach((name) => {
      if (storeRegistry) Object.assign(state, storeRegistry.get(name));
    });
    recomputeComputed(state, computeds, helpers);
    root.querySelectorAll("[data-gowdk-ref]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      refs[node.getAttribute("data-gowdk-ref")] = node;
    });
    const effectValues = Object.create(null);
    const effectCleanups = Object.create(null);
    effects.forEach((effect) => {
      effectValues[effect.field] = state[effect.field];
    });
    let bindings = collectBindings(root);
    const runEffectCleanup = async (effect) => {
      const cleanup = effectCleanups[effect.field];
      if (!cleanup || cleanup.length === 0) return;
      effectCleanups[effect.field] = null;
      await applyStatements(cleanup, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
    };
    const runAllEffectCleanups = async () => {
      for (const effect of effects) {
        await runEffectCleanup(effect);
      }
    };
    const settleEffects = async () => {
      for (let pass = 0; pass < 10; pass++) {
        let ran = false;
        for (const effect of effects) {
          const current = state[effect.field];
          if (Object.is(effectValues[effect.field], current)) continue;
          await runEffectCleanup(effect);
          effectValues[effect.field] = current;
          await applyStatements(effect.statements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
          effectCleanups[effect.field] = effect.cleanup || null;
          ran = true;
        }
        if (!ran) return;
      }
    };
    let applyingStoreUpdate = false;
    const publishStores = () => {
      if (applyingStoreUpdate || !storeRegistry || storeNames.length === 0) return;
      storeNames.forEach((name) => storeRegistry.set(name, state));
    };
    const rerender = () => {
      bindings = render(root, state, helpers, bindings);
      bindInteractiveNodes();
      syncChildProps(root, state, helpers);
      publishStores();
    };
    let renderScheduled = false;
    const scheduleRender = () => {
      if (renderScheduled) return;
      renderScheduled = true;
      const flush = () => {
        renderScheduled = false;
        rerender();
      };
      if (typeof queueMicrotask === "function") queueMicrotask(flush);
      else Promise.resolve().then(flush);
    };
    const storeUnsubscribers = storeNames.map((name) => {
      if (!storeRegistry) return () => {};
      return storeRegistry.subscribe(name, async (next) => {
        applyingStoreUpdate = true;
        try {
          Object.assign(state, next || {});
          recomputeComputed(state, computeds, helpers);
          await settleEffects();
          recomputeComputed(state, computeds, helpers);
          rerender();
        } finally {
          applyingStoreUpdate = false;
        }
      });
    });
    const bindInteractiveNodes = () => {
      root.querySelectorAll("*").forEach((node) => {
        const owned = ownsNode(root, node);
        if (owned && node.hasAttribute("data-gowdk-bind-value") && !node.hasAttribute("data-gowdk-bound-value")) {
          node.setAttribute("data-gowdk-bound-value", "");
          const field = node.getAttribute("data-gowdk-bind-value");
          const type = node.getAttribute("data-gowdk-bind-type") || "string";
          const event = node.tagName === "SELECT" || node.type === "radio" ? "change" : "input";
          node.addEventListener(event, async () => {
            if (node.type === "radio") {
              if (!node.checked) return;
              state[field] = node.value;
            } else if (type === "int") {
              const next = parseInt(node.value, 10);
              state[field] = Number.isNaN(next) ? 0 : next;
            } else if (type === "float") {
              const next = parseFloat(node.value);
              state[field] = Number.isNaN(next) ? 0 : next;
            } else {
              state[field] = node.value;
            }
            recomputeComputed(state, computeds, helpers);
            await settleEffects();
            recomputeComputed(state, computeds, helpers);
            scheduleRender();
          });
        }
        if (owned && node.hasAttribute("data-gowdk-bind-checked") && !node.hasAttribute("data-gowdk-bound-checked")) {
          node.setAttribute("data-gowdk-bound-checked", "");
          const field = node.getAttribute("data-gowdk-bind-checked");
          node.addEventListener("change", async () => {
            state[field] = node.checked;
            recomputeComputed(state, computeds, helpers);
            await settleEffects();
            recomputeComputed(state, computeds, helpers);
            scheduleRender();
          });
        }
        Array.from(node.attributes).forEach((attr) => {
          if (attr.name.startsWith("data-gowdk-parent-on-")) {
            const event = attr.name.slice("data-gowdk-parent-on-".length);
            const boundAttr = "data-gowdk-bound-parent-on-" + event;
            if (node.hasAttribute(boundAttr)) return;
            node.setAttribute(boundAttr, "");
            const modifiers = eventModifiers(node.getAttribute("data-gowdk-parent-event-" + event));
            let debounceTimer = 0;
            let throttleUntil = 0;
            const invoke = async (customEvent) => {
              const eventScope = Object.create(null);
              eventScope.event = customEvent.detail || {};
              try {
                await applyExpression(attr.value, state, handlers, helpers, eventScope, refs, computeds, asyncTokens, root, emitEvents);
              } catch (error) {
                if (error !== staleAsyncResult) recordAsyncError(state, error);
              } finally {
                recomputeComputed(state, computeds, helpers);
                await settleEffects();
                recomputeComputed(state, computeds, helpers);
                scheduleRender();
              }
            };
            const listener = (customEvent) => {
              if (modifiers.prevent) customEvent.preventDefault();
              if (modifiers.stop) customEvent.stopPropagation();
              if (modifiers.debounce > 0) {
                clearTimeout(debounceTimer);
                debounceTimer = setTimeout(() => invoke(customEvent), modifiers.debounce);
                return;
              }
              if (modifiers.throttle > 0) {
                const now = Date.now();
                if (now < throttleUntil) return;
                throttleUntil = now + modifiers.throttle;
              }
              invoke(customEvent);
            };
            node.addEventListener(event, listener, { once: modifiers.once, capture: modifiers.capture });
            return;
          }
          if (!owned) return;
          if (!attr.name.startsWith("data-gowdk-on-")) return;
          const event = attr.name.slice("data-gowdk-on-".length);
          const boundAttr = "data-gowdk-bound-on-" + event;
          if (node.hasAttribute(boundAttr)) return;
          node.setAttribute(boundAttr, "");
          const modifiers = eventModifiers(node.getAttribute("data-gowdk-event-" + event));
          let debounceTimer = 0;
          let throttleUntil = 0;
          const invoke = async (domEvent) => {
            try {
              await applyExpression(attr.value, state, handlers, helpers, domEventScope(domEvent), refs, computeds, asyncTokens, root, emitEvents);
            } catch (error) {
              if (error !== staleAsyncResult) recordAsyncError(state, error);
            } finally {
              recomputeComputed(state, computeds, helpers);
              await settleEffects();
              recomputeComputed(state, computeds, helpers);
              scheduleRender();
            }
          };
          const listener = (domEvent) => {
            if (modifiers.prevent) domEvent.preventDefault();
            if (modifiers.stop) domEvent.stopPropagation();
            if (modifiers.debounce > 0) {
              clearTimeout(debounceTimer);
              debounceTimer = setTimeout(() => invoke(domEvent), modifiers.debounce);
              return;
            }
            if (modifiers.throttle > 0) {
              const now = Date.now();
              if (now < throttleUntil) return;
              throttleUntil = now + modifiers.throttle;
            }
            invoke(domEvent);
          };
          node.addEventListener(event, listener, { once: modifiers.once, capture: modifiers.capture });
        });
      });
    };
    root.addEventListener("gowdk:props", async (event) => {
      Object.assign(state, event.detail || {});
      recomputeComputed(state, computeds, helpers);
      await settleEffects();
      recomputeComputed(state, computeds, helpers);
      scheduleRender();
    });
    await applyStatements(mountStatements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
    await settleEffects();
    recomputeComputed(state, computeds, helpers);
    const destroyIsland = async function %s() {
      if (root.getAttribute("data-gowdk-mounted") !== "js") return;
      root.removeAttribute("data-gowdk-mounted");
      registry.roots.delete(root);
      storeUnsubscribers.forEach((unsubscribe) => unsubscribe());
      if (destroyStatements.length > 0) {
        await runAllEffectCleanups();
        await applyStatements(destroyStatements, state, handlers, helpers, null, refs, computeds, asyncTokens, root, emitEvents);
      } else if (effects.length > 0) {
        await runAllEffectCleanups();
      }
    };
    registry.roots.set(root, destroyIsland);
    window.addEventListener("pagehide", destroyIsland, { once: true });
    rerender();
  });
  }

  registry.components[component] = %s;
  %s(document);
})();
`, component, mountFunction, destroyFunction, mountFunction, mountFunction)
	if includeSourceMap {
		source += "//# sourceMappingURL=" + path.Base(islandJSSourceMapAssetPath(componentName)) + "\n"
	}
	return source
}

type jsSourceMap struct {
	Version        int      `json:"version"`
	File           string   `json:"file"`
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent,omitempty"`
	Names          []string `json:"names"`
	Mappings       string   `json:"mappings"`
}

func islandJSSourceMap(component manifest.Component, generatedSource string) []byte {
	source := component.Source
	if source == "" {
		source = "components/" + component.Name + ".cmp.gwdk"
	}
	content := componentSourceMapContent(component)
	payload, err := json.MarshalIndent(jsSourceMap{
		Version:        3,
		File:           path.Base(islandJSAssetPath(component.Name)),
		Sources:        []string{source},
		SourcesContent: []string{content},
		Names:          []string{},
		Mappings:       sourceMapMappings(component, generatedSource),
	}, "", "  ")
	if err != nil {
		return []byte(`{"version":3,"file":"` + path.Base(islandJSAssetPath(component.Name)) + `","sources":[],"names":[],"mappings":""}`)
	}
	return append(payload, '\n')
}

type sourceMapAnchor struct {
	generatedLine int
	sourceLine    int
	sourceColumn  int
}

func sourceMapMappings(component manifest.Component, generatedSource string) string {
	anchors := sourceMapAnchors(component, generatedSource)
	if len(anchors) == 0 {
		return ""
	}
	byLine := map[int]sourceMapAnchor{}
	maxLine := 0
	for _, anchor := range anchors {
		if anchor.generatedLine <= 0 || anchor.sourceLine <= 0 || anchor.sourceColumn <= 0 {
			continue
		}
		if _, exists := byLine[anchor.generatedLine]; exists {
			continue
		}
		byLine[anchor.generatedLine] = anchor
		if anchor.generatedLine > maxLine {
			maxLine = anchor.generatedLine
		}
	}
	if maxLine == 0 {
		return ""
	}

	var builder strings.Builder
	previousGeneratedColumn := 0
	previousSourceIndex := 0
	previousSourceLine := 0
	previousSourceColumn := 0
	for line := 1; line <= maxLine; line++ {
		if line > 1 {
			builder.WriteByte(';')
			previousGeneratedColumn = 0
		}
		anchor, ok := byLine[line]
		if !ok {
			continue
		}
		sourceLine := anchor.sourceLine - 1
		sourceColumn := anchor.sourceColumn - 1
		builder.WriteString(sourceMapVLQ(0 - previousGeneratedColumn))
		builder.WriteString(sourceMapVLQ(0 - previousSourceIndex))
		builder.WriteString(sourceMapVLQ(sourceLine - previousSourceLine))
		builder.WriteString(sourceMapVLQ(sourceColumn - previousSourceColumn))
		previousGeneratedColumn = 0
		previousSourceIndex = 0
		previousSourceLine = sourceLine
		previousSourceColumn = sourceColumn
	}
	return builder.String()
}

func sourceMapAnchors(component manifest.Component, generatedSource string) []sourceMapAnchor {
	name := componentAssetName(component.Name)
	componentSpan := firstSourceSpan(component.Span, component.Blocks.Spans.Client, component.Blocks.Spans.View)
	clientSpan := firstSourceSpan(component.Blocks.Spans.Client, componentSpan)
	viewSpan := firstSourceSpan(component.Blocks.Spans.View, componentSpan)
	var anchors []sourceMapAnchor
	for index, line := range strings.Split(generatedSource, "\n") {
		lineNumber := index + 1
		switch {
		case strings.Contains(line, "const component = "):
			anchors = appendSourceMapAnchor(anchors, lineNumber, componentSpan)
		case sourceMapLineBelongsToClient(line, name):
			anchors = appendSourceMapAnchor(anchors, lineNumber, clientSpan)
		case sourceMapLineBelongsToView(line):
			anchors = appendSourceMapAnchor(anchors, lineNumber, viewSpan)
		}
	}
	return anchors
}

func sourceMapLineBelongsToClient(line, componentAssetName string) bool {
	return strings.Contains(line, "async function mount"+componentAssetName+"Island(scope)") ||
		strings.Contains(line, "async function applyExpression(") ||
		strings.Contains(line, "async function applyStatements(") ||
		strings.Contains(line, "function recomputeComputed(")
}

func sourceMapLineBelongsToView(line string) bool {
	return strings.Contains(line, "const bindingTable = Object.freeze(") ||
		strings.Contains(line, "function collectBindings(") ||
		strings.Contains(line, "function renderConditionals(") ||
		strings.Contains(line, "function renderListLoops(") ||
		strings.Contains(line, "function updateTextBindings(") ||
		strings.Contains(line, "function updateValueBindings(") ||
		strings.Contains(line, "function updateCheckedBindings(") ||
		strings.Contains(line, "function updateClassBindings(") ||
		strings.Contains(line, "function updateStyleBindings(") ||
		strings.Contains(line, "function updateAttrBindings(") ||
		strings.Contains(line, "function updateBindings(") ||
		strings.Contains(line, "function render(root, state, helpers, bindings)")
}

func appendSourceMapAnchor(anchors []sourceMapAnchor, generatedLine int, span manifest.SourceSpan) []sourceMapAnchor {
	if span.Start.Line <= 0 || span.Start.Column <= 0 {
		return anchors
	}
	return append(anchors, sourceMapAnchor{
		generatedLine: generatedLine,
		sourceLine:    span.Start.Line,
		sourceColumn:  span.Start.Column,
	})
}

func firstSourceSpan(spans ...manifest.SourceSpan) manifest.SourceSpan {
	for _, span := range spans {
		if span.Start.Line > 0 && span.Start.Column > 0 {
			return span
		}
	}
	return manifest.SourceSpan{}
}

const sourceMapBase64 = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func sourceMapVLQ(value int) string {
	vlq := value << 1
	if value < 0 {
		vlq = ((-value) << 1) + 1
	}
	var out strings.Builder
	for {
		digit := vlq & 31
		vlq >>= 5
		if vlq > 0 {
			digit |= 32
		}
		out.WriteByte(sourceMapBase64[digit])
		if vlq == 0 {
			break
		}
	}
	return out.String()
}

func componentSourceMapContent(component manifest.Component) string {
	if component.Blocks.ClientBody == "" {
		return "view {\n" + component.Blocks.ViewBody + "\n}\n"
	}
	return "client {\n" + component.Blocks.ClientBody + "\n}\n\nview {\n" + component.Blocks.ViewBody + "\n}\n"
}

func clientGoBlockWASMLoaderSource(page manifest.Page) string {
	pageID := strconv.Quote(page.ID)
	loaderPath := strconv.Quote("/" + clientGoBlockWASMLoaderAssetPath(page))
	wasmPath := strconv.Quote("/" + clientGoBlockWASMAssetPath(page))
	wasmExecPath := strconv.Quote("/" + islandWASMExecAssetPath())
	mountExport := strconv.Quote(clientGoBlockMountExportName(page))
	return fmt.Sprintf(`(() => {
  const pageID = %s;
  const loaderPath = %s;
  const wasmPath = %s;
  const wasmExecPath = %s;
  const mountExport = %s;
  const registry = window.__gowdkClientGoBlockRegistry || (window.__gowdkClientGoBlockRegistry = { entries: Object.create(null) });
  window.__gowdkMountClientGoBlocks = () => {
    Object.keys(registry.entries).forEach((key) => registry.entries[key].mount());
  };
  if (typeof WebAssembly === "undefined") return;

  function currentPageUsesScript() {
    const expected = new URL(loaderPath, window.location.href).href;
    return Array.prototype.some.call(document.querySelectorAll("script[src]"), (script) => script.src === expected);
  }

  function loadGoRuntime() {
    if (window.Go) return Promise.resolve();
    if (window.__gowdkGoWASMLoading) return window.__gowdkGoWASMLoading;
    window.__gowdkGoWASMLoading = new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = wasmExecPath;
      script.onload = resolve;
      script.onerror = () => reject(new Error("failed to load Go WASM runtime"));
      document.head.appendChild(script);
    });
    return window.__gowdkGoWASMLoading;
  }

  async function instantiate(go) {
    if (WebAssembly.instantiateStreaming) {
      try {
        return await WebAssembly.instantiateStreaming(fetch(wasmPath), go.importObject);
      } catch (_error) {}
    }
    const response = await fetch(wasmPath);
    const bytes = await response.arrayBuffer();
    return WebAssembly.instantiate(bytes, go.importObject);
  }

  loadGoRuntime().then(async () => {
    const go = new Go();
    const result = await instantiate(go);
    const exports = result.instance && result.instance.exports || {};
    if (typeof exports[mountExport] !== "function") {
      if (typeof console !== "undefined") console.error("GOWDK client go block missing export", mountExport);
      return;
    }
    const mountedBodies = new WeakSet();
    registry.entries[loaderPath] = {
      mount() {
        if (!currentPageUsesScript() || mountedBodies.has(document.body)) return;
        mountedBodies.add(document.body);
        try {
          exports[mountExport]();
        } catch (error) {
          if (typeof console !== "undefined") console.error("GOWDK client go block mount failed", pageID, error);
        }
      }
    };
    const run = go.run(result.instance);
    if (run && typeof run.catch === "function") {
      run.catch((error) => {
        if (typeof console !== "undefined") console.error("GOWDK client go block Go runtime failed", pageID, error);
      });
    }
    window.__gowdkMountClientGoBlocks();
  }).catch((error) => {
    if (typeof console !== "undefined") console.error("GOWDK client go block failed to start", pageID, error);
  });
})();
`, pageID, loaderPath, wasmPath, wasmExecPath, mountExport)
}

func islandWASMLoaderSource(componentName string) string {
	component := strconv.Quote(componentName)
	wasmPath := strconv.Quote("/" + islandWASMAssetPath(componentName))
	wasmExecPath := strconv.Quote("/" + islandWASMExecAssetPath())
	return fmt.Sprintf(`(() => {
  const component = %s;
  const wasmPath = %s;
  const wasmExecPath = %s;
  const mountExport = "GOWDKMount" + component;
  const handleExport = "GOWDKHandle" + component;
  const destroyExport = "GOWDKDestroy" + component;
  const roots = document.querySelectorAll("gowdk-island[data-gowdk-component=\"" + component + "\"][data-gowdk-runtime=\"wasm\"]");
  if (roots.length === 0 || typeof WebAssembly === "undefined") return;

  function parseJSON(value, fallback) {
    try {
      return JSON.parse(value || "");
    } catch (_error) {
      return fallback;
    }
  }

  function ownsNode(root, node) {
    return node.closest("gowdk-island") === root;
  }

  function matchingNodes(root, selector) {
    const nodes = [];
    if (root.matches && root.matches(selector)) nodes.push(root);
    root.querySelectorAll(selector).forEach((node) => nodes.push(node));
    return nodes.filter((node) => ownsNode(root, node));
  }

  function collectRefs(root) {
    const refs = Object.create(null);
    root.querySelectorAll("[data-gowdk-ref]").forEach((node) => {
      if (!ownsNode(root, node)) return;
      refs[node.getAttribute("data-gowdk-ref")] = node.getAttribute("data-gowdk-binding-ref") || "";
    });
    return refs;
  }

  function collectBindings(root) {
    const bindings = { text: [], attrs: [], classes: [], styles: [], conditionals: [], lists: [], events: [] };
    matchingNodes(root, "[data-gowdk-binding-text]").forEach((node) => {
      bindings.text.push({ id: node.getAttribute("data-gowdk-binding-text"), field: node.getAttribute("data-gowdk-bind") });
    });
    matchingNodes(root, "[data-gowdk-binding-if]").forEach((node) => {
      bindings.conditionals.push({ id: node.getAttribute("data-gowdk-binding-if"), expr: node.getAttribute("data-gowdk-if") || "" });
    });
    matchingNodes(root, "[data-gowdk-binding-list]").forEach((node) => {
      bindings.lists.push({ id: node.getAttribute("data-gowdk-binding-list"), source: node.getAttribute("data-gowdk-for-source") || "", key: node.getAttribute("data-gowdk-for-key") || "" });
    });
    root.querySelectorAll("*").forEach((node) => {
      if (!ownsNode(root, node)) return;
      Array.from(node.attributes).forEach((attr) => {
        if (attr.name.startsWith("data-gowdk-binding-on-")) {
          bindings.events.push({ id: attr.value, event: attr.name.slice("data-gowdk-binding-on-".length), expr: node.getAttribute("data-gowdk-on-" + attr.name.slice("data-gowdk-binding-on-".length)) || "" });
        } else if (attr.name.startsWith("data-gowdk-binding-attr-")) {
          const name = attr.name.slice("data-gowdk-binding-attr-".length);
          bindings.attrs.push({ id: attr.value, name, expr: node.getAttribute("data-gowdk-attr-" + name) || "" });
        } else if (attr.name.startsWith("data-gowdk-binding-class-")) {
          const name = attr.name.slice("data-gowdk-binding-class-".length);
          bindings.classes.push({ id: attr.value, name, expr: node.getAttribute("data-gowdk-class-" + name) || "" });
        } else if (attr.name.startsWith("data-gowdk-binding-style-")) {
          const name = attr.name.slice("data-gowdk-binding-style-".length);
          bindings.styles.push({ id: attr.value, name, expr: node.getAttribute("data-gowdk-style-" + name) || "", unit: node.getAttribute("data-gowdk-style-unit-" + name) || "" });
        }
      });
    });
    return bindings;
  }

  function bootstrap(root) {
    const client = parseJSON(root.getAttribute("data-gowdk-client"), {});
    return {
      component,
      state: parseJSON(root.getAttribute("data-gowdk-state"), {}),
      props: parseJSON(root.getAttribute("data-gowdk-props"), {}),
      emits: client.emits || {},
      refs: collectRefs(root),
      bindings: collectBindings(root)
    };
  }

  function targetByBinding(root, id) {
    if (!id) return null;
    const escaped = typeof CSS !== "undefined" && CSS.escape ? CSS.escape(id) : String(id).replace(/"/g, "\\\"");
    return root.querySelector("[data-gowdk-binding-text=\"" + escaped + "\"], [data-gowdk-binding-if=\"" + escaped + "\"], [data-gowdk-binding-list=\"" + escaped + "\"], [data-gowdk-binding-value=\"" + escaped + "\"], [data-gowdk-binding-checked=\"" + escaped + "\"]");
  }

  function applyPatch(root, patch) {
    if (!patch || typeof patch !== "object") return;
    const node = targetByBinding(root, patch.target || patch.binding);
    if (patch.type === "setText" && node) node.textContent = patch.value == null ? "" : String(patch.value);
    else if (patch.type === "setHidden" && node) node.hidden = Boolean(patch.value);
    else if (patch.type === "setAttr" && node && patch.name) node.setAttribute(patch.name, String(patch.value == null ? "" : patch.value));
    else if (patch.type === "removeAttr" && node && patch.name) node.removeAttribute(patch.name);
    else if (patch.type === "toggleClass" && node && patch.name) node.classList.toggle(patch.name, Boolean(patch.value));
    else if (patch.type === "setStyle" && node && patch.name) node.style.setProperty(patch.name, String(patch.value == null ? "" : patch.value));
    else if (patch.type === "replaceList" && node) node.innerHTML = Array.isArray(patch.html) ? patch.html.join("") : String(patch.html || "");
    else if (patch.type === "emit" && patch.name) root.dispatchEvent(new CustomEvent(patch.name, { detail: patch.detail || {}, bubbles: true }));
    else if (patch.type && typeof console !== "undefined") console.error("GOWDK WASM island rejected patch", patch.type, patch);
  }

  function applyPatches(root, result) {
    const patches = typeof result === "string" ? parseJSON(result, []) : result;
    if (!Array.isArray(patches)) return;
    patches.forEach((patch) => applyPatch(root, patch));
  }

  function callExport(exports, name, payload) {
    const fn = exports && exports[name];
    if (typeof fn !== "function") {
      if (typeof console !== "undefined") console.error("GOWDK WASM island missing export", name);
      return undefined;
    }
    return fn(payload);
  }

  function missingExports(exports) {
    return [mountExport, handleExport, destroyExport].filter((name) => typeof exports[name] !== "function");
  }

  function loadScript(src) {
    return new Promise((resolve, reject) => {
      const script = document.createElement("script");
      script.src = src;
      script.async = true;
      script.onload = resolve;
      script.onerror = () => reject(new Error("failed to load " + src));
      document.head.appendChild(script);
    });
  }

  async function loadGoRuntime() {
    if (typeof Go !== "function") {
      await loadScript(wasmExecPath);
    }
    if (typeof Go !== "function") return null;
    return new Go();
  }

  async function instantiateWithImports(imports) {
    if (WebAssembly.instantiateStreaming) {
      try {
        return await WebAssembly.instantiateStreaming(fetch(wasmPath), imports);
      } catch (_error) {
        // Fall through for servers that do not serve application/wasm yet.
      }
    }
    const response = await fetch(wasmPath);
    const bytes = await response.arrayBuffer();
    return WebAssembly.instantiate(bytes, imports);
  }

  async function instantiate() {
    try {
      return await instantiateWithImports({});
    } catch (directError) {
      const go = await loadGoRuntime();
      if (!go) throw directError;
      const result = await instantiateWithImports(go.importObject);
      const run = go.run(result.instance);
      if (run && typeof run.catch === "function") {
        run.catch((error) => {
          if (typeof console !== "undefined") console.error("GOWDK WASM island Go runtime failed", error);
        });
      }
      return result;
    }
  }

  instantiate().then((result) => {
    const exports = result.instance && result.instance.exports || {};
    const missing = missingExports(exports);
    if (missing.length > 0) {
      if (typeof console !== "undefined") console.error("GOWDK WASM island missing exports", missing.join(", "));
      return;
    }
    roots.forEach((root) => {
      const mountPayload = bootstrap(root);
      applyPatches(root, callExport(exports, mountExport, mountPayload));
      root.querySelectorAll("*").forEach((node) => {
        if (!ownsNode(root, node)) return;
        Array.from(node.attributes).forEach((attr) => {
          if (!attr.name.startsWith("data-gowdk-binding-on-")) return;
          const event = attr.name.slice("data-gowdk-binding-on-".length);
          node.addEventListener(event, (domEvent) => {
            applyPatches(root, callExport(exports, handleExport, {
              event,
              binding: attr.value,
              detail: { value: domEvent && domEvent.target ? domEvent.target.value : undefined }
            }));
          });
        });
      });
      window.addEventListener("pagehide", () => {
        applyPatches(root, callExport(exports, destroyExport, { component, state: parseJSON(root.getAttribute("data-gowdk-state"), {}) }));
      }, { once: true });
    });
  }).catch((error) => {
    if (typeof console !== "undefined") console.error("GOWDK WASM island failed to start", component, error);
  });
})();
`, component, wasmPath, wasmExecPath)
}

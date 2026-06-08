package buildgen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

var wasmMagic = []byte{0x00, 0x61, 0x73, 0x6d}

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
	if err := sourceFile.Close(); err != nil {
		_ = os.Remove(sourcePath)
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_build_error", fmt.Errorf("close temp source: %w", err))
	}
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		_ = os.Remove(sourcePath)
		return nil, clientGoBlockDiagnosticError(page, "client_go_block_wasm_build_error", fmt.Errorf("write temp source: %w", err))
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

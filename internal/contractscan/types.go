package contractscan

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type functionInfo struct {
	Signature *types.Signature
	Package   *types.Package
}

type typedPackageInfo struct {
	Functions map[string]functionInfo
	Types     map[string]contractTypeInfo
}

func inspectTypedPackage(fset *token.FileSet, packageDir string, files []*ast.File, inspectionCache *packageInspectionCache) typedPackageInfo {
	info := &types.Info{
		Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{},
	}
	config := types.Config{
		Importer: contractScanImporter(packageDir, fset, files, inspectionCache),
		Error:    func(error) {},
	}
	packageName := ""
	if len(files) > 0 && files[0].Name != nil {
		packageName = files[0].Name.Name
	}
	pkg, _ := config.Check(packageName, fset, files, info)
	functions := map[string]functionInfo{}
	typesBySelector := map[string]contractTypeInfo{}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}
			obj, ok := info.Defs[fn.Name].(*types.Func)
			if !ok || obj == nil {
				continue
			}
			signature, ok := obj.Type().(*types.Signature)
			if !ok {
				continue
			}
			functions[fn.Name.Name] = functionInfo{Signature: signature, Package: pkg}
		}
	}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel == nil {
				return true
			}
			obj, ok := info.Uses[selector.Sel].(*types.Func)
			if !ok || obj == nil {
				return true
			}
			signature, ok := obj.Type().(*types.Signature)
			if !ok {
				return true
			}
			functions[exprString(fset, selector)] = functionInfo{Signature: signature, Package: pkg}
			return true
		})
	}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok || selector.Sel == nil {
				return true
			}
			obj, ok := info.Uses[selector.Sel].(*types.TypeName)
			if !ok || obj == nil {
				return true
			}
			typesBySelector[exprString(fset, selector)] = contractTypeInfo{
				Exported: obj.Exported(),
				Struct:   isStructType(obj.Type()),
			}
			return true
		})
	}
	return typedPackageInfo{Functions: functions, Types: typesBySelector}
}

func isStructType(typ types.Type) bool {
	_, ok := typ.Underlying().(*types.Struct)
	return ok
}

type packageInspectionCache struct {
	exports         map[string]map[string]string
	loadExportFiles func(packageDir string, importPaths []string) (map[string]string, error)
}

func newPackageInspectionCache() *packageInspectionCache {
	return &packageInspectionCache{
		exports:         map[string]map[string]string{},
		loadExportFiles: scanGoListExportFiles,
	}
}

func (cache *packageInspectionCache) exportFiles(packageDir string, importPaths []string) (map[string]string, error) {
	if cache == nil {
		return scanGoListExportFiles(packageDir, importPaths)
	}
	key := packageInspectionCacheKey(packageDir, importPaths)
	if exports, ok := cache.exports[key]; ok {
		return exports, nil
	}
	exports, err := cache.loadExportFiles(packageDir, importPaths)
	if err != nil {
		return nil, err
	}
	cache.exports[key] = exports
	return exports, nil
}

func packageInspectionCacheKey(packageDir string, importPaths []string) string {
	paths := append([]string(nil), importPaths...)
	sort.Strings(paths)
	return packageDir + "\x00" + strings.Join(paths, "\x00")
}

func contractScanImporter(packageDir string, fset *token.FileSet, files []*ast.File, inspectionCache *packageInspectionCache) types.Importer {
	importPaths := scanImportedGoPaths(files)
	if packageDir == "" || len(importPaths) == 0 || !insideGoModule(packageDir) {
		return importer.ForCompiler(fset, "source", nil)
	}
	exports, err := inspectionCache.exportFiles(packageDir, importPaths)
	return importer.ForCompiler(fset, "gc", func(path string) (io.ReadCloser, error) {
		if err != nil {
			return nil, fmt.Errorf("load export data for %s: %w", path, err)
		}
		exportPath := exports[path]
		if exportPath == "" {
			return nil, fmt.Errorf("missing export data for %s", path)
		}
		return os.Open(exportPath)
	})
}

func insideGoModule(dir string) bool {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return true
		}
		next := filepath.Dir(dir)
		if next == dir {
			return false
		}
		dir = next
	}
}

func scanImportedGoPaths(files []*ast.File) []string {
	seen := map[string]bool{}
	var paths []string
	for _, file := range files {
		for _, spec := range file.Imports {
			if spec.Path == nil {
				continue
			}
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil || path == "" || seen[path] {
				continue
			}
			seen[path] = true
			paths = append(paths, path)
		}
	}
	sort.Strings(paths)
	return paths
}

func scanGoListExportFiles(packageDir string, importPaths []string) (map[string]string, error) {
	args := append([]string{"list", "-deps", "-export", "-json"}, importPaths...)
	command := exec.Command("go", args...)
	command.Dir = packageDir
	output, err := command.Output()
	if err != nil {
		return nil, goListExportError(err)
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	exports := map[string]string{}
	for {
		var item struct {
			ImportPath string
			Export     string
		}
		if err := decoder.Decode(&item); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if item.ImportPath == "" || item.Export == "" {
			continue
		}
		exports[item.ImportPath] = item.Export
	}
	return exports, nil
}

// goListExportError surfaces the underlying go list failure, including its
// stderr, instead of letting the bare exit status reach the type checker as an
// opaque "load export data for X: exit status 1" with no cause.
func goListExportError(err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if stderr := strings.TrimSpace(string(exit.Stderr)); stderr != "" {
			return fmt.Errorf("%w\n%s", err, stderr)
		}
	}
	return err
}

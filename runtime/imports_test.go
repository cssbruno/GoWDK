package runtime_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

func TestRuntimePackagesDoNotImportInternalPackages(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, item := range file.Imports {
			importPath, err := strconv.Unquote(item.Path.Value)
			if err != nil {
				return err
			}
			if strings.Contains(importPath, "/internal/") || strings.HasSuffix(importPath, "/internal") {
				t.Errorf("%s imports internal package %q", path, importPath)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedRuntimePackagesDoNotDependOnRootConfigPackage(t *testing.T) {
	config := &packages.Config{
		Dir:   "..",
		Mode:  packages.NeedName | packages.NeedImports,
		Tests: false,
	}
	loaded, err := packages.Load(config,
		"./runtime/actions",
		"./runtime/api",
		"./runtime/app",
		"./runtime/auth",
		"./runtime/contracts",
		"./runtime/form",
		"./runtime/guard",
		"./runtime/html",
		"./runtime/partial",
		"./runtime/ratelimit",
		"./runtime/realtime",
		"./runtime/response",
		"./runtime/route",
		"./runtime/ssr",
		"./runtime/trace",
		"./runtime/validation",
	)
	if err != nil {
		t.Fatal(err)
	}
	if packages.PrintErrors(loaded) > 0 {
		t.Fatal("failed to load runtime package dependency graph")
	}
	for _, pkg := range loaded {
		if dependsOnPackage(pkg, "github.com/cssbruno/gowdk", map[*packages.Package]bool{}) {
			t.Fatalf("%s depends on root config package github.com/cssbruno/gowdk", pkg.PkgPath)
		}
	}
}

func dependsOnPackage(pkg *packages.Package, importPath string, seen map[*packages.Package]bool) bool {
	if pkg == nil || seen[pkg] {
		return false
	}
	seen[pkg] = true
	for path, imported := range pkg.Imports {
		if path == importPath {
			return true
		}
		if dependsOnPackage(imported, importPath, seen) {
			return true
		}
	}
	return false
}

// Package gotypes resolves Go contracts referenced from .gwdk component files.
package gotypes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

var goNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Field describes one resolved Go struct field.
type Field struct {
	Name string
	Type string
}

// Struct describes a resolved Go struct type.
type Struct struct {
	ImportPath string
	Name       string
	Fields     []Field
	FieldTypes map[string]string
}

// HasField reports whether the resolved struct declares field.
func (item Struct) HasField(field string) bool {
	for _, candidate := range item.Fields {
		if candidate.Name == field {
			return true
		}
	}
	return false
}

// FieldNames returns the resolved struct fields in source/type-checker order.
func (item Struct) FieldNames() []string {
	names := make([]string, 0, len(item.Fields))
	for _, field := range item.Fields {
		names = append(names, field.Name)
	}
	return names
}

// ResolveStruct resolves a Go struct type referenced by a component contract.
func ResolveStruct(imports []manifest.Import, ref manifest.GoTypeRef) (Struct, error) {
	importPath, err := ImportPathForAlias(imports, ref.Alias)
	if err != nil {
		return Struct{}, err
	}
	pkg, err := loadPackage(importPath)
	if err != nil {
		return Struct{}, err
	}
	obj := pkg.types.Scope().Lookup(ref.Name)
	if obj == nil {
		return Struct{}, fmt.Errorf("type %s.%s was not found in %q", ref.Alias, ref.Name, importPath)
	}
	typeName, ok := obj.(*types.TypeName)
	if !ok {
		return Struct{}, fmt.Errorf("%s.%s in %q is not a type", ref.Alias, ref.Name, importPath)
	}
	structType, ok := typeName.Type().Underlying().(*types.Struct)
	if !ok {
		return Struct{}, fmt.Errorf("type %s.%s in %q must be a struct", ref.Alias, ref.Name, importPath)
	}
	fields := make([]Field, 0, structType.NumFields())
	fieldTypes := map[string]string{}
	for index := 0; index < structType.NumFields(); index++ {
		field := structType.Field(index)
		if !field.Exported() {
			continue
		}
		fieldType := types.TypeString(field.Type(), qualifyPackage)
		fields = append(fields, Field{
			Name: field.Name(),
			Type: fieldType,
		})
		fieldTypes[field.Name()] = fieldType
		collectFieldTypes(field.Name(), field.Type(), fieldTypes, map[string]bool{}, 0)
	}
	return Struct{ImportPath: importPath, Name: ref.Name, Fields: fields, FieldTypes: fieldTypes}, nil
}

func collectFieldTypes(prefix string, typ types.Type, output map[string]string, seen map[string]bool, depth int) {
	if depth >= 4 {
		return
	}
	for {
		pointer, ok := typ.Underlying().(*types.Pointer)
		if !ok {
			break
		}
		typ = pointer.Elem()
	}
	key := canonicalType(typ)
	if key != "" {
		if seen[key] {
			return
		}
		seen[key] = true
		defer delete(seen, key)
	}
	switch typed := typ.Underlying().(type) {
	case *types.Struct:
		for index := 0; index < typed.NumFields(); index++ {
			field := typed.Field(index)
			if !field.Exported() {
				continue
			}
			path := prefix + "." + field.Name()
			output[path] = types.TypeString(field.Type(), qualifyPackage)
			collectFieldTypes(path, field.Type(), output, seen, depth+1)
		}
	case *types.Slice:
		path := prefix + "[]"
		output[path] = types.TypeString(typed.Elem(), qualifyPackage)
		collectFieldTypes(path, typed.Elem(), output, seen, depth+1)
	case *types.Array:
		path := prefix + "[]"
		output[path] = types.TypeString(typed.Elem(), qualifyPackage)
		collectFieldTypes(path, typed.Elem(), output, seen, depth+1)
	}
}

// ValidateStateInit verifies that the state init function can initialize the
// declared state type.
func ValidateStateInit(imports []manifest.Import, state manifest.StateContract) error {
	statePath, err := ImportPathForAlias(imports, state.Type.Alias)
	if err != nil {
		return err
	}
	initPath, err := ImportPathForAlias(imports, state.Init.Alias)
	if err != nil {
		return err
	}
	statePkg, err := loadPackage(statePath)
	if err != nil {
		return err
	}
	stateObj := statePkg.types.Scope().Lookup(state.Type.Name)
	if stateObj == nil {
		return fmt.Errorf("type %s.%s was not found in %q", state.Type.Alias, state.Type.Name, statePath)
	}
	stateType, ok := stateObj.(*types.TypeName)
	if !ok {
		return fmt.Errorf("%s.%s in %q is not a type", state.Type.Alias, state.Type.Name, statePath)
	}

	initPkg := statePkg
	if initPath != statePath {
		initPkg, err = loadPackage(initPath)
		if err != nil {
			return err
		}
	}
	initObj := initPkg.types.Scope().Lookup(state.Init.Name)
	if initObj == nil {
		return fmt.Errorf("state init function %s.%s was not found in %q", state.Init.Alias, state.Init.Name, initPath)
	}
	initFunc, ok := initObj.(*types.Func)
	if !ok {
		return fmt.Errorf("%s.%s in %q is not a function", state.Init.Alias, state.Init.Name, initPath)
	}
	signature, ok := initFunc.Type().(*types.Signature)
	if !ok {
		return fmt.Errorf("%s.%s in %q is not a function", state.Init.Alias, state.Init.Name, initPath)
	}
	if signature.Params().Len() != 0 {
		return fmt.Errorf("state init function %s.%s must not accept arguments", state.Init.Alias, state.Init.Name)
	}
	if signature.Results().Len() != 1 {
		return fmt.Errorf("state init function %s.%s must return exactly one value", state.Init.Alias, state.Init.Name)
	}
	if canonicalType(signature.Results().At(0).Type()) != canonicalType(stateType.Type()) {
		return fmt.Errorf("state init function %s.%s returns %s, not %s.%s", state.Init.Alias, state.Init.Name, canonicalType(signature.Results().At(0).Type()), state.Type.Alias, state.Type.Name)
	}
	return nil
}

// RunStateInitJSON runs a declared state init function and returns its JSON
// encoding.
func RunStateInitJSON(imports []manifest.Import, state manifest.StateContract) ([]byte, error) {
	importPath, err := ImportPathForAlias(imports, state.Init.Alias)
	if err != nil {
		return nil, err
	}
	if !goNamePattern.MatchString(state.Init.Alias) {
		return nil, fmt.Errorf("invalid state init import alias %q", state.Init.Alias)
	}
	if !goNamePattern.MatchString(state.Init.Name) {
		return nil, fmt.Errorf("invalid state init function name %q", state.Init.Name)
	}
	source := fmt.Sprintf(`package main

import (
	"encoding/json"
	"os"
	"reflect"

	%s %q
)

func main() {
	value := %s.%s()
	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Pointer {
		reflected = reflected.Elem()
	}
	typed := reflected.Type()
	fields := map[string]any{}
	for index := 0; index < reflected.NumField(); index++ {
		field := typed.Field(index)
		if field.PkgPath != "" {
			continue
		}
		fields[field.Name] = reflected.Field(index).Interface()
	}
	if err := json.NewEncoder(os.Stdout).Encode(fields); err != nil {
		panic(err)
	}
}
`, state.Init.Alias, importPath, state.Init.Alias, state.Init.Name)
	file, err := os.CreateTemp("", "gowdk-state-init-*.go")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	defer os.Remove(path)
	if _, err := file.WriteString(source); err != nil {
		file.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	command := exec.Command("go", "run", path)
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("run state init function %s.%s: %w\n%s", state.Init.Alias, state.Init.Name, err, strings.TrimSpace(string(output)))
	}
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return nil, fmt.Errorf("state init function %s.%s produced empty JSON", state.Init.Alias, state.Init.Name)
	}
	if !json.Valid(output) {
		return nil, fmt.Errorf("state init function %s.%s produced invalid JSON", state.Init.Alias, state.Init.Name)
	}
	return append([]byte(nil), output...), nil
}

// ImportPathForAlias returns the concrete Go import path for a .gwdk import
// alias and rejects relative import paths.
func ImportPathForAlias(imports []manifest.Import, alias string) (string, error) {
	if strings.TrimSpace(alias) == "" {
		return "", fmt.Errorf("Go import alias is required")
	}
	for _, item := range imports {
		effective, err := EffectiveImportAlias(item)
		if err != nil {
			return "", err
		}
		if effective == alias {
			return item.Path, nil
		}
	}
	return "", fmt.Errorf("Go import alias %q is not declared", alias)
}

// EffectiveImportAlias returns the explicit import alias or the package name
// used by Go for an unaliased import.
func EffectiveImportAlias(item manifest.Import) (string, error) {
	if strings.TrimSpace(item.Alias) != "" {
		return item.Alias, nil
	}
	info, err := goList(item.Path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(info.Name) == "" {
		return "", fmt.Errorf("go list %q did not return a package name", item.Path)
	}
	return info.Name, nil
}

// ValidateImportPath rejects relative or malformed import paths in component
// contracts.
func ValidateImportPath(importPath string) error {
	path := strings.TrimSpace(importPath)
	if path == "" {
		return fmt.Errorf("Go import path is required")
	}
	if strings.HasPrefix(path, ".") || strings.HasPrefix(path, "/") || strings.Contains(path, `\`) {
		return fmt.Errorf("Go import path %q must be a module import path, not a relative path", importPath)
	}
	return nil
}

func canonicalType(typ types.Type) string {
	return types.TypeString(typ, qualifyPackage)
}

func qualifyPackage(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	return pkg.Path()
}

type loadedPackage struct {
	info  goListPackage
	files []*ast.File
	types *types.Package
}

type goListPackage struct {
	ImportPath string
	Dir        string
	Name       string
	GoFiles    []string
	Error      *struct {
		Err string
	}
}

func loadPackage(importPath string) (loadedPackage, error) {
	info, err := goList(importPath)
	if err != nil {
		return loadedPackage{}, err
	}
	if info.Dir == "" {
		return loadedPackage{}, fmt.Errorf("go list %q did not return a package directory", importPath)
	}
	fileSet := token.NewFileSet()
	files := make([]*ast.File, 0, len(info.GoFiles))
	for _, name := range info.GoFiles {
		filePath := filepath.Join(info.Dir, name)
		file, err := parser.ParseFile(fileSet, filePath, nil, parser.ParseComments)
		if err != nil {
			return loadedPackage{}, fmt.Errorf("parse %s: %w", filePath, err)
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return loadedPackage{}, fmt.Errorf("Go package %q has no buildable Go files", importPath)
	}
	config := types.Config{Importer: importer.Default()}
	checked, err := config.Check(info.ImportPath, fileSet, files, nil)
	if err != nil {
		return loadedPackage{}, fmt.Errorf("type-check %q: %w", importPath, err)
	}
	return loadedPackage{info: info, files: files, types: checked}, nil
}

func goList(importPath string) (goListPackage, error) {
	if err := ValidateImportPath(importPath); err != nil {
		return goListPackage{}, err
	}
	command := exec.Command("go", "list", "-json", importPath)
	output, err := command.CombinedOutput()
	if err != nil {
		return goListPackage{}, fmt.Errorf("go list %q: %w\n%s", importPath, err, strings.TrimSpace(string(output)))
	}
	var info goListPackage
	if err := json.Unmarshal(output, &info); err != nil {
		return goListPackage{}, fmt.Errorf("decode go list %q: %w", importPath, err)
	}
	if info.Error != nil && info.Error.Err != "" {
		return goListPackage{}, fmt.Errorf("go list %q: %s", importPath, info.Error.Err)
	}
	return info, nil
}

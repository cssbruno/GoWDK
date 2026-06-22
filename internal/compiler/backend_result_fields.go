package compiler

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

func collectResultStructs(files []*ast.File) map[string]resultStruct {
	structTypes := map[string]*ast.StructType{}
	for _, file := range files {
		for _, declaration := range file.Decls {
			gen, ok := declaration.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name == nil || !typeSpec.Name.IsExported() {
					continue
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				structTypes[typeSpec.Name.Name] = structType
			}
		}
	}
	structs := map[string]resultStruct{}
	for name, structType := range structTypes {
		structs[name] = backendResultStruct(name, structType, structTypes)
	}
	return structs
}

func backendResultStruct(typeName string, structType *ast.StructType, structs map[string]*ast.StructType) resultStruct {
	fields, message := backendResultStructFields(typeName, "", "", structType, structs, map[string]bool{})
	if message != "" {
		return resultStruct{Message: message}
	}
	return resultStruct{Fields: fields}
}

func backendResultStructFields(typeName string, pathPrefix string, selectorPrefix string, structType *ast.StructType, structs map[string]*ast.StructType, stack map[string]bool) ([]source.BackendResultField, string) {
	if structType == nil || structType.Fields == nil {
		return nil, ""
	}
	if stack[typeName] {
		return nil, ""
	}
	stack[typeName] = true
	defer delete(stack, typeName)

	seen := map[string]bool{}
	var fields []source.BackendResultField
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			return nil, fmt.Sprintf("typed load result %s cannot use embedded fields", typeName)
		}
		fieldPath, skip, explicit, err := jsonTagName(field)
		if err != nil {
			return nil, fmt.Sprintf("typed load result %s has invalid json tag: %v", typeName, err)
		}
		var exportedNames []*ast.Ident
		for _, name := range field.Names {
			if name != nil && name.IsExported() {
				exportedNames = append(exportedNames, name)
			}
		}
		if len(exportedNames) == 0 || skip {
			continue
		}
		if explicit && fieldPath != "" && len(exportedNames) > 1 {
			return nil, fmt.Sprintf("typed load result %s cannot reuse one explicit json tag across multiple fields", typeName)
		}
		for _, name := range exportedNames {
			selector := resultPath(selectorPrefix, name.Name)
			for _, namePath := range resultFieldPathNames(fieldPath, name.Name) {
				path := resultPath(pathPrefix, namePath)
				if seen[path] {
					return nil, fmt.Sprintf("typed load result %s maps multiple fields to load field %q", typeName, path)
				}
				seen[path] = true
				fields = append(fields, source.BackendResultField{
					Path:     path,
					Selector: selector,
					Type:     astExprString(field.Type),
				})
			}
			if nestedName := localResultStructName(field.Type); nestedName != "" {
				nestedType := structs[nestedName]
				for _, namePath := range resultFieldPathNames(fieldPath, name.Name) {
					nestedFields, message := backendResultStructFields(nestedName, resultPath(pathPrefix, namePath), selector, nestedType, structs, stack)
					if message != "" {
						return nil, message
					}
					fields = append(fields, nestedFields...)
				}
			}
		}
	}
	return fields, ""
}

func backendTypedResultStruct(typeName string, typ types.Type) resultStruct {
	fields, message := backendTypedResultStructFields(typeName, "", "", typ, map[string]bool{})
	if message != "" {
		return resultStruct{Message: message}
	}
	return resultStruct{Fields: fields}
}

func backendTypedResultStructFields(typeName string, pathPrefix string, selectorPrefix string, typ types.Type, stack map[string]bool) ([]source.BackendResultField, string) {
	typ = derefResultType(types.Unalias(typ))
	structType, ok := types.Unalias(typ).Underlying().(*types.Struct)
	if !ok {
		return nil, fmt.Sprintf("typed load result %s must be an exported struct in the same package", typeName)
	}
	stackKey := typeName
	if named, ok := types.Unalias(typ).(*types.Named); ok && named.Obj() != nil {
		pkgPath := ""
		if named.Obj().Pkg() != nil {
			pkgPath = named.Obj().Pkg().Path()
		}
		stackKey = pkgPath + "." + named.Obj().Name()
	}
	if stack[stackKey] {
		return nil, ""
	}
	stack[stackKey] = true
	defer delete(stack, stackKey)

	seen := map[string]bool{}
	var fields []source.BackendResultField
	for index := 0; index < structType.NumFields(); index++ {
		field := structType.Field(index)
		if field.Embedded() {
			return nil, fmt.Sprintf("typed load result %s cannot use embedded fields", typeName)
		}
		fieldPath, skip, _, err := jsonTagNameValue(structType.Tag(index))
		if err != nil {
			return nil, fmt.Sprintf("typed load result %s has invalid json tag: %v", typeName, err)
		}
		if !field.Exported() || skip {
			continue
		}
		selector := resultPath(selectorPrefix, field.Name())
		for _, namePath := range resultFieldPathNames(fieldPath, field.Name()) {
			path := resultPath(pathPrefix, namePath)
			if seen[path] {
				return nil, fmt.Sprintf("typed load result %s maps multiple fields to load field %q", typeName, path)
			}
			seen[path] = true
			fields = append(fields, source.BackendResultField{
				Path:     path,
				Selector: selector,
				Type:     types.TypeString(field.Type(), func(pkg *types.Package) string { return pkg.Name() }),
			})
		}
		for _, namePath := range resultFieldPathNames(fieldPath, field.Name()) {
			nestedFields, message := backendTypedResultStructFields(typeName, resultPath(pathPrefix, namePath), selector, field.Type(), stack)
			if message != "" && strings.Contains(message, "must be an exported struct") {
				continue
			}
			if message != "" {
				return nil, message
			}
			fields = append(fields, nestedFields...)
		}
	}
	return fields, ""
}

func jsonTagName(field *ast.Field) (string, bool, bool, error) {
	if field == nil || field.Tag == nil {
		return "", false, false, nil
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false, false, err
	}
	return jsonTagNameValue(tag)
}

func jsonTagNameValue(tag string) (string, bool, bool, error) {
	value, ok, err := structTagValue(tag, "json")
	if err != nil || !ok {
		return "", false, ok, err
	}
	name, _, _ := strings.Cut(value, ",")
	if name == "-" {
		return "", true, true, nil
	}
	return strings.TrimSpace(name), false, true, nil
}

func localResultStructName(expression ast.Expr) string {
	switch expr := expression.(type) {
	case *ast.Ident:
		if expr.IsExported() {
			return expr.Name
		}
	case *ast.StarExpr:
		return localResultStructName(expr.X)
	}
	return ""
}

func astExprString(expression ast.Expr) string {
	if expression == nil {
		return ""
	}
	var buf bytes.Buffer
	if err := format.Node(&buf, token.NewFileSet(), expression); err != nil {
		return ""
	}
	return buf.String()
}

func resultPath(prefix string, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func resultFieldPathNames(tagName string, fieldName string) []string {
	if tagName == "" || tagName == fieldName {
		return []string{fieldName}
	}
	return []string{tagName, fieldName}
}

func derefResultType(typ types.Type) types.Type {
	for {
		pointer, ok := types.Unalias(typ).(*types.Pointer)
		if !ok {
			return typ
		}
		typ = pointer.Elem()
	}
}

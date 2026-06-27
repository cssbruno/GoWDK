package compiler

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

func collectInputStructs(files []*ast.File, importMaps []map[string]string) map[string]inputStruct {
	structs := map[string]inputStruct{}
	for index, file := range files {
		var imports map[string]string
		if index < len(importMaps) {
			imports = importMaps[index]
		}
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
				structs[typeSpec.Name.Name] = backendInputStruct(typeSpec.Name.Name, structType, imports)
			}
		}
	}
	return structs
}

func backendInputStruct(typeName string, structType *ast.StructType, imports map[string]string) inputStruct {
	return backendTaggedInputStruct("typed action input", typeName, structType, imports, "form", true)
}

func backendTaggedInputStruct(label string, typeName string, structType *ast.StructType, imports map[string]string, tagKey string, allowFiles bool) inputStruct {
	if structType == nil || structType.Fields == nil {
		return inputStruct{}
	}
	seen := map[string]bool{}
	var fields []source.BackendInputField
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			return inputStruct{Message: fmt.Sprintf("%s %s cannot use embedded fields", label, typeName)}
		}
		formName, skip, explicit, err := inputTagName(field, tagKey)
		if err != nil {
			return inputStruct{Message: fmt.Sprintf("%s %s has invalid %s tag: %v", label, typeName, tagKey, err)}
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
		if explicit && len(exportedNames) > 1 {
			return inputStruct{Message: fmt.Sprintf("%s %s cannot reuse one explicit %s tag across multiple fields", label, typeName, tagKey)}
		}
		fieldType, ok := backendInputFieldType(field.Type, imports)
		if !ok || (!allowFiles && backendInputFieldTypeIsFile(fieldType)) {
			return inputStruct{Message: fmt.Sprintf("%s %s uses unsupported field type", label, typeName)}
		}
		for _, name := range exportedNames {
			nameFormName := formName
			if nameFormName == "" {
				nameFormName = name.Name
			}
			if seen[nameFormName] {
				return inputStruct{Message: fmt.Sprintf("%s %s maps multiple fields to input field %q", label, typeName, nameFormName)}
			}
			seen[nameFormName] = true
			fields = append(fields, source.BackendInputField{
				FieldName: name.Name,
				FormName:  nameFormName,
				Type:      fieldType,
			})
		}
	}
	return inputStruct{Fields: fields}
}

func backendTypedInputStruct(typeName string, typ types.Type) inputStruct {
	return backendTypedTaggedInputStruct("typed action input", typeName, typ, "form", true)
}

func backendTypedAPIInputStruct(typeName string, typ types.Type) inputStruct {
	return backendTypedTaggedInputStruct("typed API input", typeName, typ, "json", false)
}

func backendTypedTaggedInputStruct(label string, typeName string, typ types.Type, tagKey string, allowFiles bool) inputStruct {
	structType, ok := types.Unalias(typ).Underlying().(*types.Struct)
	if !ok {
		return inputStruct{Message: fmt.Sprintf("%s %s must be an exported struct in the same package", label, typeName)}
	}
	seen := map[string]bool{}
	var fields []source.BackendInputField
	for index := 0; index < structType.NumFields(); index++ {
		field := structType.Field(index)
		if field.Embedded() {
			return inputStruct{Message: fmt.Sprintf("%s %s cannot use embedded fields", label, typeName)}
		}
		formName, skip, _, err := inputTagNameValue(structType.Tag(index), tagKey)
		if err != nil {
			return inputStruct{Message: fmt.Sprintf("%s %s has invalid %s tag: %v", label, typeName, tagKey, err)}
		}
		if !field.Exported() || skip {
			continue
		}
		fieldType, ok := backendTypedInputFieldType(field.Type())
		if !ok || (!allowFiles && backendInputFieldTypeIsFile(fieldType)) {
			return inputStruct{Message: fmt.Sprintf("%s %s uses unsupported field type", label, typeName)}
		}
		if formName == "" {
			formName = field.Name()
		}
		if seen[formName] {
			return inputStruct{Message: fmt.Sprintf("%s %s maps multiple fields to input field %q", label, typeName, formName)}
		}
		seen[formName] = true
		fields = append(fields, source.BackendInputField{
			FieldName: field.Name(),
			FormName:  formName,
			Type:      fieldType,
		})
	}
	return inputStruct{Fields: fields}
}

func inputTagName(field *ast.Field, key string) (string, bool, bool, error) {
	if field == nil || field.Tag == nil {
		return "", false, false, nil
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false, false, err
	}
	return inputTagNameValue(tag, key)
}

func inputTagNameValue(tag string, key string) (string, bool, bool, error) {
	value, ok, err := structTagValue(tag, key)
	if err != nil || !ok {
		return "", false, ok, err
	}
	name, _, _ := strings.Cut(value, ",")
	if name == "-" {
		return "", true, true, nil
	}
	return strings.TrimSpace(name), false, true, nil
}

func structTagValue(tag string, key string) (string, bool, error) {
	for tag != "" {
		tag = strings.TrimLeft(tag, " ")
		if tag == "" {
			return "", false, nil
		}
		keyEnd := strings.IndexByte(tag, ':')
		if keyEnd <= 0 {
			return "", false, fmt.Errorf("malformed struct tag")
		}
		name := tag[:keyEnd]
		rest := tag[keyEnd+1:]
		if rest == "" || rest[0] != '"' {
			return "", false, fmt.Errorf("malformed struct tag")
		}
		valueEnd := 1
		for valueEnd < len(rest) {
			if rest[valueEnd] == '\\' {
				valueEnd += 2
				continue
			}
			if rest[valueEnd] == '"' {
				break
			}
			valueEnd++
		}
		if valueEnd >= len(rest) || rest[valueEnd] != '"' {
			return "", false, fmt.Errorf("malformed struct tag")
		}
		rawValue := rest[:valueEnd+1]
		value, err := strconv.Unquote(rawValue)
		if err != nil {
			return "", false, err
		}
		if name == key {
			return value, true, nil
		}
		tag = rest[valueEnd+1:]
	}
	return "", false, nil
}

func backendInputFieldType(expression ast.Expr, imports map[string]string) (string, bool) {
	if ident, ok := expression.(*ast.Ident); ok {
		if fieldType, ok := source.LookupBackendInputFieldType(ident.Name); ok {
			return fieldType.Name, true
		}
		return "", false
	}
	if isSelector(expression, imports, formImportPath, "File") {
		return source.BackendInputTypeFile, true
	}
	array, ok := expression.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return "", false
	}
	if ident, ok := array.Elt.(*ast.Ident); ok && ident.Name == "string" {
		return source.BackendInputTypeStringSlice, true
	}
	if isSelector(array.Elt, imports, formImportPath, "File") {
		return source.BackendInputTypeFileSlice, true
	}
	return "", false
}

func backendInputFieldTypeIsFile(fieldType string) bool {
	info, ok := source.LookupBackendInputFieldType(fieldType)
	return ok && (info.Kind == source.BackendInputFieldKindFile || info.Kind == source.BackendInputFieldKindFileSlice)
}

func backendTypedInputFieldType(typ types.Type) (string, bool) {
	typ = types.Unalias(typ)
	if basic, ok := typ.Underlying().(*types.Basic); ok {
		if fieldType, ok := source.LookupBackendInputFieldType(basic.Name()); ok {
			return fieldType.Name, true
		}
		return "", false
	}
	if isTypedNamed(typ, formImportPath, "File") {
		return source.BackendInputTypeFile, true
	}
	slice, ok := typ.Underlying().(*types.Slice)
	if !ok {
		return "", false
	}
	if isTypedNamed(slice.Elem(), formImportPath, "File") {
		return source.BackendInputTypeFileSlice, true
	}
	basic, ok := types.Unalias(slice.Elem()).Underlying().(*types.Basic)
	if ok && basic.Kind() == types.String {
		return source.BackendInputTypeStringSlice, true
	}
	return "", false
}

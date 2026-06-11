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

func collectInputStructs(files []*ast.File) map[string]inputStruct {
	structs := map[string]inputStruct{}
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
				structs[typeSpec.Name.Name] = backendInputStruct(typeSpec.Name.Name, structType)
			}
		}
	}
	return structs
}

func backendInputStruct(typeName string, structType *ast.StructType) inputStruct {
	if structType == nil || structType.Fields == nil {
		return inputStruct{}
	}
	seen := map[string]bool{}
	var fields []source.BackendInputField
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			return inputStruct{Message: fmt.Sprintf("typed action input %s cannot use embedded fields", typeName)}
		}
		formName, skip, explicit, err := formTagName(field)
		if err != nil {
			return inputStruct{Message: fmt.Sprintf("typed action input %s has invalid form tag: %v", typeName, err)}
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
			return inputStruct{Message: fmt.Sprintf("typed action input %s cannot reuse one explicit form tag across multiple fields", typeName)}
		}
		fieldType, ok := backendInputFieldType(field.Type)
		if !ok {
			return inputStruct{Message: fmt.Sprintf("typed action input %s uses unsupported field type", typeName)}
		}
		for _, name := range exportedNames {
			nameFormName := formName
			if nameFormName == "" {
				nameFormName = name.Name
			}
			if seen[nameFormName] {
				return inputStruct{Message: fmt.Sprintf("typed action input %s maps multiple fields to form field %q", typeName, nameFormName)}
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
	structType, ok := types.Unalias(typ).Underlying().(*types.Struct)
	if !ok {
		return inputStruct{Message: fmt.Sprintf("typed action input %s must be an exported struct in the same package", typeName)}
	}
	seen := map[string]bool{}
	var fields []source.BackendInputField
	for index := 0; index < structType.NumFields(); index++ {
		field := structType.Field(index)
		if field.Embedded() {
			return inputStruct{Message: fmt.Sprintf("typed action input %s cannot use embedded fields", typeName)}
		}
		formName, skip, _, err := formTagNameValue(structType.Tag(index))
		if err != nil {
			return inputStruct{Message: fmt.Sprintf("typed action input %s has invalid form tag: %v", typeName, err)}
		}
		if !field.Exported() || skip {
			continue
		}
		fieldType, ok := backendTypedInputFieldType(field.Type())
		if !ok {
			return inputStruct{Message: fmt.Sprintf("typed action input %s uses unsupported field type", typeName)}
		}
		if formName == "" {
			formName = field.Name()
		}
		if seen[formName] {
			return inputStruct{Message: fmt.Sprintf("typed action input %s maps multiple fields to form field %q", typeName, formName)}
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

func formTagName(field *ast.Field) (string, bool, bool, error) {
	if field == nil || field.Tag == nil {
		return "", false, false, nil
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false, false, err
	}
	return formTagNameValue(tag)
}

func formTagNameValue(tag string) (string, bool, bool, error) {
	value, ok, err := structTagValue(tag, "form")
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

func backendInputFieldType(expression ast.Expr) (string, bool) {
	if ident, ok := expression.(*ast.Ident); ok {
		switch ident.Name {
		case "string", "bool", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
			return ident.Name, true
		default:
			return "", false
		}
	}
	array, ok := expression.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return "", false
	}
	ident, ok := array.Elt.(*ast.Ident)
	if !ok || ident.Name != "string" {
		return "", false
	}
	return "[]string", true
}

func backendTypedInputFieldType(typ types.Type) (string, bool) {
	typ = types.Unalias(typ)
	if basic, ok := typ.Underlying().(*types.Basic); ok {
		switch basic.Kind() {
		case types.String, types.Bool, types.Int, types.Int8, types.Int16, types.Int32, types.Int64, types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64:
			return basic.Name(), true
		default:
			return "", false
		}
	}
	slice, ok := typ.Underlying().(*types.Slice)
	if !ok {
		return "", false
	}
	basic, ok := types.Unalias(slice.Elem()).Underlying().(*types.Basic)
	if !ok || basic.Kind() != types.String {
		return "", false
	}
	return "[]string", true
}

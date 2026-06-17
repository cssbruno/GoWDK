package contractscan

import (
	"fmt"
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/source"
)

func contractInputStruct(typeName string, structType *ast.StructType) inputStruct {
	if structType == nil || structType.Fields == nil {
		return inputStruct{}
	}
	seen := map[string]bool{}
	var fields []source.BackendInputField
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			return inputStruct{Message: fmt.Sprintf("contract input %s cannot use embedded fields", typeName)}
		}
		formName, skip, explicit, err := contractFormTagName(field)
		if err != nil {
			return inputStruct{Message: fmt.Sprintf("contract input %s has invalid form tag: %v", typeName, err)}
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
			return inputStruct{Message: fmt.Sprintf("contract input %s cannot reuse one explicit form tag across multiple fields", typeName)}
		}
		fieldType, ok := contractInputFieldType(field.Type)
		if !ok {
			return inputStruct{Message: fmt.Sprintf("contract input %s uses unsupported field type", typeName)}
		}
		for _, name := range exportedNames {
			nameFormName := formName
			if nameFormName == "" {
				nameFormName = name.Name
			}
			if seen[nameFormName] {
				return inputStruct{Message: fmt.Sprintf("contract input %s maps multiple fields to form field %q", typeName, nameFormName)}
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

func contractPayloadStruct(typeName string, structType *ast.StructType) inputStruct {
	if structType == nil || structType.Fields == nil {
		return inputStruct{}
	}
	seen := map[string]bool{}
	var fields []source.BackendInputField
	for _, field := range structType.Fields.List {
		if len(field.Names) == 0 {
			return inputStruct{Message: fmt.Sprintf("contract payload %s cannot use embedded fields", typeName)}
		}
		jsonName, skip, explicit, err := contractJSONTagName(field)
		if err != nil {
			return inputStruct{Message: fmt.Sprintf("contract payload %s has invalid json tag: %v", typeName, err)}
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
			return inputStruct{Message: fmt.Sprintf("contract payload %s cannot reuse one explicit json tag across multiple fields", typeName)}
		}
		fieldType, ok := contractInputFieldType(field.Type)
		if !ok {
			return inputStruct{Message: fmt.Sprintf("contract payload %s uses unsupported field type", typeName)}
		}
		for _, name := range exportedNames {
			nameJSONName := jsonName
			if nameJSONName == "" {
				nameJSONName = name.Name
			}
			if seen[nameJSONName] {
				return inputStruct{Message: fmt.Sprintf("contract payload %s maps multiple fields to json field %q", typeName, nameJSONName)}
			}
			seen[nameJSONName] = true
			fields = append(fields, source.BackendInputField{
				FieldName: name.Name,
				FormName:  nameJSONName,
				Type:      fieldType,
			})
		}
	}
	return inputStruct{Fields: fields}
}

func contractPayloadType(typeName string, typ types.Type) inputStruct {
	structType, ok := types.Unalias(typ).Underlying().(*types.Struct)
	if !ok {
		return inputStruct{}
	}
	seen := map[string]bool{}
	var fields []source.BackendInputField
	for index := 0; index < structType.NumFields(); index++ {
		field := structType.Field(index)
		if field.Anonymous() {
			return inputStruct{Message: fmt.Sprintf("contract payload %s cannot use embedded fields", typeName)}
		}
		jsonName, skip, _, err := contractJSONTagNameRaw(structType.Tag(index))
		if err != nil {
			return inputStruct{Message: fmt.Sprintf("contract payload %s has invalid json tag: %v", typeName, err)}
		}
		if !field.Exported() || skip {
			continue
		}
		fieldType, ok := contractInputFieldTypeFromType(field.Type())
		if !ok {
			return inputStruct{Message: fmt.Sprintf("contract payload %s uses unsupported field type", typeName)}
		}
		if jsonName == "" {
			jsonName = field.Name()
		}
		if seen[jsonName] {
			return inputStruct{Message: fmt.Sprintf("contract payload %s maps multiple fields to json field %q", typeName, jsonName)}
		}
		seen[jsonName] = true
		fields = append(fields, source.BackendInputField{
			FieldName: field.Name(),
			FormName:  jsonName,
			Type:      fieldType,
		})
	}
	return inputStruct{Fields: fields}
}

func contractFormTagName(field *ast.Field) (string, bool, bool, error) {
	if field == nil || field.Tag == nil {
		return "", false, false, nil
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false, false, err
	}
	value, ok, err := contractStructTagValue(tag, "form")
	if err != nil || !ok {
		return "", false, ok, err
	}
	name, _, _ := strings.Cut(value, ",")
	if name == "-" {
		return "", true, true, nil
	}
	return strings.TrimSpace(name), false, true, nil
}

func contractJSONTagName(field *ast.Field) (string, bool, bool, error) {
	if field == nil || field.Tag == nil {
		return "", false, false, nil
	}
	tag, err := strconv.Unquote(field.Tag.Value)
	if err != nil {
		return "", false, false, err
	}
	return contractJSONTagNameRaw(tag)
}

func contractJSONTagNameRaw(tag string) (string, bool, bool, error) {
	value, ok, err := contractStructTagValue(tag, "json")
	if err != nil || !ok {
		return "", false, ok, err
	}
	name, _, _ := strings.Cut(value, ",")
	if name == "-" {
		return "", true, true, nil
	}
	return strings.TrimSpace(name), false, true, nil
}

func contractStructTagValue(tag string, key string) (string, bool, error) {
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

func contractInputFieldType(expression ast.Expr) (string, bool) {
	if ident, ok := expression.(*ast.Ident); ok {
		if fieldType, ok := source.LookupBackendInputFieldType(ident.Name); ok {
			return fieldType.Name, true
		}
		return "", false
	}
	array, ok := expression.(*ast.ArrayType)
	if !ok || array.Len != nil {
		return "", false
	}
	ident, ok := array.Elt.(*ast.Ident)
	if !ok || ident.Name != "string" {
		return "", false
	}
	return source.BackendInputTypeStringSlice, true
}

func contractInputFieldTypeFromType(typ types.Type) (string, bool) {
	typ = types.Unalias(typ)
	if basic, ok := typ.Underlying().(*types.Basic); ok {
		switch basic.Kind() {
		case types.Bool:
			return source.BackendInputTypeBool, true
		case types.Int:
			return source.BackendInputTypeInt, true
		case types.Int8:
			return source.BackendInputTypeInt8, true
		case types.Int16:
			return source.BackendInputTypeInt16, true
		case types.Int32:
			return source.BackendInputTypeInt32, true
		case types.Int64:
			return source.BackendInputTypeInt64, true
		case types.Uint:
			return source.BackendInputTypeUint, true
		case types.Uint8:
			return source.BackendInputTypeUint8, true
		case types.Uint16:
			return source.BackendInputTypeUint16, true
		case types.Uint32:
			return source.BackendInputTypeUint32, true
		case types.Uint64:
			return source.BackendInputTypeUint64, true
		case types.String:
			return source.BackendInputTypeString, true
		}
	}
	slice, ok := typ.Underlying().(*types.Slice)
	if !ok {
		return "", false
	}
	elem, ok := types.Unalias(slice.Elem()).Underlying().(*types.Basic)
	if !ok || elem.Kind() != types.String {
		return "", false
	}
	return source.BackendInputTypeStringSlice, true
}

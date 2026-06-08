package contractscan

import (
	"fmt"
	"go/ast"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

func contractInputStruct(typeName string, structType *ast.StructType) inputStruct {
	if structType == nil || structType.Fields == nil {
		return inputStruct{}
	}
	seen := map[string]bool{}
	var fields []manifest.BackendInputField
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
			fields = append(fields, manifest.BackendInputField{
				FieldName: name.Name,
				FormName:  nameFormName,
				Type:      fieldType,
			})
		}
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

package view

import (
	"encoding/json"
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"strconv"
	"strings"
)

func validateIslandField(field string, fields map[string]bool) error {
	if !islandFieldPattern.MatchString(field) {
		return fmt.Errorf("invalid island field %q", field)
	}
	if fields != nil && !fields[field] {
		return fmt.Errorf("unknown island field %q", field)
	}
	return nil
}

func validateIslandReadableValue(value string, fields map[string]bool) error {
	value = strings.TrimSpace(value)
	if isIslandScalarLiteral(value) {
		return nil
	}
	if islandFieldPattern.MatchString(value) {
		return validateIslandField(value, fields)
	}
	return fmt.Errorf("unsupported island value %q", value)
}

func validateIslandSymbol(field string, symbols map[string]clientlang.ValueType) (clientlang.ValueType, error) {
	if !islandFieldPattern.MatchString(field) {
		return clientlang.TypeUnknown, fmt.Errorf("invalid island field %q", field)
	}
	typ, ok := symbols[field]
	if symbols != nil && !ok {
		return clientlang.TypeUnknown, fmt.Errorf("unknown island field %q", field)
	}
	return typ, nil
}

func boolFieldSymbols(fields map[string]bool) map[string]clientlang.ValueType {
	if fields == nil {
		return nil
	}
	symbols := map[string]clientlang.ValueType{}
	for field, ok := range fields {
		if ok {
			symbols[field] = clientlang.TypeUnknown
		}
	}
	return symbols
}

func firstHandlerMap(handlers []map[string]clientlang.Handler) map[string]clientlang.Handler {
	if len(handlers) == 0 {
		return nil
	}
	return handlers[0]
}

func handlerParamType(handler clientlang.Handler, index int) clientlang.ValueType {
	if index < 0 || index >= len(handler.ParamTypes) {
		return clientlang.TypeUnknown
	}
	return handler.ParamTypes[index]
}

func compatibleNumericType(actual, expected clientlang.ValueType) bool {
	if actual == clientlang.TypeUnknown || expected == clientlang.TypeUnknown {
		return true
	}
	return (actual == clientlang.TypeInt || actual == clientlang.TypeFloat) &&
		(expected == clientlang.TypeInt || expected == clientlang.TypeFloat)
}

func isIslandScalarLiteral(value string) bool {
	if value == "true" || value == "false" || value == "null" {
		return true
	}
	if islandNumberPattern.MatchString(value) {
		return true
	}
	if strings.HasPrefix(value, `"`) {
		_, err := strconv.Unquote(value)
		return err == nil
	}
	return false
}

func islandCallFields(call clientlang.Call) []string {
	seen := map[string]bool{}
	for _, arg := range call.Args {
		arg = strings.TrimSpace(arg)
		if islandFieldPattern.MatchString(arg) && !isIslandScalarLiteral(arg) {
			seen[arg] = true
		}
	}
	return sortedKeys(seen)
}

func islandTextBinding(value string) (string, bool) {
	match := islandTextBindingPattern.FindStringSubmatch(value)
	if match == nil {
		return "", false
	}
	return match[1], true
}

func isIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index, char := range value {
		switch {
		case char >= 'A' && char <= 'Z':
		case char >= 'a' && char <= 'z':
		case char == '_':
		case index > 0 && char >= '0' && char <= '9':
		default:
			return false
		}
	}
	return true
}

func cloneStack(input map[string]bool) map[string]bool {
	output := map[string]bool{}
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneValues(input map[string]string) map[string]string {
	output := map[string]string{}
	for key, value := range input {
		output[key] = value
	}
	return output
}

func cloneActionInputFields(input map[string][]ActionInputField) map[string][]ActionInputField {
	output := map[string][]ActionInputField{}
	for key, fields := range input {
		output[key] = append([]ActionInputField(nil), fields...)
	}
	return output
}

func mergeValues(base map[string]string, overlay map[string]string) map[string]string {
	out := cloneValues(base)
	for key, value := range overlay {
		out[key] = value
	}
	return out
}

func evalComputedValues(computeds []clientlang.Computed, values map[string]string) (map[string]string, map[string]any, error) {
	if len(computeds) == 0 {
		return nil, nil, nil
	}
	stringsOut := map[string]string{}
	valuesOut := map[string]any{}
	scope := cloneValues(values)
	for _, computed := range computeds {
		value, err := clientlang.EvalValue(computed.Expr, scope)
		if err != nil {
			return nil, nil, fmt.Errorf("computed %s: %w", computed.Name, err)
		}
		scalar, ok := scalarString(value)
		if !ok {
			return nil, nil, fmt.Errorf("computed %s must evaluate to a scalar value", computed.Name)
		}
		stringsOut[computed.Name] = scalar
		valuesOut[computed.Name] = value
		scope[computed.Name] = scalar
	}
	return stringsOut, valuesOut, nil
}

func componentStateJSON(stateJSON string, props map[string]any, computed map[string]any) (string, error) {
	if stateJSON == "" && len(props) == 0 && len(computed) == 0 {
		return "", nil
	}
	values := map[string]any{}
	if stateJSON != "" {
		if err := json.Unmarshal([]byte(stateJSON), &values); err != nil {
			return "", fmt.Errorf("decode component state JSON: %w", err)
		}
	}
	for key, value := range props {
		values[key] = value
	}
	for key, value := range computed {
		values[key] = value
	}
	payload, err := json.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("encode component state JSON: %w", err)
	}
	return string(payload), nil
}

func scalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case nil:
		return "", true
	case string:
		return typed, true
	case bool:
		return strconv.FormatBool(typed), true
	case int:
		return strconv.Itoa(typed), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case json.Number:
		return typed.String(), true
	default:
		return "", false
	}
}

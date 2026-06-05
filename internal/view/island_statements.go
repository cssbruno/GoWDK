package view

import (
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"strings"
)

func validateAwaitFetchAssignment(statement string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction) error {
	match := islandAssignPattern.FindStringSubmatch(strings.TrimSpace(statement))
	if match == nil {
		return fmt.Errorf("await fetchJSON must assign to a state field")
	}
	left := strings.TrimSpace(match[1])
	leftType, err := validateIslandSymbol(left, writeSymbols)
	if err != nil {
		return err
	}
	right := strings.TrimSpace(match[2])
	fetch := islandAwaitFetchPattern.FindStringSubmatch(right)
	if fetch == nil {
		return fmt.Errorf("await supports only fetchJSON[T](urlExpr)")
	}
	fetchType := clientlang.NormalizeType(strings.TrimSpace(fetch[1]))
	if fetchType != clientlang.TypeUnknown && leftType != clientlang.TypeUnknown && fetchType != leftType && !compatibleNumericType(fetchType, leftType) {
		return fmt.Errorf("cannot assign fetched %s value to %s field %q", fetchType, leftType, left)
	}
	urlType, _, err := clientlang.CheckExprWithFunctions(strings.TrimSpace(fetch[2]), readSymbols, helpers)
	if err != nil {
		return fmt.Errorf("fetchJSON url: %w", err)
	}
	if urlType != clientlang.TypeString && urlType != clientlang.TypeUnknown {
		return fmt.Errorf("fetchJSON url must be string, got %s", urlType)
	}
	return nil
}

type letStatement struct {
	Name string
	Type string
	Expr string
}

func parseLetStatement(statement string) (letStatement, bool, error) {
	match := islandLetPattern.FindStringSubmatch(strings.TrimSpace(statement))
	if match == nil {
		if strings.HasPrefix(strings.TrimSpace(statement), "let ") {
			return letStatement{}, false, fmt.Errorf("let statement must use `let name type = expr`")
		}
		return letStatement{}, false, nil
	}
	return letStatement{Name: match[1], Type: match[2], Expr: strings.TrimSpace(match[3])}, true, nil
}

func isSupportedLocalType(typ clientlang.ValueType) bool {
	switch typ {
	case clientlang.TypeString, clientlang.TypeInt, clientlang.TypeFloat, clientlang.TypeBool:
		return true
	default:
		return false
	}
}

func mergeClientSymbols(left, right map[string]clientlang.ValueType) map[string]clientlang.ValueType {
	output := map[string]clientlang.ValueType{}
	for key, value := range left {
		output[key] = value
	}
	for key, value := range right {
		output[key] = value
	}
	return output
}

// IslandRefStatement reports whether expr is a safe DOM ref method call.
func IslandRefStatement(expr string) (string, bool) {
	match := islandRefCallPattern.FindStringSubmatch(strings.TrimSpace(expr))
	if match == nil {
		return "", false
	}
	return match[1], true
}

// IslandExpressionFields returns field references in a supported island event
// expression.
func IslandExpressionFields(expr string) []string {
	expr = strings.TrimSpace(expr)
	if call, ok := clientlang.ParseCall(expr); ok {
		if isArrayMutationCall(call.Name) {
			return arrayMutationFields(call)
		}
		return islandCallFields(call)
	}
	seen := map[string]bool{}
	add := func(name string) {
		if name != "" {
			seen[name] = true
		}
	}
	if match := islandIncDecPattern.FindStringSubmatch(expr); match != nil {
		add(match[1])
		return sortedKeys(seen)
	}
	if islandFieldPattern.MatchString(expr) {
		add(expr)
		return sortedKeys(seen)
	}
	if match := islandAssignPattern.FindStringSubmatch(expr); match != nil {
		add(strings.TrimSpace(match[1]))
		right := strings.TrimSpace(match[2])
		if fields, err := clientlang.ExprFields(right); err == nil {
			for _, field := range fields {
				add(field)
			}
		}
	}
	return sortedKeys(seen)
}

func isArrayMutationCall(name string) bool {
	switch name {
	case "append", "remove", "move":
		return true
	default:
		return false
	}
}

func validateArrayMutationCall(call clientlang.Call, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType) error {
	return validateArrayMutationCallWithFunctions(call, writeSymbols, readSymbols, nil)
}

func validateArrayMutationCallWithFunctions(call clientlang.Call, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction) error {
	switch call.Name {
	case "append":
		if len(call.Args) != 2 {
			return fmt.Errorf("append expects 2 arguments, got %d", len(call.Args))
		}
		field := strings.TrimSpace(call.Args[0])
		if typ, err := validateIslandSymbol(field, writeSymbols); err != nil {
			return err
		} else if typ != clientlang.TypeArray && typ != clientlang.TypeUnknown {
			return fmt.Errorf("append target %q must be array, got %s", field, typ)
		}
		itemFields, err := parseObjectLiteral(call.Args[1])
		if err != nil {
			return fmt.Errorf("append item: %w", err)
		}
		itemSymbols := itemFieldSymbols(field, readSymbols)
		for name, expr := range itemFields {
			expected, ok := itemSymbols[name]
			if !ok {
				return fmt.Errorf("append item has unknown field %q", name)
			}
			actual, _, err := clientlang.CheckExprWithFunctions(expr, readSymbols, helpers)
			if err != nil {
				return fmt.Errorf("append item field %s: %w", name, err)
			}
			if expected == clientlang.TypeArray || expected == clientlang.TypeObject {
				return fmt.Errorf("append item field %s must be scalar", name)
			}
			if actual == clientlang.TypeArray || actual == clientlang.TypeObject {
				return fmt.Errorf("append item field %s cannot use %s expression", name, actual)
			}
			if expected != clientlang.TypeUnknown && actual != clientlang.TypeUnknown && expected != actual && !compatibleNumericType(actual, expected) {
				return fmt.Errorf("append item field %s expects %s, got %s", name, expected, actual)
			}
		}
		return nil
	case "remove":
		if len(call.Args) != 2 {
			return fmt.Errorf("remove expects 2 arguments, got %d", len(call.Args))
		}
		field := strings.TrimSpace(call.Args[0])
		if typ, err := validateIslandSymbol(field, writeSymbols); err != nil {
			return err
		} else if typ != clientlang.TypeArray && typ != clientlang.TypeUnknown {
			return fmt.Errorf("remove target %q must be array, got %s", field, typ)
		}
		return validateArrayIndexExprWithFunctions("remove", call.Args[1], readSymbols, helpers)
	case "move":
		if len(call.Args) != 3 {
			return fmt.Errorf("move expects 3 arguments, got %d", len(call.Args))
		}
		field := strings.TrimSpace(call.Args[0])
		if typ, err := validateIslandSymbol(field, writeSymbols); err != nil {
			return err
		} else if typ != clientlang.TypeArray && typ != clientlang.TypeUnknown {
			return fmt.Errorf("move target %q must be array, got %s", field, typ)
		}
		if err := validateArrayIndexExprWithFunctions("move", call.Args[1], readSymbols, helpers); err != nil {
			return err
		}
		return validateArrayIndexExprWithFunctions("move", call.Args[2], readSymbols, helpers)
	default:
		return fmt.Errorf("unsupported array mutation %q", call.Name)
	}
}

func validateArrayIndexExpr(name, expr string, readSymbols map[string]clientlang.ValueType) error {
	return validateArrayIndexExprWithFunctions(name, expr, readSymbols, nil)
}

func validateArrayIndexExprWithFunctions(name, expr string, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction) error {
	typ, _, err := clientlang.CheckExprWithFunctions(expr, readSymbols, helpers)
	if err != nil {
		return fmt.Errorf("%s index: %w", name, err)
	}
	if typ != clientlang.TypeInt && typ != clientlang.TypeUnknown {
		return fmt.Errorf("%s index must be int, got %s", name, typ)
	}
	return nil
}

func itemFieldSymbols(arrayField string, symbols map[string]clientlang.ValueType) map[string]clientlang.ValueType {
	out := map[string]clientlang.ValueType{}
	prefix := arrayField + "[]."
	for name, typ := range symbols {
		if strings.HasPrefix(name, prefix) {
			out[strings.TrimPrefix(name, prefix)] = typ
		}
	}
	return out
}

func parseObjectLiteral(source string) (map[string]string, error) {
	source = strings.TrimSpace(source)
	if !strings.HasPrefix(source, "{") || !strings.HasSuffix(source, "}") {
		return nil, fmt.Errorf("must use { Field: expr }")
	}
	body := strings.TrimSpace(source[1 : len(source)-1])
	if body == "" {
		return nil, fmt.Errorf("must declare at least one field")
	}
	parts, err := splitTopLevelComma(body)
	if err != nil {
		return nil, err
	}
	fields := map[string]string{}
	for _, part := range parts {
		name, expr, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("field %q must use name: expr", part)
		}
		name = strings.TrimSpace(name)
		expr = strings.TrimSpace(expr)
		if !isIdentifier(name) {
			return nil, fmt.Errorf("invalid field name %q", name)
		}
		if expr == "" {
			return nil, fmt.Errorf("field %s has empty expression", name)
		}
		if _, exists := fields[name]; exists {
			return nil, fmt.Errorf("duplicate field %q", name)
		}
		fields[name] = expr
	}
	return fields, nil
}

func splitTopLevelComma(source string) ([]string, error) {
	var parts []string
	start := 0
	depth := 0
	inString := false
	escaped := false
	for index, char := range source {
		if escaped {
			escaped = false
			continue
		}
		if inString {
			switch char {
			case '\\':
				escaped = true
			case '"':
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced expression")
			}
		case ',':
			if depth > 0 {
				continue
			}
			part := strings.TrimSpace(source[start:index])
			if part == "" {
				return nil, fmt.Errorf("empty item")
			}
			parts = append(parts, part)
			start = index + 1
		}
	}
	if inString {
		return nil, fmt.Errorf("unterminated string")
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced expression")
	}
	part := strings.TrimSpace(source[start:])
	if part == "" {
		return nil, fmt.Errorf("empty item")
	}
	return append(parts, part), nil
}

func arrayMutationFields(call clientlang.Call) []string {
	seen := map[string]bool{}
	if len(call.Args) > 0 {
		field := strings.TrimSpace(call.Args[0])
		if field != "" {
			seen[field] = true
		}
	}
	for _, arg := range call.Args[1:] {
		if objectFields, err := parseObjectLiteral(arg); err == nil {
			for _, expr := range objectFields {
				if fields, err := clientlang.ExprFields(expr); err == nil {
					for _, field := range fields {
						seen[field] = true
					}
				}
			}
			continue
		}
		if fields, err := clientlang.ExprFields(arg); err == nil {
			for _, field := range fields {
				seen[field] = true
			}
		}
	}
	return sortedKeys(seen)
}

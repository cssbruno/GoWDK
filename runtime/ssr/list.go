package ssr

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"strings"

	gowdkhtml "github.com/cssbruno/gowdk/runtime/html"
)

// ListSpec describes one server-rendered g:for list. It is generated at build
// time from a request-time page view and consumed by RenderRegions at request
// time. A spec is a tree: Lists and Conds describe nested g:for lists and
// g:if conditionals whose data is resolved relative to each parent row.
type ListSpec struct {
	// Placeholder is the unique token embedded in the parent template that the
	// rendered rows replace.
	Placeholder string
	// SourcePath is the dotted path to the slice, resolved against the enclosing
	// container (the request-time load data at the top level, or a parent row
	// element when nested).
	SourcePath string
	// RowTemplate is the escaped HTML rendered once per slice element, still
	// containing this spec's Field, List, and Cond placeholders.
	RowTemplate string
	// Fields are the per-row scalar interpolations, each escaped at request time.
	Fields []ListField
	// Lists are nested g:for lists found inside RowTemplate.
	Lists []ListSpec
	// Conds are g:if conditionals found inside RowTemplate.
	Conds []CondSpec
}

// CondSpec describes one server-rendered g:if conditional. Its branch is
// rendered into the output only when SourcePath resolves to a truthy value
// (negated when Negate is set). The branch shares the enclosing container scope,
// so its fields and nested regions resolve against the same data as the
// conditional itself.
type CondSpec struct {
	Placeholder string
	// SourcePath is the dotted field path for a simple field/!field condition,
	// resolved against the enclosing container. Empty when Expr is set.
	SourcePath string
	Negate     bool
	// Expr is a full bool expression (comparisons, logic, literals) evaluated
	// against the enclosing container at request time. When set it takes
	// precedence over SourcePath/Negate; used for top-level server g:if.
	Expr     string
	Template string
	Fields   []ListField
	Lists    []ListSpec
	Conds    []CondSpec
}

// ListField is one per-row scalar substitution inside a region template.
type ListField struct {
	// Placeholder is the unique token inside the template replaced per render.
	Placeholder string
	// Path is the dotted path to the value (item-relative inside a row, or a
	// load field path inside a top-level region). Ignored when Index is true.
	Path string
	// Index is true when this field substitutes the zero-based row index.
	Index bool
	// URL is true when this field is substituted into a URL-bearing attribute.
	URL bool
}

// RenderRegions expands every top-level g:for list and g:if conditional in
// html by resolving their data from the request-time load data. Field values
// are always HTML-escaped, preserving GOWDK's escape-by-default contract for
// request-time server data.
func RenderRegions(html string, lists []ListSpec, conds []CondSpec, data map[string]any) string {
	return expandRegion(html, nil, lists, conds, data, -1)
}

// expandRegion substitutes a region template's fields, nested lists, and nested
// conditionals against a container value (the load data map at the top level, or
// a row element when nested). index is the current row index, or -1 outside a
// row.
func expandRegion(template string, fields []ListField, lists []ListSpec, conds []CondSpec, container any, index int) string {
	for _, field := range fields {
		template = strings.ReplaceAll(template, field.Placeholder, listFieldValue(field, container, index))
	}
	for _, list := range lists {
		slice, _ := elementSlice(container, list.SourcePath)
		template = strings.ReplaceAll(template, list.Placeholder, renderListRows(list, slice))
	}
	for _, cond := range conds {
		branch := ""
		if condHolds(container, cond) {
			branch = expandRegion(cond.Template, cond.Fields, cond.Lists, cond.Conds, container, index)
		}
		template = strings.ReplaceAll(template, cond.Placeholder, branch)
	}
	return template
}

func renderListRows(list ListSpec, slice []any) string {
	var builder strings.Builder
	for index, element := range slice {
		builder.WriteString(expandRegion(list.RowTemplate, list.Fields, list.Lists, list.Conds, element, index))
	}
	return builder.String()
}

func condHolds(container any, cond CondSpec) bool {
	if cond.Expr != "" {
		// A compound condition is evaluated against the request-time container.
		// Evaluation failure fails closed so malformed or missing data never
		// renders attacker-influenceable markup.
		result, err := evalBoolExpr(cond.Expr, container)
		return err == nil && result
	}
	value, ok := ElementPath(container, cond.SourcePath)
	show := ok && truthy(value)
	if cond.Negate {
		return !show
	}
	return show
}

func listFieldValue(field ListField, container any, index int) string {
	if field.Index {
		if field.URL {
			return gowdkhtml.EscapeURL(strconv.Itoa(index))
		}
		return gowdkhtml.Escape(strconv.Itoa(index))
	}
	value, ok := ElementPath(container, field.Path)
	if !ok || value == nil {
		return ""
	}
	if field.URL {
		return gowdkhtml.EscapeURL(fmt.Sprint(value))
	}
	return gowdkhtml.Escape(fmt.Sprint(value))
}

func evalBoolExpr(source string, container any) (bool, error) {
	expr, err := parser.ParseExpr(source)
	if err != nil {
		return false, err
	}
	value, err := evalExpr(expr, container)
	if err != nil {
		return false, err
	}
	result, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("SSR conditional %q does not evaluate to bool", source)
	}
	return result, nil
}

func evalExpr(expr ast.Expr, container any) (any, error) {
	switch typed := expr.(type) {
	case *ast.BasicLit:
		return evalBasicLit(typed)
	case *ast.Ident:
		switch typed.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "nil":
			return nilEvalValue(), nil
		default:
			value, ok := ElementPath(container, typed.Name)
			if !ok {
				return nil, fmt.Errorf("unknown SSR conditional field %q", typed.Name)
			}
			return normalizeEvalValue(value), nil
		}
	case *ast.SelectorExpr:
		value, err := evalExpr(typed.X, container)
		if err != nil {
			return nil, err
		}
		field, ok := loadPathValue(value, typed.Sel.Name)
		if !ok {
			return nil, fmt.Errorf("unknown SSR conditional field %q", typed.Sel.Name)
		}
		return normalizeEvalValue(field), nil
	case *ast.IndexExpr:
		value, err := evalExpr(typed.X, container)
		if err != nil {
			return nil, err
		}
		indexValue, err := evalExpr(typed.Index, container)
		if err != nil {
			return nil, err
		}
		index, ok := intValue(indexValue)
		if !ok {
			return nil, fmt.Errorf("SSR conditional index must be an integer")
		}
		items, ok := toAnySlice(value)
		if !ok || index < 0 || index >= len(items) {
			return nil, fmt.Errorf("SSR conditional index %d out of range", index)
		}
		return normalizeEvalValue(items[index]), nil
	case *ast.ParenExpr:
		return evalExpr(typed.X, container)
	case *ast.UnaryExpr:
		value, err := evalExpr(typed.X, container)
		if err != nil {
			return nil, err
		}
		switch typed.Op {
		case token.NOT:
			boolValue, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("operator ! requires bool")
			}
			return !boolValue, nil
		case token.SUB:
			number, ok := numericFloat(value)
			if !ok {
				return nil, fmt.Errorf("operator - requires number")
			}
			return -number, nil
		default:
			return nil, fmt.Errorf("unsupported unary operator %s", typed.Op)
		}
	case *ast.BinaryExpr:
		return evalBinaryExpr(typed, container)
	case *ast.CallExpr:
		return evalCallExpr(typed, container)
	default:
		return nil, fmt.Errorf("unsupported SSR conditional expression")
	}
}

func nilEvalValue() any {
	return nil
}

func evalBasicLit(expr *ast.BasicLit) (any, error) {
	switch expr.Kind {
	case token.STRING:
		return strconv.Unquote(expr.Value)
	case token.INT:
		return strconv.Atoi(expr.Value)
	case token.FLOAT:
		return strconv.ParseFloat(expr.Value, 64)
	default:
		return nil, fmt.Errorf("unsupported literal %q", expr.Value)
	}
}

func evalBinaryExpr(expr *ast.BinaryExpr, container any) (any, error) {
	if expr.Op == token.LAND || expr.Op == token.LOR {
		left, err := evalExpr(expr.X, container)
		if err != nil {
			return nil, err
		}
		leftBool, ok := left.(bool)
		if !ok {
			return nil, fmt.Errorf("operator %s requires bools", expr.Op)
		}
		if expr.Op == token.LAND && !leftBool {
			return false, nil
		}
		if expr.Op == token.LOR && leftBool {
			return true, nil
		}
		right, err := evalExpr(expr.Y, container)
		if err != nil {
			return nil, err
		}
		rightBool, ok := right.(bool)
		if !ok {
			return nil, fmt.Errorf("operator %s requires bools", expr.Op)
		}
		if expr.Op == token.LAND {
			return leftBool && rightBool, nil
		}
		return leftBool || rightBool, nil
	}

	left, err := evalExpr(expr.X, container)
	if err != nil {
		return nil, err
	}
	right, err := evalExpr(expr.Y, container)
	if err != nil {
		return nil, err
	}
	switch expr.Op {
	case token.ADD:
		if leftString, ok := left.(string); ok {
			rightString, ok := right.(string)
			if !ok {
				return nil, fmt.Errorf("operator + requires matching types")
			}
			return leftString + rightString, nil
		}
		return numericBinary(expr.Op, left, right)
	case token.SUB, token.MUL, token.QUO, token.REM:
		return numericBinary(expr.Op, left, right)
	case token.EQL:
		return valuesEqual(left, right), nil
	case token.NEQ:
		return !valuesEqual(left, right), nil
	case token.LSS, token.LEQ, token.GTR, token.GEQ:
		return compareValues(expr.Op, left, right)
	default:
		return nil, fmt.Errorf("unsupported binary operator %s", expr.Op)
	}
}

func evalCallExpr(expr *ast.CallExpr, container any) (any, error) {
	name, ok := expr.Fun.(*ast.Ident)
	if !ok {
		return nil, fmt.Errorf("unsupported SSR conditional call")
	}
	switch name.Name {
	case "len":
		if len(expr.Args) != 1 {
			return nil, fmt.Errorf("built-in len expects 1 argument")
		}
		value, err := evalExpr(expr.Args[0], container)
		if err != nil {
			return nil, err
		}
		if typed, ok := value.(string); ok {
			return len(typed), nil
		}
		reflected := reflect.ValueOf(value)
		if reflected.IsValid() && (reflected.Kind() == reflect.Slice || reflected.Kind() == reflect.Array || reflected.Kind() == reflect.Map) {
			return reflected.Len(), nil
		}
		return nil, fmt.Errorf("built-in len expects string, array, slice, or map")
	case "lower", "upper":
		if len(expr.Args) != 1 {
			return nil, fmt.Errorf("built-in %s expects 1 argument", name.Name)
		}
		value, err := evalExpr(expr.Args[0], container)
		if err != nil {
			return nil, err
		}
		text, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("built-in %s expects string", name.Name)
		}
		if name.Name == "lower" {
			return strings.ToLower(text), nil
		}
		return strings.ToUpper(text), nil
	case "contains":
		if len(expr.Args) != 2 {
			return nil, fmt.Errorf("built-in contains expects 2 arguments")
		}
		haystack, err := evalExpr(expr.Args[0], container)
		if err != nil {
			return nil, err
		}
		needle, err := evalExpr(expr.Args[1], container)
		if err != nil {
			return nil, err
		}
		haystackString, ok := haystack.(string)
		if !ok {
			return nil, fmt.Errorf("built-in contains expects string haystack")
		}
		needleString, ok := needle.(string)
		if !ok {
			return nil, fmt.Errorf("built-in contains expects string needle")
		}
		return strings.Contains(haystackString, needleString), nil
	case "string", "int", "float":
		if len(expr.Args) != 1 {
			return nil, fmt.Errorf("built-in %s expects 1 argument", name.Name)
		}
		value, err := evalExpr(expr.Args[0], container)
		if err != nil {
			return nil, err
		}
		return convertValue(name.Name, value)
	default:
		return nil, fmt.Errorf("unsupported SSR conditional helper %q", name.Name)
	}
}

func numericBinary(op token.Token, left, right any) (any, error) {
	leftNumber, leftOK := numericFloat(left)
	rightNumber, rightOK := numericFloat(right)
	if !leftOK || !rightOK {
		return nil, fmt.Errorf("operator %s requires numbers", op)
	}
	switch op {
	case token.ADD:
		return leftNumber + rightNumber, nil
	case token.SUB:
		return leftNumber - rightNumber, nil
	case token.MUL:
		return leftNumber * rightNumber, nil
	case token.QUO:
		return leftNumber / rightNumber, nil
	case token.REM:
		divisor := int(rightNumber)
		if divisor == 0 {
			return nil, fmt.Errorf("operator %% requires a non-zero divisor")
		}
		return float64(int(leftNumber) % divisor), nil
	default:
		return nil, fmt.Errorf("unsupported numeric operator %s", op)
	}
}

func compareValues(op token.Token, left, right any) (bool, error) {
	if leftString, ok := left.(string); ok {
		rightString, ok := right.(string)
		if !ok {
			return false, fmt.Errorf("operator %s requires matching types", op)
		}
		switch op {
		case token.LSS:
			return leftString < rightString, nil
		case token.LEQ:
			return leftString <= rightString, nil
		case token.GTR:
			return leftString > rightString, nil
		case token.GEQ:
			return leftString >= rightString, nil
		}
	}
	leftNumber, leftOK := numericFloat(left)
	rightNumber, rightOK := numericFloat(right)
	if !leftOK || !rightOK {
		return false, fmt.Errorf("operator %s requires numbers or strings", op)
	}
	switch op {
	case token.LSS:
		return leftNumber < rightNumber, nil
	case token.LEQ:
		return leftNumber <= rightNumber, nil
	case token.GTR:
		return leftNumber > rightNumber, nil
	case token.GEQ:
		return leftNumber >= rightNumber, nil
	default:
		return false, fmt.Errorf("unsupported comparison operator %s", op)
	}
}

func convertValue(kind string, value any) (any, error) {
	switch kind {
	case "string":
		switch typed := value.(type) {
		case nil:
			return "", nil
		case string:
			return typed, nil
		case bool:
			return strconv.FormatBool(typed), nil
		case int:
			return strconv.Itoa(typed), nil
		case float64:
			return strconv.FormatFloat(typed, 'f', -1, 64), nil
		case json.Number:
			return typed.String(), nil
		default:
			return fmt.Sprint(typed), nil
		}
	case "int":
		if number, ok := intValue(value); ok {
			return number, nil
		}
		if text, ok := value.(string); ok {
			return strconv.Atoi(text)
		}
	case "float":
		if number, ok := numericFloat(value); ok {
			return number, nil
		}
		if text, ok := value.(string); ok {
			return strconv.ParseFloat(text, 64)
		}
	}
	return nil, fmt.Errorf("cannot convert value to %s", kind)
}

func normalizeEvalValue(value any) any {
	if typed, ok := value.(json.Number); ok {
		return typed
	}
	reflected := reflect.ValueOf(value)
	if !reflected.IsValid() {
		return value
	}
	for reflected.Kind() == reflect.Interface || reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return nil
		}
		reflected = reflected.Elem()
	}
	return reflected.Interface()
}

func valuesEqual(left, right any) bool {
	leftNumber, leftOK := numericFloat(left)
	rightNumber, rightOK := numericFloat(right)
	if leftOK && rightOK {
		return leftNumber == rightNumber
	}
	return reflect.DeepEqual(left, right)
}

func intValue(value any) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8, int16, int32, int64:
		reflected := reflect.ValueOf(value)
		return int(reflected.Int()), true
	case uint, uint8, uint16, uint32, uint64:
		reflected := reflect.ValueOf(value)
		unsigned := reflected.Uint()
		if unsigned > uint64(^uint(0)>>1) {
			return 0, false
		}
		return int(unsigned), true
	case float64:
		position := int(typed)
		return position, float64(position) == typed
	case json.Number:
		parsed, err := strconv.Atoi(typed.String())
		return parsed, err == nil
	default:
		return 0, false
	}
}

func numericFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int8, int16, int32, int64:
		return float64(reflect.ValueOf(value).Int()), true
	case uint, uint8, uint16, uint32, uint64:
		return float64(reflect.ValueOf(value).Uint()), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func elementSlice(container any, path string) ([]any, bool) {
	value, ok := ElementPath(container, path)
	if !ok {
		return nil, false
	}
	return toAnySlice(value)
}

// truthy reports whether a request-time value should render its g:if branch.
// It mirrors common template semantics: false, "", 0, nil, and empty
// slices/maps are falsy; everything else is truthy.
func truthy(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	for reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface {
		if reflected.IsNil() {
			return false
		}
		reflected = reflected.Elem()
	}
	switch reflected.Kind() {
	case reflect.Bool:
		return reflected.Bool()
	case reflect.String:
		return reflected.String() != ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflected.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflected.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return reflected.Float() != 0
	case reflect.Slice, reflect.Array, reflect.Map:
		return reflected.Len() > 0
	default:
		return true
	}
}

// ElementPath resolves a dotted field path against an arbitrary value (map,
// struct, pointer, or interface), mirroring LoadPath's field-matching rules but
// rooted at a row element or the load data map.
func ElementPath(value any, path string) (any, bool) {
	if strings.TrimSpace(path) == "" {
		return value, true
	}
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if part == "" {
			return nil, false
		}
		next, ok := loadPathValue(value, part)
		if !ok {
			return nil, false
		}
		value = next
	}
	return value, true
}

func toAnySlice(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}
	if slice, ok := value.([]any); ok {
		return slice, true
	}
	reflected := reflect.ValueOf(value)
	for reflected.Kind() == reflect.Interface || reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return nil, false
		}
		reflected = reflected.Elem()
	}
	switch reflected.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, reflected.Len())
		for index := range out {
			out[index] = reflected.Index(index).Interface()
		}
		return out, true
	default:
		return nil, false
	}
}

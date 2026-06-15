package clientlang

import (
	"fmt"
	"strconv"
	"strings"
)

// ValidateIslandEventExpression validates a client event expression. When
// fields is non-nil, every referenced field must exist.
func ValidateIslandEventExpression(expr string, fields map[string]bool, handlers ...map[string]Handler) error {
	symbols := boolFieldSymbols(fields)
	return ValidateIslandEventExpressionTyped(expr, symbols, symbols, firstHandlerMap(handlers))
}

// ValidateIslandBoolExpression validates a g:if-style bool expression.
func ValidateIslandBoolExpression(expr string, fields map[string]bool) error {
	return ValidateIslandBoolExpressionTyped(expr, boolFieldSymbols(fields))
}

// ValidateIslandBoolExpressionTyped validates a g:if-style bool expression
// with scalar type information.
func ValidateIslandBoolExpressionTyped(expr string, symbols map[string]ValueType) error {
	typ, _, err := CheckExpr(expr, symbols)
	if err != nil {
		return err
	}
	if typ != TypeBool && typ != TypeUnknown {
		return fmt.Errorf("expression must be bool, got %s", typ)
	}
	return nil
}

// ValidateIslandEventExpressionTyped validates an event expression with scalar
// type information.
func ValidateIslandEventExpressionTyped(expr string, readSymbols map[string]ValueType, writeSymbols map[string]ValueType, handlers map[string]Handler) error {
	return ValidateIslandEventExpressionTypedWithFunctions(expr, readSymbols, writeSymbols, handlers, nil)
}

// ValidateIslandEventExpressionTypedWithFunctions validates an event expression
// with scalar type information and return-valued helper functions.
func ValidateIslandEventExpressionTypedWithFunctions(expr string, readSymbols map[string]ValueType, writeSymbols map[string]ValueType, handlers map[string]Handler, helpers map[string]ExprFunction) error {
	return ValidateIslandEventExpressionTypedWithEvents(expr, readSymbols, writeSymbols, handlers, helpers, nil)
}

// ValidateIslandEventExpressionTypedWithEvents validates an event expression,
// including component event dispatch statements.
func ValidateIslandEventExpressionTypedWithEvents(expr string, readSymbols map[string]ValueType, writeSymbols map[string]ValueType, handlers map[string]Handler, helpers map[string]ExprFunction, emits map[string]Emit) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("empty island event expression")
	}
	if emit, ok := ParseEmitCall(expr); ok {
		return validateEmitCall(emit, readSymbols, helpers, emits)
	}
	if call, ok := ParseCall(expr); ok {
		if isArrayMutationCall(call.Name) {
			return validateArrayMutationCallWithFunctions(call, writeSymbols, readSymbols, helpers)
		}
		if handlers == nil {
			return fmt.Errorf("unknown island client function %q", call.Name)
		}
		handler, exists := handlers[call.Name]
		if !exists {
			return fmt.Errorf("unknown island client function %q", call.Name)
		}
		if len(call.Args) != len(handler.Params) {
			return fmt.Errorf("island client function %s expects %d arguments, got %d", call.Name, len(handler.Params), len(call.Args))
		}
		for index, arg := range call.Args {
			typ, _, err := CheckExprWithFunctions(arg, readSymbols, helpers)
			if err != nil {
				return err
			}
			expected := handlerParamType(handler, index)
			if expected != TypeUnknown && typ != expected && !compatibleIslandNumericType(typ, expected) {
				return fmt.Errorf("island client function %s argument %d expects %s, got %s", call.Name, index+1, expected, typ)
			}
		}
		return nil
	}
	return ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, helpers)
}

func validateEmitCall(call EmitCall, readSymbols map[string]ValueType, helpers map[string]ExprFunction, emits map[string]Emit) error {
	if emits == nil {
		return fmt.Errorf("unknown component event %q", call.Name)
	}
	event, exists := emits[call.Name]
	if !exists {
		return fmt.Errorf("unknown component event %q", call.Name)
	}
	if len(call.Args) != len(event.Params) {
		return fmt.Errorf("component event %s expects %d arguments, got %d", call.Name, len(event.Params), len(call.Args))
	}
	for index, arg := range call.Args {
		typ, _, err := CheckExprWithFunctions(arg, readSymbols, helpers)
		if err != nil {
			return err
		}
		expected := TypeUnknown
		if index < len(event.ParamTypes) {
			expected = event.ParamTypes[index]
		}
		if expected != TypeUnknown && typ != expected && !compatibleIslandNumericType(typ, expected) {
			return fmt.Errorf("component event %s argument %d expects %s, got %s", call.Name, index+1, expected, typ)
		}
	}
	return nil
}

// ValidateIslandStateStatement validates a client statement that may write only
// writeFields and may read readFields plus scalar literals.
func ValidateIslandStateStatement(expr string, writeFields map[string]bool, readFields map[string]bool) error {
	return ValidateIslandStateStatementTyped(expr, boolFieldSymbols(writeFields), boolFieldSymbols(readFields))
}

// ValidateIslandStateStatementTyped validates a state statement with scalar
// type information.
func ValidateIslandStateStatementTyped(expr string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType) error {
	return ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, nil)
}

// ValidateIslandStateStatementTypedWithFunctions validates a state statement
// with scalar type information and return-valued helper functions.
func ValidateIslandStateStatementTypedWithFunctions(expr string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, helpers map[string]ExprFunction) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("empty island event expression")
	}
	if call, ok := ParseCall(expr); ok {
		if isArrayMutationCall(call.Name) {
			return validateArrayMutationCallWithFunctions(call, writeSymbols, readSymbols, helpers)
		}
		return fmt.Errorf("unsupported island event expression %q", expr)
	}
	if name, op, ok := parseIslandIncDec(expr); ok {
		typ, err := validateIslandSymbol(name, writeSymbols)
		if err != nil {
			return err
		}
		if !compatibleIslandNumericType(typ, TypeInt) {
			return fmt.Errorf("operator %s requires numeric island field %q", op, name)
		}
		return nil
	}
	if isIdentifier(expr) {
		_, err := validateIslandSymbol(expr, readSymbols)
		return err
	}
	if left, right, ok := parseIslandAssign(expr); ok {
		leftType, err := validateIslandSymbol(left, writeSymbols)
		if err != nil {
			return err
		}
		if leftType == TypeObject || leftType == TypeArray {
			return fmt.Errorf("cannot assign to non-scalar island field %q", left)
		}
		rightType, _, err := CheckExprWithFunctions(right, readSymbols, helpers)
		if err != nil {
			return err
		}
		if rightType == TypeObject || rightType == TypeArray {
			return fmt.Errorf("cannot assign %s expression to island field %q", rightType, left)
		}
		if leftType != TypeUnknown && rightType != leftType && !compatibleIslandNumericType(rightType, leftType) {
			return fmt.Errorf("cannot assign %s expression to %s field %q", rightType, leftType, left)
		}
		return nil
	}
	return fmt.Errorf("unsupported island event expression %q", expr)
}

// ValidateIslandClientStatementTyped validates a client statement that may
// mutate state or call a safe DOM ref method.
func ValidateIslandClientStatementTyped(expr string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, refs map[string]Ref) error {
	return ValidateIslandClientStatementTypedWithFunctions(expr, writeSymbols, readSymbols, refs, nil)
}

// ValidateIslandClientStatementTypedWithFunctions validates a client statement
// that may mutate state, call a safe DOM ref method, or read helper functions.
func ValidateIslandClientStatementTypedWithFunctions(expr string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, refs map[string]Ref, helpers map[string]ExprFunction) error {
	return ValidateIslandClientStatementTypedWithEvents(expr, writeSymbols, readSymbols, refs, helpers, nil, nil)
}

// ValidateIslandClientStatementTypedWithEvents validates a client statement
// that may mutate state, call a safe DOM ref method, read helper functions,
// dispatch declared component events, or clear a used page store. stores is the
// set of store names the component declares with `use`; a `clear <store>`
// statement is rejected unless the store is in that set.
func ValidateIslandClientStatementTypedWithEvents(expr string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, refs map[string]Ref, helpers map[string]ExprFunction, emits map[string]Emit, stores map[string]bool) error {
	if refName, ok := IslandRefStatement(expr); ok {
		if refs == nil {
			return fmt.Errorf("unknown DOM ref %q", refName)
		}
		if _, exists := refs[refName]; !exists {
			return fmt.Errorf("unknown DOM ref %q", refName)
		}
		return nil
	}
	if store, ok := ParseClearStatement(expr); ok {
		if !stores[store] {
			return fmt.Errorf("clear references store %q, but this component does not `use` it", store)
		}
		return nil
	}
	if emit, ok := ParseEmitCall(expr); ok {
		return validateEmitCall(emit, readSymbols, helpers, emits)
	}
	return ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, helpers)
}

// ValidateIslandClientStatementsTyped validates an ordered client statement
// block. Local variables declared with let are visible only to later
// statements in the same block.
func ValidateIslandClientStatementsTyped(statements []string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, refs map[string]Ref) (map[string]bool, error) {
	return ValidateIslandClientStatementsTypedWithFunctions(statements, writeSymbols, readSymbols, refs, nil)
}

// ValidateIslandClientStatementsTypedWithFunctions validates an ordered client
// statement block. Local variables declared with let are visible only to later
// statements in the same block.
func ValidateIslandClientStatementsTypedWithFunctions(statements []string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, refs map[string]Ref, helpers map[string]ExprFunction) (map[string]bool, error) {
	return ValidateIslandClientStatementsTypedWithOptions(statements, writeSymbols, readSymbols, refs, helpers, false)
}

// ValidateIslandClientStatementsTypedWithOptions validates an ordered client
// statement block with the same local-variable rules as
// ValidateIslandClientStatementsTypedWithFunctions. Async blocks may use
// compiler-owned await expressions.
func ValidateIslandClientStatementsTypedWithOptions(statements []string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, refs map[string]Ref, helpers map[string]ExprFunction, async bool) (map[string]bool, error) {
	return ValidateIslandClientStatementsTypedWithEvents(statements, writeSymbols, readSymbols, refs, helpers, async, nil, nil)
}

// ValidateIslandClientStatementsTypedWithEvents validates an ordered client
// statement block with optional component event dispatch support. stores is the
// set of store names the component declares with `use`, used to validate
// `clear <store>` statements.
func ValidateIslandClientStatementsTypedWithEvents(statements []string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, refs map[string]Ref, helpers map[string]ExprFunction, async bool, emits map[string]Emit, stores map[string]bool) (map[string]bool, error) {
	locals := mergeClientSymbols(nil, readSymbols)
	usedRefs := map[string]bool{}
	for index, statement := range statements {
		if refName, ok := IslandRefStatement(statement); ok {
			usedRefs[refName] = true
		}
		if local, ok, err := parseIslandLetStatement(statement); err != nil {
			return usedRefs, StatementValidationError{Index: index, Err: err}
		} else if ok {
			if strings.Contains(local.Expr, "await ") {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("await is not supported in let statements")}
			}
			if _, exists := writeSymbols[local.Name]; exists {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q conflicts with a state field", local.Name)}
			}
			if _, exists := locals[local.Name]; exists {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q is already declared", local.Name)}
			}
			typ := NormalizeType(local.Type)
			if !isSupportedLocalType(typ) {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q uses unsupported type %q", local.Name, local.Type)}
			}
			actual, _, err := CheckExprWithFunctions(local.Expr, locals, helpers)
			if err != nil {
				return usedRefs, StatementValidationError{Index: index, Err: err}
			}
			if actual == TypeArray || actual == TypeObject {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q cannot use %s expression", local.Name, actual)}
			}
			if typ != TypeUnknown && actual != TypeUnknown && typ != actual && !compatibleIslandNumericType(actual, typ) {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q expects %s, got %s", local.Name, typ, actual)}
			}
			locals[local.Name] = typ
			continue
		}
		if strings.Contains(statement, "await ") {
			if !async {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("await is only supported inside async client functions")}
			}
			if err := validateAwaitFetchAssignment(statement, writeSymbols, locals, helpers); err != nil {
				return usedRefs, StatementValidationError{Index: index, Err: err}
			}
			continue
		}
		if err := ValidateIslandClientStatementTypedWithEvents(statement, writeSymbols, locals, refs, helpers, emits, stores); err != nil {
			return usedRefs, StatementValidationError{Index: index, Err: err}
		}
	}
	return usedRefs, nil
}

// StatementValidationError identifies the statement index that failed within a
// client statement block.
type StatementValidationError struct {
	Index int
	Err   error
}

func (err StatementValidationError) Error() string {
	if err.Err == nil {
		return ""
	}
	return err.Err.Error()
}

func (err StatementValidationError) Unwrap() error {
	return err.Err
}

// IslandRefStatement reports whether expr is a safe DOM ref method call.
func IslandRefStatement(expr string) (string, bool) {
	return parseIslandRefCall(strings.TrimSpace(expr))
}

// IslandExpressionFields returns field references in a supported island event
// expression.
func IslandExpressionFields(expr string) []string {
	expr = strings.TrimSpace(expr)
	if call, ok := ParseCall(expr); ok {
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
	if name, _, ok := parseIslandIncDec(expr); ok {
		add(name)
		return sortedStringKeys(seen)
	}
	if isIdentifier(expr) {
		add(expr)
		return sortedStringKeys(seen)
	}
	if left, right, ok := parseIslandAssign(expr); ok {
		add(left)
		if fields, err := ExprFields(right); err == nil {
			for _, field := range fields {
				add(field)
			}
		}
	}
	return sortedStringKeys(seen)
}

func validateAwaitFetchAssignment(statement string, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, helpers map[string]ExprFunction) error {
	left, right, ok := parseIslandAssign(strings.TrimSpace(statement))
	if !ok {
		return fmt.Errorf("await fetchJSON must assign to a state field")
	}
	leftType, err := validateIslandSymbol(left, writeSymbols)
	if err != nil {
		return err
	}
	fetchType, urlExpr, ok := parseIslandAwaitFetch(right)
	if !ok {
		return fmt.Errorf("await supports only fetchJSON[T](urlExpr)")
	}
	if fetchType != TypeUnknown && leftType != TypeUnknown && fetchType != leftType && !compatibleIslandNumericType(fetchType, leftType) {
		return fmt.Errorf("cannot assign fetched %s value to %s field %q", fetchType, leftType, left)
	}
	urlType, _, err := CheckExprWithFunctions(urlExpr, readSymbols, helpers)
	if err != nil {
		return fmt.Errorf("fetchJSON url: %w", err)
	}
	if urlType != TypeString && urlType != TypeUnknown {
		return fmt.Errorf("fetchJSON url must be string, got %s", urlType)
	}
	return nil
}

func parseIslandLetStatement(statement string) (letStatement, bool, error) {
	name, typ, expr, ok := parseIslandLet(strings.TrimSpace(statement))
	if !ok {
		if strings.HasPrefix(strings.TrimSpace(statement), "let ") {
			return letStatement{}, false, fmt.Errorf("let statement must use `let name type = expr`")
		}
		return letStatement{}, false, nil
	}
	return letStatement{Name: name, Type: typ, Expr: expr}, true, nil
}

func validateArrayMutationCallWithFunctions(call Call, writeSymbols map[string]ValueType, readSymbols map[string]ValueType, helpers map[string]ExprFunction) error {
	switch call.Name {
	case "append":
		if len(call.Args) != 2 {
			return fmt.Errorf("append expects 2 arguments, got %d", len(call.Args))
		}
		field := strings.TrimSpace(call.Args[0])
		if typ, err := validateIslandSymbol(field, writeSymbols); err != nil {
			return err
		} else if typ != TypeArray && typ != TypeUnknown {
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
			actual, _, err := CheckExprWithFunctions(expr, readSymbols, helpers)
			if err != nil {
				return fmt.Errorf("append item field %s: %w", name, err)
			}
			if expected == TypeArray || expected == TypeObject {
				return fmt.Errorf("append item field %s must be scalar", name)
			}
			if actual == TypeArray || actual == TypeObject {
				return fmt.Errorf("append item field %s cannot use %s expression", name, actual)
			}
			if expected != TypeUnknown && actual != TypeUnknown && expected != actual && !compatibleIslandNumericType(actual, expected) {
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
		} else if typ != TypeArray && typ != TypeUnknown {
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
		} else if typ != TypeArray && typ != TypeUnknown {
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

func validateArrayIndexExprWithFunctions(name, expr string, readSymbols map[string]ValueType, helpers map[string]ExprFunction) error {
	typ, _, err := CheckExprWithFunctions(expr, readSymbols, helpers)
	if err != nil {
		return fmt.Errorf("%s index: %w", name, err)
	}
	if typ != TypeInt && typ != TypeUnknown {
		return fmt.Errorf("%s index must be int, got %s", name, typ)
	}
	return nil
}

func validateIslandSymbol(field string, symbols map[string]ValueType) (ValueType, error) {
	if !isIdentifier(field) {
		return TypeUnknown, fmt.Errorf("invalid island field %q", field)
	}
	typ, ok := symbols[field]
	if symbols != nil && !ok {
		return TypeUnknown, fmt.Errorf("unknown island field %q", field)
	}
	return typ, nil
}

func boolFieldSymbols(fields map[string]bool) map[string]ValueType {
	if fields == nil {
		return nil
	}
	symbols := map[string]ValueType{}
	for field, ok := range fields {
		if ok {
			symbols[field] = TypeUnknown
		}
	}
	return symbols
}

func firstHandlerMap(handlers []map[string]Handler) map[string]Handler {
	if len(handlers) == 0 {
		return nil
	}
	return handlers[0]
}

func handlerParamType(handler Handler, index int) ValueType {
	if index < 0 || index >= len(handler.ParamTypes) {
		return TypeUnknown
	}
	return handler.ParamTypes[index]
}

func compatibleIslandNumericType(actual, expected ValueType) bool {
	if actual == TypeUnknown || expected == TypeUnknown {
		return true
	}
	return (actual == TypeInt || actual == TypeFloat) &&
		(expected == TypeInt || expected == TypeFloat)
}

func isSupportedLocalType(typ ValueType) bool {
	switch typ {
	case TypeString, TypeInt, TypeFloat, TypeBool:
		return true
	default:
		return false
	}
}

func mergeClientSymbols(left, right map[string]ValueType) map[string]ValueType {
	output := map[string]ValueType{}
	for key, value := range left {
		output[key] = value
	}
	for key, value := range right {
		output[key] = value
	}
	return output
}

func isArrayMutationCall(name string) bool {
	switch name {
	case "append", "remove", "move":
		return true
	default:
		return false
	}
}

func itemFieldSymbols(arrayField string, symbols map[string]ValueType) map[string]ValueType {
	out := map[string]ValueType{}
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
	parts, err := splitCommaList(body)
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

func islandCallFields(call Call) []string {
	seen := map[string]bool{}
	for _, arg := range call.Args {
		arg = strings.TrimSpace(arg)
		if isIdentifier(arg) && !isIslandScalarLiteral(arg) {
			seen[arg] = true
		}
	}
	return sortedStringKeys(seen)
}

func arrayMutationFields(call Call) []string {
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
				if fields, err := ExprFields(expr); err == nil {
					for _, field := range fields {
						seen[field] = true
					}
				}
			}
			continue
		}
		if fields, err := ExprFields(arg); err == nil {
			for _, field := range fields {
				seen[field] = true
			}
		}
	}
	return sortedStringKeys(seen)
}

func isIslandScalarLiteral(value string) bool {
	if value == "true" || value == "false" || value == "null" {
		return true
	}
	if isIslandNumber(value) {
		return true
	}
	if strings.HasPrefix(value, `"`) {
		_, err := strconv.Unquote(value)
		return err == nil
	}
	return false
}

func isIslandNumber(source string) bool {
	if source == "" {
		return false
	}
	cursor := 0
	if source[cursor] == '-' {
		cursor++
		if cursor == len(source) {
			return false
		}
	}
	digits := 0
	for cursor < len(source) && source[cursor] >= '0' && source[cursor] <= '9' {
		cursor++
		digits++
	}
	if digits == 0 {
		return false
	}
	if cursor == len(source) {
		return true
	}
	if source[cursor] != '.' {
		return false
	}
	cursor++
	fractionDigits := 0
	for cursor < len(source) && source[cursor] >= '0' && source[cursor] <= '9' {
		cursor++
		fractionDigits++
	}
	return fractionDigits > 0 && cursor == len(source)
}

func parseIslandIncDec(source string) (string, string, bool) {
	for _, operator := range []string{"++", "--"} {
		name, ok := strings.CutSuffix(source, operator)
		if ok && isIdentifier(name) {
			return name, operator, true
		}
	}
	return "", "", false
}

func parseIslandAssign(source string) (string, string, bool) {
	if source == "" || !isIdentStart(rune(source[0])) {
		return "", "", false
	}
	cursor := 1
	for cursor < len(source) && isIdentPart(rune(source[cursor])) {
		cursor++
	}
	name := source[:cursor]
	for cursor < len(source) && isSpace(source[cursor]) {
		cursor++
	}
	if cursor >= len(source) || source[cursor] != '=' {
		return "", "", false
	}
	cursor++
	for cursor < len(source) && isSpace(source[cursor]) {
		cursor++
	}
	if cursor >= len(source) {
		return "", "", false
	}
	return name, strings.TrimSpace(source[cursor:]), true
}

func parseIslandRefCall(source string) (string, bool) {
	name, method, ok := strings.Cut(source, ".")
	if !ok || !isIdentifier(name) {
		return "", false
	}
	method, ok = strings.CutSuffix(method, "()")
	if !ok {
		return "", false
	}
	switch method {
	case "Focus", "Blur", "ScrollIntoView":
		return name, true
	default:
		return "", false
	}
}

func parseIslandLet(source string) (string, string, string, bool) {
	rest, ok := strings.CutPrefix(source, "let")
	if !ok || rest == "" || !isSpace(rest[0]) {
		return "", "", "", false
	}
	rest = strings.TrimSpace(rest)
	name, rest, ok := nextIslandIdent(rest)
	if !ok {
		return "", "", "", false
	}
	rest = strings.TrimLeftFunc(rest, func(r rune) bool { return isSpaceRune(r) })
	typ, rest, ok := nextIslandIdent(rest)
	if !ok {
		return "", "", "", false
	}
	rest = strings.TrimLeftFunc(rest, func(r rune) bool { return isSpaceRune(r) })
	if !strings.HasPrefix(rest, "=") {
		return "", "", "", false
	}
	expr := strings.TrimSpace(rest[1:])
	if expr == "" {
		return "", "", "", false
	}
	return name, typ, expr, true
}

func parseIslandAwaitFetch(source string) (ValueType, string, bool) {
	rest, ok := strings.CutPrefix(source, "await")
	if !ok || rest == "" || !isSpace(rest[0]) {
		return TypeUnknown, "", false
	}
	rest = strings.TrimSpace(rest)
	rest, ok = strings.CutPrefix(rest, "fetchJSON[")
	if !ok {
		return TypeUnknown, "", false
	}
	closeType := strings.LastIndex(rest, "](")
	if closeType < 0 {
		return TypeUnknown, "", false
	}
	typ := rest[:closeType]
	args := rest[closeType+1:]
	if !strings.HasPrefix(args, "(") || !strings.HasSuffix(args, ")") || typ == "" {
		return TypeUnknown, "", false
	}
	return NormalizeType(strings.TrimSpace(typ)), strings.TrimSpace(args[1 : len(args)-1]), true
}

func nextIslandIdent(source string) (string, string, bool) {
	if source == "" || !isIdentStart(rune(source[0])) {
		return "", "", false
	}
	cursor := 1
	for cursor < len(source) && isIdentPart(rune(source[cursor])) {
		cursor++
	}
	return source[:cursor], source[cursor:], true
}

func isSpaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

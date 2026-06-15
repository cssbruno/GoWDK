package view

import (
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"strings"
)

// ValidateIslandEventExpression validates the first generated-JS event
// expression subset. When fields is non-nil, every referenced field must exist.
// Named client function calls are valid only when handlers declares that
// function.
func ValidateIslandEventExpression(expr string, fields map[string]bool, handlers ...map[string]clientlang.Handler) error {
	symbols := boolFieldSymbols(fields)
	return ValidateIslandEventExpressionTyped(expr, symbols, symbols, firstHandlerMap(handlers))
}

// ValidateIslandBoolExpression validates a g:if-style bool expression.
func ValidateIslandBoolExpression(expr string, fields map[string]bool) error {
	symbols := boolFieldSymbols(fields)
	typ, _, err := clientlang.CheckExpr(expr, symbols)
	if err != nil {
		return err
	}
	if typ != clientlang.TypeBool && typ != clientlang.TypeUnknown {
		return fmt.Errorf("expression must be bool, got %s", typ)
	}
	return nil
}

// ValidateIslandBoolExpressionTyped validates a g:if-style bool expression
// with scalar type information.
func ValidateIslandBoolExpressionTyped(expr string, symbols map[string]clientlang.ValueType) error {
	typ, _, err := clientlang.CheckExpr(expr, symbols)
	if err != nil {
		return err
	}
	if typ != clientlang.TypeBool && typ != clientlang.TypeUnknown {
		return fmt.Errorf("expression must be bool, got %s", typ)
	}
	return nil
}

// ValidateReactiveAttrExpressionTyped validates a first-slice reactive
// attribute expression.
func ValidateReactiveAttrExpressionTyped(name, expr string, symbols map[string]clientlang.ValueType) error {
	if unsafeReactiveAttr(name) {
		return fmt.Errorf("reactive attribute %q is not supported before safe URL/style/event rules are defined", name)
	}
	typ, _, err := clientlang.CheckExpr(expr, symbols)
	if err != nil {
		return err
	}
	if isBooleanHTMLAttr(name) && typ != clientlang.TypeBool && typ != clientlang.TypeUnknown {
		return fmt.Errorf("boolean attribute %q requires bool expression, got %s", name, typ)
	}
	return nil
}

// ValidateClassToggleExpressionTyped validates a class:name directive
// expression.
func ValidateClassToggleExpressionTyped(name, expr string, symbols map[string]clientlang.ValueType) error {
	if classToggleName(name) == "" {
		return fmt.Errorf("class toggle directive %q requires a class name", name)
	}
	return ValidateIslandBoolExpressionTyped(expr, symbols)
}

// ValidateStyleBindingExpressionTyped validates a style:name directive
// expression.
func ValidateStyleBindingExpressionTyped(name, expr string, symbols map[string]clientlang.ValueType) error {
	if _, err := parseStyleBindingAttr(name); err != nil {
		return err
	}
	typ, _, err := clientlang.CheckExpr(expr, symbols)
	if err != nil {
		return err
	}
	if typ == clientlang.TypeBool {
		return fmt.Errorf("style binding requires string or numeric expression, got %s", typ)
	}
	if typ == clientlang.TypeObject || typ == clientlang.TypeArray {
		return fmt.Errorf("style binding requires string or numeric expression, got %s", typ)
	}
	return nil
}

// ValidateIslandEventExpressionTyped validates an event expression with scalar
// type information.
func ValidateIslandEventExpressionTyped(expr string, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler) error {
	return ValidateIslandEventExpressionTypedWithFunctions(expr, readSymbols, writeSymbols, handlers, nil)
}

// ValidateIslandEventExpressionTypedWithFunctions validates an event expression
// with scalar type information and return-valued helper functions.
func ValidateIslandEventExpressionTypedWithFunctions(expr string, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction) error {
	return ValidateIslandEventExpressionTypedWithEvents(expr, readSymbols, writeSymbols, handlers, helpers, nil)
}

// ValidateIslandEventExpressionTypedWithEvents validates an event expression,
// including component event dispatch statements.
func ValidateIslandEventExpressionTypedWithEvents(expr string, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, emits map[string]clientlang.Emit) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("empty island event expression")
	}
	if emit, ok := clientlang.ParseEmitCall(expr); ok {
		return validateEmitCall(emit, readSymbols, helpers, emits)
	}
	if call, ok := clientlang.ParseCall(expr); ok {
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
			typ, _, err := clientlang.CheckExprWithFunctions(arg, readSymbols, helpers)
			if err != nil {
				return err
			}
			expected := handlerParamType(handler, index)
			if expected != clientlang.TypeUnknown && typ != expected && !compatibleNumericType(typ, expected) {
				return fmt.Errorf("island client function %s argument %d expects %s, got %s", call.Name, index+1, expected, typ)
			}
		}
		return nil
	}
	return ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, helpers)
}

func validateEmitCall(call clientlang.EmitCall, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction, emits map[string]clientlang.Emit) error {
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
		typ, _, err := clientlang.CheckExprWithFunctions(arg, readSymbols, helpers)
		if err != nil {
			return err
		}
		expected := clientlang.TypeUnknown
		if index < len(event.ParamTypes) {
			expected = event.ParamTypes[index]
		}
		if expected != clientlang.TypeUnknown && typ != expected && !compatibleNumericType(typ, expected) {
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
func ValidateIslandStateStatementTyped(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType) error {
	return ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, nil)
}

// ValidateIslandStateStatementTypedWithFunctions validates a state statement
// with scalar type information and return-valued helper functions.
func ValidateIslandStateStatementTypedWithFunctions(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return fmt.Errorf("empty island event expression")
	}
	if call, ok := clientlang.ParseCall(expr); ok {
		if isArrayMutationCall(call.Name) {
			return validateArrayMutationCallWithFunctions(call, writeSymbols, readSymbols, helpers)
		}
		return fmt.Errorf("unsupported island event expression %q", expr)
	}
	if match := islandIncDecPattern.FindStringSubmatch(expr); match != nil {
		typ, err := validateIslandSymbol(match[1], writeSymbols)
		if err != nil {
			return err
		}
		if !compatibleNumericType(typ, clientlang.TypeInt) {
			return fmt.Errorf("operator %s requires numeric island field %q", match[2], match[1])
		}
		return nil
	}
	if islandFieldPattern.MatchString(expr) {
		_, err := validateIslandSymbol(expr, readSymbols)
		return err
	}
	if match := islandAssignPattern.FindStringSubmatch(expr); match != nil {
		left := strings.TrimSpace(match[1])
		leftType, err := validateIslandSymbol(left, writeSymbols)
		if err != nil {
			return err
		}
		if leftType == clientlang.TypeObject || leftType == clientlang.TypeArray {
			return fmt.Errorf("cannot assign to non-scalar island field %q", left)
		}
		right := strings.TrimSpace(match[2])
		rightType, _, err := clientlang.CheckExprWithFunctions(right, readSymbols, helpers)
		if err != nil {
			return err
		}
		if rightType == clientlang.TypeObject || rightType == clientlang.TypeArray {
			return fmt.Errorf("cannot assign %s expression to island field %q", rightType, left)
		}
		if leftType != clientlang.TypeUnknown && rightType != leftType && !compatibleNumericType(rightType, leftType) {
			return fmt.Errorf("cannot assign %s expression to %s field %q", rightType, leftType, left)
		}
		return nil
	}
	return fmt.Errorf("unsupported island event expression %q", expr)
}

// ValidateIslandClientStatementTyped validates a client statement that may
// mutate state or call a safe DOM ref method.
func ValidateIslandClientStatementTyped(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref) error {
	return ValidateIslandClientStatementTypedWithFunctions(expr, writeSymbols, readSymbols, refs, nil)
}

// ValidateIslandClientStatementTypedWithFunctions validates a client statement
// that may mutate state, call a safe DOM ref method, or read helper functions.
func ValidateIslandClientStatementTypedWithFunctions(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction) error {
	return ValidateIslandClientStatementTypedWithEvents(expr, writeSymbols, readSymbols, refs, helpers, nil, nil)
}

// ValidateIslandClientStatementTypedWithEvents validates a client statement
// that may mutate state, call a safe DOM ref method, read helper functions,
// dispatch declared component events, or clear a used page store. stores is the
// set of store names the component declares with `use`; a `clear <store>`
// statement is rejected unless the store is in that set.
func ValidateIslandClientStatementTypedWithEvents(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction, emits map[string]clientlang.Emit, stores map[string]bool) error {
	if refName, ok := IslandRefStatement(expr); ok {
		if refs == nil {
			return fmt.Errorf("unknown DOM ref %q", refName)
		}
		if _, exists := refs[refName]; !exists {
			return fmt.Errorf("unknown DOM ref %q", refName)
		}
		return nil
	}
	if store, ok := clientlang.ParseClearStatement(expr); ok {
		if !stores[store] {
			return fmt.Errorf("clear references store %q, but this component does not `use` it", store)
		}
		return nil
	}
	if emit, ok := clientlang.ParseEmitCall(expr); ok {
		return validateEmitCall(emit, readSymbols, helpers, emits)
	}
	return ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, helpers)
}

// ValidateIslandClientStatementsTyped validates an ordered client statement
// block. Local variables declared with let are visible only to later
// statements in the same block.
func ValidateIslandClientStatementsTyped(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref) (map[string]bool, error) {
	return ValidateIslandClientStatementsTypedWithFunctions(statements, writeSymbols, readSymbols, refs, nil)
}

// ValidateIslandClientStatementsTypedWithFunctions validates an ordered client
// statement block. Local variables declared with let are visible only to later
// statements in the same block.
func ValidateIslandClientStatementsTypedWithFunctions(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction) (map[string]bool, error) {
	return ValidateIslandClientStatementsTypedWithOptions(statements, writeSymbols, readSymbols, refs, helpers, false)
}

// ValidateIslandClientStatementsTypedWithOptions validates an ordered client
// statement block with the same local-variable rules as
// ValidateIslandClientStatementsTypedWithFunctions. Async blocks may use
// compiler-owned await expressions.
func ValidateIslandClientStatementsTypedWithOptions(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction, async bool) (map[string]bool, error) {
	return ValidateIslandClientStatementsTypedWithEvents(statements, writeSymbols, readSymbols, refs, helpers, async, nil, nil)
}

// ValidateIslandClientStatementsTypedWithEvents validates an ordered client
// statement block with optional component event dispatch support. stores is the
// set of store names the component declares with `use`, used to validate
// `clear <store>` statements.
func ValidateIslandClientStatementsTypedWithEvents(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction, async bool, emits map[string]clientlang.Emit, stores map[string]bool) (map[string]bool, error) {
	locals := mergeClientSymbols(nil, readSymbols)
	usedRefs := map[string]bool{}
	for index, statement := range statements {
		if refName, ok := IslandRefStatement(statement); ok {
			usedRefs[refName] = true
		}
		if local, ok, err := parseLetStatement(statement); err != nil {
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
			typ := clientlang.NormalizeType(local.Type)
			if !isSupportedLocalType(typ) {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q uses unsupported type %q", local.Name, local.Type)}
			}
			actual, _, err := clientlang.CheckExprWithFunctions(local.Expr, locals, helpers)
			if err != nil {
				return usedRefs, StatementValidationError{Index: index, Err: err}
			}
			if actual == clientlang.TypeArray || actual == clientlang.TypeObject {
				return usedRefs, StatementValidationError{Index: index, Err: fmt.Errorf("local %q cannot use %s expression", local.Name, actual)}
			}
			if typ != clientlang.TypeUnknown && actual != clientlang.TypeUnknown && typ != actual && !compatibleNumericType(actual, typ) {
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

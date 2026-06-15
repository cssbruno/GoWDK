package view

import (
	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/viewvalidation"
)

// ValidateIslandEventExpression validates a client event expression. When
// fields is non-nil, every referenced field must exist.
func ValidateIslandEventExpression(expr string, fields map[string]bool, handlers ...map[string]clientlang.Handler) error {
	return clientlang.ValidateIslandEventExpression(expr, fields, handlers...)
}

// ValidateIslandBoolExpression validates a g:if-style bool expression.
func ValidateIslandBoolExpression(expr string, fields map[string]bool) error {
	return clientlang.ValidateIslandBoolExpression(expr, fields)
}

// ValidateIslandBoolExpressionTyped validates a g:if-style bool expression
// with scalar type information.
func ValidateIslandBoolExpressionTyped(expr string, symbols map[string]clientlang.ValueType) error {
	return clientlang.ValidateIslandBoolExpressionTyped(expr, symbols)
}

// ValidateReactiveAttrExpressionTyped validates a first-slice reactive
// attribute expression.
func ValidateReactiveAttrExpressionTyped(name, expr string, symbols map[string]clientlang.ValueType) error {
	return viewvalidation.ValidateReactiveAttrExpressionTyped(name, expr, symbols)
}

// ValidateClassToggleExpressionTyped validates a class:name directive
// expression.
func ValidateClassToggleExpressionTyped(name, expr string, symbols map[string]clientlang.ValueType) error {
	return viewvalidation.ValidateClassToggleExpressionTyped(name, expr, symbols)
}

// ValidateStyleBindingExpressionTyped validates a style:name directive
// expression.
func ValidateStyleBindingExpressionTyped(name, expr string, symbols map[string]clientlang.ValueType) error {
	return viewvalidation.ValidateStyleBindingExpressionTyped(name, expr, symbols)
}

// ValidateIslandEventExpressionTyped validates an event expression with scalar
// type information.
func ValidateIslandEventExpressionTyped(expr string, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler) error {
	return clientlang.ValidateIslandEventExpressionTyped(expr, readSymbols, writeSymbols, handlers)
}

// ValidateIslandEventExpressionTypedWithFunctions validates an event expression
// with scalar type information and return-valued helper functions.
func ValidateIslandEventExpressionTypedWithFunctions(expr string, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction) error {
	return clientlang.ValidateIslandEventExpressionTypedWithFunctions(expr, readSymbols, writeSymbols, handlers, helpers)
}

// ValidateIslandEventExpressionTypedWithEvents validates an event expression,
// including component event dispatch statements.
func ValidateIslandEventExpressionTypedWithEvents(expr string, readSymbols map[string]clientlang.ValueType, writeSymbols map[string]clientlang.ValueType, handlers map[string]clientlang.Handler, helpers map[string]clientlang.ExprFunction, emits map[string]clientlang.Emit) error {
	return clientlang.ValidateIslandEventExpressionTypedWithEvents(expr, readSymbols, writeSymbols, handlers, helpers, emits)
}

// ValidateIslandStateStatement validates a client statement that may write only
// writeFields and may read readFields plus scalar literals.
func ValidateIslandStateStatement(expr string, writeFields map[string]bool, readFields map[string]bool) error {
	return clientlang.ValidateIslandStateStatement(expr, writeFields, readFields)
}

// ValidateIslandStateStatementTyped validates a state statement with scalar
// type information.
func ValidateIslandStateStatementTyped(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType) error {
	return clientlang.ValidateIslandStateStatementTyped(expr, writeSymbols, readSymbols)
}

// ValidateIslandStateStatementTypedWithFunctions validates a state statement
// with scalar type information and return-valued helper functions.
func ValidateIslandStateStatementTypedWithFunctions(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, helpers map[string]clientlang.ExprFunction) error {
	return clientlang.ValidateIslandStateStatementTypedWithFunctions(expr, writeSymbols, readSymbols, helpers)
}

// ValidateIslandClientStatementTyped validates a client statement that may
// mutate state or call a safe DOM ref method.
func ValidateIslandClientStatementTyped(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref) error {
	return clientlang.ValidateIslandClientStatementTyped(expr, writeSymbols, readSymbols, refs)
}

// ValidateIslandClientStatementTypedWithFunctions validates a client statement
// that may mutate state, call a safe DOM ref method, or read helper functions.
func ValidateIslandClientStatementTypedWithFunctions(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction) error {
	return clientlang.ValidateIslandClientStatementTypedWithFunctions(expr, writeSymbols, readSymbols, refs, helpers)
}

// ValidateIslandClientStatementTypedWithEvents validates a client statement
// that may mutate state, call a safe DOM ref method, read helper functions, or
// dispatch declared component events.
func ValidateIslandClientStatementTypedWithEvents(expr string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction, emits map[string]clientlang.Emit) error {
	return clientlang.ValidateIslandClientStatementTypedWithEvents(expr, writeSymbols, readSymbols, refs, helpers, emits, nil)
}

// ValidateIslandClientStatementsTyped validates an ordered client statement
// block. Local variables declared with let are visible only to later
// statements in the same block.
func ValidateIslandClientStatementsTyped(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref) (map[string]bool, error) {
	return clientlang.ValidateIslandClientStatementsTyped(statements, writeSymbols, readSymbols, refs)
}

// ValidateIslandClientStatementsTypedWithFunctions validates an ordered client
// statement block. Local variables declared with let are visible only to later
// statements in the same block.
func ValidateIslandClientStatementsTypedWithFunctions(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction) (map[string]bool, error) {
	return clientlang.ValidateIslandClientStatementsTypedWithFunctions(statements, writeSymbols, readSymbols, refs, helpers)
}

// ValidateIslandClientStatementsTypedWithOptions validates an ordered client
// statement block with the same local-variable rules as
// ValidateIslandClientStatementsTypedWithFunctions. Async blocks may use
// compiler-owned await expressions.
func ValidateIslandClientStatementsTypedWithOptions(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction, async bool) (map[string]bool, error) {
	return clientlang.ValidateIslandClientStatementsTypedWithOptions(statements, writeSymbols, readSymbols, refs, helpers, async)
}

// ValidateIslandClientStatementsTypedWithEvents validates an ordered client
// statement block with optional component event dispatch support.
func ValidateIslandClientStatementsTypedWithEvents(statements []string, writeSymbols map[string]clientlang.ValueType, readSymbols map[string]clientlang.ValueType, refs map[string]clientlang.Ref, helpers map[string]clientlang.ExprFunction, async bool, emits map[string]clientlang.Emit) (map[string]bool, error) {
	return clientlang.ValidateIslandClientStatementsTypedWithEvents(statements, writeSymbols, readSymbols, refs, helpers, async, emits, nil)
}

// StatementValidationError identifies the statement index that failed within a
// client statement block.
type StatementValidationError = clientlang.StatementValidationError

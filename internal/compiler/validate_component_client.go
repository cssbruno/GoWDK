package compiler

import (
	"errors"
	"fmt"
	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/view"
	"strings"
)

func validateComponentClient(component manifest.Component, stateTypes map[string]clientlang.ValueType, symbolTypes map[string]clientlang.ValueType) (map[string]clientlang.Handler, map[string]clientlang.Helper, map[string]clientlang.Ref, map[string]bool, map[string]clientlang.ValueType, []ValidationError) {
	if !component.Blocks.Client && strings.TrimSpace(component.Blocks.ClientBody) == "" {
		return nil, nil, nil, nil, nil, nil
	}
	program, err := clientlang.Parse(component.Blocks.ClientBody)
	if err != nil {
		return nil, nil, nil, nil, nil, []ValidationError{{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          clientParseErrorSpan(component, err),
			Message:       fmt.Sprintf("component %s client block is invalid: %v", component.Name, err),
		}}
	}
	handlers := program.HandlerMap()
	helpers := program.HelperMap()
	helperFuncs := helperExprFunctions(helpers)
	emits := componentEmitMap(component)
	refs := program.RefMap()
	usedRefs := map[string]bool{}
	computedTypes := map[string]clientlang.ValueType{}
	var diagnostics []ValidationError
	readSymbols := mergeTypeSymbols(nil, symbolTypes)
	for _, computed := range program.Computed {
		if _, exists := symbolTypes[computed.Name]; exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s computed %q conflicts with a prop or state field", component.Name, computed.Name),
			})
			continue
		}
		if _, exists := computedTypes[computed.Name]; exists {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s computed %q is declared more than once", component.Name, computed.Name),
			})
			continue
		}
		declared := clientlang.NormalizeType(computed.Type)
		computedTypes[computed.Name] = declared
		readSymbols[computed.Name] = declared
	}
	orderedComputeds, err := program.OrderedComputed()
	if err != nil {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
			Message:       fmt.Sprintf("component %s computed dependency graph is invalid: %v", component.Name, err),
		})
	}
	for _, computed := range orderedComputeds {
		typ, _, err := clientlang.CheckExpr(computed.Expr, readSymbols)
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientExpressionErrorSpan(component, "return "+computed.Expr, computed.ExprSpan, err),
				Message:       fmt.Sprintf("component %s computed %s expression %q is invalid: %v", component.Name, computed.Name, computed.Expr, err),
			})
			continue
		}
		declared := clientlang.NormalizeType(computed.Type)
		if declared != clientlang.TypeUnknown && typ != clientlang.TypeUnknown && declared != typ && !compatibleNumericType(typ, declared) {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s computed %s returns %s, not %s", component.Name, computed.Name, typ, declared),
			})
			continue
		}
	}
	if err := validateHelperCallGraph(helpers); err != nil {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
			Message:       fmt.Sprintf("component %s helper call graph is invalid: %v", component.Name, err),
		})
	}
	for _, function := range program.Functions {
		if function.ReturnType == "" {
			continue
		}
		helper := helpers[function.Name]
		readFields := mergeTypeSymbols(nil, readSymbols)
		for _, param := range function.Params {
			readFields[param.Name] = clientlang.NormalizeType(param.Type)
		}
		actual, _, err := clientlang.CheckExprWithFunctions(helper.Return, readFields, helperFuncs)
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientExpressionErrorSpan(component, function.Statements[len(function.Statements)-1], functionReturnSpan(function), err),
				Message:       fmt.Sprintf("component %s helper function %s return expression %q is invalid: %v", component.Name, function.Name, helper.Return, err),
			})
			continue
		}
		declared := helper.ReturnType
		if actual == clientlang.TypeArray || actual == clientlang.TypeObject {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s helper function %s cannot return %s expression", component.Name, function.Name, actual),
			})
			continue
		}
		if declared != clientlang.TypeUnknown && actual != clientlang.TypeUnknown && declared != actual && !compatibleNumericType(actual, declared) {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s helper function %s returns %s, not %s", component.Name, function.Name, actual, declared),
			})
		}
	}
	for _, function := range program.Functions {
		if function.ReturnType != "" {
			continue
		}
		readFields := mergeTypeSymbols(nil, readSymbols)
		for _, param := range function.Params {
			readFields[param.Name] = clientlang.NormalizeType(param.Type)
		}
		functionRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(function.Statements, stateTypes, readFields, refs, helperFuncs, function.Async, emits)
		for refName := range functionRefs {
			usedRefs[refName] = true
		}
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientStatementErrorSpan(component, function.Statements, function.StatementSpans, err),
				Message:       fmt.Sprintf("component %s client function %s is invalid: %v", component.Name, function.Name, err),
			})
		}
	}
	mountRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(program.Mount, stateTypes, readSymbols, refs, helperFuncs, false, emits)
	for refName := range mountRefs {
		usedRefs[refName] = true
	}
	if err != nil {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          clientStatementErrorSpan(component, program.Mount, program.MountSpans, err),
			Message:       fmt.Sprintf("component %s mount block is invalid: %v", component.Name, err),
		})
	}
	destroyRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(program.Destroy, stateTypes, readSymbols, refs, helperFuncs, false, emits)
	for refName := range destroyRefs {
		usedRefs[refName] = true
	}
	if err != nil {
		diagnostics = append(diagnostics, ValidationError{
			Code:          "component_client_error",
			ComponentName: component.Name,
			Source:        component.Source,
			Span:          clientStatementErrorSpan(component, program.Destroy, program.DestroySpans, err),
			Message:       fmt.Sprintf("component %s destroy block is invalid: %v", component.Name, err),
		})
	}
	for _, effect := range program.Effects {
		if _, ok := stateTypes[effect.Field]; !ok {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          firstSpan(component.Blocks.Spans.Client, component.Span),
				Message:       fmt.Sprintf("component %s effect dependency %q must be a state field", component.Name, effect.Field),
			})
		}
		effectRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(effect.Statements, stateTypes, readSymbols, refs, helperFuncs, false, emits)
		for refName := range effectRefs {
			usedRefs[refName] = true
		}
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientStatementErrorSpan(component, effect.Statements, effect.StatementSpans, err),
				Message:       fmt.Sprintf("component %s effect block for %q is invalid: %v", component.Name, effect.Field, err),
			})
		}
		cleanupRefs, err := view.ValidateIslandClientStatementsTypedWithEvents(effect.Cleanup, stateTypes, readSymbols, refs, helperFuncs, false, emits)
		for refName := range cleanupRefs {
			usedRefs[refName] = true
		}
		if err != nil {
			diagnostics = append(diagnostics, ValidationError{
				Code:          "component_client_error",
				ComponentName: component.Name,
				Source:        component.Source,
				Span:          clientStatementErrorSpan(component, effect.Cleanup, effect.CleanupSpans, err),
				Message:       fmt.Sprintf("component %s effect cleanup for %q is invalid: %v", component.Name, effect.Field, err),
			})
		}
	}
	return handlers, helpers, refs, usedRefs, computedTypes, diagnostics
}

func componentEmitMap(component manifest.Component) map[string]clientlang.Emit {
	if len(component.Emits) == 0 {
		return nil
	}
	out := map[string]clientlang.Emit{}
	for _, event := range component.Emits {
		params := make([]string, 0, len(event.Params))
		paramTypes := make([]clientlang.ValueType, 0, len(event.Params))
		for _, param := range event.Params {
			params = append(params, param.Name)
			paramTypes = append(paramTypes, clientlang.NormalizeType(param.Type))
		}
		out[event.Name] = clientlang.Emit{Name: event.Name, Params: params, ParamTypes: paramTypes}
	}
	return out
}

func clientStatementErrorSpan(component manifest.Component, statements []string, spans []clientlang.Span, err error) source.SourceSpan {
	var statementErr view.StatementValidationError
	if errors.As(err, &statementErr) && statementErr.Index >= 0 && statementErr.Index < len(spans) {
		if statementErr.Index < len(statements) {
			return clientExpressionErrorSpan(component, statements[statementErr.Index], spans[statementErr.Index], statementErr.Err)
		}
		return clientSpan(component, spans[statementErr.Index])
	}
	return firstSpan(component.Blocks.Spans.Client, component.Span)
}

func clientParseErrorSpan(component manifest.Component, err error) source.SourceSpan {
	var parseErr *clientlang.ParseError
	if errors.As(err, &parseErr) && parseErr.Line > 0 {
		return clientSpan(component, clientlang.Span{StartLine: parseErr.Line, EndLine: parseErr.Line})
	}
	return firstSpan(component.Blocks.Spans.Client, component.Span)
}

func clientExpressionErrorSpan(component manifest.Component, statement string, span clientlang.Span, err error) source.SourceSpan {
	var exprErr clientlang.ExprValidationError
	if !errors.As(err, &exprErr) || exprErr.Span.StartColumn <= 0 {
		return clientSpan(component, span)
	}
	exprStart := expressionStartColumn(statement)
	if exprStart <= 0 {
		return clientSpan(component, span)
	}
	return clientSpanColumns(component, span, exprStart+exprErr.Span.StartColumn-1, exprStart+exprErr.Span.EndColumn-1)
}

func expressionStartColumn(statement string) int {
	trimmed := strings.TrimSpace(statement)
	if strings.HasPrefix(trimmed, "return ") {
		return strings.Index(statement, "return") + len("return") + 2
	}
	if index := strings.Index(statement, "="); index >= 0 {
		column := index + 2
		for column <= len(statement) && statement[column-1] == ' ' {
			column++
		}
		return column
	}
	return 0
}

func functionReturnSpan(function clientlang.Function) clientlang.Span {
	if len(function.StatementSpans) == 0 {
		return function.Span
	}
	return function.StatementSpans[len(function.StatementSpans)-1]
}

func clientSpan(component manifest.Component, span clientlang.Span) source.SourceSpan {
	return clientSpanColumns(component, span, 1, 2)
}

func clientSpanColumns(component manifest.Component, span clientlang.Span, startColumn, endColumn int) source.SourceSpan {
	if span.StartLine <= 0 {
		return firstSpan(component.Blocks.Spans.Client, component.Span)
	}
	base := component.Blocks.Spans.Client.Start.Line
	if base <= 0 {
		return firstSpan(component.Blocks.Spans.Client, component.Span)
	}
	startLine := base + span.StartLine
	endLine := base + span.EndLine
	if endLine < startLine {
		endLine = startLine
	}
	if startColumn <= 0 {
		startColumn = 1
	}
	if endColumn <= startColumn {
		endColumn = startColumn + 1
	}
	return source.SourceSpan{
		Start: source.SourcePosition{Line: startLine, Column: startColumn},
		End:   source.SourcePosition{Line: endLine, Column: endColumn},
	}
}

func mergeTypeSymbols(left, right map[string]clientlang.ValueType) map[string]clientlang.ValueType {
	output := map[string]clientlang.ValueType{}
	for key, value := range left {
		output[key] = value
	}
	for key, value := range right {
		output[key] = value
	}
	return output
}

func helperExprFunctions(helpers map[string]clientlang.Helper) map[string]clientlang.ExprFunction {
	if len(helpers) == 0 {
		return nil
	}
	out := map[string]clientlang.ExprFunction{}
	for name, helper := range helpers {
		out[name] = clientlang.ExprFunction{
			Params: append([]clientlang.ValueType(nil), helper.ParamTypes...),
			Return: helper.ReturnType,
		}
	}
	return out
}

func validateHelperCallGraph(helpers map[string]clientlang.Helper) error {
	if len(helpers) == 0 {
		return nil
	}
	graph := map[string][]string{}
	for name, helper := range helpers {
		calls, err := clientlang.ExprCalls(helper.Return)
		if err != nil {
			return fmt.Errorf("%s return expression: %w", name, err)
		}
		for _, call := range calls {
			if _, ok := helpers[call]; ok {
				graph[name] = append(graph[name], call)
			}
		}
	}
	state := map[string]int{}
	var stack []string
	var visit func(string) error
	visit = func(name string) error {
		switch state[name] {
		case 1:
			return fmt.Errorf("cycle %s", helperCycle(stack, name))
		case 2:
			return nil
		}
		state[name] = 1
		stack = append(stack, name)
		for _, next := range graph[name] {
			if err := visit(next); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = 2
		return nil
	}
	for name := range helpers {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}

func helperCycle(stack []string, repeated string) string {
	start := 0
	for index, name := range stack {
		if name == repeated {
			start = index
			break
		}
	}
	cycle := append([]string(nil), stack[start:]...)
	cycle = append(cycle, repeated)
	return strings.Join(cycle, " -> ")
}

func compatibleNumericType(actual, expected clientlang.ValueType) bool {
	if actual == clientlang.TypeUnknown || expected == clientlang.TypeUnknown {
		return true
	}
	return (actual == clientlang.TypeInt || actual == clientlang.TypeFloat) &&
		(expected == clientlang.TypeInt || expected == clientlang.TypeFloat)
}

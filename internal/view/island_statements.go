package view

import "github.com/cssbruno/gowdk/internal/clientlang"

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
	return clientlang.IslandRefStatement(expr)
}

// IslandExpressionFields returns field references in a supported island event
// expression.
func IslandExpressionFields(expr string) []string {
	return clientlang.IslandExpressionFields(expr)
}

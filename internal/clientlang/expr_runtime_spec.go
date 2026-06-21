package clientlang

import "encoding/json"

// ExpressionRuntimeSpec describes the client expression surface that the
// generated browser evaluator must mirror. Keep parser/operator additions here
// before changing the JavaScript runtime.
type ExpressionRuntimeSpec struct {
	Builtins          []ExpressionBuiltinSpec `json:"builtins"`
	UnaryOperators    []string                `json:"unaryOperators"`
	BinaryOperators   []string                `json:"binaryOperators"`
	EqualityOperators []string                `json:"equalityOperators"`
	CompareOperators  []string                `json:"compareOperators"`
	TermOperators     []string                `json:"termOperators"`
	FactorOperators   []string                `json:"factorOperators"`
	TokenOperators    []string                `json:"tokenOperators"`
	SwitchKeywords    []string                `json:"switchKeywords"`
}

// ExpressionBuiltinSpec describes one compiler-owned client expression builtin.
type ExpressionBuiltinSpec struct {
	Name string `json:"name"`
	Args int    `json:"args"`
}

// RuntimeExpressionSpec returns the Go-owned expression operator and builtin
// table used by the browser runtime.
func RuntimeExpressionSpec() ExpressionRuntimeSpec {
	return ExpressionRuntimeSpec{
		Builtins: []ExpressionBuiltinSpec{
			{Name: "len", Args: 1},
			{Name: "string", Args: 1},
			{Name: "lower", Args: 1},
			{Name: "upper", Args: 1},
			{Name: "contains", Args: 2},
			{Name: "int", Args: 1},
			{Name: "float", Args: 1},
		},
		UnaryOperators:    []string{"!", "-"},
		BinaryOperators:   []string{"==", "!=", "<", "<=", ">", ">=", "+", "-", "*", "/", "%", "&&", "||"},
		EqualityOperators: []string{"==", "!="},
		CompareOperators:  []string{"<", "<=", ">", ">="},
		TermOperators:     []string{"+", "-"},
		FactorOperators:   []string{"*", "/", "%"},
		TokenOperators:    []string{"==", "!=", "<=", ">=", "&&", "||"},
		SwitchKeywords:    []string{"switch", "match"},
	}
}

// RuntimeExpressionSpecJSON returns RuntimeExpressionSpec as compact JSON for
// direct embedding in generated JavaScript.
func RuntimeExpressionSpecJSON() string {
	payload, err := json.Marshal(RuntimeExpressionSpec())
	if err != nil {
		panic("marshal GOWDK client expression runtime spec: " + err.Error())
	}
	return string(payload)
}

package parser

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

// ParseComponent extracts component metadata and top-level block declarations.
func ParseComponent(src []byte) (gwdkir.Component, error) {
	ast, err := ParseSyntax(src)
	if err != nil {
		return gwdkir.Component{}, err
	}
	return lowerComponentSyntax(src, ast)
}

func supportedScalarType(value string) bool {
	return value == "string" || value == "int" || value == "float" || value == "bool"
}

func parseEmitParams(src string, lineNumber int, rawLine string) ([]gwdkir.EmitParam, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, nil
	}
	parts := strings.Split(src, ",")
	params := make([]gwdkir.EmitParam, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		item := strings.TrimSpace(part)
		fields := strings.Fields(item)
		if len(fields) != 2 {
			return nil, fmt.Errorf("line %d: emit parameter %q must use `name type`", lineNumber, item)
		}
		name, typ := fields[0], fields[1]
		if !identifierPattern.MatchString(name) {
			return nil, fmt.Errorf("line %d: invalid emit parameter name %q", lineNumber, name)
		}
		if !supportedScalarType(typ) {
			return nil, fmt.Errorf("line %d: emit parameter %s uses unsupported type %q", lineNumber, name, typ)
		}
		if seen[name] {
			return nil, fmt.Errorf("line %d: duplicate emit parameter %q", lineNumber, name)
		}
		seen[name] = true
		params = append(params, gwdkir.EmitParam{Name: name, Type: typ, Span: sourceLineSpan(lineNumber, rawLine)})
	}
	return params, nil
}

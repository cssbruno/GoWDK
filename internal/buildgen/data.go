package buildgen

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
)

func parsePathDeclarations(body string) ([]map[string]string, error) {
	return parseLiteralDeclarations(body, "paths", "path param")
}

func parsePathDeclarationsFromBlocks(blocks gwdkir.Blocks) ([]map[string]string, error) {
	if len(blocks.PathsRecords) == 0 {
		return parsePathDeclarations(blocks.PathsBody)
	}
	declarations := make([]map[string]string, 0, len(blocks.PathsRecords))
	for index, record := range blocks.PathsRecords {
		names := literalRecordFieldOrder(record)
		if len(names) == 0 {
			return nil, fmt.Errorf("paths line %d: literal declaration must include values", index+1)
		}
		params := make(map[string]string, len(names))
		for _, name := range names {
			if !isLiteralName(name) {
				return nil, fmt.Errorf("paths line %d: invalid path param name %q", index+1, name)
			}
			value, err := parsePathString(literalRecordExpression(record, name))
			if err != nil {
				return nil, fmt.Errorf("paths line %d: path param %s: %w", index+1, name, err)
			}
			params[name] = value
		}
		declarations = append(declarations, params)
	}
	return declarations, nil
}

func parsePathParams(source string) (map[string]string, error) {
	return parseLiteralStringMap(source, "path param")
}

func parseBuildData(body string, routeParams map[string]string, locale string, imports []gwdkir.Import, scripts []gwdkir.GoBlock, source string) (map[string]string, error) {
	lines := significantBuildLines(body)
	if len(lines) == 1 {
		call, ok, err := parseBuildDataCallLine(lines[0])
		if err != nil {
			return nil, err
		}
		if ok {
			return runBuildDataCallRef(call, imports, scripts, source, routeParams, locale)
		}
	}
	data := map[string]buildValue{}
	env := newBuildEnv(routeParams, data)
	declarations := 0
	for index, line := range lines {
		fields, ok, err := buildLiteralRecordFields(line)
		if err != nil {
			return nil, fmt.Errorf("build line %d: %w", index+1, err)
		}
		if !ok {
			return nil, fmt.Errorf("build line %d must use `=> { name: value }` or `=> BuildData()`", index+1)
		}
		declarations++
		if len(fields) == 0 && index == 0 {
			return nil, fmt.Errorf("build {} declaration must not be empty")
		}
		for _, field := range fields {
			key, value, err := buildFieldValueFromString(field.name, field.expr, env)
			if err != nil {
				return nil, fmt.Errorf("build line %d: %w", index+1, err)
			}
			if _, exists := data[key]; exists {
				return nil, fmt.Errorf("duplicate build field %q", key)
			}
			data[key] = value
		}
	}
	if declarations == 0 {
		return nil, nil
	}
	return buildValueStrings(data), nil
}

// buildLiteralField is one parsed "name: value" entry from a `=> { ... }` build
// declaration line, keeping the value in source form so build-time iteration
// syntax survives to the evaluator.
type buildLiteralField struct {
	name string
	expr string
}

// buildLiteralRecordFields splits a `=> { name: value, ... }` build line into its
// ordered fields. ok reports whether the line is a literal declaration at all;
// when ok is true an empty inner body yields no fields (an empty declaration the
// caller diagnoses).
func buildLiteralRecordFields(line string) ([]buildLiteralField, bool, error) {
	body, ok := strings.CutPrefix(strings.TrimSpace(line), "=>")
	if !ok {
		return nil, false, nil
	}
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "{") || !strings.HasSuffix(body, "}") {
		return nil, false, nil
	}
	elements, err := splitLiteralElements(strings.TrimSpace(body[1 : len(body)-1]))
	if err != nil {
		return nil, true, fmt.Errorf("build field must use name: value")
	}
	fields := make([]buildLiteralField, 0, len(elements))
	for _, element := range elements {
		colon := indexTopLevelByte(element, ':')
		if colon < 0 {
			return nil, true, fmt.Errorf("build field must use name: value")
		}
		name := strings.TrimSpace(element[:colon])
		expr := strings.TrimSpace(element[colon+1:])
		if !isLiteralName(name) {
			return nil, true, fmt.Errorf("invalid build field name %q", name)
		}
		if expr == "" {
			return nil, true, fmt.Errorf("build field %s: value must not be empty", name)
		}
		fields = append(fields, buildLiteralField{name: name, expr: expr})
	}
	return fields, true, nil
}

func parseBuildDataFromBlocks(blocks gwdkir.Blocks, routeParams map[string]string, locale string, imports []gwdkir.Import, source string) (map[string]string, error) {
	if blocks.BuildCall != nil {
		return runBuildDataCallRef(buildCallRef{Alias: blocks.BuildCall.Alias, Function: blocks.BuildCall.Function}, imports, blocks.GoBlocks, source, routeParams, locale)
	}
	if len(blocks.BuildRecords) == 0 {
		return parseBuildData(blocks.BuildBody, routeParams, locale, imports, blocks.GoBlocks, source)
	}
	data := map[string]buildValue{}
	env := newBuildEnv(routeParams, data)
	for index, record := range blocks.BuildRecords {
		names := literalRecordFieldOrder(record)
		if len(names) == 0 && index == 0 {
			return nil, fmt.Errorf("build {} declaration must not be empty")
		}
		for _, name := range names {
			valueExpr := literalRecordExpression(record, name)
			key, value, err := buildFieldValueFromString(name, valueExpr, env)
			if err != nil {
				return nil, fmt.Errorf("build line %d: %w", index+1, err)
			}
			if _, exists := data[key]; exists {
				return nil, fmt.Errorf("duplicate build field %q", key)
			}
			data[key] = value
		}
	}
	return buildValueStrings(data), nil
}

func literalRecordFieldOrder(record gwdkir.LiteralRecord) []string {
	if len(record.FieldOrder) > 0 {
		return append([]string(nil), record.FieldOrder...)
	}
	names := make([]string, 0, len(record.Expressions)+len(record.Fields))
	seen := map[string]bool{}
	for name := range record.Expressions {
		names = append(names, name)
		seen[name] = true
	}
	for name := range record.Fields {
		if seen[name] {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func literalRecordExpression(record gwdkir.LiteralRecord, name string) string {
	if record.Expressions != nil {
		if expr, ok := record.Expressions[name]; ok {
			return expr
		}
	}
	return strconv.Quote(record.Fields[name])
}

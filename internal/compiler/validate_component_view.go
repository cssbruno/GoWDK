package compiler

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/view"
)

var componentSimpleInterpolationPattern = regexp.MustCompile(`\{([A-Za-z_][A-Za-z0-9_]*)\}`)

type reactiveAttrExpr struct {
	Name       string
	Expression string
}

type eventExpr struct {
	Name       string
	Expression string
}

type classToggleExpr struct {
	Name       string
	Expression string
}

type styleBindingExpr struct {
	Name       string
	Expression string
}

type valueBindExpr struct {
	Field      string
	Element    string
	InputType  string
	InputValue string
}

type componentViewRefs struct {
	Fields        map[string]bool
	Events        []eventExpr
	Bools         []string
	Attrs         []reactiveAttrExpr
	ClassToggles  []classToggleExpr
	StyleBindings []styleBindingExpr
	ValueBinds    []valueBindExpr
	CheckedBinds  []string
	RefBinds      []string
}

func componentViewReferences(source string) (componentViewRefs, error) {
	nodes, err := view.Parse(source)
	if err != nil {
		return componentViewRefs{}, err
	}
	refs := componentViewRefs{Fields: map[string]bool{}}
	collectComponentViewReferences(nodes, &refs)
	return refs, nil
}

func validateInterpolations(value string, symbols map[string]clientlang.ValueType, messages *[]spannedMessage) {
	exprs, err := interpolationExpressions(value)
	if err != nil {
		*messages = append(*messages, spannedMessage{Message: err.Error()})
		return
	}
	for _, expr := range exprs {
		typ, _, err := clientlang.CheckExpr(expr, symbols)
		if err != nil {
			*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("interpolation %q is invalid: %v", expr, err)})
			continue
		}
		if typ == clientlang.TypeArray || typ == clientlang.TypeObject {
			*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("interpolation %q must be scalar, got %s", expr, typ)})
		}
	}
}

func interpolationExpressions(value string) ([]string, error) {
	var expressions []string
	for strings.Contains(value, "{") {
		start := strings.Index(value, "{")
		end := strings.Index(value[start:], "}")
		if end < 0 {
			return nil, fmt.Errorf("unterminated interpolation")
		}
		end += start
		expr := strings.TrimSpace(value[start+1 : end])
		if expr == "" {
			return nil, fmt.Errorf("empty interpolation")
		}
		expressions = append(expressions, expr)
		value = value[end+1:]
	}
	return expressions, nil
}

func loopSymbols(symbols map[string]clientlang.ValueType, loop view.ForDirective) map[string]clientlang.ValueType {
	out := mergeTypeSymbols(nil, symbols)
	itemType := out[loop.Collection+"[]"]
	if itemType == "" {
		itemType = clientlang.TypeObject
	}
	out[loop.Var] = itemType
	if loop.IndexVar != "" {
		out[loop.IndexVar] = clientlang.TypeInt
	}
	prefix := loop.Collection + "[]."
	for name, typ := range symbols {
		if strings.HasPrefix(name, prefix) {
			out[loop.Var+"."+strings.TrimPrefix(name, prefix)] = typ
		}
	}
	return out
}

func elementForDirective(element view.Element) (view.Attr, bool) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:for" {
			return attr, true
		}
	}
	return view.Attr{}, false
}

func elementKeyExpression(element view.Element) (string, bool) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:key" {
			return strings.TrimSpace(attr.Value), true
		}
	}
	return "", false
}

func collectComponentViewReferences(nodes []view.Node, refs *componentViewRefs) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case view.Text:
			collectSimpleInterpolations(typed.Value, refs.Fields)
		case view.Element:
			if loop, ok := elementForDirective(typed); ok {
				if parsed, err := view.ParseForDirective(loop.Value); err == nil {
					for _, field := range expressionFields(parsed.Collection) {
						refs.Fields[field] = true
					}
				}
				continue
			}
			for _, attr := range typed.Attrs {
				if strings.HasPrefix(attr.Name, "g:on:") {
					refs.Events = append(refs.Events, eventExpr{Name: attr.Name, Expression: strings.TrimSpace(attr.Value)})
					for _, field := range view.IslandExpressionFields(attr.Value) {
						if isDOMEventScopeField(field) {
							continue
						}
						refs.Fields[field] = true
					}
					continue
				}
				if attr.Name == "g:if" {
					expr := strings.TrimSpace(attr.Value)
					refs.Bools = append(refs.Bools, expr)
					for _, field := range expressionFields(expr) {
						refs.Fields[field] = true
					}
					continue
				}
				if attr.Name == "g:else-if" {
					expr := strings.TrimSpace(attr.Value)
					refs.Bools = append(refs.Bools, expr)
					for _, field := range expressionFields(expr) {
						refs.Fields[field] = true
					}
					continue
				}
				if attr.Name == "g:bind:value" {
					field := strings.TrimSpace(attr.Value)
					refs.ValueBinds = append(refs.ValueBinds, valueBindExpr{
						Field:      field,
						Element:    typed.Name,
						InputType:  literalAttrValue(typed.Attrs, "type"),
						InputValue: literalAttrValue(typed.Attrs, "value"),
					})
					refs.Fields[field] = true
					continue
				}
				if attr.Name == "g:bind:checked" {
					field := strings.TrimSpace(attr.Value)
					refs.CheckedBinds = append(refs.CheckedBinds, field)
					refs.Fields[field] = true
					continue
				}
				if attr.Name == "g:ref" {
					refs.RefBinds = append(refs.RefBinds, strings.TrimSpace(attr.Value))
					continue
				}
				if strings.HasPrefix(attr.Name, "g:") {
					continue
				}
				if strings.HasPrefix(attr.Name, "class:") {
					expr := expressionAttrSource(attr.Value)
					refs.ClassToggles = append(refs.ClassToggles, classToggleExpr{Name: attr.Name, Expression: expr})
					for _, field := range expressionFields(expr) {
						refs.Fields[field] = true
					}
					continue
				}
				if strings.HasPrefix(attr.Name, "style:") {
					expr := expressionAttrSource(attr.Value)
					refs.StyleBindings = append(refs.StyleBindings, styleBindingExpr{Name: attr.Name, Expression: expr})
					for _, field := range expressionFields(expr) {
						refs.Fields[field] = true
					}
					continue
				}
				if attr.Expression {
					expr := expressionAttrSource(attr.Value)
					refs.Attrs = append(refs.Attrs, reactiveAttrExpr{Name: attr.Name, Expression: expr})
					for _, field := range expressionFields(expr) {
						refs.Fields[field] = true
					}
					continue
				}
				collectSimpleInterpolations(attr.Value, refs.Fields)
			}
			collectComponentViewReferences(typed.Children, refs)
		case view.ComponentCall:
			for _, attr := range typed.Attrs {
				if strings.HasPrefix(attr.Name, "g:") {
					continue
				}
				collectSimpleInterpolations(attr.Value, refs.Fields)
			}
			collectComponentViewReferences(typed.Children, refs)
		}
	}
}

func isDOMEventScopeField(field string) bool {
	return field == "event" || strings.HasPrefix(field, "event.")
}

func expressionFields(expr string) []string {
	fields, err := clientlang.ExprFields(expr)
	if err != nil {
		return nil
	}
	return fields
}

func expressionAttrSource(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		return strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func isValueBindableElement(name string) bool {
	switch name {
	case "input", "textarea", "select":
		return true
	default:
		return false
	}
}

func literalAttrValue(attrs []view.Attr, name string) string {
	for _, attr := range attrs {
		if attr.Name == name && !attr.Boolean && !attr.Expression {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func collectSimpleInterpolations(value string, fields map[string]bool) {
	for _, match := range componentSimpleInterpolationPattern.FindAllStringSubmatch(value, -1) {
		fields[match[1]] = true
	}
}

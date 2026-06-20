package compiler

import (
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

type reactiveAttrExpr struct {
	Name       string
	Expression string
	Start      int
	End        int
}

type eventExpr struct {
	Name       string
	Expression string
	Start      int
	End        int
}

type classToggleExpr struct {
	Name       string
	Expression string
	Start      int
	End        int
}

type styleBindingExpr struct {
	Name       string
	Expression string
	Start      int
	End        int
}

type valueBindExpr struct {
	Field      string
	Element    string
	InputType  string
	InputValue string
	Start      int
	End        int
}

type fieldRef struct {
	Name  string
	Start int
	End   int
}

type stringRef struct {
	Value string
	Start int
	End   int
}

type componentViewRefs struct {
	Fields        map[string]bool
	FieldRefs     []fieldRef
	Events        []eventExpr
	Bools         []stringRef
	Attrs         []reactiveAttrExpr
	ClassToggles  []classToggleExpr
	StyleBindings []styleBindingExpr
	ValueBinds    []valueBindExpr
	CheckedBinds  []fieldRef
	RefBinds      []fieldRef
}

func componentViewReferences(source string) (componentViewRefs, error) {
	nodes, err := viewparse.Parse(source)
	if err != nil {
		return componentViewRefs{}, err
	}
	return componentViewReferencesFromNodes(source, nodes), nil
}

func componentViewReferencesFromNodes(source string, nodes []viewmodel.Node) componentViewRefs {
	refs := componentViewRefs{Fields: map[string]bool{}}
	collectComponentViewReferences(source, nodes, &refs)
	return refs
}

// validateInterpolations checks every {expr} in value. enclosing is the source
// span of the text node or attribute that holds value; it is stamped on each
// emitted diagnostic so loop-body interpolation errors point at that node
// instead of falling back to the whole view block.
func validateInterpolations(value string, symbols map[string]clientlang.ValueType, messages *[]spannedMessage, enclosing source.SourceSpan) {
	exprs, err := interpolationExpressions(value)
	if err != nil {
		*messages = append(*messages, spannedMessage{Message: err.Error(), Span: enclosing})
		return
	}
	for _, expr := range exprs {
		typ, _, err := clientlang.CheckExpr(expr, symbols)
		if err != nil {
			*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("interpolation %q is invalid: %v", expr, err), Span: enclosing})
			continue
		}
		if typ == clientlang.TypeArray || typ == clientlang.TypeObject {
			*messages = append(*messages, spannedMessage{Message: fmt.Sprintf("interpolation %q must be scalar, got %s", expr, typ), Span: enclosing})
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

func loopSymbols(symbols map[string]clientlang.ValueType, loop viewparse.ForDirective) map[string]clientlang.ValueType {
	out := mergeTypeSymbols(nil, symbols)
	itemType := out[loop.Collection+"[]"]
	if itemType == "" {
		itemType = clientlang.TypeObject
		if out[loop.Collection] == clientlang.TypeUnknown {
			itemType = clientlang.TypeUnknown
		}
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

func elementForDirective(element viewmodel.Element) (viewmodel.Attr, bool) {
	for _, attr := range element.Attrs {
		if attr.Name == "g:for" {
			return attr, true
		}
	}
	return viewmodel.Attr{}, false
}

func collectComponentViewReferences(source string, nodes []viewmodel.Node, refs *componentViewRefs) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Text:
			collectSimpleInterpolationRefs(typed.Value, typed.Start, refs)
		case viewmodel.Element:
			if loop, ok := elementForDirective(typed); ok {
				if parsed, err := viewparse.ParseForDirective(loop.Value); err == nil {
					for _, field := range expressionFields(parsed.Collection) {
						addFieldRef(refs, field, loop.Start, loop.End)
					}
				}
				continue
			}
			for _, attr := range typed.Attrs {
				if strings.HasPrefix(attr.Name, "g:on:") {
					exprStart, exprEnd := attrValueOffset(source, attr, strings.TrimSpace(attr.Value))
					refs.Events = append(refs.Events, eventExpr{Name: attr.Name, Expression: strings.TrimSpace(attr.Value), Start: exprStart, End: exprEnd})
					for _, field := range clientlang.IslandExpressionFields(attr.Value) {
						if isDOMEventScopeField(field) {
							continue
						}
						addFieldRef(refs, field, exprStart, exprEnd)
					}
					continue
				}
				if attr.Name == "g:if" {
					expr := strings.TrimSpace(attr.Value)
					exprStart, exprEnd := attrValueOffset(source, attr, expr)
					refs.Bools = append(refs.Bools, stringRef{Value: expr, Start: exprStart, End: exprEnd})
					for _, field := range expressionFields(expr) {
						addFieldRef(refs, field, exprStart, exprEnd)
					}
					continue
				}
				if attr.Name == "g:else-if" {
					expr := strings.TrimSpace(attr.Value)
					exprStart, exprEnd := attrValueOffset(source, attr, expr)
					refs.Bools = append(refs.Bools, stringRef{Value: expr, Start: exprStart, End: exprEnd})
					for _, field := range expressionFields(expr) {
						addFieldRef(refs, field, exprStart, exprEnd)
					}
					continue
				}
				if attr.Name == "g:bind:value" {
					field := strings.TrimSpace(attr.Value)
					fieldStart, fieldEnd := attrValueOffset(source, attr, field)
					refs.ValueBinds = append(refs.ValueBinds, valueBindExpr{
						Field:      field,
						Element:    typed.Name,
						InputType:  literalAttrValue(typed.Attrs, "type"),
						InputValue: literalAttrValue(typed.Attrs, "value"),
						Start:      fieldStart,
						End:        fieldEnd,
					})
					addFieldRef(refs, field, fieldStart, fieldEnd)
					continue
				}
				if attr.Name == "g:bind:checked" {
					field := strings.TrimSpace(attr.Value)
					fieldStart, fieldEnd := attrValueOffset(source, attr, field)
					refs.CheckedBinds = append(refs.CheckedBinds, fieldRef{Name: field, Start: fieldStart, End: fieldEnd})
					addFieldRef(refs, field, fieldStart, fieldEnd)
					continue
				}
				if attr.Name == "g:ref" {
					refName := strings.TrimSpace(attr.Value)
					refStart, refEnd := attrValueOffset(source, attr, refName)
					refs.RefBinds = append(refs.RefBinds, fieldRef{Name: refName, Start: refStart, End: refEnd})
					continue
				}
				if strings.HasPrefix(attr.Name, "g:") {
					continue
				}
				if strings.HasPrefix(attr.Name, "class:") {
					expr := expressionAttrSource(attr.Value)
					exprStart, exprEnd := attrValueOffset(source, attr, expr)
					refs.ClassToggles = append(refs.ClassToggles, classToggleExpr{Name: attr.Name, Expression: expr, Start: exprStart, End: exprEnd})
					for _, field := range expressionFields(expr) {
						addFieldRef(refs, field, exprStart, exprEnd)
					}
					continue
				}
				if strings.HasPrefix(attr.Name, "style:") {
					expr := expressionAttrSource(attr.Value)
					exprStart, exprEnd := attrValueOffset(source, attr, expr)
					refs.StyleBindings = append(refs.StyleBindings, styleBindingExpr{Name: attr.Name, Expression: expr, Start: exprStart, End: exprEnd})
					for _, field := range expressionFields(expr) {
						addFieldRef(refs, field, exprStart, exprEnd)
					}
					continue
				}
				if attr.Expression {
					expr := expressionAttrSource(attr.Value)
					exprStart, exprEnd := attrValueOffset(source, attr, expr)
					refs.Attrs = append(refs.Attrs, reactiveAttrExpr{Name: attr.Name, Expression: expr, Start: exprStart, End: exprEnd})
					for _, field := range expressionFields(expr) {
						addFieldRef(refs, field, exprStart, exprEnd)
					}
					continue
				}
				collectSimpleAttrInterpolationRefs(source, attr, refs)
			}
			collectComponentViewReferences(source, typed.Children, refs)
		case viewmodel.ComponentCall:
			for _, attr := range typed.Attrs {
				if strings.HasPrefix(attr.Name, "g:") {
					continue
				}
				collectSimpleAttrInterpolationRefs(source, attr, refs)
			}
			collectComponentViewReferences(source, typed.Children, refs)
		case viewmodel.AwaitBlock:
			if _, urlExpr, ok := clientlang.ParseAwaitFetchExpression(typed.Expression); ok {
				for _, field := range expressionFields(urlExpr) {
					addFieldRef(refs, field, typed.Start, typed.End)
				}
			}
			collectComponentViewReferences(source, typed.Pending, refs)
		}
	}
}

func addFieldRef(refs *componentViewRefs, name string, start int, end int) {
	refs.Fields[name] = true
	refs.FieldRefs = append(refs.FieldRefs, fieldRef{Name: name, Start: start, End: end})
}

func attrValueOffset(source string, attr viewmodel.Attr, value string) (int, int) {
	if value == "" {
		return attr.Start, attr.End
	}
	runes := []rune(source)
	if attr.Start < 0 || attr.End > len(runes) || attr.Start >= attr.End {
		return attr.Start, attr.End
	}
	raw := string(runes[attr.Start:attr.End])
	index := strings.Index(raw, value)
	if index < 0 {
		return attr.Start, attr.End
	}
	start := attr.Start + len([]rune(raw[:index]))
	return start, start + len([]rune(value))
}

func collectSimpleAttrInterpolationRefs(source string, attr viewmodel.Attr, refs *componentViewRefs) {
	start, _ := attrValueOffset(source, attr, attr.Value)
	collectSimpleInterpolationRefs(attr.Value, start, refs)
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

func literalAttrValue(attrs []viewmodel.Attr, name string) string {
	for _, attr := range attrs {
		if attr.Name == name && !attr.Boolean && !attr.Expression {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func collectSimpleInterpolations(value string, fields map[string]bool) {
	refs := componentViewRefs{Fields: fields}
	collectSimpleInterpolationRefs(value, 0, &refs)
}

func collectSimpleInterpolationRefs(value string, base int, refs *componentViewRefs) {
	for index := 0; index < len(value); index++ {
		if value[index] != '{' {
			continue
		}
		end := strings.IndexByte(value[index+1:], '}')
		if end < 0 {
			return
		}
		end += index + 1
		name := value[index+1 : end]
		if isSimpleInterpolationName(name) {
			addFieldRef(refs, name, base+index+1, base+end)
		}
		index = end
	}
}

func isSimpleInterpolationName(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		if index == 0 {
			if char != '_' && (char < 'A' || char > 'Z') && (char < 'a' || char > 'z') {
				return false
			}
			continue
		}
		if char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

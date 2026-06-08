package view

import (
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
)

func Canonical(source string) (string, error) {
	nodes, err := Parse(stripLineComments(source))
	if err != nil {
		return "", err
	}
	var out renderOutput
	writeCanonicalNodes(&out, nodes)
	return out.string(), nil
}

func writeCanonicalNodes(out *renderOutput, nodes []Node) {
	for _, node := range nodes {
		writeCanonicalNode(out, node)
	}
}

func writeCanonicalNode(out *renderOutput, node Node) {
	switch typed := node.(type) {
	case Text:
		out.write("text(")
		out.write(strconv.Quote(strings.Join(strings.Fields(typed.Value), " ")))
		out.writeByte(')')
	case Element:
		out.write("element(")
		out.write(typed.Name)
		writeCanonicalAttrs(out, typed.Attrs)
		out.writeByte('[')
		writeCanonicalNodes(out, typed.Children)
		out.write("])")
	case ComponentCall:
		out.write("component(")
		out.write(typed.Name)
		writeCanonicalAttrs(out, typed.Attrs)
		out.writeByte('[')
		writeCanonicalNodes(out, typed.Children)
		out.write("])")
	}
}

func writeCanonicalAttrs(out *renderOutput, attrs []Attr) {
	normalized := make([]Attr, 0, len(attrs))
	for _, attr := range attrs {
		value := strings.TrimSpace(attr.Value)
		if attr.Name == "class" {
			classes := strings.Fields(value)
			sort.Strings(classes)
			value = strings.Join(classes, " ")
		}
		value = canonicalAttrValue(attr.Name, value, attr.Expression)
		normalized = append(normalized, Attr{Name: attr.Name, Value: value, Boolean: attr.Boolean, Expression: attr.Expression})
	}
	sort.Slice(normalized, func(i, j int) bool {
		if normalized[i].Name != normalized[j].Name {
			return normalized[i].Name < normalized[j].Name
		}
		if normalized[i].Value != normalized[j].Value {
			return normalized[i].Value < normalized[j].Value
		}
		return !normalized[i].Boolean && normalized[j].Boolean
	})
	out.writeByte('{')
	for index, attr := range normalized {
		if index > 0 {
			out.writeByte(',')
		}
		out.write(attr.Name)
		if attr.Boolean {
			out.write(":bool")
			continue
		}
		if attr.Expression {
			out.write(":expr")
		}
		out.writeByte('=')
		out.write(strconv.Quote(attr.Value))
	}
	out.writeByte('}')
}

func canonicalAttrValue(name string, value string, expression bool) string {
	if strings.HasPrefix(name, "g:on:") {
		return clientlang.CanonicalStatement(value)
	}
	if expression || name == "g:if" || name == "g:else-if" || name == "g:key" ||
		strings.HasPrefix(name, "class:") || strings.HasPrefix(name, "style:") {
		expr := expressionAttrSource(value)
		if canonical, err := clientlang.CanonicalExpr(expr); err == nil {
			return canonical
		}
		return strings.Join(strings.Fields(expr), " ")
	}
	return value
}

// ParamReferences returns unique param("name") route-param references directly
// visible in the current view markup subset.

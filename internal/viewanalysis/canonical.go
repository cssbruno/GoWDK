package viewanalysis

import (
	"sort"
	"strconv"
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/viewmodel"
	"github.com/cssbruno/gowdk/internal/viewparse"
)

// Canonical returns a deterministic AST-backed representation of a view body.
func Canonical(source string) (string, error) {
	nodes, err := viewparse.Parse(stripLineComments(source))
	if err != nil {
		return "", err
	}
	var out strings.Builder
	writeCanonicalNodes(&out, nodes)
	return out.String(), nil
}

func writeCanonicalNodes(out *strings.Builder, nodes []viewmodel.Node) {
	for _, node := range nodes {
		writeCanonicalNode(out, node)
	}
}

func writeCanonicalNode(out *strings.Builder, node viewmodel.Node) {
	switch typed := node.(type) {
	case viewmodel.Text:
		out.WriteString("text(")
		out.WriteString(strconv.Quote(strings.Join(strings.Fields(typed.Value), " ")))
		out.WriteByte(')')
	case viewmodel.Element:
		out.WriteString("element(")
		out.WriteString(typed.Name)
		writeCanonicalAttrs(out, typed.Attrs)
		out.WriteByte('[')
		writeCanonicalNodes(out, typed.Children)
		out.WriteString("])")
	case viewmodel.ComponentCall:
		out.WriteString("component(")
		out.WriteString(typed.Name)
		writeCanonicalAttrs(out, typed.Attrs)
		out.WriteByte('[')
		writeCanonicalNodes(out, typed.Children)
		out.WriteString("])")
	}
}

func writeCanonicalAttrs(out *strings.Builder, attrs []viewmodel.Attr) {
	normalized := make([]viewmodel.Attr, 0, len(attrs))
	for _, attr := range attrs {
		value := strings.TrimSpace(attr.Value)
		if attr.Name == "class" {
			classes := strings.Fields(value)
			sort.Strings(classes)
			value = strings.Join(classes, " ")
		}
		value = canonicalAttrValue(attr.Name, value, attr.Expression)
		normalized = append(normalized, viewmodel.Attr{Name: attr.Name, Value: value, Boolean: attr.Boolean, Expression: attr.Expression})
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
	out.WriteByte('{')
	for index, attr := range normalized {
		if index > 0 {
			out.WriteByte(',')
		}
		out.WriteString(attr.Name)
		if attr.Boolean {
			out.WriteString(":bool")
			continue
		}
		if attr.Expression {
			out.WriteString(":expr")
		}
		out.WriteByte('=')
		out.WriteString(strconv.Quote(attr.Value))
	}
	out.WriteByte('}')
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

func expressionAttrSource(value string) string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		return strings.TrimSpace(value[1 : len(value)-1])
	}
	return value
}

func stripLineComments(source string) string {
	var lines []string
	for _, rawLine := range strings.Split(source, "\n") {
		line := strings.TrimSpace(rawLine)
		if strings.HasPrefix(line, "//") {
			continue
		}
		lines = append(lines, rawLine)
	}
	return strings.Join(lines, "\n")
}

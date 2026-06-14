package view

import (
	"strings"

	gowhtml "github.com/cssbruno/gowdk/runtime/html"
)

func (node Element) writeActionInputAttrs(ctx *renderContext, out *renderOutput) {
	if node.Name != "input" || ctx.formAction == "" {
		return
	}
	name, ok := literalAttrValue(node.Attrs, "name")
	if !ok || name == "" {
		return
	}
	field, ok := ctx.actionInputField(ctx.formAction, name)
	if !ok {
		return
	}
	attrs := synthesizedInputAttrs(field.Type, node.Attrs)
	for _, attr := range attrs {
		out.writeByte(' ')
		out.write(attr.Name)
		out.write(`="`)
		out.write(gowhtml.Escape(attr.Value))
		out.writeByte('"')
	}
}

func (ctx *renderContext) actionInputField(action string, formName string) (ActionInputField, bool) {
	for _, field := range ctx.actionFields[action] {
		if field.FormName == formName {
			return field, true
		}
	}
	return ActionInputField{}, false
}

func synthesizedInputAttrs(fieldType string, attrs []Attr) []Attr {
	if !integerActionInputType(fieldType) {
		return nil
	}
	typeValue, hasType := literalAttrValue(attrs, "type")
	typeIsNumber := !hasType || strings.EqualFold(strings.TrimSpace(typeValue), "number")
	if !typeIsNumber {
		return nil
	}
	var out []Attr
	if !hasType {
		out = append(out, Attr{Name: "type", Value: "number"})
	}
	if !hasLiteralAttr(attrs, "inputmode") {
		out = append(out, Attr{Name: "inputmode", Value: "numeric"})
	}
	if min, max, ok := integerActionInputBounds(fieldType); ok {
		if min != "" && !hasLiteralAttr(attrs, "min") {
			out = append(out, Attr{Name: "min", Value: min})
		}
		if max != "" && !hasLiteralAttr(attrs, "max") {
			out = append(out, Attr{Name: "max", Value: max})
		}
	}
	return out
}

func integerActionInputType(fieldType string) bool {
	switch fieldType {
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return true
	default:
		return false
	}
}

func integerActionInputBounds(fieldType string) (string, string, bool) {
	switch fieldType {
	case "int8":
		return "-128", "127", true
	case "int16":
		return "-32768", "32767", true
	case "int32":
		return "-2147483648", "2147483647", true
	case "int64":
		return "-9223372036854775808", "9223372036854775807", true
	case "uint":
		return "0", "", true
	case "uint8":
		return "0", "255", true
	case "uint16":
		return "0", "65535", true
	case "uint32":
		return "0", "4294967295", true
	case "uint64":
		return "0", "18446744073709551615", true
	default:
		return "", "", false
	}
}

func literalAttrValue(attrs []Attr, name string) (string, bool) {
	for _, attr := range attrs {
		if attr.Name == name && !attr.Boolean && !attr.Expression {
			return strings.TrimSpace(attr.Value), true
		}
	}
	return "", false
}

func hasLiteralAttr(attrs []Attr, name string) bool {
	for _, attr := range attrs {
		if attr.Name == name && !attr.Expression {
			return true
		}
	}
	return false
}

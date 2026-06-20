package html

import (
	stdhtml "html"
	htmltemplate "html/template"
	"net/url"
	"strings"
)

// Escape escapes text for safe HTML output.
func Escape(value string) string {
	return stdhtml.EscapeString(value)
}

// EscapeURL escapes a request-time interpolation segment before it is placed
// inside a URL-bearing HTML attribute.
func EscapeURL(value string) string {
	return stdhtml.EscapeString(url.PathEscape(value))
}

// Attr renders an escaped HTML attribute when value is non-empty.
func Attr(name, value string) string {
	if value == "" || !validAttrName(name) {
		return ""
	}

	var out strings.Builder
	out.WriteByte(' ')
	out.WriteString(name)
	out.WriteString(`="`)
	htmltemplate.HTMLEscape(&out, []byte(value))
	out.WriteByte('"')
	return out.String()
}

func validAttrName(name string) bool {
	if name == "" {
		return false
	}
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
		case char >= 'A' && char <= 'Z':
		case char >= '0' && char <= '9':
		case char == '-' || char == '_' || char == ':' || char == '.':
		default:
			return false
		}
	}
	return true
}

// Classes joins generated class tokens.
func Classes(values ...string) string {
	var classes []string
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			classes = append(classes, strings.TrimSpace(value))
		}
	}
	return strings.Join(classes, " ")
}

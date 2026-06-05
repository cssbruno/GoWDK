package form

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// Values is the normalized representation passed to generated action decoders.
type Values map[string][]string

// Field describes one expected form field for generated decoders.
type Field struct {
	Name string
}

// Schema describes the submitted fields accepted by a generated decoder.
type Schema struct {
	Fields []Field
}

// DecodeError describes a generated form decoding failure without exposing
// submitted values.
type DecodeError struct {
	Field   string
	Message string
}

func (err DecodeError) Error() string {
	if strings.TrimSpace(err.Field) == "" {
		return err.Message
	}
	return fmt.Sprintf("%s: %s", err.Field, err.Message)
}

// FromURLValues copies request form values into a stable runtime structure.
func FromURLValues(values url.Values) Values {
	out := Values{}
	for key, list := range values {
		out[key] = append([]string(nil), list...)
	}
	return out
}

// DecodeExpected returns a copy of the submitted values restricted to the
// schema field allowlist. Missing expected fields are allowed; validation
// decides whether an absent value is acceptable.
func DecodeExpected(values Values, schema Schema) (Values, error) {
	allowed := map[string]bool{}
	for _, field := range schema.Fields {
		name := strings.TrimSpace(field.Name)
		if name == "" {
			return nil, DecodeError{Message: "expected field name is required"}
		}
		if allowed[name] {
			return nil, DecodeError{Field: name, Message: "duplicate expected field"}
		}
		allowed[name] = true
	}

	for name := range values {
		if !allowed[name] {
			return nil, DecodeError{Field: name, Message: "unexpected field"}
		}
	}

	out := Values{}
	for _, field := range schema.Fields {
		if submitted, ok := values[field.Name]; ok {
			out[field.Name] = append([]string(nil), submitted...)
		}
	}
	return out, nil
}

// First returns the first submitted value for a field.
func (values Values) First(name string) string {
	if len(values[name]) == 0 {
		return ""
	}
	return values[name][0]
}

// All returns all submitted values for a field.
func (values Values) All(name string) []string {
	return append([]string(nil), values[name]...)
}

// HasSubmitted reports whether a field was submitted with at least one
// non-blank value.
func (values Values) HasSubmitted(name string) bool {
	for _, value := range values[name] {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}

// Names returns submitted field names in stable order.
func (values Values) Names() []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

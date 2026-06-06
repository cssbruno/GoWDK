package form

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
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
		if IsRuntimeField(name) {
			continue
		}
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

// IsRuntimeField reports whether a field is reserved for generated runtime
// metadata instead of user form input.
func IsRuntimeField(name string) bool {
	name = strings.TrimSpace(name)
	switch name {
	case "_csrf", "_gwdk", "_gowdk", "_method", "_gowdk_csrf":
		return true
	default:
		return strings.HasPrefix(name, "_gowdk_") || strings.HasPrefix(name, "_gwdk_")
	}
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

// String decodes one scalar string field and rejects repeated scalar values.
func String(values Values, name string) (string, bool, error) {
	return scalar(values, name)
}

// Strings returns all submitted values for a repeated string field.
func Strings(values Values, name string) []string {
	return values.All(name)
}

// Select decodes a single-select field.
func Select(values Values, name string) (string, bool, error) {
	return String(values, name)
}

// SelectMultiple returns all submitted values for a multiple select field.
func SelectMultiple(values Values, name string) []string {
	return values.All(name)
}

// Radio decodes one selected radio value.
func Radio(values Values, name string) (string, bool, error) {
	return String(values, name)
}

// Checkbox decodes one checkbox as checked when the field was submitted.
// Absent checkboxes are false. Repeated values are rejected so checkbox groups
// use CheckboxGroup instead.
func Checkbox(values Values, name string) (bool, error) {
	submitted, ok := values[name]
	if !ok || len(submitted) == 0 {
		return false, nil
	}
	if len(submitted) > 1 {
		return false, DecodeError{Field: name, Message: "repeated checkbox field"}
	}
	switch strings.ToLower(strings.TrimSpace(submitted[0])) {
	case "", "0", "false", "off", "no":
		return false, nil
	default:
		return true, nil
	}
}

// CheckboxGroup returns all submitted values for a checkbox group.
func CheckboxGroup(values Values, name string) []string {
	return values.All(name)
}

// Bool decodes one scalar boolean field and rejects repeated scalar values.
func Bool(values Values, name string) (bool, bool, error) {
	value, ok, err := scalar(values, name)
	if err != nil || !ok {
		return false, ok, err
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "off", "no":
		return false, true, nil
	case "1", "true", "on", "yes":
		return true, true, nil
	default:
		return false, true, DecodeError{Field: name, Message: "invalid boolean"}
	}
}

// Int decodes one signed integer field with the requested bit size.
func Int(values Values, name string, bitSize int) (int64, bool, error) {
	value, ok, err := scalar(values, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true, nil
	}
	parsed, err := strconv.ParseInt(value, 10, bitSize)
	if err != nil {
		return 0, true, DecodeError{Field: name, Message: "invalid signed integer"}
	}
	return parsed, true, nil
}

// Uint decodes one unsigned integer field with the requested bit size.
func Uint(values Values, name string, bitSize int) (uint64, bool, error) {
	value, ok, err := scalar(values, name)
	if err != nil || !ok {
		return 0, ok, err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, true, nil
	}
	parsed, err := strconv.ParseUint(value, 10, bitSize)
	if err != nil {
		return 0, true, DecodeError{Field: name, Message: "invalid unsigned integer"}
	}
	return parsed, true, nil
}

func scalar(values Values, name string) (string, bool, error) {
	submitted, ok := values[name]
	if !ok || len(submitted) == 0 {
		return "", false, nil
	}
	if len(submitted) > 1 {
		return "", true, DecodeError{Field: name, Message: "repeated scalar field"}
	}
	return submitted[0], true, nil
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

package ssr

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/cssbruno/gowdk/runtime/guard"
)

// LoadContext is passed to generated request-time load {} functions.
type LoadContext = guard.Context

// LoadFunc is generated from a request-time load {} block.
type LoadFunc func(LoadContext) (map[string]any, error)

// NewLoadContext creates the first-slice request context for generated SSR load
// functions. Session storage is intentionally caller-supplied until the SSR
// addon defines secure session defaults.
func NewLoadContext(request *http.Request, session map[string]any) LoadContext {
	return guard.NewContext(request, session)
}

// LoadPath resolves a declared load {} path from generated SSR load data.
// Supported values are nested maps with string keys, structs, pointers, and
// interfaces. Struct fields may be matched by exported Go field name or json tag.
func LoadPath(data map[string]any, path string) (any, bool) {
	parts := strings.Split(strings.TrimSpace(path), ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, false
	}
	value, ok := data[parts[0]]
	if !ok {
		return nil, false
	}
	for _, part := range parts[1:] {
		if part == "" {
			return nil, false
		}
		value, ok = loadPathValue(value, part)
		if !ok {
			return nil, false
		}
	}
	return value, true
}

func loadPathValue(value any, field string) (any, bool) {
	if value == nil {
		return nil, false
	}
	switch typed := value.(type) {
	case map[string]any:
		found, ok := typed[field]
		return found, ok
	case map[string]string:
		found, ok := typed[field]
		return found, ok
	}

	reflected := reflect.ValueOf(value)
	for reflected.Kind() == reflect.Interface || reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return nil, false
		}
		reflected = reflected.Elem()
	}

	switch reflected.Kind() {
	case reflect.Map:
		return loadMapField(reflected, field)
	case reflect.Struct:
		return loadStructField(reflected, field)
	default:
		return nil, false
	}
}

func loadMapField(value reflect.Value, field string) (any, bool) {
	if value.Type().Key().Kind() != reflect.String {
		return nil, false
	}
	found := value.MapIndex(reflect.ValueOf(field))
	if !found.IsValid() {
		return nil, false
	}
	return found.Interface(), true
}

func loadStructField(value reflect.Value, field string) (any, bool) {
	valueType := value.Type()
	for index := range value.NumField() {
		structField := valueType.Field(index)
		if !structField.IsExported() {
			continue
		}
		if structField.Name == field || strings.EqualFold(structField.Name, field) || jsonFieldName(structField) == field {
			return value.Field(index).Interface(), true
		}
	}
	return nil, false
}

func jsonFieldName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
	if name == "-" {
		return ""
	}
	return name
}

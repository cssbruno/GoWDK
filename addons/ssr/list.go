package ssr

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	gowdkhtml "github.com/cssbruno/gowdk/runtime/html"
)

// ListSpec describes one server-rendered g:each list. It is generated at build
// time from a request-time page view and consumed by RenderLists at request
// time. A spec is a tree: Children describe nested g:each lists whose data is
// resolved relative to each parent row element.
type ListSpec struct {
	// Placeholder is the unique token embedded in the parent HTML (for a
	// top-level list) or parent RowTemplate (for a nested list) that the
	// rendered rows replace.
	Placeholder string
	// SourcePath is the dotted load path to the slice. For a top-level list it
	// is resolved against the request-time load {} data map; for a nested list
	// it is resolved relative to the parent row element.
	SourcePath string
	// RowTemplate is the escaped HTML rendered once per slice element, still
	// containing this spec's Field and Child placeholders.
	RowTemplate string
	// Fields are the per-row scalar interpolations to substitute, each escaped
	// at request time.
	Fields []ListField
	// Children are nested g:each lists found inside RowTemplate.
	Children []ListSpec
}

// ListField is one per-row scalar substitution inside a ListSpec's RowTemplate.
type ListField struct {
	// Placeholder is the unique token inside RowTemplate replaced per row.
	Placeholder string
	// Path is the dotted, item-relative path to the value (e.g. "title" or
	// "author.name"). Ignored when Index is true.
	Path string
	// Index is true when this field substitutes the zero-based row index rather
	// than an element field.
	Index bool
}

// RenderLists expands every top-level list spec into html by resolving each
// spec's slice from the request-time load data and substituting rendered rows.
// Field values are always HTML-escaped, preserving GOWDK's escape-by-default
// contract for request-time server data.
func RenderLists(html string, specs []ListSpec, data map[string]any) string {
	for _, spec := range specs {
		slice, _ := resolveListSlice(data, spec.SourcePath, true)
		html = strings.ReplaceAll(html, spec.Placeholder, renderListRows(spec, slice))
	}
	return html
}

func renderListRows(spec ListSpec, slice []any) string {
	var builder strings.Builder
	for index, element := range slice {
		row := spec.RowTemplate
		for _, field := range spec.Fields {
			row = strings.ReplaceAll(row, field.Placeholder, listFieldValue(field, element, index))
		}
		for _, child := range spec.Children {
			childSlice, _ := resolveListSlice(element, child.SourcePath, false)
			row = strings.ReplaceAll(row, child.Placeholder, renderListRows(child, childSlice))
		}
		builder.WriteString(row)
	}
	return builder.String()
}

func listFieldValue(field ListField, element any, index int) string {
	if field.Index {
		return gowdkhtml.Escape(strconv.Itoa(index))
	}
	value, ok := ElementPath(element, field.Path)
	if !ok || value == nil {
		return ""
	}
	return gowdkhtml.Escape(fmt.Sprint(value))
}

// resolveListSlice resolves a dotted path to a slice. When isMap is true the
// container is the top-level load data map; otherwise it is a parent row
// element resolved with the same field-access rules as LoadPath.
func resolveListSlice(container any, path string, isMap bool) ([]any, bool) {
	var value any
	var ok bool
	if isMap {
		data, isData := container.(map[string]any)
		if !isData {
			return nil, false
		}
		value, ok = LoadPath(data, path)
	} else {
		value, ok = ElementPath(container, path)
	}
	if !ok {
		return nil, false
	}
	return toAnySlice(value)
}

// ElementPath resolves a dotted field path against an arbitrary value (map,
// struct, pointer, or interface), mirroring LoadPath's field-matching rules but
// rooted at a slice element rather than the load data map.
func ElementPath(value any, path string) (any, bool) {
	if strings.TrimSpace(path) == "" {
		return value, true
	}
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if part == "" {
			return nil, false
		}
		next, ok := loadPathValue(value, part)
		if !ok {
			return nil, false
		}
		value = next
	}
	return value, true
}

func toAnySlice(value any) ([]any, bool) {
	if value == nil {
		return nil, false
	}
	if slice, ok := value.([]any); ok {
		return slice, true
	}
	reflected := reflect.ValueOf(value)
	for reflected.Kind() == reflect.Interface || reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return nil, false
		}
		reflected = reflected.Elem()
	}
	switch reflected.Kind() {
	case reflect.Slice, reflect.Array:
		out := make([]any, reflected.Len())
		for index := range out {
			out[index] = reflected.Index(index).Interface()
		}
		return out, true
	default:
		return nil, false
	}
}

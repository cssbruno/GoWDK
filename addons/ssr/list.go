package ssr

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	gowdkhtml "github.com/cssbruno/gowdk/runtime/html"
)

// ListSpec describes one server-rendered g:for list. It is generated at build
// time from a request-time page view and consumed by RenderRegions at request
// time. A spec is a tree: Lists and Conds describe nested g:for lists and
// g:if conditionals whose data is resolved relative to each parent row.
type ListSpec struct {
	// Placeholder is the unique token embedded in the parent template that the
	// rendered rows replace.
	Placeholder string
	// SourcePath is the dotted path to the slice, resolved against the enclosing
	// container (the request-time load data at the top level, or a parent row
	// element when nested).
	SourcePath string
	// RowTemplate is the escaped HTML rendered once per slice element, still
	// containing this spec's Field, List, and Cond placeholders.
	RowTemplate string
	// Fields are the per-row scalar interpolations, each escaped at request time.
	Fields []ListField
	// Lists are nested g:for lists found inside RowTemplate.
	Lists []ListSpec
	// Conds are g:if conditionals found inside RowTemplate.
	Conds []CondSpec
}

// CondSpec describes one server-rendered g:if conditional. Its branch is
// rendered into the output only when SourcePath resolves to a truthy value
// (negated when Negate is set). The branch shares the enclosing container scope,
// so its fields and nested regions resolve against the same data as the
// conditional itself.
type CondSpec struct {
	Placeholder string
	SourcePath  string
	Negate      bool
	Template    string
	Fields      []ListField
	Lists       []ListSpec
	Conds       []CondSpec
}

// ListField is one per-row scalar substitution inside a region template.
type ListField struct {
	// Placeholder is the unique token inside the template replaced per render.
	Placeholder string
	// Path is the dotted path to the value (item-relative inside a row, or a
	// load field path inside a top-level region). Ignored when Index is true.
	Path string
	// Index is true when this field substitutes the zero-based row index.
	Index bool
}

// RenderRegions expands every top-level g:for list and g:if conditional in
// html by resolving their data from the request-time load data. Field values
// are always HTML-escaped, preserving GOWDK's escape-by-default contract for
// request-time server data.
func RenderRegions(html string, lists []ListSpec, conds []CondSpec, data map[string]any) string {
	return expandRegion(html, nil, lists, conds, data, -1)
}

// expandRegion substitutes a region template's fields, nested lists, and nested
// conditionals against a container value (the load data map at the top level, or
// a row element when nested). index is the current row index, or -1 outside a
// row.
func expandRegion(template string, fields []ListField, lists []ListSpec, conds []CondSpec, container any, index int) string {
	for _, field := range fields {
		template = strings.ReplaceAll(template, field.Placeholder, listFieldValue(field, container, index))
	}
	for _, list := range lists {
		slice, _ := elementSlice(container, list.SourcePath)
		template = strings.ReplaceAll(template, list.Placeholder, renderListRows(list, slice))
	}
	for _, cond := range conds {
		branch := ""
		if condHolds(container, cond) {
			branch = expandRegion(cond.Template, cond.Fields, cond.Lists, cond.Conds, container, index)
		}
		template = strings.ReplaceAll(template, cond.Placeholder, branch)
	}
	return template
}

func renderListRows(list ListSpec, slice []any) string {
	var builder strings.Builder
	for index, element := range slice {
		builder.WriteString(expandRegion(list.RowTemplate, list.Fields, list.Lists, list.Conds, element, index))
	}
	return builder.String()
}

func condHolds(container any, cond CondSpec) bool {
	value, ok := ElementPath(container, cond.SourcePath)
	show := ok && truthy(value)
	if cond.Negate {
		return !show
	}
	return show
}

func listFieldValue(field ListField, container any, index int) string {
	if field.Index {
		return gowdkhtml.Escape(strconv.Itoa(index))
	}
	value, ok := ElementPath(container, field.Path)
	if !ok || value == nil {
		return ""
	}
	return gowdkhtml.Escape(fmt.Sprint(value))
}

func elementSlice(container any, path string) ([]any, bool) {
	value, ok := ElementPath(container, path)
	if !ok {
		return nil, false
	}
	return toAnySlice(value)
}

// truthy reports whether a request-time value should render its g:if branch.
// It mirrors common template semantics: false, "", 0, nil, and empty
// slices/maps are falsy; everything else is truthy.
func truthy(value any) bool {
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	for reflected.Kind() == reflect.Pointer || reflected.Kind() == reflect.Interface {
		if reflected.IsNil() {
			return false
		}
		reflected = reflected.Elem()
	}
	switch reflected.Kind() {
	case reflect.Bool:
		return reflected.Bool()
	case reflect.String:
		return reflected.String() != ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflected.Int() != 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflected.Uint() != 0
	case reflect.Float32, reflect.Float64:
		return reflected.Float() != 0
	case reflect.Slice, reflect.Array, reflect.Map:
		return reflected.Len() > 0
	default:
		return true
	}
}

// ElementPath resolves a dotted field path against an arbitrary value (map,
// struct, pointer, or interface), mirroring LoadPath's field-matching rules but
// rooted at a row element or the load data map.
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

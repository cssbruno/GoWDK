package lsp

import (
	"strings"

	"github.com/cssbruno/gowdk/internal/clientlang"
	"github.com/cssbruno/gowdk/internal/viewmodel"
)

func inferredComponentFields(nodes []viewmodel.Node, program *clientlang.Program) []string {
	fields := map[string]bool{}
	collectViewNodeFields(nodes, fields)
	if program != nil {
		for _, function := range program.Functions {
			for _, statement := range function.Statements {
				collectAssignmentFields(statement, fields)
			}
		}
		for _, statement := range program.Mount {
			collectAssignmentFields(statement, fields)
		}
		for _, effect := range program.Effects {
			for _, statement := range effect.Statements {
				collectAssignmentFields(statement, fields)
			}
		}
	}
	out := make([]string, 0, len(fields))
	for field := range fields {
		out = append(out, field)
	}
	return out
}

func collectViewNodeFields(nodes []viewmodel.Node, fields map[string]bool) {
	for _, node := range nodes {
		switch typed := node.(type) {
		case viewmodel.Text:
			collectInterpolationFields(typed.Value, fields)
		case viewmodel.Element:
			collectViewAttrs(typed.Attrs, fields)
			collectViewNodeFields(typed.Children, fields)
		case viewmodel.ComponentCall:
			collectViewAttrs(typed.Attrs, fields)
			collectViewNodeFields(typed.Children, fields)
		case viewmodel.AwaitBlock:
			collectViewNodeFields(typed.Pending, fields)
			collectViewNodeFields(typed.Then, fields)
			collectViewNodeFields(typed.Catch, fields)
		}
	}
}

func collectViewAttrs(attrs []viewmodel.Attr, fields map[string]bool) {
	for _, attr := range attrs {
		collectInterpolationFields(attr.Value, fields)
		if attr.Name == "g:bind:value" || attr.Name == "g:bind:checked" {
			name := strings.Trim(strings.TrimSpace(attr.Value), "{}")
			if isLSPIdentifier(name) {
				fields[name] = true
			}
		}
	}
}

func collectInterpolationFields(source string, fields map[string]bool) {
	for index := 0; index < len(source); index++ {
		if source[index] != '{' {
			continue
		}
		end := strings.IndexByte(source[index+1:], '}')
		if end < 0 {
			return
		}
		end += index + 1
		name := strings.TrimSpace(source[index+1 : end])
		if isLSPIdentifier(name) {
			fields[name] = true
		}
		index = end
	}
}

func collectAssignmentFields(source string, fields map[string]bool) {
	for cursor := 0; cursor < len(source); cursor++ {
		if !isLSPIdentStart(source[cursor]) {
			continue
		}
		start := cursor
		cursor++
		for cursor < len(source) && isLSPIdentPart(source[cursor]) {
			cursor++
		}
		name := source[start:cursor]
		rest := strings.TrimLeftFunc(source[cursor:], isLSPSpace)
		if strings.HasPrefix(rest, "=") || strings.HasPrefix(rest, "++") || strings.HasPrefix(rest, "--") {
			fields[name] = true
		}
	}
}

func isLSPIdentifier(value string) bool {
	if value == "" || !isLSPIdentStart(value[0]) {
		return false
	}
	for index := 1; index < len(value); index++ {
		if !isLSPIdentPart(value[index]) {
			return false
		}
	}
	return true
}

func isLSPIdentStart(char byte) bool {
	return char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z')
}

func isLSPIdentPart(char byte) bool {
	return isLSPIdentStart(char) || (char >= '0' && char <= '9')
}

func isLSPSpace(char rune) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}

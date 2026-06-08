package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
)

// ParseComponent extracts component metadata and top-level block declarations.
func ParseComponent(source []byte) (manifest.Component, error) {
	var component manifest.Component
	var viewBody []string
	inView := false
	var styleBody []string
	inStyle := false
	styleDepth := 0
	inProps := false
	inExports := false
	inEmits := false
	var clientBody []string
	inClient := false
	clientDepth := 0
	var goBlockBody []string
	inGoBlock := false
	goBlockDepth := 0
	goBlockTarget := ""
	seenGoBlocks := map[string]manifest.SourceSpan{}
	seenDeclaration := false

	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if inGoBlock {
			if line == "}" {
				goBlockDepth--
				if goBlockDepth == 0 {
					component.Blocks.GoBlocks = append(component.Blocks.GoBlocks, manifest.GoBlock{
						Target: goBlockTarget,
						Body:   strings.TrimSpace(strings.Join(goBlockBody, "\n")),
						Span:   seenGoBlocks[goBlockTarget],
					})
					component.Blocks.Spans.GoBlocks = append(component.Blocks.Spans.GoBlocks, manifest.NamedSpan{Name: goBlockTarget, Span: seenGoBlocks[goBlockTarget]})
					inGoBlock = false
					goBlockBody = nil
					goBlockDepth = 0
					goBlockTarget = ""
					continue
				}
				goBlockBody = append(goBlockBody, rawLine)
				continue
			}
			goBlockDepth += braceDelta(rawLine)
			if goBlockDepth < 1 {
				return manifest.Component{}, fmt.Errorf("line %d: go block closed unexpectedly", lineNumber)
			}
			goBlockBody = append(goBlockBody, rawLine)
			continue
		}
		if inStyle {
			styleDepth += braceDelta(rawLine)
			if styleDepth < 0 {
				return manifest.Component{}, fmt.Errorf("line %d: style block closed unexpectedly", lineNumber)
			}
			if styleDepth == 0 {
				component.Blocks.StyleBody = strings.TrimSpace(strings.Join(styleBody, "\n"))
				component.Blocks.Style = component.Blocks.StyleBody != ""
				inStyle = false
				styleBody = nil
				styleDepth = 0
				continue
			}
			styleBody = append(styleBody, rawLine)
			continue
		}
		if inClient {
			if line == "}" && clientDepth == 1 {
				component.Blocks.ClientBody = strings.TrimSpace(strings.Join(clientBody, "\n"))
				inClient = false
				clientBody = nil
				clientDepth = 0
				continue
			}
			clientBody = append(clientBody, rawLine)
			clientDepth += braceDelta(rawLine)
			if clientDepth < 1 {
				return manifest.Component{}, fmt.Errorf("line %d: client block closed unexpectedly", lineNumber)
			}
			continue
		}
		if inView {
			if line == "style {" {
				return manifest.Component{}, fmt.Errorf("line %d: style block must be outside view {}", lineNumber)
			}
			if line == "}" {
				component.Blocks.ViewBody = strings.TrimSpace(strings.Join(viewBody, "\n"))
				inView = false
				viewBody = nil
				continue
			}
			viewBody = append(viewBody, rawLine)
			continue
		}
		if inProps {
			if line == "}" {
				inProps = false
				continue
			}
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			match := propPattern.FindStringSubmatch(line)
			if match == nil {
				return manifest.Component{}, fmt.Errorf("line %d: invalid prop declaration %q", lineNumber, line)
			}
			if match[2] != "string" {
				return manifest.Component{}, fmt.Errorf("line %d: prop %s uses unsupported type %q", lineNumber, match[1], match[2])
			}
			component.Props = append(component.Props, manifest.Prop{Name: match[1], Type: match[2], Span: sourceLineSpan(lineNumber, rawLine)})
			continue
		}
		if inExports {
			if line == "}" {
				inExports = false
				continue
			}
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			match := propPattern.FindStringSubmatch(line)
			if match == nil {
				return manifest.Component{}, fmt.Errorf("line %d: invalid export declaration %q", lineNumber, line)
			}
			if !supportedScalarType(match[2]) {
				return manifest.Component{}, fmt.Errorf("line %d: export %s uses unsupported type %q", lineNumber, match[1], match[2])
			}
			component.Exports = append(component.Exports, manifest.Export{Name: match[1], Type: match[2], Span: sourceLineSpan(lineNumber, rawLine)})
			continue
		}
		if inEmits {
			if line == "}" {
				inEmits = false
				continue
			}
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			event, err := parseEmitDeclaration(line, lineNumber, rawLine)
			if err != nil {
				return manifest.Component{}, err
			}
			component.Emits = append(component.Emits, event)
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if match := packagePattern.FindStringSubmatch(line); match != nil {
			if seenDeclaration {
				return manifest.Component{}, fmt.Errorf("line %d: package declaration must be the first non-comment declaration", lineNumber)
			}
			component.Package = match[1]
			component.PackageSpan = sourceLineSpan(lineNumber, rawLine)
			seenDeclaration = true
			continue
		}
		if strings.HasPrefix(line, "package ") {
			return manifest.Component{}, fmt.Errorf("line %d: malformed package declaration %q", lineNumber, line)
		}
		seenDeclaration = true

		if match := importPattern.FindStringSubmatch(line); match != nil {
			component.Imports = append(component.Imports, manifest.Import{
				Alias: match[1],
				Path:  match[2],
				Span:  sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedImport(line) {
			return manifest.Component{}, fmt.Errorf("line %d: malformed import %q", lineNumber, line)
		}
		if match := usePattern.FindStringSubmatch(line); match != nil {
			component.Uses = append(component.Uses, manifest.Use{
				Alias:   match[1],
				Package: match[2],
				Span:    sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedUse(line) {
			return manifest.Component{}, fmt.Errorf("line %d: malformed use %q", lineNumber, line)
		}
		if match := jsPattern.FindStringSubmatch(line); match != nil {
			component.JS = append(component.JS, match[1])
			component.Spans.JS = append(component.Spans.JS, manifest.NamedSpan{Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)})
			continue
		}
		if isMalformedJS(line) {
			return manifest.Component{}, fmt.Errorf("line %d: malformed js declaration %q", lineNumber, line)
		}

		if strings.HasPrefix(line, "@") {
			match := annotationPattern.FindStringSubmatch(line)
			if match == nil {
				return manifest.Component{}, fmt.Errorf("line %d: malformed annotation %q", lineNumber, line)
			}
			if err := applyComponentAnnotation(&component, match[1], match[2], lineNumber, rawLine); err != nil {
				return manifest.Component{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			continue
		}

		if match := componentTypePattern.FindStringSubmatch(line); match != nil {
			span := sourceLineSpan(lineNumber, rawLine)
			kind := match[1]
			typeRef := manifest.GoTypeRef{Alias: match[2], Name: match[3], Span: span}
			initRef := manifest.GoFuncRef{Alias: match[4], Name: match[5], Span: span}
			switch kind {
			case "props":
				if initRef.Name != "" {
					return manifest.Component{}, fmt.Errorf("line %d: props contract must not declare an init function", lineNumber)
				}
				if component.PropsType.Name != "" || len(component.Props) > 0 {
					return manifest.Component{}, fmt.Errorf("line %d: component declares multiple props contracts", lineNumber)
				}
				component.PropsType = typeRef
			case "state":
				if initRef.Name == "" {
					return manifest.Component{}, fmt.Errorf("line %d: state contract requires an init function", lineNumber)
				}
				if component.State.Type.Name != "" {
					return manifest.Component{}, fmt.Errorf("line %d: component declares multiple state contracts", lineNumber)
				}
				component.State = manifest.StateContract{Type: typeRef, Init: initRef, Span: span}
			}
			continue
		}

		switch line {
		case "props {":
			if component.PropsType.Name != "" || len(component.Props) > 0 {
				return manifest.Component{}, fmt.Errorf("line %d: component declares multiple props contracts", lineNumber)
			}
			inProps = true
			continue
		case "exports {":
			if len(component.Exports) > 0 {
				return manifest.Component{}, fmt.Errorf("line %d: component declares multiple exports blocks", lineNumber)
			}
			component.Blocks.Spans.Exports = sourceLineSpan(lineNumber, rawLine)
			inExports = true
			continue
		case "client {":
			if component.Blocks.Client {
				return manifest.Component{}, fmt.Errorf("line %d: component declares multiple client blocks", lineNumber)
			}
			component.Blocks.Client = true
			component.Blocks.Spans.Client = sourceLineSpan(lineNumber, rawLine)
			inClient = true
			clientDepth = 1
			continue
		case "go {":
			span := sourceLineSpan(lineNumber, rawLine)
			if first, exists := seenGoBlocks[""]; exists {
				return manifest.Component{}, fmt.Errorf("line %d: duplicate go block; first declared on line %d", lineNumber, first.Start.Line)
			}
			seenGoBlocks[""] = span
			inGoBlock = true
			goBlockDepth = 1
			goBlockTarget = ""
			continue
		case "emits {":
			if len(component.Emits) > 0 {
				return manifest.Component{}, fmt.Errorf("line %d: component declares multiple emits blocks", lineNumber)
			}
			component.Blocks.Spans.Emits = sourceLineSpan(lineNumber, rawLine)
			inEmits = true
			continue
		case "view {":
			component.Blocks.View = true
			component.Blocks.Spans.View = sourceLineSpan(lineNumber, rawLine)
			inView = true
			continue
		case "style {":
			if component.Blocks.Style {
				return manifest.Component{}, fmt.Errorf("line %d: component declares multiple style blocks", lineNumber)
			}
			component.Blocks.Style = true
			inStyle = true
			styleDepth = 1
			continue
		}
		if match := goBlockPattern.FindStringSubmatch(line); match != nil {
			target := strings.TrimSpace(match[1])
			span := sourceLineSpan(lineNumber, rawLine)
			if first, exists := seenGoBlocks[target]; exists {
				label := "go"
				if target != "" {
					label = "go " + target
				}
				return manifest.Component{}, fmt.Errorf("line %d: duplicate %s block; first declared on line %d", lineNumber, label, first.Start.Line)
			}
			seenGoBlocks[target] = span
			inGoBlock = true
			goBlockDepth = 1
			goBlockTarget = target
			continue
		}

		if name := unsupportedTopLevelBlockName(line); name != "" {
			return manifest.Component{}, fmt.Errorf("line %d: unsupported top-level block %q", lineNumber, name)
		}
	}
	if err := scanner.Err(); err != nil {
		return manifest.Component{}, err
	}
	if inView {
		return manifest.Component{}, fmt.Errorf("view block missing closing }")
	}
	if inStyle {
		return manifest.Component{}, fmt.Errorf("style block missing closing }")
	}
	if inProps {
		return manifest.Component{}, fmt.Errorf("props block missing closing }")
	}
	if inExports {
		return manifest.Component{}, fmt.Errorf("exports block missing closing }")
	}
	if inEmits {
		return manifest.Component{}, fmt.Errorf("emits block missing closing }")
	}
	if inClient {
		return manifest.Component{}, fmt.Errorf("client block missing closing }")
	}
	if inGoBlock {
		return manifest.Component{}, fmt.Errorf("go block missing closing }")
	}
	if component.Name == "" {
		return manifest.Component{}, fmt.Errorf("missing @component")
	}
	return component, nil
}

func supportedScalarType(value string) bool {
	return value == "string" || value == "int" || value == "float" || value == "bool"
}

func parseEmitDeclaration(line string, lineNumber int, rawLine string) (manifest.Emit, error) {
	match := emitPattern.FindStringSubmatch(line)
	if match == nil {
		return manifest.Emit{}, fmt.Errorf("line %d: invalid emit declaration %q", lineNumber, line)
	}
	params, err := parseEmitParams(match[2], lineNumber, rawLine)
	if err != nil {
		return manifest.Emit{}, err
	}
	return manifest.Emit{Name: match[1], Params: params, Span: sourceLineSpan(lineNumber, rawLine)}, nil
}

func parseEmitParams(source string, lineNumber int, rawLine string) ([]manifest.EmitParam, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, nil
	}
	parts := strings.Split(source, ",")
	params := make([]manifest.EmitParam, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		item := strings.TrimSpace(part)
		fields := strings.Fields(item)
		if len(fields) != 2 {
			return nil, fmt.Errorf("line %d: emit parameter %q must use `name type`", lineNumber, item)
		}
		name, typ := fields[0], fields[1]
		if !identifierPattern.MatchString(name) {
			return nil, fmt.Errorf("line %d: invalid emit parameter name %q", lineNumber, name)
		}
		if !supportedScalarType(typ) {
			return nil, fmt.Errorf("line %d: emit parameter %s uses unsupported type %q", lineNumber, name, typ)
		}
		if seen[name] {
			return nil, fmt.Errorf("line %d: duplicate emit parameter %q", lineNumber, name)
		}
		seen[name] = true
		params = append(params, manifest.EmitParam{Name: name, Type: typ, Span: sourceLineSpan(lineNumber, rawLine)})
	}
	return params, nil
}

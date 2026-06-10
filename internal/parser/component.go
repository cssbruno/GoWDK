package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
)

// ParseComponent extracts component metadata and top-level block declarations.
func ParseComponent(src []byte) (gwdkir.Component, error) {
	var component gwdkir.Component
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
	var jsBody []string
	inJS := false
	jsDepth := 0
	var goBlockBody []string
	inGoBlock := false
	goBlockDepth := 0
	goBlockTarget := ""
	seenGoBlocks := map[string]source.SourceSpan{}
	seenDeclaration := false
	var blockScanner braceScanner

	scanner := bufio.NewScanner(bytes.NewReader(src))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if inGoBlock {
			if line == "}" && !blockScanner.inMultiline() {
				goBlockDepth--
				if goBlockDepth == 0 {
					component.Blocks.GoBlocks = append(component.Blocks.GoBlocks, gwdkir.GoBlock{
						Target: goBlockTarget,
						Body:   strings.TrimSpace(strings.Join(goBlockBody, "\n")),
						Span:   seenGoBlocks[goBlockTarget],
					})
					component.Blocks.Spans.GoBlocks = append(component.Blocks.Spans.GoBlocks, source.NamedSpan{Name: goBlockTarget, Span: seenGoBlocks[goBlockTarget]})
					inGoBlock = false
					goBlockBody = nil
					goBlockDepth = 0
					goBlockTarget = ""
					continue
				}
				goBlockBody = append(goBlockBody, rawLine)
				continue
			}
			goBlockDepth += blockScanner.delta(rawLine)
			if goBlockDepth < 1 {
				return gwdkir.Component{}, fmt.Errorf("line %d: go block closed unexpectedly", lineNumber)
			}
			goBlockBody = append(goBlockBody, rawLine)
			continue
		}
		if inStyle {
			styleDepth += blockScanner.delta(rawLine)
			if styleDepth < 0 {
				return gwdkir.Component{}, fmt.Errorf("line %d: style block closed unexpectedly", lineNumber)
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
			if line == "}" && clientDepth == 1 && !blockScanner.inMultiline() {
				component.Blocks.ClientBody = strings.TrimSpace(strings.Join(clientBody, "\n"))
				inClient = false
				clientBody = nil
				clientDepth = 0
				continue
			}
			clientBody = append(clientBody, rawLine)
			clientDepth += blockScanner.delta(rawLine)
			if clientDepth < 1 {
				return gwdkir.Component{}, fmt.Errorf("line %d: client block closed unexpectedly", lineNumber)
			}
			continue
		}
		if inJS {
			if line == "}" && !blockScanner.inMultiline() {
				jsDepth--
				if jsDepth == 0 {
					name := source.InlineScriptName(len(component.InlineJS))
					component.InlineJS = append(component.InlineJS, source.InlineScript{
						Name: name,
						Body: strings.TrimSpace(strings.Join(jsBody, "\n")),
						Span: component.Spans.InlineJS[len(component.Spans.InlineJS)-1].Span,
					})
					inJS = false
					jsBody = nil
					jsDepth = 0
					continue
				}
				jsBody = append(jsBody, rawLine)
				continue
			}
			jsDepth += blockScanner.delta(rawLine)
			if jsDepth < 1 {
				return gwdkir.Component{}, fmt.Errorf("line %d: js block closed unexpectedly", lineNumber)
			}
			jsBody = append(jsBody, rawLine)
			continue
		}
		if inView {
			if line == "style {" {
				return gwdkir.Component{}, fmt.Errorf("line %d: style block must be outside view {}", lineNumber)
			}
			if line == "}" {
				component.Blocks.ViewBody = strings.TrimSpace(strings.Join(viewBody, "\n"))
				component.Blocks.Spans.ViewBodyStart = sourceBodyStart(viewBody, lineNumber-len(viewBody))
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
				return gwdkir.Component{}, fmt.Errorf("line %d: invalid prop declaration %q", lineNumber, line)
			}
			if match[2] != "string" {
				return gwdkir.Component{}, fmt.Errorf("line %d: prop %s uses unsupported type %q", lineNumber, match[1], match[2])
			}
			component.Props = append(component.Props, gwdkir.Prop{Name: match[1], Type: match[2], Span: sourceLineSpan(lineNumber, rawLine)})
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
				return gwdkir.Component{}, fmt.Errorf("line %d: invalid export declaration %q", lineNumber, line)
			}
			if !supportedScalarType(match[2]) {
				return gwdkir.Component{}, fmt.Errorf("line %d: export %s uses unsupported type %q", lineNumber, match[1], match[2])
			}
			component.Exports = append(component.Exports, gwdkir.Export{Name: match[1], Type: match[2], Span: sourceLineSpan(lineNumber, rawLine)})
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
				return gwdkir.Component{}, err
			}
			component.Emits = append(component.Emits, event)
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if match := packagePattern.FindStringSubmatch(line); match != nil {
			if seenDeclaration {
				return gwdkir.Component{}, fmt.Errorf("line %d: package declaration must be the first non-comment declaration", lineNumber)
			}
			component.Package = match[1]
			component.PackageSpan = sourceLineSpan(lineNumber, rawLine)
			seenDeclaration = true
			continue
		}
		if strings.HasPrefix(line, "package ") {
			return gwdkir.Component{}, fmt.Errorf("line %d: malformed package declaration %q", lineNumber, line)
		}
		seenDeclaration = true

		if match := importPattern.FindStringSubmatch(line); match != nil {
			component.Imports = append(component.Imports, gwdkir.Import{
				Alias: match[1],
				Path:  match[2],
				Span:  sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedImport(line) {
			return gwdkir.Component{}, fmt.Errorf("line %d: malformed import %q", lineNumber, line)
		}
		if match := usePattern.FindStringSubmatch(line); match != nil {
			component.Uses = append(component.Uses, gwdkir.Use{
				Alias:   match[1],
				Package: match[2],
				Span:    sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedUse(line) {
			return gwdkir.Component{}, fmt.Errorf("line %d: malformed use %q", lineNumber, line)
		}
		if match := jsPattern.FindStringSubmatch(line); match != nil {
			component.JS = append(component.JS, match[1])
			component.Spans.JS = append(component.Spans.JS, source.NamedSpan{Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)})
			continue
		}
		if jsBlockPattern.MatchString(line) {
			span := sourceLineSpan(lineNumber, rawLine)
			name := source.InlineScriptName(len(component.InlineJS))
			component.Spans.InlineJS = append(component.Spans.InlineJS, source.NamedSpan{Name: name, Span: span})
			inJS = true
			jsDepth = 1
			blockScanner = braceScanner{lang: braceLangJS}
			continue
		}
		if isMalformedJS(line) {
			return gwdkir.Component{}, fmt.Errorf("line %d: malformed js declaration %q", lineNumber, line)
		}

		if strings.HasPrefix(line, "@") {
			match := annotationPattern.FindStringSubmatch(line)
			if match == nil {
				return gwdkir.Component{}, fmt.Errorf("line %d: malformed annotation %q", lineNumber, line)
			}
			if err := applyComponentAnnotation(&component, match[1], match[2], lineNumber, rawLine); err != nil {
				return gwdkir.Component{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			continue
		}

		if match := componentTypePattern.FindStringSubmatch(line); match != nil {
			span := sourceLineSpan(lineNumber, rawLine)
			kind := match[1]
			typeRef := gwdkir.GoRef{Alias: match[2], Name: match[3], Span: span}
			initRef := gwdkir.GoRef{Alias: match[4], Name: match[5], Span: span}
			switch kind {
			case "props":
				if initRef.Name != "" {
					return gwdkir.Component{}, fmt.Errorf("line %d: props contract must not declare an init function", lineNumber)
				}
				if component.PropsType.Name != "" || len(component.Props) > 0 {
					return gwdkir.Component{}, fmt.Errorf("line %d: component declares multiple props contracts", lineNumber)
				}
				component.PropsType = typeRef
			case "state":
				if initRef.Name == "" {
					return gwdkir.Component{}, fmt.Errorf("line %d: state contract requires an init function", lineNumber)
				}
				if component.State.Type.Name != "" {
					return gwdkir.Component{}, fmt.Errorf("line %d: component declares multiple state contracts", lineNumber)
				}
				component.State = gwdkir.StateContract{Type: typeRef, Init: initRef, Span: span}
			}
			continue
		}

		switch line {
		case "props {":
			if component.PropsType.Name != "" || len(component.Props) > 0 {
				return gwdkir.Component{}, fmt.Errorf("line %d: component declares multiple props contracts", lineNumber)
			}
			inProps = true
			continue
		case "exports {":
			if len(component.Exports) > 0 {
				return gwdkir.Component{}, fmt.Errorf("line %d: component declares multiple exports blocks", lineNumber)
			}
			component.Blocks.Spans.Exports = sourceLineSpan(lineNumber, rawLine)
			inExports = true
			continue
		case "client {":
			if component.Blocks.Client {
				return gwdkir.Component{}, fmt.Errorf("line %d: component declares multiple client blocks", lineNumber)
			}
			component.Blocks.Client = true
			component.Blocks.Spans.Client = sourceLineSpan(lineNumber, rawLine)
			inClient = true
			clientDepth = 1
			blockScanner = braceScanner{lang: braceLangJS}
			continue
		case "go {":
			span := sourceLineSpan(lineNumber, rawLine)
			if first, exists := seenGoBlocks[""]; exists {
				return gwdkir.Component{}, fmt.Errorf("line %d: duplicate go block; first declared on line %d", lineNumber, first.Start.Line)
			}
			seenGoBlocks[""] = span
			inGoBlock = true
			goBlockDepth = 1
			goBlockTarget = ""
			blockScanner = braceScanner{lang: braceLangGo}
			continue
		case "emits {":
			if len(component.Emits) > 0 {
				return gwdkir.Component{}, fmt.Errorf("line %d: component declares multiple emits blocks", lineNumber)
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
				return gwdkir.Component{}, fmt.Errorf("line %d: component declares multiple style blocks", lineNumber)
			}
			component.Blocks.Style = true
			inStyle = true
			styleDepth = 1
			blockScanner = braceScanner{lang: braceLangCSS}
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
				return gwdkir.Component{}, fmt.Errorf("line %d: duplicate %s block; first declared on line %d", lineNumber, label, first.Start.Line)
			}
			seenGoBlocks[target] = span
			inGoBlock = true
			goBlockDepth = 1
			goBlockTarget = target
			blockScanner = braceScanner{lang: braceLangGo}
			continue
		}

		if name := unsupportedTopLevelBlockName(line); name != "" {
			return gwdkir.Component{}, fmt.Errorf("line %d: unsupported top-level block %q", lineNumber, name)
		}
	}
	if err := scanner.Err(); err != nil {
		return gwdkir.Component{}, err
	}
	if inView {
		return gwdkir.Component{}, fmt.Errorf("view block missing closing }")
	}
	if inStyle {
		return gwdkir.Component{}, fmt.Errorf("style block missing closing }")
	}
	if inProps {
		return gwdkir.Component{}, fmt.Errorf("props block missing closing }")
	}
	if inExports {
		return gwdkir.Component{}, fmt.Errorf("exports block missing closing }")
	}
	if inEmits {
		return gwdkir.Component{}, fmt.Errorf("emits block missing closing }")
	}
	if inClient {
		return gwdkir.Component{}, fmt.Errorf("client block missing closing }")
	}
	if inJS {
		return gwdkir.Component{}, fmt.Errorf("js block missing closing }")
	}
	if inGoBlock {
		return gwdkir.Component{}, fmt.Errorf("go block missing closing }")
	}
	if component.Name == "" {
		return gwdkir.Component{}, fmt.Errorf("missing @component")
	}
	return component, nil
}

func supportedScalarType(value string) bool {
	return value == "string" || value == "int" || value == "float" || value == "bool"
}

func parseEmitDeclaration(line string, lineNumber int, rawLine string) (gwdkir.Emit, error) {
	match := emitPattern.FindStringSubmatch(line)
	if match == nil {
		return gwdkir.Emit{}, fmt.Errorf("line %d: invalid emit declaration %q", lineNumber, line)
	}
	params, err := parseEmitParams(match[2], lineNumber, rawLine)
	if err != nil {
		return gwdkir.Emit{}, err
	}
	return gwdkir.Emit{Name: match[1], Params: params, Span: sourceLineSpan(lineNumber, rawLine)}, nil
}

func parseEmitParams(src string, lineNumber int, rawLine string) ([]gwdkir.EmitParam, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return nil, nil
	}
	parts := strings.Split(src, ",")
	params := make([]gwdkir.EmitParam, 0, len(parts))
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
		params = append(params, gwdkir.EmitParam{Name: name, Type: typ, Span: sourceLineSpan(lineNumber, rawLine)})
	}
	return params, nil
}

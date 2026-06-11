package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/cssscope"
	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/view"
)

type SyntaxFile = gwdkast.File
type SyntaxPackage = gwdkast.Package
type SyntaxMetadata = gwdkast.MetadataDecl
type SyntaxImport = gwdkast.Import
type SyntaxUse = gwdkast.Use
type SyntaxBlock = gwdkast.Block
type SyntaxEndpoint = gwdkast.Endpoint
type SyntaxFragmentEndpoint = gwdkast.FragmentEndpoint
type GoTypeRef = gwdkast.GoTypeRef
type GoFuncRef = gwdkast.GoFuncRef
type StateContract = gwdkast.StateContract
type WASMContract = gwdkast.WASMContract
type LiteralRecord = gwdkast.LiteralRecord
type BuildCall = gwdkast.BuildCall
type Prop = gwdkast.Prop
type Export = gwdkast.Export
type Emit = gwdkast.Emit
type EmitParam = gwdkast.EmitParam
type ActionStatement = gwdkast.ActionStatement
type APIStatement = gwdkast.APIStatement

// ParseSyntax parses a .gwdk source file into a typed syntax AST for the
// current compiler subset.
func ParseSyntax(src []byte) (SyntaxFile, error) {
	var file SyntaxFile
	var body []syntaxBodyLine
	var captured SyntaxBlock
	var capturedFragment *SyntaxFragmentEndpoint
	var blockScanner braceScanner
	depth := 0
	seenDeclaration := false
	seenGoBlocks := map[string]source.SourceSpan{}

	scanner := bufio.NewScanner(bytes.NewReader(src))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if capturedFragment != nil {
			if line == "}" {
				capturedFragment.Body = strings.TrimSpace(joinSyntaxBody(body))
				file.Fragments = append(file.Fragments, *capturedFragment)
				capturedFragment = nil
				body = nil
				continue
			}
			body = append(body, syntaxBodyLine{Text: rawLine, Line: lineNumber})
			continue
		}
		if captured.Kind != "" {
			if captured.Kind == "view" && line == "style {" {
				return SyntaxFile{}, fmt.Errorf("line %d: style block must be outside view {}", lineNumber)
			}
			if line == "}" && !blockScanner.inMultiline() {
				depth--
				if depth == 0 {
					if captured.Kind == "js" {
						file.JS = append(file.JS, gwdkast.AssetRef{
							Kind:   "js",
							Inline: strings.TrimSpace(joinSyntaxBody(body)),
							Span:   captured.Span,
						})
						captured = SyntaxBlock{}
						body = nil
						continue
					}
					block, err := finishSyntaxBlock(captured, body)
					if err != nil {
						return SyntaxFile{}, err
					}
					file.Blocks = append(file.Blocks, block)
					captured = SyntaxBlock{}
					body = nil
					continue
				}
				body = append(body, syntaxBodyLine{Text: rawLine, Line: lineNumber})
				continue
			}
			if captured.Kind == "act" && actionFragmentPattern.FindStringSubmatch(line) != nil {
				depth++
			}
			if captured.Kind == "client" {
				depth += blockScanner.delta(rawLine)
				if depth < 1 {
					return SyntaxFile{}, fmt.Errorf("line %d: client block closed unexpectedly", lineNumber)
				}
			}
			if captured.Kind == "go" {
				depth += blockScanner.delta(rawLine)
				if depth < 1 {
					return SyntaxFile{}, fmt.Errorf("line %d: go block closed unexpectedly", lineNumber)
				}
			}
			if captured.Kind == "style" {
				depth += blockScanner.delta(rawLine)
				if depth < 1 {
					return SyntaxFile{}, fmt.Errorf("line %d: style block closed unexpectedly", lineNumber)
				}
			}
			if captured.Kind == "js" {
				depth += blockScanner.delta(rawLine)
				if depth < 1 {
					return SyntaxFile{}, fmt.Errorf("line %d: js block closed unexpectedly", lineNumber)
				}
			}
			body = append(body, syntaxBodyLine{Text: rawLine, Line: lineNumber})
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if match := packagePattern.FindStringSubmatch(line); match != nil {
			if seenDeclaration {
				return SyntaxFile{}, fmt.Errorf("line %d: package declaration must be the first non-comment declaration", lineNumber)
			}
			pkg := SyntaxPackage{Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)}
			file.Package = &pkg
			seenDeclaration = true
			continue
		}
		if strings.HasPrefix(line, "package ") {
			return SyntaxFile{}, fmt.Errorf("line %d: malformed package declaration %q", lineNumber, line)
		}
		seenDeclaration = true
		if match := metadataPattern.FindStringSubmatch(line); match != nil {
			metadata := SyntaxMetadata{
				Name:  match[1],
				Value: strings.TrimSpace(match[2]),
				Span:  sourceLineSpan(lineNumber, rawLine),
			}
			if err := applySyntaxMetadata(&file, metadata, lineNumber, rawLine); err != nil {
				return SyntaxFile{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			if match[1] == "wasm" {
				file.WASM = &WASMContract{
					Package: strings.TrimSpace(match[2]),
					Span:    sourceLineSpan(lineNumber, rawLine),
				}
			}
			file.Metadata = append(file.Metadata, metadata)
			continue
		}
		if strings.HasPrefix(line, "@") {
			return SyntaxFile{}, fmt.Errorf("line %d: malformed legacy metadata %q", lineNumber, line)
		}
		if match := importPattern.FindStringSubmatch(line); match != nil {
			file.Imports = append(file.Imports, SyntaxImport{
				Alias: match[1],
				Path:  match[2],
				Span:  sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedImport(line) {
			return SyntaxFile{}, fmt.Errorf("line %d: malformed import %q", lineNumber, line)
		}
		if match := usePattern.FindStringSubmatch(line); match != nil {
			file.Uses = append(file.Uses, SyntaxUse{
				Alias:   match[1],
				Package: match[2],
				Span:    sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if isMalformedUse(line) {
			return SyntaxFile{}, fmt.Errorf("line %d: malformed use %q", lineNumber, line)
		}
		if match := jsPattern.FindStringSubmatch(line); match != nil {
			file.JS = append(file.JS, gwdkast.AssetRef{
				Kind: "js",
				Path: match[1],
				Span: sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if jsBlockPattern.MatchString(line) {
			captured = SyntaxBlock{Kind: "js", Span: sourceLineSpan(lineNumber, rawLine)}
			blockScanner = braceScanner{lang: braceLangJS}
			depth = 1
			continue
		}
		if isMalformedJS(line) {
			return SyntaxFile{}, fmt.Errorf("line %d: malformed js declaration %q", lineNumber, line)
		}
		if match := storePattern.FindStringSubmatch(line); match != nil {
			span := sourceLineSpan(lineNumber, rawLine)
			file.Stores = append(file.Stores, gwdkast.Store{
				Name: match[1],
				Type: GoTypeRef{Alias: match[2], Name: match[3], Span: span},
				Init: GoFuncRef{Alias: match[4], Name: match[5], Span: span},
				Span: span,
			})
			continue
		}
		if match := componentTypePattern.FindStringSubmatch(line); match != nil {
			span := sourceLineSpan(lineNumber, rawLine)
			typeRef := GoTypeRef{Alias: match[2], Name: match[3], Span: span}
			switch match[1] {
			case "props":
				if match[4] != "" || match[5] != "" {
					return SyntaxFile{}, fmt.Errorf("line %d: props contract must not declare an init function", lineNumber)
				}
				file.PropsType = &typeRef
			case "state":
				if match[4] == "" || match[5] == "" {
					return SyntaxFile{}, fmt.Errorf("line %d: state contract requires an init function", lineNumber)
				}
				file.State = &StateContract{
					Type: typeRef,
					Init: GoFuncRef{Alias: match[4], Name: match[5], Span: span},
					Span: span,
				}
			}
			continue
		}
		if match := syntaxBlockPattern.FindStringSubmatch(line); match != nil {
			captured = SyntaxBlock{Kind: match[1], Span: sourceLineSpan(lineNumber, rawLine)}
			blockScanner = braceScanner{lang: blockScanLang(match[1])}
			depth = 1
			continue
		}
		if match := goBlockPattern.FindStringSubmatch(line); match != nil {
			target := strings.TrimSpace(match[1])
			if span, exists := seenGoBlocks[target]; exists {
				label := "go"
				if target != "" {
					label = "go " + target
				}
				return SyntaxFile{}, fmt.Errorf("line %d: duplicate %s block; first declared on line %d", lineNumber, label, span.Start.Line)
			}
			span := sourceLineSpan(lineNumber, rawLine)
			seenGoBlocks[target] = span
			captured = SyntaxBlock{Kind: "go", Name: target, Span: span}
			blockScanner = braceScanner{lang: braceLangGo}
			depth = 1
			continue
		}
		if match := actionEndpointPattern.FindStringSubmatch(line); match != nil {
			if !isExportedIdentifier(match[1]) {
				return SyntaxFile{}, fmt.Errorf("line %d: action handler %q must be an exported Go identifier", lineNumber, match[1])
			}
			if match[2] != "POST" {
				return SyntaxFile{}, fmt.Errorf("line %d: action %s uses unsupported method %s; actions currently require POST", lineNumber, match[1], match[2])
			}
			errorPage, err := endpointErrorPage(match, lineNumber)
			if err != nil {
				return SyntaxFile{}, err
			}
			file.Actions = append(file.Actions, SyntaxEndpoint{
				Kind:          "act",
				Name:          match[1],
				Method:        match[2],
				Route:         match[3],
				ErrorPage:     errorPage,
				Span:          sourceLineSpan(lineNumber, rawLine),
				ErrorPageSpan: endpointErrorPageSpan(match, sourceLineSpan(lineNumber, rawLine)),
			})
			continue
		}
		if match := actionPattern.FindStringSubmatch(line); match != nil {
			return SyntaxFile{}, fmt.Errorf("line %d: old action block syntax is not supported; use `act %s POST \"<path>\"` and move behavior to Go", lineNumber, exportedIdentifierSuggestion(match[1]))
		}
		if match := apiEndpointPattern.FindStringSubmatch(line); match != nil {
			if !isExportedIdentifier(match[1]) {
				return SyntaxFile{}, fmt.Errorf("line %d: API handler %q must be an exported Go identifier", lineNumber, match[1])
			}
			errorPage, err := endpointErrorPage(match, lineNumber)
			if err != nil {
				return SyntaxFile{}, err
			}
			file.APIs = append(file.APIs, SyntaxEndpoint{
				Kind:          "api",
				Name:          match[1],
				Method:        match[2],
				Route:         match[3],
				ErrorPage:     errorPage,
				Span:          sourceLineSpan(lineNumber, rawLine),
				ErrorPageSpan: endpointErrorPageSpan(match, sourceLineSpan(lineNumber, rawLine)),
			})
			continue
		}
		if match := apiPattern.FindStringSubmatch(line); match != nil {
			return SyntaxFile{}, fmt.Errorf("line %d: old API block syntax is not supported; use `api %s GET \"<path>\"` and move behavior to Go", lineNumber, exportedIdentifierSuggestion(match[1]))
		}
		if match := fragmentEndpointPattern.FindStringSubmatch(line); match != nil {
			if match[2] != "GET" {
				return SyntaxFile{}, fmt.Errorf("line %d: fragment %s uses unsupported method %s; fragments currently require GET", lineNumber, match[1], match[2])
			}
			if err := validateFragmentTarget(match[4]); err != nil {
				return SyntaxFile{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			span := sourceLineSpan(lineNumber, rawLine)
			capturedFragment = &SyntaxFragmentEndpoint{
				Name:       match[1],
				Method:     match[2],
				Route:      match[3],
				Target:     match[4],
				Span:       span,
				RouteSpan:  span,
				TargetSpan: span,
			}
			body = nil
			continue
		}
		if name := unsupportedTopLevelBlockName(line); name != "" {
			return SyntaxFile{}, fmt.Errorf("line %d: unsupported top-level block %q", lineNumber, name)
		}
	}
	if err := scanner.Err(); err != nil {
		return SyntaxFile{}, err
	}
	if captured.Kind != "" {
		return SyntaxFile{}, fmt.Errorf("%s block missing closing }", captured.Kind)
	}
	if capturedFragment != nil {
		return SyntaxFile{}, fmt.Errorf("fragment %s block missing closing }", capturedFragment.Name)
	}
	attachSyntaxAssetScopes(&file)
	return file, nil
}

func attachSyntaxAssetScopes(file *SyntaxFile) {
	if file.Component == nil {
		return
	}
	packageName := ""
	if file.Package != nil {
		packageName = file.Package.Name
	}
	for index := range file.CSS {
		if file.CSS[index].Kind != "css" {
			continue
		}
		hashKey := cssscope.HashKey("component", packageName, file.Component.Name, "", file.CSS[index].Path)
		file.CSS[index].Scope = gwdkast.AssetScope{
			OwnerKind: "component",
			OwnerID:   file.Component.Name,
			Package:   packageName,
			ScopeID:   cssscope.ScopeID(hashKey),
			HashKey:   hashKey,
		}
	}
}

func applySyntaxMetadata(file *SyntaxFile, metadata SyntaxMetadata, lineNumber int, rawLine string) error {
	value := strings.TrimSpace(metadata.Value)
	switch metadata.Name {
	case "page":
		file.Page = &gwdkast.PageDecl{ID: value, Span: metadata.Span}
	case "component":
		file.Component = &gwdkast.ComponentDecl{Name: value, Span: metadata.Span}
	case "layout":
		values := splitList(value)
		if len(values) == 1 {
			file.Layout = &gwdkast.LayoutDecl{ID: values[0], Span: metadata.Span}
		}
		for _, span := range namedValueSpans(values, lineNumber, rawLine) {
			file.Layouts = append(file.Layouts, gwdkast.LayoutRef{ID: span.Name, Span: span.Span})
		}
	case "route":
		path, params, _, err := parseRouteDeclaration(trimQuotes(value), lineNumber, rawLine)
		if err != nil {
			return err
		}
		routeParams := make([]gwdkast.RouteParam, 0, len(params))
		for _, param := range params {
			routeParams = append(routeParams, gwdkast.RouteParam{Name: param.Name, Type: param.Type, Span: param.Span})
		}
		file.Route = &gwdkast.RouteDecl{Path: path, Params: routeParams, Span: metadata.Span}
	case "cache":
		file.Cache = &gwdkast.CacheDecl{Policy: trimQuotes(value), Span: metadata.Span}
	case "revalidate":
		seconds, err := revalidateSecondsValue(value)
		if err != nil {
			return err
		}
		file.Revalidate = &gwdkast.RevalidateDecl{Seconds: seconds, Span: metadata.Span}
	case "error":
		errorPage, err := source.ErrorPagePath(trimQuotes(value))
		if err != nil {
			return err
		}
		file.ErrorPage = &gwdkast.ErrorPageDecl{Path: errorPage, Span: metadata.Span}
	case "guard":
		for _, span := range namedValueSpans(splitList(value), lineNumber, rawLine) {
			file.Guards = append(file.Guards, gwdkast.GuardRef{Name: span.Name, Span: span.Span})
		}
	case "css":
		for _, span := range namedValueSpans(splitCSSList(value), lineNumber, rawLine) {
			file.CSS = append(file.CSS, gwdkast.AssetRef{Kind: "css", Path: span.Name, Span: span.Span})
		}
	case "asset":
		for _, span := range namedValueSpans(splitCSSList(value), lineNumber, rawLine) {
			file.Assets = append(file.Assets, gwdkast.AssetRef{Kind: "asset", Path: span.Name, Span: span.Span})
		}
	}
	return nil
}

type syntaxBodyLine struct {
	Text string
	Line int
}

func finishSyntaxBlock(block SyntaxBlock, body []syntaxBodyLine) (SyntaxBlock, error) {
	block.Body = strings.TrimSpace(joinSyntaxBody(body))
	block.BodyStart = syntaxBodyStart(body)
	switch block.Kind {
	case "view":
		nodes, err := view.Parse(block.Body)
		if err != nil {
			return SyntaxBlock{}, fmt.Errorf("line %d: view body: %w", block.Span.Start.Line, err)
		}
		block.View = nodes
	case "style":
		block.StyleBody = block.Body
	case "paths":
		records, err := parseLiteralRecords(body)
		if err != nil {
			return SyntaxBlock{}, err
		}
		block.Records = records
	case "build":
		call, ok, err := parseBuildCall(body)
		if err != nil {
			return SyntaxBlock{}, err
		}
		if ok {
			block.Call = &call
			return block, nil
		}
		records, err := parseLiteralRecords(body)
		if err != nil {
			return SyntaxBlock{}, err
		}
		block.Records = records
	case "load":
	case "client":
	case "props":
		props, err := parseSyntaxProps(body)
		if err != nil {
			return SyntaxBlock{}, err
		}
		block.Props = props
	case "exports":
		exports, err := parseSyntaxExports(body)
		if err != nil {
			return SyntaxBlock{}, err
		}
		block.Exports = exports
	case "emits":
		emits, err := parseSyntaxEmits(body)
		if err != nil {
			return SyntaxBlock{}, err
		}
		block.Emits = emits
	case "act":
		statements, err := parseActionStatements(block.Name, body)
		if err != nil {
			return SyntaxBlock{}, err
		}
		block.Actions = statements
	case "api":
		statements, err := parseAPIStatements(block.Name, body)
		if err != nil {
			return SyntaxBlock{}, err
		}
		block.APIs = statements
	}
	return block, nil
}

func syntaxBodyStart(body []syntaxBodyLine) source.SourcePosition {
	for _, raw := range body {
		for index, char := range []rune(raw.Text) {
			if strings.TrimSpace(string(char)) == "" {
				continue
			}
			return source.SourcePosition{Line: raw.Line, Column: index + 1}
		}
	}
	return source.SourcePosition{}
}

func parseBuildCall(body []syntaxBodyLine) (BuildCall, bool, error) {
	var significant []syntaxBodyLine
	for _, raw := range body {
		line := strings.TrimSpace(raw.Text)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		significant = append(significant, raw)
	}
	if len(significant) != 1 {
		return BuildCall{}, false, nil
	}
	line := strings.TrimSpace(significant[0].Text)
	match := buildCallPattern.FindStringSubmatch(line)
	if match == nil {
		return BuildCall{}, false, nil
	}
	return BuildCall{
		Alias:    match[1],
		Function: match[2],
		Span:     sourceLineSpan(significant[0].Line, significant[0].Text),
	}, true, nil
}

func joinSyntaxBody(body []syntaxBodyLine) string {
	lines := make([]string, 0, len(body))
	for _, line := range body {
		lines = append(lines, line.Text)
	}
	return strings.Join(lines, "\n")
}

func parseLiteralRecords(body []syntaxBodyLine) ([]LiteralRecord, error) {
	var records []LiteralRecord
	for _, raw := range body {
		line := strings.TrimSpace(raw.Text)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		match := literalRecordPattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("line %d: unsupported literal record syntax %q", raw.Line, line)
		}
		fields, err := parseLiteralRecordFields(match[1])
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", raw.Line, err)
		}
		records = append(records, LiteralRecord{Fields: fields, Span: sourceLineSpan(raw.Line, raw.Text)})
	}
	return records, nil
}

func parseLiteralRecordFields(body string) (map[string]string, error) {
	fields := map[string]string{}
	for _, part := range strings.Split(body, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, value, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("literal record field %q must use name: \"value\"", part)
		}
		name = strings.TrimSpace(name)
		value = trimQuotes(value)
		if name == "" {
			return nil, fmt.Errorf("literal record field name is required")
		}
		fields[name] = value
	}
	return fields, nil
}

func parseSyntaxProps(body []syntaxBodyLine) ([]Prop, error) {
	var props []Prop
	for _, raw := range body {
		line := strings.TrimSpace(raw.Text)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		match := propPattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("line %d: unsupported prop syntax %q", raw.Line, line)
		}
		if !supportedSyntaxPropType(match[2]) {
			return nil, fmt.Errorf("line %d: unsupported prop type %q", raw.Line, match[2])
		}
		props = append(props, Prop{Name: match[1], Type: match[2], Span: sourceLineSpan(raw.Line, raw.Text)})
	}
	return props, nil
}

func supportedSyntaxPropType(value string) bool {
	return value == "string"
}

func parseSyntaxExports(body []syntaxBodyLine) ([]Export, error) {
	var exports []Export
	for _, raw := range body {
		line := strings.TrimSpace(raw.Text)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		match := propPattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("line %d: unsupported export syntax %q", raw.Line, line)
		}
		if !supportedScalarType(match[2]) {
			return nil, fmt.Errorf("line %d: unsupported export type %q", raw.Line, match[2])
		}
		exports = append(exports, Export{Name: match[1], Type: match[2], Span: sourceLineSpan(raw.Line, raw.Text)})
	}
	return exports, nil
}

func parseSyntaxEmits(body []syntaxBodyLine) ([]Emit, error) {
	var emits []Emit
	for _, raw := range body {
		line := strings.TrimSpace(raw.Text)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		match := emitPattern.FindStringSubmatch(line)
		if match == nil {
			return nil, fmt.Errorf("line %d: unsupported emit syntax %q", raw.Line, line)
		}
		params, err := parseEmitParams(match[2], raw.Line, raw.Text)
		if err != nil {
			return nil, err
		}
		outParams := make([]EmitParam, 0, len(params))
		for _, param := range params {
			outParams = append(outParams, EmitParam{Name: param.Name, Type: param.Type, Span: param.Span})
		}
		emits = append(emits, Emit{Name: match[1], Params: outParams, Span: sourceLineSpan(raw.Line, raw.Text)})
	}
	return emits, nil
}

func parseActionStatements(actionName string, body []syntaxBodyLine) ([]ActionStatement, error) {
	var statements []ActionStatement
	for index := 0; index < len(body); index++ {
		raw := body[index]
		line := strings.TrimSpace(raw.Text)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		span := sourceLineSpan(raw.Line, raw.Text)
		if match := actionFragmentPattern.FindStringSubmatch(line); match != nil {
			fragmentBody, next, err := parseActionStatementFragment(actionName, body, index, match[1])
			if err != nil {
				return nil, err
			}
			statements = append(statements, ActionStatement{Kind: "fragment", Target: match[1], Body: fragmentBody, Span: span})
			index = next
			continue
		}
		if match := actionInputPattern.FindStringSubmatch(line); match != nil {
			statements = append(statements, ActionStatement{Kind: "input", InputName: match[1], InputType: match[2], Span: span})
			continue
		}
		if match := actionValidPattern.FindStringSubmatch(line); match != nil {
			statements = append(statements, ActionStatement{Kind: "valid", Name: match[1], Span: span})
			continue
		}
		if match := actionRedirectPattern.FindStringSubmatch(line); match != nil {
			statements = append(statements, ActionStatement{Kind: "redirect", Redirect: match[1], Span: span})
			continue
		}
		return nil, fmt.Errorf("line %d: action %s has unsupported syntax %q", raw.Line, actionName, line)
	}
	return statements, nil
}

func parseActionStatementFragment(actionName string, body []syntaxBodyLine, start int, target string) (string, int, error) {
	var lines []string
	for index := start + 1; index < len(body); index++ {
		line := strings.TrimSpace(body[index].Text)
		if line == "}" {
			return strings.TrimSpace(strings.Join(lines, "\n")), index, nil
		}
		lines = append(lines, body[index].Text)
	}
	return "", start, fmt.Errorf("line %d: action %s fragment %q missing closing }", body[start].Line, actionName, target)
}

func parseAPIStatements(apiName string, body []syntaxBodyLine) ([]APIStatement, error) {
	var statements []APIStatement
	for _, raw := range body {
		line := strings.TrimSpace(raw.Text)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if match := apiRoutePattern.FindStringSubmatch(line); match != nil {
			statements = append(statements, APIStatement{
				Method: match[1],
				Route:  match[2],
				Span:   sourceLineSpan(raw.Line, raw.Text),
			})
			continue
		}
		return nil, fmt.Errorf("line %d: api %s has unsupported syntax %q", raw.Line, apiName, line)
	}
	return statements, nil
}

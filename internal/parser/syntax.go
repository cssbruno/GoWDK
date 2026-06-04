package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/cssbruno/gowdk/internal/manifest"
	"github.com/cssbruno/gowdk/internal/view"
)

var literalRecordPattern = regexp.MustCompile(`^=>\s*\{(.*)\}$`)

// SyntaxFile is the typed AST for the currently supported .gwdk syntax subset.
type SyntaxFile struct {
	Annotations []SyntaxAnnotation
	Blocks      []SyntaxBlock
}

// SyntaxAnnotation is one top-level @annotation.
type SyntaxAnnotation struct {
	Name  string
	Value string
	Span  manifest.SourceSpan
}

// SyntaxBlock is one parsed top-level block.
type SyntaxBlock struct {
	Kind    string
	Name    string
	Body    string
	Span    manifest.SourceSpan
	View    []view.Node
	Records []LiteralRecord
	Actions []ActionStatement
	APIs    []APIStatement
}

// LiteralRecord is a first-slice paths/build return record.
type LiteralRecord struct {
	Fields map[string]string
	Span   manifest.SourceSpan
}

// ActionStatement is one supported statement inside act {}.
type ActionStatement struct {
	Kind      string
	Name      string
	InputName string
	InputType string
	Target    string
	Redirect  string
	Body      string
	Span      manifest.SourceSpan
}

// APIStatement is one supported statement inside api {}.
type APIStatement struct {
	Method string
	Route  string
	Span   manifest.SourceSpan
}

// ParseSyntax parses a .gwdk source file into a typed syntax AST for the
// current compiler subset.
func ParseSyntax(source []byte) (SyntaxFile, error) {
	var file SyntaxFile
	var body []syntaxBodyLine
	var captured SyntaxBlock
	depth := 0

	scanner := bufio.NewScanner(bytes.NewReader(source))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if captured.Kind != "" {
			if line == "}" {
				depth--
				if depth == 0 {
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
			body = append(body, syntaxBodyLine{Text: rawLine, Line: lineNumber})
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if strings.HasPrefix(line, "@") {
			match := annotationPattern.FindStringSubmatch(line)
			if match == nil {
				return SyntaxFile{}, fmt.Errorf("line %d: malformed annotation %q", lineNumber, line)
			}
			file.Annotations = append(file.Annotations, SyntaxAnnotation{
				Name:  match[1],
				Value: strings.TrimSpace(match[2]),
				Span:  sourceLineSpan(lineNumber, rawLine),
			})
			continue
		}
		if match := blockPattern.FindStringSubmatch(line); match != nil {
			captured = SyntaxBlock{Kind: match[1], Span: sourceLineSpan(lineNumber, rawLine)}
			depth = 1
			continue
		}
		if match := actionPattern.FindStringSubmatch(line); match != nil {
			captured = SyntaxBlock{Kind: "act", Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)}
			depth = 1
			continue
		}
		if match := apiPattern.FindStringSubmatch(line); match != nil {
			captured = SyntaxBlock{Kind: "api", Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)}
			depth = 1
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
	return file, nil
}

type syntaxBodyLine struct {
	Text string
	Line int
}

func finishSyntaxBlock(block SyntaxBlock, body []syntaxBodyLine) (SyntaxBlock, error) {
	block.Body = strings.TrimSpace(joinSyntaxBody(body))
	switch block.Kind {
	case "view":
		nodes, err := view.Parse(block.Body)
		if err != nil {
			return SyntaxBlock{}, fmt.Errorf("line %d: view body: %w", block.Span.Start.Line, err)
		}
		block.View = nodes
	case "paths", "build":
		records, err := parseLiteralRecords(body)
		if err != nil {
			return SyntaxBlock{}, err
		}
		block.Records = records
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

package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"strings"

	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/gwdkir"
	"github.com/cssbruno/gowdk/internal/source"
	"github.com/cssbruno/gowdk/internal/syntax"
)

type capturedAuditBlock struct {
	kind    string
	name    string
	extends []string
	span    source.SourceSpan
	body    []syntaxBodyLine
	scanner braceScanner
	depth   int
}

// ParseAuditFile parses a *.audit.gwdk source into audit IR.
func ParseAuditFile(path string, src []byte) (gwdkir.AuditSpec, error) {
	ast, err := ParseAuditSyntax(src)
	if err != nil {
		return gwdkir.AuditSpec{}, err
	}
	return lowerAuditSyntax(path, ast), nil
}

// ParseAuditSyntax parses the dedicated audit policy file kind.
func ParseAuditSyntax(src []byte) (gwdkast.AuditFile, error) {
	var file gwdkast.AuditFile
	var parseErrors []error
	addError := func(err error) {
		if err != nil {
			parseErrors = append(parseErrors, err)
		}
	}

	var captured *capturedAuditBlock
	seenDeclaration := false

	scanner := bufio.NewScanner(bytes.NewReader(src))
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)
		if captured != nil {
			if line == "}" && !captured.scanner.inMultiline() && captured.depth == 1 {
				addError(finishAuditBlock(&file, *captured))
				captured = nil
				continue
			}
			captured.depth += captured.scanner.delta(rawLine)
			if captured.depth <= 0 {
				addError(finishAuditBlock(&file, *captured))
				captured = nil
				continue
			}
			captured.body = append(captured.body, syntaxBodyLine{Text: rawLine, Line: lineNumber})
			continue
		}

		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if match := packagePattern.FindStringSubmatch(line); match != nil {
			if seenDeclaration {
				addError(lineDiagnosticError(DiagnosticPackageMustBeFirst, lineNumber, rawLine, "package declaration must be the first non-comment declaration"))
				continue
			}
			pkg := gwdkast.Package{Name: match[1], Span: sourceLineSpan(lineNumber, rawLine)}
			file.Package = &pkg
			seenDeclaration = true
			continue
		}
		seenDeclaration = true
		if policy, ok, err := parseAuditPolicyStart(line, lineNumber, rawLine); ok || err != nil {
			if err != nil {
				addError(err)
				continue
			}
			captured = &capturedAuditBlock{kind: "policy", name: policy.Name, extends: policy.Extends, span: policy.Span, scanner: braceScanner{lang: braceLangGo}, depth: 1}
			continue
		}
		if test, ok, err := parseAuditTestStart(line, lineNumber, rawLine); ok || err != nil {
			if err != nil {
				addError(err)
				continue
			}
			captured = &capturedAuditBlock{kind: "test", name: test.Name, span: test.Span, scanner: braceScanner{lang: braceLangGo}, depth: 1}
			continue
		}
		addError(fmt.Errorf("line %d: unsupported audit declaration %q", lineNumber, line))
	}
	if err := scanner.Err(); err != nil {
		addError(err)
	}
	if captured != nil {
		addError(fmt.Errorf("line %d: unterminated %s block %q", captured.span.Start.Line, captured.kind, captured.name))
	}
	if len(parseErrors) > 0 {
		return gwdkast.AuditFile{}, diagnosticErrors(parseErrors)
	}
	return file, nil
}

func parseAuditPolicyStart(line string, lineNumber int, rawLine string) (gwdkast.AuditPolicy, bool, error) {
	tokens := syntaxTokens(line)
	if len(tokens) == 0 || tokens[0].Lexeme != "policy" {
		return gwdkast.AuditPolicy{}, false, nil
	}
	if len(tokens) < 3 || tokens[len(tokens)-1].Kind != syntax.TokenLBrace {
		return gwdkast.AuditPolicy{}, true, fmt.Errorf("line %d: policy declaration must use policy <name> [extends <name>[, ...]] {", lineNumber)
	}
	name, ok := auditName(tokens[1])
	if !ok {
		return gwdkast.AuditPolicy{}, true, fmt.Errorf("line %d: policy name must be an identifier or string", lineNumber)
	}
	policy := gwdkast.AuditPolicy{Name: name, Span: sourceLineSpan(lineNumber, rawLine)}
	index := 2
	if index < len(tokens)-1 {
		if tokens[index].Lexeme != "extends" {
			return gwdkast.AuditPolicy{}, true, fmt.Errorf("line %d: unexpected policy declaration token %q", lineNumber, tokens[index].Lexeme)
		}
		index++
		for index < len(tokens)-1 {
			parent, ok := auditName(tokens[index])
			if !ok {
				return gwdkast.AuditPolicy{}, true, fmt.Errorf("line %d: extends target must be an identifier or string", lineNumber)
			}
			policy.Extends = append(policy.Extends, parent)
			index++
			if index < len(tokens)-1 {
				if tokens[index].Kind != syntax.TokenComma {
					return gwdkast.AuditPolicy{}, true, fmt.Errorf("line %d: extends targets must be comma-separated", lineNumber)
				}
				index++
			}
		}
	}
	return policy, true, nil
}

func parseAuditTestStart(line string, lineNumber int, rawLine string) (gwdkast.AuditTest, bool, error) {
	tokens := syntaxTokens(line)
	if len(tokens) == 0 || tokens[0].Lexeme != "test" {
		return gwdkast.AuditTest{}, false, nil
	}
	if len(tokens) != 3 || tokens[2].Kind != syntax.TokenLBrace {
		return gwdkast.AuditTest{}, true, fmt.Errorf("line %d: test declaration must use test <name> {", lineNumber)
	}
	name, ok := auditName(tokens[1])
	if !ok {
		return gwdkast.AuditTest{}, true, fmt.Errorf("line %d: test name must be an identifier or string", lineNumber)
	}
	return gwdkast.AuditTest{Name: name, Span: sourceLineSpan(lineNumber, rawLine)}, true, nil
}

func auditName(token syntax.Token) (string, bool) {
	switch token.Kind {
	case syntax.TokenIdentifier:
		return token.Lexeme, true
	case syntax.TokenString:
		return decodeStringLiteral(token.Lexeme), true
	default:
		return "", false
	}
}

func finishAuditBlock(file *gwdkast.AuditFile, block capturedAuditBlock) error {
	switch block.kind {
	case "policy":
		policy := gwdkast.AuditPolicy{Name: block.name, Extends: append([]string(nil), block.extends...), Span: block.span}
		for _, raw := range block.body {
			line := strings.TrimSpace(raw.Text)
			if line == "" || strings.HasPrefix(line, "//") {
				continue
			}
			if apply, ok, err := parseAuditApply(line, raw.Line, raw.Text); ok || err != nil {
				if err != nil {
					return err
				}
				policy.Applies = append(policy.Applies, apply)
				continue
			}
			if rule, ok, err := parseAuditRule(line, raw.Line, raw.Text); ok || err != nil {
				if err != nil {
					return err
				}
				policy.Rules = append(policy.Rules, rule)
				continue
			}
			return fmt.Errorf("line %d: unsupported policy syntax %q", raw.Line, line)
		}
		file.Policies = append(file.Policies, policy)
	case "test":
		file.Tests = append(file.Tests, gwdkast.AuditTest{Name: block.name, Body: strings.TrimSpace(joinSyntaxBody(block.body)), Span: block.span})
	}
	return nil
}

func parseAuditApply(line string, lineNumber int, rawLine string) (gwdkast.AuditApply, bool, error) {
	tokens := syntaxTokens(line)
	if len(tokens) == 0 {
		return gwdkast.AuditApply{}, false, nil
	}
	switch {
	case tokens[0].Lexeme == "match":
		if len(tokens) != 2 || tokens[1].Kind != syntax.TokenString {
			return gwdkast.AuditApply{}, true, fmt.Errorf("line %d: match must use match \"<selector>\"", lineNumber)
		}
		return gwdkast.AuditApply{Selector: decodeStringLiteral(tokens[1].Lexeme), Span: sourceLineSpan(lineNumber, rawLine)}, true, nil
	case tokens[0].Lexeme == "apply":
		if len(tokens) != 3 || tokens[1].Lexeme != "to" || tokens[2].Kind != syntax.TokenString {
			return gwdkast.AuditApply{}, true, fmt.Errorf("line %d: apply must use apply to \"<selector>\"", lineNumber)
		}
		return gwdkast.AuditApply{Selector: decodeStringLiteral(tokens[2].Lexeme), Span: sourceLineSpan(lineNumber, rawLine)}, true, nil
	default:
		return gwdkast.AuditApply{}, false, nil
	}
}

func parseAuditRule(line string, lineNumber int, rawLine string) (gwdkast.AuditRule, bool, error) {
	tokens := syntaxTokens(line)
	if len(tokens) == 0 {
		return gwdkast.AuditRule{}, false, nil
	}
	switch tokens[0].Lexeme {
	case "require":
		return parseAuditRequireRule(tokens, lineNumber, rawLine)
	case "deny":
		return parseAuditDenyRule(tokens, lineNumber, rawLine)
	case "allow":
		return parseAuditAllowRule(tokens, lineNumber, rawLine)
	default:
		return gwdkast.AuditRule{}, false, nil
	}
}

func parseAuditRequireRule(tokens []syntax.Token, lineNumber int, rawLine string) (gwdkast.AuditRule, bool, error) {
	if len(tokens) < 2 {
		return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: require rule is missing a subject", lineNumber)
	}
	rule := gwdkast.AuditRule{Span: sourceLineSpan(lineNumber, rawLine)}
	switch tokens[1].Lexeme {
	case "csrf":
		rule.Kind = "require_csrf"
		// Leave Code empty so the engine resolves a kind-appropriate code per
		// matched endpoint (action vs command). An explicit `as <code>` still
		// overrides this in finishAuditRule.
		return finishAuditRule(rule, tokens[2:], lineNumber)
	case "guard":
		if len(tokens) < 3 {
			return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: require guard needs a guard value", lineNumber)
		}
		value, ok := auditValue(tokens[2])
		if !ok {
			return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: require guard value must be an identifier or string", lineNumber)
		}
		rule.Kind = "require_guard"
		rule.Value = value
		rule.Code = "audit_required_guard_missing"
		return finishAuditRule(rule, tokens[3:], lineNumber)
	case "header":
		if len(tokens) < 3 || tokens[2].Kind != syntax.TokenString {
			return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: require header needs a string header name", lineNumber)
		}
		rule.Kind = "require_header"
		rule.Value = decodeStringLiteral(tokens[2].Lexeme)
		rule.Code = "audit_headers_missing"
		return finishAuditRule(rule, tokens[3:], lineNumber)
	case "max_body":
		if len(tokens) < 3 {
			return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: require max_body needs a size", lineNumber)
		}
		value, ok := auditValue(tokens[2])
		if !ok {
			return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: require max_body size must be an identifier or string", lineNumber)
		}
		rule.Kind = "max_body"
		rule.Value = value
		rule.Code = "audit_max_body_exceeds_policy"
		return finishAuditRule(rule, tokens[3:], lineNumber)
	case "no_secrets_in_bundle":
		rule.Kind = "no_secrets_in_bundle"
		rule.Code = "audit_bundle_secret"
		return finishAuditRule(rule, tokens[2:], lineNumber)
	default:
		return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: unsupported require rule %q", lineNumber, tokens[1].Lexeme)
	}
}

func parseAuditDenyRule(tokens []syntax.Token, lineNumber int, rawLine string) (gwdkast.AuditRule, bool, error) {
	if len(tokens) < 2 {
		return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: deny rule is missing a subject", lineNumber)
	}
	rule := gwdkast.AuditRule{Span: sourceLineSpan(lineNumber, rawLine)}
	switch tokens[1].Lexeme {
	case "public":
		rule.Kind = "deny_public"
		rule.Code = "audit_public_not_allowed"
		return finishAuditRule(rule, tokens[2:], lineNumber)
	case "raw_html":
		rule.Kind = "deny_raw_html_sinks"
		rule.Code = "audit_raw_html_sink"
		return finishAuditRule(rule, tokens[2:], lineNumber)
	default:
		return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: unsupported deny rule %q", lineNumber, tokens[1].Lexeme)
	}
}

func parseAuditAllowRule(tokens []syntax.Token, lineNumber int, rawLine string) (gwdkast.AuditRule, bool, error) {
	if len(tokens) < 3 || tokens[1].Lexeme != "raw_html" {
		return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: allow currently supports allow raw_html \"<source-or-owner:field>\"", lineNumber)
	}
	value, ok := auditValue(tokens[2])
	if !ok {
		return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: allow raw_html value must be an identifier or string", lineNumber)
	}
	rule := gwdkast.AuditRule{Kind: "allow_raw_html", Value: value, Span: sourceLineSpan(lineNumber, rawLine)}
	return finishAuditRule(rule, tokens[3:], lineNumber)
}

func finishAuditRule(rule gwdkast.AuditRule, tail []syntax.Token, lineNumber int) (gwdkast.AuditRule, bool, error) {
	if len(tail) == 0 {
		return rule, true, nil
	}
	if len(tail) == 2 && tail[0].Lexeme == "as" {
		code, ok := auditValue(tail[1])
		if !ok {
			return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: rule diagnostic code must be an identifier or string", lineNumber)
		}
		rule.Code = code
		return rule, true, nil
	}
	return gwdkast.AuditRule{}, true, fmt.Errorf("line %d: unsupported trailing rule syntax", lineNumber)
}

func auditValue(token syntax.Token) (string, bool) {
	switch token.Kind {
	case syntax.TokenIdentifier:
		return token.Lexeme, true
	case syntax.TokenString:
		return decodeStringLiteral(token.Lexeme), true
	default:
		return "", false
	}
}

func lowerAuditSyntax(path string, ast gwdkast.AuditFile) gwdkir.AuditSpec {
	spec := gwdkir.AuditSpec{Source: path}
	if ast.Package != nil {
		spec.Package = ast.Package.Name
		spec.Span = ast.Package.Span
	}
	for _, policy := range ast.Policies {
		out := gwdkir.AuditPolicy{Name: policy.Name, Extends: append([]string(nil), policy.Extends...), Span: policy.Span}
		for _, apply := range policy.Applies {
			out.Applies = append(out.Applies, gwdkir.AuditApply{Selector: apply.Selector, Span: apply.Span})
		}
		for _, rule := range policy.Rules {
			out.Rules = append(out.Rules, gwdkir.AuditRule{Kind: rule.Kind, Value: rule.Value, Code: rule.Code, Span: rule.Span})
		}
		spec.Policies = append(spec.Policies, out)
	}
	for _, test := range ast.Tests {
		spec.Tests = append(spec.Tests, gwdkir.AuditTest{Name: test.Name, Body: test.Body, Span: test.Span})
	}
	if spec.Span.Start.Line == 0 && len(spec.Policies) > 0 {
		spec.Span = spec.Policies[0].Span
	}
	return spec
}

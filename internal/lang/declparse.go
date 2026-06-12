package lang

import (
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/gwdkast"
)

// TopLevel holds the top-level declaration nodes a recursive-descent parse
// recovers from .gwdk source. It is the first slice of the ADR 0010 parser
// producing real gwdkast nodes (rather than the tooling outline): the package
// declaration, Go imports and GOWDK uses, and the top-level metadata
// declarations — all of which map one-to-one to the line-oriented parser's
// output and are exercised by an equivalence test against
// internal/parser.ParseSyntax. Metadata is recovered both as the raw
// declaration list (Metadata, mirroring SyntaxFile.Metadata) and routed into the
// validation-free typed fields the line parser assigns unconditionally (Page,
// Component, Cache). Metadata that needs validation to reach a typed field
// (route, revalidate, error, layout, guard, css, asset), block bodies, view
// markup, and endpoints remain on the line-oriented parser until later phases.
type TopLevel struct {
	Package   *gwdkast.Package
	Imports   []gwdkast.Import
	Uses      []gwdkast.Use
	Metadata  []gwdkast.MetadataDecl
	Page      *gwdkast.PageDecl
	Component *gwdkast.ComponentDecl
	Cache     *gwdkast.CacheDecl
}

// ParseTopLevel parses the package, import, and use declarations of .gwdk source
// with a recursive-descent pass over the shared tokenizer. It accepts only the
// exact shapes the line-oriented parser accepts (so an equivalence-anchored
// cutover never turns malformed source into valid nodes) and recovers from any
// other line by skipping to the next line, skipping block bodies by brace
// matching, so one bad declaration never hides the rest.
func ParseTopLevel(src string) TopLevel {
	tokens, _ := Lex(src)
	var result TopLevel

	index := 0
	for index < len(tokens) {
		token := tokens[index]
		if token.Kind == TokenEOF {
			break
		}
		if token.Kind == TokenNewline {
			index++
			continue
		}

		lineEnd, hasBrace := lineExtent(tokens, index)
		if hasBrace {
			index = matchBrace(tokens, index) + 1
			continue
		}

		line := tokens[index:lineEnd]
		first := line[0]
		switch {
		case first.Kind == TokenIdentifier:
			switch first.Lexeme {
			case "package":
				if result.Package == nil {
					if name, ok := parsePackageName(line); ok {
						result.Package = &gwdkast.Package{Name: name, Span: spanOf(first, line[len(line)-1])}
					}
				}
			case "import":
				if imp, ok := parseImportTokens(line); ok {
					result.Imports = append(result.Imports, imp)
				}
			case "use":
				if use, ok := parseUseTokens(line); ok {
					result.Uses = append(result.Uses, use)
				}
			}
		case first.Kind == TokenMetadata:
			result.applyMetadata(first, line, metadataValue(src, first, tokens[lineEnd]))
		}
		index = lineEnd
	}

	return result
}

// metadataValue returns the source text after a line-leading metadata keyword,
// trimmed, mirroring parseMetadataLine in the line parser (which keeps the raw
// remainder of the line — quotes and any trailing comment included — and only
// trims surrounding whitespace). end is the newline or EOF token that closes the
// line, so its byte offset bounds the value.
func metadataValue(src string, keyword, end Token) string {
	start := keyword.Offset + len(keyword.Lexeme)
	stop := end.Offset
	if start < 0 || stop > len(src) || start > stop {
		return ""
	}
	return strings.TrimSpace(src[start:stop])
}

// applyMetadata records one top-level metadata declaration. Every metadata line
// is appended to Metadata as a raw {Name, Value} pair (matching
// SyntaxFile.Metadata), and the three keywords the line parser routes into typed
// fields without validation are mirrored here. The line parser assigns these
// unconditionally, so the last occurrence wins. Keywords whose typed field needs
// validation (route, revalidate, error, layout, guard, css, asset) are left to a
// later phase and survive only in the raw Metadata list for now.
func (top *TopLevel) applyMetadata(keyword Token, line []Token, value string) {
	span := spanOf(keyword, line[len(line)-1])
	top.Metadata = append(top.Metadata, gwdkast.MetadataDecl{Name: keyword.Lexeme, Value: value, Span: span})
	switch keyword.Lexeme {
	case "page":
		top.Page = &gwdkast.PageDecl{ID: value, Span: span}
	case "component":
		top.Component = &gwdkast.ComponentDecl{Name: value, Span: span}
	case "cache":
		top.Cache = &gwdkast.CacheDecl{Policy: unquote(value), Span: span}
	}
}

// parsePackageName accepts exactly `package <strict-ident>` with no trailing
// tokens, matching parsePackageLine in the line parser.
func parsePackageName(line []Token) (string, bool) {
	if len(line) != 2 || line[1].Kind != TokenIdentifier || !isStrictIdent(line[1].Lexeme) {
		return "", false
	}
	return line[1].Lexeme, true
}

// parseImportTokens accepts exactly `import [<strict-ident alias>] <string path>`
// with no trailing tokens. The alias is optional; the path may be any import
// path. Extra identifiers or trailing tokens are rejected so the line is
// recovered past rather than emitted.
func parseImportTokens(line []Token) (gwdkast.Import, bool) {
	index := 1
	alias := ""
	if index < len(line) && line[index].Kind == TokenIdentifier {
		if !isStrictIdent(line[index].Lexeme) {
			return gwdkast.Import{}, false
		}
		alias = line[index].Lexeme
		index++
	}
	if index >= len(line) || line[index].Kind != TokenString {
		return gwdkast.Import{}, false
	}
	path := unquote(line[index].Lexeme)
	index++
	if index != len(line) || path == "" {
		return gwdkast.Import{}, false
	}
	return gwdkast.Import{Alias: alias, Path: path, Span: spanOf(line[0], line[len(line)-1])}, true
}

// parseUseTokens accepts exactly `use <strict-ident alias> "<strict-ident package>"`
// with no trailing tokens, matching parseUseLine (the package must be a strict
// identifier, so paths like "ui/forms" are rejected).
func parseUseTokens(line []Token) (gwdkast.Use, bool) {
	if len(line) != 3 {
		return gwdkast.Use{}, false
	}
	if line[1].Kind != TokenIdentifier || !isStrictIdent(line[1].Lexeme) {
		return gwdkast.Use{}, false
	}
	if line[2].Kind != TokenString {
		return gwdkast.Use{}, false
	}
	pkg := unquote(line[2].Lexeme)
	if !isStrictIdent(pkg) {
		return gwdkast.Use{}, false
	}
	return gwdkast.Use{Alias: line[1].Lexeme, Package: pkg, Span: spanOf(line[0], line[len(line)-1])}, true
}

// isStrictIdent mirrors the line parser's identifier rule: a leading letter or
// underscore followed by letters, underscores, or digits. The shared tokenizer
// also admits '.' and '-' in identifier lexemes, so this re-checks strictness.
func isStrictIdent(value string) bool {
	if value == "" {
		return false
	}
	for index, r := range value {
		if index == 0 {
			if !unicode.IsLetter(r) && r != '_' {
				return false
			}
			continue
		}
		if !unicode.IsLetter(r) && r != '_' && !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

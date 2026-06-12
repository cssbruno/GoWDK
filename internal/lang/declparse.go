package lang

import (
	"unicode"

	"github.com/cssbruno/gowdk/internal/gwdkast"
)

// TopLevel holds the top-level declaration nodes a recursive-descent parse
// recovers from .gwdk source. It is the first slice of the ADR 0010 parser
// producing real gwdkast nodes (rather than the tooling outline): the package
// declaration plus Go imports and GOWDK uses, which map one-to-one to the
// line-oriented parser's output and are exercised by an equivalence test against
// internal/parser.ParseSyntax. Metadata routing, blocks, view markup, and
// endpoints remain on the line-oriented parser until later phases.
type TopLevel struct {
	Package *gwdkast.Package
	Imports []gwdkast.Import
	Uses    []gwdkast.Use
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
		if first.Kind == TokenIdentifier {
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
		}
		index = lineEnd
	}

	return result
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

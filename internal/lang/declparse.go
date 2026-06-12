package lang

import "github.com/cssbruno/gowdk/internal/gwdkast"

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
// with a recursive-descent pass over the shared tokenizer. It recovers from
// unrecognized or malformed lines by skipping to the next line and skips block
// bodies by brace matching, so one bad declaration never hides the rest.
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
					if name := firstIdentifier(line, 1); name != "" {
						result.Package = &gwdkast.Package{Name: name, Span: spanOf(first, line[len(line)-1])}
					}
				}
			case "import":
				if imp, ok := parseImportLine(line); ok {
					result.Imports = append(result.Imports, imp)
				}
			case "use":
				if use, ok := parseUseLine(line); ok {
					result.Uses = append(result.Uses, use)
				}
			}
		}
		index = lineEnd
	}

	return result
}

// parseImportLine reads `import [alias] "path"`. The alias is the identifier
// between import and the path string, or empty, matching the line parser.
func parseImportLine(line []Token) (gwdkast.Import, bool) {
	var alias, path string
	for index := 1; index < len(line); index++ {
		switch line[index].Kind {
		case TokenIdentifier:
			if alias == "" {
				alias = line[index].Lexeme
			}
		case TokenString:
			path = unquote(line[index].Lexeme)
		}
	}
	if path == "" {
		return gwdkast.Import{}, false
	}
	return gwdkast.Import{Alias: alias, Path: path, Span: spanOf(line[0], line[len(line)-1])}, true
}

// parseUseLine reads `use <alias> "<package>"`.
func parseUseLine(line []Token) (gwdkast.Use, bool) {
	var alias, pkg string
	for index := 1; index < len(line); index++ {
		switch line[index].Kind {
		case TokenIdentifier:
			if alias == "" {
				alias = line[index].Lexeme
			}
		case TokenString:
			pkg = unquote(line[index].Lexeme)
		}
	}
	if alias == "" || pkg == "" {
		return gwdkast.Use{}, false
	}
	return gwdkast.Use{Alias: alias, Package: pkg, Span: spanOf(line[0], line[len(line)-1])}, true
}

func firstIdentifier(line []Token, from int) string {
	for index := from; index < len(line); index++ {
		if line[index].Kind == TokenIdentifier {
			return line[index].Lexeme
		}
	}
	return ""
}

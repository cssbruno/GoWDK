package syntax

import (
	"go/ast"
	goparser "go/parser"
	"go/token"
	"strings"
	"unicode"

	"github.com/cssbruno/gowdk/internal/gwdkast"
	"github.com/cssbruno/gowdk/internal/source"
)

// TopLevel holds the top-level declaration nodes a recursive-descent parse
// recovers from .gwdk source. It is the ADR 0010 parser producing real gwdkast
// nodes (rather than the tooling outline), built behind an equivalence test
// against internal/parser.ParseSyntax. It currently recovers:
//
//   - the package declaration, Go imports, and GOWDK uses;
//   - the top-level metadata declarations, both as the raw list (Metadata,
//     mirroring SyntaxFile.Metadata) and routed into the validation-free typed
//     fields the line parser assigns unconditionally (Page, Component, Cache,
//     WASM);
//   - the Go-typed contracts (Stores, PropsType, State), whose pkg.Type and
//     pkg.NewFn() references are parsed by go/parser per ADR 0010's split —
//     custom grammar for .gwdk, the real Go parser for embedded Go — then
//     constrained to the shapes the line parser accepts;
//   - the exact act/api endpoint declarations (Actions, APIs), with the line
//     parser's post-match validation (exported handler name, allowed method,
//     resolvable error page).
//
// Metadata that needs validation to reach a typed field (route, revalidate,
// error, layout, guard, css, asset), block bodies, view markup, and the
// block-bodied fragment endpoint remain on the line-oriented parser until later
// phases.
type TopLevel struct {
	Package   *gwdkast.Package
	Imports   []gwdkast.Import
	Uses      []gwdkast.Use
	Metadata  []gwdkast.MetadataDecl
	Page      *gwdkast.PageDecl
	Component *gwdkast.ComponentDecl
	Cache     *gwdkast.CacheDecl
	Stores    []gwdkast.Store
	PropsType *gwdkast.GoTypeRef
	State     *gwdkast.StateContract
	WASM      *gwdkast.WASMContract
	Actions   []gwdkast.Endpoint
	APIs      []gwdkast.Endpoint
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

		lineEnd, hasBrace := LineExtent(tokens, index)
		if hasBrace {
			index = MatchBrace(tokens, index) + 1
			continue
		}

		line := tokens[index:lineEnd]
		first := line[0]
		switch {
		case first.Kind == TokenIdentifier:
			// The line parser requires eof() after every identifier-led
			// declaration, but the shared tokenizer strips // comments, so a
			// trailing comment leaves a clean token line here. Reject any line
			// whose raw tail still holds a dropped comment so recovery never
			// emits a node ParseSyntax (which sees the comment as a leftover
			// token and bails) would not.
			if hasTrailingContent(src, line, tokens[lineEnd]) {
				break
			}
			switch first.Lexeme {
			case "package":
				if result.Package == nil {
					if name, ok := parsePackageName(line); ok {
						result.Package = &gwdkast.Package{Name: name, Span: SpanOf(first, line[len(line)-1])}
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
			case "store":
				if store, ok := parseStoreTokens(src, line, tokens[lineEnd]); ok {
					result.Stores = append(result.Stores, store)
				}
			case "props":
				if ref, ok := parsePropsTokens(src, line, tokens[lineEnd]); ok {
					result.PropsType = &ref
				}
			case "state":
				if state, ok := parseStateTokens(src, line, tokens[lineEnd]); ok {
					result.State = &state
				}
			case "act":
				if endpoint, ok := parseEndpointTokens("act", line); ok {
					result.Actions = append(result.Actions, endpoint)
				}
			case "api":
				if endpoint, ok := parseEndpointTokens("api", line); ok {
					result.APIs = append(result.APIs, endpoint)
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
	span := SpanOf(keyword, line[len(line)-1])
	top.Metadata = append(top.Metadata, gwdkast.MetadataDecl{Name: keyword.Lexeme, Value: value, Span: span})
	switch keyword.Lexeme {
	case "page":
		top.Page = &gwdkast.PageDecl{ID: value, Span: span}
	case "component":
		top.Component = &gwdkast.ComponentDecl{Name: value, Span: span}
	case "cache":
		top.Cache = &gwdkast.CacheDecl{Policy: Unquote(value), Span: span}
	case "wasm":
		// The SyntaxFile path keeps the raw quoted value (it does not trimQuotes),
		// so match it for equivalence rather than unquoting.
		top.WASM = &gwdkast.WASMContract{Package: value, Span: span}
	}
}

// parseStoreTokens recovers a `store <name> <pkg.Type> = <pkg.NewFn()>` line. The
// name is a strict identifier; the type and initializer are Go expressions
// delegated to go/parser per ADR 0010 (custom .gwdk grammar, reused Go parser for
// embedded Go), then constrained to the single-selector / zero-arg-call shapes
// the line parser's qualifiedIdent accepts so recovery stays equivalent.
func parseStoreTokens(src string, line []Token, end Token) (gwdkast.Store, bool) {
	if len(line) < 2 || line[1].Kind != TokenIdentifier || !isStrictIdent(line[1].Lexeme) {
		return gwdkast.Store{}, false
	}
	assign := assignIndex(line)
	if assign < 2 {
		return gwdkast.Store{}, false
	}
	span := SpanOf(line[0], line[len(line)-1])
	typeRef, ok := goTypeRef(sourceBetween(src, tokenEnd(line[1]), line[assign].Offset), span)
	if !ok {
		return gwdkast.Store{}, false
	}
	initRef, ok := goFuncRef(sourceBetween(src, tokenEnd(line[assign]), end.Offset), span)
	if !ok {
		return gwdkast.Store{}, false
	}
	return gwdkast.Store{Name: line[1].Lexeme, Type: typeRef, Init: initRef, Span: span}, true
}

// parsePropsTokens recovers a `props <pkg.Type>` contract: a type reference with
// no initializer (an `=` makes it a malformed props line the line parser
// rejects, so it is skipped rather than emitted).
func parsePropsTokens(src string, line []Token, end Token) (gwdkast.GoTypeRef, bool) {
	if assignIndex(line) != -1 {
		return gwdkast.GoTypeRef{}, false
	}
	span := SpanOf(line[0], line[len(line)-1])
	return goTypeRef(sourceBetween(src, tokenEnd(line[0]), end.Offset), span)
}

// parseStateTokens recovers a `state <pkg.Type> = <pkg.NewFn()>` contract. Unlike
// props, the initializer is required, so a missing `=` is rejected.
func parseStateTokens(src string, line []Token, end Token) (gwdkast.StateContract, bool) {
	assign := assignIndex(line)
	if assign < 1 {
		return gwdkast.StateContract{}, false
	}
	span := SpanOf(line[0], line[len(line)-1])
	typeRef, ok := goTypeRef(sourceBetween(src, tokenEnd(line[0]), line[assign].Offset), span)
	if !ok {
		return gwdkast.StateContract{}, false
	}
	initRef, ok := goFuncRef(sourceBetween(src, tokenEnd(line[assign]), end.Offset), span)
	if !ok {
		return gwdkast.StateContract{}, false
	}
	return gwdkast.StateContract{Type: typeRef, Init: initRef, Span: span}, true
}

// goTypeRef parses a Go type reference (pkg.Type) and accepts only a single
// selector of two strict identifiers — the line parser's qualifiedIdent rule, so
// generics (pkg.Type[T]) and multi-segment paths (a.b.c) are rejected.
func goTypeRef(expr string, span source.SourceSpan) (gwdkast.GoTypeRef, bool) {
	parsed, err := goparser.ParseExpr(strings.TrimSpace(expr))
	if err != nil {
		return gwdkast.GoTypeRef{}, false
	}
	alias, name, ok := selectorParts(parsed)
	if !ok {
		return gwdkast.GoTypeRef{}, false
	}
	return gwdkast.GoTypeRef{Alias: alias, Name: name, Span: span}, true
}

// goFuncRef parses a Go constructor reference (pkg.NewFn()) — a call of a
// pkg.Fn selector with no arguments, matching the line parser's `()` requirement.
func goFuncRef(expr string, span source.SourceSpan) (gwdkast.GoFuncRef, bool) {
	parsed, err := goparser.ParseExpr(strings.TrimSpace(expr))
	if err != nil {
		return gwdkast.GoFuncRef{}, false
	}
	call, ok := parsed.(*ast.CallExpr)
	if !ok || len(call.Args) != 0 || call.Ellipsis != token.NoPos {
		return gwdkast.GoFuncRef{}, false
	}
	alias, name, ok := selectorParts(call.Fun)
	if !ok {
		return gwdkast.GoFuncRef{}, false
	}
	return gwdkast.GoFuncRef{Alias: alias, Name: name, Span: span}, true
}

// selectorParts splits a pkg.Name selector into its two strict identifiers.
func selectorParts(node ast.Expr) (string, string, bool) {
	selector, ok := node.(*ast.SelectorExpr)
	if !ok {
		return "", "", false
	}
	pkg, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", "", false
	}
	if !isStrictIdent(pkg.Name) || !isStrictIdent(selector.Sel.Name) {
		return "", "", false
	}
	return pkg.Name, selector.Sel.Name, true
}

// assignIndex returns the position of the first framework-level `=` token in the
// line, or -1. The shared tokenizer emits `=>` as an arrow, so this never matches
// inside a paths block's arrow.
func assignIndex(line []Token) int {
	for index, token := range line {
		if token.Kind == TokenAssign {
			return index
		}
	}
	return -1
}

// tokenEnd returns the byte offset just past a token.
func tokenEnd(token Token) int {
	return token.Offset + len(token.Lexeme)
}

// sourceBetween returns the source slice for a byte range, clamped to valid
// bounds (an empty string for an inverted or out-of-range span).
func sourceBetween(src string, start, stop int) string {
	if start < 0 || stop > len(src) || start > stop {
		return ""
	}
	return src[start:stop]
}

// hasTrailingContent reports whether non-whitespace source remains between the
// last token of line and the line terminator end — that is, a // comment the
// shared tokenizer stripped. The line-oriented parser requires eof() after
// every identifier-led declaration, so a line with a trailing comment is
// rejected there; this check keeps recovery from emitting a node ParseSyntax
// would not.
func hasTrailingContent(src string, line []Token, end Token) bool {
	last := line[len(line)-1]
	return strings.TrimSpace(sourceBetween(src, tokenEnd(last), end.Offset)) != ""
}

// decodeString decodes a "..."-quoted lexeme using the same escape rules as the
// line parser's lexLineString: \" and \\ collapse, \n and \t become the control
// characters, any other \x keeps the backslash and the character, and a
// trailing backslash is kept literally. Recovered string values (routes, error
// pages, import paths, use packages) must match ParseSyntax byte for byte, and
// ParseSyntax decodes these escapes, so trimming quotes alone is not enough.
func decodeString(lexeme string) string {
	if len(lexeme) < 2 || lexeme[0] != '"' {
		return Unquote(lexeme)
	}
	var builder strings.Builder
	for index := 1; index < len(lexeme); index++ {
		ch := lexeme[index]
		if ch == '"' {
			break
		}
		if ch == '\\' {
			if index+1 >= len(lexeme) {
				builder.WriteByte(ch)
				continue
			}
			index++
			switch lexeme[index] {
			case '"', '\\':
				builder.WriteByte(lexeme[index])
			case 'n':
				builder.WriteByte('\n')
			case 't':
				builder.WriteByte('\t')
			default:
				builder.WriteByte('\\')
				builder.WriteByte(lexeme[index])
			}
			continue
		}
		builder.WriteByte(ch)
	}
	return builder.String()
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
	path := decodeString(line[index].Lexeme)
	index++
	if index != len(line) || path == "" {
		return gwdkast.Import{}, false
	}
	return gwdkast.Import{Alias: alias, Path: path, Span: SpanOf(line[0], line[len(line)-1])}, true
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
	pkg := decodeString(line[2].Lexeme)
	if !isStrictIdent(pkg) {
		return gwdkast.Use{}, false
	}
	return gwdkast.Use{Alias: line[1].Lexeme, Package: pkg, Span: SpanOf(line[0], line[len(line)-1])}, true
}

// parseEndpointTokens recovers an exact endpoint declaration:
//
//	act <Name> POST "<route>" [error "<page>"]
//	api <Name> <METHOD> "<route>" [error "<page>"]
//
// matching parseActionEndpointLine / parseAPIEndpointLine plus the validation the
// line parser applies after the pattern: the handler name must be an exported Go
// identifier, the method must be an uppercase token (POST for actions; one of the
// REST verbs for APIs), and an optional error page must resolve. Lines that fail
// any check are skipped so recovery never emits an endpoint the line parser would
// reject.
func parseEndpointTokens(kind string, line []Token) (gwdkast.Endpoint, bool) {
	if len(line) != 4 && len(line) != 6 {
		return gwdkast.Endpoint{}, false
	}
	name := line[1]
	if name.Kind != TokenIdentifier || !isExportedIdent(name.Lexeme) {
		return gwdkast.Endpoint{}, false
	}
	method := line[2]
	if method.Kind != TokenIdentifier || !isUpperMethod(method.Lexeme) || !methodAllowed(kind, method.Lexeme) {
		return gwdkast.Endpoint{}, false
	}
	if line[3].Kind != TokenString {
		return gwdkast.Endpoint{}, false
	}
	span := SpanOf(line[0], line[len(line)-1])
	endpoint := gwdkast.Endpoint{
		Kind:   kind,
		Name:   name.Lexeme,
		Method: method.Lexeme,
		Route:  decodeString(line[3].Lexeme),
		Span:   span,
	}
	if len(line) == 6 {
		if line[4].Kind != TokenIdentifier || line[4].Lexeme != "error" || line[5].Kind != TokenString {
			return gwdkast.Endpoint{}, false
		}
		errorPage, err := source.ErrorPagePath(decodeString(line[5].Lexeme))
		if err != nil {
			return gwdkast.Endpoint{}, false
		}
		endpoint.ErrorPage = errorPage
		endpoint.ErrorPageSpan = span
	}
	return endpoint, true
}

// isExportedIdent reports whether value is a strict identifier whose first rune
// is an ASCII capital, matching the line parser's isExportedIdentifier.
func isExportedIdent(value string) bool {
	if !isStrictIdent(value) {
		return false
	}
	first := []rune(value)[0]
	return first >= 'A' && first <= 'Z'
}

// isUpperMethod reports whether value is a non-empty run of ASCII capitals,
// matching the line parser's method() rule.
func isUpperMethod(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// methodAllowed enforces the per-kind method constraint: actions require POST;
// APIs accept the REST verb set.
func methodAllowed(kind, method string) bool {
	switch kind {
	case "act":
		return method == "POST"
	case "api":
		switch method {
		case "GET", "POST", "PUT", "PATCH", "DELETE":
			return true
		}
	}
	return false
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

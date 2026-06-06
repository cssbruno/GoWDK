// Package lsp implements the GOWDK Language Server Protocol entrypoint.
package lsp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/cssbruno/gowdk"
	"github.com/cssbruno/gowdk/internal/lang"
	"github.com/cssbruno/gowdk/internal/manifest"
)

const (
	jsonRPCVersion = "2.0"

	parseError     = -32700
	invalidRequest = -32600
	methodNotFound = -32601
	invalidParams  = -32602
	internalError  = -32603

	textDocumentSyncFull = 1

	diagnosticSeverityError   = 1
	diagnosticSeverityWarning = 2

	completionItemKindText      = 1
	completionItemKindFunction  = 3
	completionItemKindClass     = 7
	completionItemKindProperty  = 10
	completionItemKindReference = 18
	completionItemKindKeyword   = 14
)

var (
	simpleInterpolationCompletionPattern = regexp.MustCompile(`\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}`)
	bindingCompletionPattern             = regexp.MustCompile(`g:bind:(?:value|checked)\s*=\s*\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}`)
	assignmentCompletionPattern          = regexp.MustCompile(`\b([A-Za-z_][A-Za-z0-9_]*)\s*(?:=|\+\+|--)`)
	semanticTokenTypes                   = []string{"decorator", "variable", "string", "operator"}
	semanticTokenTypeIndex               = map[string]int{"decorator": 0, "variable": 1, "string": 2, "operator": 3}
)

// Server handles one LSP session.
type Server struct {
	config    gowdk.Config
	documents map[string]document
	shutdown  bool
	log       io.Writer
}

type document struct {
	URI     string
	Path    string
	Version int
	Text    string
}

// NewServer returns a language server using the provided compiler config.
func NewServer(config gowdk.Config) *Server {
	return &Server{
		config:    config,
		documents: map[string]document{},
		log:       os.Stderr,
	}
}

// Serve runs the JSON-RPC message loop until the client sends exit or input closes.
func (server *Server) Serve(in io.Reader, out io.Writer) error {
	reader := bufio.NewReader(in)
	for {
		body, err := readMessage(reader)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		messages, exit := server.handle(body)
		for _, message := range messages {
			if err := writeMessage(out, message); err != nil {
				return err
			}
		}
		if exit {
			return nil
		}
	}
}

func (server *Server) handle(body []byte) ([][]byte, bool) {
	var request rpcRequest
	if err := json.Unmarshal(body, &request); err != nil {
		server.logf("invalid lsp message: %v", err)
		return singleMessage(errorResponse(nil, parseError, fmt.Sprintf("parse error: %v", err))), false
	}

	if request.Method == "exit" {
		return nil, true
	}
	if request.Method == "" {
		if request.ID != nil {
			return singleMessage(errorResponse(request.ID, invalidRequest, "missing method")), false
		}
		return nil, false
	}
	if request.ID == nil {
		return server.handleNotification(request), false
	}
	return server.handleRequest(request), false
}

func (server *Server) handleRequest(request rpcRequest) [][]byte {
	switch request.Method {
	case "initialize":
		return singleMessage(response(request.ID, initializeResult{
			Capabilities: serverCapabilities{
				TextDocumentSync: textDocumentSyncOptions{
					OpenClose: true,
					Change:    textDocumentSyncFull,
					Save:      saveOptions{IncludeText: true},
				},
				HoverProvider:              true,
				DefinitionProvider:         true,
				DocumentFormattingProvider: true,
				CompletionProvider: completionOptions{
					TriggerCharacters: []string{"@", ":", "<", " "},
				},
				SemanticTokensProvider: semanticTokensOptions{
					Legend: semanticTokensLegend{
						TokenTypes:     semanticTokenTypes,
						TokenModifiers: []string{},
					},
					Full: true,
				},
			},
			ServerInfo: serverInfo{
				Name:    "gowdk",
				Version: "0.1.5",
			},
		}))
	case "shutdown":
		server.shutdown = true
		return singleMessage(response(request.ID, nil))
	case "textDocument/formatting":
		var params documentFormattingParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		doc, ok := server.documents[params.TextDocument.URI]
		if !ok {
			return singleMessage(response(request.ID, []textEdit{}))
		}
		formatted := string(lang.Format([]byte(doc.Text)))
		if formatted == doc.Text {
			return singleMessage(response(request.ID, []textEdit{}))
		}
		return singleMessage(response(request.ID, []textEdit{{
			Range:   fullRange(doc.Text),
			NewText: formatted,
		}}))
	case "textDocument/completion":
		var params completionParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, completionList{
			IsIncomplete: false,
			Items:        server.completionItems(params),
		}))
	case "textDocument/hover":
		var params hoverParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, server.hover(params)))
	case "textDocument/definition":
		var params definitionParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, server.definition(params)))
	case "textDocument/semanticTokens/full":
		var params semanticTokensParams
		if err := decodeParams(request.Params, &params); err != nil {
			return singleMessage(errorResponse(request.ID, invalidParams, err.Error()))
		}
		return singleMessage(response(request.ID, server.semanticTokens(params)))
	default:
		return singleMessage(errorResponse(request.ID, methodNotFound, fmt.Sprintf("method not found: %s", request.Method)))
	}
}

func (server *Server) handleNotification(request rpcRequest) [][]byte {
	switch request.Method {
	case "initialized":
		return nil
	case "textDocument/didOpen":
		var params didOpenTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didOpen params: %v", err)
			return nil
		}
		doc := document{
			URI:     params.TextDocument.URI,
			Path:    pathFromURI(params.TextDocument.URI),
			Version: params.TextDocument.Version,
			Text:    params.TextDocument.Text,
		}
		server.documents[doc.URI] = doc
		return singleMessage(server.publishDiagnostics(doc))
	case "textDocument/didChange":
		var params didChangeTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didChange params: %v", err)
			return nil
		}
		doc := server.documents[params.TextDocument.URI]
		doc.URI = params.TextDocument.URI
		doc.Path = pathFromURI(params.TextDocument.URI)
		doc.Version = params.TextDocument.Version
		if len(params.ContentChanges) > 0 {
			doc.Text = params.ContentChanges[len(params.ContentChanges)-1].Text
		}
		server.documents[doc.URI] = doc
		return singleMessage(server.publishDiagnostics(doc))
	case "textDocument/didSave":
		var params didSaveTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didSave params: %v", err)
			return nil
		}
		doc, ok := server.documents[params.TextDocument.URI]
		if !ok {
			return nil
		}
		if params.Text != nil {
			doc.Text = *params.Text
			server.documents[doc.URI] = doc
		}
		return singleMessage(server.publishDiagnostics(doc))
	case "textDocument/didClose":
		var params didCloseTextDocumentParams
		if err := decodeParams(request.Params, &params); err != nil {
			server.logf("didClose params: %v", err)
			return nil
		}
		delete(server.documents, params.TextDocument.URI)
		return singleMessage(publishDiagnostics(params.TextDocument.URI, nil))
	default:
		return nil
	}
}

func (server *Server) publishDiagnostics(doc document) []byte {
	_, diagnostics := lang.CheckSource(server.config, doc.Path, []byte(doc.Text))
	items := make([]diagnostic, 0, len(diagnostics))
	for _, item := range diagnostics {
		items = append(items, diagnosticFromLang(item, doc.Text))
	}
	return publishDiagnostics(doc.URI, items)
}

func (server *Server) completionItems(params completionParams) []completionItem {
	completions := lang.Completions()
	items := make([]completionItem, 0, len(completions))
	for _, completion := range completions {
		items = append(items, completionItem{
			Label:  completion.Label,
			Kind:   completionItemKindKeyword,
			Detail: completion.Detail,
		})
	}
	return appendProjectCompletionItems(items, server.projectCompletions(params.TextDocument.URI))
}

func (server *Server) hover(params hoverParams) *hoverResult {
	doc, ok := server.documents[params.TextDocument.URI]
	if !ok {
		return nil
	}
	token := tokenAtPosition(doc.Text, params.Position)
	if token == "" {
		return nil
	}
	for _, item := range server.hoverItems(params.TextDocument.URI) {
		if item.Label == token {
			return &hoverResult{
				Contents: markupContent{
					Kind:  "markdown",
					Value: "**" + item.Label + "**\n\n" + item.Detail,
				},
			}
		}
	}
	return nil
}

func (server *Server) definition(params definitionParams) *location {
	doc, ok := server.documents[params.TextDocument.URI]
	if !ok {
		return nil
	}
	name, ok := componentCallAtPosition(doc.Text, params.Position)
	if !ok {
		return nil
	}
	component, ok := server.resolveComponentDefinition(doc, name)
	if !ok {
		return nil
	}
	return &location{
		URI:   component.URI,
		Range: lspRangeFromSourceSpan(component.Span, component.Text),
	}
}

func (server *Server) semanticTokens(params semanticTokensParams) semanticTokensResult {
	doc, ok := server.documents[params.TextDocument.URI]
	if !ok {
		return semanticTokensResult{Data: []int{}}
	}
	tokens, _ := lang.Lex(doc.Text)
	data := make([]int, 0, len(tokens)*5)
	previousLine := 0
	previousCharacter := 0
	seen := false
	for _, token := range tokens {
		tokenType, ok := semanticTokenType(token.Kind)
		if !ok || token.Lexeme == "" {
			continue
		}
		start := positionFromLangPosition(token.Pos, doc.Text)
		length := utf16Length(token.Lexeme)
		if length == 0 {
			continue
		}

		deltaLine := start.Line
		deltaStart := start.Character
		if seen {
			deltaLine = start.Line - previousLine
			if deltaLine == 0 {
				deltaStart = start.Character - previousCharacter
			}
		}
		data = append(data, deltaLine, deltaStart, length, semanticTokenTypeIndex[tokenType], 0)
		previousLine = start.Line
		previousCharacter = start.Character
		seen = true
	}
	return semanticTokensResult{Data: data}
}

type componentDefinition struct {
	URI     string
	Text    string
	Package string
	Name    string
	Span    manifest.SourceSpan
}

func (server *Server) resolveComponentDefinition(doc document, name string) (componentDefinition, bool) {
	ownerPackage, ownerUses := server.ownerPackageAndUses(doc)
	definitions := server.componentDefinitions()
	if alias, componentName, ok := strings.Cut(name, "."); ok {
		packageName, ok := ownerUses[alias]
		if !ok {
			return componentDefinition{}, false
		}
		definition, ok := definitions[componentDefinitionKey(packageName, componentName)]
		return definition, ok
	}
	if ownerPackage != "" {
		if definition, ok := definitions[componentDefinitionKey(ownerPackage, name)]; ok {
			return definition, true
		}
	}
	definition, ok := definitions[componentDefinitionKey("", name)]
	return definition, ok
}

func (server *Server) ownerPackageAndUses(doc document) (string, map[string]string) {
	switch lang.ClassifySource(doc.Path, []byte(doc.Text)) {
	case lang.FileKindPage:
		page, diagnostics := lang.ParseSource(doc.Path, []byte(doc.Text))
		if diagnostics.HasErrors() {
			return "", nil
		}
		return page.Package, usePackagesByAlias(page.Uses)
	case lang.FileKindComponent:
		component, diagnostics := lang.ParseComponentSource(doc.Path, []byte(doc.Text))
		if diagnostics.HasErrors() {
			return "", nil
		}
		return component.Package, usePackagesByAlias(component.Uses)
	default:
		return "", nil
	}
}

func (server *Server) componentDefinitions() map[string]componentDefinition {
	definitions := map[string]componentDefinition{}
	for _, doc := range server.documents {
		if lang.ClassifySource(doc.Path, []byte(doc.Text)) != lang.FileKindComponent {
			continue
		}
		component, diagnostics := lang.ParseComponentSource(doc.Path, []byte(doc.Text))
		if diagnostics.HasErrors() || component.Name == "" {
			continue
		}
		definition := componentDefinition{
			URI:     doc.URI,
			Text:    doc.Text,
			Package: component.Package,
			Name:    component.Name,
			Span:    component.Span,
		}
		definitions[componentDefinitionKey(component.Package, component.Name)] = definition
		if component.Package == "" {
			definitions[componentDefinitionKey("", component.Name)] = definition
		}
	}
	return definitions
}

func usePackagesByAlias(uses []manifest.Use) map[string]string {
	packages := map[string]string{}
	for _, use := range uses {
		if _, exists := packages[use.Alias]; !exists {
			packages[use.Alias] = use.Package
		}
	}
	return packages
}

func componentDefinitionKey(packageName, componentName string) string {
	return packageName + "\x00" + componentName
}

func semanticTokenType(kind lang.TokenKind) (string, bool) {
	switch kind {
	case lang.TokenAnnotation:
		return "decorator", true
	case lang.TokenIdentifier, lang.TokenText:
		return "variable", true
	case lang.TokenString:
		return "string", true
	case lang.TokenLBrace, lang.TokenRBrace, lang.TokenComma, lang.TokenColon, lang.TokenQuestion, lang.TokenArrow:
		return "operator", true
	default:
		return "", false
	}
}

func (server *Server) hoverItems(currentURI string) []completionItem {
	items := server.completionItems(completionParams{
		TextDocument: textDocumentIdentifier{URI: currentURI},
	})
	return appendProjectCompletionItems(items, server.projectHoverItems(currentURI))
}

func appendProjectCompletionItems(items []completionItem, project []completionItem) []completionItem {
	seen := map[string]bool{}
	for _, item := range items {
		seen[item.Label+"\x00"+item.Detail] = true
	}
	for _, item := range project {
		key := item.Label + "\x00" + item.Detail
		if seen[key] {
			continue
		}
		seen[key] = true
		items = append(items, item)
	}
	return items
}

func (server *Server) projectCompletions(currentURI string) []completionItem {
	var items []completionItem
	for _, doc := range server.documents {
		switch lang.ClassifySource(doc.Path, []byte(doc.Text)) {
		case lang.FileKindPage:
			page, diagnostics := lang.ParseSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() {
				continue
			}
			if page.ID != "" {
				items = append(items, completionItem{Label: page.ID, Kind: completionItemKindReference, Detail: "GOWDK page id"})
			}
			if page.Route != "" {
				items = append(items, completionItem{Label: page.Route, Kind: completionItemKindText, Detail: "GOWDK route"})
			}
			for _, guard := range page.Guard {
				items = append(items, completionItem{Label: guard, Kind: completionItemKindFunction, Detail: "GOWDK guard"})
			}
			for _, store := range page.Stores {
				items = append(items, completionItem{Label: store.Name, Kind: completionItemKindProperty, Detail: "GOWDK store"})
			}
		case lang.FileKindComponent:
			component, diagnostics := lang.ParseComponentSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() {
				continue
			}
			if component.Name != "" {
				items = append(items, completionItem{Label: component.Name, Kind: completionItemKindClass, Detail: "GOWDK component"})
			}
			if doc.URI == currentURI {
				for _, prop := range component.Props {
					items = append(items, completionItem{Label: prop.Name, Kind: completionItemKindProperty, Detail: "component prop"})
				}
				for _, field := range inferredComponentFields(component.Blocks.ViewBody, component.Blocks.ClientBody) {
					items = append(items, completionItem{Label: field, Kind: completionItemKindProperty, Detail: "component state/value"})
				}
			}
		case lang.FileKindLayout:
			layout, diagnostics := lang.ParseLayoutSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() || layout.ID == "" {
				continue
			}
			items = append(items, completionItem{Label: layout.ID, Kind: completionItemKindReference, Detail: "GOWDK layout"})
		}
	}
	return items
}

func (server *Server) projectHoverItems(currentURI string) []completionItem {
	var items []completionItem
	for _, doc := range server.documents {
		switch lang.ClassifySource(doc.Path, []byte(doc.Text)) {
		case lang.FileKindPage:
			page, diagnostics := lang.ParseSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() {
				continue
			}
			for _, action := range page.Blocks.Actions {
				items = append(items, completionItem{Label: action.Name, Kind: completionItemKindFunction, Detail: "GOWDK action handler"})
			}
			for _, api := range page.Blocks.APIs {
				items = append(items, completionItem{Label: api.Name, Kind: completionItemKindFunction, Detail: "GOWDK API handler"})
			}
			for _, fragment := range page.Blocks.Fragments {
				items = append(items, completionItem{Label: fragment.Name, Kind: completionItemKindFunction, Detail: "GOWDK fragment handler"})
			}
		case lang.FileKindComponent:
			component, diagnostics := lang.ParseComponentSource(doc.Path, []byte(doc.Text))
			if diagnostics.HasErrors() || doc.URI != currentURI {
				continue
			}
			for _, emit := range component.Emits {
				items = append(items, completionItem{Label: emit.Name, Kind: completionItemKindFunction, Detail: "component event"})
			}
		}
	}
	return items
}

func tokenAtPosition(source string, pos position) string {
	lines := strings.Split(source, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return ""
	}
	line := lines[pos.Line]
	index := byteIndexFromUTF16Column(line, pos.Character)
	if index > len(line) {
		index = len(line)
	}
	start := index
	for start > 0 && hoverTokenByte(line[start-1]) {
		start--
	}
	end := index
	for end < len(line) && hoverTokenByte(line[end]) {
		end++
	}
	return strings.TrimSpace(line[start:end])
}

func componentCallAtPosition(source string, pos position) (string, bool) {
	lines := strings.Split(source, "\n")
	if pos.Line < 0 || pos.Line >= len(lines) {
		return "", false
	}
	line := lines[pos.Line]
	index := byteIndexFromUTF16Column(line, pos.Character)
	if index > len(line) {
		index = len(line)
	}
	start := index
	for start > 0 && componentCallNameByte(line[start-1]) {
		start--
	}
	end := index
	for end < len(line) && componentCallNameByte(line[end]) {
		end++
	}
	if start == end || start == 0 {
		return "", false
	}
	name := line[start:end]
	if !isGOWDKComponentCallName(name) {
		return "", false
	}
	before := start - 1
	if line[before] == '<' {
		return name, true
	}
	if line[before] == '/' && before > 0 && line[before-1] == '<' {
		return name, true
	}
	return "", false
}

func byteIndexFromUTF16Column(line string, column int) int {
	if column <= 0 {
		return 0
	}
	units := 0
	for index, r := range line {
		next := units + len(utf16.Encode([]rune{r}))
		if next > column {
			return index
		}
		units = next
	}
	return len(line)
}

func componentCallNameByte(value byte) bool {
	switch {
	case value >= 'A' && value <= 'Z':
		return true
	case value >= 'a' && value <= 'z':
		return true
	case value >= '0' && value <= '9':
		return true
	case strings.ContainsRune("_-:.", rune(value)):
		return true
	default:
		return false
	}
}

func isGOWDKComponentCallName(value string) bool {
	if alias, name, ok := strings.Cut(value, "."); ok {
		return isGOWDKIdentifier(alias) && isExportedGOWDKName(name)
	}
	return isExportedGOWDKName(value)
}

func isGOWDKIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		char := value[index]
		switch {
		case index == 0 && (char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z'):
		case index > 0 && (char == '_' || char >= 'A' && char <= 'Z' || char >= 'a' && char <= 'z' || char >= '0' && char <= '9'):
		default:
			return false
		}
	}
	return true
}

func isExportedGOWDKName(value string) bool {
	return value != "" && value[0] >= 'A' && value[0] <= 'Z'
}

func hoverTokenByte(value byte) bool {
	switch {
	case value >= 'A' && value <= 'Z':
		return true
	case value >= 'a' && value <= 'z':
		return true
	case value >= '0' && value <= '9':
		return true
	case strings.ContainsRune("@:_-./", rune(value)):
		return true
	default:
		return false
	}
}

func inferredComponentFields(viewBody, clientBody string) []string {
	fields := map[string]bool{}
	for _, match := range simpleInterpolationCompletionPattern.FindAllStringSubmatch(viewBody, -1) {
		fields[match[1]] = true
	}
	for _, match := range bindingCompletionPattern.FindAllStringSubmatch(viewBody, -1) {
		fields[match[1]] = true
	}
	for _, match := range assignmentCompletionPattern.FindAllStringSubmatch(clientBody, -1) {
		fields[match[1]] = true
	}
	out := make([]string, 0, len(fields))
	for field := range fields {
		out = append(out, field)
	}
	return out
}

func diagnosticFromLang(item lang.Diagnostic, source string) diagnostic {
	severity := diagnosticSeverityError
	if item.Severity == "warning" {
		severity = diagnosticSeverityWarning
	}
	return diagnostic{
		Range:    rangeFromLangDiagnostic(item, source),
		Severity: severity,
		Code:     item.Code,
		Source:   "gowdk",
		Message:  item.Message,
	}
}

func rangeFromLangDiagnostic(item lang.Diagnostic, source string) lspRange {
	if item.Range != nil {
		return rangeFromLangRange(*item.Range, source)
	}
	return rangeFromPosition(item.Pos, source)
}

func rangeFromLangRange(item lang.Range, source string) lspRange {
	start := positionFromLangPosition(item.Start, source)
	end := positionFromLangPosition(item.End, source)
	if end.Line < start.Line || (end.Line == start.Line && end.Character <= start.Character) {
		end = position{Line: start.Line, Character: start.Character + 1}
	}
	return lspRange{Start: start, End: end}
}

func lspRangeFromSourceSpan(span manifest.SourceSpan, source string) lspRange {
	return rangeFromLangRange(lang.Range{
		Start: lang.Position{Line: span.Start.Line, Column: span.Start.Column},
		End:   lang.Position{Line: span.End.Line, Column: span.End.Column},
	}, source)
}

func rangeFromPosition(pos lang.Position, source string) lspRange {
	if pos.Line <= 0 {
		return lspRange{
			Start: position{Line: 0, Character: 0},
			End:   position{Line: 0, Character: 1},
		}
	}

	lines := strings.Split(source, "\n")
	lineIndex := clamp(pos.Line-1, 0, len(lines)-1)
	character := 0
	if pos.Column > 1 && len(lines) > 0 {
		character = utf16Column(lines[lineIndex], pos.Column-1)
	}
	lineLength := utf16Length(lines[lineIndex])
	if character > lineLength {
		character = lineLength
	}
	end := character + 1
	if end > lineLength {
		end = character
	}
	return lspRange{
		Start: position{Line: lineIndex, Character: character},
		End:   position{Line: lineIndex, Character: end},
	}
}

func positionFromLangPosition(pos lang.Position, source string) position {
	if pos.Line <= 0 {
		return position{Line: 0, Character: 0}
	}
	lines := strings.Split(source, "\n")
	lineIndex := clamp(pos.Line-1, 0, len(lines)-1)
	character := 0
	if pos.Column > 1 && len(lines) > 0 {
		character = utf16Column(lines[lineIndex], pos.Column-1)
	}
	lineLength := utf16Length(lines[lineIndex])
	if character > lineLength {
		character = lineLength
	}
	return position{Line: lineIndex, Character: character}
}

func fullRange(text string) lspRange {
	lines := strings.Split(text, "\n")
	lastLine := len(lines) - 1
	return lspRange{
		Start: position{Line: 0, Character: 0},
		End: position{
			Line:      lastLine,
			Character: utf16Length(lines[lastLine]),
		},
	}
}

func utf16Column(line string, oneBasedColumn int) int {
	if oneBasedColumn <= 0 {
		return 0
	}
	runes := []rune(line)
	if oneBasedColumn > len(runes) {
		oneBasedColumn = len(runes)
	}
	return len(utf16.Encode(runes[:oneBasedColumn]))
}

func utf16Length(text string) int {
	return len(utf16.Encode([]rune(text)))
}

func clamp(value, minValue, maxValue int) int {
	if maxValue < minValue {
		return minValue
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func publishDiagnostics(uri string, diagnostics []diagnostic) []byte {
	if diagnostics == nil {
		diagnostics = []diagnostic{}
	}
	return notification("textDocument/publishDiagnostics", publishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diagnostics,
	})
}

func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, target)
}

func singleMessage(message []byte) [][]byte {
	if len(message) == 0 {
		return nil
	}
	return [][]byte{message}
}

func response(id *json.RawMessage, result any) []byte {
	payload, err := json.Marshal(rpcResultResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	})
	if err != nil {
		return errorResponse(id, internalError, err.Error())
	}
	return payload
}

func errorResponse(id *json.RawMessage, code int, message string) []byte {
	payload, err := json.Marshal(rpcErrorResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &rpcError{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		return nil
	}
	return payload
}

func notification(method string, params any) []byte {
	payload, err := json.Marshal(rpcNotification{
		JSONRPC: jsonRPCVersion,
		Method:  method,
		Params:  params,
	})
	if err != nil {
		return nil
	}
	return payload
}

func pathFromURI(rawURI string) string {
	parsed, err := url.Parse(rawURI)
	if err != nil || parsed.Scheme != "file" {
		return rawURI
	}
	if runtime.GOOS == "windows" {
		return strings.TrimPrefix(parsed.Path, "/")
	}
	return parsed.Path
}

func (server *Server) logf(format string, args ...any) {
	if server.log == nil {
		return
	}
	fmt.Fprintf(server.log, format+"\n", args...)
}

func readMessage(reader *bufio.Reader) ([]byte, error) {
	contentLength := -1
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) && line == "" {
				return nil, io.EOF
			}
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		name, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(name), "Content-Length") {
			parsed, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length %q", value)
			}
			contentLength = parsed
		}
	}
	if contentLength < 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	return body, err
}

func writeMessage(writer io.Writer, payload []byte) error {
	if len(payload) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(writer, "Content-Length: %d\r\n\r\n", len(payload)); err != nil {
		return err
	}
	_, err := writer.Write(payload)
	return err
}

type rpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type rpcResultResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  any              `json:"result"`
}

type rpcErrorResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Error   *rpcError        `json:"error"`
}

type rpcNotification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initializeResult struct {
	Capabilities serverCapabilities `json:"capabilities"`
	ServerInfo   serverInfo         `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type serverCapabilities struct {
	TextDocumentSync           textDocumentSyncOptions `json:"textDocumentSync"`
	HoverProvider              bool                    `json:"hoverProvider"`
	DefinitionProvider         bool                    `json:"definitionProvider"`
	DocumentFormattingProvider bool                    `json:"documentFormattingProvider"`
	CompletionProvider         completionOptions       `json:"completionProvider"`
	SemanticTokensProvider     semanticTokensOptions   `json:"semanticTokensProvider"`
}

type textDocumentSyncOptions struct {
	OpenClose bool        `json:"openClose"`
	Change    int         `json:"change"`
	Save      saveOptions `json:"save"`
}

type saveOptions struct {
	IncludeText bool `json:"includeText"`
}

type completionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters"`
}

type semanticTokensOptions struct {
	Legend semanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}

type semanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

type textDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type textDocumentIdentifier struct {
	URI string `json:"uri"`
}

type definitionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type location struct {
	URI   string   `json:"uri"`
	Range lspRange `json:"range"`
}

type semanticTokensParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type semanticTokensResult struct {
	Data []int `json:"data"`
}

type versionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

type didOpenTextDocumentParams struct {
	TextDocument textDocumentItem `json:"textDocument"`
}

type didChangeTextDocumentParams struct {
	TextDocument   versionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []textDocumentContentChangeEvent `json:"contentChanges"`
}

type textDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

type didSaveTextDocumentParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Text         *string                `json:"text,omitempty"`
}

type didCloseTextDocumentParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type documentFormattingParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type completionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type hoverParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
}

type publishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []diagnostic `json:"diagnostics"`
}

type diagnostic struct {
	Range    lspRange `json:"range"`
	Severity int      `json:"severity,omitempty"`
	Code     string   `json:"code,omitempty"`
	Source   string   `json:"source,omitempty"`
	Message  string   `json:"message"`
}

type textEdit struct {
	Range   lspRange `json:"range"`
	NewText string   `json:"newText"`
}

type lspRange struct {
	Start position `json:"start"`
	End   position `json:"end"`
}

type position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type completionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []completionItem `json:"items"`
}

type completionItem struct {
	Label  string `json:"label"`
	Kind   int    `json:"kind,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type hoverResult struct {
	Contents markupContent `json:"contents"`
}

type markupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

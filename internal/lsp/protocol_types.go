package lsp

import "encoding/json"

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
	ReferencesProvider         bool                    `json:"referencesProvider"`
	CodeActionProvider         bool                    `json:"codeActionProvider"`
	DocumentFormattingProvider bool                    `json:"documentFormattingProvider"`
	DocumentSymbolProvider     bool                    `json:"documentSymbolProvider"`
	CompletionProvider         completionOptions       `json:"completionProvider"`
	SemanticTokensProvider     semanticTokensOptions   `json:"semanticTokensProvider"`
}

type documentSymbolParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
}

type documentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          lspRange         `json:"range"`
	SelectionRange lspRange         `json:"selectionRange"`
	Children       []documentSymbol `json:"children,omitempty"`
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

type referenceParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Position     position               `json:"position"`
	Context      referenceContext       `json:"context"`
}

type referenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type codeActionParams struct {
	TextDocument textDocumentIdentifier `json:"textDocument"`
	Range        lspRange               `json:"range"`
	Context      codeActionContext      `json:"context"`
}

type codeActionContext struct {
	Diagnostics []diagnostic `json:"diagnostics"`
}

type codeAction struct {
	Title       string        `json:"title"`
	Kind        string        `json:"kind,omitempty"`
	Diagnostics []diagnostic  `json:"diagnostics,omitempty"`
	Edit        workspaceEdit `json:"edit"`
}

type workspaceEdit struct {
	Changes map[string][]textEdit `json:"changes"`
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
	Range              lspRange                       `json:"range"`
	Severity           int                            `json:"severity,omitempty"`
	Code               string                         `json:"code,omitempty"`
	Source             string                         `json:"source,omitempty"`
	Message            string                         `json:"message"`
	RelatedInformation []diagnosticRelatedInformation `json:"relatedInformation,omitempty"`
}

type diagnosticRelatedInformation struct {
	Location location `json:"location"`
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

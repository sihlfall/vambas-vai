// Package lsproto contains the LSP subset shared by the transport and language
// services, following typescript-go's protocol boundary. Positions are UTF-16.
package lsproto

type DocumentURI string

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}
type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}
type TextDocumentIdentifier struct {
	URI DocumentURI `json:"uri"`
}
type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}
type TextDocumentContentChangeEvent struct {
	Range *Range `json:"range,omitempty"`
	Text  string `json:"text"`
}
type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}
type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    Range         `json:"range"`
}
type DiagnosticRelatedInformation struct {
	Location Location `json:"location"`
	Message  string   `json:"message"`
}
type Diagnostic struct {
	Range              Range                          `json:"range"`
	Severity           int                            `json:"severity"`
	Code               string                         `json:"code"`
	Source             string                         `json:"source"`
	Message            string                         `json:"message"`
	RelatedInformation []DiagnosticRelatedInformation `json:"relatedInformation,omitempty"`
}
type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Version     *int         `json:"version,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}
type DocumentSymbol struct {
	Name           string            `json:"name"`
	Detail         string            `json:"detail,omitempty"`
	Kind           int               `json:"kind"`
	Range          Range             `json:"range"`
	SelectionRange Range             `json:"selectionRange"`
	Children       []*DocumentSymbol `json:"children,omitempty"`
}

type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

// SemanticTokens uses LSP's relative, five-integer encoding with UTF-16 lengths.
// This server supplies complete results, without result IDs or delta caching.
type SemanticTokens struct {
	Data []uint32 `json:"data"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

const CompletionItemKindField = 5

type CompletionItem struct {
	Label      string   `json:"label"`
	Kind       int      `json:"kind"`
	Detail     string   `json:"detail,omitempty"`
	SortText   string   `json:"sortText,omitempty"`
	FilterText string   `json:"filterText,omitempty"`
	TextEdit   TextEdit `json:"textEdit"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

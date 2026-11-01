package lsp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
)

func TestSemanticTokensProtocolNegotiationAndEdits(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "Main.bas")
	text := "Public Const Limit As Long = 2\nSub Run(ByVal value As Long)\n Dim result As Long\n result = value + Limit\nEnd Sub\n"
	if err := os.WriteFile(name, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
	uri := lsconv.FileNameToURI(name)
	c := newTestClient(t)
	c.send(1, "initialize", map[string]any{
		"rootUri": lsconv.FileNameToURI(root),
		"capabilities": map[string]any{"textDocument": map[string]any{"semanticTokens": map[string]any{
			"requests":       map[string]any{"full": true, "range": true},
			"formats":        []string{"relative"},
			"tokenTypes":     []string{"function", "variable", "parameter", "unsupported"},
			"tokenModifiers": []string{"readonly", "declaration", "unsupported"},
		}}},
	})
	var initialized struct {
		Capabilities struct {
			Provider struct {
				Legend lsproto.SemanticTokensLegend `json:"legend"`
				Full   bool                         `json:"full"`
				Range  bool                         `json:"range"`
			} `json:"semanticTokensProvider"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(c.response(1)["result"], &initialized); err != nil {
		t.Fatal(err)
	}
	provider := initialized.Capabilities.Provider
	if !provider.Full || !provider.Range || strings.Join(provider.Legend.TokenTypes, ",") != "parameter,variable,function" || strings.Join(provider.Legend.TokenModifiers, ",") != "declaration,readonly" {
		t.Fatal("legend did not negotiate stable indexes", provider)
	}
	c.send(nil, "initialized", map[string]any{})
	params := map[string]any{"textDocument": map[string]any{"uri": uri}}
	c.send(2, "textDocument/semanticTokens/full", params)
	var before lsproto.SemanticTokens
	if err := json.Unmarshal(c.response(2)["result"], &before); err != nil || len(before.Data) == 0 {
		t.Fatal(before, err)
	}
	// A range result remains document-relative, with unfiltered token indexes
	// from the same negotiated legend (parameter = 0, variable = 1).
	c.send(3, "textDocument/semanticTokens/range", map[string]any{
		"textDocument": params["textDocument"],
		"range":        lsproto.Range{Start: lsproto.Position{Line: 3}, End: lsproto.Position{Line: 4}},
	})
	var part lsproto.SemanticTokens
	if err := json.Unmarshal(c.response(3)["result"], &part); err != nil || !bytes.Equal(mustJSON(part.Data), []byte("[3,1,6,1,0,0,9,5,0,0,0,8,5,1,2]")) {
		t.Fatal(part, err)
	}
	edited := strings.ReplaceAll(text, "result", "changed")
	c.send(nil, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 1, "text": text}})
	c.send(nil, "textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": 2},
		"contentChanges": []lsproto.TextDocumentContentChangeEvent{{Text: edited}},
	})
	c.send(4, "textDocument/semanticTokens/full", params)
	var after lsproto.SemanticTokens
	if err := json.Unmarshal(c.response(4)["result"], &after); err != nil || bytes.Equal(mustJSON(before), mustJSON(after)) {
		t.Fatal("semantic tokens ignored unsaved edits", after, err)
	}
	c.send(5, "textDocument/semanticTokens/range", map[string]any{
		"textDocument": params["textDocument"],
		"range":        lsproto.Range{Start: lsproto.Position{Line: 5}, End: lsproto.Position{Line: 2}},
	})
	if !bytes.Contains(c.response(5)["error"], []byte("-32602")) {
		t.Fatal("invalid range accepted")
	}
	c.send(6, "shutdown", nil)
	c.response(6)
	c.send(nil, "exit", nil)
}

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func TestSemanticTokensEmptyResponseAndNoCapability(t *testing.T) {
	c := newTestClient(t)
	c.initialize(t.TempDir(), true)
	if len(c.server.semanticLegend.TokenTypes) != 0 {
		t.Fatal("semantic token capability was invented")
	}
	// A client with no supported types gets an empty array, never JSON null.
	uri := lsconv.FileNameToURI(filepath.Join(t.TempDir(), "Main.bas"))
	c.send(nil, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 1, "text": "Sub Run()\nEnd Sub\n"}})
	c.send(2, "textDocument/semanticTokens/full", map[string]any{"textDocument": map[string]any{"uri": uri}})
	if got := c.response(2); string(got["result"]) != `{"data":[]}` {
		t.Fatal(got)
	}
}

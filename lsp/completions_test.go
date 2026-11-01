package lsp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
)

func TestUDTCompletionProtocolAndUnsavedEdits(t *testing.T) {
	root := t.TempDir()
	shape := filepath.Join(root, "Shapes.bas")
	shapeText := "Public Type Point\n X As Long\n Y As Double\nEnd Type\n"
	if err := os.WriteFile(shape, []byte(shapeText), 0600); err != nil {
		t.Fatal(err)
	}
	uri := lsconv.FileNameToURI(filepath.Join(root, "Main.bas"))
	c := newTestClient(t)
	c.send(1, "initialize", map[string]any{"rootUri": lsconv.FileNameToURI(root), "capabilities": map[string]any{}})
	var initialized struct {
		Capabilities struct {
			Completion struct {
				TriggerCharacters []string `json:"triggerCharacters"`
				ResolveProvider   bool     `json:"resolveProvider"`
			} `json:"completionProvider"`
		} `json:"capabilities"`
	}
	if err := json.Unmarshal(c.response(1)["result"], &initialized); err != nil || !reflect.DeepEqual(initialized.Capabilities.Completion.TriggerCharacters, []string{"."}) || initialized.Capabilities.Completion.ResolveProvider {
		t.Fatal(initialized, err)
	}
	c.send(nil, "initialized", map[string]any{})
	text := "Option Explicit\nSub Run()\n Dim p As Shapes.Point\n p.\nEnd Sub\n"
	c.send(nil, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 1, "text": text}})
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri}, "position": lsproto.Position{Line: 3, Character: 3},
		"context": map[string]any{"triggerKind": 2, "triggerCharacter": "."},
	}
	c.send(2, "textDocument/completion", params)
	var result lsproto.CompletionList
	if err := json.Unmarshal(c.response(2)["result"], &result); err != nil || len(result.Items) != 2 || result.IsIncomplete {
		t.Fatal(result, err)
	}
	x, y := result.Items[0], result.Items[1]
	if x.Label != "X" || x.Kind != lsproto.CompletionItemKindField || x.Detail != "X As Long" || y.Label != "Y" || y.Detail != "Y As Double" {
		t.Fatal(result.Items)
	}
	if x.TextEdit != (lsproto.TextEdit{Range: lsproto.Range{Start: lsproto.Position{Line: 3, Character: 3}, End: lsproto.Position{Line: 3, Character: 3}}, NewText: "X"}) {
		t.Fatal(x.TextEdit)
	}
	c.send(nil, "textDocument/didChange", map[string]any{
		"textDocument":   map[string]any{"uri": uri, "version": 2},
		"contentChanges": []lsproto.TextDocumentContentChangeEvent{{Range: &x.TextEdit.Range, Text: "x"}},
	})
	params["position"] = lsproto.Position{Line: 3, Character: 4}
	params["context"] = map[string]any{"triggerKind": 1} // Ctrl+Space after a partial name.
	c.send(3, "textDocument/completion", params)
	if err := json.Unmarshal(c.response(3)["result"], &result); err != nil || len(result.Items) != 2 || result.Items[0].TextEdit.Range.End.Character != 4 {
		t.Fatal("prefix was not replaced", result, err)
	}
	c.send(nil, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{
		"uri": lsconv.FileNameToURI(shape), "version": 1, "text": strings.Replace(shapeText, "Y As Double", "Z As String", 1),
	}})
	c.send(4, "textDocument/completion", params)
	if err := json.Unmarshal(c.response(4)["result"], &result); err != nil || len(result.Items) != 2 || result.Items[1].Label != "Z" || result.Items[1].Detail != "Z As String" {
		t.Fatal("unsaved cross-file edit was ignored", result, err)
	}
	params["position"] = lsproto.Position{Line: -1}
	c.send(5, "textDocument/completion", params)
	if !bytes.Contains(c.response(5)["error"], []byte("-32602")) {
		t.Fatal("invalid completion position accepted")
	}
	params["position"] = lsproto.Position{Line: 1, Character: 0}
	c.send(6, "textDocument/completion", params)
	if got := c.response(6); string(got["result"]) != `{"isIncomplete":false,"items":[]}` {
		t.Fatal("non-member completion was not an empty list", got)
	}
	c.send(7, "shutdown", nil)
	c.response(7)
	c.send(nil, "exit", nil)
}

package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
	"sihlfall.ch/vambas-vai/project"
)

type testClient struct {
	t        *testing.T
	conn     *connection
	server   *Server
	messages chan map[string]json.RawMessage
	done     chan error
	pending  []map[string]json.RawMessage
}

func newTestClient(t *testing.T) *testClient {
	t.Helper()
	client, server := net.Pipe()
	c := &testClient{t: t, conn: &connection{reader: bufio.NewReader(client), writer: client}, messages: make(chan map[string]json.RawMessage, 100), done: make(chan error, 1)}
	c.server = NewServer(server, server)
	go func() { c.done <- c.server.Run(context.Background()); _ = server.Close() }()
	go func() {
		defer close(c.messages)
		for {
			body, err := c.conn.read()
			if err != nil {
				return
			}
			var value map[string]json.RawMessage
			if json.Unmarshal(body, &value) != nil {
				return
			}
			c.messages <- value
		}
	}()
	t.Cleanup(func() {
		_ = client.Close()
		select {
		case <-c.done:
		case <-time.After(5 * time.Second):
			t.Error("server did not stop")
		}
	})
	return c
}
func (c *testClient) send(id any, method string, params any) {
	c.t.Helper()
	value := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id != nil {
		value["id"] = id
	}
	if err := c.conn.write(value); err != nil {
		c.t.Fatal(err)
	}
}
func (c *testClient) wait(match func(map[string]json.RawMessage) bool) map[string]json.RawMessage {
	c.t.Helper()
	for i, value := range c.pending {
		if match(value) {
			c.pending = append(c.pending[:i], c.pending[i+1:]...)
			return value
		}
	}
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case value, ok := <-c.messages:
			if !ok {
				c.t.Fatal("connection ended before response")
			}
			if match(value) {
				return value
			}
			c.pending = append(c.pending, value)
		case <-timer.C:
			c.t.Fatal("timed out waiting for message")
		}
	}
}
func (c *testClient) response(id int) map[string]json.RawMessage {
	c.t.Helper()
	return c.wait(func(value map[string]json.RawMessage) bool {
		var got int
		return json.Unmarshal(value["id"], &got) == nil && got == id
	})
}
func (c *testClient) diagnostics(uri lsproto.DocumentURI, version *int, empty bool) lsproto.PublishDiagnosticsParams {
	c.t.Helper()
	var result lsproto.PublishDiagnosticsParams
	c.wait(func(value map[string]json.RawMessage) bool {
		if string(value["method"]) != `"textDocument/publishDiagnostics"` {
			return false
		}
		var params lsproto.PublishDiagnosticsParams
		if json.Unmarshal(value["params"], &params) != nil || params.URI != uri || (len(params.Diagnostics) == 0) != empty {
			return false
		}
		if version == nil && params.Version != nil || version != nil && (params.Version == nil || *version != *params.Version) {
			return false
		}
		result = params
		return true
	})
	return result
}
func (c *testClient) initialize(root string, hierarchical bool) {
	c.t.Helper()
	params := map[string]any{"capabilities": map[string]any{"textDocument": map[string]any{"documentSymbol": map[string]any{"hierarchicalDocumentSymbolSupport": hierarchical}}}}
	if root != "" {
		params["rootUri"] = lsconv.FileNameToURI(root)
	}
	c.send(1, "initialize", params)
	value := c.response(1)
	if value["error"] != nil || !bytes.Contains(value["result"], []byte(`"positionEncoding":"utf-16"`)) {
		c.t.Fatal(value)
	}
	c.send(nil, "initialized", map[string]any{})
}

func TestLifecycleEditsAndProjectQueries(t *testing.T) {
	root := t.TempDir()
	utilities := filepath.Join(root, "Utilities.bas")
	utilCode := "Public Function Answer(ByVal count As Long) As Long\n Answer = count\nEnd Function\n"
	if err := os.WriteFile(utilities, []byte(utilCode), 0600); err != nil {
		t.Fatal(err)
	}
	main := filepath.Join(root, "Main.bas")
	mainURI, utilURI := lsconv.FileNameToURI(main), lsconv.FileNameToURI(utilities)
	code := "Option Explicit\nSub Run()\n Dim result As Long\n result = Utilities.Answer(count:=1)\nEnd Sub\n"
	c := newTestClient(t)
	c.send(99, "textDocument/hover", map[string]any{})
	if !bytes.Contains(c.response(99)["error"], []byte("-32002")) {
		t.Fatal("pre-initialize request accepted")
	}
	c.initialize(root, true)
	c.send(nil, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": mainURI, "version": 1, "text": code}})
	v1 := 1
	c.diagnostics(mainURI, &v1, true)
	params := lsproto.TextDocumentPositionParams{TextDocument: lsproto.TextDocumentIdentifier{URI: mainURI}, Position: lsproto.Position{Line: 3, Character: 23}}
	c.send(2, "textDocument/hover", params)
	var hover lsproto.Hover
	if err := json.Unmarshal(c.response(2)["result"], &hover); err != nil || !strings.Contains(hover.Contents.Value, "Function Answer") {
		t.Fatal(hover, err)
	}
	c.send(3, "textDocument/definition", params)
	var definitions []lsproto.Location
	if err := json.Unmarshal(c.response(3)["result"], &definitions); err != nil || len(definitions) != 1 || definitions[0].URI != utilURI {
		t.Fatal(definitions, err)
	}
	c.send(4, "textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": mainURI}})
	var symbols []lsproto.DocumentSymbol
	if err := json.Unmarshal(c.response(4)["result"], &symbols); err != nil || len(symbols) != 1 || len(symbols[0].Children) != 1 {
		t.Fatal(symbols, err)
	}
	c.send(nil, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": utilURI, "version": 1, "text": strings.ReplaceAll(utilCode, "Answer", "Renamed")}})
	c.diagnostics(mainURI, &v1, false)
	c.send(nil, "textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": mainURI, "version": 2}, "contentChanges": []lsproto.TextDocumentContentChangeEvent{{Range: &lsproto.Range{Start: lsproto.Position{Line: 3, Character: 20}, End: lsproto.Position{Line: 3, Character: 26}}, Text: "Renamed"}}})
	v2 := 2
	c.diagnostics(mainURI, &v2, true)
	// Closing restores the original disk declaration and invalidates callers.
	c.send(nil, "textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": utilURI}})
	c.diagnostics(utilURI, nil, true)
	c.diagnostics(mainURI, &v2, false)
	c.send(nil, "textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": mainURI}})
	c.diagnostics(mainURI, nil, true)
	c.send(5, "no/suchMethod", map[string]any{})
	if !bytes.Contains(c.response(5)["error"], []byte("-32601")) {
		t.Fatal("unknown method accepted")
	}
	c.send(6, "shutdown", nil)
	if result := c.response(6); string(result["result"]) != "null" || result["error"] != nil {
		t.Fatal(result)
	}
	c.send(7, "textDocument/documentSymbol", map[string]any{})
	if !bytes.Contains(c.response(7)["error"], []byte("-32600")) {
		t.Fatal("request after shutdown accepted")
	}
	c.send(nil, "exit", nil)
}

func TestFlatSymbolsAndCancellation(t *testing.T) {
	c := newTestClient(t)
	c.initialize("", false)
	uri := lsconv.FileNameToURI(filepath.Join(t.TempDir(), "Main.bas"))
	c.send(nil, "textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 1, "text": "Sub Run()\n Dim value As Long\nEnd Sub\n"}})
	c.send(2, "textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": uri}})
	result := c.response(2)
	if !bytes.Contains(result["result"], []byte(`"containerName":"Run"`)) || !bytes.Contains(result["result"], []byte(`"location"`)) {
		t.Fatal(string(result["result"]))
	}
	for i := 0; i < 10; i++ {
		c.send(2, "textDocument/documentSymbol", map[string]any{"textDocument": map[string]any{"uri": uri}})
		if value := c.response(2); value["error"] != nil {
			t.Fatal("completed request ID could not be reused", value)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.server.requestsMu.Lock()
	c.server.requests[`"cancel-me"`] = cancel
	c.server.requestsMu.Unlock()
	c.send(nil, "$/cancelRequest", map[string]any{"id": "cancel-me"})
	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("cancel notification ignored")
	}
	c.server.requestsMu.Lock()
	delete(c.server.requests, `"cancel-me"`)
	c.server.requestsMu.Unlock()
	c.send(3, "shutdown", nil)
	c.response(3)
	c.send(nil, "exit", nil)
}

func TestCancelledQueryResponse(t *testing.T) {
	var output bytes.Buffer
	s := NewServer(strings.NewReader(""), &output)
	s.session = project.NewSession(project.Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.startRequest(ctx, message{ID: json.RawMessage(`1`), Method: "textDocument/hover", Params: json.RawMessage(`{"textDocument":{"uri":"file:///Main.bas"},"position":{"line":0,"character":0}}`)})
	s.workers.Wait()
	conn := connection{reader: bufio.NewReader(&output)}
	body, err := conn.read()
	if err != nil || !bytes.Contains(body, []byte(`"code":-32800`)) {
		t.Fatal(string(body), err)
	}
}

func TestFailedInitializationCanShutDown(t *testing.T) {
	c := newTestClient(t)
	c.send(1, "initialize", map[string]any{"rootUri": lsconv.FileNameToURI(t.TempDir()), "initializationOptions": map[string]any{"projectFile": "Missing.vbp"}})
	if value := c.response(1); value["error"] == nil {
		t.Fatal("bad project initialized", value)
	}
	c.send(2, "shutdown", nil)
	if value := c.response(2); value["error"] != nil || string(value["result"]) != "null" {
		t.Fatal("failed initialization prevented shutdown", value)
	}
	c.send(nil, "exit", nil)
}

func TestLegacyEncodingSelectionSurvivesReload(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "Utilities.bas")
	// C3 A4 is valid UTF-8 but means two characters in CP1252. An explicit
	// encoding selection must be preserved across save/watch reloads.
	code := "Public Function Answer(Optional ByVal value As String = \"\xc3\xa4\") As String\n Answer = value\nEnd Function\n"
	if err := os.WriteFile(name, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	c := newTestClient(t)
	c.send(1, "initialize", map[string]any{"rootUri": lsconv.FileNameToURI(root), "initializationOptions": map[string]any{"sourceEncoding": "windows1252"}})
	if value := c.response(1); value["error"] != nil {
		t.Fatal(value)
	}
	params := lsproto.TextDocumentPositionParams{TextDocument: lsproto.TextDocumentIdentifier{URI: lsconv.FileNameToURI(name)}, Position: lsproto.Position{Line: 0, Character: 18}}
	for i := 2; i <= 3; i++ {
		c.send(i, "textDocument/hover", params)
		var hover lsproto.Hover
		if err := json.Unmarshal(c.response(i)["result"], &hover); err != nil || !strings.Contains(hover.Contents.Value, "Ã¤") {
			t.Fatal("encoding selection lost", hover, err)
		}
		c.send(nil, "workspace/didChangeWatchedFiles", map[string]any{"changes": []any{}})
	}
	c.send(4, "shutdown", nil)
	c.response(4)
	c.send(nil, "exit", nil)
}

func TestSupersededDiagnosticsCannotPublish(t *testing.T) {
	var output bytes.Buffer
	s := NewServer(strings.NewReader(""), &output)
	s.session = project.NewSession(project.Config{Name: "Demo"})
	uri := lsconv.FileNameToURI(filepath.Join(t.TempDir(), "Main.bas"))
	if err := s.session.Open(uri, 1, "Option Explicit\nSub Run()\n absent = 1\nEnd Sub\n"); err != nil {
		t.Fatal(err)
	}
	// Hold publication while both revisions are computed. The old worker must
	// recheck its captured snapshot after acquiring the lock.
	s.publication.Lock()
	s.scheduleDiagnostics(context.Background())
	if err := s.session.Change(uri, 2, []lsproto.TextDocumentContentChangeEvent{{Text: "Sub Run()\nEnd Sub\n"}}); err != nil {
		t.Fatal(err)
	}
	s.scheduleDiagnostics(context.Background())
	s.publication.Unlock()
	s.workers.Wait()
	conn := connection{reader: bufio.NewReader(&output)}
	body, err := conn.read()
	if err != nil {
		t.Fatal(err)
	}
	var value struct {
		Params lsproto.PublishDiagnosticsParams `json:"params"`
	}
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatal(err)
	}
	if value.Params.Version == nil || *value.Params.Version != 2 || len(value.Params.Diagnostics) != 0 {
		t.Fatal(string(body))
	}
	if _, err := conn.read(); err != io.EOF {
		t.Fatal("superseded diagnostics published", err)
	}
}

func TestTransportValidation(t *testing.T) {
	for _, input := range []string{"Content-Length: -1\r\n\r\n", "Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}", "Other: header\r\n\r\n", "Content-Length: 16777217\r\n\r\n", "Content-Length: 4\r\n\r\n{}", "Content-Length: 2\r\n", "Content-Length: 2", strings.Repeat("x", 10000)} {
		conn := connection{reader: bufio.NewReader(strings.NewReader(input))}
		if _, err := conn.read(); err == nil {
			t.Errorf("accepted %q", input)
		}
	}
	var output bytes.Buffer
	conn := connection{reader: bufio.NewReader(strings.NewReader("Content-Length: 2\r\nContent-Type: application/vscode-jsonrpc; charset=utf-8\r\n\r\n{}")), writer: &output}
	if body, err := conn.read(); err != nil || string(body) != "{}" {
		t.Fatal(string(body), err)
	}
	if err := conn.write(map[string]any{"text": "😀"}); err != nil {
		t.Fatal(err)
	}
	conn.reader = bufio.NewReader(&output)
	if body, err := conn.read(); err != nil || !bytes.Contains(body, []byte("😀")) {
		t.Fatal(string(body), err)
	}
	if _, err := conn.read(); err != io.EOF {
		t.Fatal(err)
	}
}

// Package lsp implements the stdio Language Server Protocol adapter. Workspace
// changes are ordered on the reader; queries run on immutable captured snapshots.
package lsp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sync"

	"sihlfall.ch/vambas-vai/ls"
	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
	"sihlfall.ch/vambas-vai/project"
)

var ErrExitWithoutShutdown = errors.New("exit received before shutdown")

type Server struct {
	conn                  *connection
	session               *project.Session
	root, projectFile     string
	sourceEncoding        string
	initialized, shutdown bool
	initializationFailed  bool
	hierarchicalSymbols   bool
	semanticLegend        lsproto.SemanticTokensLegend
	diagnosticCancel      context.CancelFunc
	// Serializes snapshot changes and diagnostic publication to prevent stale pushes.
	publication sync.Mutex
	requestsMu  sync.Mutex
	requests    map[string]context.CancelFunc
	workers     sync.WaitGroup
}

func NewServer(reader io.Reader, writer io.Writer) *Server {
	return &Server{conn: &connection{reader: bufio.NewReader(reader), writer: writer}, requests: make(map[string]context.CancelFunc)}
}

// Run serves one connection. Closing the input ends the server. Callers using a
// cancellable context should also close their reader to interrupt a pending read.
func (s *Server) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer func() { cancel(); s.workers.Wait() }()
	for {
		body, err := s.conn.read()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		var msg message
		if !json.Valid(body) {
			s.conn.reply(nil, nil, &responseError{-32700, "Invalid JSON"})
			continue
		}
		if json.Unmarshal(body, &msg) != nil || msg.JSONRPC != "2.0" || msg.Method == "" || !validID(msg.ID) {
			s.conn.reply(nil, nil, &responseError{-32600, "Invalid request"})
			continue
		}
		isRequest := len(msg.ID) > 0
		if msg.Method == "exit" && !isRequest {
			if !s.shutdown {
				return ErrExitWithoutShutdown
			}
			return nil
		}
		if msg.Method == "$/cancelRequest" && !isRequest {
			var params struct {
				ID json.RawMessage `json:"id"`
			}
			if json.Unmarshal(msg.Params, &params) == nil {
				s.requestsMu.Lock()
				if cancel := s.requests[idKey(params.ID)]; cancel != nil {
					cancel()
				}
				s.requestsMu.Unlock()
			}
			continue
		}
		if msg.Method == "initialize" && isRequest {
			if s.initialized || s.shutdown {
				s.conn.reply(msg.ID, nil, &responseError{-32600, "Already initialized"})
				continue
			}
			result, err := s.initialize(ctx, msg.Params)
			if err != nil {
				s.initializationFailed = true
				s.conn.reply(msg.ID, nil, &responseError{-32602, err.Error()})
			} else {
				s.conn.reply(msg.ID, result, nil)
			}
			continue
		}
		// A client must also be able to release a server after rejected
		// initialization, rather than timing out and force-killing it.
		if msg.Method == "shutdown" && isRequest && (s.initialized || s.initializationFailed) && !s.shutdown {
			s.publication.Lock()
			s.shutdown = true
			if s.diagnosticCancel != nil {
				s.diagnosticCancel()
			}
			s.publication.Unlock()
			s.requestsMu.Lock()
			for _, cancel := range s.requests {
				cancel()
			}
			s.requestsMu.Unlock()
			s.conn.reply(msg.ID, nil, nil)
			continue
		}
		if !s.initialized || s.shutdown {
			if isRequest {
				code := -32002
				if s.shutdown {
					code = -32600
				}
				s.conn.reply(msg.ID, nil, &responseError{code, "Server is not running"})
			}
			continue
		}
		if !isRequest {
			s.publication.Lock()
			err := s.handleNotification(ctx, msg)
			s.publication.Unlock()
			if err != nil {
				s.conn.notify("window/logMessage", map[string]any{"type": 1, "message": err.Error()})
			}
			continue
		}
		s.startRequest(ctx, msg)
	}
}

func validID(id json.RawMessage) bool {
	if len(id) == 0 {
		return true
	}
	var value any
	if json.Unmarshal(id, &value) != nil {
		return false
	}
	switch value := value.(type) {
	case string:
		return true
	case float64:
		return value == math.Trunc(value) && value >= math.MinInt32 && value <= math.MaxInt32
	}
	return false
}
func idKey(id json.RawMessage) string {
	var value any
	_ = json.Unmarshal(id, &value)
	data, _ := json.Marshal(value)
	return string(data)
}
func decodeParams(data json.RawMessage, target any) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || data[0] != '{' {
		return fmt.Errorf("expected object parameters")
	}
	return json.Unmarshal(data, target)
}

func (s *Server) initialize(ctx context.Context, raw json.RawMessage) (any, error) {
	var params struct {
		RootURI          lsproto.DocumentURI `json:"rootUri"`
		WorkspaceFolders []struct {
			URI lsproto.DocumentURI `json:"uri"`
		} `json:"workspaceFolders"`
		InitializationOptions struct {
			ProjectFile    string `json:"projectFile"`
			SourceEncoding string `json:"sourceEncoding"`
		} `json:"initializationOptions"`
		Capabilities struct {
			TextDocument struct {
				DocumentSymbol struct {
					Hierarchical bool `json:"hierarchicalDocumentSymbolSupport"`
				} `json:"documentSymbol"`
				SemanticTokens *struct {
					TokenTypes     []string `json:"tokenTypes"`
					TokenModifiers []string `json:"tokenModifiers"`
				} `json:"semanticTokens"`
			} `json:"textDocument"`
		} `json:"capabilities"`
	}
	if err := decodeParams(raw, &params); err != nil {
		return nil, err
	}
	if len(params.WorkspaceFolders) > 1 {
		return nil, fmt.Errorf("this server currently supports one workspace folder")
	}
	uri := params.RootURI
	if len(params.WorkspaceFolders) == 1 {
		uri = params.WorkspaceFolders[0].URI
	}
	root := ""
	if uri != "" {
		var err error
		root, err = lsconv.URIToFileName(uri)
		if err != nil {
			return nil, err
		}
	}
	cfg, err := project.LoadConfigWithOptions(ctx, project.LoadOptions{Root: root, ProjectFile: params.InitializationOptions.ProjectFile, SourceEncoding: params.InitializationOptions.SourceEncoding})
	if err != nil {
		return nil, err
	}
	s.root, s.projectFile = cfg.Root, cfg.ProjectFile
	s.sourceEncoding = cfg.SourceEncoding
	s.session = project.NewSession(cfg)
	s.hierarchicalSymbols = params.Capabilities.TextDocument.DocumentSymbol.Hierarchical
	s.initialized = true
	capabilities := map[string]any{
		"positionEncoding": "utf-16",
		"textDocumentSync": map[string]any{"openClose": true, "change": 2, "save": map[string]any{"includeText": false}},
		"hoverProvider":    true, "definitionProvider": true, "documentSymbolProvider": true,
		"completionProvider": map[string]any{"triggerCharacters": []string{"."}, "resolveProvider": false},
	}
	if semantic := params.Capabilities.TextDocument.SemanticTokens; semantic != nil {
		legend := ls.SemanticTokensLegend()
		s.semanticLegend = lsproto.SemanticTokensLegend{
			TokenTypes:     supportedSemanticTokens(legend.TokenTypes, semantic.TokenTypes),
			TokenModifiers: supportedSemanticTokens(legend.TokenModifiers, semantic.TokenModifiers),
		}
		capabilities["semanticTokensProvider"] = map[string]any{"legend": s.semanticLegend, "full": true, "range": true}
	}
	return map[string]any{
		"capabilities": capabilities,
		"serverInfo":   map[string]any{"name": "vambas-language-server", "version": "0.1.0"},
	}, nil
}

func (s *Server) handleNotification(ctx context.Context, msg message) error {
	switch msg.Method {
	case "initialized":
		s.scheduleDiagnostics(ctx)
	case "textDocument/didOpen":
		var params struct {
			TextDocument struct {
				URI     lsproto.DocumentURI `json:"uri"`
				Version int                 `json:"version"`
				Text    string              `json:"text"`
			} `json:"textDocument"`
		}
		if err := decodeParams(msg.Params, &params); err != nil {
			return err
		}
		d := params.TextDocument
		if err := s.session.Open(d.URI, d.Version, d.Text); err != nil {
			return err
		}
		s.scheduleDiagnostics(ctx)
	case "textDocument/didChange":
		var params struct {
			TextDocument struct {
				URI     lsproto.DocumentURI `json:"uri"`
				Version int                 `json:"version"`
			} `json:"textDocument"`
			ContentChanges []lsproto.TextDocumentContentChangeEvent `json:"contentChanges"`
		}
		if err := decodeParams(msg.Params, &params); err != nil {
			return err
		}
		if err := s.session.Change(params.TextDocument.URI, params.TextDocument.Version, params.ContentChanges); err != nil {
			return err
		}
		s.scheduleDiagnostics(ctx)
	case "textDocument/didClose", "textDocument/didSave":
		var params struct {
			TextDocument lsproto.TextDocumentIdentifier `json:"textDocument"`
		}
		if err := decodeParams(msg.Params, &params); err != nil {
			return err
		}
		// Refresh disk before removing the overlay, including files saved since open.
		reloadErr := s.reload(ctx)
		if msg.Method == "textDocument/didClose" {
			if err := s.session.Close(params.TextDocument.URI); err != nil {
				return err
			}
			s.conn.notify("textDocument/publishDiagnostics", lsproto.PublishDiagnosticsParams{URI: params.TextDocument.URI, Diagnostics: []lsproto.Diagnostic{}})
		}
		s.scheduleDiagnostics(ctx)
		return reloadErr
	case "workspace/didChangeWatchedFiles":
		if err := s.reload(ctx); err != nil {
			return err
		}
		s.scheduleDiagnostics(ctx)
	}
	return nil
}

func (s *Server) reload(ctx context.Context) error {
	cfg, err := project.LoadConfigWithOptions(ctx, project.LoadOptions{Root: s.root, ProjectFile: s.projectFile, SourceEncoding: s.sourceEncoding})
	if err == nil {
		s.session.SetConfig(cfg)
	}
	return err
}

func (s *Server) scheduleDiagnostics(ctx context.Context) {
	if s.diagnosticCancel != nil {
		s.diagnosticCancel()
	}
	ctx, s.diagnosticCancel = context.WithCancel(ctx)
	snapshot := s.session.Snapshot()
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		service := ls.NewLanguageService(snapshot)
		for _, doc := range snapshot.Documents() {
			if !doc.Open {
				continue
			}
			diagnostics, err := service.GetDiagnostics(ctx, doc.URI)
			if err != nil {
				return
			}
			s.publication.Lock()
			if ctx.Err() != nil || s.shutdown || s.session.Snapshot() != snapshot {
				s.publication.Unlock()
				return
			}
			version := doc.Version
			s.conn.notify("textDocument/publishDiagnostics", lsproto.PublishDiagnosticsParams{URI: doc.URI, Version: &version, Diagnostics: diagnostics})
			s.publication.Unlock()
		}
	}()
}

func (s *Server) startRequest(parent context.Context, msg message) {
	key := idKey(msg.ID)
	ctx, cancel := context.WithCancel(parent)
	s.requestsMu.Lock()
	if s.requests[key] != nil {
		s.requestsMu.Unlock()
		cancel()
		s.conn.reply(msg.ID, nil, &responseError{-32600, "Duplicate request ID"})
		return
	}
	s.requests[key] = cancel
	s.requestsMu.Unlock()
	// Capture before the reader processes another change notification.
	snapshot := s.session.Snapshot()
	s.workers.Add(1)
	go func() {
		defer s.workers.Done()
		defer cancel()
		reply := func(value any, err *responseError) {
			// A client may reuse this ID as soon as it receives the response.
			s.requestsMu.Lock()
			delete(s.requests, key)
			s.requestsMu.Unlock()
			s.conn.reply(msg.ID, value, err)
		}
		service := ls.NewLanguageService(snapshot)
		var result any
		var err error
		switch msg.Method {
		case "textDocument/completion":
			var params lsproto.TextDocumentPositionParams
			err = decodeParams(msg.Params, &params)
			if err == nil {
				result, err = service.ProvideCompletion(ctx, params)
			}
		case "textDocument/semanticTokens/full", "textDocument/semanticTokens/range":
			var params struct {
				TextDocument lsproto.TextDocumentIdentifier `json:"textDocument"`
				Range        lsproto.Range                  `json:"range"`
			}
			err = decodeParams(msg.Params, &params)
			if err == nil {
				if msg.Method == "textDocument/semanticTokens/range" {
					result, err = service.ProvideSemanticTokensRange(ctx, params.TextDocument.URI, params.Range, s.semanticLegend)
				} else {
					result, err = service.ProvideSemanticTokens(ctx, params.TextDocument.URI, s.semanticLegend)
				}
			}
		case "textDocument/hover", "textDocument/definition":
			var params lsproto.TextDocumentPositionParams
			err = decodeParams(msg.Params, &params)
			if err == nil {
				if msg.Method == "textDocument/hover" {
					result, err = service.ProvideHover(ctx, params)
				} else {
					result, err = service.ProvideDefinition(ctx, params)
				}
			}
		case "textDocument/documentSymbol":
			var params struct {
				TextDocument lsproto.TextDocumentIdentifier `json:"textDocument"`
			}
			err = decodeParams(msg.Params, &params)
			if err == nil {
				var symbols []*lsproto.DocumentSymbol
				symbols, err = service.ProvideDocumentSymbols(ctx, params.TextDocument.URI)
				result = symbols
				if !s.hierarchicalSymbols {
					result = flattenSymbols(symbols, params.TextDocument.URI)
				}
			}
		default:
			reply(nil, &responseError{-32601, "Method not found: " + msg.Method})
			return
		}
		if ctx.Err() != nil {
			reply(nil, &responseError{-32800, "Request cancelled"})
		} else if err != nil {
			reply(nil, &responseError{-32602, err.Error()})
		} else {
			reply(result, nil)
		}
	}()
}

func flattenSymbols(symbols []*lsproto.DocumentSymbol, uri lsproto.DocumentURI) []any {
	result := []any{}
	var visit func([]*lsproto.DocumentSymbol, string)
	visit = func(symbols []*lsproto.DocumentSymbol, container string) {
		for _, symbol := range symbols {
			item := map[string]any{"name": symbol.Name, "kind": symbol.Kind, "location": lsproto.Location{URI: uri, Range: symbol.SelectionRange}}
			if container != "" {
				item["containerName"] = container
			}
			result = append(result, item)
			visit(symbol.Children, symbol.Name)
		}
	}
	visit(symbols, "")
	return result
}

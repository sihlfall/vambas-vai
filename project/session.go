package project

import (
	"context"
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
)

// Document is a value copied out of a snapshot; its text and line map are immutable.
type Document struct {
	URI      lsproto.DocumentURI
	FileName string
	Text     string
	Version  int
	Open     bool
	Lines    *lsconv.LineMap
}

type Snapshot struct {
	id        uint64
	documents map[lsproto.DocumentURI]Document
	inputs    []compiler.SourceFileInput
	name      string
	once      sync.Once
	program   *compiler.Program
}

func (s *Snapshot) ID() uint64 { return s.id }

func documentKey(uri lsproto.DocumentURI) lsproto.DocumentURI {
	if name, err := lsconv.URIToFileName(uri); err == nil {
		uri = lsconv.FileNameToURI(name)
	}
	if runtime.GOOS == "windows" {
		uri = lsproto.DocumentURI(strings.ToLower(string(uri)))
	}
	return uri
}

func (s *Snapshot) Document(uri lsproto.DocumentURI) (Document, bool) {
	d, ok := s.documents[documentKey(uri)]
	return d, ok
}

// ReadFile and LSPLineMap implement the language-service host without exposing
// overlays, document versions, or mutable session state to language services.
func (s *Snapshot) ReadFile(fileName string) (string, bool) {
	d, ok := s.Document(lsconv.FileNameToURI(fileName))
	return d.Text, ok
}

func (s *Snapshot) LSPLineMap(fileName string) *lsconv.LineMap {
	d, _ := s.Document(lsconv.FileNameToURI(fileName))
	return d.Lines
}

func (s *Snapshot) Documents() []Document {
	docs := make([]Document, 0, len(s.documents))
	for _, d := range s.documents {
		docs = append(docs, d)
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].URI < docs[j].URI })
	return docs
}

// Program lazily builds fresh syntax/binding for this revision. Cancellation is
// checked around construction; parser/binder/checker do not yet preempt mid-pass.
func (s *Snapshot) Program(ctx context.Context) (*compiler.Program, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.once.Do(func() {
		s.program = compiler.NewProgram(compiler.ProgramOptions{ProjectName: s.name, Sources: s.inputs})
	})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.program, nil
}

// Session serializes overlay changes and publishes immutable snapshots. Queries
// retain their captured snapshot while later edits proceed independently.
type Session struct {
	mu       sync.Mutex
	config   Config
	overlays map[lsproto.DocumentURI]Document
	current  *Snapshot
	revision uint64
}

func NewSession(config Config) *Session {
	config.Sources = append([]compiler.SourceFileInput(nil), config.Sources...)
	config.References = append([]string(nil), config.References...)
	s := &Session{config: config, overlays: make(map[lsproto.DocumentURI]Document)}
	s.rebuild()
	return s
}
func (s *Session) Snapshot() *Snapshot { s.mu.Lock(); defer s.mu.Unlock(); return s.current }

func (s *Session) Open(uri lsproto.DocumentURI, version int, text string) error {
	name, err := lsconv.URIToFileName(uri)
	if err != nil {
		return err
	}
	uri = lsconv.FileNameToURI(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.overlays[documentKey(uri)]; exists {
		return fmt.Errorf("document already open: %s", uri)
	}
	s.overlays[documentKey(uri)] = Document{URI: uri, FileName: name, Text: text, Version: version, Open: true, Lines: lsconv.NewLineMap(text)}
	s.rebuild()
	return nil
}

func (s *Session) Change(uri lsproto.DocumentURI, version int, changes []lsproto.TextDocumentContentChangeEvent) error {
	name, err := lsconv.URIToFileName(uri)
	if err != nil {
		return err
	}
	uri = lsconv.FileNameToURI(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.overlays[documentKey(uri)]
	if !ok {
		return fmt.Errorf("document is not open: %s", uri)
	}
	if version <= d.Version {
		return fmt.Errorf("document version %d is not newer than %d", version, d.Version)
	}
	// Changes apply in order to each preceding result. Commit only if all are valid.
	for _, change := range changes {
		if change.Range == nil {
			d.Text = change.Text
		} else {
			start, err := d.Lines.Offset(change.Range.Start)
			if err != nil {
				return err
			}
			end, err := d.Lines.Offset(change.Range.End)
			if err != nil {
				return err
			}
			if start > end {
				return fmt.Errorf("reversed edit range")
			}
			d.Text = d.Text[:start] + change.Text + d.Text[end:]
		}
		d.Lines = lsconv.NewLineMap(d.Text)
	}
	d.Version = version
	s.overlays[documentKey(uri)] = d
	s.rebuild()
	return nil
}

func (s *Session) Close(uri lsproto.DocumentURI) error {
	name, err := lsconv.URIToFileName(uri)
	if err != nil {
		return err
	}
	uri = lsconv.FileNameToURI(name)
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.overlays, documentKey(uri))
	s.rebuild()
	return nil
}

// SetConfig replaces the disk view, preserving all unsaved overlays.
func (s *Session) SetConfig(config Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	config.Sources = append([]compiler.SourceFileInput(nil), config.Sources...)
	config.References = append([]string(nil), config.References...)
	s.config = config
	s.rebuild()
}

func (s *Session) rebuild() {
	s.revision++
	snap := &Snapshot{id: s.revision, name: s.config.Name, documents: make(map[lsproto.DocumentURI]Document)}
	inputs := make(map[lsproto.DocumentURI]compiler.SourceFileInput)
	for _, input := range s.config.Sources {
		uri := lsconv.FileNameToURI(input.FileName)
		inputs[documentKey(uri)] = input
		snap.documents[documentKey(uri)] = Document{URI: uri, FileName: input.FileName, Text: input.Text, Lines: lsconv.NewLineMap(input.Text)}
	}
	for uri, d := range s.overlays {
		snap.documents[uri] = d
		input := inputs[uri]
		input.FileName, input.Text = d.FileName, d.Text
		inputs[uri] = input
	}
	for _, d := range snap.Documents() {
		input := inputs[documentKey(d.URI)]
		input.Text = codeText(input.FileName, input.Text)
		snap.inputs = append(snap.inputs, input)
	}
	s.current = snap
}

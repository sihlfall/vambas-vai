package project

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
)

func TestOverlaySnapshots(t *testing.T) {
	ctx := context.Background()
	name := filepath.Join(t.TempDir(), "Main.bas")
	uri := lsconv.FileNameToURI(name)
	disk := "Public Const Answer As Long = 1\n"
	s := NewSession(Config{Name: "Demo", Sources: []compiler.SourceFileInput{{FileName: name, Text: disk}}})
	first := s.Snapshot()
	p1, err := first.Program(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Open(uri, 1, "' 😀\r\n"+disk); err != nil {
		t.Fatal(err)
	}
	second := s.Snapshot()
	changes := []lsproto.TextDocumentContentChangeEvent{
		{Range: &lsproto.Range{Start: lsproto.Position{Line: 0, Character: 2}, End: lsproto.Position{Line: 0, Character: 4}}, Text: "ok"},
		{Range: &lsproto.Range{Start: lsproto.Position{Line: 0, Character: 2}, End: lsproto.Position{Line: 0, Character: 4}}, Text: "edited"},
	}
	if err := s.Change(uri, 2, changes); err != nil {
		t.Fatal(err)
	}
	third := s.Snapshot()
	d, _ := third.Document(uri)
	if d.Text != "' edited\r\n"+disk || d.Version != 2 {
		t.Fatal(d)
	}
	before, _ := second.Document(uri)
	if before.Text != "' 😀\r\n"+disk {
		t.Fatal("old snapshot mutated")
	}
	p2, err := third.Program(ctx)
	if err != nil || p1 == p2 || p1.SourceFile(name) == p2.SourceFile(name) {
		t.Fatal("bound trees reused", err)
	}
	if err := s.Change(uri, 2, changes); err == nil {
		t.Fatal("stale version accepted")
	}
	bad := append([]lsproto.TextDocumentContentChangeEvent{{Text: "new"}}, lsproto.TextDocumentContentChangeEvent{Range: &lsproto.Range{Start: lsproto.Position{Line: -1}, End: lsproto.Position{Line: -1}}})
	if err := s.Change(uri, 3, bad); err == nil || s.Snapshot() != third {
		t.Fatal("bad edits partially committed")
	}
	if err := s.Close(uri); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Snapshot().Document(uri)
	if d.Open || d.Text != disk {
		t.Fatal("disk view not restored", d)
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Snapshot().Program(ctx); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestConcurrentQueriesAndEdits(t *testing.T) {
	s := NewSession(Config{Name: "Demo"})
	uri := lsconv.FileNameToURI(filepath.Join(t.TempDir(), "Main.bas"))
	if err := s.Open(uri, 1, "Dim value As Long\n"); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 2; i < 20; i++ {
		snapshot := s.Snapshot()
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := snapshot.Program(context.Background())
			if err != nil {
				t.Error(err)
				return
			}
			p.GetTypeChecker()
		}()
		if err := s.Change(uri, i, []lsproto.TextDocumentContentChangeEvent{{Text: "Dim value As String\n"}}); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}

func TestVBPAndExportedSources(t *testing.T) {
	root := t.TempDir()
	write := func(name, text string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	class := "VERSION 1.0 CLASS\r\nBEGIN\r\n  MultiUse = -1\r\nEND\r\nAttribute VB_Name = \"Counter\"\r\nPublic Value As Long\r\n"
	form := "VERSION 5.00\nBegin VB.Form Window\n  Caption = \"😀\"\n  Begin VB.CommandButton Button\n  End\nEnd\nAttribute VB_Name = \"Window\"\nPublic Sub Run()\nEnd Sub\n"
	write("Counter.cls", class)
	write("Window.frm", form)
	write("Main.bas", "Public n As Long\n")
	write("Demo.vbp", "Name=\"Demo\"\nModule=Main; Main.bas\nClass=Counter; Counter.cls\nForm=Window.frm\nReference=*\\G{example}#1.0#0#library.tlb#Library\n")
	cfg, err := LoadConfig(context.Background(), root, "")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "Demo" || len(cfg.Sources) != 3 || len(cfg.References) != 1 {
		t.Fatal(cfg)
	}
	snapshot := NewSession(cfg).Snapshot()
	p, err := snapshot.Program(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if diagnostics := p.ParseDiagnostics(); len(diagnostics) != 0 {
		t.Fatal(diagnostics[0].Message)
	}
	if ds := p.BindDiagnostics(); len(ds) != 0 {
		t.Fatal(ds)
	}
	if p.Module("Counter").ModuleKind != ast.ModuleKindClass || !p.Module("Window").PredeclaredID {
		t.Fatal("module identity lost")
	}
	for name, original := range map[string]string{"Counter.cls": class, "Window.frm": form} {
		file := p.SourceFile(filepath.Join(root, name))
		if len(file.Text) != len(original) || strings.Index(file.Text, "Public") != strings.Index(original, "Public") {
			t.Fatal("export offsets changed")
		}
		d, _ := snapshot.Document(lsconv.FileNameToURI(file.FileName))
		if d.Text != original {
			t.Fatal("original document lost")
		}
	}
	write("Other.vbp", "Name=\"Other\"\n")
	if _, err := LoadConfig(context.Background(), root, ""); err == nil {
		t.Fatal("ambiguous projects merged")
	}
	if _, err := LoadConfig(context.Background(), root, "Demo.vbp"); err != nil {
		t.Fatal(err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, filepath.Join(root, "Demo.vbp"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg, err := LoadConfig(context.Background(), "", relative); err != nil || cfg.ProjectFile != filepath.Join(root, "Demo.vbp") {
		t.Fatal("relative manifest without root was resolved twice", cfg, err)
	}
}

func TestInferenceAndReloadOverlays(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "Main.bas")
	if err := os.WriteFile(name, []byte("Dim disk As Long\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(context.Background(), root, "")
	if err != nil || len(cfg.Sources) != 1 {
		t.Fatal(cfg, err)
	}
	s := NewSession(cfg)
	uri := lsconv.FileNameToURI(name)
	if err := s.Open(uri, 1, "Dim unsaved As String\n"); err != nil {
		t.Fatal(err)
	}
	cfg.Sources[0].Text = "Dim changedOnDisk As Byte\n"
	s.SetConfig(cfg)
	d, _ := s.Snapshot().Document(uri)
	if !strings.Contains(d.Text, "unsaved") {
		t.Fatal("reload overwrote overlay")
	}
	if err := s.Close(uri); err != nil {
		t.Fatal(err)
	}
	d, _ = s.Snapshot().Document(uri)
	if d.Text != cfg.Sources[0].Text {
		t.Fatal("new disk view not restored")
	}
}

func TestIncompleteDesignerDoesNotHideCode(t *testing.T) {
	text := "VERSION 1.0 CLASS\nBEGIN\nAttribute VB_Name = \"Counter\"\nPublic value As Long\n"
	if got := codeText("Counter.cls", text); got != text {
		t.Fatal("unfinished export hid VBA code", got)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := LoadConfig(ctx, "", ""); err != context.Canceled {
		t.Fatal(err)
	}
}

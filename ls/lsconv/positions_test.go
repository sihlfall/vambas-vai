package lsconv

import (
	"path/filepath"
	"testing"

	"sihlfall.ch/vambas-vai/lsp/lsproto"
)

func TestUTF16AndLineEndings(t *testing.T) {
	m := NewLineMap("a😀b\r\ncé\rz\n")
	for _, pair := range []struct{ offset, line, char int }{{0, 0, 0}, {1, 0, 1}, {5, 0, 3}, {6, 0, 4}, {8, 1, 0}, {11, 1, 2}, {12, 2, 0}, {14, 3, 0}} {
		p := lsproto.Position{Line: pair.line, Character: pair.char}
		if got := m.Position(pair.offset); got != p {
			t.Errorf("Position(%d) = %+v, want %+v", pair.offset, got, p)
		}
		if got, err := m.Offset(p); err != nil || got != pair.offset {
			t.Errorf("Offset(%+v) = %d, %v", p, got, err)
		}
	}
	if _, err := m.Offset(lsproto.Position{Line: 0, Character: 2}); err == nil {
		t.Fatal("split surrogate accepted")
	}
	if got, err := m.Offset(lsproto.Position{Line: 0, Character: 500}); err != nil || got != 6 {
		t.Fatal("end of line not clamped", got, err)
	}
	if got, err := m.Offset(lsproto.Position{Line: 99}); err != nil || got != len(m.text) {
		t.Fatal("past-EOF line not clamped", got, err)
	}
	for _, p := range []lsproto.Position{{Line: -1}, {Character: -1}} {
		if _, err := m.Offset(p); err == nil {
			t.Fatal("invalid position accepted", p)
		}
	}
}

func TestFileURI(t *testing.T) {
	name := filepath.Join(t.TempDir(), "with space # é.bas")
	uri := FileNameToURI(name)
	if got, err := URIToFileName(uri); err != nil || got != name {
		t.Fatal(got, err, uri)
	}
	for _, uri := range []lsproto.DocumentURI{"untitled:Main.bas", "file://remote/a.bas", "file:relative", "file:///a.bas#fragment", "file:///a.bas?query"} {
		if _, err := URIToFileName(uri); err == nil {
			t.Fatal("invalid URI accepted", uri)
		}
	}
}

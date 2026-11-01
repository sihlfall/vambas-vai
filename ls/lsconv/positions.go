// Package lsconv bridges source byte offsets and LSP coordinates. The compiler
// never depends on protocol positions, as in typescript-go.
package lsconv

import (
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
)

type LineMap struct {
	text   string
	starts []int
}

func NewLineMap(text string) *LineMap {
	m := &LineMap{text: text, starts: []int{0}}
	for i := 0; i < len(text); i++ {
		if text[i] == '\r' {
			if i+1 < len(text) && text[i+1] == '\n' {
				i++
			}
			m.starts = append(m.starts, i+1)
		} else if text[i] == '\n' {
			m.starts = append(m.starts, i+1)
		}
	}
	return m
}

// Offset clamps characters past end-of-line and lines past EOF, as in
// typescript-go. Negative coordinates and split surrogate pairs are rejected.
func (m *LineMap) Offset(p lsproto.Position) (int, error) {
	if p.Line < 0 || p.Character < 0 {
		return 0, fmt.Errorf("invalid position %+v", p)
	}
	if p.Line >= len(m.starts) {
		return len(m.text), nil
	}
	i, units := m.starts[p.Line], 0
	for i < len(m.text) && m.text[i] != '\r' && m.text[i] != '\n' {
		if units == p.Character {
			return i, nil
		}
		r, n := utf8.DecodeRuneInString(m.text[i:])
		width := 1
		if r > 0xffff {
			width = 2
		}
		if units+width > p.Character {
			return 0, fmt.Errorf("position splits a surrogate pair")
		}
		units += width
		i += n
	}
	return i, nil
}

func (m *LineMap) Position(offset int) lsproto.Position {
	offset = max(0, min(offset, len(m.text)))
	line := sort.Search(len(m.starts), func(i int) bool { return m.starts[i] > offset }) - 1
	units := 0
	for i := m.starts[line]; i < offset && m.text[i] != '\r' && m.text[i] != '\n'; {
		r, n := utf8.DecodeRuneInString(m.text[i:])
		if i+n > offset {
			break
		}
		units++
		if r > 0xffff {
			units++
		}
		i += n
	}
	return lsproto.Position{Line: line, Character: units}
}

func (m *LineMap) Range(r core.TextRange) lsproto.Range {
	return lsproto.Range{Start: m.Position(r.Pos()), End: m.Position(r.End())}
}

func FileNameToURI(name string) lsproto.DocumentURI {
	name = filepath.ToSlash(name)
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	return lsproto.DocumentURI((&url.URL{Scheme: "file", Path: name}).String())
}

func URIToFileName(uri lsproto.DocumentURI) (string, error) {
	u, err := url.Parse(string(uri))
	if err != nil || u.Scheme != "file" || u.RawQuery != "" || u.Fragment != "" || (u.Host != "" && u.Host != "localhost") || !strings.HasPrefix(u.Path, "/") {
		return "", fmt.Errorf("expected an absolute local file URI: %q", uri)
	}
	name := u.Path
	if runtime.GOOS == "windows" {
		name = strings.TrimPrefix(name, "/")
	}
	name = filepath.FromSlash(name)
	if !filepath.IsAbs(name) {
		return "", fmt.Errorf("file URI has no absolute native path: %q", uri)
	}
	return filepath.Clean(name), nil
}

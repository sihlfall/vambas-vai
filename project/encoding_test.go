package project

import (
	"bytes"
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unicode/utf16"
)

func utf16Bytes(text string, order binary.AppendByteOrder) []byte {
	data := order.AppendUint16(nil, 0xfeff)
	for _, unit := range utf16.Encode([]rune(text)) {
		data = order.AppendUint16(data, unit)
	}
	return data
}

func TestDecodeSource(t *testing.T) {
	for _, test := range []struct {
		name           string
		data           []byte
		encoding, want string
	}{
		{"ASCII", []byte("Option Explicit\r\n"), "auto", "Option Explicit\r\n"},
		{"UTF8", []byte("ÄäÖöÜüß 😀"), "auto", "ÄäÖöÜüß 😀"},
		{"UTF8BOM", []byte("\ufeffÄ😀"), "auto", "Ä😀"},
		{"Windows1252", []byte{0xc4, 0xe4, 0xd6, 0xf6, 0xdc, 0xfc, 0xdf}, "auto", "ÄäÖöÜüß"},
		{"Windows1252Punctuation", []byte{0x80, 0x91, 0x92, 0x93, 0x94, 0x96, 0x97, 0x85, 0x99}, "auto", "€‘’“”–—…™"},
		{"Undefined1252", []byte{0x81, 0x8d, 0x8f, 0x90, 0x9d}, "auto", "�����"},
		{"UTF16LE", utf16Bytes("Ä😀\r\nSub Run()", binary.LittleEndian), "auto", "Ä😀\r\nSub Run()"},
		{"UTF16BE", utf16Bytes("Ä😀\r\nSub Run()", binary.BigEndian), "auto", "Ä😀\r\nSub Run()"},
		{"PreferUTF8", []byte{0xc3, 0xa4}, "auto", "ä"},
		{"Forced1252", []byte{0xc3, 0xa4}, "windows1252", "Ã¤"},
		{"BOMOverridesFallback", []byte("\ufeffä"), "windows1252", "ä"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := decodeSource(test.data, test.encoding)
			if err != nil || got != test.want {
				t.Fatalf("got %q, %v; want %q", got, err, test.want)
			}
		})
	}
	for _, test := range []struct {
		data     []byte
		encoding string
	}{
		{[]byte{0xff, 0xfe, 0x61}, "auto"},
		{[]byte{0xfe, 0xff, 0x61}, "auto"},
		{[]byte{0xef, 0xbb, 0xbf, 0xe4}, "auto"},
		{[]byte{0xe4}, "utf8"},
	} {
		if _, err := decodeSource(test.data, test.encoding); err == nil {
			t.Fatal("malformed Unicode accepted", test)
		}
	}
}

func TestLoadLegacyManifestAndSourceWithoutRewriting(t *testing.T) {
	root := t.TempDir()
	name := filepath.Join(root, "Modül.bas")
	data := []byte("Attribute VB_Name = \"Main\"\r\nPublic Const Greeting As String = \"Gr\xfc\xdfe \x80\"\r\n")
	manifest := []byte("Name=\"Demo\"\r\nModule=Main; Mod\xfcl.bas\r\n")
	if err := os.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "Demo.vbp"), manifest, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(context.Background(), root, "")
	if err != nil || len(cfg.Sources) != 1 {
		t.Fatal(cfg, err)
	}
	if cfg.SourceEncoding != "auto" || cfg.Sources[0].FileName != name || cfg.Sources[0].Text != "Attribute VB_Name = \"Main\"\r\nPublic Const Greeting As String = \"Grüße €\"\r\n" {
		t.Fatal(cfg)
	}
	p, err := NewSession(cfg).Snapshot().Program(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ParseDiagnostics()) != 0 || len(p.BindDiagnostics()) != 0 || len(p.GetSemanticDiagnostics(nil)) != 0 {
		t.Fatal("decoded source did not compile")
	}
	got, err := os.ReadFile(name)
	if err != nil || !bytes.Equal(got, data) {
		t.Fatal("loader rewrote source", err)
	}
	if _, err := LoadConfigWithOptions(context.Background(), LoadOptions{Root: root, SourceEncoding: "utf8"}); err == nil {
		t.Fatal("strict UTF8 accepted ANSI manifest")
	}
	if _, err := LoadConfigWithOptions(context.Background(), LoadOptions{Root: root, SourceEncoding: "not-an-encoding"}); err == nil {
		t.Fatal("unknown encoding silently accepted")
	}
}

func TestUnicodeBOMsExcludedFromDocumentPositions(t *testing.T) {
	for _, data := range [][]byte{[]byte("\ufeffDim value As String\n"), utf16Bytes("Dim value As String\n", binary.LittleEndian), utf16Bytes("Dim value As String\n", binary.BigEndian)} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "Main.bas"), data, 0600); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(context.Background(), root, "")
		if err != nil || cfg.Sources[0].Text != "Dim value As String\n" {
			t.Fatal(cfg, err)
		}
	}
}

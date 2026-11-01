package project

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func sourceEncoding(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "auto":
		return "auto", nil
	case "utf8", "utf-8":
		return "utf8", nil
	case "windows1252", "windows-1252", "cp1252":
		return "windows1252", nil
	default:
		return "", fmt.Errorf("unsupported source encoding %q; use auto, utf8, or windows1252", name)
	}
}

// decodeSource is the disk boundary: compiler inputs and LSP overlays are always
// Unicode represented as UTF-8, never legacy byte offsets. As in typescript-go's
// vfs reader, BOMs select UTF-8/UTF-16 and are excluded from editor text. VBA also
// needs a legacy fallback, since exported VB6/VBA modules commonly use an ANSI
// code page. Auto prefers valid UTF-8 and otherwise assumes Windows-1252; this is
// a documented default, not reliable detection of arbitrary Windows code pages.
func decodeSource(data []byte, encoding string) (string, error) {
	if bytes.HasPrefix(data, []byte{0xff, 0xfe}) {
		return decodeUTF16(data[2:], binary.LittleEndian)
	}
	if bytes.HasPrefix(data, []byte{0xfe, 0xff}) {
		return decodeUTF16(data[2:], binary.BigEndian)
	}
	if bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}) {
		data = data[3:]
		if !utf8.Valid(data) {
			return "", fmt.Errorf("invalid UTF-8 after byte order mark")
		}
		return string(data), nil
	}
	if encoding != "windows1252" && utf8.Valid(data) {
		return string(data), nil
	}
	if encoding == "utf8" {
		return "", fmt.Errorf("invalid UTF-8; use sourceEncoding auto or windows1252 for legacy exports")
	}
	var result strings.Builder
	result.Grow(len(data))
	for _, b := range data {
		r := rune(b)
		if b >= 0x80 && b < 0xa0 {
			r = windows1252[b-0x80]
		}
		result.WriteRune(r)
	}
	return result.String(), nil
}

// Microsoft mapping: https://www.unicode.org/Public/MAPPINGS/VENDORS/MICSFT/WINDOWS/CP1252.TXT
// Undefined entries decode to U+FFFD, rather than dropping a character.
var windows1252 = [...]rune{
	'\u20ac', '\ufffd', '\u201a', '\u0192', '\u201e', '\u2026', '\u2020', '\u2021',
	'\u02c6', '\u2030', '\u0160', '\u2039', '\u0152', '\ufffd', '\u017d', '\ufffd',
	'\ufffd', '\u2018', '\u2019', '\u201c', '\u201d', '\u2022', '\u2013', '\u2014',
	'\u02dc', '\u2122', '\u0161', '\u203a', '\u0153', '\ufffd', '\u017e', '\u0178',
}

func decodeUTF16(data []byte, order binary.ByteOrder) (string, error) {
	if len(data)%2 != 0 {
		return "", fmt.Errorf("truncated UTF-16 source")
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = order.Uint16(data[i*2:])
	}
	return string(utf16.Decode(units)), nil
}

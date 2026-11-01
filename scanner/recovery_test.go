package scanner

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
)

func TestTokenPositionsAndTrivia(t *testing.T) {
	input := "  Let x = _ \t\r\n  2 ' a comment\r\nRem another comment\r\n"
	s := ScannerState{Input: input}
	for i, want := range []struct {
		kind ast.Kind
		text string
	}{
		{ast.KindLET, "Let"}, {ast.KindIdentifier, "x"}, {ast.KindEQ, "="},
		{ast.KindNumericLiteral, "2"}, {ast.KindNEWLINE, "\r\n"},
		{ast.KindNEWLINE, "\r\n"}, {ast.KindEndOfFile, ""},
	} {
		fullStart := s.TokenEnd()
		if got := s.Scan(ScanFlagNone); got != want.kind {
			t.Fatalf("token %d = %d, want %d", i, got, want.kind)
		}
		if s.TokenFullStart() != fullStart || s.TokenStart() < fullStart || s.TokenEnd() < s.TokenStart() {
			t.Fatalf("invalid token positions at token %d", i)
		}
		if got := input[s.TokenStart():s.TokenEnd()]; got != want.text {
			t.Errorf("token %d text = %q, want %q", i, got, want.text)
		}
		if i == 3 && input[s.TokenFullStart():s.TokenStart()] != " _ \t\r\n  " {
			t.Error("continuation was not leading trivia")
		}
	}
}

func TestUnterminatedStringPreservesTextAndResetsFlags(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n", "\r", "\u2028", "\u2029", ""} {
		t.Run(ending, func(t *testing.T) {
			input := "\"partial \"\"quote" + ending
			if ending != "" {
				input += "\"valid\""
			}
			s := ScannerState{Input: input}
			count := 0
			s.SetOnError(func(code ast.DiagnosticCode, pos, length int, message string) {
				count++
				if code != ast.DiagnosticUnterminatedString || pos != 0 || length != len("\"partial \"\"quote") || message == "" {
					t.Error("invalid lexical diagnostic")
				}
			})
			if s.Scan(ScanFlagNone) != ast.KindStringLiteral || s.TokenValue != "partial \"quote" || s.TokenFlags()&ast.TokenFlagsUnterminated == 0 || count != 1 {
				t.Fatal("unterminated string did not preserve its text, flags, and diagnostic")
			}
			if ending != "" {
				if s.Scan(ScanFlagNone) != ast.KindNEWLINE || s.TokenFlags() != ast.TokenFlagsNone || s.TokenValue != "" {
					t.Fatal("newline or token state was lost")
				}
				if s.Scan(ScanFlagNone) != ast.KindStringLiteral || s.TokenFlags() != ast.TokenFlagsNone || s.TokenValue != "valid" {
					t.Fatal("unterminated flag leaked to the next literal")
				}
			}
		})
	}
}

func TestUnknownCharactersAdvanceByUTF8Rune(t *testing.T) {
	s := ScannerState{Input: "?é\xff~"}
	var positions []int
	s.SetOnError(func(code ast.DiagnosticCode, pos, length int, message string) {
		if code != ast.DiagnosticInvalidCharacter || length <= 0 {
			t.Error("invalid diagnostic")
		}
		positions = append(positions, pos)
	})
	for i, width := range []int{1, 2, 1, 1} {
		before := s.TokenEnd()
		if s.Scan(ScanFlagNone) != ast.KindUnknown || s.TokenEnd()-before != width {
			t.Fatalf("token %d did not advance by %d bytes", i, width)
		}
	}
	if s.Scan(ScanFlagNone) != ast.KindEndOfFile || len(positions) != 4 {
		t.Fatal("unknown input did not reach EOF with diagnostics")
	}
}

func TestCommentsAndContinuationModes(t *testing.T) {
	for _, flags := range []ScanFlags{ScanFlagNone, ScanFlagExplicitWhitespace, ScanFlagExplicitLineContinuation} {
		for _, ending := range []string{"\n", "\r\n"} {
			s := ScannerState{Input: "Let _ \t" + ending + "  x = 1"}
			if s.Scan(flags) != ast.KindLET {
				t.Fatal("missing Let")
			}
			if flags != ScanFlagNone {
				want := ast.KindWS
				if flags == ScanFlagExplicitLineContinuation {
					want = ast.KindLineContinuation
				}
				if s.Scan(flags) != want {
					t.Fatal("explicit trivia mode did not report continuation")
				}
			}
			if s.Scan(flags) != ast.KindIdentifier || s.TokenValue != "x" {
				t.Fatal("continuation lost the next identifier")
			}
		}
	}
	for _, input := range []string{"' comment ? \" _", "Rem comment ? \" _", "rEM: End Function"} {
		s := ScannerState{Input: input}
		if s.Scan(ScanFlagNone) != ast.KindEndOfFile {
			t.Fatalf("comment %q did not end at EOF", input)
		}
	}
	s := ScannerState{Input: "Remember"}
	if s.Scan(ScanFlagNone) != ast.KindIdentifier {
		t.Fatal("Rem prefix consumed an identifier as a comment")
	}
}

func FuzzScannerProgress(f *testing.F) {
	for _, source := range []string{"", "?é\xff", "\"unterminated", "' comment", "x _ \t\r\n  + 1", "\ufeffFunction F()\nEnd Function", strings.Repeat("_", 100), "Const Pi = 3.14, Mask = &HFF, Tiny = 1d-2, Broken = &O, Exponent = 1e+"} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 4096 {
			t.Skip()
		}
		s := ScannerState{Input: source}
		s.SetOnError(func(_ ast.DiagnosticCode, pos, length int, _ string) {
			if pos < 0 || length < 0 || pos+length > len(source) {
				t.Fatal("diagnostic outside source")
			}
		})
		for i := 0; i <= len(source); i++ {
			before := s.TokenEnd()
			kind := s.Scan(ScanFlagNone)
			if s.TokenFullStart() != before || s.TokenStart() < before || s.TokenStart() > s.TokenEnd() || s.TokenEnd() > len(source) {
				t.Fatal("invalid token range")
			}
			if kind == ast.KindEndOfFile {
				if s.TokenEnd() != len(source) {
					t.Fatal("premature EOF")
				}
				return
			}
			if s.TokenEnd() <= before {
				t.Fatal("scanner failed to advance")
			}
		}
		t.Fatal("scanner did not terminate")
	})
}

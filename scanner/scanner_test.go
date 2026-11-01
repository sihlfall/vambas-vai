package scanner

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
)

func TestScan001(t *testing.T) {
	input := "If"
	want := ast.KindIF

	state := ScannerState{Input: input}
	res := state.Scan(ScanFlagExplicitWhitespace)
	if want != res {
		t.Errorf(`scan("%s") = %d, but want %d`, input, res, want)
	}
}

func TestScan002(t *testing.T) {
	input := "something"
	want := ast.KindIdentifier

	state := ScannerState{Input: input}
	res := state.Scan(ScanFlagExplicitWhitespace)
	if want != res {
		t.Errorf(`scan("%s") = %d, but want %d`, input, res, want)
	}
}

func TestScan003(t *testing.T) {
	input := "\"Some string literal with a \"\" quote\""
	want := ast.KindStringLiteral
	wantValue := "Some string literal with a \" quote"

	state := ScannerState{Input: input}
	res := state.Scan(ScanFlagExplicitWhitespace)
	if want != res {
		t.Errorf(`scan("%s") = %d, but want %d`, input, res, want)
	}
	if wantValue != state.TokenValue {
		t.Errorf(`tokenValue = %s, want %s`, state.TokenValue, wantValue)
	}
}

func TestKeywordPrefixesInIdentifiers(t *testing.T) {
	for _, input := range []string{
		"IfElse", "EndFunction", "FunctionValue", "IntegerValue", "OrElse",
		"Let1", "If_", "If$", "ifValue", "ENDValue",
	} {
		t.Run(input, func(t *testing.T) {
			state := ScannerState{Input: input}
			if got := state.Scan(ScanFlagNone); got != ast.KindIdentifier {
				t.Fatalf("token kind = %d, want identifier", got)
			}
			if state.TokenValue != input {
				t.Errorf("identifier = %q, want %q", state.TokenValue, input)
			}
			if got := state.Scan(ScanFlagNone); got != ast.KindEndOfFile {
				t.Errorf("trailing token = %d, want EOF", got)
			}
		})
	}
}

func TestKeywordBoundaries(t *testing.T) {
	state := ScannerState{Input: "iF (x) Then\nEnd Function"}
	for i, want := range []ast.Kind{
		ast.KindIF, ast.KindLPAREN, ast.KindIdentifier, ast.KindRPAREN,
		ast.KindTHEN, ast.KindNEWLINE, ast.KindEND, ast.KindFUNCTION, ast.KindEndOfFile,
	} {
		if got := state.Scan(ScanFlagNone); got != want {
			t.Fatalf("token %d = %d, want %d", i, got, want)
		}
	}
}

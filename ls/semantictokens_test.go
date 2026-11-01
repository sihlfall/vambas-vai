package ls_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ls"
	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
	"sihlfall.ch/vambas-vai/project"
)

type decodedToken struct {
	text, kind string
	start      lsproto.Position
	modifiers  []string
}

func decodeTokens(t *testing.T, text string, result *lsproto.SemanticTokens, legend lsproto.SemanticTokensLegend) []decodedToken {
	t.Helper()
	if result == nil || result.Data == nil || len(result.Data)%5 != 0 {
		t.Fatalf("invalid token response: %+v", result)
	}
	var decoded []decodedToken
	lineMap := lsconv.NewLineMap(text)
	position, previousEnd := lsproto.Position{}, lsproto.Position{}
	for i := 0; i < len(result.Data); i += 5 {
		data := result.Data[i : i+5]
		if data[0] != 0 {
			position.Character = 0
		}
		position.Line += int(data[0])
		position.Character += int(data[1])
		end := lsproto.Position{Line: position.Line, Character: position.Character + int(data[2])}
		if data[2] == 0 || int(data[3]) >= len(legend.TokenTypes) || position.Line == previousEnd.Line && position.Character < previousEnd.Character {
			t.Fatalf("invalid/overlapping token: %v at %+v", data, position)
		}
		startByte, err1 := lineMap.Offset(position)
		endByte, err2 := lineMap.Offset(end)
		if err1 != nil || err2 != nil || lineMap.Position(startByte) != position || lineMap.Position(endByte) != end {
			t.Fatalf("invalid UTF-16 token range: %+v..%+v", position, end)
		}
		token := decodedToken{text: text[startByte:endByte], kind: legend.TokenTypes[data[3]], start: position}
		for bit, name := range legend.TokenModifiers {
			if data[4]&(1<<bit) != 0 {
				token.modifiers = append(token.modifiers, name)
			}
		}
		decoded = append(decoded, token)
		previousEnd = end
	}
	return decoded
}
func requireToken(t *testing.T, tokens []decodedToken, text, needle string, inside int, kind string, modifiers ...string) {
	t.Helper()
	position := lsconv.NewLineMap(text).Position(strings.Index(text, needle) + inside)
	for _, token := range tokens {
		if token.start == position {
			if token.kind != kind || !reflect.DeepEqual(token.modifiers, modifiers) {
				t.Fatalf("%q: %+v, want %s %v", needle, token, kind, modifiers)
			}
			return
		}
	}
	t.Fatalf("no token at %q %+v: %+v", needle, position, tokens)
}
func TestSemanticTokensUseResolvedDeclarations(t *testing.T) {
	const code = `Attribute VB_Name = "Main"
Option Explicit
Public Const Limit As Long = 2
Public total As Long
Sub Run(ByVal count As Long, ByVal target As Counter)
 Dim parts As New Collection
 parts.Add Item:="Collection Add Limit"
 total = Utilities.Add(amount:=count)
 target.Value = total
 total = parts.Count
 Dim record As Shapes.Point
 record.X = Color.Blue
 total = Err.Number
 VBA.Err.Clear
 GoTo done
done:
End Sub
Function Echo(ByVal arg As Long) As Long
 Echo = arg
End Function
Sub Shadows()
 Dim Len As Long
 Len = 1
End Sub
`
	const shapes = "Public Type Point\n X As Long\nEnd Type\nPublic Enum Color\n Red = 1\n Blue\nEnd Enum\n"
	root := t.TempDir()
	main, shape := filepath.Join(root, "Main.bas"), filepath.Join(root, "Shapes.bas")
	session := project.NewSession(project.Config{Name: "Demo", Sources: []compiler.SourceFileInput{
		{FileName: main, Text: code}, {FileName: shape, Text: shapes},
		{FileName: filepath.Join(root, "Utilities.bas"), Text: utilitiesCode},
		{FileName: filepath.Join(root, "Counter.cls"), Text: classCode},
	}})
	service := ls.NewLanguageService(session.Snapshot())
	uri, legend := lsconv.FileNameToURI(main), ls.SemanticTokensLegend()
	if ds, err := service.GetDiagnostics(context.Background(), uri); err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	result, err := service.ProvideSemanticTokens(context.Background(), uri, legend)
	if err != nil {
		t.Fatal(err)
	}
	tokens := decodeTokens(t, code, result, legend)
	for _, test := range []struct {
		needle string
		inside int
		kind   string
		mods   []string
	}{
		{"Limit As", 0, "variable", []string{"declaration", "readonly"}},
		{"total As", 0, "variable", []string{"declaration"}},
		{"Sub Run", 4, "function", []string{"declaration"}},
		{"count As", 0, "parameter", []string{"declaration"}},
		{"As Counter", 3, "class", nil},
		{"parts As", 0, "variable", []string{"declaration"}},
		{"New Collection", 4, "class", []string{"defaultLibrary"}},
		{"parts.Add", 6, "method", []string{"defaultLibrary"}},
		{"Item:=", 0, "parameter", []string{"defaultLibrary"}},
		{"Utilities.Add", 0, "namespace", nil},
		{"Utilities.Add", 10, "function", nil},
		{"amount:=", 0, "parameter", nil},
		{"amount:=count", 8, "parameter", nil},
		{"target.Value", 7, "property", nil},
		{"parts.Count", 6, "property", []string{"readonly", "defaultLibrary"}},
		{"Shapes.Point", 0, "namespace", nil},
		{"Shapes.Point", 7, "struct", nil},
		{"record.X", 7, "property", nil},
		{"Color.Blue", 0, "enum", nil},
		{"Color.Blue", 6, "enumMember", []string{"readonly"}},
		{"Err.Number", 0, "namespace", []string{"defaultLibrary"}},
		{"Err.Number", 4, "variable", []string{"defaultLibrary"}},
		{"VBA.Err.Clear", 4, "namespace", []string{"defaultLibrary"}},
		{"VBA.Err.Clear", 8, "function", []string{"defaultLibrary"}},
		{"GoTo done", 5, "label", nil},
		{"done:", 0, "label", []string{"declaration"}},
		{"Function Echo", 9, "function", []string{"declaration"}},
		{" Echo =", 1, "variable", nil},
		{"Dim Len", 4, "variable", []string{"declaration"}},
		{" Len =", 1, "variable", nil},
	} {
		requireToken(t, tokens, code, test.needle, test.inside, test.kind, test.mods...)
	}
	for _, token := range tokens {
		if token.start.Line == 0 || token.start.Line == 6 && token.start.Character > strings.Index(strings.Split(code, "\n")[6], `"`) {
			t.Fatal("metadata/string classified as a reference", token)
		}
	}
	shapeTokens, err := service.ProvideSemanticTokens(context.Background(), lsconv.FileNameToURI(shape), legend)
	if err != nil {
		t.Fatal(err)
	}
	decoded := decodeTokens(t, shapes, shapeTokens, legend)
	requireToken(t, decoded, shapes, "Type Point", 5, "struct", "declaration")
	requireToken(t, decoded, shapes, "X As", 0, "property", "declaration")
	requireToken(t, decoded, shapes, "Enum Color", 5, "enum", "declaration")
	requireToken(t, decoded, shapes, "Red =", 0, "enumMember", "declaration", "readonly")
}
func TestSemanticTokensUnicodeRangesAndRecovery(t *testing.T) {
	text := "Option Explicit\r\nSub Run()\r\n Dim value$\r\n value$ = \"😀\": value$ = value$\r\n GoTo 100\r\n100 value$ = \"x\"\r\n ' value$ 😀\r\n Rem value$\r\nEnd Sub\r\nSub Broken(\r\n"
	name := filepath.Join(t.TempDir(), "Main.bas")
	s := project.NewSession(project.Config{Sources: []compiler.SourceFileInput{{FileName: name, Text: text}}})
	service, uri, legend := ls.NewLanguageService(s.Snapshot()), lsconv.FileNameToURI(name), ls.SemanticTokensLegend()
	full, err := service.ProvideSemanticTokens(context.Background(), uri, legend)
	if err != nil {
		t.Fatal(err)
	}
	tokens := decodeTokens(t, text, full, legend)
	requireToken(t, tokens, text, ": value$", 2, "variable")
	requireToken(t, tokens, text, "GoTo 100", 5, "label")
	requireToken(t, tokens, text, "100 value$", 0, "label", "declaration")
	for _, token := range tokens {
		if token.start.Line == 6 || token.start.Line == 7 {
			t.Fatal("comment received a semantic token", token)
		}
	}
	rng := lsproto.Range{Start: lsproto.Position{Line: 3, Character: 17}, End: lsproto.Position{Line: 3, Character: 32}}
	part, err := service.ProvideSemanticTokensRange(context.Background(), uri, rng, legend)
	if err != nil {
		t.Fatal(err)
	}
	decoded := decodeTokens(t, text, part, legend)
	if len(decoded) != 2 || decoded[0].start.Line != 3 || decoded[0].start.Character != 16 || decoded[1].start.Character != 25 {
		t.Fatal("range positions are not document-relative UTF-16 positions", decoded)
	}
	empty, err := service.ProvideSemanticTokensRange(context.Background(), uri, lsproto.Range{Start: rng.Start, End: rng.Start}, legend)
	if err != nil || len(empty.Data) != 0 {
		t.Fatal(empty, err)
	}
	for _, bad := range []lsproto.Range{
		{Start: rng.End, End: rng.Start},
		{Start: lsproto.Position{Line: -1}, End: rng.End},
		{Start: lsproto.Position{Line: 3, Character: 12}, End: rng.End},
	} {
		if _, err := service.ProvideSemanticTokensRange(context.Background(), uri, bad, legend); err == nil {
			t.Fatal("invalid range accepted", bad)
		}
	}
}
func TestSemanticTokensSnapshotCancellationAndLegend(t *testing.T) {
	s, main, utilities, _ := fixture(t)
	old := ls.NewLanguageService(s.Snapshot())
	legend := ls.SemanticTokensLegend()
	before, err := old.ProvideSemanticTokens(context.Background(), main, legend)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Open(utilities, 1, strings.ReplaceAll(utilitiesCode, "Add", "Plus")); err != nil {
		t.Fatal(err)
	}
	current := ls.NewLanguageService(s.Snapshot())
	after, err := current.ProvideSemanticTokens(context.Background(), main, legend)
	if err != nil || reflect.DeepEqual(before, after) {
		t.Fatal("new snapshot kept a stale cross-file reference", err)
	}
	again, err := old.ProvideSemanticTokens(context.Background(), main, legend)
	if err != nil || !reflect.DeepEqual(before, again) {
		t.Fatal("captured snapshot changed", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := old.ProvideSemanticTokens(ctx, main, legend); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := old.ProvideSemanticTokens(context.Background(), "file:///outside.bas", legend); err == nil {
		t.Fatal("unknown document accepted")
	}
	filtered := lsproto.SemanticTokensLegend{TokenTypes: []string{"parameter"}, TokenModifiers: []string{"declaration"}}
	result, err := old.ProvideSemanticTokens(context.Background(), main, filtered)
	if err != nil {
		t.Fatal(err)
	}
	for _, token := range decodeTokens(t, mainCode, result, filtered) {
		if token.kind != "parameter" {
			t.Fatal(token)
		}
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := old.ProvideSemanticTokens(context.Background(), main, legend)
			if err != nil || !reflect.DeepEqual(before, got) {
				t.Error("unstable parallel semantic tokens", err)
			}
		}()
	}
	wg.Wait()
}

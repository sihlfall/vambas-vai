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

const completionShapes = `Public Type Point
 Y As Long
 X As Double
End Type
Public Type Context
 Position As Point
 Points() As Point
 Caption As String
 Values(1 To 2, 1 To 3) As Long
End Type
Public Type Escaped
 [Display Name] As String
 [If] As Long
End Type
`
const completionPrefix = `Option Explicit
Sub Run(ByRef p As Shapes.Point, ByRef state As Shapes.Context)
 Dim points() As Shapes.Point
 Dim index As Long, scalar As Long, late As Object, dynamic As Variant
 Dim escaped As Shapes.Escaped
`
const completionSuffix = `
End Sub
Function MakePoint() As Shapes.Point
End Function
`

func completionFixture(t *testing.T, fragment string) (*project.Session, lsproto.TextDocumentPositionParams, string, lsproto.DocumentURI) {
	t.Helper()
	root := t.TempDir()
	main, shapes := filepath.Join(root, "Main.bas"), filepath.Join(root, "Shapes.bas")
	marked := completionPrefix + fragment + completionSuffix
	offset := strings.Index(marked, "|")
	if offset < 0 {
		t.Fatal("fixture has no cursor")
	}
	text := marked[:offset] + marked[offset+1:]
	s := project.NewSession(project.Config{Name: "Demo", Sources: []compiler.SourceFileInput{{FileName: main, Text: text}, {FileName: shapes, Text: completionShapes}}})
	params := lsproto.TextDocumentPositionParams{TextDocument: lsproto.TextDocumentIdentifier{URI: lsconv.FileNameToURI(main)}, Position: lsconv.NewLineMap(text).Position(offset)}
	return s, params, text, lsconv.FileNameToURI(shapes)
}
func completionLabels(result *lsproto.CompletionList) []string {
	labels := []string{}
	for _, item := range result.Items {
		labels = append(labels, item.Label)
	}
	return labels
}
func TestUDTMemberCompletions(t *testing.T) {
	point := []string{"X", "Y"}
	state := []string{"Caption", "Points", "Position", "Values"}
	for _, test := range []struct {
		fragment string
		want     []string
	}{
		{" p.|", point},
		{" p.x|", point},
		{" p.X|wrong", point},
		{" state.Position.|", point},
		{" points(index).|", point},
		{" state.Points(index).|", point},
		{" scalar = (p).|", point},
		{" MakePoint().|", point},
		{" scalar = MakePoint.|", point},
		{" scalar = p .|", point},
		{" p. |", point},
		{" p. _\n |", point},
		{" p. _\n X|", point},
		{" p. _\r\n X|", point},
		{" p.| ' comment", point},
		{" With p\n .|\n End With", point},
		{" With state\n .Position.|\n End With", point},
		{" With state\n With .Position\n .|\n End With\n End With", point},
		{" With state\n .|\n End With", state},
		{" state.|", state},
	} {
		t.Run(test.fragment, func(t *testing.T) {
			s, params, _, _ := completionFixture(t, test.fragment)
			result, err := ls.NewLanguageService(s.Snapshot()).ProvideCompletion(context.Background(), params)
			if err != nil || result == nil {
				t.Fatal(result, err)
			}
			if result.IsIncomplete || !reflect.DeepEqual(completionLabels(result), test.want) {
				t.Fatal(result)
			}
			for _, item := range result.Items {
				if item.Kind != lsproto.CompletionItemKindField || item.FilterText != item.Label || item.SortText != strings.ToLower(item.Label) {
					t.Fatal(item)
				}
			}
		})
	}
}
func TestUDTCompletionEditsAndDetails(t *testing.T) {
	for _, test := range []struct{ fragment, oldName string }{
		{" p.|", ""}, {" p.x|", "x"}, {" p.X|wrong", "Xwrong"}, {" p.|X", "X"}, {" p. |X", "X"},
		{" p.X| + p.Y", "X"}, {" scalar = \"😀\": p.x|", "x"},
	} {
		t.Run(test.fragment, func(t *testing.T) {
			s, params, text, _ := completionFixture(t, test.fragment)
			result, err := ls.NewLanguageService(s.Snapshot()).ProvideCompletion(context.Background(), params)
			if err != nil || len(result.Items) != 2 {
				t.Fatal(result, err)
			}
			x := result.Items[0]
			if x.Detail != "X As Double" || result.Items[1].Detail != "Y As Long" || x.TextEdit.NewText != "X" {
				t.Fatal(result.Items)
			}
			lines := lsconv.NewLineMap(text)
			start, e1 := lines.Offset(x.TextEdit.Range.Start)
			end, e2 := lines.Offset(x.TextEdit.Range.End)
			cursor, _ := lines.Offset(params.Position)
			if e1 != nil || e2 != nil || text[start:end] != test.oldName || start > cursor || end < cursor {
				t.Fatal(x.TextEdit, e1, e2)
			}
			if x.TextEdit.Range.Start.Line != params.Position.Line || x.TextEdit.Range.End.Line != params.Position.Line {
				t.Fatal("multiline completion edit", x)
			}
			if test.oldName == "" && (x.TextEdit.Range.Start != params.Position || x.TextEdit.Range.End != params.Position) {
				t.Fatal("empty name did not insert at cursor", x)
			}
		})
	}
	s, params, _, _ := completionFixture(t, " state.|")
	result, err := ls.NewLanguageService(s.Snapshot()).ProvideCompletion(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	details := []string{}
	for _, item := range result.Items {
		details = append(details, item.Detail)
	}
	if !reflect.DeepEqual(details, []string{"Caption As String", "Points() As Point", "Position As Point", "Values(,) As Long"}) {
		t.Fatal(details)
	}
}
func TestUDTCompletionEscapedNames(t *testing.T) {
	for _, fragment := range []string{" escaped.|", " escaped.[Di|splay Name]", " escaped.[|", " escaped.[I|f]"} {
		t.Run(fragment, func(t *testing.T) {
			s, params, text, _ := completionFixture(t, fragment)
			result, err := ls.NewLanguageService(s.Snapshot()).ProvideCompletion(context.Background(), params)
			if err != nil || !reflect.DeepEqual(completionLabels(result), []string{"Display Name", "If"}) {
				t.Fatal(result, err)
			}
			for _, item := range result.Items {
				if item.TextEdit.NewText != "["+item.Label+"]" {
					t.Fatal(item)
				}
				if strings.Contains(fragment, "[") && item.FilterText != item.TextEdit.NewText {
					t.Fatal("bracket prefix cannot match filter text", item)
				}
				lines := lsconv.NewLineMap(text)
				start, _ := lines.Offset(item.TextEdit.Range.Start)
				end, _ := lines.Offset(item.TextEdit.Range.End)
				edited := text[:start] + item.TextEdit.NewText + text[end:]
				if !strings.Contains(edited, "escaped."+item.TextEdit.NewText) {
					t.Fatal("escaped member duplicated brackets", edited)
				}
			}
		})
	}
}
func TestUDTCompletionsExcludeOtherContexts(t *testing.T) {
	for _, fragment := range []string{
		" ' p.|", " Rem p.|", ` scalar = "p.|"`, " p.X ' p.|", " p.\n ' |",
		" scalar = .5|", " scalar.|", " late.|", " dynamic.|", " missing.|", " points.|",
		" state.Values.|", " state.Values(1, 1).|", " state.Caption.|", " Shapes.|", " Shapes.Point.|",
		" p!|", " .|", " With p\n End With\n .|",
		" p.| _\n X", // Completing here would require a multiline replacement.
	} {
		t.Run(fragment, func(t *testing.T) {
			s, params, _, _ := completionFixture(t, fragment)
			result, err := ls.NewLanguageService(s.Snapshot()).ProvideCompletion(context.Background(), params)
			if err != nil || result.Items == nil || len(result.Items) != 0 {
				t.Fatal(result, err)
			}
		})
	}
}
func TestUDTCompletionSnapshotsCancellationAndPositions(t *testing.T) {
	s, params, _, shapes := completionFixture(t, " p.|")
	old := ls.NewLanguageService(s.Snapshot())
	before, err := old.ProvideCompletion(context.Background(), params)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Open(shapes, 1, strings.Replace(completionShapes, "Y As Long", "Z As String", 1)); err != nil {
		t.Fatal(err)
	}
	current := ls.NewLanguageService(s.Snapshot())
	after, err := current.ProvideCompletion(context.Background(), params)
	if err != nil || !reflect.DeepEqual(completionLabels(after), []string{"X", "Z"}) {
		t.Fatal(after, err)
	}
	again, err := old.ProvideCompletion(context.Background(), params)
	if err != nil || !reflect.DeepEqual(before, again) {
		t.Fatal("captured snapshot changed", again, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := old.ProvideCompletion(context.Background(), params)
			if err != nil || !reflect.DeepEqual(before, got) {
				t.Error("parallel completion changed", err)
			}
		}()
	}
	wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := old.ProvideCompletion(ctx, params); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	bad := params
	bad.Position.Line = -1
	if _, err := old.ProvideCompletion(context.Background(), bad); err == nil {
		t.Fatal("negative position accepted")
	}
	bad = params
	bad.TextDocument.URI = "file:///outside.bas"
	if _, err := old.ProvideCompletion(context.Background(), bad); err == nil {
		t.Fatal("unknown document accepted")
	}
}

func TestUDTCompletionAtEOFAndUnicodeValidation(t *testing.T) {
	s, params, text, _ := completionFixture(t, " scalar = \"😀\": p.|")
	text = strings.TrimSuffix(text, completionSuffix)
	if err := s.Open(params.TextDocument.URI, 1, text); err != nil {
		t.Fatal(err)
	}
	service := ls.NewLanguageService(s.Snapshot())
	result, err := service.ProvideCompletion(context.Background(), params)
	if err != nil || !reflect.DeepEqual(completionLabels(result), []string{"X", "Y"}) {
		t.Fatal("incomplete procedure at EOF lost the receiver", result, err)
	}
	bad := params
	bad.Position = lsconv.NewLineMap(text).Position(strings.Index(text, "😀"))
	bad.Position.Character++
	if _, err := service.ProvideCompletion(context.Background(), bad); err == nil {
		t.Fatal("position inside an emoji surrogate pair was accepted")
	}
}

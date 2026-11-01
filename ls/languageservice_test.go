package ls_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ls"
	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
	"sihlfall.ch/vambas-vai/project"
)

const mainCode = `Option Explicit
' Comment mentions Add and obj.Value 😀
Sub Run(ByVal obj As Counter)
    Dim result As Long
    result = Utilities.Add(amount:=obj.Value)
    obj.Value = result
    GoTo done
done:
End Sub
`
const utilitiesCode = `Public Function Add(ByVal amount As Long, Optional ByVal extra As Long = 2) As Long
    Add = amount + extra
End Function
`
const classCode = `Public Property Get Value() As Long
    Value = 1
End Property
Public Property Let Value(ByVal rhs As Long)
End Property
`

func fixture(t *testing.T) (*project.Session, lsproto.DocumentURI, lsproto.DocumentURI, lsproto.DocumentURI) {
	t.Helper()
	root := t.TempDir()
	main, utilities, class := filepath.Join(root, "Main.bas"), filepath.Join(root, "Utilities.bas"), filepath.Join(root, "Counter.cls")
	s := project.NewSession(project.Config{Name: "Demo", Sources: []compiler.SourceFileInput{{FileName: main, Text: mainCode}, {FileName: utilities, Text: utilitiesCode}, {FileName: class, Text: classCode}}})
	return s, lsconv.FileNameToURI(main), lsconv.FileNameToURI(utilities), lsconv.FileNameToURI(class)
}

func at(uri lsproto.DocumentURI, text, needle string, inside int) lsproto.TextDocumentPositionParams {
	return lsproto.TextDocumentPositionParams{TextDocument: lsproto.TextDocumentIdentifier{URI: uri}, Position: lsconv.NewLineMap(text).Position(strings.Index(text, needle) + inside)}
}

func TestHoverDefinitionAndNamedArguments(t *testing.T) {
	ctx := context.Background()
	s, main, utilities, class := fixture(t)
	service := ls.NewLanguageService(s.Snapshot())
	for _, uri := range []lsproto.DocumentURI{main, utilities, class} {
		if ds, err := service.GetDiagnostics(ctx, uri); err != nil || len(ds) != 0 {
			t.Fatalf("%s: %+v, %v", uri, ds, err)
		}
	}
	for _, test := range []struct {
		needle                 string
		inside                 int
		hover                  string
		target                 lsproto.DocumentURI
		targetText, targetName string
	}{
		{"Utilities.Add", 11, "Function Add(ByVal amount As Long, Optional ByVal extra As Long = 2) As Long", utilities, utilitiesCode, "Add"},
		{"amount:=", 2, "ByVal amount As Long", utilities, utilitiesCode, "amount"},
		{"obj.Value)", 6, "Property Get Value() As Long", class, classCode, "Value"},
		{"obj.Value =", 6, "Property Let Value(ByVal rhs As Long)", class, classCode, "Value(ByVal"},
		{"GoTo done", 7, "done:", main, mainCode, "done:"},
		{"obj.Value)", 1, "ByVal obj As Counter", main, mainCode, "obj As"},
		{"As Counter", 5, "Class Counter", class, classCode, ""},
	} {
		params := at(main, mainCode, test.needle, test.inside)
		hover, err := service.ProvideHover(ctx, params)
		if err != nil || hover == nil || hover.Contents.Value != test.hover {
			t.Errorf("%s hover = %+v, %v; want %q", test.needle, hover, err, test.hover)
		}
		locations, err := service.ProvideDefinition(ctx, params)
		want := lsconv.NewLineMap(test.targetText).Position(max(0, strings.Index(test.targetText, test.targetName)))
		if err != nil || len(locations) != 1 || locations[0].URI != test.target || locations[0].Range.Start != want {
			t.Errorf("%s definition = %+v, %v; want %s %+v", test.needle, locations, err, test.target, want)
		}
	}
	for _, needle := range []string{"Comment mentions", "Add and obj", " 😀", "Dim result"} {
		params := at(main, mainCode, needle, 1)
		if hover, err := service.ProvideHover(ctx, params); err != nil || hover != nil {
			t.Errorf("hover in trivia/keyword: %q, %+v, %v", needle, hover, err)
		}
	}
}

func TestQueriesKeepCapturedSnapshot(t *testing.T) {
	s, main, utilities, _ := fixture(t)
	old := ls.NewLanguageService(s.Snapshot())
	if err := s.Open(utilities, 1, strings.ReplaceAll(utilitiesCode, "Add", "Plus")); err != nil {
		t.Fatal(err)
	}
	current := ls.NewLanguageService(s.Snapshot())
	params := at(main, mainCode, "Utilities.Add", 11)
	if defs, err := old.ProvideDefinition(context.Background(), params); err != nil || len(defs) != 1 {
		t.Fatal(defs, err)
	}
	if defs, err := current.ProvideDefinition(context.Background(), params); err != nil || len(defs) != 0 {
		t.Fatal("new snapshot used stale symbol", defs, err)
	}
	if ds, err := current.GetDiagnostics(context.Background(), main); err != nil || len(ds) == 0 {
		t.Fatal("cross-file edits not reflected", ds, err)
	}
}

func TestDocumentSymbols(t *testing.T) {
	s, main, _, class := fixture(t)
	service := ls.NewLanguageService(s.Snapshot())
	symbols, err := service.ProvideDocumentSymbols(context.Background(), main)
	if err != nil || len(symbols) != 1 || symbols[0].Name != "Run" || len(symbols[0].Children) != 3 {
		t.Fatalf("%+v, %v", symbols, err)
	}
	for i, want := range []string{"obj", "result", "done"} {
		if symbols[0].Children[i].Name != want {
			t.Fatal(symbols[0].Children)
		}
	}
	symbols, err = service.ProvideDocumentSymbols(context.Background(), class)
	if err != nil || len(symbols) != 2 || symbols[0].Name != "Value (Get)" || symbols[1].Name != "Value (Let)" {
		t.Fatal(symbols, err)
	}
	for _, symbol := range symbols {
		if symbol.Range.Start.Line > symbol.SelectionRange.Start.Line || symbol.Range.End.Line < symbol.SelectionRange.End.Line {
			t.Fatal("selection outside symbol range", symbol)
		}
	}
}

func TestAllDiagnosticPhasesAndCancellation(t *testing.T) {
	root := t.TempDir()
	text := "Option Explicit\nSub Run()\n Dim typed As MissingType\n absent = 1\n @\nEnd Sub\n"
	name := filepath.Join(root, "Main.bas")
	snapshot := project.NewSession(project.Config{Name: "Demo", Sources: []compiler.SourceFileInput{{FileName: name, Text: text}}}).Snapshot()
	service := ls.NewLanguageService(snapshot)
	uri := lsconv.FileNameToURI(name)
	ds, err := service.GetDiagnostics(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, d := range ds {
		codes[d.Code] = true
	}
	for _, want := range []string{"statement-expected", "undeclared-name", "unknown-type"} {
		if !codes[want] {
			t.Errorf("missing %s in %+v", want, ds)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.GetDiagnostics(ctx, uri); err != context.Canceled {
		t.Fatal(err)
	}
	if _, err := service.ProvideHover(ctx, at(uri, text, "absent", 2)); err != context.Canceled {
		t.Fatal(err)
	}
}

func TestDuplicateDiagnosticRelatedLocation(t *testing.T) {
	name := filepath.Join(t.TempDir(), "Main.bas")
	text := "Dim value As Long\nDim value As String\n"
	snapshot := project.NewSession(project.Config{Name: "Demo", Sources: []compiler.SourceFileInput{{FileName: name, Text: text}}}).Snapshot()
	ds, err := ls.NewLanguageService(snapshot).GetDiagnostics(context.Background(), lsconv.FileNameToURI(name))
	if err != nil || len(ds) == 0 || ds[0].Code != "duplicate-declaration" || len(ds[0].RelatedInformation) != 1 || ds[0].RelatedInformation[0].Location.Range.Start.Line != 0 {
		t.Fatal(ds, err)
	}
}

func TestNumericLabelsArraysAndObjectSetters(t *testing.T) {
	root := t.TempDir()
	mainName, className := filepath.Join(root, "Main.bas"), filepath.Join(root, "Counter.cls")
	main := "Sub Run(ByVal obj As Counter)\n Dim items(1 To 2) As Long\n Set obj.Child = obj\n GoTo 10\n10 done: items(1) = 2\nEnd Sub\n"
	class := "Public Property Set Child(ByVal value As Counter)\nEnd Property\n"
	snapshot := project.NewSession(project.Config{Name: "Demo", Sources: []compiler.SourceFileInput{{FileName: mainName, Text: main}, {FileName: className, Text: class}}}).Snapshot()
	service := ls.NewLanguageService(snapshot)
	uri := lsconv.FileNameToURI(mainName)
	if ds, err := service.GetDiagnostics(context.Background(), uri); err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	for _, test := range []struct {
		needle, hover string
		inside        int
	}{{"items(1)", "Dim items() As Long", 2}, {"obj.Child", "Property Set Child(ByVal value As Counter)", 6}} {
		hover, err := service.ProvideHover(context.Background(), at(uri, main, test.needle, test.inside))
		if err != nil || hover == nil || hover.Contents.Value != test.hover {
			t.Fatal(hover, err, test)
		}
	}
	defs, err := service.ProvideDefinition(context.Background(), at(uri, main, "GoTo 10", 6))
	if err != nil || len(defs) != 1 || defs[0].Range.Start != (lsproto.Position{Line: 4, Character: 0}) {
		t.Fatal("numeric definition selected named label", defs, err)
	}
	defs, err = service.ProvideDefinition(context.Background(), at(uri, main, "obj.Child", 6))
	if err != nil || len(defs) != 1 || defs[0].URI != lsconv.FileNameToURI(className) {
		t.Fatal(defs, err)
	}
}

func TestEditorQueriesOnIncompleteSource(t *testing.T) {
	name := filepath.Join(t.TempDir(), "Main.bas")
	uri := lsconv.FileNameToURI(name)
	for _, text := range []string{"", "Sub", "\nSub ", "Sub Run(ByVal\n", "Dim", "Public Property Get (", "Sub Run()\n Dim\n obj.\n GoTo\n", "Sub Run()\n10 done:\n If True Then\n"} {
		t.Run(text, func(t *testing.T) {
			snapshot := project.NewSession(project.Config{Name: "Demo", Sources: []compiler.SourceFileInput{{FileName: name, Text: text}}}).Snapshot()
			service := ls.NewLanguageService(snapshot)
			if _, err := service.GetDiagnostics(context.Background(), uri); err != nil {
				t.Fatal(err)
			}
			symbols, err := service.ProvideDocumentSymbols(context.Background(), uri)
			if err != nil {
				t.Fatal(err)
			}
			var check func([]*lsproto.DocumentSymbol)
			check = func(items []*lsproto.DocumentSymbol) {
				for _, item := range items {
					if item.Name == "" {
						t.Error("empty outline name")
					}
					start, _ := lsconv.NewLineMap(text).Offset(item.Range.Start)
					end, _ := lsconv.NewLineMap(text).Offset(item.Range.End)
					selectionStart, _ := lsconv.NewLineMap(text).Offset(item.SelectionRange.Start)
					selectionEnd, _ := lsconv.NewLineMap(text).Offset(item.SelectionRange.End)
					if selectionStart < start || selectionEnd > end {
						t.Errorf("invalid outline ranges: %+v", item)
					}
					check(item.Children)
				}
			}
			check(symbols)
			for offset := 0; offset <= len(text); offset++ {
				params := lsproto.TextDocumentPositionParams{TextDocument: lsproto.TextDocumentIdentifier{URI: uri}, Position: lsconv.NewLineMap(text).Position(offset)}
				if _, err := service.ProvideHover(context.Background(), params); err != nil {
					t.Fatal(err)
				}
				if _, err := service.ProvideDefinition(context.Background(), params); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

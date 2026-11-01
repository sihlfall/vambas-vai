package ls_test

import (
	"context"
	"path/filepath"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ls"
	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
	"sihlfall.ch/vambas-vai/project"
)

func TestRecordAndEnumEditorQueries(t *testing.T) {
	root := t.TempDir()
	declarations := `Public Type Point
    X As Long
End Type
Public Enum Color
    Red = 7
    Blue
End Enum
`
	main := `Option Explicit
Sub Run()
    Dim value As Shapes.Point, choice As Color
    value.X = Color.Blue
    choice = Red
End Sub
`
	declarationName, mainName := filepath.Join(root, "Shapes.bas"), filepath.Join(root, "Main.bas")
	snapshot := project.NewSession(project.Config{Name: "Demo", Sources: []compiler.SourceFileInput{{FileName: declarationName, Text: declarations}, {FileName: mainName, Text: main}}}).Snapshot()
	service := ls.NewLanguageService(snapshot)
	uri := lsconv.FileNameToURI(mainName)
	if ds, err := service.GetDiagnostics(context.Background(), uri); err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	for _, test := range []struct {
		needle string
		inside int
		hover  string
		target lsproto.Position
	}{
		{"Shapes.Point", 9, "Type Point", lsproto.Position{Line: 0, Character: 12}},
		{"As Color", 5, "Enum Color", lsproto.Position{Line: 3, Character: 12}},
		{"value.X", 6, "Point.X As Long", lsproto.Position{Line: 1, Character: 4}},
		{"Color.Blue", 8, "Color.Blue As Long = 8", lsproto.Position{Line: 5, Character: 4}},
		{"= Red", 3, "Color.Red As Long = 7", lsproto.Position{Line: 4, Character: 4}},
	} {
		params := at(uri, main, test.needle, test.inside)
		hover, err := service.ProvideHover(context.Background(), params)
		if err != nil || hover == nil || hover.Contents.Value != test.hover {
			t.Error(test, hover, err)
		}
		defs, err := service.ProvideDefinition(context.Background(), params)
		if err != nil || len(defs) != 1 || defs[0].URI != lsconv.FileNameToURI(declarationName) || defs[0].Range.Start != test.target {
			t.Error(test, defs, err)
		}
	}
	symbols, err := service.ProvideDocumentSymbols(context.Background(), lsconv.FileNameToURI(declarationName))
	if err != nil || len(symbols) != 2 || symbols[0].Name != "Point" || symbols[0].Kind != 23 || len(symbols[0].Children) != 1 || symbols[1].Kind != 10 || len(symbols[1].Children) != 2 || symbols[1].Children[0].Kind != 22 {
		t.Fatal(symbols, err)
	}
}

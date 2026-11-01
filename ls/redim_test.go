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

func TestReDimAndEnumExpressionEditorQueries(t *testing.T) {
	text := `Option Explicit
Enum Errors
 Base = vbObjectError + 3000
 NextError = Base + 1
End Enum
Sub Run()
 ReDim fresh(10) As Long
 fresh(1) = NextError
 ReDim Preserve fresh(20)
End Sub
`
	name := filepath.Join(t.TempDir(), "Main.bas")
	uri := lsconv.FileNameToURI(name)
	service := ls.NewLanguageService(project.NewSession(project.Config{Name: "Demo", Sources: []compiler.SourceFileInput{{FileName: name, Text: text}}}).Snapshot())
	if ds, err := service.GetDiagnostics(context.Background(), uri); err != nil || len(ds) != 0 {
		t.Fatal(ds, err)
	}
	for _, test := range []struct {
		needle, hover string
		inside        int
		target        lsproto.Position
	}{
		{"fresh(1)", "Dim fresh() As Long", 2, lsproto.Position{Line: 6, Character: 7}},
		{"= NextError", "Errors.NextError As Long = -2147218503", 4, lsproto.Position{Line: 3, Character: 1}},
	} {
		params := at(uri, text, test.needle, test.inside)
		hover, err := service.ProvideHover(context.Background(), params)
		if err != nil || hover == nil || hover.Contents.Value != test.hover {
			t.Fatal(test, hover, err)
		}
		defs, err := service.ProvideDefinition(context.Background(), params)
		if err != nil || len(defs) != 1 || defs[0].Range.Start != test.target {
			t.Fatal(test, defs, err)
		}
	}
	symbols, err := service.ProvideDocumentSymbols(context.Background(), uri)
	if err != nil || len(symbols) != 2 || len(symbols[1].Children) != 1 || symbols[1].Children[0].Name != "fresh" {
		t.Fatal("implicit array was absent or duplicated in Outline", symbols, err)
	}
}

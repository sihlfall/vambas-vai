package parser

import (
	"sihlfall.ch/vambas-vai/ast"
	"testing"
)

func TestAnnotationsComeOnlyFromComments(t *testing.T) {
	text := "'@TestModule\r\nRem @Folder(\"Tests\")\r\n'@TestMethod(\"Text \"\"quotes\"\"\")\r\n' ordinary comment\r\nPrivate Sub Example()\r\n Dim s As String\r\n s = \"'@TestMethod\" ' @IgnoreTest\r\nEnd Sub\r\n"
	f := ParseSourceFile(text).AsSourceFile()
	if len(f.ParseDiagnostics) > 0 {
		t.Fatal(f.ParseDiagnostics)
	}
	if len(f.Comments) != 5 || len(f.Annotations) != 4 {
		t.Fatalf("comments=%+v annotations=%+v", f.Comments, f.Annotations)
	}
	a := f.Annotations
	if a[0].Target != f.AsNode() || a[1].Target != f.AsNode() || a[2].Target != f.FunctionDeclarations.Nodes[0] || a[2].Arguments[0] != "Text \"quotes\"" || a[3].Target != nil {
		t.Fatalf("%+v", a)
	}
	for _, c := range f.Comments {
		if c.Range.Pos() < 0 || c.Range.End() > len(text) {
			t.Fatal(c)
		}
	}
}
func TestAnnotationsDoNotCrossCodeOrAttachFromProcedureBodies(t *testing.T) {
	f := ParseSourceFile("'@TestMethod\nDim value As Long\nSub First()\n '@TestModule\n '@TestMethod\nEnd Sub\nSub Second()\nEnd Sub\n").AsSourceFile()
	for _, a := range f.Annotations {
		if a.Target != nil {
			t.Fatalf("body/intervening code annotation acquired target: %+v", a)
		}
	}
}
func TestUnknownAndMalformedAnnotationsAreMetadata(t *testing.T) {
	f := ParseSourceFile("'@Description(\"safe\")\n'@TestMethod(\"broken)\nSub Example()\nEnd Sub\n").AsSourceFile()
	if len(f.ParseDiagnostics) > 0 || len(f.Annotations) != 2 || f.Annotations[0].Name != "Description" || f.Annotations[1].Error == "" {
		t.Fatal(f.Annotations, f.ParseDiagnostics)
	}
	if f.Annotations[0].Target.Kind != ast.KindSubDeclaration {
		t.Fatal("unknown annotation lost association")
	}
}

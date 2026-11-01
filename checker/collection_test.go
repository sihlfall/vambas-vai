package checker_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestCollectionUsesLibrarySignaturesAndDefaultMember(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run()
 Dim c As New VBA.Collection
 c.Add Item:="first", After:=1
 Dim v As Variant, n As Long
 v = c("key")
 n = c.Count
 With c
  .Remove Index:=1
 End With
End Sub`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	item := find(t, file, ast.KindCallExpression, `c("key")`)
	r := c.GetCallResolution(item)
	if r == nil || !r.Valid || r.DefaultMember == nil || r.DefaultMember.Library != stdlib.CollectionItem || r.DefaultMember.Intrinsic != ast.IntrinsicNone || r.Arguments[0].Parameter.Name != "Index" {
		t.Fatal("default member lost library identity", r)
	}
	requireType(t, c, item, checker.TypeVariant)
	add := c.GetCallResolution(find(t, file, ast.KindCallExpression, `c.Add Item:="first", After:=1`))
	if add == nil || !add.Valid || add.Signature.Declaration.Symbol.Library != stdlib.CollectionAdd || add.Signature.Declaration.Symbol.Intrinsic != ast.IntrinsicNone || add.Arguments[1].Parameter.Name != "After" {
		t.Fatal("named argument binding lost", add)
	}
	requireType(t, c, find(t, file, ast.KindPropertyAccessExpression, "c.Count"), checker.TypeLong)
}

func TestCollectionReadOnlyMembers(t *testing.T) {
	for _, statement := range []string{`c(1) = "value"`, `c.Item(1) = "value"`, `c.Count = 3`} {
		t.Run(statement, func(t *testing.T) {
			p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: "Sub Run(c As Collection)\n" + statement + "\nEnd Sub"}}})
			if len(p.BindDiagnostics()) == 0 && len(p.GetTypeChecker().GetDiagnostics(nil)) == 0 {
				t.Fatal("read-only member accepted as assignment target")
			}
		})
	}
}

func TestProjectCollectionTypeShadowsAmbientClass(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Collection.cls", Text: "Public Value As Long\n"},
		compiler.SourceFileInput{FileName: "Main.bas", Text: "Sub Run(c As Collection, builtIn As VBA.Collection)\n c.Value = 42\n builtIn.Add c.Value\nEnd Sub"})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	user := c.GetTypeAtLocation(find(t, file, ast.KindIdentifier, "c"))
	builtin := c.GetTypeAtLocation(find(t, file, ast.KindIdentifier, "builtIn"))
	if user.Symbol == nil || user.Symbol.Library != stdlib.None || builtin.Symbol == nil || builtin.Symbol.Library != stdlib.Collection || builtin.Symbol.Intrinsic != ast.IntrinsicNone || user.Symbol == builtin.Symbol {
		t.Fatal("ambient type shadowed a project declaration", user, builtin)
	}
}

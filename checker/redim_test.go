package checker_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
)

func TestReDimResolutionsAndScope(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Type State
 Buffer() As Long
End Type
Sub Run(ByRef state As State, ByVal n As Long)
 Dim arr() As Long, boxed As Variant
 ReDim arr(0 To Grow(n)), fresh(1 To n) As Long
 ReDim Preserve arr(0 To n + 10)
 With state
  ReDim Preserve .Buffer(0 To n)
 End With
 ReDim boxed(0 To n) As String
 ReDim Preserve boxed(0 To n + 1) As String
 Call Touch(fresh)
End Sub
Function Grow(ByVal n As Long) As Long
 Grow = n + 1
End Function
Sub Touch(ByRef values() As Long)
End Sub
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	var resolutions []*checker.ReDimResolution
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if node.Kind == ast.KindReDimDeclaration {
			r := c.GetReDimResolution(node)
			if r == nil || !r.Valid || !r.Storage.Writable || r.ArrayType.Kind != checker.TypeArray || len(r.Bounds) == 0 || !r.CheckArrayLock {
				t.Fatal(r)
			}
			if c.GetCallResolution(node) != nil {
				t.Fatal("ReDim was treated as a call")
			}
			resolutions = append(resolutions, r)
		}
		return node.ForEachChild(visit)
	}
	visit(file.AsNode())
	if len(resolutions) != 6 || !resolutions[2].Preserve || !resolutions[2].CheckPreservedBounds || resolutions[3].Storage.WithTarget == nil || !resolutions[5].CheckPreservedElementType {
		t.Fatal(resolutions)
	}
	fresh := file.FunctionDeclarations.Nodes[0].Locals.Lookup("fresh")
	if fresh == nil || fresh.ValueDeclaration.Kind != ast.KindReDimDeclaration || c.GetTypeOfSymbol(fresh).ElementType.Kind != checker.TypeLong {
		t.Fatal("declarative ReDim did not create a typed array")
	}
	boxed := file.FunctionDeclarations.Nodes[0].Locals.Lookup("boxed")
	if c.GetTypeOfSymbol(boxed).Kind != checker.TypeVariant || !resolutions[4].Deferred || resolutions[4].ArrayType.ElementType.Kind != checker.TypeString {
		t.Fatal("ReDim changed Variant scope type")
	}
	call := find(t, file, ast.KindCallExpression, "Touch(fresh)")
	if r := c.GetCallResolution(call); !r.Valid || r.Arguments[0].Mode != checker.ArgumentReference {
		t.Fatal(r)
	}
}

func TestReDimExplicitLaterDeclarationsWin(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Option Explicit\nSub Run()\n ReDim values(10)\n Dim values() As Long\nEnd Sub\n"})
	c := p.GetTypeChecker()
	noErrors(t, c)
	s := p.Module("Main").FunctionDeclarations.Nodes[0].Locals.Lookup("values")
	if s.ValueDeclaration.Kind != ast.KindVariableDeclaration {
		t.Fatal("ReDim shadowed a later explicit declaration")
	}
}

func TestInvalidReDimTargetsAndTypes(t *testing.T) {
	for _, text := range []string{
		"Sub Run()\n Dim values(1 To 3) As Long\n ReDim values(10)\nEnd Sub",
		"Sub Run()\n Dim value As Long\n ReDim value(10)\nEnd Sub",
		"Sub Run()\n Dim values() As Long\n ReDim values(10) As String\nEnd Sub",
		"Sub Run(ParamArray values() As Variant)\n ReDim values(10)\nEnd Sub",
		"Type Record\n X As Long\nEnd Type\nSub Run(ByRef bound As Record)\n Dim values() As Long\n ReDim values(bound)\nEnd Sub",
		"Function Values() As Long()\nEnd Function\nSub Run()\n ReDim Values(10)\nEnd Sub",
	} {
		p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: text})
		found := false
		for _, d := range p.GetSemanticDiagnostics(nil) {
			found = found || d.Code == checker.DiagnosticReDim
		}
		if !found {
			t.Error("invalid ReDim was accepted", text)
		}
	}
}

func TestEnumExpressionsWithAmbientConstants(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Public Enum RegexError
 REGEX_ERR = vbObjectError + 3000
 INVALID_ESCAPE = REGEX_ERR + 2
 QUALIFIED = VBA.vbObjectError + 3003
 NEXT_ERROR
End Enum
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	for name, want := range map[string]int64{"REGEX_ERR": -2147218504, "INVALID_ESCAPE": -2147218502, "QUALIFIED": -2147218501, "NEXT_ERROR": -2147218500} {
		if got, ok := c.GetEnumMemberValue(p.Module("Main").Locals.Lookup(name)); !ok || got != want {
			t.Error(name, got, ok, want)
		}
	}
	p = program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Private Const vbObjectError As Long = 10\nEnum E\n First = vbObjectError + 2\nEnd Enum\n"})
	if got, ok := p.GetTypeChecker().GetEnumMemberValue(p.Module("Main").Locals.Lookup("First")); !ok || got != 12 {
		t.Fatal("ambient declaration prevented source shadowing", got, ok)
	}
}

func TestEraseDistinguishesParamArrayEntityAndElements(t *testing.T) {
	for _, target := range []string{"values", "values()"} {
		p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Sub Run(ParamArray values() As Variant)\n Erase " + target + "\nEnd Sub\n"})
		found := false
		for _, d := range p.GetSemanticDiagnostics(nil) {
			if d.Code == checker.DiagnosticInvalidStorage {
				found = true
			}
		}
		if !found {
			t.Errorf("Erase %s accepted a ParamArray entity", target)
		}
	}
	for _, source := range []string{
		"Sub Run(ParamArray values() As Variant)\n Erase values(0)\nEnd Sub\n",
		"Sub Run(ByRef values() As Variant)\n Erase values\nEnd Sub\n",
	} {
		p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: source})
		noErrors(t, p.GetTypeChecker())
	}
}

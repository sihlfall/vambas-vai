package checker_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/stdlib"
)

func TestLibraryCallPlans(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Sub Run()
 Dim v As Variant,s As String,a() As String,n As Long
 n=AscW("x")
 v=StrComp("a","b")
 v=InStr("abc","b")
 v=InStr(2,"abc","b")
 v=InStr(String2:="b",String1:="abc")
 s=Left$("abc",1)
 v=Left("abc",1)
 v=String(2,"x")
 s=String$(2,"x")
 a=Split("a b")
 v=IIf(True,1,2)
End Sub`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	var calls []*checker.CallResolution
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindCallExpression {
			calls = append(calls, c.GetCallResolution(n))
		}
		return n.ForEachChild(visit)
	}
	visit(p.Module("Main").AsNode())
	wants := []checker.TypeKind{checker.TypeInteger, checker.TypeVariant, checker.TypeVariant, checker.TypeVariant, checker.TypeVariant, checker.TypeString, checker.TypeVariant, checker.TypeVariant, checker.TypeString, checker.TypeArray, checker.TypeVariant}
	members := []stdlib.MemberID{stdlib.AscW, stdlib.StrComp, stdlib.InStr, stdlib.InStr, stdlib.InStr, stdlib.LeftString, stdlib.Left, stdlib.StringRepeat, stdlib.StringRepeatString, stdlib.Split, stdlib.IIf}
	if len(calls) != len(wants) {
		t.Fatal("missing library calls", len(calls))
	}
	for i, r := range calls {
		if r == nil || !r.Valid || r.Deferred || r.ResultType.Kind != wants[i] {
			t.Fatalf("call %d: %+v", i, r)
		}
		if r.Signature.Declaration.Symbol.Library != members[i] {
			t.Fatalf("call %d lost its catalog member identity", i)
		}
	}
	for i, order := range map[int][]int{2: {1, 2, 0, 3}, 3: {0, 1, 2, 3}, 4: {2, 1, 0, 3}} {
		for j, index := range order {
			if calls[i].Arguments[j].ParameterIndex != index {
				t.Fatalf("call %d argument %d lost binding order", i, j)
			}
		}
	}
	if calls[9].ResultType.ElementType.Kind != checker.TypeString {
		t.Fatal("Split must produce a String SAFEARRAY")
	}
}

func TestCatalogSpecializedCallPlans(t *testing.T) {
	for _, test := range []struct {
		call     string
		member   stdlib.MemberID
		declared checker.TypeKind
		result   checker.TypeKind
	}{
		{`VBA.Len(Expression:="abc")`, stdlib.Len, checker.TypeVariant, checker.TypeLong},
		{`VBA.LenB(Expression:="abc")`, stdlib.LenB, checker.TypeVariant, checker.TypeLong},
		{`VBA.ChrW(CharCode:=65)`, stdlib.ChrW, checker.TypeString, checker.TypeString},
		{`VBA.Mid(String:="abc", Start:=2)`, stdlib.Mid, checker.TypeVariant, checker.TypeString},
		{`VBA.Mid$(String:="abc", Start:=2)`, stdlib.MidString, checker.TypeString, checker.TypeString},
		{`VBA.VarType(VarName:=value)`, stdlib.VarType, checker.TypeInteger, checker.TypeInteger},
		{`VBA.IsEmpty(Expression:=value)`, stdlib.IsEmpty, checker.TypeBoolean, checker.TypeBoolean},
		{`VBA.IsNull(Expression:=value)`, stdlib.IsNull, checker.TypeBoolean, checker.TypeBoolean},
		{`VBA.IsMissing(Expression:=absent)`, stdlib.IsMissing, checker.TypeBoolean, checker.TypeBoolean},
		{`VBA.IsArray(Expression:=values)`, stdlib.IsArray, checker.TypeBoolean, checker.TypeBoolean},
		{`VBA.CVar(Expression:=value)`, stdlib.CVar, checker.TypeVariant, checker.TypeVariant},
		{`VBA.CInt(Expression:=value)`, stdlib.CInt, checker.TypeInteger, checker.TypeInteger},
		{`VBA.CLng(Expression:=value)`, stdlib.CLng, checker.TypeLong, checker.TypeLong},
		{`VBA.CBool(Expression:=value)`, stdlib.CBool, checker.TypeBoolean, checker.TypeBoolean},
		{`VBA.CStr(Expression:=value)`, stdlib.CStr, checker.TypeString, checker.TypeString},
		{`VBA.CSng(Expression:=value)`, stdlib.CSng, checker.TypeSingle, checker.TypeSingle},
		{`VBA.CDbl(Expression:=value)`, stdlib.CDbl, checker.TypeDouble, checker.TypeDouble},
		{`VBA.LBound(ArrayName:=values)`, stdlib.LBound, checker.TypeLong, checker.TypeLong},
		{`VBA.UBound(ArrayName:=values)`, stdlib.UBound, checker.TypeLong, checker.TypeLong},
	} {
		t.Run(test.call, func(t *testing.T) {
			p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Option Explicit\nSub Run(ByRef values() As Long, Optional absent As Variant)\nDim value As Variant\nvalue=" + test.call + "\nEnd Sub\n"})
			c := p.GetTypeChecker()
			noErrors(t, c)
			n := find(t, p.Module("Main"), ast.KindCallExpression, test.call)
			r := c.GetCallResolution(n)
			if r == nil || !r.Valid || r.Deferred || r.Signature.Declaration.Symbol.Library != test.member || r.Signature.ReturnType.Kind != test.declared || r.ResultType.Kind != test.result {
				t.Fatalf("catalog call lost its specialized contract: %+v", r)
			}
			switch test.member {
			case stdlib.Mid, stdlib.MidString:
				length := r.Arguments[2]
				if length.ParameterIndex != 2 || length.Mode != checker.ArgumentMissing || length.Default != nil {
					t.Fatal("omitted Mid length lost Missing", length)
				}
			case stdlib.LBound, stdlib.UBound:
				array, dimension := r.Arguments[0], r.Arguments[1]
				if !array.Parameter.IsByRef() || array.Mode != checker.ArgumentReference || !array.PreserveStorageType {
					t.Fatal("array-bound argument lost its typed ByRef view", array)
				}
				if dimension.Mode != checker.ArgumentDefault || dimension.Parameter.IsByRef() || dimension.Parameter.Type.Kind != checker.TypeLong || dimension.Default == nil {
					t.Fatal("array-bound dimension lost its typed default", dimension)
				}
				if value, ok := c.GetNumericLiteralValue(dimension.Default); !ok || value != 1 {
					t.Fatal("array-bound dimension default is not one")
				}
			}
		})
	}
}

func TestLibraryNamesCanBeShadowed(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Function AscW(ByVal s As String) As Long
 AscW=42
End Function
Function Run() As Long
 Run=AscW("x")+VBA.AscW("A")
End Function`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	var members []stdlib.MemberID
	var visit func(*ast.Node) bool
	visit = func(n *ast.Node) bool {
		if n.Kind == ast.KindCallExpression {
			members = append(members, c.GetCallResolution(n).Signature.Declaration.Symbol.Library)
		}
		return n.ForEachChild(visit)
	}
	visit(p.Module("Main").AsNode())
	if len(members) != 2 || members[0] != stdlib.None || members[1] != stdlib.AscW {
		t.Fatal("catalog identities resolved by spelling", members)
	}
}

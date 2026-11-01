package compiler_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/parser"
)

func TestTypeVisibilityAndDuplicateMembers(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Owner.bas", Text: "Private Type Hidden\n X As Long\nEnd Type\nPrivate Enum Secret\n Forbidden = 1\nEnd Enum\nPublic Type PublicRecord\n X As Long\nEnd Type\n"},
		compiler.SourceFileInput{FileName: "Client.bas", Text: "Option Explicit\nSub Run()\n Dim a As Owner.Hidden, b As Owner.PublicRecord\n b.X = Owner.Forbidden\nEnd Sub\n"})
	if len(p.BindDiagnostics()) == 0 {
		t.Fatal("private enum constant was exported")
	}
	unknown := false
	for _, d := range p.GetSemanticDiagnostics(nil) {
		if d.Code == "unknown-type" {
			unknown = true
		}
	}
	if !unknown {
		t.Fatal("private record type leaked")
	}
	p = program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: "Type Record\n X As Long\n X As String\nEnd Type\nEnum E\n First\n First\nEnd Enum\n"})
	if len(p.BindDiagnostics()) < 2 {
		t.Fatal("duplicate members not diagnosed")
	}
}

func TestStandaloneTypesAndEnums(t *testing.T) {
	file := parser.ParseSourceFile("Type Record\n X As Long\nEnd Type\nEnum E\n First=1\n Second=E.First+1\nEnd Enum\nSub Run()\n Dim value As Record\n value.X=E.Second\nEnd Sub\n").AsSourceFile()
	binder.BindSourceFile(file)
	if len(file.ParseDiagnostics) != 0 || len(file.BindDiagnostics) != 0 {
		t.Fatal(file.ParseDiagnostics, file.BindDiagnostics)
	}
	member := file.FunctionDeclarations.Nodes[0].AsSubDeclaration().Body.AsBlock().Statements.Nodes[1].AsLetStatement().LExpression
	if member.Symbol == nil || member.Symbol.Flags&ast.SymbolFlagsProperty == 0 {
		t.Fatal("standalone field lookup failed")
	}
}

func TestEnumAliasAmbiguityAndClassScope(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Colors.bas", Text: "Enum Warm\n Shared = 1\nEnd Enum\nEnum Cool\n Shared = 2\nEnd Enum\nSub Run(result As Long)\n result = Warm.Shared + Cool.Shared\n result = Shared\nEnd Sub\n"})
	ds := p.BindDiagnostics()
	if len(ds) != 1 || ds[0].Code != ast.BindingAmbiguousName {
		t.Fatal("enum members were merged or rejected instead of remaining qualifiable", ds)
	}
	p = program(t,
		compiler.SourceFileInput{FileName: "Counter.cls", Text: "Public Enum State\n Ready = 1\nEnd Enum\n"},
		compiler.SourceFileInput{FileName: "Main.bas", Text: "Sub Run(ByVal obj As Counter, result As Long)\n result = State.Ready\n result = Ready\n result = obj.Ready\nEnd Sub\n"})
	ds = p.BindDiagnostics()
	if len(ds) != 1 || ds[0].Code != ast.BindingMemberNotFound {
		t.Fatal("class enum was made an instance member or hidden globally", ds)
	}
}

func TestSubValuesRemainInvalidInsideWith(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: "Option Explicit\nSub Run(ByRef result As Long, ByVal context As Object)\n With context\n result = Utilities.Update(1)\n End With\nEnd Sub\n"},
		compiler.SourceFileInput{FileName: "Utilities.bas", Text: "Public Sub Update(ByVal value As Long)\nEnd Sub\n"})
	found := false
	for _, d := range p.BindDiagnostics() {
		found = found || d.Code == ast.BindingInvalidValueReference
	}
	if !found {
		t.Fatal("Sub used in a value expression was accepted")
	}
}

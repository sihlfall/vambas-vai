package parser_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func TestPropertyAccessorSyntax(t *testing.T) {
	file := parseDeclarations(t, `Private stored As Variant
Public Property Get Value(Optional index As Long = 0) As Variant
Value = stored
Exit Property
End Property
Private Static Property Let Value(Optional index As Long = 0, ByVal rhs As Variant)
stored = rhs
End Property
Friend Property Set Value(Optional index As Long = 0, ByVal rhs As Object)
Set stored = rhs
End Property`)
	if len(file.Statements.Nodes) != 4 || len(file.FunctionDeclarations.Nodes) != 3 {
		t.Fatal("accessors were not retained as module elements/procedures")
	}
	for i, kind := range []ast.Kind{ast.KindGetAccessor, ast.KindLetAccessor, ast.KindSetAccessor} {
		node := file.FunctionDeclarations.Nodes[i]
		if node.Kind != kind || !ast.IsPropertyAccessor(node) || !ast.IsProcedureDeclaration(node) || ast.DeclarationName(node).AsIdentifier().Text != "Value" {
			t.Fatal("accessor identity/name was lost")
		}
	}
	getter := file.FunctionDeclarations.Nodes[0].AsGetAccessor()
	if getter.Type.AsTypeReference().TypeName.Kind != ast.KindVARIANT || len(getter.Parameters.Nodes) != 1 || getter.Body.AsBlock().Statements.Nodes[1].AsExitStatement().BlockKind != ast.KindPROPERTY {
		t.Fatal("getter signature or Exit Property was lost")
	}
	setter := file.FunctionDeclarations.Nodes[1].AsLetAccessor()
	if setter.Modifiers() != ast.ModifierPrivate|ast.ModifierStatic || setter.Type != nil || len(setter.Parameters.Nodes) != 2 || setter.Parameters.Nodes[1].AsParameterDeclaration().IsByRef() {
		t.Fatal("setter signature/modifiers were lost")
	}
	if file.FunctionDeclarations.Nodes[2].AsSetAccessor().Modifiers() != ast.VisibiltiyFriend {
		t.Fatal("Friend Property Set was not retained")
	}
	other := parseDeclarations(t, file.Text)
	if !file.AsNode().DeeplyEquals(other.AsNode()) {
		t.Fatal("equivalent accessors did not compare equal")
	}
	if file.FunctionDeclarations.Nodes[1].DeeplyEquals(file.FunctionDeclarations.Nodes[2]) {
		t.Fatal("Let and Set accessor kinds were erased")
	}
}

func TestPropertyParameterAndHeaderVariants(t *testing.T) {
	for _, source := range []string{
		"Property Get Value\nValue = 1\nEnd Property",
		"Property Let Value(ByRef rhs As Variant)\nEnd Property",
		"Property Set Value(rhs As Object)\nEnd Property",
		"Property Get Name$()\nName = \"text\"\nEnd Property",
		"Property Get Items(ParamArray index() As Variant) As Variant\nEnd Property\nProperty Let Items(ParamArray index() As Variant, ByVal rhs As Variant)\nEnd Property",
		"Property Set Item(Optional index As Long = 0, rhs As Object)\nEnd Property",
		"Property Get [Property]() As Byte()\nEnd Property",
		"Public Static Property _\n Get Value() As Long\nEnd _\n Property",
		"Property Get Value(): Value = 1: Exit Property: End Property",
	} {
		t.Run(source, func(t *testing.T) { parseDeclarations(t, source) })
	}
}

func TestPropertyRecoveryRetainsFollowingProcedure(t *testing.T) {
	for _, broken := range []string{
		"Property Get ()\nEnd Property", "Property Get Value(\nEnd Property",
		"Property Let Value()\nEnd Property", "Property Set Value\nEnd Property",
		"Property Let Value(Optional rhs As Variant)\nEnd Property",
		"Property Set Value(ParamArray rhs() As Variant)\nEnd Property",
		"Property Let Value(rhs = 1)\nEnd Property",
		"Property Let Value(ByVal rhs As Variant) As Long\nEnd Property",
		"Property Set Value(rhs As Object) As Object\nEnd Property",
		"Property Get Value() As\nEnd Property",
		"Property Get Value()\nWith obj\nEnd Property",
		"Property Get Value()\nDo\nEnd Property",
		"Property Get Value()\nValue = 1", "Property Set Value(rhs As Object)\nSet other = rhs",
		"Property Get Value()\nEnd Sub",
	} {
		t.Run(broken, func(t *testing.T) {
			file := parser.ParseSourceFile(broken + "\nPublic Property Get Good() As Long\nGood = 42\nEnd Property").AsSourceFile()
			if len(file.ParseDiagnostics) == 0 {
				t.Fatal("expected malformed accessor diagnostic")
			}
			validateRanges(t, file)
			binder.BindSourceFile(file)
			symbol := file.Locals.Lookup("Good")
			if symbol == nil || ast.PropertyAccessor(symbol, ast.KindGetAccessor) == nil {
				t.Fatal("recovery lost the next getter")
			}
			good := ast.PropertyAccessor(symbol, ast.KindGetAccessor)
			if good.Flags&ast.NodeFlagsThisNodeOrAnySubNodesHasError != 0 {
				t.Fatal("recovery marked a valid following getter")
			}
		})
	}
}

func TestPropertyKeywordInMembersAndArgumentLabels(t *testing.T) {
	expressionStatements(t, "obj.Property Property:=1\nCall obj.Set(Property:=obj.Get)")
	file := parseDeclarations(t, "Sub S()\nDim Property As Long\nProperty = 1\nGoTo Property\nProperty: Property = 2\nEnd Sub")
	binder.BindSourceFile(file)
	if len(file.BindDiagnostics) != 0 || file.FunctionDeclarations.Nodes[0].AsSubDeclaration().Labels.Lookup("Property") == nil {
		t.Fatal("the Property keyword broke an ordinary variable or label")
	}
}

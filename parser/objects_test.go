package parser_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func TestSetAndNewSyntax(t *testing.T) {
	statements := expressionStatements(t, `Set obj = New Library.Widget
Set obj.Child = Nothing
Set items(1) = obj
Set F = obj
value = New [Widget]
If obj Is Nothing Then Set obj = New Widget`)
	first := statements[0].AsSetStatement()
	if statements[0].Kind != ast.KindSetStatement || first.LExpression.AsIdentifier().Text != "obj" {
		t.Fatal("Set lost its assignment kind/target")
	}
	name := first.RExpression.AsNewExpression().Expression.AsQualifiedName()
	if name.Left.AsIdentifier().Text != "Library" || name.Right.AsIdentifier().Text != "Widget" {
		t.Fatal("New lost its qualified class name")
	}
	if statements[1].AsSetStatement().RExpression.Kind != ast.KindNOTHING || statements[2].AsSetStatement().LExpression.Kind != ast.KindCallExpression {
		t.Fatal("Set lost Nothing or its indexed target")
	}
	if statements[4].AsLetStatement().RExpression.AsNewExpression().Expression.AsIdentifier().Text != "Widget" {
		t.Fatal("New was not parsed in value-expression context")
	}
	condition := statements[5].AsIfStatement()
	if condition.Expression.AsBinaryExpression().OperatorToken.Kind != ast.KindIS || condition.ThenStatement.AsBlock().Statements.Nodes[0].Kind != ast.KindSetStatement {
		t.Fatal("object identity or inline Set was lost")
	}
}

func TestWithSyntaxAndImplicitMembers(t *testing.T) {
	statements := expressionStatements(t, `With Make()
.Value = 1
Set .Child = New Widget
.Update first:=.Value, child:=!Child
With .Child
!Key = .Value
End With
End With`)
	with := statements[0].AsWithStatement()
	if with.Expression.AsCallExpression().Expression.AsIdentifier().Text != "Make" {
		t.Fatal("With lost its receiver")
	}
	body := with.Statement.AsBlock().Statements.Nodes
	left := body[0].AsLetStatement().LExpression.AsPropertyAccessExpression()
	if left.Expression != nil || left.OperatorToken.Kind != ast.KindDOT || left.Name.AsIdentifier().Text != "Value" {
		t.Fatal("leading-dot access was rewritten into an explicit receiver")
	}
	call := body[2].AsCallStatement().Expression.AsCallExpression()
	if call.Expression.AsPropertyAccessExpression().Expression != nil || len(call.Arguments.Nodes) != 2 || call.Arguments.Nodes[1].AsNamedArgument().Expression.AsPropertyAccessExpression().OperatorToken.Kind != ast.KindEXCLAMATIONPOINT {
		t.Fatal("implicit call or dictionary argument lost its syntax")
	}
	inner := body[3].AsWithStatement()
	if inner.Expression.AsPropertyAccessExpression().Expression != nil || inner.Statement.AsBlock().Statements.Nodes[0].AsLetStatement().LExpression.AsPropertyAccessExpression().OperatorToken.Kind != ast.KindEXCLAMATIONPOINT {
		t.Fatal("nested With or leading-bang syntax was lost")
	}
}

func TestObjectSyntaxVariants(t *testing.T) {
	for _, body := range []string{
		"With obj: .Child.Value = 1: End With",
		"With obj\nSet !Child = Nothing\nCall .Update(1, count:=!Count)\nEnd With",
		"Set obj = New _\n Library. _\n Widget", // Line continuations remain trivia in class-name ranges.
		"With obj _\n .Child\n.Value = .Other\nEnd _\n With",
		"With (New Library.Widget)\n.[Column Name] = ![Other Column]\nEnd With",
		"With obj\nFor i = 1 To 2\nIf ready Then Set .Child = Nothing\nNext i\nEnd With",
		"With obj\n10 Again: .Update\nGoTo Again\nEnd With",
	} {
		t.Run(body, func(t *testing.T) { expressionStatements(t, body) })
	}
}

func TestObjectSyntaxEqualityPreservesDistinctions(t *testing.T) {
	for _, body := range []string{"Set obj = New Widget", "With obj\n.Value = 1\nEnd With"} {
		first, second := expressionStatements(t, body)[0], expressionStatements(t, body)[0]
		if !first.DeeplyEquals(second) {
			t.Fatal("equivalent object syntax did not compare equal")
		}
	}
	if expressionStatements(t, "Set obj = other")[0].DeeplyEquals(expressionStatements(t, "obj = other")[0]) {
		t.Fatal("Set and Let compared equal")
	}
	if expressionStatements(t, "With obj\n.Value = 1\nEnd With")[0].DeeplyEquals(expressionStatements(t, "With obj\nobj.Value = 1\nEnd With")[0]) {
		t.Fatal("implicit and explicit receivers compared equal")
	}
}

func TestObjectSyntaxRecovery(t *testing.T) {
	for _, broken := range []string{
		"Set", "Set obj", "Set = Nothing", "Set obj =", "Set obj = New", "Set obj = New Library.",
		"With\nEnd With", "With obj\nSet . = Nothing\nEnd With", "With obj\nSet .Child = New\nEnd With",
		"With obj\n.Update(\nEnd With", "Do\nWith obj\n.Value = 1\nLoop",
		"With obj\nDo\n.Value = 1\nEnd With", "End With",
	} {
		t.Run(broken, func(t *testing.T) {
			source := "Function F()\n" + broken + "\nF = 42\nEnd Function\nSub Good()\nEnd Sub"
			file := parser.ParseSourceFile(source).AsSourceFile()
			if len(file.ParseDiagnostics) == 0 {
				t.Fatal("expected a parse diagnostic")
			}
			validateRanges(t, file)
			binder.BindSourceFile(file)
			if file.Locals.Lookup("Good") == nil {
				t.Fatal("recovery lost the following procedure")
			}
			body := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
			last := body[len(body)-1]
			if last.Kind != ast.KindLetStatement || last.AsLetStatement().RExpression.AsNumericLiteral().Text != "42" || last.Flags&ast.NodeFlagsThisNodeOrAnySubNodesHasError != 0 {
				t.Fatal("recovery lost or marked the next valid assignment")
			}
		})
	}
	file := parser.ParseSourceFile("Sub S()\nWith obj\n.Value = 1\nEnd Sub\nSub Good()\nEnd Sub").AsSourceFile()
	if len(file.ParseDiagnostics) == 0 || len(file.FunctionDeclarations.Nodes) != 2 {
		t.Fatal("missing End With consumed a procedure boundary")
	}
	validateRanges(t, file)
}

func TestDeepWithRecovery(t *testing.T) {
	file := parser.ParseSourceFile("Sub S()\n" + strings.Repeat("With obj\n", 1000) + strings.Repeat("End With\n", 1000) + "End Sub\nSub Good()\nEnd Sub").AsSourceFile()
	if len(file.ParseDiagnostics) == 0 {
		t.Fatal("With nesting limit was not enforced")
	}
	validateRanges(t, file)
	binder.BindSourceFile(file)
	if file.Locals.Lookup("Good") == nil {
		t.Fatal("nested With recovery lost the next procedure")
	}
}

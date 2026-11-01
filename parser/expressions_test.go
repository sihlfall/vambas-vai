package parser_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func expressionStatements(t *testing.T, body string) []*ast.Node {
	t.Helper()
	file := parseDeclarations(t, "Function F()\n"+body+"\nEnd Function")
	return file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
}

func TestMemberAndApplicationChains(t *testing.T) {
	statement := expressionStatements(t, "F = -obj.Items(i, 2).Value + Make()(i)!Name")[0].AsLetStatement()
	binary := statement.RExpression.AsBinaryExpression()
	left := binary.Left.AsPrefixUnaryExpression()
	member := left.Operand.AsPropertyAccessExpression()
	items := member.Expression.AsCallExpression()
	if left.Operator != ast.KindMINUS || member.Name.AsIdentifier().Text != "Value" ||
		items.Expression.AsPropertyAccessExpression().Name.AsIdentifier().Text != "Items" ||
		len(items.Arguments.Nodes) != 2 || !items.HasParentheses {
		t.Fatal("postfix operations did not bind above unary/binary operators")
	}
	right := binary.Right.AsPropertyAccessExpression()
	index := right.Expression.AsCallExpression()
	if right.OperatorToken.Kind != ast.KindEXCLAMATIONPOINT || right.Name.AsIdentifier().Text != "Name" ||
		index.Expression.AsCallExpression().Expression.AsIdentifier().Text != "Make" ||
		len(index.Expression.AsCallExpression().Arguments.Nodes) != 0 || index.Arguments.Nodes[0].AsIdentifier().Text != "i" {
		t.Fatal("chained applications or dictionary access were lost")
	}
	assignment := expressionStatements(t, "Me.Child.Value = obj.Items(1, 2)")[0].AsLetStatement()
	if assignment.LExpression.AsPropertyAccessExpression().Expression.AsPropertyAccessExpression().Expression.Kind != ast.KindMeExpression {
		t.Fatal("Me member assignment did not retain its receiver")
	}
}

func TestExplicitAndImplicitCallSyntax(t *testing.T) {
	for _, test := range []struct {
		text                  string
		explicit, parentheses bool
		arguments             []ast.Kind
	}{
		{"S", false, false, nil},
		{"S()", false, true, nil},
		{"Call S", true, false, nil},
		{"Call S()", true, true, nil},
		{"S x, (y)", false, false, []ast.Kind{ast.KindIdentifier, ast.KindParenthesizedExpression}},
		{"S (x)", false, false, []ast.Kind{ast.KindParenthesizedExpression}},
		{"S (x), y", false, false, []ast.Kind{ast.KindParenthesizedExpression, ast.KindIdentifier}},
		{"S (x) + y", false, false, []ast.Kind{ast.KindBinaryExpression}},
		{"S (x) = y", false, false, []ast.Kind{ast.KindBinaryExpression}},
		{"S (x).Value", false, false, []ast.Kind{ast.KindPropertyAccessExpression}},
		{"S(x)", false, true, []ast.Kind{ast.KindIdentifier}},
		{"Call S(x, y)", true, true, []ast.Kind{ast.KindIdentifier, ast.KindIdentifier}},
		{"obj.Items(i).Update x", false, false, []ast.Kind{ast.KindIdentifier}},
	} {
		t.Run(test.text, func(t *testing.T) {
			statement := expressionStatements(t, test.text)[0].AsCallStatement()
			call := statement.Expression.AsCallExpression()
			if (statement.CallKeyword != nil) != test.explicit || call.HasParentheses != test.parentheses || len(call.Arguments.Nodes) != len(test.arguments) {
				t.Fatalf("call syntax lost: %+v", call)
			}
			for i, kind := range test.arguments {
				if call.Arguments.Nodes[i].Kind != kind {
					t.Fatalf("argument %d kind = %d, want %d", i, call.Arguments.Nodes[i].Kind, kind)
				}
			}
		})
	}
}

func TestNamedOmittedAndModifiedArguments(t *testing.T) {
	statements := expressionStatements(t, "F = Apply(, value, , count:=2, label:=\"ok\")\n"+
		"F = Apply(1,,)\nF = Apply(,)\nCall Apply(ByVal value, AddressOf Callback)")
	args := statements[0].AsLetStatement().RExpression.AsCallExpression().Arguments.Nodes
	if len(args) != 5 || args[0].Kind != ast.KindOmittedExpression || args[2].Kind != ast.KindOmittedExpression ||
		args[3].AsNamedArgument().Name.AsIdentifier().Text != "count" || args[4].AsNamedArgument().Expression.AsStringLiteral().Text != "ok" {
		t.Fatal("argument positions or names were lost")
	}
	for index, count := range map[int]int{1: 3, 2: 2} {
		args := statements[index].AsLetStatement().RExpression.AsCallExpression().Arguments.Nodes
		if len(args) != count || args[len(args)-1].Kind != ast.KindOmittedExpression {
			t.Fatal("trailing/adjacent omitted arguments were lost")
		}
	}
	args = statements[3].AsCallStatement().Expression.AsCallExpression().Arguments.Nodes
	if args[0].AsPrefixUnaryExpression().Operator != ast.KindBYVAL || args[1].AsPrefixUnaryExpression().Operator != ast.KindADDRESSOF {
		t.Fatal("argument passing modifiers were lost")
	}
}

func TestImplicitCallWithMemberArguments(t *testing.T) {
	for _, callee := range []string{"AppendFill", "ArrayBuffer.AppendFill", ".AppendFill", "items(i).AppendFill", "obj!AppendFill", "ArrayBuffer _\n .AppendFill"} {
		for _, separator := range []string{" ", "\t"} {
			t.Run(callee+separator, func(t *testing.T) {
				body := "With context\n" + callee + separator + ".capturesStack, nCapturePoints, -1\nEnd With"
				statement := expressionStatements(t, body)[0].AsWithStatement().Statement.AsBlock().Statements.Nodes[0].AsCallStatement()
				call := statement.Expression.AsCallExpression()
				if call.HasParentheses || len(call.Arguments.Nodes) != 3 {
					t.Fatalf("With argument was consumed as a callee member: %+v", call)
				}
				first := call.Arguments.Nodes[0]
				if first.Kind != ast.KindPropertyAccessExpression || first.AsPropertyAccessExpression().Expression != nil || first.AsPropertyAccessExpression().Name.AsIdentifier().Text != "capturesStack" {
					t.Fatal("first argument lost its omitted With receiver", first)
				}
			})
		}
	}
	statement := expressionStatements(t, "With context\nAppendFill !capturesStack, nCapturePoints, -1\nEnd With")[0].AsWithStatement().Statement.AsBlock().Statements.Nodes[0].AsCallStatement()
	call := statement.Expression.AsCallExpression()
	if len(call.Arguments.Nodes) != 3 || call.Arguments.Nodes[0].AsPropertyAccessExpression().Expression != nil || call.Arguments.Nodes[0].AsPropertyAccessExpression().OperatorToken.Kind != ast.KindEXCLAMATIONPOINT {
		t.Fatal("With dictionary argument became part of the callee")
	}
}

func TestMemberNamesAndContinuedCalls(t *testing.T) {
	for _, body := range []string{
		"F = obj!End + obj.If + obj.[Column Name]",
		"Call obj.Call(Then:=1, Else:=2)",
		"F = [Compute value]([some arg]:=1)",
		"obj _\n .Items _\n (i).Update _\n first:=Make(1), _\n second:=2",
		"If True Then obj.Update 1: Call S(2) Else obj.Update 3",
		"Let items (i) = obj!Value",
	} {
		t.Run(body, func(t *testing.T) { expressionStatements(t, body) })
	}
}

func TestCallRecoveryRetainsFollowingStatements(t *testing.T) {
	for _, broken := range []string{
		"Call", "Call S 1, 2", "Call(S)", "Call obj.", "F = obj !", "F = Apply(1,",
		"F = Apply(name:=)", "F = Apply(name:=1, 2)", "F = Apply(name:=1,)",
		"F = Apply(name:=1, NAME:=2)", "F = Apply(1 2)", "S (1, 2)", "F = obj.[broken",
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
				t.Fatal("recovery lost the next procedure")
			}
			body := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
			last := body[len(body)-1]
			if last.Kind != ast.KindLetStatement || last.AsLetStatement().RExpression.AsNumericLiteral().Text != "42" || last.Flags&ast.NodeFlagsThisNodeOrAnySubNodesHasError != 0 {
				t.Fatal("call recovery lost or marked the following assignment")
			}
		})
	}
}

func TestDeeplyNestedCallsRecover(t *testing.T) {
	file := parser.ParseSourceFile("Function F()\nF = " + strings.Repeat("Apply(", 1000) + "1" + strings.Repeat(")", 1000) + "\nEnd Function\nSub Good()\nEnd Sub").AsSourceFile()
	if len(file.ParseDiagnostics) == 0 {
		t.Fatal("deep call nesting was not diagnosed")
	}
	validateRanges(t, file)
	binder.BindSourceFile(file)
	if file.Locals.Lookup("Good") == nil {
		t.Fatal("deep nesting discarded the following procedure")
	}
}

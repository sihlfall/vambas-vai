package parser_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func TestLoopSyntaxAndSourceOrder(t *testing.T) {
	file := parseDeclarations(t, `Sub S()
For i = 10 To 1 Step -2
    While i > 2
        Exit For
    Wend
Next I
For Each item In items
    Call Visit(item)
Next item
Do While ready
    Exit Do
Loop
Do Until done
Loop
Do
Loop While ready
Do
Loop Until done
Do
    Exit Do
Loop
End Sub`)
	statements := file.FunctionDeclarations.Nodes[0].AsSubDeclaration().Body.AsBlock().Statements.Nodes
	loop := statements[0].AsForStatement()
	if loop.Initializer.AsLetStatement().LExpression.AsIdentifier().Text != "i" || loop.Limit.AsNumericLiteral().Text != "1" || loop.Step.AsPrefixUnaryExpression().Operator != ast.KindMINUS || loop.NextVariable.AsIdentifier().Text != "I" {
		t.Fatal("For header or Next name was lost")
	}
	if loop.Statement.AsBlock().Statements.Nodes[0].Kind != ast.KindWhileStatement {
		t.Fatal("nested While was lost")
	}
	each := statements[1].AsForEachStatement()
	if each.Initializer.AsIdentifier().Text != "item" || each.Expression.AsIdentifier().Text != "items" || each.NextVariable.AsIdentifier().Text != "item" {
		t.Fatal("For Each fields were lost")
	}
	for i, test := range []struct {
		kind ast.Kind
		end  bool
	}{{ast.KindWHILE, false}, {ast.KindUNTIL, false}, {ast.KindWHILE, true}, {ast.KindUNTIL, true}, {ast.KindNONE, false}} {
		loop := statements[i+2].AsDoStatement()
		if loop.ConditionKind != test.kind || loop.ConditionAtEnd != test.end || (loop.Expression == nil) != (test.kind == ast.KindNONE) {
			t.Fatal("Do loop test placement/kind was changed")
		}
	}
}

func TestSharedNextAndStatementSeparators(t *testing.T) {
	for _, body := range []string{
		"For i = 1 To 3\nFor j = 1 To 4\nx = i + j\nNext j, i",
		"For Each i In items\nFor j = 1 To 4\nNext j, i",
		"For i = 1 To 3: For j = 1 To 4: Next j, i",
		"Do: Exit Do: Loop\nWhile False: Wend",
		"For i = 1 To _\n 3 Step _\n 1\nNext i",
	} {
		t.Run(body, func(t *testing.T) { expressionStatements(t, body) })
	}
}

func TestSelectCaseSyntax(t *testing.T) {
	statements := expressionStatements(t, `Select Case value
Case 1, 2 To 4, Is >= 10, < -1
    F = 1
Case Else
    Exit Function
End Select`)
	selectNode := statements[0].AsSelectStatement()
	clauses := selectNode.CaseBlock.AsCaseBlock().Clauses.Nodes
	if selectNode.Expression.AsIdentifier().Text != "value" || len(clauses) != 2 || clauses[1].Kind != ast.KindDefaultClause || clauses[1].AsCaseClause().Expressions != nil {
		t.Fatal("Select/Case Else shape is incorrect")
	}
	items := clauses[0].AsCaseClause().Expressions.Nodes
	if len(items) != 4 || items[0].Kind != ast.KindNumericLiteral || items[1].AsCaseRangeClause().UpperBound.AsNumericLiteral().Text != "4" || items[2].AsCaseComparisonClause().IsKeyword.Kind != ast.KindIS || items[2].AsCaseComparisonClause().OperatorToken.Kind != ast.KindGEQ || items[3].AsCaseComparisonClause().IsKeyword != nil {
		t.Fatal("Case alternatives lost their syntax")
	}
}

func TestOnGotoSyntax(t *testing.T) {
	file := parseDeclarations(t, `Sub Run()
 On Pick() GoTo First, 100, _
 Last
 On index& GoTo 100: index& = 2
 If True Then On 0 GoTo First Else On 1 GoTo Last
First: Exit Sub
100: Exit Sub
Last: Exit Sub
End Sub`)
	validateRanges(t, file)
	statements := file.FunctionDeclarations.Nodes[0].AsSubDeclaration().Body.AsBlock().Statements.Nodes
	n := statements[0].AsOnGotoStatement()
	if n.Expression.Kind != ast.KindCallExpression || len(n.Destinations.Nodes) != 3 || n.Destinations.Nodes[1].AsGotoStatement().LineNumber.AsLineNumber().Text != "100" {
		t.Fatal("computed GoTo lost its expression or mixed destination list")
	}
	if statements[1].AsOnGotoStatement().Expression.AsIdentifier().TypeHint != ast.KindAMPERSAND || statements[2].Kind != ast.KindLetStatement {
		t.Fatal("type hint or colon continuation was lost")
	}
}

func TestOnGotoRecovery(t *testing.T) {
	for _, body := range []string{"On 1 GoTo", "On 1 GoTo First,", "On 1 First", "On 1 GoSub"} {
		file := parser.ParseSourceFile("Sub Bad()\n" + body + "\nEnd Sub\nSub Good()\nEnd Sub").AsSourceFile()
		if len(file.ParseDiagnostics) == 0 || len(file.FunctionDeclarations.Nodes) != 2 {
			t.Fatal("malformed dispatch swallowed the next procedure", body, file.ParseDiagnostics)
		}
		validateRanges(t, file)
		binder.BindSourceFile(file)
	}
}

func TestNumericIfBranchesAndEndStatement(t *testing.T) {
	statements := expressionStatements(t, "If ready Then 100 Else 200\n100: F = 1\nGoTo Done\n200: End\nDone: Exit Function")
	conditional := statements[0].AsIfStatement()
	for _, branch := range []*ast.Node{conditional.ThenStatement, conditional.ElseStatement} {
		if branch.AsBlock().Statements.Nodes[0].Kind != ast.KindGotoStatement {
			t.Fatal("numeric If branch was not a jump")
		}
	}
	if statements[3].AsLabeledStatement().Statement.Kind != ast.KindEndStatement {
		t.Fatal("End was confused with a block terminator")
	}
}

func TestControlFlowRecovery(t *testing.T) {
	for _, body := range []string{
		"While True\nx =\nWend", "Do While\nLoop", "Do Until True\nLoop While False",
		"For i = To\nNext", "For Each value items\nNext value", "Select Case x\nCase Is\nEnd Select",
		"Select Case x\nCase Else\nCase 1\nEnd Select", "Select Case x\nCase 1,\nEnd Select",
		"Exit While", "Next i", "Loop", "Wend", "Case 1", "End Select",
		"For i = 1 To 3\nDo\nx = 1\nNext i", // Keep Next when Loop is missing.
	} {
		t.Run(body, func(t *testing.T) {
			source := "Function F()\n" + body + "\nF = 42\nEnd Function\nSub Good()\nEnd Sub"
			file := parser.ParseSourceFile(source).AsSourceFile()
			if len(file.ParseDiagnostics) == 0 {
				t.Fatal("expected a diagnostic")
			}
			validateRanges(t, file)
			binder.BindSourceFile(file)
			if file.Locals.Lookup("Good") == nil {
				t.Fatal("recovery lost the following procedure")
			}
			found := false
			var walk func(*ast.Node) bool
			walk = func(n *ast.Node) bool {
				if n.Kind == ast.KindLetStatement && n.AsLetStatement().RExpression.Kind == ast.KindNumericLiteral && n.AsLetStatement().RExpression.AsNumericLiteral().Text == "42" {
					found = true
					if n.Flags&ast.NodeFlagsThisNodeOrAnySubNodesHasError != 0 {
						t.Fatal("recovery marked a later valid statement")
					}
				}
				return n.ForEachChild(walk)
			}
			walk(file.AsNode())
			if !found {
				t.Fatal("recovery discarded the next statement")
			}
		})
	}
	for _, body := range []string{"Do\nx = 1", "While True\nx = 1", "For i = 1 To 3\nx = 1", "Select Case x\nCase 1\nx = 1"} {
		file := parser.ParseSourceFile("Sub S()\n" + body + "\nEnd Sub\nSub Good()\nEnd Sub").AsSourceFile()
		if len(file.ParseDiagnostics) == 0 || len(file.FunctionDeclarations.Nodes) != 2 {
			t.Fatal("missing terminator recovery lost a procedure")
		}
		validateRanges(t, file)
	}
}

func TestDeepControlFlowRecovery(t *testing.T) {
	file := parser.ParseSourceFile("Sub S()\n" + strings.Repeat("Do\n", 1000) + strings.Repeat("Loop\n", 1000) + "End Sub\nSub Good()\nEnd Sub").AsSourceFile()
	if len(file.ParseDiagnostics) == 0 {
		t.Fatal("nesting limit was not enforced")
	}
	validateRanges(t, file)
	binder.BindSourceFile(file)
	if file.Locals.Lookup("Good") == nil {
		t.Fatal("nested-loop recovery lost the next procedure")
	}
}

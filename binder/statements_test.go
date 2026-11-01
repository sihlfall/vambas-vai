package binder_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
)

func controlFile(t *testing.T, body string) *ast.SourceFile {
	t.Helper()
	file := bindExpressions(t, "Function F()\n"+body+"\nEnd Function")
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	return file
}

func TestLoopReachability(t *testing.T) {
	for _, test := range []struct {
		body  string
		after bool
	}{
		{"Do\nLoop", false}, {"Do While True\nLoop", false}, {"Do Until False\nLoop", false},
		{"Do While False\nLoop", true}, {"Do Until True\nLoop", true},
		{"Do\nLoop While False", true}, {"Do\nLoop Until True", true},
		{"Do\nLoop While True", false}, {"Do\nLoop Until False", false},
		{"While True\nWend", false}, {"While False\nWend", true},
		{"Do\nExit Do\nLoop", true}, {"For i = 1 To 3\nNext i", true}, {"For Each i In items\nNext i", true},
	} {
		t.Run(test.body, func(t *testing.T) {
			file := controlFile(t, test.body+"\nF = 42")
			body := statements(file, 0)
			requireReachable(t, body[len(body)-1], test.after)
			if (file.FunctionDeclarations.Nodes[0].Flags&ast.NodeFlagsHasImplicitReturn != 0) != test.after {
				t.Fatal("fallthrough flag disagrees with loop flow")
			}
		})
	}
}

func TestOnGotoBindsLabelsAndRetainsFallthrough(t *testing.T) {
	file := controlFile(t, `On index GoTo Forward, 100, Forward
F = 1
GoTo Done
Forward: F = 2
GoTo Done
100: F = 3
On index GoTo Forward
Done: F = 4`)
	body := statements(file, 0)
	fn := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration()
	destinations := body[0].AsOnGotoStatement().Destinations.Nodes
	for i, name := range []string{"Forward", "100", "Forward"} {
		if destinations[i].AsGotoStatement().Target != fn.Labels.Lookup(name) {
			t.Fatal("computed destination did not use the procedure label namespace")
		}
	}
	requireReachable(t, body[1], true)
	requireReachable(t, fn.Labels.Lookup("Forward").ValueDeclaration, true)
	requireReachable(t, fn.Labels.Lookup("100").ValueDeclaration, true)
	bad := bindExpressions(t, "Sub Run()\n On 1 GoTo Missing, MissingToo\nEnd Sub")
	if len(bad.BindDiagnostics) != 2 {
		t.Fatal(bad.BindDiagnostics)
	}
}

func TestPreAndPostTestLoopBodies(t *testing.T) {
	for _, test := range []struct {
		loop string
		body bool
	}{
		{"Do While False\nF = 1\nLoop", false}, {"Do Until True\nF = 1\nLoop", false},
		{"Do\nF = 1\nLoop While False", true}, {"Do\nF = 1\nLoop Until True", true},
		{"While False\nF = 1\nWend", false},
	} {
		file := controlFile(t, test.loop)
		node := statements(file, 0)[0]
		var body *ast.Node
		if node.Kind == ast.KindDoStatement {
			body = node.AsDoStatement().Statement
		} else {
			body = node.AsWhileStatement().Statement
		}
		requireReachable(t, body.AsBlock().Statements.Nodes[0], test.body)
	}
}

func TestTypedExitsAndProcedureCompletion(t *testing.T) {
	file := controlFile(t, `For i = 1 To 3
Do
Exit For
F = 1
Loop
Next i
Do
While True
Exit Do
Wend
Loop
Exit Function
F = 2`)
	outer := statements(file, 0)[0]
	do := outer.AsForStatement().Statement.AsBlock().Statements.Nodes[0]
	exitFor := do.AsDoStatement().Statement.AsBlock().Statements.Nodes[0]
	if exitFor.AsExitStatement().Target != outer {
		t.Fatal("Exit For targeted the intervening Do")
	}
	requireReachable(t, do.AsDoStatement().Statement.AsBlock().Statements.Nodes[1], false)
	second := statements(file, 0)[1]
	while := second.AsDoStatement().Statement.AsBlock().Statements.Nodes[0]
	if while.AsWhileStatement().Statement.AsBlock().Statements.Nodes[0].AsExitStatement().Target != second {
		t.Fatal("Exit Do was intercepted by While")
	}
	fn := file.FunctionDeclarations.Nodes[0]
	if statements(file, 0)[2].AsExitStatement().Target != fn || fn.Flags&ast.NodeFlagsHasImplicitReturn != 0 || fn.AsFunctionDeclaration().ReturnFlowNode.Flags&ast.FlowFlagsUnreachable != 0 {
		t.Fatal("explicit procedure return was confused with fallthrough")
	}
	requireReachable(t, statements(file, 0)[3], false)
}

func TestGotoEntersLoopsAndDisconnectedCycles(t *testing.T) {
	for _, loop := range []string{
		"Do While False\nInside: Exit Do\nLoop",
		"Do\nInside: Exit Do\nLoop While True",
		"While False\nInside: F = 1\nWend",
		"For i = 1 To 3\nInside: Exit For\nNext i",
		"For Each i In items\nInside: Exit For\nNext i",
	} {
		t.Run(loop, func(t *testing.T) {
			file := controlFile(t, "GoTo Inside\n"+loop+"\nF = 42")
			label := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Labels.Lookup("Inside").ValueDeclaration
			requireReachable(t, label, true)
			requireReachable(t, label.AsLabeledStatement().Statement, true)
			body := statements(file, 0)
			requireReachable(t, body[len(body)-1], true)
		})
	}
	file := controlFile(t, "GoTo Done\nDo\nDead: F = 1\nGoTo Dead\nLoop\nDone: F = 42")
	requireReachable(t, file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Labels.Lookup("Dead").ValueDeclaration, false)
	file = controlFile(t, "GoTo Later\nDo\nInside: Exit Do\nLoop\nGoTo Done\nLater: GoTo Inside\nDone: F = 42")
	requireReachable(t, statements(file, 0)[2], true)
}

func TestSelectCasesHaveNoFallthrough(t *testing.T) {
	file := controlFile(t, `Select Case value
Case 1, 2 To 4
F = 1
Case Is >= 10
F = 2
Case Else
F = 3
End Select
F = 42`)
	clauses := statements(file, 0)[0].AsSelectStatement().CaseBlock.AsCaseBlock().Clauses.Nodes
	for i, clause := range clauses {
		requireReachable(t, clause.AsCaseClause().Statement.AsBlock().Statements.Nodes[0], true)
		if i > 0 && clause.FlowNode.Flags&ast.FlowFlagsFalseCondition == 0 {
			t.Fatal("a case body fell into the following case")
		}
	}
	merge := statements(file, 0)[1].FlowNode
	count := 0
	for item := merge.Antecedents; item != nil; item = item.Next {
		count++
	}
	if count != 3 {
		t.Fatal("Select did not merge the three independent case bodies")
	}
	for _, withDefault := range []bool{false, true} {
		body := "Select Case value\nCase 1\nExit Function\n"
		if withDefault {
			body += "Case Else\nExit Function\n"
		}
		file := controlFile(t, body+"End Select\nF = 42")
		requireReachable(t, statements(file, 0)[1], !withDefault)
	}
}

func TestGotoEntersCaseWithoutEvaluatingSelectorOrFallingThrough(t *testing.T) {
	file := controlFile(t, `GoTo Inside
Select Case value
Case 1
Inside: F = 1
Case Else
F = 2
End Select
F = 42`)
	selection := statements(file, 0)[1]
	clauses := selection.AsSelectStatement().CaseBlock.AsCaseBlock().Clauses.Nodes
	requireReachable(t, selection, false)
	requireReachable(t, clauses[0].AsCaseClause().Statement.AsBlock().Statements.Nodes[0], true)
	requireReachable(t, clauses[1].AsCaseClause().Statement.AsBlock().Statements.Nodes[0], false)
	requireReachable(t, statements(file, 0)[2], true)
}

func TestForBoundsAndCollectionAreEvaluatedOnce(t *testing.T) {
	file := bindExpressions(t, `Function F()
For i = First() To Last() Step Stride()
F = i
Next I
For Each item In Items()
F = item
Next item
End Function
Function First()
End Function
Function Last()
End Function
Function Stride()
End Function
Function Items()
End Function`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	loop := statements(file, 0)[0].AsForStatement()
	if loop.Initializer.AsLetStatement().RExpression.FlowNode.Flags&ast.FlowFlagsStart == 0 || loop.Limit.FlowNode.Flags&ast.FlowFlagsAssignment == 0 || loop.Step.FlowNode.Node != loop.Limit {
		t.Fatal("For header expressions were not evaluated in entry order")
	}
	if loop.Statement.FlowNode.Flags&ast.FlowFlagsTrueCondition == 0 || loop.NextVariable.Symbol != loop.Initializer.AsLetStatement().LExpression.Symbol {
		t.Fatal("For control variable/condition binding failed")
	}
	each := statements(file, 0)[1].AsForEachStatement()
	if each.Statement.FlowNode.Flags&ast.FlowFlagsAssignment == 0 || each.Statement.FlowNode.Node != each.Initializer {
		t.Fatal("For Each does not assign its control variable per iteration")
	}
}

func TestControlFlowDiagnosticsAndEnd(t *testing.T) {
	for _, body := range []string{"Exit Do", "Exit For", "Exit Sub", "For i = 1 To 3\nNext j"} {
		file := bindExpressions(t, "Function F()\n"+body+"\nEnd Function")
		if len(file.BindDiagnostics) != 1 {
			t.Fatalf("%s: unexpected diagnostics %+v", body, file.BindDiagnostics)
		}
		if body == "Exit Sub" && file.BindDiagnostics[0].Code != ast.BindingInvalidExit {
			t.Fatal("wrong diagnostic for mismatched procedure exit")
		}
		if file.FunctionDeclarations.Nodes[0].Locals.Lookup("j") != nil {
			t.Fatal("Next mismatch introduced an implicit variable")
		}
	}
	file := controlFile(t, "End\nF = 42")
	requireReachable(t, statements(file, 0)[1], false)
	if file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().ReturnFlowNode.Flags&ast.FlowFlagsUnreachable == 0 {
		t.Fatal("End was treated as a procedure return")
	}
	file = controlFile(t, "If ready Then 100 Else 200\n100: Exit Function\n200: Exit Function\nF = 42")
	requireReachable(t, statements(file, 0)[3], false)
}

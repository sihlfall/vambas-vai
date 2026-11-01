package binder_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
)

func TestNewUsesTypeNamesAndCreatesOneFlow(t *testing.T) {
	file := bindExpressions(t, `Option Explicit
Sub S()
Dim obj As Object
Set obj = New Library.Widget
Set obj = Nothing
End Sub`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	fn := file.FunctionDeclarations.Nodes[0]
	if len(fn.Locals) != 1 {
		t.Fatal("class-name syntax introduced value locals")
	}
	statement := statements(file, 0)[1].AsSetStatement()
	new := statement.RExpression
	name := new.AsNewExpression().Expression.AsQualifiedName()
	if name.Left.Symbol != nil || name.Right.Symbol != nil || name.Left.FlowNode != nil || name.Right.FlowNode != nil {
		t.Fatal("New class names were bound as values")
	}
	flow := statements(file, 0)[2].FlowNode
	if flow.Flags&ast.FlowFlagsAssignment == 0 || flow.Node != statement.LExpression || flow.Antecedent.Flags&ast.FlowFlagsCall == 0 || flow.Antecedent.Node != new || flow.Antecedent.Antecedent != new.FlowNode || new.FlowNode.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("New did not create exactly one creation flow before Set")
	}
}

func TestNewAndAsNewRemainDistinct(t *testing.T) {
	file := bindExpressions(t, `Option Explicit
Sub S()
Dim lazy As New Widget
Dim eager As Object
Set eager = New Widget
End Sub
Function Widget()
End Function`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	start := file.FunctionDeclarations.Nodes[0].FlowNode
	if statements(file, 0)[2].FlowNode != start || statements(file, 0)[2].AsSetStatement().RExpression.AsNewExpression().Expression.Symbol != nil {
		t.Fatal("As New eagerly constructed or New invoked a same-named procedure")
	}
}

func TestWithCapturesReceiverOnceAndRestoresOuterContext(t *testing.T) {
	file := bindExpressions(t, `Option Explicit
Sub S(other As Object)
With Make()
.Update
With .Child
Set !Child = other
.Update
End With
Set .Child = other
End With
End Sub
Function Make() As Object
End Function`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	outer := statements(file, 0)[0]
	body := outer.AsWithStatement().Statement.AsBlock().Statements.Nodes
	inner := body[1]
	if inner.AsWithStatement().Expression.AsPropertyAccessExpression().WithTarget != outer || body[0].AsCallStatement().Expression.AsCallExpression().Expression.AsPropertyAccessExpression().WithTarget != outer || body[2].AsSetStatement().LExpression.AsPropertyAccessExpression().WithTarget != outer {
		t.Fatal("nested headers or following accesses did not use the outer With")
	}
	innerBody := inner.AsWithStatement().Statement.AsBlock().Statements.Nodes
	if innerBody[0].AsSetStatement().LExpression.AsPropertyAccessExpression().WithTarget != inner || innerBody[1].AsCallStatement().Expression.AsCallExpression().Expression.AsPropertyAccessExpression().WithTarget != inner {
		t.Fatal("nested body did not use its own With")
	}
	flow := file.FunctionDeclarations.Nodes[0].AsSubDeclaration().EndFlowNode
	creations := 0
	for flow != nil {
		if flow.Flags&ast.FlowFlagsCall != 0 && flow.Node == outer.AsWithStatement().Expression {
			creations++
		}
		flow = flow.Antecedent
	}
	if creations != 1 || outer.AsWithStatement().Statement.FlowNode.Node != outer.AsWithStatement().Expression || len(file.FunctionDeclarations.Nodes[0].Locals) != 1 {
		t.Fatal("With receiver was repeated or introduced a synthetic variable/scope")
	}
	// Back references must not become syntax children or affect syntax equality.
	other := bindExpressions(t, file.Text)
	if !file.AsNode().DeeplyEquals(other.AsNode()) {
		t.Fatal("With binding metadata changed syntax equality")
	}
}

func TestWithScopeIsLexicalAndDoesNotLeak(t *testing.T) {
	file := bindExpressions(t, `Sub S(obj As Object)
With .Child
.Value = 1
End With
With obj
If True Then .Value = 2
End With
.Value = 3
End Sub
Sub T()
!Value = 4
End Sub`)
	if len(file.BindDiagnostics) != 3 {
		t.Fatalf("unexpected diagnostics: %+v", file.BindDiagnostics)
	}
	for _, diagnostic := range file.BindDiagnostics {
		if diagnostic.Code != ast.BindingWithBlockExpected {
			t.Fatalf("unexpected diagnostic: %+v", diagnostic)
		}
	}
	first := statements(file, 0)[0]
	if first.AsWithStatement().Expression.AsPropertyAccessExpression().WithTarget != nil || first.AsWithStatement().Statement.AsBlock().Statements.Nodes[0].AsLetStatement().LExpression.AsPropertyAccessExpression().WithTarget != first {
		t.Fatal("a With header incorrectly bound to itself")
	}
	if len(file.FunctionDeclarations.Nodes[1].Locals) != 0 {
		t.Fatal("out-of-scope member names became locals")
	}
}

func TestWithDoesNotReevaluateReassignedVariable(t *testing.T) {
	file := bindExpressions(t, `Sub S(obj As Object, other As Object)
With obj
Set obj = other
Set .Child = Nothing
End With
End Sub`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	with := statements(file, 0)[0]
	body := with.AsWithStatement().Statement.AsBlock().Statements.Nodes
	member := body[1].AsSetStatement().LExpression.AsPropertyAccessExpression()
	if member.Expression != nil || member.WithTarget != with || member.FlowNode != body[1].FlowNode || member.FlowNode.Flags&ast.FlowFlagsAssignment == 0 || member.FlowNode.Node != body[0].AsSetStatement().LExpression {
		t.Fatal("implicit receiver was expanded into a fresh evaluation")
	}
}

func TestWithAndNewReachability(t *testing.T) {
	file := bindExpressions(t, `Sub S()
GoTo Inside
With Make()
Inside: Set .Child = New Widget
End With
Exit Sub
Set obj = New Widget
End Sub
Function Make() As Object
End Function`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	with := statements(file, 0)[1]
	requireReachable(t, with, false)
	label := with.AsWithStatement().Statement.AsBlock().Statements.Nodes[0]
	requireReachable(t, label, true)
	set := label.AsLabeledStatement().Statement.AsSetStatement()
	if set.LExpression.AsPropertyAccessExpression().WithTarget != with {
		t.Fatal("GoTo changed the lexical With target")
	}
	requireReachable(t, set.RExpression, true)
	requireReachable(t, statements(file, 0)[3].AsSetStatement().RExpression, false)
}

func TestSetAssignmentChecksAndFunctionResults(t *testing.T) {
	file := bindExpressions(t, `Option Explicit
Function F() As Object
Set F = New Widget
End Function
Sub S(obj As Object)
Const Value = 1
Set Value = Nothing
Set Missing = Nothing
Set F = Nothing
Set Me = Nothing
Set obj.Child = F()
End Sub`)
	if len(file.BindDiagnostics) != 4 {
		t.Fatalf("unexpected diagnostics: %+v", file.BindDiagnostics)
	}
	if statements(file, 0)[0].AsSetStatement().LExpression.Symbol != file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().ResultSymbol {
		t.Fatal("Set did not assign the function's object result")
	}
	counts := map[ast.BindingDiagnosticCode]int{}
	for _, d := range file.BindDiagnostics {
		counts[d.Code]++
	}
	if counts[ast.BindingAssignmentToConstant] != 1 || counts[ast.BindingUndeclaredName] != 1 || counts[ast.BindingInvalidAssignmentTarget] != 2 {
		t.Fatal("Set did not share assignment target checks")
	}
	set := statements(file, 1)[5].AsSetStatement()
	flow := file.FunctionDeclarations.Nodes[1].AsSubDeclaration().EndFlowNode
	if flow.Flags&ast.FlowFlagsAssignment == 0 || flow.Node != set.LExpression || flow.Antecedent.Node != set.RExpression || flow.Antecedent.Antecedent.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("invalid Set targets invoked procedures or a valid call was duplicated")
	}
}

package binder_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func bindExpressions(t *testing.T, source string) *ast.SourceFile {
	t.Helper()
	file := parser.ParseSourceFile(source).AsSourceFile()
	if len(file.ParseDiagnostics) != 0 {
		for _, diagnostic := range file.ParseDiagnostics {
			t.Errorf("%s: %s", diagnostic.Code, diagnostic.Message)
		}
		t.FailNow()
	}
	binder.BindSourceFile(file)
	return file
}

func TestMembersAndArgumentNamesAreNotLocals(t *testing.T) {
	file := bindExpressions(t, `Option Explicit
Sub S(obj As Object, value As Long)
Call obj.Update(count:=value)
obj!End = value
End Sub`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	fn := file.FunctionDeclarations.Nodes[0]
	if len(fn.Locals) != 2 {
		t.Fatalf("member or named-argument label became a local: %v", fn.Locals)
	}
	call := statements(file, 0)[0].AsCallStatement().Expression.AsCallExpression()
	member := call.Expression.AsPropertyAccessExpression()
	arg := call.Arguments.Nodes[0].AsNamedArgument()
	if member.Name.Symbol != nil || arg.Name.Symbol != nil || member.Expression.Symbol != fn.Locals.Lookup("obj") || arg.Expression.Symbol != fn.Locals.Lookup("value") {
		t.Fatal("receiver/value binding did not distinguish names from expressions")
	}
	if statements(file, 0)[1].AsLetStatement().LExpression.AsPropertyAccessExpression().Name.Symbol != nil {
		t.Fatal("dictionary access name was treated as a value reference")
	}
}

func TestCallFlowsEvaluateTargetAndArgumentsOnce(t *testing.T) {
	file := bindExpressions(t, `Function F()
F = Outer(First(), second:=Last())
End Function
Function First()
End Function
Function Last()
End Function
Function Outer(ByVal first As Long, ByVal second As Long)
End Function`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	fn := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration()
	call := statements(file, 0)[0].AsLetStatement().RExpression
	outer := call.AsCallExpression()
	first, last := outer.Arguments.Nodes[0], outer.Arguments.Nodes[1].AsNamedArgument().Expression
	flow := fn.EndFlowNode.Antecedent // Before assigning the function result.
	for _, expected := range []*ast.Node{call, last, first} {
		if flow.Flags&ast.FlowFlagsCall == 0 || flow.Node != expected {
			t.Fatal("missing, duplicate, or misordered call flow")
		}
		flow = flow.Antecedent
	}
	if flow.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("callee identifiers were also invoked as bare functions")
	}
	if last.FlowNode.Node != first || outer.Arguments.Nodes[1].FlowNode != last.FlowNode {
		t.Fatal("argument incoming flow did not include the preceding invocation")
	}
}

func TestSubStatementsAndRecursiveFunctionCalls(t *testing.T) {
	file := bindExpressions(t, `Sub S()
Call Later(1)
Later 2
Later (3)
End Sub
Sub Later(ByVal value As Long)
End Sub
Function F(ByVal value As Long)
F = F(value - 1)
End Function`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	sub := file.FunctionDeclarations.Nodes[0].AsSubDeclaration()
	flow := sub.EndFlowNode
	for i := 2; i >= 0; i-- {
		call := statements(file, 0)[i].AsCallStatement().Expression
		if flow.Node != call || flow.Flags&ast.FlowFlagsCall == 0 || call.AsCallExpression().Expression.Symbol != file.Locals.Lookup("Later") {
			t.Fatal("Sub invocation did not bind or produced duplicate flow")
		}
		flow = flow.Antecedent
	}
	fnNode := file.FunctionDeclarations.Nodes[2]
	assignment := statements(file, 2)[0].AsLetStatement()
	if assignment.LExpression.Symbol != fnNode.AsFunctionDeclaration().ResultSymbol || assignment.RExpression.AsCallExpression().Expression.Symbol != fnNode.Symbol {
		t.Fatal("recursive invocation was bound to the function result variable")
	}
}

func TestArrayIndexingAndMemberAssignments(t *testing.T) {
	file := bindExpressions(t, `Option Explicit
Function F(obj As Object, values() As Long, i As Long)
Dim rows(1 To 3, 1 To 4) As Long
rows(i, 2) = 1
values(i) = rows(i, 2)
obj.Item(i) = values(i)
obj.Value = rows(i, 2)
F = values(i)
End Function`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	flow := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().EndFlowNode
	for i := 5; i >= 1; i-- {
		if flow.Flags&ast.FlowFlagsAssignment == 0 || flow.Node != statements(file, 0)[i].AsLetStatement().LExpression {
			t.Fatal("array/property assignment lost flow or introduced an invocation")
		}
		flow = flow.Antecedent
	}
	if flow.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("array indices were treated as calls")
	}
}

func TestCallsInUnreachableCodeAndAddressOf(t *testing.T) {
	file := bindExpressions(t, `Sub S()
Call Register(AddressOf Callback)
GoTo Done
Call Callback
Done: Callback
End Sub
Sub Register(callback As Long)
End Sub
Sub Callback()
End Sub`)
	if len(file.BindDiagnostics) != 0 {
		t.Fatal(file.BindDiagnostics)
	}
	first := statements(file, 0)[0].AsCallStatement().Expression
	if statements(file, 0)[1].FlowNode.Node != first || first.FlowNode.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("AddressOf invoked its callback instead of referencing it")
	}
	requireReachable(t, statements(file, 0)[2], false)
	requireReachable(t, statements(file, 0)[3], true)
}

func TestInvalidCallValuesAndUnknownNames(t *testing.T) {
	for _, test := range []struct {
		body string
		code ast.BindingDiagnosticCode
	}{
		{"value = P()", ast.BindingInvalidValueReference},
		{"Call Missing()", ast.BindingUndeclaredName},
		{"Call obj.Update(count:=missing)", ast.BindingUndeclaredName},
		{"P(1) = 2", ast.BindingInvalidAssignmentTarget},
	} {
		t.Run(test.body, func(t *testing.T) {
			file := bindExpressions(t, "Option Explicit\nSub S(obj As Object, value As Long)\n"+test.body+"\nEnd Sub\nSub P(Optional x As Long = 1)\nEnd Sub")
			if len(file.BindDiagnostics) != 1 || file.BindDiagnostics[0].Code != test.code {
				t.Fatalf("unexpected diagnostics: %+v", file.BindDiagnostics)
			}
			if file.FunctionDeclarations.Nodes[0].Locals.Lookup("Missing") != nil || file.FunctionDeclarations.Nodes[0].Locals.Lookup("count") != nil {
				t.Fatal("unknown value/argument name became a local under Option Explicit")
			}
		})
	}
}

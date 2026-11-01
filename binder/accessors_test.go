package binder_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
)

func TestAccessorsMergeAndKeepIndependentScopes(t *testing.T) {
	file := bindDeclarations(t, `Option Explicit
Private stored As Variant
Public Property Set Value(ByVal rhs As Object)
Dim local As Object
Set stored = rhs
Done: Exit Property
End Property
Public Property Let VALUE(ByVal rhs As Variant)
Dim local As Variant
stored = rhs
Done: Exit Property
End Property
Public Property Get value() As Variant
Dim local As Variant
value = stored
Done: Exit Property
End Property`)
	requireNoBindDiagnostics(t, file)
	symbol := file.Locals.Lookup("VALUE")
	if symbol == nil || symbol.Flags&ast.SymbolFlagsAccessor != ast.SymbolFlagsAccessor || len(symbol.Declarations) != 3 || symbol.ValueDeclaration.Kind != ast.KindGetAccessor || file.Symbol.Exports.Lookup("value") != symbol {
		t.Fatal("property declarations did not merge")
	}
	var previousLocal, previousLabel *ast.Symbol
	for _, node := range file.FunctionDeclarations.Nodes {
		fn := node.AsFunctionDeclaration()
		if node.Symbol != symbol || fn.Name().Symbol != symbol || ast.PropertyAccessor(symbol, node.Kind) != node {
			t.Fatal("accessor symbol/declaration links disagree")
		}
		if node.Locals.Lookup("local") == previousLocal || fn.Labels.Lookup("Done") == previousLabel {
			t.Fatal("accessors shared local or label scopes")
		}
		previousLocal, previousLabel = node.Locals.Lookup("local"), fn.Labels.Lookup("Done")
		if node.Kind == ast.KindGetAccessor {
			if fn.ResultSymbol == nil || fn.ResultSymbol == symbol || node.Locals.Lookup("value") != fn.ResultSymbol || statementsForAccessor(node)[1].AsLetStatement().LExpression.Symbol != fn.ResultSymbol {
				t.Fatal("getter did not receive its own result variable")
			}
		} else if fn.ResultSymbol != nil {
			t.Fatal("setter received a result variable")
		}
		last := statementsForAccessor(node)[2].AsLabeledStatement().Statement
		if last.AsExitStatement().Target != node || fn.ReturnFlowNode.Flags&ast.FlowFlagsUnreachable != 0 || node.Flags&ast.NodeFlagsHasImplicitReturn != 0 {
			t.Fatal("Exit Property did not return from its accessor")
		}
	}
}

func statementsForAccessor(node *ast.Node) []*ast.Node {
	return node.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
}

func TestPropertyReadsAndWritesHaveOneInvocation(t *testing.T) {
	file := bindDeclarations(t, `Option Explicit
Sub S(obj As Object, result As Variant)
result = Value
Value = result
Set Value = obj
End Sub
Property Get Value() As Variant
Value = 1
End Property
Property Let Value(ByVal rhs As Variant)
End Property
Property Set Value(ByVal rhs As Object)
End Property`)
	requireNoBindDiagnostics(t, file)
	property := file.Locals.Lookup("Value")
	body := statements(file, 0)
	if body[0].AsLetStatement().RExpression.Symbol != property || body[1].AsLetStatement().LExpression.Symbol != property || body[2].AsSetStatement().LExpression.Symbol != property {
		t.Fatal("property references lost the merged symbol")
	}
	flow := file.FunctionDeclarations.Nodes[0].AsSubDeclaration().EndFlowNode
	for _, expected := range []*ast.Node{body[2].AsSetStatement().LExpression, body[1].AsLetStatement().LExpression, body[0].AsLetStatement().LExpression} {
		if flow.Flags&ast.FlowFlagsAssignment == 0 || flow.Node != expected || flow.Antecedent.Flags&ast.FlowFlagsCall == 0 {
			t.Fatal("property read/write is missing a call or write")
		}
		flow = flow.Antecedent.Antecedent
	}
	if flow.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("a property operation was invoked twice")
	}
}

func TestIndexedPropertyEvaluationOrder(t *testing.T) {
	file := bindDeclarations(t, `Sub S()
Value(Index()) = RHS()
End Sub
Property Let Value(index As Long, ByVal rhs As Variant)
End Property
Function Index()
End Function
Function RHS()
End Function`)
	requireNoBindDiagnostics(t, file)
	write := statements(file, 0)[0].AsLetStatement()
	index := write.LExpression.AsCallExpression().Arguments.Nodes[0]
	flow := file.FunctionDeclarations.Nodes[0].AsSubDeclaration().EndFlowNode
	if flow.Flags&ast.FlowFlagsAssignment == 0 || flow.Node != write.LExpression {
		t.Fatal("missing indexed property write")
	}
	flow = flow.Antecedent
	for _, node := range []*ast.Node{write.LExpression, write.RExpression, index} {
		if flow.Flags&ast.FlowFlagsCall == 0 || flow.Node != node {
			t.Fatal("index, RHS, or setter was invoked out of order")
		}
		flow = flow.Antecedent
	}
	if flow.Flags&ast.FlowFlagsStart == 0 || write.LExpression.Symbol != file.Locals.Lookup("Value") {
		t.Fatal("setter was read as a getter or lost its property symbol")
	}
}

func TestPropertyConflicts(t *testing.T) {
	for _, source := range []string{
		"Property Get Value()\nEnd Property\nProperty Get VALUE()\nEnd Property",
		"Property Let Value(rhs)\nEnd Property\nProperty Let Value(rhs)\nEnd Property",
		"Property Set Value(rhs)\nEnd Property\nProperty Set Value(rhs)\nEnd Property",
		"Dim Value\nProperty Get Value()\nEnd Property",
		"Sub Value()\nEnd Sub\nProperty Get Value()\nEnd Property",
		"Property Get Value()\nEnd Property\nFunction Value()\nEnd Function",
	} {
		file := bindDeclarations(t, source)
		if len(file.BindDiagnostics) != 1 || file.BindDiagnostics[0].Code != ast.BindingDuplicateDeclaration || file.BindDiagnostics[0].RelatedNode == nil || !file.BindDiagnostics[0].RelatedLoc.IsValid() {
			t.Fatalf("unexpected conflict diagnostics: %+v", file.BindDiagnostics)
		}
	}
}

func TestPropertyAccessDiagnostics(t *testing.T) {
	for _, test := range []struct {
		body, accessor string
		code           ast.BindingDiagnosticCode
	}{
		{"Value = 1", "Property Get Value()\nEnd Property", ast.BindingInvalidAssignmentTarget},
		{"Set Value = Nothing", "Property Let Value(rhs)\nEnd Property", ast.BindingInvalidAssignmentTarget},
		{"Value = 1", "Property Set Value(rhs)\nEnd Property", ast.BindingInvalidAssignmentTarget},
		{"result = Value", "Property Let Value(rhs)\nEnd Property", ast.BindingInvalidValueReference},
		{"result = Value(1)", "Property Set Value(index, rhs)\nEnd Property", ast.BindingInvalidValueReference},
	} {
		file := bindDeclarations(t, "Sub S(result)\n"+test.body+"\nEnd Sub\n"+test.accessor)
		if len(file.BindDiagnostics) != 1 || file.BindDiagnostics[0].Code != test.code {
			t.Fatalf("%s: %+v", test.body, file.BindDiagnostics)
		}
	}
}

func TestPropertyExitAndLabelNamespaces(t *testing.T) {
	file := bindDeclarations(t, `Property Get Value()
GoTo Missing
End Property
Property Let Value(rhs)
Missing: Exit Property
End Property
Sub S()
Exit Property
End Sub`)
	if len(file.BindDiagnostics) != 2 || file.BindDiagnostics[0].Code != ast.BindingUndefinedLabel || file.BindDiagnostics[1].Code != ast.BindingInvalidExit {
		t.Fatalf("property scopes/exits leaked: %+v", file.BindDiagnostics)
	}
}

func TestPrivatePropertyExportsAndGetterRecursion(t *testing.T) {
	file := bindDeclarations(t, `Private Property Let Value(index As Long, rhs)
End Property
Private Property Get Value(index As Long)
Value = Value(index - 1)
End Property`)
	requireNoBindDiagnostics(t, file)
	property := file.Locals.Lookup("value")
	if property == nil || file.Symbol.Exports.Lookup("value") != nil {
		t.Fatal("private property was exported")
	}
	getter := ast.PropertyAccessor(property, ast.KindGetAccessor)
	assignment := statementsForAccessor(getter)[0].AsLetStatement()
	if assignment.LExpression.Symbol != getter.AsGetAccessor().ResultSymbol || assignment.RExpression.AsCallExpression().Expression.Symbol != property {
		t.Fatal("getter call was confused with the result variable")
	}
}

func TestAssignmentThroughGetterResultDoesNotInvokePropertySetter(t *testing.T) {
	file := bindDeclarations(t, `Sub S()
Value()(1) = 42
End Sub
Property Get Value() As Variant
End Property`)
	requireNoBindDiagnostics(t, file)
	left := statements(file, 0)[0].AsLetStatement().LExpression
	getterCall := left.AsCallExpression().Expression
	flow := file.FunctionDeclarations.Nodes[0].AsSubDeclaration().EndFlowNode
	if flow.Flags&ast.FlowFlagsAssignment == 0 || flow.Node != left || flow.Antecedent.Flags&ast.FlowFlagsCall == 0 || flow.Antecedent.Node != getterCall || flow.Antecedent.Antecedent.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("getter result indexing was mistaken for a property setter")
	}
}

package binder_test

import (
	"fmt"
	"sync"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func bind(t *testing.T, source string) *ast.SourceFile {
	t.Helper()
	file := parser.ParseSourceFile(source).AsSourceFile()
	binder.BindSourceFile(file)
	if len(file.BindDiagnostics) != 0 {
		t.Fatalf("unexpected binding diagnostics: %+v", file.BindDiagnostics)
	}
	return file
}

func statements(file *ast.SourceFile, index int) []*ast.Node {
	return file.FunctionDeclarations.Nodes[index].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes
}

func requireReachable(t *testing.T, node *ast.Node, want bool) {
	t.Helper()
	if node.FlowNode == nil {
		t.Fatal("missing flow node")
	}
	if got := node.Flags&ast.NodeFlagsUnreachable == 0; got != want {
		t.Errorf("kind %d reachable = %v, want %v", node.Kind, got, want)
	}
	if got := node.FlowNode.Flags&ast.FlowFlagsUnreachable == 0; got != want {
		t.Errorf("kind %d flow reachable = %v, want %v", node.Kind, got, want)
	}
}

func TestSymbolsScopesAndParents(t *testing.T) {
	file := bind(t, "Private Function Hidden()\nvalue = Later + VALUE\nHidden = value\nEnd Function\n"+
		"Public Function Later()\nvalue = 2\nLater = value\nEnd Function")
	hidden, later := file.FunctionDeclarations.Nodes[0], file.FunctionDeclarations.Nodes[1]
	if !file.IsBound() || file.Parent != nil || file.Symbol.Flags != ast.SymbolFlagsValueModule {
		t.Fatal("source file was not initialized")
	}
	if len(file.Locals) != 2 || file.Locals.Lookup("HIDDEN") != hidden.Symbol || file.Locals.Lookup("later") != later.Symbol {
		t.Fatal("module functions were not declared case-insensitively")
	}
	if len(file.Symbol.Exports) != 1 || file.Symbol.Exports.Lookup("LATER") != later.Symbol {
		t.Fatal("private function was exported or public function was missing")
	}
	for _, fn := range []*ast.Node{hidden, later} {
		if fn.Symbol.Flags != ast.SymbolFlagsFunction || fn.Symbol.Parent != file.Symbol ||
			fn.Symbol.ValueDeclaration != fn || len(fn.Symbol.Declarations) != 1 || fn.Symbol.Declarations[0] != fn {
			t.Fatal("invalid procedure symbol")
		}
		decl := fn.AsFunctionDeclaration()
		if decl.Name().Symbol != fn.Symbol || decl.ResultSymbol == fn.Symbol ||
			decl.ResultSymbol.Flags != ast.SymbolFlagsFunctionScopedVariable ||
			fn.Locals.Lookup(decl.Name().AsIdentifier().Text) != decl.ResultSymbol {
			t.Fatal("function declaration and local result variable were not distinguished")
		}
	}
	first := statements(file, 0)[0].AsLetStatement()
	value := hidden.Locals.Lookup("VALUE")
	if value == nil || first.LExpression.Symbol != value || first.RExpression.AsBinaryExpression().Right.Symbol != value {
		t.Fatal("variable spellings did not resolve to one procedure local")
	}
	if first.RExpression.AsBinaryExpression().Left.Symbol != later.Symbol {
		t.Fatal("reference to a later function did not resolve")
	}
	if later.Locals.Lookup("value") == value || value.Parent != hidden.Symbol {
		t.Fatal("procedure locals leaked between functions")
	}
	if file.SymbolCount != 7 || file.ClassifiableNames.Len() != 3 || !file.ClassifiableNames.Has("value") {
		t.Fatalf("unexpected symbol metadata: count=%d, names=%v", file.SymbolCount, file.ClassifiableNames.Keys())
	}
	var checkParents func(*ast.Node) bool
	checkParents = func(node *ast.Node) bool {
		node.ForEachChild(func(child *ast.Node) bool {
			if child.Parent != node {
				t.Errorf("kind %d has incorrect parent", child.Kind)
			}
			return checkParents(child)
		})
		return false
	}
	checkParents(file.AsNode())
}

func TestAssignmentFlow(t *testing.T) {
	file := bind(t, "Function F()\nx = 1\nx = x + 2\nF = x\nEnd Function")
	fn := file.FunctionDeclarations.Nodes[0]
	stmts := statements(file, 0)
	start := fn.FlowNode
	first := stmts[1].FlowNode
	second := stmts[2].FlowNode
	end := fn.AsFunctionDeclaration().EndFlowNode
	if start.Flags&ast.FlowFlagsStart == 0 || first.Flags&ast.FlowFlagsAssignment == 0 ||
		second.Flags&ast.FlowFlagsAssignment == 0 || end.Flags&ast.FlowFlagsAssignment == 0 ||
		first.Antecedent != start || second.Antecedent != first || end.Antecedent != second {
		t.Fatal("assignments did not form a sequential flow chain")
	}
	if stmts[1].AsLetStatement().RExpression.AsBinaryExpression().Left.FlowNode != first ||
		first.Node != stmts[0].AsLetStatement().LExpression {
		t.Fatal("assignment flow does not describe reads before writes")
	}
	if end.Node.Symbol != fn.AsFunctionDeclaration().ResultSymbol || fn.Flags&ast.NodeFlagsHasImplicitReturn == 0 {
		t.Fatal("function result assignment did not fall through to End Function")
	}
}

func TestBranchesShareProcedureScopeAndMerge(t *testing.T) {
	file := bind(t, "Function F()\nIf condition Then\nx = 1\nElseIf other Then\nx = 2\nElse\nx = 3\nEnd If\nF = x\nEnd Function")
	fn := file.FunctionDeclarations.Nodes[0]
	stmts := statements(file, 0)
	outer := stmts[0].AsIfStatement()
	inner := outer.ElseStatement.AsIfStatement()
	x := fn.Locals.Lookup("x")
	for _, block := range []*ast.Node{outer.ThenStatement, inner.ThenStatement, inner.ElseStatement} {
		assignment := block.AsBlock().Statements.Nodes[0]
		requireReachable(t, assignment, true)
		if assignment.AsLetStatement().LExpression.Symbol != x || block.Locals != nil {
			t.Fatal("If branches introduced block-scoped VBA locals")
		}
	}
	if outer.ThenStatement.FlowNode.Flags&ast.FlowFlagsTrueCondition == 0 ||
		outer.ElseStatement.FlowNode.Flags&ast.FlowFlagsFalseCondition == 0 {
		t.Fatal("branch conditions are missing")
	}
	merge := stmts[1].FlowNode
	if merge.Flags&ast.FlowFlagsBranchLabel == 0 || merge.Antecedents == nil ||
		merge.Antecedents.Next == nil || merge.Antecedents.Next.Next != nil {
		t.Fatal("If/ElseIf paths did not merge")
	}
	if stmts[1].AsLetStatement().RExpression.Symbol != x || len(fn.Locals) != 4 {
		t.Fatal("branch variable was not visible after End If")
	}
}

func TestConstantConditions(t *testing.T) {
	for _, test := range []struct {
		expression    string
		thenReachable bool
	}{
		{"True", true}, {"False", false}, {"Not True", false}, {"Not False", true},
	} {
		t.Run(test.expression, func(t *testing.T) {
			file := bind(t, "Function F()\nIf "+test.expression+" Then\nx = 1\nElse\nx = 2\nEnd If\nF = x\nEnd Function")
			stmt := statements(file, 0)[0].AsIfStatement()
			requireReachable(t, stmt.ThenStatement.AsBlock().Statements.Nodes[0], test.thenReachable)
			requireReachable(t, stmt.ElseStatement.AsBlock().Statements.Nodes[0], !test.thenReachable)
			requireReachable(t, statements(file, 0)[1], true)
		})
	}
}

func TestLogicalOperandsAreEager(t *testing.T) {
	for _, operator := range []string{"And", "Or", "Xor", "Eqv", "Imp"} {
		t.Run(operator, func(t *testing.T) {
			file := bind(t, "Function F()\nIf leftValue "+operator+" rightValue Then F = 1 Else F = 2\nEnd Function")
			stmt := statements(file, 0)[0].AsIfStatement()
			expr := stmt.Expression.AsBinaryExpression()
			if expr.Left.FlowNode != expr.Right.FlowNode || expr.Left.FlowNode.Flags&ast.FlowFlagsStart == 0 ||
				stmt.ThenStatement.FlowNode.Node != stmt.Expression {
				t.Fatal("logical operands incorrectly introduced short-circuit paths")
			}
		})
	}
}

func TestBareFunctionReferenceCreatesCallFlow(t *testing.T) {
	file := bind(t, "Function F()\nF = Later\nEnd Function\nFunction Later()\nLater = 1\nEnd Function")
	fn := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration()
	rhs := statements(file, 0)[0].AsLetStatement().RExpression
	call := fn.EndFlowNode.Antecedent
	if call.Flags&ast.FlowFlagsCall == 0 || call.Node != rhs || call.Antecedent != rhs.FlowNode {
		t.Fatal("implicit parameterless invocation has no call flow")
	}
	if file.FunctionDeclarations.Nodes[1].AsFunctionDeclaration().EndFlowNode.Antecedent.Flags&ast.FlowFlagsStart == 0 {
		t.Fatal("function result variable was treated as a recursive call")
	}
}

func TestForwardGoto(t *testing.T) {
	file := bind(t, "Function F()\nGoTo Target\nx = 1\nTarget: F = 2\nEnd Function")
	stmts := statements(file, 0)
	jump := stmts[0].AsGotoStatement()
	label := stmts[2].AsLabeledStatement()
	if jump.Target != label.Label.Symbol || jump.Label.Symbol != jump.Target ||
		file.FunctionDeclarations.Nodes[0].Locals.Lookup("target") != nil {
		t.Fatal("forward GoTo did not resolve in the label namespace")
	}
	requireReachable(t, stmts[1], false)
	requireReachable(t, stmts[2], true)
	requireReachable(t, label.Statement, true)
}

func TestBackwardGoto(t *testing.T) {
	file := bind(t, "Function F()\nAgain: x = x + 1\nGoTo Again\nF = x\nEnd Function")
	fn := file.FunctionDeclarations.Nodes[0]
	stmts := statements(file, 0)
	if stmts[1].AsGotoStatement().Target != stmts[0].AsLabeledStatement().Label.Symbol {
		t.Fatal("backward GoTo did not resolve")
	}
	requireReachable(t, stmts[0], true)
	requireReachable(t, stmts[1], true)
	requireReachable(t, stmts[2], false)
	if fn.Flags&ast.NodeFlagsHasImplicitReturn != 0 || fn.AsFunctionDeclaration().EndFlowNode.Flags&ast.FlowFlagsUnreachable == 0 {
		t.Fatal("an infinite GoTo loop has a reachable function exit")
	}
}

func TestLateBackEdgeRevivesEarlierLabel(t *testing.T) {
	file := bind(t, "Function F()\nGoTo Later\nEarlier: x = 1\nGoTo Done\nLater: GoTo Earlier\nDone: F = x\nEnd Function")
	for _, stmt := range statements(file, 0) {
		requireReachable(t, stmt, true)
	}
}

func TestDisconnectedGotoCycleRemainsUnreachable(t *testing.T) {
	file := bind(t, "Function F()\nGoTo Done\nDead: x = 1\nGoTo Dead\nDone: F = 2\nEnd Function")
	stmts := statements(file, 0)
	requireReachable(t, stmts[1], false)
	requireReachable(t, stmts[1].AsLabeledStatement().Statement, false)
	requireReachable(t, stmts[2], false)
	requireReachable(t, stmts[3], true)
}

func TestGotoCanEnterDeadBranch(t *testing.T) {
	file := bind(t, "Function F()\nGoTo Inside\nIf False Then\nInside: x = 1\nElse\nx = 2\nEnd If\nF = x\nEnd Function")
	stmts := statements(file, 0)
	conditional := stmts[1].AsIfStatement()
	label := conditional.ThenStatement.AsBlock().Statements.Nodes[0]
	requireReachable(t, stmts[1], false)
	requireReachable(t, label, true)
	requireReachable(t, label.AsLabeledStatement().Statement, true)
	requireReachable(t, conditional.ElseStatement.AsBlock().Statements.Nodes[0], false)
	requireReachable(t, stmts[2], true)
}

func TestNumericAndCombinedLabels(t *testing.T) {
	file := bind(t, "Function F()\n    GoTo 010\n    10 AliasName: F = 1\n    GoTo ALIASNAME\nEnd Function")
	fn := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration()
	stmts := statements(file, 0)
	numeric, named := fn.Labels.Lookup("10"), fn.Labels.Lookup("aliasname")
	label := stmts[1].AsLabeledStatement()
	if numeric == nil || named == nil || numeric == named || numeric.ValueDeclaration != named.ValueDeclaration ||
		stmts[0].AsGotoStatement().Target != numeric || stmts[2].AsGotoStatement().Target != named ||
		label.LineNumber.FlowNode != label.Label.FlowNode {
		t.Fatal("numeric/named labels did not resolve to the shared statement flow")
	}
	requireReachable(t, label.Statement, true)
}

func TestLabelNamespaceIsSeparateFromVariables(t *testing.T) {
	file := bind(t, "Function F()\nvalue = 1\nvalue: value = value + 1\nGoTo value\nEnd Function")
	fn := file.FunctionDeclarations.Nodes[0]
	if fn.Locals.Lookup("value") == fn.AsFunctionDeclaration().Labels.Lookup("value") {
		t.Fatal("label and variable symbols collided")
	}
}

func TestBindingDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name, source string
		code         ast.BindingDiagnosticCode
		related      bool
	}{
		{"duplicate procedure", "Function Foo()\nEnd Function\nFunction fOO()\nEnd Function", ast.BindingDuplicateDeclaration, true},
		{"duplicate label", "Function F()\nMark:\nmARK:\nEnd Function", ast.BindingDuplicateLabel, true},
		{"duplicate line number", "Function F()\n010:\n10:\nEnd Function", ast.BindingDuplicateLabel, true},
		{"undefined label", "Function F()\nGoTo Missing\nEnd Function", ast.BindingUndefinedLabel, false},
		{"cross procedure label", "Function F()\nGoTo Target\nEnd Function\nFunction G()\nTarget:\nEnd Function", ast.BindingUndefinedLabel, false},
		{"assign other function", "Function F()\nOther = 1\nEnd Function\nFunction Other()\nEnd Function", ast.BindingInvalidAssignmentTarget, false},
		{"assign Me", "Function F()\nLet Me = 1\nEnd Function", ast.BindingInvalidAssignmentTarget, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := parser.ParseSourceFile(test.source).AsSourceFile()
			binder.BindSourceFile(file)
			if len(file.BindDiagnostics) != 1 {
				t.Fatalf("diagnostics = %+v, want one", file.BindDiagnostics)
			}
			diagnostic := file.BindDiagnostics[0]
			if diagnostic.Code != test.code || diagnostic.Node == nil || diagnostic.Message == "" ||
				(diagnostic.RelatedNode != nil) != test.related {
				t.Fatalf("unexpected diagnostic: %+v", diagnostic)
			}
			if !file.IsBound() {
				t.Fatal("invalid source was not marked bound")
			}
			binder.BindSourceFile(file)
			if len(file.BindDiagnostics) != 1 {
				t.Fatal("rebinding duplicated diagnostics")
			}
		})
	}
}

func TestBindingIsOnceOnlyAndConcurrent(t *testing.T) {
	file := parser.ParseSourceFile("Function F()\nx = x + 1\nF = x\nEnd Function").AsSourceFile()
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			binder.BindSourceFile(file)
			if !file.IsBound() || file.SymbolCount != 4 {
				t.Error("observed incomplete binding")
			}
		}()
	}
	workers.Wait()
	symbol := file.Locals.Lookup("f")
	flow := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().EndFlowNode
	binder.BindSourceFile(file)
	if file.Locals.Lookup("f") != symbol || file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().EndFlowNode != flow {
		t.Fatal("rebinding replaced existing symbols or flows")
	}
}

func TestBinderPoolsDoNotCorruptEarlierFiles(t *testing.T) {
	first := bind(t, "Function Original()\nx = 1\nOriginal = x\nEnd Function")
	symbol := first.Locals.Lookup("original")
	end := first.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().EndFlowNode
	for i := range 40 {
		file := bind(t, fmt.Sprintf("Function Later%d()\ny = 2\nLater%d = y\nEnd Function", i, i))
		if file.Locals.Lookup("original") != nil {
			t.Fatal("symbol table leaked from another file")
		}
	}
	if symbol.Name != "Original" || symbol.ValueDeclaration != first.FunctionDeclarations.Nodes[0] ||
		end.Node.Symbol != first.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().ResultSymbol {
		t.Fatal("reusing a binder overwrote a previously bound tree")
	}
}

func TestEmptyFileAndEmptyFunction(t *testing.T) {
	file := bind(t, "")
	if file.SymbolCount != 1 || len(file.Locals) != 0 {
		t.Fatal("unexpected empty module symbols")
	}
	file = bind(t, "Function F()\nEnd Function")
	fn := file.FunctionDeclarations.Nodes[0]
	if fn.AsFunctionDeclaration().EndFlowNode != fn.FlowNode || fn.Flags&ast.NodeFlagsHasImplicitReturn == 0 {
		t.Fatal("empty function has no fallthrough flow")
	}
}

func TestBindingDoesNotAffectSyntaxEquality(t *testing.T) {
	source := "Function F()\nAgain: F = 1\nGoTo Again\nEnd Function"
	bound := bind(t, source)
	plain := parser.ParseSourceFile(source)
	if !bound.AsNode().DeeplyEquals(plain) {
		t.Fatal("binding changed syntax equality")
	}
}

func TestIfWithoutElsePreservesFallthrough(t *testing.T) {
	file := bind(t, "Function F()\nIf condition Then F = 1\nF = 2\nEnd Function")
	stmts := statements(file, 0)
	merge := stmts[1].FlowNode
	if merge.Flags&ast.FlowFlagsBranchLabel == 0 || merge.Antecedents == nil || merge.Antecedents.Next == nil ||
		merge.Antecedents.Next.Flow.Flags&ast.FlowFlagsFalseCondition == 0 {
		t.Fatal("If without Else lost its false fallthrough path")
	}
	requireReachable(t, stmts[1], true)
}

func TestExpressionChildrenAndInstanceFlags(t *testing.T) {
	file := bind(t, "Function F()\nobjectValue = Me\nemptyValue = Empty\nnullValue = Null\nnothingValue = Nothing\nF = -(objectValue + 1)\nEnd Function\nFunction G()\nEnd Function")
	fn, other := file.FunctionDeclarations.Nodes[0], file.FunctionDeclarations.Nodes[1]
	if fn.Flags&ast.NodeFlagsContainsThis == 0 || other.Flags&ast.NodeFlagsContainsThis != 0 {
		t.Fatal("instance reference flag was missing or leaked between functions")
	}
	stmts := statements(file, 0)
	operand := ast.SkipParentheses(stmts[4].AsLetStatement().RExpression.AsPrefixUnaryExpression().Operand).AsBinaryExpression().Left
	if operand.Symbol != fn.Locals.Lookup("objectValue") || operand.FlowNode != stmts[4].FlowNode {
		t.Fatal("unary expression operand was not bound")
	}
}

func TestStringTypeSuffixDoesNotCreateAnotherName(t *testing.T) {
	file := bind(t, "Function F$()\nmessage$ = \"hello\"\nF$ = MESSAGE\nEnd Function")
	fn := file.FunctionDeclarations.Nodes[0]
	stmts := statements(file, 0)
	if len(fn.Locals) != 2 || stmts[0].AsLetStatement().LExpression.Symbol != stmts[1].AsLetStatement().RExpression.Symbol ||
		file.Locals.Lookup("F") != fn.Symbol {
		t.Fatal("String type suffix changed identifier identity")
	}
}

func TestFactoryParametersAndDuplicates(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		t.Run(fmt.Sprint(duplicate), func(t *testing.T) {
			f := ast.NodeFactory{}
			parameter := f.NewIdentifier("argument")
			parameters := []*ast.Node{parameter}
			if duplicate {
				parameters = append(parameters, f.NewIdentifier("ARGUMENT"))
			}
			read := f.NewIdentifier("Argument")
			fn := f.NewFunctionDeclaration(ast.ModifierNone, f.NewIdentifier("F"), f.NewNodeList(parameters),
				f.NewBlock(f.NewNodeList([]*ast.Node{f.NewLetStatement(f.NewIdentifier("F"), read)})))
			file := f.NewSourceFile(f.NewNodeList([]*ast.Node{fn})).AsSourceFile()
			binder.BindSourceFile(file)
			if parameter.Parent != fn || read.Symbol != parameter.Symbol || fn.Locals.Lookup("argument") != parameter.Symbol {
				t.Fatal("parameter was not retained or bound in the function scope")
			}
			if duplicate {
				if len(file.BindDiagnostics) != 1 || file.BindDiagnostics[0].Code != ast.BindingDuplicateDeclaration ||
					file.BindDiagnostics[0].RelatedNode != parameter {
					t.Fatal("duplicate parameter was not diagnosed")
				}
			} else if len(file.BindDiagnostics) != 0 {
				t.Fatal(file.BindDiagnostics)
			}
		})
	}
}

func TestSyntaxVisitorCanStopWithoutFollowingBindingCycles(t *testing.T) {
	file := bind(t, "Function F()\nGoTo Mark\nMark: F = 1\nEnd Function\nFunction G()\nEnd Function")
	var visited []*ast.Node
	stopped := file.ForEachChild(func(node *ast.Node) bool {
		visited = append(visited, node)
		return true
	})
	if !stopped || len(visited) != 1 || visited[0] != file.FunctionDeclarations.Nodes[0] {
		t.Fatal("syntax visitor did not stop at the first child")
	}
	visited = nil
	stopped = file.FunctionDeclarations.Nodes[0].ForEachChild(func(child *ast.Node) bool {
		visited = append(visited, child)
		return true
	})
	if !stopped || len(visited) != 1 || visited[0].Kind != ast.KindIdentifier {
		t.Fatal("function child traversal did not start with its name")
	}
}

func ExampleBindSourceFile() {
	file := parser.ParseSourceFile("Function Answer()\nAnswer = 42\nEnd Function").AsSourceFile()
	binder.BindSourceFile(file)
	fmt.Println(file.Locals.Lookup("ANSWER").Name)
	fmt.Println(len(file.BindDiagnostics))
	// Output:
	// Answer
	// 0
}

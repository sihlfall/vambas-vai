package binder

import (
	"sync"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/collections"
	"sihlfall.ch/vambas-vai/core"
)

// Binder declares symbols and builds control-flow graphs for the syntax currently
// supported by the VBA parser. Project binding supplies the outer name scope.
type Binder struct {
	file                   *ast.SourceFile
	project                *projectScope
	intrinsics             ast.SymbolTable
	container              *ast.Node
	currentFlow            *ast.FlowNode
	unreachableFlow        *ast.FlowNode
	labelTargets           map[*ast.Symbol]*ast.FlowLabel
	flowNodes              []*ast.FlowNode
	exitTargets            []exitTarget
	returnTarget           *ast.FlowNode
	gosubContinuations     []*ast.FlowNode
	gosubReturns           *ast.FlowNode
	withTarget             *ast.Node
	symbolCount            int
	classifiableNames      collections.Set[string]
	enumAliases            map[string][]*ast.Symbol
	symbolPool             core.Pool[ast.Symbol]
	flowNodePool           core.Pool[ast.FlowNode]
	flowListPool           core.Pool[ast.FlowList]
	singleDeclarationsPool core.Pool[*ast.Node]
}

var binderPool = sync.Pool{New: func() any { return &Binder{} }}

func getBinder() *Binder { return binderPool.Get().(*Binder) }

func putBinder(b *Binder) {
	// AST nodes retain allocated symbols and flows. Reset the pools as well so
	// a subsequent file cannot overwrite objects belonging to an earlier file.
	*b = Binder{}
	binderPool.Put(b)
}

// BindSourceFile binds an immutable syntax tree at most once, as in typescript-go.
// Binding errors are recorded in file.BindDiagnostics.
func BindSourceFile(file *ast.SourceFile) {
	if !file.IsBound() {
		bindSourceFile(file)
	}
}

func bindSourceFile(file *ast.SourceFile) {
	file.BindOnce(func() {
		b := getBinder()
		defer putBinder(b)
		b.initializeSourceFile(file)
		b.bind(file.AsNode())
		b.finishSourceFile()
	})
}

func (b *Binder) initializeSourceFile(file *ast.SourceFile) {
	b.file = file
	if b.project != nil {
		b.intrinsics = b.project.intrinsics
	} else {
		b.intrinsics = newIntrinsicScope()
	}
	b.unreachableFlow = b.newFlowNode(ast.FlowFlagsUnreachable)
	setParents(file.AsNode(), nil)
	file.Locals = make(ast.SymbolTable)
	flags := ast.SymbolFlagsValueModule
	if file.ModuleKind.IsClass() {
		flags = ast.SymbolFlagsClass
	}
	file.Symbol = b.newSymbol(flags, file.ModuleName, file.AsNode(), nil)
	file.Symbol.Exports = make(ast.SymbolTable)
	if file.ModuleKind.IsClass() {
		file.Symbol.Members = file.Locals
	}
	b.container = file.AsNode()
	if file.Statements != nil {
		for _, declaration := range file.Statements.Nodes {
			switch {
			case ast.IsProcedureDeclaration(declaration):
				b.declareFunction(declaration)
			case declaration.Kind == ast.KindVariableStatement:
				b.predeclareVariables(declaration)
			case declaration.Kind == ast.KindTypeDeclaration || declaration.Kind == ast.KindEnumDeclaration:
				b.declareType(declaration)
			case declaration.Kind == ast.KindOptionExplicitStatement:
				file.OptionExplicit = true
			case declaration.Kind == ast.KindOptionBaseStatement:
				file.OptionBase = declaration.AsOptionBaseStatement().Base
			case declaration.Kind == ast.KindOptionPrivateModuleStatement:
				file.OptionPrivateModule = true
			}
		}
	}
}

func (b *Binder) finishSourceFile() {
	b.file.SymbolCount = b.symbolCount
	b.file.ClassifiableNames = b.classifiableNames
}

func setParents(node, parent *ast.Node) {
	if node == nil {
		return
	}
	node.Parent = parent
	node.ForEachChild(func(child *ast.Node) bool {
		setParents(child, node)
		return false
	})
}

func (b *Binder) bind(node *ast.Node) bool {
	if node == nil {
		return false
	}
	if node.Kind == ast.KindLabeledStatement {
		b.bindLabeledStatement(node)
		return false
	}
	node.FlowNode = b.currentFlow
	switch node.Kind {
	case ast.KindSourceFile:
		node.ForEachChild(b.bind)
	case ast.KindFunctionDeclaration, ast.KindSubDeclaration, ast.KindGetAccessor, ast.KindLetAccessor, ast.KindSetAccessor:
		b.bindFunction(node)
	case ast.KindVariableStatement:
		b.bind(node.AsVariableStatement().DeclarationList)
	case ast.KindVariableDeclaration:
		b.bindVariableDeclaration(node)
	case ast.KindTypeDeclaration, ast.KindEnumDeclaration:
		b.bindTypeDeclaration(node)
	case ast.KindReDimStatement:
		b.bindReDim(node)
	case ast.KindEraseStatement:
		for _, target := range node.AsEraseStatement().Expressions.Nodes {
			b.bind(target)
		}
		if b.currentFlow != nil && b.currentFlow != b.unreachableFlow {
			b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsArrayMutation, node, b.currentFlow)
		}
	case ast.KindTypeReference:
		b.bindTypeAnnotation(node, false)
	case ast.KindImplementsStatement:
		b.resolveTypeName(node.AsImplementsStatement().Interface)
	case ast.KindOptionExplicitStatement, ast.KindOptionBaseStatement, ast.KindOptionPrivateModuleStatement, ast.KindAttributeStatement:
		// The module prepass handles the option; an invalid local option has no effect.
	case ast.KindIdentifier:
		b.bindIdentifier(node)
		b.bindBareInvocation(node)
	case ast.KindLetStatement, ast.KindSetStatement:
		b.bindAssignment(node)
	case ast.KindMidAssignmentStatement:
		n := node.AsMidAssignmentStatement()
		b.bind(n.Target)
		b.bind(n.Start)
		b.bind(n.Length)
		b.bind(n.Expression)
		if ast.IsConstantSymbol(n.Target.Symbol) {
			b.report(ast.BindingAssignmentToConstant, n.Target, nil, "Cannot modify a constant with Mid")
		} else if b.currentFlow != nil && b.currentFlow != b.unreachableFlow {
			// Mid modifies bound variable storage, never invokes a property setter.
			b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsAssignment, n.Target, b.currentFlow)
		}
	case ast.KindPropertyAccessExpression:
		b.bindPropertyAccess(node)
	case ast.KindNamedArgument:
		// The label is parameter syntax, not a value reference.
		b.bind(node.AsNamedArgument().Expression)
	case ast.KindCallExpression:
		b.bindCallExpression(node)
	case ast.KindNewExpression:
		b.bindNewExpression(node)
	case ast.KindWithStatement:
		b.bindWithStatement(node)
	case ast.KindIfStatement:
		b.bindIfStatement(node)
	case ast.KindGotoStatement:
		b.bindGotoStatement(node)
	case ast.KindGosubStatement:
		b.bindGosubStatement(node)
	case ast.KindGosubReturnStatement:
		if b.gosubReturns == nil {
			b.report(ast.BindingInvalidValueReference, node, nil, "Return requires a procedure")
		} else {
			b.addAntecedent(b.gosubReturns, b.currentFlow)
		}
		b.currentFlow = b.unreachableFlow
	case ast.KindOnErrorStatement:
		if target := node.AsOnErrorStatement().Target; target != nil {
			flow := b.currentFlow
			b.bind(target)
			b.currentFlow = flow
		}
	case ast.KindResumeStatement:
		if target := node.AsResumeStatement().Target; target != nil {
			b.bind(target)
		}
		b.currentFlow = b.unreachableFlow
	case ast.KindErrorStatement:
		b.bind(node.AsErrorStatement().Expression)
		b.currentFlow = b.unreachableFlow
	case ast.KindOnGotoStatement, ast.KindOnGosubStatement:
		n := node.AsOnGotoStatement()
		b.bind(n.Expression)
		flow := b.currentFlow
		for _, destination := range n.Destinations.Nodes {
			b.currentFlow = flow
			b.bind(destination)
		}
		b.currentFlow = flow // Zero/out-of-list indices continue to the next statement.
		if node.Kind == ast.KindOnGosubStatement {
			b.gosubContinuation(flow)
		}
	case ast.KindWhileStatement:
		b.bindWhileStatement(node)
	case ast.KindDoStatement:
		b.bindDoStatement(node)
	case ast.KindForStatement, ast.KindForEachStatement:
		b.bindForStatement(node)
	case ast.KindSelectStatement:
		b.bindSelectStatement(node)
	case ast.KindExitStatement:
		b.bindExitStatement(node)
	case ast.KindEndStatement:
		b.currentFlow = b.unreachableFlow
	case ast.KindMeExpression:
		b.container.Flags |= ast.NodeFlagsContainsThis
		if b.project != nil {
			if b.file.ModuleKind.IsClass() {
				node.Symbol = b.file.Symbol
			} else {
				b.report(ast.BindingInvalidValueReference, node, nil, "Me requires a class, document, or form module")
			}
		}
	default:
		// VBA logical operators evaluate both operands. They do not introduce
		// TypeScript's &&/|| short-circuit paths.
		node.ForEachChild(b.bind)
	}
	return false
}

func (b *Binder) bindFunction(node *ast.Node) {
	fn := node.AsFunctionDeclaration()
	saveContainer, saveFlow := b.container, b.currentFlow
	saveTargets, saveNodes := b.labelTargets, b.flowNodes
	saveExits, saveReturn := b.exitTargets, b.returnTarget
	saveWith := b.withTarget
	saveGosubs, saveGosubReturns := b.gosubContinuations, b.gosubReturns
	b.gosubContinuations = nil
	b.gosubReturns = nil
	b.container = node
	b.labelTargets = make(map[*ast.Symbol]*ast.FlowLabel)
	b.flowNodes = nil
	b.exitTargets = nil
	b.withTarget = nil
	node.Locals = make(ast.SymbolTable)
	fn.Labels = make(ast.SymbolTable)
	// Only Function has a result variable. Sub shares the signature/body machinery.
	if node.Kind == ast.KindFunctionDeclaration || node.Kind == ast.KindGetAccessor {
		fn.ResultSymbol = b.newSymbol(ast.SymbolFlagsFunctionScopedVariable, fn.Name().AsIdentifier().Text, node, node.Symbol)
		if fn.ResultSymbol.Name != "" {
			node.Locals[ast.SymbolNameKey(fn.ResultSymbol.Name)] = fn.ResultSymbol
		}
	}
	if fn.Parameters != nil {
		for _, parameter := range fn.Parameters.Nodes {
			b.declareParameter(parameter)
		}
	}
	b.predeclareVariables(fn.Body)
	b.predeclareReDims(fn.Body)
	// Annotations are type-space syntax. Defaults are compile-time expressions,
	// not executable assignments and not references to procedure locals.
	if fn.Parameters != nil {
		for _, parameter := range fn.Parameters.Nodes {
			if parameter.Kind != ast.KindParameter {
				continue
			}
			decl := parameter.AsParameterDeclaration()
			b.bindTypeAnnotation(decl.Type, true)
			b.bindConstantExpression(decl.Initializer, true)
		}
	}
	b.bindTypeAnnotation(fn.Type, true)
	b.collectLabels(fn.Body)
	start := b.newFlowNode(ast.FlowFlagsStart)
	start.Node = node
	node.FlowNode = start
	b.currentFlow = start
	b.returnTarget = b.newFlowNode(ast.FlowFlagsBranchLabel)
	b.gosubReturns = b.newFlowNode(ast.FlowFlagsBranchLabel)
	b.bind(fn.Body)
	fn.EndFlowNode = b.currentFlow
	b.addAntecedent(b.returnTarget, b.currentFlow)
	fn.ReturnFlowNode = b.finishFlowLabel(b.returnTarget)
	for _, continuation := range b.gosubContinuations {
		b.addAntecedent(continuation, b.gosubReturns)
	}
	b.finishFunctionFlow(node, start)
	if fn.EndFlowNode.Flags&ast.FlowFlagsUnreachable == 0 {
		node.Flags |= ast.NodeFlagsHasImplicitReturn
	}
	b.container, b.currentFlow = saveContainer, saveFlow
	b.labelTargets, b.flowNodes = saveTargets, saveNodes
	b.exitTargets, b.returnTarget = saveExits, saveReturn
	b.withTarget = saveWith
	b.gosubContinuations, b.gosubReturns = saveGosubs, saveGosubReturns
}

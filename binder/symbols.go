package binder

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/scanner"
	"sihlfall.ch/vambas-vai/stdlib"
)

func (b *Binder) newSymbol(flags ast.SymbolFlags, name string, declaration *ast.Node, parent *ast.Symbol) *ast.Symbol {
	b.symbolCount++
	symbol := b.symbolPool.New()
	symbol.Flags, symbol.Name, symbol.Parent = flags, name, parent
	symbol.Declarations = b.singleDeclarationsPool.NewSlice1(declaration)
	symbol.ValueDeclaration = declaration
	if name != "" && flags&(ast.SymbolFlagsFunction|ast.SymbolFlagsFunctionScopedVariable|ast.SymbolFlagsAccessor|ast.SymbolFlagsProperty|ast.SymbolFlagsEnumMember|ast.SymbolFlagsType) != 0 {
		b.classifiableNames.Add(ast.SymbolNameKey(name))
	}
	return symbol
}

func (b *Binder) declareFunction(node *ast.Node) {
	if ast.IsPropertyAccessor(node) {
		b.declareAccessor(node)
		return
	}
	fn := node.AsFunctionDeclaration()
	name := fn.Name().AsIdentifier().Text
	key := ast.SymbolNameKey(name)
	symbol := b.newSymbol(ast.SymbolFlagsFunction, name, node, b.file.Symbol)
	node.Symbol, fn.Name().Symbol = symbol, symbol
	if name == "" {
		return
	}
	if previous := b.file.Locals[key]; previous != nil {
		b.report(ast.BindingDuplicateDeclaration, fn.Name(), previous.ValueDeclaration,
			fmt.Sprintf("Duplicate procedure name %q", name))
		return
	}
	b.file.Locals[key] = symbol
	if fn.Modifiers()&ast.ModifierPrivate == 0 {
		b.file.Symbol.Exports[key] = symbol
	}
}

func (b *Binder) declareParameter(node *ast.Node) {
	b.declareLocal(node, ast.DeclarationName(node))
}

func (b *Binder) bindIdentifier(node *ast.Node) {
	name := node.AsIdentifier().Text
	if name == "" {
		return
	}
	symbol := b.container.Locals.Lookup(name)
	if symbol == nil || b.container.Kind == ast.KindSourceFile {
		var ambiguous bool
		symbol, ambiguous = b.moduleValue(b.file, name, node)
		if ambiguous {
			return
		}
	}
	if symbol == nil && b.project != nil {
		var ambiguous bool
		symbol, ambiguous = b.lookupProjectName(name, node)
		if ambiguous {
			return
		}
	}
	if symbol == nil {
		symbol = b.intrinsics.Lookup(name)
	}
	// F(x) invokes the procedure even inside F. A bare F still denotes its
	// result variable, and F(i) on an assignment's left can index that result.
	call := invocationForCallee(node)
	if call != nil && symbol != nil && !isAssignmentTarget(call) && ast.IsProcedureDeclaration(b.container) && symbol == b.container.AsFunctionDeclaration().ResultSymbol {
		symbol = b.container.Symbol
	}
	if symbol == nil {
		if b.file.OptionExplicit {
			b.report(ast.BindingUndeclaredName, node, nil, fmt.Sprintf("Variable %q is not declared", name))
			return
		}
		symbol = b.newSymbol(ast.SymbolFlagsFunctionScopedVariable, name, node, b.container.Symbol)
		b.container.Locals[ast.SymbolNameKey(name)] = symbol
	}
	node.Symbol = symbol
	b.checkValueReference(node, symbol)
}

func (b *Binder) checkValueReference(node *ast.Node, symbol *ast.Symbol) {
	call := invocationForCallee(node)
	if b.project != nil && symbol.Flags&ast.SymbolFlagsValueModule != 0 && symbol.Library != stdlib.Err && !isNamespaceQualifier(node) {
		b.report(ast.BindingInvalidValueReference, node, nil, "A project or standard module name must qualify a member")
	}
	if symbol.Flags&ast.SymbolFlagsUserDefinedType != 0 || symbol.Flags&ast.SymbolFlagsRegularEnum != 0 && !isNamespaceQualifier(node) {
		b.report(ast.BindingInvalidValueReference, node, nil, "A Type or Enum name cannot be used as a value")
	}
	if b.project != nil && symbol.Flags&ast.SymbolFlagsClass != 0 && node.Kind != ast.KindMeExpression {
		if file := ast.SourceFileOf(symbol.ValueDeclaration); file != nil && !file.PredeclaredID {
			b.report(ast.BindingInvalidValueReference, node, nil, "A class name requires an instance in value expressions")
		}
	}
	if symbol.Flags&ast.SymbolFlagsGetAccessor != 0 && !isAssignmentReference(node) {
		b.checkAccessorVisibility(node, symbol, ast.KindGetAccessor)
	}
	if symbol.Flags&ast.SymbolFlagsAccessor != 0 && symbol.Flags&ast.SymbolFlagsGetAccessor == 0 && !isAssignmentReference(node) {
		b.report(ast.BindingInvalidValueReference, node, nil, "A write-only property cannot be read")
	}
	if symbol.ValueDeclaration != nil && symbol.ValueDeclaration.Kind == ast.KindSubDeclaration {
		statementCall := call != nil && (call.Parent != nil && call.Parent.Kind == ast.KindCallStatement || isAssignmentTarget(call))
		if !isAssignmentTarget(node) && !statementCall && !isAddressOfOperand(node) {
			b.report(ast.BindingInvalidValueReference, node, nil, "A Sub procedure cannot be used as a value")
		}
	}
}

func (b *Binder) report(code ast.BindingDiagnosticCode, node, related *ast.Node, message string) {
	relatedLoc := core.UndefinedTextRange()
	if related != nil {
		relatedLoc = b.errorSpan(related)
	}
	b.file.BindDiagnostics = append(b.file.BindDiagnostics, ast.BindingDiagnostic{
		File: b.file, RelatedFile: ast.SourceFileOf(related),
		Code: code, Message: message, Node: node, RelatedNode: related,
		Loc: b.errorSpan(node), RelatedLoc: relatedLoc,
	})
}

// Like typescript-go's diagnostic span helpers, skip leading trivia when
// underlining a node, while retaining that trivia in its syntax range.
func (b *Binder) errorSpan(node *ast.Node) core.TextRange {
	if name := ast.DeclarationName(node); name != nil {
		node = name
	}
	loc := node.Loc
	file := ast.SourceFileOf(node)
	if file == nil {
		file = b.file
	}
	if !loc.IsValid() || loc.End() > len(file.Text) || loc.Len() == 0 {
		return loc
	}
	state := scanner.ScannerState{Input: file.Text[loc.Pos():loc.End()]}
	state.Scan(scanner.ScanFlagNone)
	return core.NewTextRange(loc.Pos()+state.TokenStart(), loc.End())
}

func labelName(node *ast.Node) string {
	if node.Kind == ast.KindIdentifier {
		return node.AsIdentifier().Text
	}
	line := node.AsLineNumber()
	name := strings.TrimLeft(line.Text, "0")
	if name == "" {
		name = "0"
	}
	if line.HasMinus() && name != "0" {
		name = "-" + name
	}
	return name
}

func (b *Binder) collectLabels(node *ast.Node) {
	if node == nil {
		return
	}
	if node.Kind == ast.KindLabeledStatement {
		stmt := node.AsLabeledStatement()
		flow := b.newFlowNode(ast.FlowFlagsBranchLabel)
		flow.Node = node
		for _, label := range []*ast.Node{stmt.LineNumber, stmt.Label} {
			if label == nil {
				continue
			}
			name := labelName(label)
			symbol := b.newSymbol(ast.SymbolFlagsLabel, name, node, b.container.Symbol)
			label.Symbol = symbol
			b.labelTargets[symbol] = flow
			labels := b.container.AsFunctionDeclaration().Labels
			key := ast.SymbolNameKey(name)
			if previous := labels[key]; previous != nil {
				b.report(ast.BindingDuplicateLabel, label, previous.ValueDeclaration,
					fmt.Sprintf("Duplicate statement label %q", name))
			} else {
				labels[key] = symbol
			}
		}
	}
	node.ForEachChild(func(child *ast.Node) bool {
		b.collectLabels(child)
		return false
	})
}

func (b *Binder) bindLabeledStatement(node *ast.Node) {
	stmt := node.AsLabeledStatement()
	label := stmt.LineNumber
	if label == nil {
		label = stmt.Label
	}
	target := b.labelTargets[label.Symbol]
	b.addAntecedent(target, b.currentFlow)
	// Retain the label even if no predecessor is known yet: later GoTo statements
	// may introduce back edges or jump into a syntactically unreachable block.
	b.currentFlow = target
	node.FlowNode = target
	for _, name := range []*ast.Node{stmt.LineNumber, stmt.Label} {
		if name != nil {
			name.FlowNode = target
		}
	}
	b.bind(stmt.Statement)
}

func (b *Binder) bindGotoStatement(node *ast.Node) {
	if b.container == nil || !ast.IsProcedureDeclaration(b.container) {
		b.report(ast.BindingInvalidValueReference, node, nil, "A statement jump requires a procedure")
		return
	}
	stmt := node.AsGotoStatement()
	name := stmt.LineNumber
	if name == nil {
		name = stmt.Label
	}
	name.FlowNode = b.currentFlow
	if (name.Kind == ast.KindIdentifier && name.AsIdentifier().Text == "") ||
		(name.Kind == ast.KindLineNumber && name.AsLineNumber().Text == "") {
		return // Missing targets already have a parse diagnostic.
	}
	target := b.container.AsFunctionDeclaration().Labels.Lookup(labelName(name))
	stmt.Target, name.Symbol = target, target
	if target == nil {
		b.report(ast.BindingUndefinedLabel, name, nil, fmt.Sprintf("Undefined statement label %q", labelName(name)))
	} else {
		b.addAntecedent(b.labelTargets[target], b.currentFlow)
	}
	b.currentFlow = b.unreachableFlow
}

package binder

import (
	"fmt"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
)

type projectScope struct {
	symbol      *ast.Symbol
	globals     map[string][]*ast.Symbol
	enumAliases map[*ast.SourceFile]map[string][]*ast.Symbol
	intrinsics  ast.SymbolTable
}

// BindProject binds fresh, exclusively owned trees in two phases: declare every
// module, then bind bodies against the completed project scope. Callers must not
// publish the trees until this returns. compiler.NewProgram provides this lifecycle.
// Already-bound or repeated trees are rejected before any file is modified.
func BindProject(name string, files []*ast.SourceFile) (*ast.Symbol, error) {
	seen := make(map[*ast.SourceFile]bool)
	for _, file := range files {
		if file == nil || file.IsBound() || seen[file] {
			return nil, fmt.Errorf("project binding requires distinct, fresh source files")
		}
		seen[file] = true
	}
	project := &projectScope{
		symbol:      &ast.Symbol{Flags: ast.SymbolFlagsValueModule, Name: name, Exports: make(ast.SymbolTable)},
		globals:     make(map[string][]*ast.Symbol),
		enumAliases: make(map[*ast.SourceFile]map[string][]*ast.Symbol),
		intrinsics:  newIntrinsicScope(),
	}
	binders := make([]*Binder, 0, len(files))
	defer func() {
		for _, b := range binders {
			putBinder(b)
		}
	}()
	for _, file := range files {
		b := getBinder()
		binders = append(binders, b)
		b.project = project
		b.initializeSourceFile(file)
		project.enumAliases[file] = b.enumAliases
		file.Symbol.Parent = project.symbol
		if !validModuleName(file.ModuleName) || file.ModuleKind > ast.ModuleKindUserForm {
			b.report(ast.BindingInvalidModule, moduleNameNode(file), nil, "A module requires a valid kind and an identifier of at most 31 characters")
		}
		if strings.EqualFold(file.ModuleName, name) {
			b.report(ast.BindingInvalidModule, moduleNameNode(file), nil, "A module cannot have the same name as its project")
		}
		key := ast.SymbolNameKey(file.ModuleName)
		if previous := project.symbol.Exports[key]; previous != nil {
			b.report(ast.BindingDuplicateModule, moduleNameNode(file), moduleNameNode(ast.SourceFileOf(previous.ValueDeclaration)), fmt.Sprintf("Duplicate module name %q", file.ModuleName))
		} else {
			project.symbol.Exports[key] = file.Symbol
		}
		for key, symbol := range file.Symbol.Exports {
			if !file.ModuleKind.IsClass() || symbol.Flags&(ast.SymbolFlagsRegularEnum|ast.SymbolFlagsEnumMember|ast.SymbolFlagsUserDefinedType) != 0 {
				project.globals[key] = append(project.globals[key], symbol)
			}
		}
		for key, aliases := range b.enumAliases {
			for _, member := range aliases {
				if member != file.Symbol.Exports[key] && member.Parent.ValueDeclaration.AsEnumDeclaration().Modifiers&ast.ModifierPrivate == 0 {
					project.globals[key] = append(project.globals[key], member)
				}
			}
		}
	}
	for _, b := range binders {
		b.file.BindOnce(func() {
			b.bind(b.file.AsNode())
			b.finishSourceFile()
		})
	}
	return project.symbol, nil
}

func moduleNameNode(file *ast.SourceFile) *ast.Node {
	var result *ast.Node
	if file.Statements != nil {
		for _, node := range file.Statements.Nodes {
			if node.Kind == ast.KindAttributeStatement {
				a := node.AsAttributeStatement()
				if a.Name.Kind == ast.KindIdentifier && ast.SymbolNameKey(a.Name.AsIdentifier().Text) == "vb_name" && len(a.Values.Nodes) != 0 {
					result = a.Values.Nodes[0]
				}
			}
		}
	}
	if result != nil {
		return result
	}
	return file.AsNode()
}

func validModuleName(name string) bool {
	if len(name) == 0 || len(name) > 31 {
		return false
	}
	for i, ch := range name {
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || i > 0 && (ch >= '0' && ch <= '9' || ch == '_') {
			continue
		}
		return false
	}
	return true
}

// The Boolean distinguishes an ambiguous reference from an absent name. An
// ambiguity must not introduce an implicit local when Option Explicit is absent.
func (b *Binder) lookupProjectName(name string, node *ast.Node) (*ast.Symbol, bool) {
	key := ast.SymbolNameKey(name)
	if candidates := b.project.globals[key]; len(candidates) != 0 {
		if len(candidates) > 1 {
			b.report(ast.BindingAmbiguousName, node, candidates[0].ValueDeclaration, fmt.Sprintf("Project name %q is ambiguous; qualify it with its module", name))
			return nil, true
		}
		return candidates[0], false
	}
	if symbol := b.project.symbol.Exports[key]; symbol != nil {
		return symbol, false
	}
	if key == ast.SymbolNameKey(b.project.symbol.Name) {
		return b.project.symbol, false
	}
	return nil, false
}

func isNamespaceQualifier(node *ast.Node) bool {
	return node.Parent != nil && node.Parent.Kind == ast.KindPropertyAccessExpression && node.Parent.AsPropertyAccessExpression().Expression == node && node.Parent.AsPropertyAccessExpression().OperatorToken.Kind == ast.KindDOT
}

func (b *Binder) bindBareInvocation(node *ast.Node) {
	if node.Symbol != nil && node.Symbol.Flags&(ast.SymbolFlagsFunction|ast.SymbolFlagsGetAccessor) != 0 && !isAssignmentReference(node) && invocationForCallee(node) == nil && !isAddressOfOperand(node) && b.currentFlow != nil && b.currentFlow != b.unreachableFlow {
		b.currentFlow = b.newFlowNodeEx(ast.FlowFlagsCall, node, b.currentFlow)
	}
}

func (b *Binder) bindPropertyAccess(node *ast.Node) {
	expression := node.AsPropertyAccessExpression()
	receiver := expression.Expression
	if receiver != nil {
		b.bind(receiver)
	} else if b.withTarget != nil {
		expression.WithTarget = b.withTarget
		receiver = b.withTarget.AsWithStatement().Expression
	} else if expression.Name.AsIdentifier().Text != "" {
		b.report(ast.BindingWithBlockExpected, node, nil, "An omitted member receiver requires an enclosing With block")
	}
	b.bind(expression.OperatorToken)
	expression.Name.FlowNode = b.currentFlow
	if expression.OperatorToken.Kind != ast.KindDOT || expression.Name.AsIdentifier().Text == "" {
		return // Bang/default-member lookup needs type checking.
	}
	if owner := b.receiverSymbol(receiver); owner != nil {
		node.Symbol = b.lookupMember(owner, expression.Name)
		expression.Name.Symbol = node.Symbol
		if node.Symbol != nil {
			b.checkValueReference(node, node.Symbol)
			b.bindBareInvocation(node)
		}
	}
}

func (b *Binder) lookupMember(owner *ast.Symbol, name *ast.Node) *ast.Symbol {
	text := name.AsIdentifier().Text
	if b.isAmbientContainer(owner) {
		if symbol := owner.Exports.Lookup(text); symbol != nil {
			return symbol
		}
	} else if owner.Flags&(ast.SymbolFlagsUserDefinedType|ast.SymbolFlagsRegularEnum) != 0 {
		if symbol := owner.Members.Lookup(text); symbol != nil {
			return symbol
		}
	} else if b.project != nil && owner == b.project.symbol {
		if symbol := owner.Exports.Lookup(text); symbol != nil {
			return symbol
		}
		// VBA also permits project-qualified public standard-module members.
		if candidates := b.project.globals[ast.SymbolNameKey(text)]; len(candidates) == 1 {
			return candidates[0]
		} else if len(candidates) > 1 {
			b.report(ast.BindingAmbiguousName, name, candidates[0].ValueDeclaration, fmt.Sprintf("Project member %q is ambiguous", text))
			return nil
		}
	} else if file := ast.SourceFileOf(owner.ValueDeclaration); file != nil {
		if symbol, ambiguous := b.moduleValue(file, text, name); ambiguous {
			return nil
		} else if symbol != nil {
			if owner.Flags&ast.SymbolFlagsClass != 0 && symbol.Flags&(ast.SymbolFlagsRegularEnum|ast.SymbolFlagsEnumMember|ast.SymbolFlagsUserDefinedType) != 0 {
				b.report(ast.BindingMemberNotFound, name, nil, "Types and enum constants are not class instance members")
				return nil
			}
			return symbol
		}
		if file == b.file {
			if symbol := file.Locals.Lookup(text); symbol != nil {
				return symbol
			}
		} else {
			if symbol := owner.Exports.Lookup(text); symbol != nil {
				return symbol
			}
			if symbol := file.Locals.Lookup(text); symbol != nil {
				b.report(ast.BindingInaccessibleMember, name, symbol.ValueDeclaration, fmt.Sprintf("Member %q is private to module %q", text, file.ModuleName))
				return nil
			}
		}
	}
	b.report(ast.BindingMemberNotFound, name, nil, fmt.Sprintf("Member %q does not exist in %q", text, owner.Name))
	return nil
}

// Resolve only syntactically known receiver classes. Variant/Object receivers,
// assignment-based inference, default members and external libraries stay deferred.
func (b *Binder) receiverSymbol(node *ast.Node) *ast.Symbol {
	node = ast.SkipParentheses(node)
	if node == nil {
		return nil
	}
	if node.Kind == ast.KindCallExpression {
		if target := ast.SkipParentheses(node.AsCallExpression().Expression); target != nil {
			return b.resultClass(target.Symbol)
		}
		return nil
	}
	if node.Kind == ast.KindNewExpression {
		return node.Symbol
	}
	symbol := node.Symbol
	if symbol == nil {
		return nil
	}
	if symbol.Flags&(ast.SymbolFlagsValueModule|ast.SymbolFlagsRegularEnum) != 0 {
		return symbol
	}
	if symbol.Flags&ast.SymbolFlagsClass != 0 {
		file := ast.SourceFileOf(symbol.ValueDeclaration)
		if node.Kind == ast.KindMeExpression || file != nil && file.PredeclaredID {
			return symbol
		}
		return nil
	}
	if isArraySymbol(symbol) {
		return nil // A bare array has no instance members.
	}
	return b.resultClass(symbol)
}

func (b *Binder) resultClass(symbol *ast.Symbol) *ast.Symbol {
	if symbol == nil || symbol.ValueDeclaration == nil {
		return nil
	}
	decl := symbol.ValueDeclaration
	var annotation *ast.Node
	switch {
	case decl.Kind == ast.KindVariableDeclaration || decl.Kind == ast.KindFieldDeclaration:
		annotation = decl.AsVariableDeclaration().Type
	case decl.Kind == ast.KindParameter:
		annotation = decl.AsParameterDeclaration().Type
	case ast.IsProcedureDeclaration(decl):
		annotation = decl.AsFunctionDeclaration().Type
	}
	if annotation != nil && annotation.Kind == ast.KindTypeReference {
		// Resolve in the declaration's module, before that file's bodies have
		// necessarily been visited. Type names never use value/local lookup.
		s := b.resolveTypeName(annotation.AsTypeReference().TypeName)
		if s != nil && s.Flags&(ast.SymbolFlagsClass|ast.SymbolFlagsUserDefinedType) != 0 {
			return s
		}
	}
	return nil
}

func (b *Binder) resolveClassName(node *ast.Node) *ast.Symbol {
	symbol := b.resolveTypeName(node)
	if symbol != nil && symbol.Flags&ast.SymbolFlagsClass != 0 {
		node.Symbol = symbol
		return symbol
	}
	return nil
}

func (b *Binder) checkAccessorVisibility(node *ast.Node, symbol *ast.Symbol, kind ast.Kind) bool {
	accessor := ast.PropertyAccessor(symbol, kind)
	if accessor != nil && b.project != nil && ast.SourceFileOf(accessor) != b.file && accessor.AsFunctionDeclaration().Modifiers()&ast.ModifierPrivate != 0 {
		b.report(ast.BindingInaccessibleMember, node, accessor, "The requested property accessor is private to its module")
		return false
	}
	return true
}

func (b *Binder) resolveConstantMember(node *ast.Node, moduleOnly bool) *ast.Symbol {
	if node.Kind == ast.KindIdentifier {
		name := node.AsIdentifier().Text
		if !moduleOnly {
			node.Symbol = b.container.Locals.Lookup(name)
		}
		if node.Symbol == nil {
			var ambiguous bool
			node.Symbol, ambiguous = b.moduleValue(b.file, name, node)
			if ambiguous {
				return nil
			}
		}
		if node.Symbol == nil && b.project != nil {
			var ambiguous bool
			node.Symbol, ambiguous = b.lookupProjectName(name, node)
			if ambiguous {
				return nil
			}
		}
		if node.Symbol == nil {
			node.Symbol = b.intrinsics.Lookup(name)
		}
		if node.Symbol == nil {
			b.report(ast.BindingConstantExpressionExpected, node, nil, "Constant expression expected")
		}
		return node.Symbol
	}
	if node.Kind == ast.KindPropertyAccessExpression {
		member := node.AsPropertyAccessExpression()
		if member.Expression != nil && member.OperatorToken.Kind == ast.KindDOT {
			owner := b.resolveConstantMember(member.Expression, moduleOnly)
			if owner != nil {
				if b.project != nil && owner == b.project.symbol || owner.Flags&(ast.SymbolFlagsValueModule|ast.SymbolFlagsRegularEnum) != 0 {
					node.Symbol = b.lookupMember(owner, member.Name)
					member.Name.Symbol = node.Symbol
					return node.Symbol
				}
				b.report(ast.BindingConstantExpressionExpected, node, nil, "Constant expression expected")
			}
			return nil
		}
	}
	b.report(ast.BindingConstantExpressionExpected, node, nil, "Constant expression expected")
	return nil
}

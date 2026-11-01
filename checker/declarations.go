package checker

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ast"
)

func (c *Checker) typeOfSymbol(symbol *ast.Symbol) *Type {
	if symbol == nil {
		return c.intrinsic[TypeError] // Binding already diagnosed an unresolved name.
	}
	if c.constants[symbol] == 3 {
		return c.intrinsic[TypeError]
	}
	if t := c.symbolTypes[symbol]; t != nil {
		return t
	}
	if ast.IsConstantSymbol(symbol) && symbol.Flags&ast.SymbolFlagsEnumMember == 0 && c.constants[symbol] == 0 {
		c.resolveConstant(symbol, symbol.ValueDeclaration)
		return c.typeOfSymbol(symbol)
	}
	var t *Type
	decl := symbol.ValueDeclaration
	switch {
	case symbol.Flags&ast.SymbolFlagsEnumMember != 0:
		t = c.intrinsic[TypeLong]
		// Ambient enum declarations are outside source-file traversal. Prepare
		// referenced member values now, keeping published checker queries pure.
		if symbol.Library != 0 {
			c.numericConstant(symbol)
		}
	case symbol.Flags&ast.SymbolFlagsUserDefinedType != 0:
		t = &Type{Kind: TypeStructure, Symbol: symbol}
	case symbol.Flags&ast.SymbolFlagsRegularEnum != 0:
		t = &Type{Kind: TypeModule, Symbol: symbol} // Qualification only, never an enum runtime value.
	case symbol.Flags&ast.SymbolFlagsClass != 0:
		t = &Type{Kind: TypeClass, Symbol: symbol}
	case symbol.Flags&ast.SymbolFlagsValueModule != 0:
		t = &Type{Kind: TypeModule, Symbol: symbol}
	case symbol.Flags&ast.SymbolFlagsFunction != 0:
		t = &Type{Kind: TypeProcedure, Symbol: symbol}
		c.symbolTypes[symbol] = t
		if ast.IsProcedureDeclaration(decl) {
			t.Signatures = []*Signature{c.signatureOfDeclaration(decl)}
		}
	case symbol.Flags&ast.SymbolFlagsAccessor != 0:
		if getter := ast.PropertyAccessor(symbol, ast.KindGetAccessor); getter != nil {
			t = c.signatureOfDeclaration(getter).ReturnType
		} else {
			// A write-only property's query type is its assignment-value type.
			t = c.intrinsic[TypeUnknown]
			if decl != nil {
				sig := c.signatureOfDeclaration(decl)
				if len(sig.Parameters) != 0 {
					t = sig.Parameters[len(sig.Parameters)-1].Type
				}
			}
		}
	case decl == nil:
		t = c.intrinsic[TypeUnknown]
	case decl.Kind == ast.KindVariableDeclaration || decl.Kind == ast.KindFieldDeclaration:
		d := decl.AsVariableDeclaration()
		t = c.declaredType(d.Name, d.Type, d.Dimensions)
		if ast.IsConstantSymbol(symbol) && d.Type == nil && hintType(d.Name.AsIdentifier().TypeHint) == TypeUnknown {
			t = c.typeOfNode(d.Initializer)
		}
		if ast.IsConstantSymbol(symbol) && (object(t.Kind) || t.Kind == TypeArray || t.Kind == TypeStructure) {
			c.report(DiagnosticInvalidConstantType, d.Name, "Constants require a scalar or Variant type")
			t = c.intrinsic[TypeError]
		}
	case decl.Kind == ast.KindParameter:
		d := decl.AsParameterDeclaration()
		t = c.declaredType(d.Name, d.Type, d.Dimensions)
	case decl.Kind == ast.KindReDimDeclaration:
		d := decl.AsReDimDeclaration()
		t = c.declaredType(d.Expression, d.Type, nil)
		if t.Kind != TypeArray && t.Kind != TypeError {
			t = &Type{Kind: TypeArray, ElementType: t}
		}
	case ast.IsProcedureDeclaration(decl):
		// The Function/Get result is storage, not its procedure symbol.
		t = c.signatureOfDeclaration(decl).ReturnType
	case decl.Kind == ast.KindIdentifier:
		t = c.declaredType(decl, nil, nil)
	default:
		t = c.intrinsic[TypeUnknown]
	}
	c.symbolTypes[symbol] = t
	return t
}

func hintType(kind ast.Kind) TypeKind {
	switch kind {
	case ast.KindPERCENT:
		return TypeInteger
	case ast.KindAMPERSAND:
		return TypeLong
	case ast.KindPOW:
		return TypeLongLong
	case ast.KindEXCLAMATIONPOINT:
		return TypeSingle
	case ast.KindHASH:
		return TypeDouble
	case ast.KindAT:
		return TypeCurrency
	case ast.KindDOLLAR:
		return TypeString
	}
	return TypeUnknown
}

func builtinType(kind ast.Kind) TypeKind {
	switch kind {
	case ast.KindBOOLEAN:
		return TypeBoolean
	case ast.KindBYTE:
		return TypeByte
	case ast.KindINTEGER:
		return TypeInteger
	case ast.KindLONG:
		return TypeLong
	case ast.KindLONGLONG:
		return TypeLongLong
	case ast.KindLONGPTR:
		return TypeLongPtr
	case ast.KindSINGLE:
		return TypeSingle
	case ast.KindDOUBLE:
		return TypeDouble
	case ast.KindCURRENCY:
		return TypeCurrency
	case ast.KindDATE:
		return TypeDate
	case ast.KindSTRING:
		return TypeString
	case ast.KindOBJECT:
		return TypeObject
	case ast.KindVARIANT:
		return TypeVariant
	}
	return TypeUnknown
}

func (c *Checker) declaredType(name, annotation *ast.Node, dimensions *ast.NodeList) *Type {
	t := c.intrinsic[TypeVariant]
	hint := TypeUnknown
	if name != nil {
		hint = hintType(name.AsIdentifier().TypeHint)
	}
	if hint != TypeUnknown {
		t = c.intrinsic[hint]
	}
	if annotation != nil {
		t = c.typeOfAnnotation(annotation)
		base := t
		if base.Kind == TypeArray {
			base = base.ElementType
		}
		if hint != TypeUnknown && base.Kind != TypeError && base.Kind != hint {
			c.report(DiagnosticConflictingType, annotation, "The As type conflicts with the declaration's type character")
			t = c.intrinsic[TypeError]
		}
	}
	if dimensions != nil && t.Kind != TypeArray && t.Kind != TypeError {
		t = &Type{Kind: TypeArray, ElementType: t, Rank: len(dimensions.Nodes)}
	}
	return t
}

func (c *Checker) typeOfAnnotation(node *ast.Node) *Type {
	if t := c.types[node]; t != nil {
		return t
	}
	if node.Kind != ast.KindTypeReference {
		return c.intrinsic[TypeUnknown]
	}
	annotation := node.AsTypeReference()
	t := c.typeFromName(annotation.TypeName)
	if annotation.NewKeyword != nil && t.Kind != TypeClass && t.Kind != TypeError {
		c.report(DiagnosticInvalidNew, annotation.NewKeyword, "As New requires a class type")
		t = c.intrinsic[TypeError]
	}
	if annotation.IsArray && t.Kind != TypeError {
		t = &Type{Kind: TypeArray, ElementType: t}
	}
	c.types[node] = t
	return t
}

func (c *Checker) typeFromName(node *ast.Node) *Type {
	if node == nil {
		return c.intrinsic[TypeError]
	}
	if t := c.types[node]; t != nil {
		return t
	}
	if kind := builtinType(node.Kind); kind != TypeUnknown {
		t := c.intrinsic[kind]
		c.types[node] = t
		return t
	}
	name := typeNameText(node)
	symbol := c.symbols[node]
	if symbol == nil && node.Kind == ast.KindIdentifier {
		if c.project != nil {
			symbol = c.project.Exports.Lookup(name)
		}
	} else if symbol == nil && node.Kind == ast.KindQualifiedName {
		n := node.AsQualifiedName()
		if n.Left.Kind == ast.KindIdentifier && n.Right.Kind == ast.KindIdentifier {
			if c.project != nil && ast.SymbolNameKey(n.Left.AsIdentifier().Text) == ast.SymbolNameKey(c.project.Name) {
				symbol = c.project.Exports.Lookup(n.Right.AsIdentifier().Text)
				c.symbols[n.Left] = c.project
				c.types[n.Left] = c.typeOfSymbol(c.project)
			}
		}
	}
	t := c.intrinsic[TypeError]
	if symbol != nil && symbol.Flags&ast.SymbolFlagsType != 0 {
		t = c.typeOfSymbol(symbol)
		if symbol.Flags&ast.SymbolFlagsRegularEnum != 0 {
			t = c.intrinsic[TypeLong]
		}
		c.symbols[node] = symbol
		if node.Kind == ast.KindQualifiedName {
			right := node.AsQualifiedName().Right
			c.symbols[right], c.types[right] = symbol, t
		}
	} else if name != "" {
		c.report(DiagnosticUnknownType, node, fmt.Sprintf("Type %q is not defined in this project", name))
	}
	c.types[node] = t
	return t
}

func typeNameText(node *ast.Node) string {
	if node.Kind == ast.KindIdentifier {
		return node.AsIdentifier().Text
	}
	if node.Kind == ast.KindQualifiedName {
		name := node.AsQualifiedName()
		left, right := typeNameText(name.Left), typeNameText(name.Right)
		if left != "" && right != "" {
			return left + "." + right
		}
	}
	return ""
}

func (c *Checker) signatureOfDeclaration(node *ast.Node) *Signature {
	if sig := c.signatures[node]; sig != nil {
		return sig
	}
	sig := &Signature{Declaration: node, ReturnType: c.intrinsic[TypeVoid]}
	c.signatures[node] = sig
	fn := node.AsFunctionDeclaration()
	if node.Kind == ast.KindFunctionDeclaration || node.Kind == ast.KindGetAccessor {
		sig.ReturnType = c.declaredType(fn.Name(), fn.Type, nil)
	}
	if fn.Parameters != nil {
		for _, node := range fn.Parameters.Nodes {
			parameter := &Parameter{Declaration: node, Symbol: node.Symbol, Type: c.typeOfSymbol(node.Symbol)}
			if name := ast.DeclarationName(node); name != nil {
				parameter.Name = name.AsIdentifier().Text
			}
			if node.Kind == ast.KindParameter {
				d := node.AsParameterDeclaration()
				parameter.Modifiers, parameter.Default = d.Modifiers, d.Initializer
			}
			if (sig.Declaration.Kind == ast.KindLetAccessor || sig.Declaration.Kind == ast.KindSetAccessor) && node == fn.Parameters.Nodes[len(fn.Parameters.Nodes)-1] {
				parameter.Modifiers = parameter.Modifiers&^ast.ParameterByRef | ast.ParameterByVal
			}
			sig.Parameters = append(sig.Parameters, parameter)
		}
	}
	return sig
}

func (c *Checker) visitConstants(node *ast.Node) {
	if node.Kind == ast.KindAttributeStatement {
		return
	}
	if (node.Kind == ast.KindVariableDeclaration || node.Kind == ast.KindEnumMember) && ast.IsConstantSymbol(node.Symbol) {
		c.resolveConstant(node.Symbol, node)
	}
	node.ForEachChild(func(child *ast.Node) bool { c.visitConstants(child); return false })
}

func (c *Checker) resolveConstant(symbol *ast.Symbol, use *ast.Node) bool {
	if symbol.Flags&ast.SymbolFlagsEnumMember != 0 {
		_, ok := c.numericConstant(symbol)
		return ok
	}
	switch c.constants[symbol] {
	case 1:
		c.report(DiagnosticCircularConstant, use, fmt.Sprintf("Constant %q depends on itself", symbol.Name))
		c.constants[symbol] = 3
		return false
	case 2:
		return true
	case 3:
		return false
	}
	c.constants[symbol] = 1
	valid := true
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if target := c.symbols[node]; ast.IsConstantSymbol(target) {
			if !c.resolveConstant(target, node) {
				valid = false
			}
		}
		return node.ForEachChild(visit)
	}
	decl := symbol.ValueDeclaration.AsVariableDeclaration()
	if decl.Initializer != nil {
		visit(decl.Initializer)
	}
	if !valid {
		c.constants[symbol] = 3
		c.symbolTypes[symbol] = c.intrinsic[TypeError]
		return false
	}
	// State remains "resolving" during inference; references to dependencies
	// have already completed, and cycle errors were handled above.
	t := c.typeOfSymbol(symbol)
	if t.Kind == TypeBoolean || t.Kind == TypeByte || t.Kind == TypeInteger || t.Kind == TypeLong || t.Kind == TypeSingle || t.Kind == TypeDouble {
		if _, ok := c.numericConstant(symbol); !ok {
			c.report(DiagnosticConstantValue, use, "Constant requires a constant expression within its declared type's range")
			c.constants[symbol] = 3
			return false
		}
	}
	c.constants[symbol] = 2
	return true
}

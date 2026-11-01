package checker

import (
	"math"
	"strconv"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/stdlib"
)

func (c *Checker) typeOfNode(node *ast.Node) *Type {
	if node == nil {
		return c.intrinsic[TypeError]
	}
	if t := c.types[node]; t != nil {
		return t
	}
	t := c.intrinsic[TypeUnknown]
	switch node.Kind {
	case ast.KindSourceFile, ast.KindVariableDeclaration, ast.KindParameter, ast.KindFieldDeclaration, ast.KindTypeDeclaration, ast.KindEnumDeclaration, ast.KindEnumMember:
		t = c.typeOfSymbol(c.symbols[node])
	case ast.KindFunctionDeclaration, ast.KindSubDeclaration, ast.KindGetAccessor, ast.KindLetAccessor, ast.KindSetAccessor:
		sig := c.signatureOfDeclaration(node)
		t = &Type{Kind: TypeProcedure, Symbol: node.Symbol, Signatures: []*Signature{sig}}
		c.typeOfSymbol(node.Symbol)
		if result := node.AsFunctionDeclaration().ResultSymbol; result != nil {
			c.typeOfSymbol(result)
		}
	case ast.KindTypeReference:
		return c.typeOfAnnotation(node)
	case ast.KindIdentifier:
		if node.AsIdentifier().Text == "" {
			t = c.intrinsic[TypeError]
		} else if node.Parent != nil && node.Parent.Kind == ast.KindNamedArgument && node.Parent.AsNamedArgument().Name == node {
			t = c.intrinsic[TypeUnknown] // Parameter-name matching is not a value lookup.
		} else if isDeclarationName(node) {
			t = c.typeOfSymbol(c.symbols[node])
		} else if parent := node.Parent; parent != nil && parent.Kind == ast.KindPropertyAccessExpression && parent.AsPropertyAccessExpression().Name == node {
			t = c.typeOfNode(parent)
		} else {
			t = c.typeOfReference(node)
		}
	case ast.KindNumericLiteral:
		t = c.numericLiteralType(node)
	case ast.KindStringLiteral:
		t = c.intrinsic[TypeString]
	case ast.KindTRUE, ast.KindFALSE:
		t = c.intrinsic[TypeBoolean]
	case ast.KindEMPTY, ast.KindNULL:
		t = c.intrinsic[TypeVariant]
	case ast.KindNOTHING:
		t = c.intrinsic[TypeObject]
	case ast.KindParenthesizedExpression:
		t = c.typeOfNode(node.AsParenthesizedExpression().Inner)
	case ast.KindPrefixUnaryExpression:
		t = c.unaryType(node)
	case ast.KindBinaryExpression:
		t = c.binaryType(node)
	case ast.KindPropertyAccessExpression:
		t = c.memberType(node)
	case ast.KindCallExpression:
		t = c.callType(node)
	case ast.KindNamedArgument:
		t = c.typeOfNode(node.AsNamedArgument().Expression)
	case ast.KindNewExpression:
		t = c.typeFromName(node.AsNewExpression().Expression)
		if t.Kind != TypeClass && t.Kind != TypeError {
			c.report(DiagnosticInvalidNew, node, "New requires a class type")
			t = c.intrinsic[TypeError]
		}
	case ast.KindMeExpression:
		if c.symbols[node] != nil {
			t = c.typeOfSymbol(c.symbols[node])
		} else {
			t = c.intrinsic[TypeError]
		}
	}
	c.types[node] = t
	return t
}

func isDeclarationName(node *ast.Node) bool {
	return node.Parent != nil && ast.DeclarationName(node.Parent) == node
}

func (c *Checker) typeOfReference(node *ast.Node) *Type {
	symbol := c.symbols[node]
	t := c.typeOfSymbol(symbol)
	if symbol == nil {
		return t
	}
	if symbol.Library == stdlib.Err {
		reference := node
		if reference.Parent != nil && reference.Parent.Kind == ast.KindPropertyAccessExpression && reference.Parent.AsPropertyAccessExpression().Name == reference {
			reference = reference.Parent
		}
		if reference.Parent == nil || reference.Parent.Kind != ast.KindPropertyAccessExpression || reference.Parent.AsPropertyAccessExpression().Expression != reference {
			return c.intrinsic[TypeLong]
		}
	}
	if symbol.Flags&ast.SymbolFlagsFunction != 0 && len(t.Signatures) != 0 {
		if addressOfReference(node) {
			return t
		}
		c.resolved[node] = t.Signatures[0]
		if calleeReference(node) {
			return t
		}
		if directAssignment(node) == nil {
			c.invocation(node, node, referenceReceiver(node, c), t.Signatures[0], nil, nil, nil)
		}
		return t.Signatures[0].ReturnType
	}
	if symbol.Flags&ast.SymbolFlagsAccessor != 0 {
		kind := ast.KindGetAccessor
		if assignment := assignmentForReference(node); assignment != nil {
			kind = ast.KindLetAccessor
			if assignment.Kind == ast.KindSetStatement {
				kind = ast.KindSetAccessor
			}
		}
		if decl := ast.PropertyAccessor(symbol, kind); decl != nil {
			sig := c.signatureOfDeclaration(decl)
			c.resolved[node] = sig
			if !calleeReference(node) {
				var rhs *ast.Node
				if assignment := directAssignment(node); assignment != nil {
					rhs = assignment.AsLetStatement().RExpression
				}
				c.invocation(node, node, referenceReceiver(node, c), sig, nil, rhs, nil)
			}
			if kind != ast.KindGetAccessor && len(sig.Parameters) != 0 {
				return sig.Parameters[len(sig.Parameters)-1].Type
			}
			return sig.ReturnType
		}
		return c.intrinsic[TypeError] // Binder reported a missing accessor.
	}
	return t
}

func referenceParent(node *ast.Node) (*ast.Node, *ast.Node) {
	for node.Parent != nil && node.Parent.Kind == ast.KindParenthesizedExpression {
		node = node.Parent
	}
	return node, node.Parent
}

func addressOfReference(node *ast.Node) bool {
	_, parent := referenceParent(node)
	return parent != nil && parent.Kind == ast.KindPrefixUnaryExpression && parent.AsPrefixUnaryExpression().Operator == ast.KindADDRESSOF
}

func calleeReference(node *ast.Node) bool {
	node, parent := referenceParent(node)
	return parent != nil && parent.Kind == ast.KindCallExpression && parent.AsCallExpression().Expression == node
}

// Only a direct property application is a setter. P()(i) writes through a
// getter's result; the first application must retain its getter signature.
func assignmentForReference(node *ast.Node) *ast.Node {
	parent := node.Parent
	if parent != nil && parent.Kind == ast.KindCallExpression && parent.AsCallExpression().Expression == node {
		node, parent = parent, parent.Parent
	}
	if parent != nil && (parent.Kind == ast.KindLetStatement || parent.Kind == ast.KindSetStatement) && parent.AsLetStatement().LExpression == node {
		return parent
	}
	return nil
}

func (c *Checker) callType(node *ast.Node) *Type {
	call := node.AsCallExpression()
	target := ast.SkipParentheses(call.Expression)
	if target == nil {
		return c.intrinsic[TypeError]
	}
	t := c.typeOfNode(call.Expression)
	var args []*ast.Node
	if call.Arguments != nil {
		args = call.Arguments.Nodes
	}
	if sig := c.resolved[target]; sig != nil && (target.Kind == ast.KindIdentifier || target.Kind == ast.KindPropertyAccessExpression) {
		var rhs *ast.Node
		if statement := directAssignment(node); statement != nil {
			rhs = statement.AsLetStatement().RExpression
		}
		if len(sig.Parameters) == 0 && len(args) != 0 && (sig.Declaration.Kind == ast.KindFunctionDeclaration || sig.Declaration.Kind == ast.KindGetAccessor) {
			prefix := c.invocation(node, target, referenceReceiver(target, c), sig, nil, nil, nil)
			delete(c.calls, node)
			delete(c.resolved, node)
			result := c.application(node, target, sig.ReturnType, args)
			c.calls[node].Prefix = prefix
			return result
		}
		return c.invocation(node, target, referenceReceiver(target, c), sig, args, rhs, nil).ResultType
	}
	return c.application(node, call.Expression, t, args)
}

func referenceReceiver(node *ast.Node, c *Checker) *ast.Node {
	if node.Kind == ast.KindPropertyAccessExpression {
		return c.memberReceiver(node)
	}
	return nil
}

func (c *Checker) numericLiteralType(node *ast.Node) *Type {
	text := strings.ToLower(node.AsNumericLiteral().Text)
	if len(text) != 0 {
		if hint := hintType(ast.IdentifierTypeHint(text[len(text)-1])); hint != TypeUnknown {
			v, ok := constantNumber(text)
			switch hint {
			case TypeInteger:
				ok = ok && v >= math.MinInt16 && v <= math.MaxInt16
			case TypeLong:
				ok = ok && v >= math.MinInt32 && v <= math.MaxInt32
			case TypeLongLong:
				ok = ok && v >= -math.Exp2(63) && v < math.Exp2(63)
			case TypeSingle:
				ok = ok && math.Abs(v) <= math.MaxFloat32
			case TypeCurrency:
				ok = ok && math.Abs(v) <= 922337203685477.5807
			}
			if ok {
				return c.intrinsic[hint]
			}
			c.report(DiagnosticInvalidLiteral, node, "Numeric literal is outside its explicit type's range")
			return c.intrinsic[TypeError]
		}
	}
	if strings.HasPrefix(text, "&h") || strings.HasPrefix(text, "&o") {
		base := 16
		if text[1] == 'o' {
			base = 8
		}
		value, err := strconv.ParseUint(text[2:], base, 32)
		if err == nil {
			if value <= 0xffff {
				return c.intrinsic[TypeInteger]
			}
			return c.intrinsic[TypeLong]
		}
	} else {
		if !strings.ContainsAny(text, ".ed") {
			if value, err := strconv.ParseUint(text, 10, 64); err == nil {
				if value <= 32767 {
					return c.intrinsic[TypeInteger]
				}
				if value <= 2147483647 {
					return c.intrinsic[TypeLong]
				}
			}
		}
		value, err := strconv.ParseFloat(strings.ReplaceAll(text, "d", "e"), 64)
		if err == nil && !math.IsInf(value, 0) && !math.IsNaN(value) {
			return c.intrinsic[TypeDouble]
		}
	}
	c.report(DiagnosticInvalidLiteral, node, "Numeric literal is invalid or outside its type's range")
	return c.intrinsic[TypeError]
}

func scalar(kind TypeKind) bool { return kind >= TypeBoolean && kind <= TypeString }
func object(kind TypeKind) bool { return kind == TypeObject || kind == TypeClass }
func integral(kind TypeKind) bool {
	return kind >= TypeBoolean && kind <= TypeLongLong
}

func (c *Checker) unaryType(node *ast.Node) *Type {
	expr := node.AsPrefixUnaryExpression()
	t := c.typeOfNode(expr.Operand)
	if expr.Operator == ast.KindBYVAL {
		return t // Call-site passing syntax does not perform arithmetic coercion.
	}
	if expr.Operator == ast.KindADDRESSOF {
		return c.intrinsic[TypeUnknown]
	}
	t = c.simpleType(expr.Operand)
	if t.Kind == TypeError || t.Kind == TypeUnknown || t.Kind == TypeVariant {
		return t
	}
	if object(t.Kind) || t.Kind == TypeLongPtr {
		return c.intrinsic[TypeUnknown]
	}
	if !scalar(t.Kind) {
		return c.invalidOperand(node)
	}
	kind := t.Kind
	if expr.Operator == ast.KindNOT {
		if !integral(kind) {
			kind = TypeLong
		}
	} else {
		if kind == TypeBoolean || kind == TypeByte && expr.Operator == ast.KindMINUS {
			kind = TypeInteger
		}
		if kind == TypeString {
			kind = TypeDouble
		}
	}
	return c.intrinsic[kind]
}

func (c *Checker) binaryType(node *ast.Node) *Type {
	expr := node.AsBinaryExpression()
	left, right := c.typeOfNode(expr.Left), c.typeOfNode(expr.Right)
	if expr.OperatorToken.Kind != ast.KindIS {
		left, right = c.simpleType(expr.Left), c.simpleType(expr.Right)
	}
	l, r := left.Kind, right.Kind
	if l == TypeError || r == TypeError {
		return c.intrinsic[TypeError]
	}
	if l == TypeArray || r == TypeArray || l == TypeVoid || r == TypeVoid || l == TypeModule || r == TypeModule || l == TypeStructure || r == TypeStructure {
		return c.invalidOperand(node)
	}
	if l == TypeUnknown || r == TypeUnknown {
		return c.intrinsic[TypeUnknown]
	}
	if expr.OperatorToken.Kind == ast.KindIS {
		if (object(l) || l == TypeVariant) && (object(r) || r == TypeVariant) {
			return c.intrinsic[TypeBoolean]
		}
		return c.invalidOperand(node)
	}
	if l == TypeVariant || r == TypeVariant {
		return c.intrinsic[TypeVariant]
	}
	switch expr.OperatorToken.Kind {
	case ast.KindEQ, ast.KindNEQ, ast.KindLT, ast.KindGT, ast.KindLEQ, ast.KindGEQ:
		return c.intrinsic[TypeBoolean]
	}
	if object(l) || object(r) || l == TypeLongPtr || r == TypeLongPtr {
		return c.intrinsic[TypeUnknown] // Default-value members/target profile are unresolved.
	}
	if !scalar(l) || !scalar(r) {
		return c.invalidOperand(node)
	}
	kind := TypeUnknown
	switch expr.OperatorToken.Kind {
	case ast.KindEQ, ast.KindNEQ, ast.KindLT, ast.KindGT, ast.KindLEQ, ast.KindGEQ, ast.KindLIKE:
		kind = TypeBoolean
	case ast.KindAMPERSAND:
		kind = TypeString
	case ast.KindAND, ast.KindOR, ast.KindXOR, ast.KindIMP, ast.KindEQV:
		switch {
		case l == TypeLongLong || r == TypeLongLong:
			kind = TypeLongLong
		case !integral(l) || !integral(r) || l == TypeLong || r == TypeLong:
			kind = TypeLong
		case l == TypeBoolean && r == TypeBoolean:
			kind = TypeBoolean
		case l == TypeByte && r == TypeByte:
			kind = TypeByte
		default:
			kind = TypeInteger
		}
	case ast.KindPLUS:
		if l == TypeString && r == TypeString {
			kind = TypeString
		} else {
			kind = arithmeticType(l, r)
		}
	case ast.KindMINUS:
		kind = arithmeticType(l, r)
		if l == TypeDate && r == TypeDate {
			kind = TypeDouble
		}
	case ast.KindMULT:
		kind = arithmeticType(l, r)
		if l == TypeDate || r == TypeDate || l == TypeCurrency && (r == TypeSingle || r == TypeDouble || r == TypeString) || r == TypeCurrency && (l == TypeSingle || l == TypeDouble || l == TypeString) {
			kind = TypeDouble
		}
	case ast.KindDIV:
		kind = TypeDouble
		if l == TypeSingle && (r == TypeSingle || r == TypeBoolean || r == TypeByte || r == TypeInteger) || r == TypeSingle && (l == TypeSingle || l == TypeBoolean || l == TypeByte || l == TypeInteger) {
			kind = TypeSingle
		}
	case ast.KindINTDIV, ast.KindMOD:
		kind = arithmeticType(l, r)
		if !integral(l) || !integral(r) {
			kind = TypeLong
		}
	case ast.KindPOW:
		kind = TypeDouble
	}
	return c.intrinsic[kind]
}

// MS-VBAL 5.6.9.3's declared-type table, before per-operator exceptions.
func arithmeticType(l, r TypeKind) TypeKind {
	switch {
	case l == TypeDate || r == TypeDate:
		return TypeDate
	case l == TypeCurrency || r == TypeCurrency:
		return TypeCurrency
	case l == TypeDouble || r == TypeDouble || l == TypeString || r == TypeString:
		return TypeDouble
	case l == TypeSingle || r == TypeSingle:
		if l == TypeLong || r == TypeLong || l == TypeLongLong || r == TypeLongLong {
			return TypeDouble
		}
		return TypeSingle
	case l == TypeLongLong || r == TypeLongLong:
		return TypeLongLong
	case l == TypeLong || r == TypeLong:
		return TypeLong
	case l == TypeBoolean || r == TypeBoolean || l == TypeInteger || r == TypeInteger:
		return TypeInteger
	default:
		return TypeByte
	}
}

func (c *Checker) invalidOperand(node *ast.Node) *Type {
	c.report(DiagnosticInvalidOperand, node, "Operator is not defined for these operand types")
	return c.intrinsic[TypeError]
}

package lowering

import (
	"unicode/utf16"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
)

func fixedString(annotation *ast.Node) bool {
	return annotation != nil && annotation.Kind == ast.KindTypeReference && annotation.AsTypeReference().Length != nil
}

func (b *procedureBuilder) stringLiteral(n *ast.Node, units []uint16, null bool) ir.ValueID {
	op := ir.OpStringLiteral
	if null {
		op = ir.OpNullString
	}
	return b.emit(ir.Instruction{Op: op, Type: ir.Type{Scalar: ir.String}, Units: units, Source: source(n)})
}

// Const evaluation is required by the language, independently of optimization.
// Keep it bounded and limited to checked String constants; calls/coercions remain
// unsupported rather than accidentally becoming a second VBA evaluator.
func (l *lowerer) constantString(n *ast.Node, depth int) ([]uint16, bool, bool) {
	if n == nil || depth > 256 {
		return nil, false, false
	}
	// Ambient literal initializers have no source owner and need no file walk.
	if n.Kind == ast.KindStringLiteral {
		return utf16.Encode([]rune(n.AsStringLiteral().Text)), false, true
	}
	if l.checker.GetTypeAtLocation(n).Kind != checker.TypeString {
		return nil, false, false
	}
	switch n.Kind {
	case ast.KindParenthesizedExpression:
		return l.constantString(n.AsParenthesizedExpression().Inner, depth+1)
	case ast.KindIdentifier, ast.KindPropertyAccessExpression:
		s := l.checker.GetSymbolAtLocation(n)
		if s == nil || !ast.IsConstantSymbol(s) || s.ValueDeclaration == nil || s.ValueDeclaration.Kind != ast.KindVariableDeclaration {
			return nil, false, false
		}
		if s.Library == stdlib.NullString {
			return nil, true, true
		}
		return l.constantString(s.ValueDeclaration.AsVariableDeclaration().Initializer, depth+1)
	case ast.KindBinaryExpression:
		e := n.AsBinaryExpression()
		if e.OperatorToken.Kind != ast.KindAMPERSAND && e.OperatorToken.Kind != ast.KindPLUS {
			return nil, false, false
		}
		a, _, okA := l.constantString(e.Left, depth+1)
		b, _, okB := l.constantString(e.Right, depth+1)
		if !okA || !okB || len(a)+len(b) > 1<<20 {
			return nil, false, false
		}
		return append(append([]uint16(nil), a...), b...), false, true
	}
	return nil, false, false
}

func (l *lowerer) constantDeclarations(n *ast.Node) {
	for _, d := range nodes(n.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations) {
		decl := d.AsVariableDeclaration()
		_, _, text := l.constantString(decl.Initializer, 0)
		_, number := l.checker.GetNumericConstantValue(d.Symbol)
		l.scalar(d, l.checker.GetTypeOfSymbol(d.Symbol), false)
		if t, valueType := l.checker.GetTypeOfSymbol(d.Symbol), l.checker.GetTypeAtLocation(decl.Initializer); t.Kind == checker.TypeCurrency || t.Kind == checker.TypeDate || valueType.Kind == checker.TypeCurrency || valueType.Kind == checker.TypeDate {
			l.unsupported(d, "Date/Currency Const evaluation")
		}
		if fixedString(decl.Type) || !text && !number {
			l.unsupported(d, "unsupported Const expressions")
		}
	}
}

func (b *procedureBuilder) stringLength(n *ast.Node, r *checker.CallResolution) ir.ValueID {
	a := r.Arguments[0]
	kind := r.Signature.Declaration.Symbol.Library
	if t := b.checker.GetTypeAtLocation(a.Expression); t.Kind == checker.TypeDate || t.Kind == checker.TypeCurrency {
		b.expression(a.Expression)
		return b.literal(n, ir.Type{Scalar: ir.Long}, 8)
	}
	if b.checker.GetTypeAtLocation(a.Expression).Kind == checker.TypeVariant {
		op := ir.OpLen
		if kind == stdlib.LenB {
			op = ir.OpLenB
		}
		return b.operation(n, op, ir.Type{Scalar: ir.Variant}, ir.MayRaise|ir.ManagesLifetime, b.expression(a.Expression))
	}
	if b.checker.GetTypeAtLocation(a.Expression).Kind != checker.TypeString {
		b.unsupported(n, "Len/LenB of non-String values")
		return 0
	}
	value := b.expression(a.Expression)
	op, effects := ir.OpLen, ir.Effects(0)
	if kind == stdlib.LenB {
		op, effects = ir.OpLenB, ir.MayRaise
	}
	return b.operation(n, op, ir.Type{Scalar: ir.Long}, effects, value)
}

func (b *procedureBuilder) midCall(n *ast.Node, r *checker.CallResolution) ir.ValueID {
	args := make([]ir.ValueID, 3)
	for _, argument := range r.Arguments {
		i := argument.ParameterIndex
		if i < 0 || i > 2 {
			b.unsupported(n, "Mid argument form")
			return 0
		}
		if i == 2 && argument.Mode == checker.ArgumentMissing {
			args[i] = b.literal(n, ir.Type{Scalar: ir.Long}, 2147483647)
			continue
		}
		if argument.Expression == nil {
			b.unsupported(n, "Mid argument form")
			return 0
		}
		if i == 0 && b.checker.GetTypeAtLocation(argument.Expression).Kind != checker.TypeString {
			b.unsupported(n, "Mid/Mid$ on non-String values (Variant/Null semantics)")
			return 0
		}
		value := b.expression(argument.Expression)
		if i != 0 {
			value = b.coerce(argument.Expression, value, ir.Type{Scalar: ir.Long})
		}
		args[i] = value
	}
	return b.operation(n, ir.OpMid, ir.Type{Scalar: ir.String}, ir.MayRaise|ir.ManagesLifetime, args...)
}

func (b *procedureBuilder) midAssignment(node *ast.Node) {
	n := node.AsMidAssignmentStatement()
	r := b.checker.GetMidAssignmentResolution(node)
	if r == nil || !r.Valid || r.Deferred || r.Storage.Type.Kind != checker.TypeString {
		b.unsupported(node, "unresolved or Variant Mid assignment")
		return
	}
	address := b.reference(n.Target, r.Storage)
	start := b.conversion(n.Start, b.expression(n.Start), r.Start)
	length := ir.ValueID(0)
	if n.Length == nil {
		length = b.literal(node, ir.Type{Scalar: ir.Long}, 2147483647)
	} else {
		length = b.conversion(n.Length, b.expression(n.Length), r.Length)
	}
	// MS-VBAL's statement semantics validate bounds before evaluating the RHS.
	// Retain the variable address, not a stale value snapshot, across RHS calls.
	b.operation(node, ir.OpMidCheck, ir.Type{Scalar: ir.Void}, ir.MayRaise, address, start, length)
	replacement := b.conversion(node, b.expression(n.Expression), r.Replacement)
	b.operation(node, ir.OpMidAssign, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime, address, start, length, replacement)
}

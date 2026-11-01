package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
)

// compare is shared by binary expressions and Select Case. Declared operand
// types still select coercion rules; boxing must not erase that distinction.
func (b *procedureBuilder) compare(n *ast.Node, op ir.Op, left, right ir.ValueID) ir.ValueID {
	l, r := b.types[left], b.types[right]
	variant := ir.Type{Scalar: ir.Variant}
	result := ir.Type{Scalar: ir.Boolean}
	if l == variant || r == variant {
		result = variant
	}
	if result != variant && (l.Scalar == ir.String) != (r.Scalar == ir.String) {
		b.unsupported(n, "mixed numeric/String comparisons")
		return 0
	}
	if l.Scalar == ir.Date || l.Scalar == ir.Currency || r.Scalar == ir.Date || r.Scalar == ir.Currency || result == variant || l.Scalar == ir.String || r.Scalar == ir.String || l.Scalar != r.Scalar && (l.Scalar == ir.Boolean || r.Scalar == ir.Boolean) {
		mode := int64(0)
		if l.Scalar == ir.String || r.Scalar == ir.String {
			mode = 2
		} else if l != variant || r != variant {
			mode = 1
		}
		left, right = b.coerce(n, left, variant), b.coerce(n, right, variant)
		value := b.operation(n, op, variant, ir.MayRaise|ir.ManagesLifetime, left, right, b.literal(n, ir.Type{Scalar: ir.Long}, mode))
		return b.coerce(n, value, result)
	}
	operand := l
	if r.Scalar > l.Scalar && r.Scalar <= ir.Double {
		operand = r
	}
	if l.Scalar == ir.Single && r.Scalar == ir.Long || r.Scalar == ir.Single && l.Scalar == ir.Long {
		operand = ir.Type{Scalar: ir.Double}
	}
	effects := ir.Effects(0)
	if ir.Floating(operand) {
		effects = ir.MayRaise
	}
	return b.operation(n, op, result, effects, b.coerce(n, left, operand), b.coerce(n, right, operand))
}

func (b *procedureBuilder) onGotoStatement(node *ast.Node) {
	n := node.AsOnGotoStatement()
	value := b.expression(n.Expression)
	if t := b.types[value]; t.Scalar == ir.Boolean || t.Scalar == ir.String {
		value = b.coerce(node, value, ir.Type{Scalar: ir.Variant})
	}
	value = b.coerce(node, value, ir.Type{Scalar: ir.Integer})
	b.operation(node, ir.OpCheckDispatch, ir.Type{Scalar: ir.Void}, ir.MayRaise, value)
	header, done := b.statement, b.block(node)
	header.End, header.Selector = ir.Switch, value
	b.link(header, done, 1)
	b.link(header, done, 0)
	for i, destination := range n.Destinations.Nodes {
		target := destination.AsGotoStatement().Target
		if target == nil || b.labels[target.ValueDeclaration] == nil {
			b.unsupported(destination, "unresolved On GoTo target")
			continue
		}
		// The validated index cannot reach destinations beyond 255. Keep their
		// syntax and binding, without introducing unrepresentable switch keys.
		if i >= 255 {
			continue
		}
		header.Cases = append(header.Cases, ir.SwitchCase{Value: int64(i + 1)})
		if node.Kind == ast.KindOnGosubStatement {
			call := b.block(node)
			b.link(header, call, len(header.Cases)+2)
			b.activate(call)
			b.emitGosub(node, b.labels[target.ValueDeclaration], done)
		} else {
			b.link(header, b.labels[target.ValueDeclaration], len(header.Cases)+2)
		}
	}
	b.activate(done)
}

type caseAlternative struct {
	node *ast.Node
	body *ir.Statement
}

func (b *procedureBuilder) selectStatement(node *ast.Node) {
	n := node.AsSelectStatement()
	header, done := b.statement, b.block(node)
	value := b.expression(n.Expression)
	t := b.types[value]
	if !ir.ScalarValue(t) {
		b.unsupported(n.Expression, "Select Case selector type "+t.String())
		return
	}
	// Never reload the source variable. Later Case expressions can modify it,
	// including through ByRef aliases or mutable String operations.
	slot := b.addSlot(n.Expression, nil, "$select.value", t, ir.LoopState, false)
	b.store(node, slot, value)
	clauses := nodes(n.CaseBlock.AsCaseBlock().Clauses)
	bodies := make([]*ir.Statement, len(clauses))
	var alternatives []caseAlternative
	otherwise := done
	for i, clause := range clauses {
		bodies[i] = b.block(clause)
		if clause.Kind == ast.KindDefaultClause {
			otherwise = bodies[i]
			continue
		}
		for _, item := range nodes(clause.AsCaseClause().Expressions) {
			alternatives = append(alternatives, caseAlternative{item, bodies[i]})
		}
	}
	first := otherwise
	if len(alternatives) != 0 {
		first = b.block(alternatives[0].node)
	}
	if t.Scalar == ir.Variant {
		isNull := b.operation(node, ir.OpIsNull, ir.Type{Scalar: ir.Boolean}, 0, value)
		b.branch(isNull, otherwise, first)
	} else {
		b.jump(first)
	}
	b.link(header, done, 0)
	for i := 0; i < len(alternatives); {
		b.activate(first)
		// Consecutive, effect-free integral equality alternatives have exactly
		// switch semantics. No density threshold, range expansion, or jump-table
		// construction belongs here; LLVM owns those target-dependent choices.
		end := i
		if t.Scalar == ir.Integer || t.Scalar == ir.Long {
			for end < len(alternatives) {
				if _, ok := b.caseInteger(alternatives[end].node); !ok {
					break
				}
				end++
			}
		}
		if end > i {
			next := otherwise
			if end < len(alternatives) {
				next = b.block(alternatives[end].node)
			}
			b.statement.End = ir.Switch
			b.statement.Selector = b.coerce(alternatives[i].node, b.load(alternatives[i].node, slot), ir.Type{Scalar: ir.Long})
			b.link(b.statement, next, 1)
			b.link(b.statement, done, 0)
			seen := make(map[int64]bool)
			for _, alt := range alternatives[i:end] {
				v, _ := b.caseInteger(alt.node)
				if seen[v] {
					continue
				} // First matching Case wins.
				seen[v] = true
				b.statement.Cases = append(b.statement.Cases, ir.SwitchCase{Value: v})
				b.link(b.statement, alt.body, len(b.statement.Cases)+2)
			}
			first, i = next, end
			continue
		}
		alt := alternatives[i]
		next := otherwise
		if i+1 < len(alternatives) {
			next = b.block(alternatives[i+1].node)
		}
		b.branch(b.caseCondition(alt.node, slot), alt.body, next)
		b.link(b.statement, done, 0)
		first, i = next, i+1
	}
	for i, clause := range clauses {
		b.activate(bodies[i])
		b.releaseSelector(clause, slot, t)
		b.sequence(clause.AsCaseClause().Statement)
		b.fallthroughTo(done)
	}
	b.activate(done)
	b.releaseSelector(node, slot, t)
}

func (b *procedureBuilder) releaseSelector(node *ast.Node, slot ir.SlotID, t ir.Type) {
	if t.Scalar == ir.String {
		b.store(node, slot, b.stringLiteral(node, nil, true))
	}
	if t.Scalar == ir.Variant {
		b.store(node, slot, b.emit(ir.Instruction{Op: ir.OpVariantConstant, Type: t, Source: source(node)}))
	}
}

func (b *procedureBuilder) caseCondition(node *ast.Node, slot ir.SlotID) ir.ValueID {
	left := b.load(node, slot)
	if node.Kind == ast.KindCaseRangeClause {
		n := node.AsCaseRangeClause()
		lower := b.compare(node, ir.OpGreaterEqual, left, b.expression(n.LowerBound))
		lower = b.conditionValue(node, lower)
		upper := b.compare(node, ir.OpLessEqual, left, b.expression(n.UpperBound))
		upper = b.conditionValue(node, upper)
		return b.operation(node, ir.OpAnd, ir.Type{Scalar: ir.Boolean}, 0, lower, upper)
	}
	op, expression := ir.OpEqual, node
	if node.Kind == ast.KindCaseComparisonClause {
		n := node.AsCaseComparisonClause()
		op = map[ast.Kind]ir.Op{ast.KindEQ: ir.OpEqual, ast.KindNEQ: ir.OpNotEqual, ast.KindLT: ir.OpLess, ast.KindLEQ: ir.OpLessEqual, ast.KindGT: ir.OpGreater, ast.KindGEQ: ir.OpGreaterEqual}[n.OperatorToken.Kind]
		expression = n.Expression
	}
	return b.conditionValue(node, b.compare(node, op, left, b.expression(expression)))
}

func (b *procedureBuilder) caseInteger(node *ast.Node) (int64, bool) {
	if node.Kind == ast.KindCaseComparisonClause {
		if node.AsCaseComparisonClause().OperatorToken.Kind != ast.KindEQ {
			return 0, false
		}
		node = node.AsCaseComparisonClause().Expression
	}
	return b.integralConstant(node, 0)
}

// Recognize checked literal/enum values only. General constant folding and
// simplification of checked arithmetic remain the optimizer's responsibility.
func (b *procedureBuilder) integralConstant(node *ast.Node, depth int) (int64, bool) {
	if depth > 64 {
		return 0, false
	}
	kind := b.checker.GetTypeAtLocation(node).Kind
	// Avoid accepting an unsupported floating-point literal or conversion by
	// skipping its ordinary lowering merely because its value looks integral.
	if kind != checker.TypeInteger && kind != checker.TypeLong {
		return 0, false
	}
	if value, ok := b.checker.GetNumericConstantValue(b.checker.GetSymbolAtLocation(node)); ok {
		return int64(value), true
	}
	if value, ok := b.checker.GetEnumMemberValue(b.checker.GetSymbolAtLocation(node)); ok {
		return value, true
	}
	switch node.Kind {
	case ast.KindNumericLiteral:
		v, ok := b.checker.GetNumericLiteralValue(node)
		return int64(v), ok
	case ast.KindParenthesizedExpression:
		return b.integralConstant(node.AsParenthesizedExpression().Inner, depth+1)
	case ast.KindPrefixUnaryExpression:
		n := node.AsPrefixUnaryExpression()
		v, ok := b.integralConstant(n.Operand, depth+1)
		if n.Operator == ast.KindMINUS {
			v = -v
		} else if n.Operator != ast.KindPLUS {
			return 0, false
		}
		if kind == checker.TypeInteger && (v < -32768 || v > 32767) || v < -2147483648 || v > 2147483647 {
			ok = false
		}
		return v, ok
	}
	return 0, false
}

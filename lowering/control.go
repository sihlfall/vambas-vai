package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
)

type controlEdge struct {
	from, to *ir.Statement
	field    int // 0: recovery Next, 1: Target, 2: FalseTarget, 3+i: Cases[i].Target.
}

func (b *procedureBuilder) block(n *ast.Node) *ir.Statement {
	return &ir.Statement{Source: source(n), Scope: b.scope}
}
func (b *procedureBuilder) activate(s *ir.Statement) {
	if b.statement != nil && b.statement.End == ir.Continue && !b.nextSet[b.statement] {
		b.link(b.statement, s, 0)
	}
	s.ID = ir.StatementID(len(b.procedure.Statements) + 1)
	s.Retry = s.ID
	s.Scope = b.scope
	b.procedure.Statements = append(b.procedure.Statements, s)
	b.statement = s
}
func (b *procedureBuilder) link(from, to *ir.Statement, field int) {
	b.edges = append(b.edges, controlEdge{from, to, field})
	if field == 0 {
		b.nextSet[from] = true
	}
}
func (b *procedureBuilder) jump(to *ir.Statement) {
	b.statement.End = ir.Jump
	b.link(b.statement, to, 1)
}
func (b *procedureBuilder) fallthroughTo(to *ir.Statement) {
	if b.statement.End == ir.Continue {
		// Resume Next follows this branch's continuation too; the next emitted
		// region may belong to a different Case/Else body.
		if !b.nextSet[b.statement] {
			b.link(b.statement, to, 0)
		}
		b.jump(to)
	}
}
func (b *procedureBuilder) branch(value ir.ValueID, yes, no *ir.Statement) {
	b.statement.End = ir.Branch
	b.statement.Condition = value
	b.link(b.statement, yes, 1)
	b.link(b.statement, no, 2)
}
func (b *procedureBuilder) resolveEdges() {
	for i, s := range b.procedure.Statements {
		if s.End != ir.Return && !b.nextSet[s] && i+1 < len(b.procedure.Statements) {
			s.Next = b.procedure.Statements[i+1].ID
		}
	}
	for _, edge := range b.edges {
		switch edge.field {
		case -1:
			edge.from.Handler = edge.to.ID
		case 0:
			edge.from.Next = edge.to.ID
		case 1:
			edge.from.Target = edge.to.ID
		case 2:
			edge.from.FalseTarget = edge.to.ID
		default:
			edge.from.Cases[edge.field-3].Target = edge.to.ID
		}
		if edge.from.Source.Node.Kind == ast.KindGotoStatement || edge.from.Source.Node.Kind == ast.KindOnGotoStatement || edge.from.End == ir.Gosub && edge.field == 1 {
			scope := edge.from.Scope
			for scope != 0 && scope != edge.to.Scope {
				scope = b.procedure.Scopes[scope-1].Parent
			}
			if scope != edge.to.Scope {
				name := "GoTo"
				if edge.from.End == ir.Gosub {
					name = "GoSub"
				}
				b.unsupported(edge.from.Source.Node, name+" entering an uninitialized resource scope")
			}
		}
	}
}
func (b *procedureBuilder) sequence(n *ast.Node) {
	if n == nil {
		return
	}
	if n.Kind != ast.KindBlock {
		b.lowerStatement(n)
		return
	}
	for _, statement := range nodes(n.AsBlock().Statements) {
		b.lowerStatement(statement)
	}
}
func (b *procedureBuilder) condition(n *ast.Node) ir.ValueID {
	return b.conditionValue(n, b.expression(n))
}
func (b *procedureBuilder) conditionValue(n *ast.Node, value ir.ValueID) ir.ValueID {
	t := b.types[value]
	if t.Scalar == ir.Boolean {
		return value
	}
	if t.Scalar == ir.String || t.Scalar == ir.Date || t.Scalar == ir.Currency {
		value = b.coerce(n, value, ir.Type{Scalar: ir.Variant})
		t = b.types[value]
	}
	effects := ir.Effects(0)
	if t.Scalar == ir.Variant || ir.Floating(t) {
		effects = ir.MayRaise
	}
	if t.Scalar != ir.Byte && t.Scalar != ir.Integer && t.Scalar != ir.Long && t.Scalar != ir.Variant && !ir.Floating(t) {
		b.unsupported(n, "condition type "+t.String())
		return 0
	}
	return b.operation(n, ir.OpCondition, ir.Type{Scalar: ir.Boolean}, effects, value)
}
func (b *procedureBuilder) ifStatement(node *ast.Node) {
	n := node.AsIfStatement()
	header := b.statement
	yes, done := b.block(n.ThenStatement), b.block(node)
	no := done
	if n.ElseStatement != nil {
		no = b.block(n.ElseStatement)
	}
	b.branch(b.condition(n.Expression), yes, no)
	b.link(header, done, 0)
	b.activate(yes)
	b.sequence(n.ThenStatement)
	b.fallthroughTo(done)
	if n.ElseStatement != nil {
		b.activate(no)
		b.sequence(n.ElseStatement)
	}
	b.activate(done)
}
func (b *procedureBuilder) whileStatement(node *ast.Node) {
	n := node.AsWhileStatement()
	header := b.statement
	body, done := b.block(n.Statement), b.block(node)
	b.branch(b.condition(n.Expression), body, done)
	b.link(header, done, 0)
	b.activate(body)
	b.sequence(n.Statement)
	b.fallthroughTo(header)
	b.activate(done)
}
func (b *procedureBuilder) doStatement(node *ast.Node) {
	n := node.AsDoStatement()
	header := b.statement
	body, done := b.block(n.Statement), b.block(node)
	b.exits[node] = done
	if n.Expression != nil && !n.ConditionAtEnd {
		value := b.condition(n.Expression)
		if n.ConditionKind == ast.KindUNTIL {
			b.branch(value, done, body)
		} else {
			b.branch(value, body, done)
		}
	} else {
		b.jump(body)
	}
	b.link(header, done, 0)
	b.activate(body)
	b.sequence(n.Statement)
	if n.ConditionAtEnd {
		b.newStatement(node)
		value := b.condition(n.Expression)
		if n.ConditionKind == ast.KindUNTIL {
			b.branch(value, done, body)
		} else {
			b.branch(value, body, done)
		}
	} else {
		b.fallthroughTo(header)
	}
	b.activate(done)
}
func (b *procedureBuilder) load(n *ast.Node, slot ir.SlotID) ir.ValueID {
	t := b.procedure.Slots[slot-1].Type
	if ir.Managed(t) {
		return b.managedStorage(n, ir.OpLoadManaged, t, slot)
	}
	return b.emit(ir.Instruction{Op: ir.OpLoad, Type: t, Slot: slot, Source: source(n)})
}
func (b *procedureBuilder) forStatement(node *ast.Node) {
	n := node.AsForStatement()
	initial := n.Initializer.AsLetStatement()
	header := b.statement
	s := b.checker.GetStorageAtLocation(initial.LExpression)
	if s.Kind != checker.StorageVariable || s.Type.Kind != checker.TypeInteger && s.Type.Kind != checker.TypeLong && s.Type.Kind != checker.TypeSingle && s.Type.Kind != checker.TypeDouble && s.Type.Kind != checker.TypeVariant {
		b.unsupported(node, "For counter storage/type")
		return
	}
	field := b.isRecordField(s) || b.isClassField(s)
	stable := b.isClassField(s) && (s.Receiver == nil || s.Receiver.Kind == ast.KindMeExpression) || b.isRecordField(s) && b.stableRecordStorage(s.Receiver)
	if field && !stable {
		b.unsupported(node, "unstable For field counter")
		return
	}
	counter := ir.SlotID(0)
	if !field {
		counter = b.storage(initial.LExpression, s)
	}
	if !field && counter == 0 {
		return
	}
	counterType := b.scalar(initial.LExpression, s.Type, false)
	loadCounter := func() ir.ValueID {
		if field {
			return b.readReference(node, b.reference(initial.LExpression, s))
		}
		return b.load(node, counter)
	}
	storeCounter := func(value ir.ValueID) {
		if field {
			b.writeReference(node, b.reference(initial.LExpression, s), value)
		} else {
			b.store(node, counter, value)
		}
	}
	start, limit := b.expression(initial.RExpression), b.expression(n.Limit)
	step := ir.ValueID(0)
	if n.Step == nil {
		step = b.literal(node, ir.Type{Scalar: ir.Integer}, 1)
	} else {
		step = b.expression(n.Step)
	}
	common := counterType
	for _, v := range []ir.ValueID{start, limit, step} {
		t := b.types[v]
		if t.Scalar == ir.String || ir.Array(t) {
			b.unsupported(node, "nonnumeric For bounds/Step")
			return
		}
		if t.Scalar == ir.Variant || t.Scalar == ir.Boolean {
			common.Scalar = ir.Variant
		} else if common.Scalar != ir.Variant {
			if common.Scalar == ir.Single && t.Scalar == ir.Long || common.Scalar == ir.Long && t.Scalar == ir.Single {
				common.Scalar = ir.Double
			} else if t.Scalar > common.Scalar {
				common = t
			}
		}
	}
	if common.Scalar == ir.Variant {
		start = b.coerce(node, start, common)
		limit = b.coerce(node, limit, common)
		step = b.coerce(node, step, common)
		b.operation(node, ir.OpForValidate, ir.Type{Scalar: ir.Void}, ir.MayRaise, start, limit, step)
	} else {
		limit = b.coerce(node, limit, common)
		step = b.coerce(node, step, common)
	}
	endSlot := b.addSlot(node, nil, "$for.limit", common, ir.LoopState, false)
	stepSlot := b.addSlot(node, nil, "$for.step", common, ir.LoopState, false)
	flag := b.addSlot(node, nil, "$for.initialized", ir.Type{Scalar: ir.Boolean}, ir.LoopState, false)
	storeCounter(b.coerce(node, start, counterType))
	b.store(node, endSlot, limit)
	b.store(node, stepSlot, step)
	b.store(node, flag, b.literal(node, ir.Type{Scalar: ir.Boolean}, -1))
	test, body, next, done := b.block(node), b.block(n.Statement), b.block(node), b.block(node)
	b.exits[node] = done
	b.jump(test)
	b.link(header, done, 0)
	b.activate(test)
	current := b.coerce(node, loadCounter(), common)
	bound := b.load(node, endSlot)
	increment := b.load(node, stepSlot)
	var condition ir.ValueID
	if common.Scalar == ir.Variant {
		condition = b.operation(node, ir.OpForTest, ir.Type{Scalar: ir.Boolean}, ir.MayRaise, current, bound, increment)
	} else {
		zero := b.literal(node, common, 0)
		boolean := ir.Type{Scalar: ir.Boolean}
		positive := b.compare(node, ir.OpGreaterEqual, increment, zero)
		negative := b.compare(node, ir.OpLess, increment, zero)
		under := b.compare(node, ir.OpLessEqual, current, bound)
		over := b.compare(node, ir.OpGreaterEqual, current, bound)
		a := b.operation(node, ir.OpAnd, boolean, 0, positive, under)
		c := b.operation(node, ir.OpAnd, boolean, 0, negative, over)
		condition = b.operation(node, ir.OpOr, boolean, 0, a, c)
	}
	b.branch(condition, body, done)
	b.link(test, done, 0)
	b.activate(body)
	b.sequence(n.Statement)
	b.fallthroughTo(next)
	b.activate(next)
	b.operation(node, ir.OpCheckInitialized, ir.Type{Scalar: ir.Void}, ir.MayRaise, b.load(node, flag))
	current = b.coerce(node, loadCounter(), common)
	increment = b.load(node, stepSlot)
	effects := ir.MayRaise
	if common.Scalar == ir.Variant {
		effects |= ir.ManagesLifetime
	}
	value := b.operation(node, ir.OpAdd, common, effects, current, increment)
	storeCounter(b.coerce(node, value, counterType))
	b.jump(test)
	b.link(next, done, 0)
	b.activate(done)
}

func (b *procedureBuilder) forEachStatement(node *ast.Node) {
	n := node.AsForEachStatement()
	if b.classes[b.checker.GetTypeAtLocation(n.Expression).Symbol] != 0 {
		b.unsupported(node, "class enumeration")
		return
	}
	header := b.statement
	counterType := b.scalar(n.Initializer, b.checker.GetTypeAtLocation(n.Initializer), false)
	if counterType.Scalar != ir.Variant && counterType.Scalar != ir.Object {
		b.unsupported(node, "For Each counter must be Variant for arrays")
		return
	}
	if counter := b.checker.GetStorageAtLocation(n.Initializer); counter.Kind != checker.StorageVariable {
		b.unsupported(node, "For Each counter requires local, parameter, or result storage")
		return
	}
	s := b.checker.GetStorageAtLocation(n.Expression)
	var ref ir.ValueID
	clone := int64(0)
	object := b.checker.GetTypeAtLocation(n.Expression).Kind == checker.TypeObject || b.checker.GetTypeAtLocation(n.Expression).Kind == checker.TypeClass
	if object {
		ref = b.expression(n.Expression)
	} else if s.Addressable || s.Kind == checker.StorageDynamic {
		ref = b.reference(n.Expression, s)
	} else {
		value := b.expression(n.Expression)
		t := b.types[value]
		slot := b.addSlot(node, nil, "$each.collection", t, ir.Temporary, false)
		b.store(node, slot, value)
		t.Form = ir.Reference
		ref = b.emit(ir.Instruction{Op: ir.OpAddress, Type: t, Slot: slot, Source: source(node)})
		clone = 1
	}
	if t := b.types[ref]; !ir.Array(t) && t.Scalar != ir.Variant && t.Scalar != ir.Object {
		b.unsupported(node, "For Each collections other than arrays")
		return
	}
	owners := b.retainReferenceObjects(node, ref)
	iterator := b.addSlot(node, nil, "$each.iterator", ir.Type{Scalar: ir.Enumerator}, ir.LoopState, false)
	b.enumOperation(node, ir.OpEnumStart, ir.Type{Scalar: ir.Void}, iterator, ref, b.literal(node, ir.Type{Scalar: ir.Long}, clone))
	parent := b.scope
	scope := ir.ScopeID(len(b.procedure.Scopes) + 1)
	b.procedure.Scopes = append(b.procedure.Scopes, ir.Scope{ID: scope, Parent: parent, Cleanup: append(owners, iterator)})
	done := b.block(node)
	b.scope = scope
	test, body := b.block(node), b.block(n.Statement)
	b.exits[node] = done
	b.jump(test)
	b.link(header, done, 0)
	b.activate(test)
	counter := b.reference(n.Initializer, b.checker.GetStorageAtLocation(n.Initializer))
	value := ir.ValueID(0)
	if counterType.Scalar == ir.Object {
		value = b.enumOperation(node, ir.OpEnumNext, ir.Type{Scalar: ir.Boolean}, iterator, b.variantReference(n.Initializer, counter))
	} else {
		value = b.enumOperation(node, ir.OpEnumNext, ir.Type{Scalar: ir.Boolean}, iterator, counter)
	}
	b.branch(value, body, done)
	b.link(test, done, 0)
	b.activate(body)
	b.sequence(n.Statement)
	b.fallthroughTo(test)
	b.scope = parent
	b.activate(done)
}
func (b *procedureBuilder) enumOperation(n *ast.Node, op ir.Op, t ir.Type, slot ir.SlotID, args ...ir.ValueID) ir.ValueID {
	r := t
	r.Form = ir.Result
	value := b.emit(ir.Instruction{Op: op, Type: r, Slot: slot, Args: args, Effects: ir.MayRaise | ir.ManagesLifetime | ir.WritesStorage, Source: source(n)})
	return b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{value}, OnError: ir.Propagate, Source: source(n)})
}

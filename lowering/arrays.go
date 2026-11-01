package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
)

func (b *procedureBuilder) reference(n *ast.Node, s *checker.Storage) ir.ValueID {
	if s != nil {
		if slot := b.captures[s.Node]; slot != 0 {
			check := b.emit(ir.Instruction{Op: ir.OpWithCheck, Slot: slot, Type: ir.Type{Scalar: ir.Void, Form: ir.Result}, Effects: ir.MayRaise, Source: source(n)})
			b.emit(ir.Instruction{Op: ir.OpTry, Type: ir.Type{Scalar: ir.Void}, Args: []ir.ValueID{check}, OnError: ir.Propagate, Source: source(n)})
			t := b.procedure.Slots[slot-1].Type
			t.Scalar = ir.Record
			t.Form = ir.Reference
			return b.emit(ir.Instruction{Op: ir.OpWithReference, Slot: slot, Type: t, Source: source(n)})
		}
	}
	if b.isRecordField(s) {
		return b.recordFieldReference(n, s)
	}
	if b.isClassField(s) {
		return b.classFieldReference(n, s)
	}
	if s != nil && s.Resolution != nil && s.Resolution.Kind == checker.CallArrayReference {
		return b.reference(s.Resolution.Target, b.checker.GetStorageAtLocation(s.Resolution.Target))
	}
	if s != nil && (s.Kind == checker.StorageArrayElement || s.Kind == checker.StorageDynamic && s.Resolution != nil && b.checker.GetTypeAtLocation(s.Resolution.Target).Kind == checker.TypeVariant) {
		r := s.Resolution
		if r == nil || !r.Valid || r.Prefix != nil || r.WithTarget != nil && !b.isRecordField(b.checker.GetStorageAtLocation(r.Target)) || len(r.Arguments) == 0 {
			b.unsupported(n, "unresolved array element storage")
			return 0
		}
		base := b.reference(r.Target, b.checker.GetStorageAtLocation(r.Target))
		if b.types[base].Scalar == ir.Variant {
			base = b.captureInvocation(n, base)
		}
		args := []ir.ValueID{base}
		for _, arg := range r.Arguments {
			if arg.Expression == nil || arg.StringValue != nil || arg.Argument.Kind == ast.KindNamedArgument || arg.Expression.Kind == ast.KindOmittedExpression {
				b.unsupported(n, "named/omitted array indices")
				return 0
			}
			args = append(args, b.coerce(arg.Expression, b.expression(arg.Expression), ir.Type{Scalar: ir.Long}))
		}
		t := b.scalar(n, s.Type, false)
		element := b.operation(n, ir.OpArrayIndex, ir.ElementOf(t), ir.MayRaise|ir.ManagesLifetime, args...)
		t.Form = ir.Reference
		ref := b.emit(ir.Instruction{Op: ir.OpElementRef, Type: t, Args: []ir.ValueID{element}, Source: source(n)})
		if b.referenceTokens == nil {
			b.referenceTokens = make(map[ir.ValueID][]ir.ValueID)
		}
		b.referenceTokens[ref] = append(append([]ir.ValueID(nil), b.referenceTokens[base]...), element)
		if owners := b.referenceObjects[base]; len(owners) > 0 {
			b.referenceObjects[ref] = append([]ir.ValueID(nil), owners...)
		}
		return ref
	}
	slot := b.storage(n, s)
	t := b.scalar(n, s.Type, false)
	t.Form = ir.Reference
	ref := b.emit(ir.Instruction{Op: ir.OpAddress, Type: t, Slot: slot, Source: source(n)})
	if slot != 0 {
		definition := b.procedure.Slots[slot-1]
		if definition.AutoClass != 0 || definition.AutoNew != 0 {
			b.setAutoReference(ref, definition.AutoClass, definition.AutoNew)
		}
	}
	return ref
}

func (b *procedureBuilder) readReference(n *ast.Node, ref ir.ValueID) ir.ValueID {
	if field, ok := b.interfaceReferences[ref]; ok {
		return b.interfaceCall(n, field.accessors[ast.KindGetAccessor], field.receiver)
	}
	t := b.types[ref]
	t.Form = ir.Value
	if auto := b.autoReferences[ref]; auto.class != 0 || auto.builtin != 0 {
		result := t
		result.Form = ir.Result
		id := b.emit(ir.Instruction{Op: ir.OpObjectAutoLoad, Class: auto.class, Integer: int64(auto.builtin), Type: result, Args: []ir.ValueID{ref}, Effects: ir.MayRaise | ir.ManagesLifetime | ir.WritesStorage, Source: source(n)})
		return b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)})
	}
	effects := ir.MayRaise
	if ir.Managed(t) {
		effects |= ir.ManagesLifetime
	}
	value := b.operation(n, ir.OpReadReference, t, effects, ref)
	// Once a value snapshot is acquired its array need not remain locked. A
	// ByRef argument keeps the token instead, through later arguments and calls.
	for i := len(b.referenceTokens[ref]) - 1; i >= 0; i-- {
		b.operation(n, ir.OpReleaseElement, ir.Type{Scalar: ir.Void}, ir.ManagesLifetime, b.referenceTokens[ref][i])
	}
	return value
}

func (b *procedureBuilder) writeReference(n *ast.Node, ref, value ir.ValueID) {
	if field, ok := b.interfaceReferences[ref]; ok {
		kind := ast.KindLetAccessor
		if b.types[value].Scalar == ir.Object || n.Kind == ast.KindSetStatement {
			kind = ast.KindSetAccessor
		}
		callee := field.accessors[kind]
		if callee == 0 {
			b.unsupported(n, "interface field assignment mode")
			return
		}
		b.interfaceCall(n, callee, field.receiver, value)
		return
	}
	effects := ir.MayRaise | ir.WritesStorage
	if ir.Managed(b.types[value]) {
		effects |= ir.ManagesLifetime
	}
	b.operation(n, ir.OpWriteReference, ir.Type{Scalar: ir.Void}, effects, ref, value)
}

func (b *procedureBuilder) resize(n *ast.Node, ref ir.ValueID, element *checker.Type, dimensions *ast.NodeList, preserve, fixed bool) {
	tag := int64(0)
	if element != nil && element.Kind != checker.TypeUnknown {
		tag = arrayTag(b.scalar(n, element, false))
	}
	flags := int64(0)
	if preserve {
		flags |= 1
	}
	if fixed {
		flags |= 2
	}
	args := []ir.ValueID{ref, b.literal(n, ir.Type{Scalar: ir.Long}, tag), b.literal(n, ir.Type{Scalar: ir.Long}, flags)}
	for _, bound := range dimensions.Nodes {
		d := bound.AsArrayDimension()
		lower := ir.ValueID(0)
		if d.LowerBound == nil {
			lower = b.literal(n, ir.Type{Scalar: ir.Long}, int64(ast.SourceFileOf(n).OptionBase))
		} else {
			lower = b.coerce(d.LowerBound, b.expression(d.LowerBound), ir.Type{Scalar: ir.Long})
		}
		upper := b.coerce(d.UpperBound, b.expression(d.UpperBound), ir.Type{Scalar: ir.Long})
		args = append(args, lower, upper)
	}
	in := ir.Instruction{Op: ir.OpArrayResize, Type: ir.Type{Scalar: ir.Void, Form: ir.Result}, Effects: ir.MayRaise | ir.WritesStorage | ir.ManagesLifetime, Args: args, Source: source(n)}
	if element != nil && (element.Kind == checker.TypeClass || element.Kind == checker.TypeObject) {
		in.Class, in.Integer = b.classes[element.Symbol], int64(virtualClass(element.Symbol))
	}
	id := b.emit(in)
	b.emit(ir.Instruction{Op: ir.OpTry, Type: ir.Type{Scalar: ir.Void}, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)})
}

func arrayTag(t ir.Type) int64 {
	switch t.Scalar {
	case ir.Object:
		return 0x1000
	case ir.Record:
		return 36
	case ir.Boolean:
		return 11
	case ir.Byte:
		return 17
	case ir.Integer:
		return 2
	case ir.Long:
		return 3
	case ir.Single:
		return 4
	case ir.Double:
		return 5
	case ir.Date:
		return 7
	case ir.Currency:
		return 6
	case ir.String:
		return 8
	case ir.Variant:
		return 12
	}
	return 0
}

func (b *procedureBuilder) reDim(n *ast.Node) {
	for _, declaration := range n.AsReDimStatement().Declarations.Nodes {
		r := b.checker.GetReDimResolution(declaration)
		if r == nil || !r.Valid || r.Storage.Kind != checker.StorageVariable && r.Storage.Kind != checker.StorageArrayElement && r.Storage.Kind != checker.StorageDynamic {
			b.unsupported(declaration, "unresolved ReDim storage")
			continue
		}
		d := declaration.AsReDimDeclaration()
		ref := b.reference(d.Expression, r.Storage)
		b.resize(declaration, ref, r.ArrayType.ElementType, d.Dimensions, r.Preserve, false)
	}
}

func (b *procedureBuilder) arrayBound(n *ast.Node, r *checker.CallResolution) ir.ValueID {
	var ref, dimension ir.ValueID
	for _, arg := range r.Arguments {
		if arg.ParameterIndex == 0 {
			s := b.checker.GetStorageAtLocation(arg.Expression)
			if s.Addressable {
				ref = b.reference(arg.Expression, s)
			} else {
				value := b.expression(arg.Expression)
				t := b.types[value]
				if !ir.Array(t) && t.Scalar != ir.Variant {
					b.unsupported(n, "bounds of non-array values")
					return 0
				}
				slot := b.addSlot(n, nil, "$bounds", t, ir.Temporary, false)
				b.store(n, slot, value)
				t.Form = ir.Reference
				ref = b.emit(ir.Instruction{Op: ir.OpAddress, Type: t, Slot: slot, Source: source(n)})
			}
		} else if arg.Expression != nil {
			dimension = b.coerce(arg.Expression, b.expression(arg.Expression), ir.Type{Scalar: ir.Long})
		}
	}
	if dimension == 0 {
		dimension = b.literal(n, ir.Type{Scalar: ir.Long}, 1)
	}
	upper := int64(0)
	if r.Signature.Declaration.Symbol.Library == stdlib.UBound {
		upper = 1
	}
	return b.operation(n, ir.OpArrayBound, ir.Type{Scalar: ir.Long}, ir.MayRaise, ref, dimension, b.literal(n, ir.Type{Scalar: ir.Long}, upper))
}

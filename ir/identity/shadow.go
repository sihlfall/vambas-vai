package identity

import "sihlfall.ch/vambas-vai/ir"

// bindInstruction records an instruction's shadow operation. Identity-
// preserving operations add no node. New or unsummarized operations default to
// the unknown root: they may invalidate or observe any tracked storage.
func (b *binder) bindInstruction(s *ir.Statement, index int, in ir.Instruction) {
	arg := func(i int) root {
		if i < len(in.Args) {
			return b.root(in.Args[i])
		}
		return unknownRoot
	}
	var effects []effect
	if ir.MayInvokeDefault(in) || in.Effects&ir.CallsExternal != 0 {
		effects = append(effects, b.reentry(in)...)
	}
	switch in.Op {
	case ir.OpAddress:
		b.roots[in.ID] = b.slotRoot(in.Slot)
	case ir.OpFieldRef, ir.OpVariantRef, ir.OpElementRef:
		b.roots[in.ID] = arg(0)
	case ir.OpWithReference:
		b.roots[in.ID] = b.slotRoot(in.Slot)
	case ir.OpClassFieldRef:
		b.roots[in.ID] = root(ambient)
	case ir.OpTry:
		if r, ok := b.roots[in.Args[0]]; ok && (in.Type.Form == ir.Reference || ir.Element(in.Type)) {
			b.roots[in.ID] = r
		}
		if ir.Element(in.Type) && in.Type.Form == ir.Value {
			node := b.newFlowNode(FlowAcquire, s, index)
			b.graph.acquired[s.ID-1][in.ID] = node
		}
	case ir.OpArrayIndex:
		// Acquiring a lock reads the lock count (error 10 at saturation).
		b.roots[in.ID] = arg(0)
		effects = append(effects, effect{kind: observe, location: arg(0)})
		if b.types[in.Args[0]].Scalar == ir.Variant {
			effects = append(effects, replace(root(ambient))...) // Erased/default-member indexing.
		}
	case ir.OpInvocationCapture:
		// Retains an erased receiver or locks its array before later arguments.
		b.roots[in.ID] = arg(0)
		effects = append(effects, effect{kind: observe, location: arg(0)})
		effects = append(effects, replace(root(ambient))...)
	case ir.OpArrayResize, ir.OpArrayErase:
		// Both check for live locks (error 10) and may free the backing storage.
		effects = append(effects, replace(arg(0))...)
		if objectBearing(ir.ElementType(b.types[in.Args[0]])) || b.types[in.Args[0]].Scalar == ir.Variant {
			b.shadow(s, index, effects...)
			b.release(s, index)
			return
		}
	case ir.OpParamArrayPack:
		// Releases the previous bindings, then holds references to the actuals.
		effects = append(effects, replace(arg(0))...)
		for i := 1; i < len(in.Args); i++ {
			if b.types[in.Args[i]].Form == ir.Reference {
				effects = append(effects, b.bindTo(arg(0), arg(i))...)
			}
		}
		b.shadow(s, index, effects...)
		b.release(s, index)
		return
	case ir.OpStoreManaged:
		t := ir.Type{}
		if in.Slot != 0 && int(in.Slot) <= len(b.procedure.Slots) {
			t = b.procedure.Slots[in.Slot-1].Type
		}
		if shapeBearing(t) {
			effects = append(effects, replace(b.slotRoot(in.Slot))...)
		}
		if objectBearing(t) {
			b.shadow(s, index, effects...)
			b.release(s, index)
			return
		}
	case ir.OpWriteReference:
		t := b.types[in.Args[0]]
		if shapeBearing(t) {
			effects = append(effects, replace(arg(0))...)
		}
		if objectBearing(t) {
			b.shadow(s, index, effects...)
			b.release(s, index)
			return
		}
	case ir.OpEnumStart:
		// The enumerator locks an array and keeps a reference into it.
		effects = append(effects, effect{kind: observe, location: arg(0)})
		effects = append(effects, b.bindTo(b.slotRoot(in.Slot), arg(0))...)
		if t := b.types[in.Args[0]]; t.Scalar == ir.Object || t.Scalar == ir.Variant {
			effects = append(effects, replace(root(ambient))...)
		}
	case ir.OpEnumNext, ir.OpEnumObjectNext:
		effects = append(effects, replace(arg(0))...)
		if in.Op == ir.OpEnumObjectNext {
			effects = append(effects, replace(root(ambient))...)
		}
		b.shadow(s, index, effects...)
		b.release(s, index)
		return
	case ir.OpKeepElement, ir.OpWithCapture:
		effects = append(effects, b.bindTo(b.slotRoot(in.Slot), arg(0))...)
	case ir.OpCall, ir.OpInterfaceCall:
		b.shadow(s, index, effects...)
		node := b.newFlowNode(FlowCall, s, index)
		if callee := int(in.Callee); callee > 0 && callee <= len(b.program.Procedures) {
			node.callee = callee - 1
		}
		for _, id := range in.Args {
			node.actuals = append(node.actuals, b.root(id))
		}
		return
	case ir.OpClassNew, ir.OpObjectAutoLoad, ir.OpCreateObject, ir.OpObjectInvoke, ir.OpObjectValue, ir.OpCollectionInvoke,
		ir.OpErrRaise, ir.OpErrSet, ir.OpTestAssert:
		// Construction, erased/native dispatch, and Variant coercion through a
		// default member can reenter VBA. Releasing old values may terminate.
		effects = append(effects, b.reentry(in)...)
		b.shadow(s, index, effects...)
		b.release(s, index)
		return
	case ir.OpLoadManaged:
		if in.Slot != 0 && int(in.Slot) <= len(b.procedure.Slots) {
			if slot := b.procedure.Slots[in.Slot-1]; slot.AutoClass != 0 || slot.AutoNew != 0 {
				effects = append(effects, b.reentry(in)...) // Lazy construction.
			}
		}
	case ir.OpConstant, ir.OpLoad, ir.OpStore, ir.OpConvert, ir.OpAdd, ir.OpSubtract, ir.OpMultiply,
		ir.OpNegate, ir.OpNot, ir.OpAnd, ir.OpOr, ir.OpXor, ir.OpEqual, ir.OpNotEqual, ir.OpLess,
		ir.OpLessEqual, ir.OpGreater, ir.OpGreaterEqual, ir.OpStringLiteral, ir.OpNullString,
		ir.OpConcat, ir.OpLen, ir.OpLenB, ir.OpChrW, ir.OpMid, ir.OpMidCheck, ir.OpMidAssign,
		ir.OpBox, ir.OpUnbox, ir.OpVariantConstant, ir.OpPinString, ir.OpVarType, ir.OpIsEmpty,
		ir.OpIsNull, ir.OpIsMissing, ir.OpReadReference, ir.OpArrayBound, ir.OpIsArray,
		ir.OpReleaseElement, ir.OpCondition, ir.OpCheckInitialized, ir.OpForValidate, ir.OpForTest,
		ir.OpCheckDispatch, ir.OpDivide, ir.OpIntegerDivide, ir.OpModulo, ir.OpWithCheck,
		ir.OpErrClear, ir.OpErrGet, ir.OpObjectNew, ir.OpObjectNull, ir.OpObjectEqual,
		ir.OpObjectCheck, ir.OpGosubPush, ir.OpGosubPop, ir.OpClassCheck, ir.OpClassCast,
		ir.OpInterfaceCheck, ir.OpLibraryCall:
		// Identity-preserving, apart from default-member reentry above. String
		// content writes and element unlocks do not replace descriptors.
	default:
		effects = append(effects, replace(unknownRoot)...)
	}
	b.shadow(s, index, effects...)
}

func (b *binder) bindTo(slot, target root) []effect {
	if slot < 0 {
		return replace(unknownRoot)
	}
	return []effect{{kind: bind, location: slot, target: target}}
}

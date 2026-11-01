package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
)

func (l *lowerer) declareRecords(files []*ast.SourceFile) {
	l.records = make(map[*ast.Symbol]ir.RecordID)
	for _, file := range files {
		for _, node := range nodes(file.Statements) {
			if node.Kind != ast.KindTypeDeclaration {
				continue
			}
			id := ir.RecordID(len(l.program.Records) + 1)
			l.records[node.Symbol] = id
			l.program.Records = append(l.program.Records, ir.RecordDefinition{ID: id, Name: file.ModuleName + "." + node.AsTypeDeclaration().Name.AsIdentifier().Text, Source: source(node)})
		}
	}
	for i := range l.program.Records {
		r := &l.program.Records[i]
		for _, field := range r.Source.Node.AsTypeDeclaration().Members.Nodes {
			d := field.AsFieldDeclaration()
			if fixedString(d.Type) {
				l.unsupported(field, "fixed-length String UDT fields")
			}
			f := ir.RecordField{Name: d.Name.AsIdentifier().Text, Type: l.scalar(field, l.checker.GetTypeOfSymbol(field.Symbol), false), Source: source(field)}
			for _, bound := range l.checker.GetArrayBounds(field) {
				f.Bounds = append(f.Bounds, ir.ArrayBound{Lower: bound.Lower, Upper: bound.Upper})
			}
			r.Fields = append(r.Fields, f)
		}
	}
}

func (b *procedureBuilder) isRecordField(s *checker.Storage) bool {
	return s != nil && s.Kind == checker.StorageVariable && s.Receiver != nil && s.Symbol != nil && s.Symbol.ValueDeclaration != nil && s.Symbol.ValueDeclaration.Kind == ast.KindFieldDeclaration && b.checker.GetTypeAtLocation(s.Receiver).Kind == checker.TypeStructure
}

func (b *procedureBuilder) recordFieldReference(n *ast.Node, s *checker.Storage) ir.ValueID {
	baseStorage := b.checker.GetStorageAtLocation(s.Receiver)
	var base ir.ValueID
	if b.captures[s.Receiver] != 0 || baseStorage != nil && baseStorage.Addressable {
		// Use the field access as provenance: an omitted With receiver's
		// original syntax precedes this statement, but its address is stable.
		base = b.reference(n, baseStorage)
	} else {
		base = b.expression(s.Receiver)
	}
	t := b.types[base]
	if t.Scalar != ir.Record || t.Record == 0 {
		b.unsupported(n, "unresolved UDT receiver")
		return 0
	}
	definition := b.program.Records[t.Record-1]
	for i, f := range definition.Fields {
		if f.Source.Node.Symbol == s.Symbol {
			fieldType := f.Type
			fieldType.Form = ir.Reference
			ref := b.emit(ir.Instruction{Op: ir.OpFieldRef, Type: fieldType, Args: []ir.ValueID{base}, Field: uint32(i), Source: source(n)})
			if tokens := b.referenceTokens[base]; len(tokens) > 0 {
				b.referenceTokens[ref] = append([]ir.ValueID(nil), tokens...)
			}
			if owners := b.referenceObjects[base]; len(owners) > 0 {
				b.referenceObjects[ref] = append([]ir.ValueID(nil), owners...)
			}
			return ref
		}
	}
	b.unsupported(n, "unresolved UDT field")
	return 0
}

// UDT variables and nested fields have stable inline storage. Their address
// calculation has no side effects, so omitted With receivers may reuse it.
// Effectful/value receivers require a separate captured-lifetime scope.
func (b *procedureBuilder) stableRecordStorage(n *ast.Node) bool {
	s := b.checker.GetStorageAtLocation(n)
	if s == nil || !s.Addressable {
		return false
	}
	if b.isRecordField(s) {
		return b.stableRecordStorage(s.Receiver)
	}
	return s.Kind == checker.StorageVariable && (s.Receiver == nil || b.checker.GetTypeAtLocation(s.Receiver).Kind == checker.TypeModule) && s.WithTarget == nil && (b.slots[s.Symbol] != 0 || b.globals[s.Symbol] != 0)
}
func (b *procedureBuilder) recordWith(node *ast.Node) {
	n := node.AsWithStatement()
	t := b.scalar(node, b.checker.GetTypeAtLocation(n.Expression), false)
	if t.Scalar != ir.Record {
		b.unsupported(node, "non-UDT With receiver")
		return
	}
	header, parent := b.statement, b.scope
	var cleanup []ir.SlotID
	storage := b.checker.GetStorageAtLocation(n.Expression)
	var ref ir.ValueID
	if storage != nil && storage.Addressable {
		ref = b.reference(n.Expression, storage)
	} else {
		value := b.expression(n.Expression)
		slot := b.addSlot(node, nil, "$with.value", t, ir.LoopState, false)
		b.store(node, slot, value)
		cleanup = append(cleanup, slot)
		tr := t
		tr.Form = ir.Reference
		ref = b.emit(ir.Instruction{Op: ir.OpAddress, Slot: slot, Type: tr, Source: source(node)})
	}
	cleanup = append(cleanup, b.retainReferenceObjects(node, ref)...)
	for _, token := range b.referenceTokens[ref] {
		slot := b.addSlot(node, nil, "$with.lock", b.types[token], ir.LoopState, false)
		b.emit(ir.Instruction{Op: ir.OpKeepElement, Type: ir.Type{Scalar: ir.Void}, Slot: slot, Args: []ir.ValueID{token}, Effects: ir.ManagesLifetime | ir.WritesStorage, Source: source(node)})
		cleanup = append(cleanup, slot)
	}
	receiver := b.addSlot(node, nil, "$with", ir.Type{Scalar: ir.Receiver, Record: t.Record}, ir.LoopState, false)
	cleanup = append(cleanup, receiver)
	b.emit(ir.Instruction{Op: ir.OpWithCapture, Type: ir.Type{Scalar: ir.Void}, Slot: receiver, Args: []ir.ValueID{ref}, Effects: ir.WritesStorage, Source: source(node)})
	if b.captures == nil {
		b.captures = make(map[*ast.Node]ir.SlotID)
	}
	b.captures[n.Expression] = receiver
	done, body := b.block(node), b.block(n.Statement)
	b.scope = ir.ScopeID(len(b.procedure.Scopes) + 1)
	b.procedure.Scopes = append(b.procedure.Scopes, ir.Scope{ID: b.scope, Parent: parent, Cleanup: cleanup})
	b.jump(body)
	b.link(header, done, 0)
	b.activate(body)
	b.sequence(n.Statement)
	b.fallthroughTo(done)
	delete(b.captures, n.Expression)
	b.scope = parent
	b.activate(done)
}

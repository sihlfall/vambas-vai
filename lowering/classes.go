package lowering

import (
	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
)

type classField struct {
	class ir.ClassID
	index uint32
}

func (l *lowerer) declareClassIdentities(files []*ast.SourceFile) {
	l.classes = make(map[*ast.Symbol]ir.ClassID)
	l.classFields = make(map[*ast.Symbol]classField)
	for _, file := range files {
		if file.ModuleKind != ast.ModuleKindClass {
			continue
		}
		id := ir.ClassID(len(l.program.Classes) + 1)
		l.classes[file.Symbol] = id
		l.program.Classes = append(l.program.Classes, ir.ClassDefinition{ID: id, Name: file.ModuleName, Source: source(file.AsNode())})
	}
}

func (l *lowerer) declareClassFields(files []*ast.SourceFile) {
	for _, file := range files {
		id := l.classes[file.Symbol]
		if id == 0 {
			continue
		}
		class := &l.program.Classes[id-1]
		for _, statement := range nodes(file.Statements) {
			if statement.Kind != ast.KindVariableStatement || statement.AsVariableStatement().Keyword == ast.KindCONST {
				continue
			}
			for _, node := range nodes(statement.AsVariableStatement().DeclarationList.AsVariableDeclarationList().Declarations) {
				d := node.AsVariableDeclaration()
				if fixedString(d.Type) || d.Initializer != nil {
					l.unsupported(node, "fixed-length or explicitly initialized class fields")
				}
				t := l.scalar(node, l.checker.GetTypeOfSymbol(node.Symbol), false)
				declared := l.checker.GetTypeOfSymbol(node.Symbol)
				field := ir.RecordField{Name: d.Name.AsIdentifier().Text, Type: t, Source: source(node), AutoClass: l.autoClass(node), AutoNew: l.autoNew(node), Class: l.classes[declared.Symbol], Builtin: virtualClass(declared.Symbol), Public: file.Symbol.Exports.Lookup(d.Name.AsIdentifier().Text) == node.Symbol}
				for _, bound := range l.checker.GetArrayBounds(node) {
					field.Bounds = append(field.Bounds, ir.ArrayBound{Lower: bound.Lower, Upper: bound.Upper})
				}
				l.classFields[node.Symbol] = classField{class: id, index: uint32(len(class.Fields))}
				class.Fields = append(class.Fields, field)
			}
		}
	}
}

func (b *procedureBuilder) declareInvocation(sig *checker.Signature) {
	decl := b.procedure.Source.Node.AsFunctionDeclaration()
	if b.procedure.Class == 0 || decl.Modifiers()&(ast.ModifierPrivate|ast.VisibiltiyFriend) != 0 {
		return
	}
	kind := ir.InvokeMethod
	switch b.procedure.Source.Node.Kind {
	case ast.KindGetAccessor:
		kind = ir.InvokeGet
	case ast.KindLetAccessor:
		kind = ir.InvokeLet
	case ast.KindSetAccessor:
		kind = ir.InvokeSet
	}
	class := b.procedure.Class
	defaultMember := b.checker.GetDefaultMember(b.checker.GetTypeOfSymbol(b.program.Classes[class-1].Source.Node.Symbol))
	contract := &ir.InvocationSignature{Name: decl.Name().AsIdentifier().Text, Kind: kind, Default: defaultMember == b.procedure.Source.Node.Symbol}
	for i, p := range sig.Parameters {
		parameter := ir.InvocationParameter{Name: p.Name, Type: b.scalar(p.Declaration, p.Type, false), Class: b.classes[p.Type.Symbol], Builtin: virtualClass(p.Type.Symbol), ByRef: p.IsByRef(), Optional: p.Modifiers&ast.ParameterOptional != 0, ParamArray: p.Modifiers&ast.ParameterParamArray != 0}
		if (kind == ir.InvokeLet || kind == ir.InvokeSet) && i == len(sig.Parameters)-1 {
			parameter.ByRef = false
		}
		if parameter.Optional && !(p.Type.Kind == checker.TypeVariant && p.Default == nil) {
			helper := &ir.Procedure{ID: ir.ProcedureID(len(b.program.Procedures) + 1), Name: b.procedure.Name + ".$default." + p.Name, Source: source(p.Declaration), ReturnType: parameter.Type}
			d := &procedureBuilder{lowerer: b.lowerer, procedure: helper, slots: make(map[*ast.Symbol]ir.SlotID), types: make(map[ir.ValueID]ir.Type)}
			helper.ResultSlot = d.addSlot(p.Declaration, nil, "$result", parameter.Type, ir.FunctionResult, false)
			d.newStatement(p.Declaration)
			value := d.defaultArgument(p.Declaration, p)
			d.store(p.Declaration, helper.ResultSlot, value)
			d.statement.End = ir.Return
			parameter.Default = helper.ID
			b.program.Procedures = append(b.program.Procedures, helper)
		}
		contract.Parameters = append(contract.Parameters, parameter)
	}
	b.procedure.Invocation = contract
}

func (b *procedureBuilder) classOperation(n *ast.Node, op ir.Op, class ir.ClassID, t ir.Type, effects ir.Effects, args ...ir.ValueID) ir.ValueID {
	if effects&ir.MayRaise != 0 {
		t.Form = ir.Result
	}
	id := b.emit(ir.Instruction{Op: op, Class: class, Type: t, Effects: effects, Args: args, Source: source(n)})
	if t.Form == ir.Result {
		t.Form = ir.Value
		return b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)})
	}
	return id
}

func (b *procedureBuilder) me(n *ast.Node) ir.ValueID {
	if b.procedure.Receiver == 0 {
		b.unsupported(n, "Me outside an instance member")
		return 0
	}
	return b.load(n, b.procedure.Receiver)
}

func (b *procedureBuilder) isClassField(s *checker.Storage) bool {
	return s != nil && s.Kind == checker.StorageVariable && b.classFields[s.Symbol].class != 0
}

func (b *procedureBuilder) classFieldReference(n *ast.Node, s *checker.Storage) ir.ValueID {
	field := b.classFields[s.Symbol]
	var receiver ir.ValueID
	if s.Receiver != nil || s.WithTarget != nil {
		receiver = b.objectReceiver(n, s.Receiver, s.WithTarget)
	} else {
		receiver = b.me(n)
	}
	if accessors := b.interfaceFields[s.Symbol]; accessors != nil && (s.Receiver != nil || s.WithTarget != nil) {
		return b.interfaceFieldReference(n, receiver, accessors)
	}
	b.classOperation(n, ir.OpClassCheck, field.class, ir.Type{Scalar: ir.Void}, ir.MayRaise, receiver)
	t := b.program.Classes[field.class-1].Fields[field.index].Type
	t.Form = ir.Reference
	ref := b.emit(ir.Instruction{Op: ir.OpClassFieldRef, Class: field.class, Field: field.index, Type: t, Args: []ir.ValueID{receiver}, Source: source(n)})
	if b.referenceObjects == nil {
		b.referenceObjects = make(map[ir.ValueID][]ir.ValueID)
	}
	b.referenceObjects[ref] = []ir.ValueID{receiver}
	f := b.program.Classes[field.class-1].Fields[field.index]
	if f.AutoClass != 0 || f.AutoNew != 0 {
		b.setAutoReference(ref, f.AutoClass, f.AutoNew)
	}
	return ref
}

type autoReference struct {
	class   ir.ClassID
	builtin uint8
}

func (b *procedureBuilder) setAutoReference(ref ir.ValueID, class ir.ClassID, builtin uint8) {
	if b.autoReferences == nil {
		b.autoReferences = make(map[ir.ValueID]autoReference)
	}
	b.autoReferences[ref] = autoReference{class, builtin}
}

// A reference captured across statement boundaries must keep its instance
// alive until after any array locks or With addresses have been released.
func (b *procedureBuilder) retainReferenceObjects(n *ast.Node, ref ir.ValueID) []ir.SlotID {
	var slots []ir.SlotID
	for _, owner := range b.referenceObjects[ref] {
		slot := b.addSlot(n, nil, "$field.owner", ir.Type{Scalar: ir.Object}, ir.LoopState, false)
		b.store(n, slot, owner)
		slots = append(slots, slot)
	}
	return slots
}

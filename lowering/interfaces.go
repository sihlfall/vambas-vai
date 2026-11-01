package lowering

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/ir"
)

type interfaceReference struct {
	receiver  ir.ValueID
	accessors map[ast.Kind]ir.ProcedureID
}

func (b *procedureBuilder) interfaceFieldReference(n *ast.Node, receiver ir.ValueID, accessors map[ast.Kind]ir.ProcedureID) ir.ValueID {
	t := b.program.Procedures[accessors[ast.KindGetAccessor]-1].ReturnType
	slot := b.addSlot(n, nil, "$interface.field", t, ir.Temporary, false)
	t.Form = ir.Reference
	ref := b.emit(ir.Instruction{Op: ir.OpAddress, Slot: slot, Type: t, Source: source(n)})
	if b.interfaceReferences == nil {
		b.interfaceReferences = make(map[ir.ValueID]interfaceReference)
	}
	b.interfaceReferences[ref] = interfaceReference{receiver: receiver, accessors: accessors}
	if b.referenceObjects == nil {
		b.referenceObjects = make(map[ir.ValueID][]ir.ValueID)
	}
	b.referenceObjects[ref] = []ir.ValueID{receiver}
	return ref
}

func (l *lowerer) declareInterfaces(files []*ast.SourceFile) {
	l.interfaceClasses = make(map[ir.ClassID]bool)
	l.interfaceFields = make(map[*ast.Symbol]map[ast.Kind]ir.ProcedureID)
	for _, file := range files {
		for _, implementation := range l.checker.GetImplementedInterfaces(file.Symbol) {
			class, target := l.classes[file.Symbol], l.classes[implementation.Interface]
			if class == 0 || target == 0 {
				l.unsupported(implementation.Declaration, "host or external interfaces")
				continue
			}
			l.interfaceClasses[target] = true
			entry := ir.InterfaceImplementation{Class: target}
			for _, member := range implementation.Members {
				var declaration ir.ProcedureID
				if member.Declaration.Kind == ast.KindVariableDeclaration {
					accessors := l.interfaceFields[member.Declaration.Symbol]
					if accessors == nil {
						accessors = make(map[ast.Kind]ir.ProcedureID)
						l.interfaceFields[member.Declaration.Symbol] = accessors
					}
					declaration = accessors[member.Kind]
					if declaration == 0 {
						declaration = l.interfaceFieldAccessor(member.Declaration, member.Kind)
						accessors[member.Kind] = declaration
					}
				} else {
					declaration = l.procedures[member.Declaration].ID
				}
				entry.Methods = append(entry.Methods, ir.InterfaceMethod{Declaration: declaration, Implementation: l.procedures[member.Implementation].ID})
			}
			l.program.Classes[class-1].Interfaces = append(l.program.Classes[class-1].Interfaces, entry)
		}
	}
}

// A public field contributes Get and Let/Set contracts to its default interface.
// These fallback accessors operate on an actual instance of the declaring class;
// an implemented view dispatches to the private property handlers instead.
func (l *lowerer) interfaceFieldAccessor(n *ast.Node, kind ast.Kind) ir.ProcedureID {
	field := l.classFields[n.Symbol]
	f := l.program.Classes[field.class-1].Fields[field.index]
	p := &ir.Procedure{ID: ir.ProcedureID(len(l.program.Procedures) + 1), Name: fmt.Sprintf("%s.$field.%s.%d", l.program.Classes[field.class-1].Name, f.Name, kind), Class: field.class, Source: source(n), ReturnType: ir.Type{Scalar: ir.Void}}
	b := &procedureBuilder{lowerer: l, procedure: p, slots: make(map[*ast.Symbol]ir.SlotID), types: make(map[ir.ValueID]ir.Type)}
	p.Receiver = b.addSlot(n, nil, "$me", ir.Type{Scalar: ir.Object}, ir.Parameter, false)
	p.Parameters = append(p.Parameters, p.Receiver)
	if kind == ast.KindGetAccessor {
		p.ReturnType = f.Type
		p.ResultSlot = b.addSlot(n, nil, "$result", f.Type, ir.FunctionResult, false)
	} else {
		p.Parameters = append(p.Parameters, b.addSlot(n, nil, "$value", f.Type, ir.Parameter, false))
	}
	// The syntax belongs to the field; As New applies to its payload, never
	// to the synthetic accessor's receiver, result, or value parameter.
	for i := range p.Slots {
		p.Slots[i].AutoClass, p.Slots[i].AutoNew = 0, 0
	}
	b.newStatement(n)
	receiver := b.me(n)
	b.classOperation(n, ir.OpClassCheck, field.class, ir.Type{Scalar: ir.Void}, ir.MayRaise, receiver)
	t := f.Type
	t.Form = ir.Reference
	ref := b.emit(ir.Instruction{Op: ir.OpClassFieldRef, Class: field.class, Field: field.index, Type: t, Args: []ir.ValueID{receiver}, Source: source(n)})
	if kind == ast.KindGetAccessor {
		if f.AutoClass != 0 || f.AutoNew != 0 {
			b.setAutoReference(ref, f.AutoClass, f.AutoNew)
		}
		b.store(n, p.ResultSlot, b.readReference(n, ref))
	} else {
		b.writeReference(n, ref, b.load(n, p.Parameters[1]))
	}
	b.statement.End = ir.Return
	l.program.Procedures = append(l.program.Procedures, p)
	l.program.Classes[field.class-1].Members = append(l.program.Classes[field.class-1].Members, p.ID)
	return p.ID
}

func (b *procedureBuilder) interfaceCall(n *ast.Node, callee ir.ProcedureID, args ...ir.ValueID) ir.ValueID {
	p := b.program.Procedures[callee-1]
	b.classOperation(n, ir.OpInterfaceCheck, p.Class, ir.Type{Scalar: ir.Void}, ir.MayRaise, args[0])
	t := p.ReturnType
	t.Form = ir.Result
	id := b.emit(ir.Instruction{Op: ir.OpInterfaceCall, Type: t, Callee: callee, Args: args, Effects: ir.MayRaise, Source: source(n)})
	t.Form = ir.Value
	return b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)})
}

// Package lowering translates checked VBA syntax into typed semantic IR. Like
// typescript-go's consumers of checker results, it keeps semantics out of the AST.
// VBA's Result values and storage aliases require a separate, explicit IR layer.
package lowering

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf16"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/stdlib"
)

// Host is implemented by compiler.Program. Lowering performs no filesystem I/O.
type Host interface {
	Name() string
	SourceFiles() []*ast.SourceFile
	ParseDiagnostics() []*ast.Diagnostic
	BindDiagnostics() []ast.BindingDiagnostic
	GetTypeChecker() *checker.Checker
}

const (
	DiagnosticUnsupported ast.DiagnosticCode = "unsupported-lowering"
	DiagnosticInvalidIR   ast.DiagnosticCode = "invalid-semantic-ir"
)

type lowerer struct {
	checker          *checker.Checker
	program          *ir.Program
	procedures       map[*ast.Node]*ir.Procedure
	records          map[*ast.Symbol]ir.RecordID
	globals          map[*ast.Symbol]ir.GlobalID
	classes          map[*ast.Symbol]ir.ClassID
	classFields      map[*ast.Symbol]classField
	interfaceClasses map[ir.ClassID]bool
	interfaceFields  map[*ast.Symbol]map[ast.Kind]ir.ProcedureID
	diagnostics      []ast.Diagnostic
}

// Lower returns either a complete verified program or diagnostics and nil IR.
// All input modules/procedures are checked, including unreachable declarations.
// Language-server recovery remains available, but never becomes executable code.
func Lower(host Host) (*ir.Program, []ast.Diagnostic) {
	l := &lowerer{checker: host.GetTypeChecker(), program: &ir.Program{Name: host.Name()}, procedures: make(map[*ast.Node]*ir.Procedure)}
	for _, d := range host.ParseDiagnostics() {
		l.diagnostics = append(l.diagnostics, *d)
	}
	for _, d := range host.BindDiagnostics() {
		l.diagnostics = append(l.diagnostics, ast.Diagnostic{File: d.File, Loc: d.Loc, Code: ast.DiagnosticCode(d.Code), Message: d.Message})
	}
	for _, d := range l.checker.GetDiagnostics(nil) {
		l.diagnostics = append(l.diagnostics, d.Diagnostic)
	}
	if len(l.diagnostics) != 0 {
		return nil, l.diagnostics
	}
	l.declareClassIdentities(host.SourceFiles())
	l.declareRecords(host.SourceFiles())
	l.declareClassFields(host.SourceFiles())
	l.globals = make(map[*ast.Symbol]ir.GlobalID)
	for _, file := range host.SourceFiles() {
		if id := l.classes[file.Symbol]; id != 0 && file.PredeclaredID {
			g := ir.Global{ID: ir.GlobalID(len(l.program.Globals) + 1), Name: file.ModuleName, Type: ir.Type{Scalar: ir.Object}, AutoClass: id, Source: source(file.AsNode())}
			l.globals[file.Symbol] = g.ID
			l.program.Globals = append(l.program.Globals, g)
		}
	}
	for _, file := range host.SourceFiles() {
		var checkRecovery func(*ast.Node)
		checkRecovery = func(n *ast.Node) {
			if n.Flags&(ast.NodeFlagsThisNodeHasError|ast.NodeFlagsThisNodeOrAnySubNodesHasError) != 0 {
				l.unsupported(n, "recovered syntax")
				return
			}
			n.ForEachChild(func(child *ast.Node) bool { checkRecovery(child); return false })
		}
		checkRecovery(file.AsNode())
		if file.ModuleKind != ast.ModuleKindStandard && file.ModuleKind != ast.ModuleKindClass {
			l.unsupported(file.AsNode(), "host modules")
			continue
		}
		for _, n := range nodes(file.Statements) {
			switch n.Kind {
			case ast.KindOptionExplicitStatement, ast.KindOptionBaseStatement, ast.KindOptionPrivateModuleStatement, ast.KindAttributeStatement, ast.KindEnumDeclaration, ast.KindTypeDeclaration, ast.KindImplementsStatement:
				// Metadata and checked enum values require no executable operations.
			case ast.KindVariableStatement:
				if n.AsVariableStatement().Keyword == ast.KindCONST {
					l.constantDeclarations(n)
				} else {
					if file.ModuleKind == ast.ModuleKindStandard {
						l.declareGlobals(n)
					}
				}
			case ast.KindFunctionDeclaration, ast.KindSubDeclaration, ast.KindGetAccessor, ast.KindLetAccessor, ast.KindSetAccessor:
				p := &ir.Procedure{ID: ir.ProcedureID(len(l.program.Procedures) + 1), Name: file.ModuleName + "." + n.AsFunctionDeclaration().Name().AsIdentifier().Text, Source: source(n)}
				if id := l.classes[file.Symbol]; id != 0 {
					p.Class = id
					l.program.Classes[id-1].Members = append(l.program.Classes[id-1].Members, p.ID)
					if strings.EqualFold(n.AsFunctionDeclaration().Name().AsIdentifier().Text, "Class_Initialize") || strings.EqualFold(n.AsFunctionDeclaration().Name().AsIdentifier().Text, "Class_Terminate") {
						if n.Kind != ast.KindSubDeclaration || len(nodes(n.AsFunctionDeclaration().Parameters)) != 0 {
							l.unsupported(n, "lifecycle handlers must be parameterless Subs")
						}
						if strings.EqualFold(n.AsFunctionDeclaration().Name().AsIdentifier().Text, "Class_Initialize") {
							l.program.Classes[id-1].Initialize = p.ID
						} else {
							l.program.Classes[id-1].Terminate = p.ID
						}
					}
				}
				l.program.Procedures = append(l.program.Procedures, p)
				l.procedures[n] = p
			default:
				l.unsupported(n, "module declaration")
			}
		}
	}
	// Declare every signature before lowering any body, including recursive calls.
	builders := make([]*procedureBuilder, 0, len(l.program.Procedures))
	for _, p := range l.program.Procedures {
		b := &procedureBuilder{lowerer: l, procedure: p, slots: make(map[*ast.Symbol]ir.SlotID), types: make(map[ir.ValueID]ir.Type)}
		b.declare()
		builders = append(builders, b)
	}
	l.declareInterfaces(host.SourceFiles())
	if len(l.diagnostics) != 0 {
		return nil, l.diagnostics
	}
	for _, b := range builders {
		b.body()
	}
	if len(l.diagnostics) != 0 {
		return nil, l.diagnostics
	}
	inferEffects(l.program)
	for _, d := range ir.Verify(l.program) {
		l.diagnostics = append(l.diagnostics, ast.Diagnostic{File: ast.SourceFileOf(d.Source.Node), Loc: d.Source.Range, Code: DiagnosticInvalidIR, Message: d.Message})
	}
	if len(l.diagnostics) != 0 {
		return nil, l.diagnostics
	}
	return l.program, nil
}

func nodes(list *ast.NodeList) []*ast.Node {
	if list == nil {
		return nil
	}
	return list.Nodes
}

func source(n *ast.Node) ir.Source {
	return ir.Source{FileName: ast.SourceFileOf(n).FileName, Range: n.Loc, Node: n}
}

func (l *lowerer) unsupported(n *ast.Node, feature string) {
	l.diagnostics = append(l.diagnostics, ast.Diagnostic{File: ast.SourceFileOf(n), Loc: n.Loc, Code: DiagnosticUnsupported, Message: "Semantic IR does not yet support " + feature})
}

func (l *lowerer) scalar(n *ast.Node, t *checker.Type, allowVoid bool) ir.Type {
	if t != nil {
		switch t.Kind {
		case checker.TypeVoid:
			if allowVoid {
				return ir.Type{Scalar: ir.Void}
			}
		case checker.TypeBoolean:
			return ir.Type{Scalar: ir.Boolean}
		case checker.TypeByte:
			return ir.Type{Scalar: ir.Byte}
		case checker.TypeInteger:
			return ir.Type{Scalar: ir.Integer}
		case checker.TypeLong:
			return ir.Type{Scalar: ir.Long}
		case checker.TypeSingle:
			return ir.Type{Scalar: ir.Single}
		case checker.TypeDouble:
			return ir.Type{Scalar: ir.Double}
		case checker.TypeDate:
			return ir.Type{Scalar: ir.Date}
		case checker.TypeCurrency:
			return ir.Type{Scalar: ir.Currency}
		case checker.TypeString:
			return ir.Type{Scalar: ir.String}
		case checker.TypeVariant:
			return ir.Type{Scalar: ir.Variant}
		case checker.TypeObject:
			return ir.Type{Scalar: ir.Object}
		case checker.TypeClass:
			if virtualClass(t.Symbol) != 0 || l.classes[t.Symbol] != 0 {
				return ir.Type{Scalar: ir.Object}
			}
		case checker.TypeStructure:
			if id := l.records[t.Symbol]; id != 0 {
				return ir.Type{Scalar: ir.Record, Record: id}
			}
		case checker.TypeArray:
			if n.Kind == ast.KindVariableDeclaration && n.AsVariableDeclaration().Type != nil && n.AsVariableDeclaration().Type.AsTypeReference().NewKeyword != nil {
				l.unsupported(n, "As New array elements")
				return ir.Type{}
			}
			element := l.scalar(n, t.ElementType, false)
			if ir.ScalarValue(element) || element.Scalar == ir.Record || element.Scalar == ir.Object {
				array := ir.ArrayOf(element)
				if element.Scalar == ir.Object {
					array.Class, array.Builtin = l.classes[t.ElementType.Symbol], virtualClass(t.ElementType.Symbol)
				}
				return array
			}
		}
	}
	l.unsupported(n, fmt.Sprintf("type %s", t))
	return ir.Type{}
}

type procedureBuilder struct {
	*lowerer
	procedure           *ir.Procedure
	slots               map[*ast.Symbol]ir.SlotID
	statement           *ir.Statement
	nextValue           ir.ValueID
	types               map[ir.ValueID]ir.Type
	fixedArrays         []*ast.Node
	referenceTokens     map[ir.ValueID][]ir.ValueID
	referenceObjects    map[ir.ValueID][]ir.ValueID // Instance owners of borrowed field storage.
	autoReferences      map[ir.ValueID]autoReference
	interfaceReferences map[ir.ValueID]interfaceReference
	edges               []controlEdge
	nextSet             map[*ir.Statement]bool
	labels              map[*ast.Node]*ir.Statement
	exits               map[*ast.Node]*ir.Statement
	scope               ir.ScopeID
	objectCaptures      map[*ast.Node]ir.SlotID
	captures            map[*ast.Node]ir.SlotID
	divisionDestination ir.ValueID // Double Let destination for the specified error-side store.
}

func (b *procedureBuilder) addSlot(n *ast.Node, symbol *ast.Symbol, name string, t ir.Type, kind ir.SlotKind, byRef bool) ir.SlotID {
	id := ir.SlotID(len(b.procedure.Slots) + 1)
	b.procedure.Slots = append(b.procedure.Slots, ir.Slot{ID: id, Name: name, Type: t, Kind: kind, ByRef: byRef, Source: source(n), AutoNew: b.autoNew(n), AutoClass: b.autoClass(n)})
	if symbol != nil {
		b.slots[symbol] = id
	}
	return id
}

func (b *procedureBuilder) declare() {
	n := b.procedure.Source.Node
	decl := n.AsFunctionDeclaration()
	if fixedString(decl.Type) {
		b.unsupported(n, "fixed-length Strings")
	}
	if decl.Modifiers()&ast.ModifierStatic != 0 {
		b.unsupported(n, "Static procedures")
	}
	sig := b.checker.GetSignatureFromDeclaration(n)
	if sig == nil {
		b.unsupported(n, "unresolved signatures")
		return
	}
	b.procedure.ReturnType = b.scalar(n, sig.ReturnType, true)
	if b.procedure.Class != 0 {
		id := b.addSlot(n, nil, "$me", ir.Type{Scalar: ir.Object}, ir.Parameter, false)
		b.procedure.Receiver = id
		b.procedure.Parameters = append(b.procedure.Parameters, id)
	}
	for _, param := range sig.Parameters {
		if fixedString(param.Declaration.AsParameterDeclaration().Type) {
			b.unsupported(param.Declaration, "fixed-length Strings")
		}
		paramArray := param.Modifiers&ast.ParameterParamArray != 0
		b.validateDefault(param)
		t := b.scalar(param.Declaration, param.Type, false)
		if ir.Array(t) && !param.IsByRef() && !paramArray {
			b.unsupported(param.Declaration, "ByVal declared array parameters")
		}
		id := b.addSlot(param.Declaration, param.Symbol, param.Name, t, ir.Parameter, param.IsByRef() || paramArray)
		b.procedure.Slots[id-1].ParamArray = paramArray
		b.procedure.Parameters = append(b.procedure.Parameters, id)
	}
	if decl.ResultSymbol != nil {
		b.procedure.ResultSlot = b.addSlot(n, decl.ResultSymbol, decl.ResultSymbol.Name, b.procedure.ReturnType, ir.FunctionResult, false)
	}
	b.declareInvocation(sig)
	var declarations []*ast.Node
	var collect func(*ast.Node)
	collect = func(node *ast.Node) {
		if node.Kind == ast.KindVariableStatement {
			declarations = append(declarations, node)
			return
		}
		node.ForEachChild(func(child *ast.Node) bool { collect(child); return false })
	}
	collect(decl.Body)
	for _, statement := range declarations {
		v := statement.AsVariableStatement()
		if v.Keyword == ast.KindCONST {
			b.constantDeclarations(statement)
			continue
		}
		if v.Keyword != ast.KindDIM || v.Modifiers&ast.ModifierStatic != 0 {
			b.unsupported(statement, "Const and Static locals")
			continue
		}
		for _, variable := range nodes(v.DeclarationList.AsVariableDeclarationList().Declarations) {
			d := variable.AsVariableDeclaration()
			if fixedString(d.Type) {
				b.unsupported(variable, "fixed-length Strings")
				continue
			}
			if d.Initializer != nil {
				b.unsupported(variable, "local initializers")
				continue
			}
			t := b.scalar(variable, b.checker.GetTypeOfSymbol(variable.Symbol), false)
			b.addSlot(variable, variable.Symbol, d.Name.AsIdentifier().Text, t, ir.Local, false)
			if d.Dimensions != nil && len(d.Dimensions.Nodes) > 0 {
				b.fixedArrays = append(b.fixedArrays, variable)
			}
		}
	}
	// Implicit locals have identifier declarations. Preserve source order, like
	// explicit declarations, without depending on symbol-table map iteration.
	var implicit func(*ast.Node)
	implicit = func(node *ast.Node) {
		s := node.Symbol
		if s != nil && s.Parent == n.Symbol && s.ValueDeclaration != nil && (s.ValueDeclaration.Kind == ast.KindIdentifier || s.ValueDeclaration.Kind == ast.KindReDimDeclaration) && b.slots[s] == 0 {
			b.addSlot(s.ValueDeclaration, s, s.Name, b.scalar(node, b.checker.GetTypeOfSymbol(s), false), ir.Local, false)
		}
		node.ForEachChild(func(child *ast.Node) bool { implicit(child); return false })
	}
	implicit(decl.Body)
}

func (b *procedureBuilder) body() {
	n := b.procedure.Source.Node
	b.nextSet = make(map[*ir.Statement]bool)
	b.labels = make(map[*ast.Node]*ir.Statement)
	b.exits = make(map[*ast.Node]*ir.Statement)
	var labels func(*ast.Node)
	labels = func(node *ast.Node) {
		if node.Kind == ast.KindLabeledStatement {
			b.labels[node] = b.block(node)
		}
		node.ForEachChild(func(child *ast.Node) bool { labels(child); return false })
	}
	labels(n.AsFunctionDeclaration().Body)
	if len(b.fixedArrays) > 0 {
		b.newStatement(n)
		for _, variable := range b.fixedArrays {
			d := variable.AsVariableDeclaration()
			t := b.scalar(variable, b.checker.GetTypeOfSymbol(variable.Symbol), false)
			t.Form = ir.Reference
			ref := b.emit(ir.Instruction{Op: ir.OpAddress, Type: t, Slot: b.slots[variable.Symbol], Source: source(variable)})
			bounds := b.checker.GetArrayBounds(variable)
			if len(bounds) != len(d.Dimensions.Nodes) {
				b.unsupported(variable, "unresolved fixed array bounds")
				continue
			}
			args := []ir.ValueID{ref, b.literal(variable, ir.Type{Scalar: ir.Long}, arrayTag(ir.ElementType(t))), b.literal(variable, ir.Type{Scalar: ir.Long}, 2)}
			for _, bound := range bounds {
				args = append(args, b.literal(variable, ir.Type{Scalar: ir.Long}, int64(bound.Lower)), b.literal(variable, ir.Type{Scalar: ir.Long}, int64(bound.Upper)))
			}
			b.operation(variable, ir.OpArrayResize, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime, args...)
		}
	}
	b.sequence(n.AsFunctionDeclaration().Body)
	b.newStatement(n) // Normal completion reads the current function result slot.
	b.statement.End = ir.Return
	b.resolveEdges()
}

func (b *procedureBuilder) lowerStatement(statement *ast.Node) {
	if statement == nil || statement.Kind == ast.KindAttributeStatement {
		return
	}
	if statement.Kind == ast.KindBlock {
		b.sequence(statement)
		return
	}
	if statement.Kind == ast.KindLabeledStatement {
		b.activate(b.labels[statement])
		b.lowerStatement(statement.AsLabeledStatement().Statement)
		return
	}
	b.newStatement(statement)
	switch statement.Kind {
	case ast.KindIfStatement:
		b.ifStatement(statement)
	case ast.KindSelectStatement:
		b.selectStatement(statement)
	case ast.KindOnGotoStatement, ast.KindOnGosubStatement:
		b.onGotoStatement(statement)
	case ast.KindWhileStatement:
		b.whileStatement(statement)
	case ast.KindDoStatement:
		b.doStatement(statement)
	case ast.KindForStatement:
		b.forStatement(statement)
	case ast.KindForEachStatement:
		b.forEachStatement(statement)
	case ast.KindOnErrorStatement:
		n := statement.AsOnErrorStatement()
		b.statement.ErrorMode = n.Mode
		if n.Target != nil {
			b.link(b.statement, b.labels[n.Target.AsGotoStatement().Target.ValueDeclaration], -1)
		}
	case ast.KindResumeStatement:
		n := statement.AsResumeStatement()
		b.statement.End = ir.Resume
		b.statement.ResumeMode = n.Mode
		if n.Target != nil {
			b.link(b.statement, b.labels[n.Target.AsGotoStatement().Target.ValueDeclaration], 1)
		}
	case ast.KindErrorStatement:
		value := b.coerce(statement, b.expression(statement.AsErrorStatement().Expression), ir.Type{Scalar: ir.Variant})
		// Error uses fresh default properties; Err.Raise can inherit explicit
		// properties. Evaluate the number before clearing the previous Err.
		b.operation(statement, ir.OpErrClear, ir.Type{Scalar: ir.Void}, ir.WritesStorage|ir.ManagesLifetime|ir.WritesAmbientErr)
		args := []ir.ValueID{value}
		for len(args) < 5 {
			args = append(args, b.emit(ir.Instruction{Op: ir.OpVariantConstant, Type: ir.Type{Scalar: ir.Variant}, Integer: 2, Source: source(statement)}))
		}
		b.operation(statement, ir.OpErrRaise, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime|ir.WritesAmbientErr, args...)
	case ast.KindWithStatement:
		if t := b.checker.GetTypeAtLocation(statement.AsWithStatement().Expression); t.Kind == checker.TypeObject || t.Kind == checker.TypeClass || t.Kind == checker.TypeVariant {
			b.objectWith(statement)
		} else {
			b.recordWith(statement)
		}
	case ast.KindGosubStatement:
		b.gosubStatement(statement)
	case ast.KindGosubReturnStatement:
		b.statement.ReturnAddress = b.operation(statement, ir.OpGosubPop, ir.Type{Scalar: ir.GosubAddress}, ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime)
		b.statement.End = ir.GosubReturn
		next := b.block(statement)
		b.link(b.statement, next, 0)
		b.activate(next)
	case ast.KindGotoStatement:
		target := statement.AsGotoStatement().Target
		if target == nil || b.labels[target.ValueDeclaration] == nil {
			b.unsupported(statement, "unresolved GoTo target")
		} else {
			b.jump(b.labels[target.ValueDeclaration])
		}
	case ast.KindVariableStatement: // Storage was allocated and zero-initialized on entry.
	case ast.KindLetStatement, ast.KindSetStatement:
		a := statement.AsLetStatement()
		if symbol := b.checker.GetSymbolAtLocation(a.LExpression); symbol != nil {
			if field, ok := errorField(symbol.Library); ok {
				value := b.coerce(statement, b.expression(a.RExpression), ir.Type{Scalar: ir.Variant})
				b.operation(statement, ir.OpErrSet, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.ManagesLifetime|ir.WritesStorage|ir.WritesAmbientErr, b.literal(statement, ir.Type{Scalar: ir.Long}, field), value)
				break
			}
		}
		if r := b.checker.GetCallResolution(a.LExpression); r != nil && (r.Kind == checker.CallPropertyLet || r.Kind == checker.CallPropertySet) {
			b.call(a.LExpression)
			break
		}
		if r := b.checker.GetCallResolution(a.LExpression); r != nil && r.Kind == checker.CallDynamic {
			b.dynamicInvoke(a.LExpression, r, 0)
			break
		}
		storage := b.checker.GetStorageAtLocation(a.LExpression)
		if storage.Kind == checker.StorageArrayElement || storage.Kind == checker.StorageDynamic || b.isRecordField(storage) || b.isClassField(storage) {
			var value ir.ValueID
			if statement.Kind == ast.KindSetStatement {
				value = b.objectExpression(a.RExpression)
				value = b.conversion(statement, value, b.checker.GetAssignmentConversion(statement))
			}
			ref := b.reference(a.LExpression, storage)
			if statement.Kind != ast.KindSetStatement {
				value = b.conversion(statement, b.assignmentValue(a.RExpression, ref), b.checker.GetAssignmentConversion(statement))
			}
			b.writeReference(statement, ref, value)
			break
		}
		slot := b.storage(a.LExpression, storage)
		ref := ir.ValueID(0)
		if storage.Type.Kind == checker.TypeDouble {
			ref = b.emit(ir.Instruction{Op: ir.OpAddress, Type: ir.Type{Scalar: ir.Double, Form: ir.Reference}, Slot: slot, Source: source(a.LExpression)})
		}
		value := ir.ValueID(0)
		if statement.Kind == ast.KindSetStatement {
			value = b.objectExpression(a.RExpression)
		} else {
			value = b.assignmentValue(a.RExpression, ref)
		}
		value = b.conversion(statement, value, b.checker.GetAssignmentConversion(statement))
		b.store(statement, slot, value)
	case ast.KindCallStatement:
		b.call(statement.AsCallStatement().Expression)
	case ast.KindMidAssignmentStatement:
		b.midAssignment(statement)
	case ast.KindReDimStatement:
		b.reDim(statement)
	case ast.KindEraseStatement:
		for _, target := range statement.AsEraseStatement().Expressions.Nodes {
			ref := b.reference(target, b.checker.GetStorageAtLocation(target))
			b.operation(statement, ir.OpArrayErase, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime, ref)
		}
	case ast.KindExitStatement:
		if statement.AsExitStatement().Target == b.procedure.Source.Node {
			b.statement.End = ir.Return
		} else if exit := b.exits[statement.AsExitStatement().Target]; exit != nil {
			b.jump(exit)
		} else {
			b.unsupported(statement, "unresolved Exit target")
		}
	default:
		b.unsupported(statement, "statement kind "+fmt.Sprint(statement.Kind))
	}
}

func (b *procedureBuilder) newStatement(n *ast.Node) {
	b.activate(b.block(n))
}

func (b *procedureBuilder) emit(in ir.Instruction) ir.ValueID {
	if ir.Managed(in.Type) {
		in.Ownership = ir.Owned
		if in.Op == ir.OpStringLiteral || in.Op == ir.OpNullString || in.Op == ir.OpVariantConstant {
			in.Ownership = ir.Immortal
		}
		if in.Op == ir.OpLoad || in.Op == ir.OpTry {
			in.Effects |= ir.ManagesLifetime
		}
	}
	if in.Op == ir.OpStore && len(in.Args) == 1 && b.types[in.Args[0]].Scalar == ir.String {
		in.Effects |= ir.ManagesLifetime
	}
	if in.Op != ir.OpStore {
		b.nextValue++
		in.ID = b.nextValue
		b.types[in.ID] = in.Type
	}
	if in.Ownership == ir.Owned {
		b.statement.Cleanup = append(b.statement.Cleanup, in.ID)
	}
	b.statement.Instructions = append(b.statement.Instructions, in)
	return in.ID
}

func (b *procedureBuilder) operation(n *ast.Node, op ir.Op, t ir.Type, effects ir.Effects, args ...ir.ValueID) ir.ValueID {
	if effects&ir.MayRaise != 0 {
		t.Form = ir.Result
	}
	id := b.emit(ir.Instruction{Op: op, Type: t, Args: args, Effects: effects, Source: source(n)})
	if t.Form == ir.Result {
		t.Form = ir.Value
		id = b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)})
	}
	return id
}

func (b *procedureBuilder) storage(n *ast.Node, s *checker.Storage) ir.SlotID {
	if s != nil && s.Resolution != nil && s.Resolution.Kind == checker.CallArrayReference {
		return b.storage(n, b.checker.GetStorageAtLocation(s.Resolution.Target))
	}
	if s == nil || s.Kind != checker.StorageVariable || !s.Writable || !s.Addressable || s.Receiver != nil && b.checker.GetTypeAtLocation(s.Receiver).Kind != checker.TypeModule || s.WithTarget != nil || len(s.Indices) != 0 || s.Symbol == nil {
		b.unsupported(n, "nonlocal or unresolved storage")
		return 0
	}
	if id := b.slots[s.Symbol]; id != 0 {
		return id
	}
	if global := b.globals[s.Symbol]; global != 0 {
		g := b.program.Globals[global-1]
		id := b.addSlot(n, s.Symbol, g.Name, g.Type, ir.GlobalAlias, false)
		b.procedure.Slots[id-1].Global = global
		b.procedure.Slots[id-1].AutoNew = g.AutoNew
		b.procedure.Slots[id-1].AutoClass = g.AutoClass
		return id
	}
	b.unsupported(n, "undeclared storage")
	return 0
}

func (b *procedureBuilder) expression(n *ast.Node) ir.ValueID {
	value := b.rawExpression(n)
	for _, r := range b.checker.GetValueResolutions(n) {
		value = b.resolvedCall(n, r, value)
	}
	return value
}
func (b *procedureBuilder) rawExpression(n *ast.Node) ir.ValueID {
	if b.checker.GetCallResolution(n) != nil {
		return b.call(n)
	}
	t := b.scalar(n, b.checker.GetTypeAtLocation(n), false)
	switch n.Kind {
	case ast.KindMeExpression:
		return b.me(n)
	case ast.KindNOTHING:
		return b.emit(ir.Instruction{Op: ir.OpObjectNull, Type: t, Source: source(n)})
	case ast.KindNewExpression:
		if class := b.classes[b.checker.GetTypeAtLocation(n).Symbol]; class != 0 {
			return b.classOperation(n, ir.OpClassNew, class, t, ir.MayRaise|ir.ManagesLifetime)
		}
		kind := virtualClass(b.checker.GetTypeAtLocation(n).Symbol)
		if kind == 0 {
			b.unsupported(n, "external object construction")
			return 0
		}
		id := b.operation(n, ir.OpObjectNew, t, ir.MayRaise|ir.ManagesLifetime, b.literal(n, ir.Type{Scalar: ir.Long}, int64(kind)))
		return id
	case ast.KindEMPTY, ast.KindNULL:
		value := int64(0)
		if n.Kind == ast.KindNULL {
			value = 1
		}
		return b.emit(ir.Instruction{Op: ir.OpVariantConstant, Type: t, Integer: value, Source: source(n)})
	case ast.KindStringLiteral:
		return b.stringLiteral(n, utf16.Encode([]rune(n.AsStringLiteral().Text)), false)
	case ast.KindTRUE, ast.KindFALSE:
		value := int64(0)
		if n.Kind == ast.KindTRUE {
			value = -1
		}
		return b.literal(n, t, value)
	case ast.KindNumericLiteral:
		if t.Scalar == ir.Currency {
			return b.currencyLiteral(n, n.AsNumericLiteral().Text)
		}
		if ir.Floating(t) {
			text := strings.ReplaceAll(strings.ToLower(strings.TrimRight(n.AsNumericLiteral().Text, "!#")), "d", "e")
			bits := 64
			if t.Scalar == ir.Single {
				bits = 32
			}
			value, err := strconv.ParseFloat(text, bits)
			if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
				b.unsupported(n, "floating literal")
				return 0
			}
			return b.emit(ir.Instruction{Op: ir.OpConstant, Type: t, Float: value, Source: source(n)})
		}
		value, ok := b.checker.GetNumericLiteralValue(n)
		if !ok {
			b.unsupported(n, "integral literal")
			return 0
		}
		return b.literal(n, t, int64(value))
	case ast.KindIdentifier, ast.KindPropertyAccessExpression:
		if symbol := b.checker.GetSymbolAtLocation(n); symbol != nil && symbol.Flags&ast.SymbolFlagsClass != 0 && b.globals[symbol] != 0 {
			g := b.program.Globals[b.globals[symbol]-1]
			slot := b.slots[symbol]
			if slot == 0 {
				slot = b.addSlot(n, symbol, g.Name, g.Type, ir.GlobalAlias, false)
				b.procedure.Slots[slot-1].Global = g.ID
				b.procedure.Slots[slot-1].AutoClass = g.AutoClass
			}
			return b.load(n, slot)
		}
		if symbol := b.checker.GetSymbolAtLocation(n); symbol != nil {
			if symbol.Library == stdlib.Err {
				return b.errorGet(n, 0, t)
			}
			if field, ok := errorField(symbol.Library); ok {
				return b.errorGet(n, field, t)
			}
		}
		if units, null, ok := b.constantString(n, 0); ok {
			return b.stringLiteral(n, units, null)
		}
		if symbol := b.checker.GetSymbolAtLocation(n); symbol != nil && symbol.Library == stdlib.BinaryCompare {
			return b.literal(n, t, 0)
		}
		if value, ok := b.checker.GetNumericConstantValue(b.checker.GetSymbolAtLocation(n)); ok {
			return b.numericLiteral(n, t, value)
		}
		if value, ok := b.checker.GetEnumMemberValue(b.checker.GetSymbolAtLocation(n)); ok {
			return b.literal(n, t, value)
		}
		if s := b.checker.GetStorageAtLocation(n); b.isRecordField(s) || b.isClassField(s) {
			return b.readReference(n, b.reference(n, s))
		}
		id := b.storage(n, b.checker.GetStorageAtLocation(n))
		if ir.Managed(t) {
			return b.managedStorage(n, ir.OpLoadManaged, t, id)
		}
		return b.emit(ir.Instruction{Op: ir.OpLoad, Type: t, Slot: id, Source: source(n)})
	case ast.KindParenthesizedExpression:
		return b.rawExpression(n.AsParenthesizedExpression().Inner)
	case ast.KindPrefixUnaryExpression:
		u := n.AsPrefixUnaryExpression()
		if t.Scalar == ir.Currency && u.Operator == ast.KindMINUS && u.Operand.Kind == ast.KindNumericLiteral && b.checker.GetTypeAtLocation(u.Operand).Kind == checker.TypeCurrency {
			return b.currencyLiteral(n, "-"+u.Operand.AsNumericLiteral().Text)
		}
		value := b.coerce(n, b.expression(u.Operand), t)
		switch u.Operator {
		case ast.KindPLUS:
			return value
		case ast.KindMINUS:
			if t.Scalar == ir.Date || t.Scalar == ir.Currency {
				v := b.coerce(n, value, ir.Type{Scalar: ir.Variant})
				return b.coerce(n, b.operation(n, ir.OpNegate, ir.Type{Scalar: ir.Variant}, ir.MayRaise|ir.ManagesLifetime, v), t)
			}
			if t.Scalar == ir.Variant {
				return b.operation(n, ir.OpNegate, t, ir.MayRaise|ir.ManagesLifetime, value)
			}
			if t.Scalar == ir.Integer || t.Scalar == ir.Long || ir.Floating(t) {
				return b.operation(n, ir.OpNegate, t, ir.MayRaise, value)
			}
		case ast.KindNOT:
			if t.Scalar == ir.Variant {
				b.unsupported(n, "Variant logical operators")
				return 0
			}
			return b.operation(n, ir.OpNot, t, 0, value)
		}
	case ast.KindBinaryExpression:
		return b.binary(n, t)
	}
	b.unsupported(n, "expression kind "+fmt.Sprint(n.Kind))
	return 0
}

func (b *procedureBuilder) literal(n *ast.Node, t ir.Type, value int64) ir.ValueID {
	return b.emit(ir.Instruction{Op: ir.OpConstant, Type: t, Integer: value, Source: source(n)})
}

func (b *procedureBuilder) conversion(n *ast.Node, value ir.ValueID, c *checker.Conversion) ir.ValueID {
	if c != nil && c.Kind == checker.ConversionDeferred && c.From.Kind == checker.TypeObject && c.To.Kind != checker.TypeObject && c.To.Kind != checker.TypeClass {
		value = b.coerce(n, value, ir.Type{Scalar: ir.Variant})
		value = b.operation(n, ir.OpObjectValue, ir.Type{Scalar: ir.Variant}, ir.MayRaise|ir.ManagesLifetime|ir.WritesStorage, value)
		return b.coerce(n, value, b.scalar(n, c.To, false))
	}
	if c == nil || c.Kind != checker.ConversionIdentity && c.Kind != checker.ConversionLet && c.Kind != checker.ConversionSet {
		b.unsupported(n, "unresolved or non-scalar conversion")
		return 0
	}
	from, to := b.scalar(n, c.From, false), b.scalar(n, c.To, false)
	if b.types[value] != from && !(c.Kind == checker.ConversionSet && from.Scalar == ir.Variant && b.types[value].Scalar == ir.Object && to.Scalar == ir.Object) {
		b.unsupported(n, "conversion with unresolved source semantics")
		return 0
	}
	if c.Kind == checker.ConversionSet {
		value = b.coerce(n, value, ir.Type{Scalar: ir.Object})
	}
	if class := b.classes[c.To.Symbol]; class != 0 && b.types[value].Scalar == ir.Object {
		b.classOperation(n, ir.OpClassCast, class, ir.Type{Scalar: ir.Void}, ir.MayRaise, value)
	}
	if c.To.Kind == checker.TypeClass && virtualClass(c.To.Symbol) != 0 && b.types[value].Scalar == ir.Object {
		b.operation(n, ir.OpObjectCheck, ir.Type{Scalar: ir.Void}, ir.MayRaise, value, b.literal(n, ir.Type{Scalar: ir.Long}, int64(virtualClass(c.To.Symbol))))
	}
	return b.coerce(n, value, to)
}

func (b *procedureBuilder) coerce(n *ast.Node, value ir.ValueID, to ir.Type) ir.ValueID {
	from := b.types[value]
	if from == to {
		return value
	}
	if to.Scalar == ir.Variant && from.Scalar != ir.ArrayRecord && from.Form == ir.Value && ((ir.ScalarValue(from) && from.Scalar != ir.Variant) || ir.Array(from) || from.Scalar == ir.Object) {
		return b.operation(n, ir.OpBox, to, ir.MayRaise|ir.ManagesLifetime, value)
	}
	if from.Scalar == ir.Variant && to.Scalar != ir.ArrayRecord && to.Form == ir.Value && ((ir.ScalarValue(to) && to.Scalar != ir.Variant) || ir.Array(to) || to.Scalar == ir.Object) {
		return b.operation(n, ir.OpUnbox, to, ir.MayRaise|ir.ManagesLifetime, value)
	}
	if from.Scalar == ir.Date || from.Scalar == ir.Currency || to.Scalar == ir.Date || to.Scalar == ir.Currency {
		return b.coerce(n, b.coerce(n, value, ir.Type{Scalar: ir.Variant}), to)
	}
	if from.Scalar == ir.Byte && (to.Scalar == ir.Integer || to.Scalar == ir.Long || ir.Floating(to)) {
		return b.operation(n, ir.OpConvert, to, 0, value)
	}
	// Byte coercions use the shared unsigned, checked runtime conversion.
	// String/Byte() coercions also pass through the same Variant contract.
	byteScalar := from.Scalar == ir.Byte && to.Scalar >= ir.Boolean && to.Scalar <= ir.String || to.Scalar == ir.Byte && from.Scalar >= ir.Boolean && from.Scalar <= ir.String
	stringBytes := from.Scalar == ir.String && to.Scalar == ir.ArrayByte || from.Scalar == ir.ArrayByte && to.Scalar == ir.String
	if byteScalar || stringBytes {
		return b.coerce(n, b.coerce(n, value, ir.Type{Scalar: ir.Variant}), to)
	}
	if (ir.Floating(from) || from.Scalar == ir.Integer || from.Scalar == ir.Long) && to.Scalar == ir.Boolean {
		effects := ir.Effects(0)
		if ir.Floating(from) {
			effects = ir.MayRaise
		}
		return b.operation(n, ir.OpCondition, to, effects, value)
	}
	if (from.Scalar == ir.Boolean || from.Scalar == ir.Integer || from.Scalar == ir.Long || ir.Floating(from)) && to.Scalar == ir.String || from.Scalar == ir.String && ir.Floating(to) {
		return b.coerce(n, b.coerce(n, value, ir.Type{Scalar: ir.Variant}), to)
	}
	if ir.Floating(from) || ir.Floating(to) {
		if from.Scalar < ir.Boolean || from.Scalar > ir.Double || to.Scalar < ir.Integer || to.Scalar > ir.Double {
			b.unsupported(n, "floating conversion")
			return 0
		}
		effects := ir.Effects(0)
		if ir.Floating(from) && (to.Scalar == ir.Integer || to.Scalar == ir.Long || to.Scalar == ir.Single) {
			effects = ir.MayRaise
		}
		return b.operation(n, ir.OpConvert, to, effects, value)
	}
	if (from.Scalar != ir.Boolean && from.Scalar != ir.Integer && from.Scalar != ir.Long) || (to.Scalar != ir.Integer && to.Scalar != ir.Long) {
		b.unsupported(n, "conversion from "+from.String()+" to "+to.String())
		return 0
	}
	var effects ir.Effects
	if to.Scalar == ir.Integer && from.Scalar != ir.Boolean {
		effects = ir.MayRaise
	}
	return b.operation(n, ir.OpConvert, to, effects, value)
}

func (b *procedureBuilder) binary(n *ast.Node, t ir.Type) ir.ValueID {
	e := n.AsBinaryExpression()
	if e.OperatorToken.Kind == ast.KindIS {
		left, right := b.coerce(n, b.objectExpression(e.Left), ir.Type{Scalar: ir.Object}), b.coerce(n, b.objectExpression(e.Right), ir.Type{Scalar: ir.Object})
		if b.types[left].Scalar == ir.Object && b.types[right].Scalar == ir.Object {
			return b.operation(n, ir.OpObjectEqual, ir.Type{Scalar: ir.Boolean}, 0, left, right)
		}
		b.unsupported(n, "object identity operands")
		return 0
	}
	left, right := b.expression(e.Left), b.expression(e.Right)
	comparison := map[ast.Kind]ir.Op{ast.KindEQ: ir.OpEqual, ast.KindNEQ: ir.OpNotEqual, ast.KindLT: ir.OpLess, ast.KindLEQ: ir.OpLessEqual, ast.KindGT: ir.OpGreater, ast.KindGEQ: ir.OpGreaterEqual}[e.OperatorToken.Kind]
	if comparison != ir.OpInvalid {
		return b.compare(n, comparison, left, right)
	}
	if t.Scalar == ir.Variant || b.types[left].Scalar == ir.Date || b.types[left].Scalar == ir.Currency || b.types[right].Scalar == ir.Date || b.types[right].Scalar == ir.Currency {
		op := map[ast.Kind]ir.Op{ast.KindPLUS: ir.OpAdd, ast.KindMINUS: ir.OpSubtract, ast.KindMULT: ir.OpMultiply, ast.KindDIV: ir.OpDivide, ast.KindAMPERSAND: ir.OpConcat, ast.KindINTDIV: ir.OpIntegerDivide, ast.KindMOD: ir.OpModulo}[e.OperatorToken.Kind]
		if op == ir.OpInvalid {
			b.unsupported(n, "Variant operator")
			return 0
		}
		variant := ir.Type{Scalar: ir.Variant}
		left, right = b.coerce(n, left, variant), b.coerce(n, right, variant)
		return b.coerce(n, b.operation(n, op, variant, ir.MayRaise|ir.ManagesLifetime, left, right), t)
	}
	if t.Scalar == ir.String && e.OperatorToken.Kind == ast.KindAMPERSAND {
		left, right = b.coerce(n, left, t), b.coerce(n, right, t)
	}
	if t.Scalar == ir.String && (e.OperatorToken.Kind == ast.KindAMPERSAND || e.OperatorToken.Kind == ast.KindPLUS) && b.types[left].Scalar == ir.String && b.types[right].Scalar == ir.String {
		return b.operation(n, ir.OpConcat, t, ir.MayRaise|ir.ManagesLifetime, left, right)
	}
	op := map[ast.Kind]ir.Op{
		ast.KindPLUS: ir.OpAdd, ast.KindMINUS: ir.OpSubtract, ast.KindMULT: ir.OpMultiply,
		ast.KindDIV: ir.OpDivide, ast.KindINTDIV: ir.OpIntegerDivide, ast.KindMOD: ir.OpModulo,
		ast.KindAND: ir.OpAnd, ast.KindOR: ir.OpOr, ast.KindXOR: ir.OpXor,
	}[e.OperatorToken.Kind]
	if op == ir.OpInvalid {
		b.unsupported(n, "binary operator "+fmt.Sprint(e.OperatorToken.Kind))
		return 0
	}
	if op == ir.OpIntegerDivide || op == ir.OpModulo {
		// These operators round each operand before dividing. Preserve the
		// effective Integer width when an Integer/Boolean is on the left.
		effective := t
		if (b.types[left].Scalar == ir.Integer || b.types[left].Scalar == ir.Boolean) && (ir.Floating(b.types[right]) || b.types[right].Scalar == ir.String) {
			effective = ir.Type{Scalar: ir.Integer}
		}
		convert := func(v ir.ValueID) ir.ValueID {
			if b.types[v].Scalar == ir.Boolean || b.types[v].Scalar == ir.String {
				v = b.coerce(n, v, ir.Type{Scalar: ir.Variant})
			}
			return b.coerce(n, v, effective)
		}
		left, right = convert(left), convert(right)
		return b.coerce(n, b.operation(n, op, effective, ir.MayRaise, left, right), t)
	}
	if (b.types[left].Scalar == ir.String || b.types[right].Scalar == ir.String) && !ir.Floating(t) {
		b.unsupported(n, "implicit numeric/String conversions")
		return 0
	}
	left, right = b.coerce(n, left, t), b.coerce(n, right, t)
	var effects ir.Effects
	if op == ir.OpAdd || op == ir.OpSubtract || op == ir.OpMultiply || op == ir.OpDivide {
		effects = ir.MayRaise
	}
	if op == ir.OpDivide && b.divisionDestination != 0 && b.checker.GetTypeAtLocation(e.Left).Kind == checker.TypeDouble && b.checker.GetTypeAtLocation(e.Right).Kind == checker.TypeDouble {
		return b.operation(n, op, t, effects|ir.WritesStorage, left, right, b.divisionDestination)
	}
	return b.operation(n, op, t, effects, left, right)
}

func (b *procedureBuilder) assignmentValue(n *ast.Node, ref ir.ValueID) ir.ValueID {
	previous := b.divisionDestination
	b.divisionDestination = 0
	if b.types[ref] == (ir.Type{Scalar: ir.Double, Form: ir.Reference}) {
		b.divisionDestination = ref
	}
	value := b.expression(n)
	b.divisionDestination = previous
	return value
}

func (b *procedureBuilder) call(n *ast.Node) ir.ValueID {
	r := b.checker.GetCallResolution(n)
	return b.resolvedCall(n, r, 0)
}
func (b *procedureBuilder) resolvedCall(n *ast.Node, r *checker.CallResolution, receiver ir.ValueID) ir.ValueID {
	if r != nil && r.Kind == checker.CallDynamic {
		return b.dynamicInvoke(n, r, receiver)
	}
	setValue := ir.ValueID(0)
	if r != nil && r.Kind == checker.CallPropertySet && len(r.Arguments) > 0 {
		arg := r.Arguments[len(r.Arguments)-1]
		if arg.Expression != nil && arg.SourceIndex < 0 {
			setValue = b.conversion(arg.Expression, b.objectExpression(arg.Expression), arg.Conversion)
			if arg.Mode == checker.ArgumentTemporary {
				setValue = b.argumentTemporary(arg.Expression, setValue, "$setter.value")
			}
		}
	}
	if r != nil && r.Prefix != nil && receiver == 0 {
		receiver = b.resolvedCall(n, r.Prefix, 0)
	}
	if id, ok := b.collectionCall(n, r, ir.Type{}); ok {
		return id
	}
	if id, ok := b.testLibraryCall(n, r); ok {
		return id
	}
	if r != nil && (r.Kind == checker.CallArrayIndex || r.Kind == checker.CallDynamic && b.checker.GetTypeAtLocation(r.Target).Kind == checker.TypeVariant) {
		return b.readReference(n, b.reference(n, b.checker.GetStorageAtLocation(n)))
	}
	if r != nil && r.Kind == checker.CallArrayReference {
		return b.readReference(n, b.reference(r.Target, b.checker.GetStorageAtLocation(r.Target)))
	}
	if r == nil || !r.Valid || (r.Kind != checker.CallProcedure && r.Kind != checker.CallPropertyGet && r.Kind != checker.CallPropertyLet && r.Kind != checker.CallPropertySet) || r.Signature == nil {
		b.unsupported(n, "unresolved, dynamic, or accessor calls")
		return 0
	}
	if symbol := r.Signature.Declaration.Symbol; symbol != nil {
		if symbol.Library != stdlib.None {
			return b.catalogCall(n, r)
		}
		switch symbol.Intrinsic {
		case ast.IntrinsicNone:
			// Ordinary user procedures are resolved below.
		default:
			b.unsupported(n, "runtime intrinsic")
			return 0
		}
	}
	callee := b.procedures[r.Signature.Declaration]
	if callee == nil {
		b.unsupported(n, "external calls")
		return 0
	}
	args := make([]ir.ValueID, len(callee.Parameters))
	offset := 0
	if callee.Class != 0 {
		offset = 1
		if receiver != 0 {
			args[0] = receiver
		} else if r.Receiver != nil || r.WithTarget != nil {
			args[0] = b.objectReceiver(n, r.Receiver, r.WithTarget)
		} else {
			args[0] = b.me(n)
		}
	}
	var packed, argumentLocks []ir.ValueID
	type objectCopyBack struct {
		destination, temporary ir.ValueID
		conversion             *checker.Conversion
		node                   *ast.Node
	}
	var copyBacks []objectCopyBack
	for _, arg := range r.Arguments { // Checker order is source order for supported explicit arguments.
		setterValue := (r.Kind == checker.CallPropertyLet || r.Kind == checker.CallPropertySet) && arg.ParameterIndex == len(r.Signature.Parameters)-1
		if arg.ParameterIndex < 0 || arg.ParameterIndex+offset >= len(args) || arg.SourceIndex < 0 && !setterValue && arg.Mode != checker.ArgumentMissing && arg.Mode != checker.ArgumentDefault || arg.StringValue != nil {
			b.unsupported(n, "default, missing, or adapted arguments")
			return 0
		}
		if setValue != 0 && setterValue {
			args[arg.ParameterIndex+offset] = setValue
			continue
		}
		if arg.ParamArrayIndex >= 0 {
			if arg.ParamArrayIndex != len(packed) {
				b.unsupported(n, "unordered ParamArray arguments")
				return 0
			}
			value := b.paramArrayArgument(n, arg)
			packed = append(packed, value)
			argumentLocks = append(argumentLocks, b.referenceTokens[value]...)
			continue
		}
		var value ir.ValueID
		switch arg.Mode {
		case checker.ArgumentCopyBack:
			destination := b.reference(arg.Expression, arg.Storage)
			input := b.conversion(arg.Expression, b.readReference(arg.Expression, destination), arg.Conversion)
			value = b.argumentTemporary(arg.Expression, input, "$object.argument")
			copyBacks = append(copyBacks, objectCopyBack{destination, value, arg.CopyBack, arg.Expression})
		case checker.ArgumentDefault:
			value = b.defaultArgument(n, arg.Parameter)
			if arg.Parameter.IsByRef() {
				value = b.argumentTemporary(n, value, "$default")
			}
		case checker.ArgumentMissing:
			t := ir.Type{Scalar: ir.Variant}
			value = b.emit(ir.Instruction{Op: ir.OpVariantConstant, Type: t, Integer: 2, Source: source(n)})
			if arg.Parameter.IsByRef() {
				slot := b.addSlot(n, nil, "$missing", t, ir.Temporary, false)
				b.store(n, slot, value)
				t.Form = ir.Reference
				value = b.emit(ir.Instruction{Op: ir.OpAddress, Type: t, Slot: slot, Source: source(n)})
			}
		case checker.ArgumentReference:
			value = b.reference(arg.Expression, arg.Storage)
			if auto := b.autoReferences[value]; auto.class != 0 || auto.builtin != 0 {
				b.readReference(arg.Expression, value)
			}
			if arg.PreserveStorageType {
				value = b.variantReference(arg.Expression, value)
			}
		case checker.ArgumentValue, checker.ArgumentTemporary:
			// The shared checker retains late-bound application plans. Within the
			// native subset a Variant application resolves to checked array storage.
			if arg.Mode == checker.ArgumentTemporary && !arg.ForcedValue && arg.Parameter.Type.Kind == checker.TypeVariant && arg.Storage != nil && arg.Storage.Kind == checker.StorageDynamic && arg.Storage.Resolution != nil && b.checker.GetTypeAtLocation(arg.Storage.Resolution.Target).Kind == checker.TypeVariant {
				value = b.reference(arg.Expression, arg.Storage)
				break
			}
			if arg.Conversion != nil && arg.Conversion.Kind == checker.ConversionSet && (arg.Conversion.To.Kind == checker.TypeObject || arg.Conversion.To.Kind == checker.TypeClass) {
				value = b.objectExpression(arg.Expression)
			} else {
				value = b.expression(arg.Expression)
			}
			value = b.conversion(arg.Expression, value, arg.Conversion)
			if arg.Mode == checker.ArgumentTemporary {
				t := b.types[value]
				slot := b.addSlot(arg.Expression, nil, "$argument", t, ir.Temporary, false)
				b.store(arg.Expression, slot, value)
				t.Form = ir.Reference
				value = b.emit(ir.Instruction{Op: ir.OpAddress, Type: t, Slot: slot, Source: source(arg.Expression)})
			}
		default:
			b.unsupported(n, "argument binding mode "+fmt.Sprint(arg.Mode))
			return 0
		}
		args[arg.ParameterIndex+offset] = value
		if r.ParamArray != nil {
			argumentLocks = append(argumentLocks, b.referenceTokens[value]...)
		}
	}
	if r.ParamArray != nil {
		index := len(callee.Parameters) - 1
		if index < 0 || !callee.Slots[callee.Parameters[index]-1].ParamArray {
			b.unsupported(n, "unresolved ParamArray formal")
			return 0
		}
		args[index] = b.packParamArray(n, packed)
	}
	op := ir.OpCall
	check := ir.OpClassCheck
	if callee.Invocation != nil && b.interfaceClasses[callee.Class] {
		op = ir.OpInterfaceCall
		check = ir.OpInterfaceCheck
	}
	if callee.Class != 0 {
		b.classOperation(n, check, callee.Class, ir.Type{Scalar: ir.Void}, ir.MayRaise, args[0])
	}
	t := callee.ReturnType
	t.Form = ir.Result
	id := b.emit(ir.Instruction{Op: op, Type: t, Callee: callee.ID, Args: args, Effects: ir.MayRaise, Source: source(n)})
	t.Form = ir.Value
	result := b.emit(ir.Instruction{Op: ir.OpTry, Type: t, Args: []ir.ValueID{id}, OnError: ir.Propagate, Source: source(n)})
	for _, copyBack := range copyBacks {
		value := b.conversion(copyBack.node, b.readReference(copyBack.node, copyBack.temporary), copyBack.conversion)
		b.writeReference(copyBack.node, copyBack.destination, value)
	}
	if r.ParamArray != nil {
		// A nested call's bindings end here, before later outer arguments execute.
		// Drop borrowed wrappers before releasing their array-element locks. Error
		// continuations already release these resources at the statement boundary.
		b.operation(n, ir.OpArrayErase, ir.Type{Scalar: ir.Void}, ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime, args[len(args)-1])
		released := map[ir.ValueID]bool{}
		for i := len(argumentLocks) - 1; i >= 0; i-- {
			token := argumentLocks[i]
			if !released[token] {
				b.operation(n, ir.OpReleaseElement, ir.Type{Scalar: ir.Void}, ir.ManagesLifetime, token)
				released[token] = true
			}
		}
	}
	return result
}

// Calls conservatively MayRaise until a later effect-specialization pass proves
// otherwise. Propagate storage and ambient effects to a fixed point, including
// mutual recursion, without changing language return types.
func inferEffects(program *ir.Program) {
	for changed := true; changed; {
		changed = false
		for _, p := range program.Procedures {
			effects := ir.EntryEffects(p) | ir.GlobalInitializationEffects(program.Globals)
			if effects&ir.ManagesLifetime != 0 {
				effects |= ir.TerminationEffects(program)
			}
			for _, s := range p.Statements {
				if ir.UsesErrors(program) {
					effects |= ir.ErrorPolicyEffects(s)
				}
				for i := range s.Instructions {
					in := &s.Instructions[i]
					if in.Op == ir.OpCall {
						in.Effects = ir.MayRaise | program.Procedures[in.Callee-1].Effects
					}
					if in.Op == ir.OpInterfaceCall {
						in.Effects = ir.InterfaceCallEffects(program, in.Callee)
					}
					if in.Op == ir.OpObjectInvoke || in.Op == ir.OpObjectValue {
						in.Effects |= ir.InvocationEffects(program)
					}
					if in.Op == ir.OpClassNew || in.Op == ir.OpObjectAutoLoad {
						in.Effects |= ir.InitializationEffects(program, in.Class)
					}
					if in.Op == ir.OpLoadManaged {
						in.Effects |= ir.InitializationEffects(program, p.Slots[in.Slot-1].AutoClass)
					}
					in.Effects |= ir.DefaultInvocationEffects(program, *in)
					if in.Effects&ir.ManagesLifetime != 0 {
						in.Effects |= ir.TerminationEffects(program)
					}
					effects |= in.Effects
				}
			}
			if ir.UsesErrors(program) && effects&ir.MayRaise != 0 {
				effects |= ir.WritesAmbientErr
			}
			if effects != p.Effects {
				p.Effects = effects
				changed = true
			}
		}
	}
}

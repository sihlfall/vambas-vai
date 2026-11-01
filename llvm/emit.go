// Package llvm emits textual LLVM IR from verified VBA semantic IR. It owns
// private code-generation details; the shared AST and checker remain unchanged.
package llvm

import (
	_ "embed"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/ir/identity"
)

//go:embed strings.ll
var stringRuntime string

//go:embed mid.ll
var midRuntime string

// Options configures emission. Native scalar locals become SSA values and phi
// nodes built by this backend; addressable and managed storage stay in memory.
type Options struct {
	// Entry optionally adds a native main for a parameterless procedure. A
	// Function's scalar result is the exit status; a successful Sub returns zero.
	// An unhandled VBA error returns its number. Zero emits a private module only.
	Entry ir.ProcedureID
}

// Emit verifies its input and returns either a complete LLVM module or diagnostics
// and an empty string. It performs no I/O and needs no LLVM Go bindings. LLVM 20
// with opaque pointers is the initial supported toolchain.
func Emit(program *ir.Program, options Options) (string, []ir.Diagnostic) {
	diagnostics := ir.Verify(program)
	if len(diagnostics) != 0 {
		return "", diagnostics
	}
	for _, p := range program.Procedures {
		if p.Effects & ^(ir.MayRaise|ir.WritesStorage|ir.ManagesLifetime|ir.ReadsAmbientErr|ir.WritesAmbientErr) != 0 {
			diagnostics = append(diagnostics, ir.Diagnostic{Source: p.Source, Message: "LLVM backend does not support external effects"})
		}
		for _, statement := range p.Statements {
			for _, in := range statement.Instructions {
				if in.Op == ir.OpLibraryCall {
					if _, ok := libraryABIs[in.Library]; !ok {
						signature, _ := ir.LookupLibraryCall(in.Library)
						diagnostics = append(diagnostics, ir.Diagnostic{Source: in.Source, Message: "LLVM backend does not support " + signature.Name})
					}
				}
			}
		}
	}
	if options.Entry != 0 {
		if int(options.Entry) > len(program.Procedures) {
			diagnostics = append(diagnostics, ir.Diagnostic{Message: "LLVM entry procedure does not exist"})
		} else if p := program.Procedures[options.Entry-1]; len(p.Parameters) != 0 {
			diagnostics = append(diagnostics, ir.Diagnostic{Source: p.Source, Message: "LLVM entry procedure must have no parameters"})
		} else if ir.Managed(p.ReturnType) || ir.Floating(p.ReturnType) || p.ReturnType.Scalar == ir.Date || p.ReturnType.Scalar == ir.Currency {
			diagnostics = append(diagnostics, ir.Diagnostic{Source: p.Source, Message: "native entry requires Sub or Boolean/Integer/Long result; use a scalar wrapper"})
		}
	}
	if len(diagnostics) != 0 {
		return "", diagnostics
	}
	m := &module{program: program, snapshotRanks: make(map[int]bool), snapshotLocks: make(map[int]bool), snapshotIndexes: make(map[[2]int]bool), snapshotBounds: make(map[int]bool), intrinsics: make(map[string]string), globals: program.Globals, name: program.Name, objectCallbacks: len(program.Classes) > 0}
	if len(program.Records) > 0 || len(program.Classes) > 0 {
		m.variants = true
	}
	for _, g := range program.Globals {
		if g.Type.Scalar == ir.String {
			m.strings = true
		}
		if g.Type.Scalar == ir.Object || g.Type.Scalar == ir.Variant || ir.Array(g.Type) || g.Type.Scalar == ir.Record {
			m.variants = true
		}
	}
	for _, p := range program.Procedures {
		if ir.UsesGosub(p) {
			m.variants = true
		}
		for _, s := range p.Slots {
			if s.Type.Scalar == ir.Object || s.Type.Scalar == ir.Variant || ir.Array(s.Type) {
				m.variants = true
			}
		}
		for _, s := range p.Statements {
			for _, in := range s.Instructions {
				if in.Op == ir.OpLibraryCall || in.Op == ir.OpTestAssert || in.Type.Scalar == ir.Object || in.Type.Scalar == ir.Variant || ir.Array(in.Type) || ir.Element(in.Type) || in.Op == ir.OpPinString {
					m.variants = true
				}
			}
		}
	}
	for _, p := range program.Procedures {
		for _, s := range p.Statements {
			if s.ErrorMode != 0 || s.End == ir.Resume {
				m.errors = true
			}
			for _, in := range s.Instructions {
				if in.Op == ir.OpErrRaise || in.Op == ir.OpErrClear || in.Op == ir.OpErrGet || in.Op == ir.OpErrSet {
					m.errors = true
				}
			}
		}
	}
	if m.errors {
		m.variants = true
	}
	if m.variants {
		m.strings = true
	}
	var body strings.Builder
	for _, p := range program.Procedures {
		body.WriteString(m.procedure(p))
	}
	body.WriteString(m.invocationMetadata(program))
	body.WriteString(interfaceDispatch(program))
	if options.Entry != 0 {
		body.WriteString(entry(program.Procedures[options.Entry-1], len(program.Globals) > 0 || m.errors))
	}
	var out strings.Builder
	fmt.Fprintf(&out, "; VBA scalar module: %s\n", comment(program.Name))
	out.WriteString("; Private ABI only. Error = {number, procedure ID, statement ID, UTF-8 offset}.\n%vba.error = type { i32, i32, i32, i32 }\n\n")
	if m.strings {
		out.WriteString(stringRuntime)
		out.WriteString(midRuntime)
		if m.variants {
			out.WriteString(variantDeclarations)
		} else {
			out.WriteString("define internal void @vba.str.foreign.drop(ptr %p) { unreachable }\n")
		}
		out.WriteByte('\n')
	}
	out.WriteString(recordMetadata(program.Records))
	out.WriteString(classMetadata(program.Classes, len(program.Records) == 0))
	if m.variants {
		out.WriteString(objectArrayMetadata(program.Classes))
	}
	out.WriteString(projectGlobals(program.Globals, m.errors))
	for _, literal := range m.literals {
		out.WriteString(literal)
		out.WriteByte('\n')
	}
	names := make([]string, 0, len(m.intrinsics))
	for name := range m.intrinsics {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		out.WriteString(m.intrinsics[name])
		out.WriteByte('\n')
	}
	if len(names) != 0 {
		out.WriteByte('\n')
	}
	out.WriteString(m.arrayAccessHelpers())
	out.WriteString(body.String())
	return out.String(), nil
}

func comment(text string) string {
	return strings.NewReplacer("\n", " ", "\r", " ", "\x00", " ").Replace(text)
}

func scalar(t ir.Type) string {
	if t.Scalar == ir.Object || t.Scalar == ir.Receiver || t.Scalar == ir.GosubAddress {
		return "ptr"
	}
	if t.Scalar == ir.Record {
		return fmt.Sprintf("%%vba.record%d", t.Record)
	}
	if t.Scalar == ir.Enumerator {
		return "%vba.enumerator"
	}
	if ir.Array(t) {
		return "ptr"
	}
	if ir.Element(t) {
		return "%vba.element"
	}
	switch t.Scalar {
	case ir.Byte:
		return "i8"
	case ir.Boolean, ir.Integer:
		return "i16"
	case ir.Long:
		return "i32"
	case ir.Currency:
		return "i64"
	case ir.Single:
		return "float"
	case ir.Double, ir.Date:
		return "double"
	case ir.Void:
		return "void"
	case ir.String:
		return "%vba.string"
	case ir.Variant:
		return "%vba.variant"
	default:
		panic("LLVM backend received an unverified type")
	}
}

func valueName(id ir.ValueID) string         { return fmt.Sprintf("%%v%d", id) }
func procedureName(id ir.ProcedureID) string { return fmt.Sprintf("@vba.p%d", id) }

type module struct {
	program            *ir.Program
	identity           *identity.Analysis // Computed on first use; see identityAnalysis.
	snapshotRanks      map[int]bool       // Snapshot types and reload helpers, by stored rank.
	snapshotLocks      map[int]bool
	snapshotIndexes    map[[2]int]bool // Index helpers by subscript count and stored rank.
	snapshotBounds     map[int]bool
	scalarElementClear bool
	cstringPrefix      string
	name               string
	errors             bool
	globals            []ir.Global
	intrinsics         map[string]string
	strings            bool
	literals           []string
	variants           bool
	objectCallbacks    bool
}

// Blocks are generated independently of binder FlowNode metadata. Terminators
// close every block; try splits the current sequence into success/error blocks.
type block struct {
	label        string
	lines        []string
	end          string
	successors   []string
	predecessors []*block
	definitions  map[ir.SlotID]string
	phis         []*slotPhi
}

type operand struct {
	text string
	typ  ir.Type
}

type result struct {
	condition     string // i1: true means failure.
	status        string // Nonempty for a callee error; its origin is already recorded.
	value         string // Success value, or aggregate containing the checked result.
	aggregate     bool
	narrow        bool
	callSlot      string
	source        ir.Source
	recordError   bool
	runtimeOrigin bool
	preserveErr   bool
	character     string
	direct        bool
	conversion    string // Guarded fptosi; emitted only on the success edge.
	errorStore    string // Optional Double destination, committed only for division by zero.
	errorValue    string
	storeWhen     string
}

type function struct {
	arraySnapshots      []*arraySnapshot
	arrayAccess         map[ir.ValueID]arrayAccessPlan // Inline accesses through a snapshot.
	entryRefresh        map[ir.StatementID][]snapshotRefresh
	afterRefresh        map[snapshotPoint][]snapshotRefresh
	snapshotFields      int
	failureTails        map[ir.StatementID]*failureTail
	unwind              *failureTail
	arrayTokens         map[ir.ValueID]arrayTokenPlan
	indexTokens         map[ir.ValueID]ir.ValueID
	elementPointers     map[ir.ValueID]string
	scalarElementMemory map[string]bool
	ssaSlots            map[ir.SlotID]bool
	ssaPhis             []*slotPhi
	module              *module
	procedure           *ir.Procedure
	blocks              []*block
	current             *block
	slots               map[ir.SlotID]string
	values              map[ir.ValueID]operand
	results             map[ir.ValueID]result
	valueTypes          map[ir.ValueID]ir.Type
	memory              map[ir.ValueID]string
}

func (f *function) newBlock(label string) *block {
	b := &block{label: label}
	f.blocks = append(f.blocks, b)
	return b
}

func (f *function) line(format string, args ...any) {
	f.current.lines = append(f.current.lines, fmt.Sprintf(format, args...))
}

func (m *module) procedure(p *ir.Procedure) string {
	f := &function{module: m, procedure: p, slots: make(map[ir.SlotID]string), results: make(map[ir.ValueID]result), valueTypes: make(map[ir.ValueID]ir.Type), memory: make(map[ir.ValueID]string)}
	f.current = f.newBlock("entry")
	f.planArrayAccess()
	f.ssaSlots = scalarSSASlots(p)
	if ir.UsesGosub(p) {
		f.line("%%gosub.stack = alloca %%vba.gosub.stack")
		f.line("store %%vba.gosub.stack zeroinitializer, ptr %%gosub.stack")
		f.line("%%gosub.target = alloca ptr")
		f.line("%%gosub.return.scope = alloca i32")
	}
	for _, s := range p.Slots {
		if s.Type.Scalar == ir.String {
			m.strings = true
		}
	}
	parameters := []string{"ptr %error"}
	if p.ReturnType.Scalar != ir.Void {
		parameters = append(parameters, "ptr %result")
	}
	for i, id := range p.Parameters {
		slot := p.Slots[id-1]
		name := fmt.Sprintf("%%arg%d", i)
		t := scalar(slot.Type)
		if slot.ByRef {
			t = "ptr"
			f.slots[id] = name
		}
		parameters = append(parameters, t+" "+name)
	}
	for _, s := range p.Slots {
		if s.Kind == ir.GlobalAlias {
			f.slots[s.ID] = fmt.Sprintf("@vba.g%d", s.Global)
			continue
		}
		if s.ByRef {
			continue
		}
		if f.ssaSlots[s.ID] {
			f.writeScalar(s.ID, zero(s.Type))
			continue
		}
		name := fmt.Sprintf("%%slot%d", s.ID)
		f.slots[s.ID] = name
		f.line("%s = alloca %s", name, scalar(s.Type))
		f.line("store %s %s, ptr %s, align 1", scalar(s.Type), zero(s.Type), name)
	}
	// Call result storage is activation-local, allocated once rather than once
	// per execution of a block. Loads are emitted only on the success edge.
	for _, s := range p.Statements {
		for _, in := range s.Instructions {
			if in.Op == ir.OpObjectInvoke {
				f.line("%%invoke.args%d = alloca [%d x %%vba.invoke.arg]", in.ID, len(in.Args)-1)
			}
			f.valueTypes[in.ID] = in.Type
			if in.Type.Scalar == ir.String {
				m.strings = true
			}
			if ir.Managed(in.Type) && !f.arrayTokens[in.ID].unlocked {
				name := fmt.Sprintf("%%spill%d", in.ID)
				if in.Ownership == ir.Owned {
					name = fmt.Sprintf("%%owned%d", in.ID)
				}
				f.memory[in.ID] = name
				if _, ok := f.arrayTokens[in.ID]; ok {
					f.scalarElementMemory[name] = true
				}
				f.line("%s = alloca %s", name, scalar(in.Type))
				f.line("store %s %s, ptr %s, align 1", scalar(in.Type), zero(in.Type), name)
			}
			if in.Op == ir.OpParamArrayPack {
				f.line("%%paramarray.args%d = alloca [%d x %%vba.paramarg]", in.ID, len(in.Args)-1)
			}
			if in.Op == ir.OpTestAssert || in.Op == ir.OpCollectionInvoke {
				f.line("%%test.args%d = alloca [%d x %%vba.test.arg]", in.ID, len(in.Args)-1)
			}
			if in.Op == ir.OpVariantRef {
				f.line("%%reference%d = alloca %%vba.variant", in.ID)
			}
			if in.Op == ir.OpArrayResize || in.Op == ir.OpArrayIndex && !f.inlineAccess(in.ID) {
				n := len(in.Args) - 1
				if in.Op == ir.OpArrayResize {
					n = len(in.Args) - 3
				}
				f.line("%%array.args%d = alloca [%d x i32]", in.ID, n)
			}
		}
	}
	if m.errors {
		f.line("%%handler = alloca i32")
		f.line("store i32 0, ptr %%handler")
		f.line("%%handler.active = alloca i1")
		f.line("store i1 false, ptr %%handler.active")
		f.line("%%resume.retry = alloca i32")
		f.line("%%resume.next = alloca i32")
		sourceValue := m.stringLiteral(utf16.Encode([]rune(m.name)))
		m.literals = append(m.literals, fmt.Sprintf("@vba.err.source%d = private constant %%vba.string %s", p.ID, sourceValue))
	}
	if ir.GlobalInitializationEffects(m.globals) != 0 {
		f.line("%%project.status = call i32 @vba.project.ensure(ptr %%error)")
		f.line("%%project.failed = icmp ne i32 %%project.status, 0")
		f.current.branch("%project.failed", "project.failure", "project.success")
		f.current = f.newBlock("project.failure")
		f.current.end = "ret i32 %project.status" // No activation owners have been acquired.
		f.current = f.newBlock("project.success")
	}
	for i, id := range p.Parameters {
		s := p.Slots[id-1]
		if s.ByRef {
			continue
		}
		if ir.Array(s.Type) {
			status := fmt.Sprintf("%%acquire%d", id)
			f.line("%s = call i32 @vba_array_copy(ptr %s, ptr %%arg%d)", status, f.slots[id], i)
			f.activationCheck(fmt.Sprintf("parameter%d", id), status, s.Source)
		} else if s.Type.Scalar == ir.Variant {
			f.line("%%incoming%d = alloca %%vba.variant", id)
			f.line("store %%vba.variant %%arg%d, ptr %%incoming%d, align 1", i, id)
			status := fmt.Sprintf("%%acquire%d", id)
			f.line("%s = call i32 @vba_variant_load(ptr %s, ptr %%incoming%d)", status, f.slots[id], id)
			f.activationCheck(fmt.Sprintf("parameter%d", id), status, s.Source)
		} else {
			f.storeSlot(id, fmt.Sprintf("%%arg%d", i))
			if s.Type.Scalar == ir.Object {
				f.line("call void @vba_object_retain(ptr %%arg%d)", i)
			}
			if s.Type.Scalar == ir.String {
				f.line("call void @vba.str.retain(%%vba.string %%arg%d)", i)
			}
		}
	}
	for _, s := range p.Slots {
		if s.Type.Scalar == ir.Record && !s.ByRef && s.Kind != ir.Temporary && s.Kind != ir.GlobalAlias && s.Kind != ir.LoopState {
			status := fmt.Sprintf("%%init.record%d", s.ID)
			f.line("%s = call i32 @vba_record_init(ptr %s, ptr @vba.record.type%d)", status, f.slots[s.ID], s.Type.Record)
			f.activationCheck(fmt.Sprintf("record%d", s.ID), status, s.Source)
		}
	}
	f.refreshAllSnapshots()
	f.current.jump("s1")
	for _, s := range p.Statements {
		f.current = f.newBlock(fmt.Sprintf("s%d", s.ID))
		f.values = make(map[ir.ValueID]operand)
		f.line("; S%d retry=S%d next=S%d %s:%d..%d", s.ID, s.Retry, s.Next, comment(s.Source.FileName), s.Source.Range.Pos(), s.Source.Range.End())
		if m.errors && s.ErrorMode != 0 {
			f.errorPolicy(s)
		}
		for _, r := range f.entryRefresh[s.ID] {
			f.refreshSnapshot(r)
		}
		for i, in := range s.Instructions {
			f.instruction(s, in)
			for _, r := range f.afterRefresh[snapshotPoint{s.ID, i}] {
				f.refreshSnapshot(r)
			}
		}
		f.cleanupValues(s.Cleanup)
		f.cleanupTemporarySlots(s)
		switch s.End {
		case ir.Continue, ir.Jump:
			target := s.Next
			if s.End == ir.Jump {
				target = s.Target
			}
			current := f.current
			destination := f.edge(s, target, "jump")
			current.jump(destination)
		case ir.Branch:
			f.line("%%branch%d = icmp ne i16 %s, 0", s.ID, f.values[s.Condition].text)
			current := f.current
			yes := f.edge(s, s.Target, "true")
			no := f.edge(s, s.FalseTarget, "false")
			current.branch(fmt.Sprintf("%%branch%d", s.ID), yes, no)
		case ir.Switch:
			current := f.current
			value := f.values[s.Selector]
			fallback := f.edge(s, s.Target, "default")
			var cases []switchCase
			for i, item := range s.Cases {
				cases = append(cases, switchCase{strconv.FormatInt(item.Value, 10), f.edge(s, item.Target, fmt.Sprintf("case%d", i))})
			}
			current.switchOn(scalar(value.typ), value.text, fallback, cases)
		case ir.Gosub:
			f.current.jump(fmt.Sprintf("s%d", s.Target))
		case ir.GosubReturn:
			f.line("store ptr %s, ptr %%gosub.target", f.values[s.ReturnAddress].text)
			f.line("store i32 %d, ptr %%gosub.return.scope", s.Scope)
			f.current.jump("gosub_return")
		case ir.Resume:
			f.resumeStatement(s)
		case ir.Return:
			if m.errors {
				f.errorReturn(s)
			}
			if p.ReturnType.Scalar != ir.Void {
				if p.ReturnType.Scalar == ir.String && m.variants {
					status := fmt.Sprintf("%%return.status%d", s.ID)
					f.line("%s = call i32 @vba_string_load(ptr %%result, ptr %s)", status, f.slots[p.ResultSlot])
					f.activationCheck(fmt.Sprintf("return%d", s.ID), status, s.Source, s)
				} else {
					value := f.loadSlot(p.ResultSlot, fmt.Sprintf("%%return%d", s.ID))
					f.line("store %s %s, ptr %%result, align 1", scalar(p.ReturnType), value)
					if ir.Managed(p.ReturnType) {
						f.line("store %s %s, ptr %s, align 1", scalar(p.ReturnType), zero(p.ReturnType), f.slots[p.ResultSlot])
					}
				}
			}
			f.cleanupSlots()
			f.current.end = "ret i32 0"
		}
	}
	if ir.UsesGosub(p) {
		f.gosubDispatcher()
	}
	// Checked scalar operations produce SSA values directly. Only actual calls
	// using the output-pointer ABI need activation-local result slots.
	var allocations []string
	for _, s := range p.Statements {
		for _, in := range s.Instructions {
			if slot := f.results[in.ID].callSlot; slot != "" {
				allocations = append(allocations, fmt.Sprintf("%s = alloca %s", slot, scalar(in.Type)), fmt.Sprintf("store %s %s, ptr %s, align 1", scalar(in.Type), zero(in.Type), slot))
			}
		}
	}
	f.blocks[0].lines = append(allocations, f.blocks[0].lines...)
	f.finishFailureTails()
	aliases := f.finishSSA()
	var out strings.Builder
	fmt.Fprintf(&out, "; %s\ndefine internal i32 %s(%s) {\n", comment(p.Name), procedureName(p.ID), strings.Join(parameters, ", "))
	for _, b := range f.blocks {
		fmt.Fprintf(&out, "%s:\n", b.label)
		f.formatPhis(&out, b, aliases)
		for _, line := range b.lines {
			fmt.Fprintf(&out, "  %s\n", resolveSSAText(line, aliases))
		}
		fmt.Fprintf(&out, "  %s\n", resolveSSAText(b.end, aliases))
	}
	out.WriteString("}\n\n")
	return out.String()
}

func (f *function) instruction(s *ir.Statement, in ir.Instruction) {
	args := make([]operand, len(in.Args))
	for i, id := range in.Args {
		args[i] = f.values[id]
	}
	name, t := valueName(in.ID), scalar(in.Type)
	value := operand{text: name, typ: in.Type}
	runtimeOrigin := in.Op == ir.OpClassNew || in.Op == ir.OpObjectAutoLoad || in.Op == ir.OpObjectInvoke || in.Op == ir.OpObjectValue || in.Op == ir.OpUnbox || in.Op == ir.OpLoadManaged && f.procedure.Slots[in.Slot-1].AutoClass != 0
	runtimeOrigin = runtimeOrigin || f.module.objectCallbacks && ir.MayInvokeDefault(in)
	if runtimeOrigin {
		f.recordError(fmt.Sprintf("origin%d", in.ID), "0", f.procedure.ID, s.ID, in.Source.Range.Pos())
		f.line("%%previous.origin%d = call ptr @vba_invocation_error_push(ptr %%error)", in.ID)
	}
	switch in.Op {
	case ir.OpInvocationCapture:
		pointer := f.memory[in.Args[0]]
		if args[0].typ.Form == ir.Reference {
			pointer = args[0].text
		}
		f.line("%s = call i32 @vba_invocation_capture(ptr %%call%d,ptr %s)", name, in.ID, pointer)
		f.pending(in)
	case ir.OpObjectValue:
		f.line("%s = call i32 @vba_object_value_at(ptr %%call%d,ptr %s,ptr %%error)", name, in.ID, f.memory[in.Args[0]])
		f.pending(in)
	case ir.OpObjectInvoke:
		value = f.objectInvokeInstruction(in, args)
	case ir.OpClassNew, ir.OpClassCheck, ir.OpClassCast, ir.OpClassFieldRef, ir.OpObjectAutoLoad, ir.OpInterfaceCheck:
		value = f.classInstruction(in, args)
	case ir.OpGosubPush, ir.OpGosubPop:
		f.gosubInstruction(s, in)
	case ir.OpCollectionInvoke:
		value = f.collectionInstruction(in, args)
	case ir.OpObjectNew, ir.OpObjectNull, ir.OpObjectEqual, ir.OpCreateObject, ir.OpTestAssert, ir.OpObjectCheck:
		value = f.testInstruction(s, in, args)
	case ir.OpErrRaise, ir.OpErrClear, ir.OpErrGet, ir.OpErrSet, ir.OpWithCheck:
		f.errorInstruction(in, args)
	case ir.OpKeepElement:
		f.clear(f.slots[in.Slot], args[0].typ)
		f.line("store %%vba.element %s, ptr %s, align 1", args[0].text, f.slots[in.Slot])
		f.line("store %%vba.element zeroinitializer, ptr %s, align 1", f.memory[in.Args[0]])
		value.text = "0"
	case ir.OpWithCapture:
		f.line("store ptr %s, ptr %s, align 1", args[0].text, f.slots[in.Slot])
		value.text = "0"
	case ir.OpWithReference:
		f.line("%s = load ptr, ptr %s, align 1", name, f.slots[in.Slot])
	case ir.OpConstant:
		value.text = strconv.FormatInt(in.Integer, 10)
		if ir.Floating(in.Type) {
			value.text = floatingLiteral(in.Float)
		}
	case ir.OpVariantConstant:
		value.text = fmt.Sprintf("{ i16 %d, i16 0, i16 0, i16 0, i64 0, i64 0 }", in.Integer)
		if in.Integer == 2 {
			value.text = "{ i16 10, i16 0, i16 0, i16 0, i64 2147614724, i64 0 }" // DISP_E_PARAMNOTFOUND
		}
	case ir.OpLoadManaged, ir.OpStoreManaged, ir.OpBox, ir.OpUnbox, ir.OpVariantRef, ir.OpPinString, ir.OpVarType, ir.OpIsEmpty, ir.OpIsNull, ir.OpIsMissing, ir.OpIsArray:
		value = f.variantInstruction(in, args)
	case ir.OpParamArrayPack, ir.OpArrayResize, ir.OpArrayIndex, ir.OpElementRef, ir.OpReadReference, ir.OpWriteReference, ir.OpArrayBound, ir.OpArrayErase, ir.OpReleaseElement:
		value = f.arrayInstruction(in, args)
	case ir.OpCondition, ir.OpCheckInitialized, ir.OpForValidate, ir.OpForTest, ir.OpEnumStart, ir.OpEnumNext, ir.OpEnumObjectNext, ir.OpCheckDispatch:
		value = f.controlInstruction(in, args)
	case ir.OpStringLiteral:
		value.text = f.module.stringLiteral(in.Units)
	case ir.OpNullString:
		value.text = nullString
	case ir.OpAddress:
		value.text = f.slots[in.Slot]
	case ir.OpFieldRef:
		base := args[0].text
		if args[0].typ.Form == ir.Value {
			base = f.memory[in.Args[0]]
		}
		f.line("%s = getelementptr %s, ptr %s, i32 0, i32 %d", name, scalar(args[0].typ), base, in.Field)
	case ir.OpLoad:
		value.text = f.loadSlot(in.Slot, name)
		if in.Type.Scalar == ir.String {
			f.line("call void @vba.str.retain(%%vba.string %s)", name)
		}
	case ir.OpStore:
		if args[0].typ.Scalar == ir.String {
			f.line("call void @vba.str.assign(ptr %s, %%vba.string %s)", f.slots[in.Slot], args[0].text)
		} else {
			f.storeSlot(in.Slot, args[0].text)
		}
	case ir.OpConcat:
		if in.Type.Scalar == ir.Variant {
			f.variantBinary(in, args, 4)
			break
		}
		f.line("%s = call i32 @vba.str.concat(ptr %%call%d, %%vba.string %s, %%vba.string %s)", name, in.ID, args[0].text, args[1].text)
		condition := fmt.Sprintf("%%failed%d", in.ID)
		f.line("%s = icmp ne i32 %s, 0", condition, name)
		f.results[in.ID] = result{condition: condition, status: name, callSlot: fmt.Sprintf("%%call%d", in.ID), source: in.Source, recordError: true}
	case ir.OpMid:
		f.line("%s = call i32 @vba.str.mid(ptr %%call%d, %%vba.string %s, i32 %s, i32 %s)", name, in.ID, args[0].text, args[1].text, args[2].text)
		condition := fmt.Sprintf("%%failed%d", in.ID)
		f.line("%s = icmp ne i32 %s, 0", condition, name)
		f.results[in.ID] = result{condition: condition, status: name, callSlot: fmt.Sprintf("%%call%d", in.ID), source: in.Source, recordError: true}
	case ir.OpMidCheck, ir.OpMidAssign:
		callee, arguments := "@vba.str.midcheck", fmt.Sprintf("ptr %s, i32 %s, i32 %s", args[0].text, args[1].text, args[2].text)
		if in.Op == ir.OpMidAssign {
			callee = "@vba.str.midassign"
			arguments += ", %vba.string " + args[3].text
		}
		f.line("%s = call i32 %s(%s)", name, callee, arguments)
		condition := fmt.Sprintf("%%failed%d", in.ID)
		f.line("%s = icmp ne i32 %s, 0", condition, name)
		f.results[in.ID] = result{condition: condition, status: name, source: in.Source, recordError: true}
	case ir.OpLen, ir.OpLenB:
		if in.Type.Scalar == ir.Variant {
			bytes := 0
			if in.Op == ir.OpLenB {
				bytes = 1
			}
			f.line("%s = call i32 @vba_variant_len(ptr %%call%d, ptr %s, i32 %d)", name, in.ID, f.memory[in.Args[0]], bytes)
			f.pending(in)
			break
		}
		if in.Op == ir.OpLen {
			f.line("%s = call i32 @vba.str.len(%%vba.string %s)", name, args[0].text)
		} else {
			f.line("%%length%d = call i32 @vba.str.len(%%vba.string %s)", in.ID, args[0].text)
			f.module.intrinsics["llvm.smul.with.overflow.i32"] = "declare { i32, i1 } @llvm.smul.with.overflow.i32(i32, i32)"
			f.line("%s = call { i32, i1 } @llvm.smul.with.overflow.i32(i32 %%length%d, i32 2)", name, in.ID)
			condition := fmt.Sprintf("%%overflow%d", in.ID)
			f.line("%s = extractvalue { i32, i1 } %s, 1", condition, name)
			f.results[in.ID] = result{condition: condition, value: name, aggregate: true, source: in.Source}
		}
	case ir.OpChrW:
		f.line("%%low%d = icmp slt i32 %s, -32768", in.ID, args[0].text)
		f.line("%%high%d = icmp sgt i32 %s, 65535", in.ID, args[0].text)
		condition := fmt.Sprintf("%%invalid%d", in.ID)
		f.line("%s = or i1 %%low%d, %%high%d", condition, in.ID, in.ID)
		f.results[in.ID] = result{condition: condition, status: "5", character: args[0].text, source: in.Source, recordError: true}
	case ir.OpConvert:
		if ir.Floating(in.Type) || ir.Floating(args[0].typ) {
			value = f.floatConvert(in, args)
			break
		}
		if args[0].typ.Scalar == ir.Byte {
			f.line("%s = zext i8 %s to %s", name, args[0].text, t)
		} else if in.Type.Form == ir.Result {
			f.line("%%narrow%d = trunc i32 %s to i16", in.ID, args[0].text)
			f.line("%%wide%d = sext i16 %%narrow%d to i32", in.ID, in.ID)
			condition := fmt.Sprintf("%%overflow%d", in.ID)
			f.line("%s = icmp ne i32 %s, %%wide%d", condition, args[0].text, in.ID)
			f.results[in.ID] = result{condition: condition, value: args[0].text, narrow: true, source: in.Source}
		} else if in.Type.Scalar == ir.Integer {
			value.text = args[0].text // Boolean and Integer have the same signed representation.
		} else {
			f.line("%s = sext i16 %s to i32", name, args[0].text)
		}
	case ir.OpAdd, ir.OpSubtract, ir.OpMultiply, ir.OpNegate, ir.OpDivide:
		if ir.Floating(in.Type) {
			f.floatArithmetic(in, args)
			break
		}
		if in.Type.Scalar == ir.Variant {
			if in.Op == ir.OpNegate {
				f.line("%s = call i32 @vba_variant_unary(ptr %%call%d, ptr %s, i32 1)", name, in.ID, f.memory[in.Args[0]])
				f.pending(in)
			} else {
				op := map[ir.Op]int{ir.OpAdd: 1, ir.OpSubtract: 2, ir.OpMultiply: 3, ir.OpDivide: 5}[in.Op]
				f.variantBinary(in, args, op)
			}
			break
		}
		op := "sadd"
		if in.Op == ir.OpSubtract || in.Op == ir.OpNegate {
			op = "ssub"
		} else if in.Op == ir.OpMultiply {
			op = "smul"
		}
		left, right := "0", args[0].text
		if in.Op != ir.OpNegate {
			left, right = args[0].text, args[1].text
		}
		if in.Type.Scalar == ir.Byte {
			op = "u" + op[1:]
		}
		intrinsic := "llvm." + op + ".with.overflow." + t
		aggregate := "{ " + t + ", i1 }"
		f.module.intrinsics[intrinsic] = fmt.Sprintf("declare %s @%s(%s, %s)", aggregate, intrinsic, t, t)
		f.line("%s = call %s @%s(%s %s, %s %s)", name, aggregate, intrinsic, t, left, t, right)
		condition := fmt.Sprintf("%%overflow%d", in.ID)
		f.line("%s = extractvalue %s %s, 1", condition, aggregate, name)
		f.results[in.ID] = result{condition: condition, value: name, aggregate: true, source: in.Source}
	case ir.OpIntegerDivide, ir.OpModulo:
		if in.Type.Scalar == ir.Variant {
			operation := 6
			if in.Op == ir.OpModulo {
				operation = 7
			}
			f.variantBinary(in, args, operation)
		} else {
			f.integerDivision(in, args)
		}
	case ir.OpLibraryCall:
		f.libraryInstruction(in)
	case ir.OpNot:
		f.line("%s = xor %s %s, -1", name, t, args[0].text)
	case ir.OpAnd, ir.OpOr, ir.OpXor:
		op := map[ir.Op]string{ir.OpAnd: "and", ir.OpOr: "or", ir.OpXor: "xor"}[in.Op]
		f.line("%s = %s %s %s, %s", name, op, t, args[0].text, args[1].text)
	case ir.OpEqual, ir.OpNotEqual, ir.OpLess, ir.OpLessEqual, ir.OpGreater, ir.OpGreaterEqual:
		if ir.Floating(args[0].typ) {
			f.floatCompare(in, args)
			break
		}
		if in.Type.Scalar == ir.Variant {
			op := map[ir.Op]int{ir.OpEqual: 1, ir.OpNotEqual: 2, ir.OpLess: 3, ir.OpLessEqual: 4, ir.OpGreater: 5, ir.OpGreaterEqual: 6}[in.Op]
			f.line("%s = call i32 @vba_variant_compare(ptr %%call%d, ptr %s, ptr %s, i32 %d, i32 %s)", name, in.ID, f.memory[in.Args[0]], f.memory[in.Args[1]], op, args[2].text)
			f.pending(in)
			break
		}
		predicate := map[ir.Op]string{ir.OpEqual: "eq", ir.OpNotEqual: "ne", ir.OpLess: "slt", ir.OpLessEqual: "sle", ir.OpGreater: "sgt", ir.OpGreaterEqual: "sge"}[in.Op]
		if args[0].typ.Scalar == ir.Byte && predicate[0] == 's' {
			predicate = "u" + predicate[1:]
		}
		f.line("%%compare%d = icmp %s %s %s, %s", in.ID, predicate, scalar(args[0].typ), args[0].text, args[1].text)
		f.line("%s = sext i1 %%compare%d to i16", name, in.ID)
	case ir.OpCall, ir.OpInterfaceCall:
		arguments := []string{"ptr %error"}
		var callSlot string
		if in.Type.Scalar != ir.Void {
			callSlot = fmt.Sprintf("%%call%d", in.ID)
			arguments = append(arguments, "ptr "+callSlot)
		}
		for _, arg := range args {
			t := scalar(arg.typ)
			if arg.typ.Form == ir.Reference {
				t = "ptr"
			}
			arguments = append(arguments, t+" "+arg.text)
		}
		callee := procedureName(in.Callee)
		if in.Op == ir.OpInterfaceCall {
			callee = fmt.Sprintf("@vba.interface.p%d", in.Callee)
		}
		f.line("%s = call i32 %s(%s)", name, callee, strings.Join(arguments, ", "))
		condition := fmt.Sprintf("%%failed%d", in.ID)
		f.line("%s = icmp ne i32 %s, 0", condition, name)
		f.results[in.ID] = result{condition: condition, status: name, callSlot: callSlot, source: in.Source}
	case ir.OpTry:
		r := f.results[in.Args[0]]
		success, failure := fmt.Sprintf("ok%d", in.ID), fmt.Sprintf("fail%d", in.ID)
		f.current.branch(r.condition, failure, success)
		f.current = f.newBlock(failure)
		if r.errorStore != "" {
			commit, report := fmt.Sprintf("commit.error%d", in.ID), fmt.Sprintf("report.error%d", in.ID)
			f.current.branch(r.storeWhen, commit, report)
			f.current = f.newBlock(commit)
			f.line("store double %s, ptr %s, align 1", r.errorValue, r.errorStore)
			f.current.jump(report)
			f.current = f.newBlock(report)
		}
		status := r.status
		if status == "" {
			status = "6"
		}
		if r.runtimeOrigin {
			f.line("%%origin.number%d = load i32,ptr %%error", in.ID)
			f.line("%%origin.missing%d = icmp eq i32 %%origin.number%d,0", in.ID, in.ID)
			f.current.branch(fmt.Sprintf("%%origin.missing%d", in.ID), fmt.Sprintf("origin.record%d", in.ID), fmt.Sprintf("origin.ready%d", in.ID))
			f.current = f.newBlock(fmt.Sprintf("origin.record%d", in.ID))
		}
		if r.status == "" || r.recordError {
			f.recordError(fmt.Sprintf("error%d", in.ID), status, f.procedure.ID, s.ID, r.source.Range.Pos())
		}
		if r.runtimeOrigin {
			if f.module.errors {
				f.line("call void @vba_err_capture(i32 %s,ptr @vba.err.source%d)", status, f.procedure.ID)
			}
			f.current.jump(fmt.Sprintf("origin.ready%d", in.ID))
			f.current = f.newBlock(fmt.Sprintf("origin.ready%d", in.ID))
		}
		if f.module.errors {
			if !r.runtimeOrigin && (r.status == "" || r.recordError) {
				capture := "vba_err_capture"
				if r.preserveErr {
					capture = "vba_err_raise_failure"
				}
				f.line("call void @%s(i32 %s, ptr @vba.err.source%d)", capture, status, f.procedure.ID)
			}
		}
		f.fail(s, status)
		f.current = f.newBlock(success)
		switch {
		case r.conversion != "":
			f.line("%s = %s", name, r.conversion)
		case r.direct:
			value.text = r.value
		case r.aggregate:
			f.line("%s = extractvalue { %s, i1 } %s, 0", name, t, r.value)
		case r.narrow:
			f.line("%s = trunc i32 %s to i16", name, r.value)
		case r.callSlot != "":
			f.line("%s = load %s, ptr %s, align 1", name, t, r.callSlot)
			if ir.Managed(in.Type) {
				f.line("store %s %s, ptr %s, align 1", t, zero(in.Type), r.callSlot)
			}
		case r.character != "":
			f.line("%%unit%d = trunc i32 %s to i16", in.ID, r.character)
			f.line("%s = insertvalue %%vba.string { ptr null, ptr null, i32 1, i16 0 }, i16 %%unit%d, 3", name, in.ID)
		default:
			value.text = "" // Result<Void> has no scalar to extract.
		}
	default:
		panic("LLVM backend received an unverified operation")
	}
	if runtimeOrigin {
		f.line("call void @vba_invocation_error_pop(ptr %%previous.origin%d)", in.ID)
		pending := f.results[in.ID]
		pending.runtimeOrigin = true
		f.results[in.ID] = pending
	}
	if in.ID != 0 && in.Type.Form != ir.Result {
		f.values[in.ID] = value
		if ir.Managed(in.Type) && !f.arrayTokens[in.ID].unlocked {
			f.line("store %s %s, ptr %s, align 1", t, value.text, f.memory[in.ID])
		}
	}
}

func entry(p *ir.Procedure, globals bool) string {
	var out strings.Builder
	out.WriteString("define i32 @main() {\nentry:\n  %error = alloca %vba.error\n")
	arguments := "ptr %error"
	if p.ReturnType.Scalar != ir.Void {
		fmt.Fprintf(&out, "  %%result = alloca %s\n", scalar(p.ReturnType))
		arguments += ", ptr %result"
	}
	fmt.Fprintf(&out, "  %%status = call i32 %s(%s)\n", procedureName(p.ID), arguments)
	if globals {
		out.WriteString("  call void @vba.project.clear()\n")
	}
	out.WriteString("  %failed = icmp ne i32 %status, 0\n  br i1 %failed, label %failure, label %success\nfailure:\n  ret i32 %status\nsuccess:\n")
	if p.ReturnType.Scalar == ir.Void {
		out.WriteString("  ret i32 0\n")
	} else {
		t := scalar(p.ReturnType)
		fmt.Fprintf(&out, "  %%value = load %s, ptr %%result, align 1\n", t)
		if t == "i16" {
			out.WriteString("  %exit = sext i16 %value to i32\n  ret i32 %exit\n")
		} else {
			out.WriteString("  ret i32 %value\n")
		}
	}
	out.WriteString("}\n")
	return out.String()
}

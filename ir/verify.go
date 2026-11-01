package ir

import (
	"fmt"
	"math"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/stdlib"
)

type Diagnostic struct {
	Source  Source
	Message string
}

func validType(t Type) bool {
	return t.Scalar >= Void && t.Scalar <= ElementObject && t.Form <= Result && (t.Scalar != GosubAddress || t.Form != Reference) && (t.Scalar != Void || t.Form != Reference) && (!Element(t) || t.Form != Reference) && (t.Scalar != Enumerator || t.Form == Value) && (t.Scalar == Record || t.Scalar == ArrayRecord || t.Scalar == ElementRecord || t.Scalar == Receiver) == (t.Record != 0) && (t.Class == 0 && t.Builtin == 0 || t.Scalar == ArrayObject) && !(t.Class != 0 && t.Builtin != 0) && t.Builtin <= 4
}

func numeric(t Type) bool {
	return integral(t) || t.Form == Value && Floating(t)
}
func integral(t Type) bool {
	return t.Form == Value && (t.Scalar == Byte || t.Scalar == Integer || t.Scalar == Long)
}

func fits(value int64, t Type) bool {
	switch t.Scalar {
	case Boolean:
		return value == 0 || value == -1
	case Byte:
		return value >= 0 && value <= 255
	case Integer:
		return value >= -32768 && value <= 32767
	case Long:
		return value >= -2147483648 && value <= 2147483647
	case Currency:
		return true
	}
	return false
}

// Verify checks the complete private semantic program, including unreachable
// regions. It accepts malformed IR as input and reports diagnostics, not panics.
// There is deliberately no public/COM ABI in this representation.
func Verify(program *Program) []Diagnostic {
	var diagnostics []Diagnostic
	report := func(source Source, format string, args ...any) {
		diagnostics = append(diagnostics, Diagnostic{source, fmt.Sprintf(format, args...)})
	}
	if program == nil {
		report(Source{}, "nil IR program")
		return diagnostics
	}
	checkSource := func(s Source) {
		if s.FileName == "" || !s.Range.IsValid() {
			report(s, "missing source provenance")
		}
		if s.Node != nil && s.Node.Flags&(ast.NodeFlagsThisNodeHasError|ast.NodeFlagsThisNodeOrAnySubNodesHasError) != 0 {
			report(s, "recovered syntax cannot enter semantic IR")
		}
		if s.Node != nil {
			file := ast.SourceFileOf(s.Node)
			if file == nil || s.FileName != file.FileName || s.Range != s.Node.Loc || s.Range.End() > len(file.Text) {
				report(s, "source provenance differs from checked syntax")
			}
		}
	}
	checkType := func(t Type) bool {
		return validType(t) && (t.Record == 0 || int(t.Record) <= len(program.Records)) && int(t.Class) <= len(program.Classes)
	}
	checkObjectConstraint := func(t Type, class, autoClass ClassID, builtin, autoBuiltin uint8) bool {
		return (class == 0 || int(class) <= len(program.Classes)) && (autoClass == 0 || int(autoClass) <= len(program.Classes)) && builtin <= 4 && autoBuiltin <= 4 && !(class != 0 && builtin != 0) && !(autoClass != 0 && autoBuiltin != 0) && (class == 0 && autoClass == 0 && builtin == 0 && autoBuiltin == 0 || t.Scalar == Object) && (autoClass == 0 || class == autoClass) && (autoBuiltin == 0 || autoBuiltin == builtin)
	}
	states := make(map[RecordID]uint8)
	var record func(RecordID, int)
	record = func(id RecordID, depth int) {
		if id == 0 || int(id) > len(program.Records) {
			return
		}
		r := program.Records[id-1]
		if states[id] == 1 || depth > 256 {
			report(r.Source, "recursive or excessively nested UDT layout")
			return
		}
		if states[id] == 2 {
			return
		}
		states[id] = 1
		checkSource(r.Source)
		if r.ID != id || r.Name == "" || len(r.Fields) == 0 {
			report(r.Source, "invalid UDT definition")
		}
		names := make(map[string]bool)
		for _, f := range r.Fields {
			if !checkObjectConstraint(f.Type, f.Class, f.AutoClass, f.Builtin, f.AutoNew) {
				report(f.Source, "invalid UDT object field constraint")
			}
			checkSource(f.Source)
			key := ast.SymbolNameKey(f.Name)
			if f.Type.Scalar == GosubAddress || key == "" || names[key] || !checkType(f.Type) || f.Type.Form != Value || f.Type.Scalar == Void || Element(f.Type) || f.Type.Scalar == Enumerator {
				report(f.Source, "invalid UDT field")
			}
			names[key] = true
			if len(f.Bounds) > 60 || len(f.Bounds) > 0 && !Array(f.Type) {
				report(f.Source, "invalid fixed UDT array bounds")
			}
			for _, bound := range f.Bounds {
				if bound.Lower > bound.Upper {
					report(f.Source, "reversed UDT array bounds")
				}
			}
			if f.Type.Scalar == Record || f.Type.Scalar == ArrayRecord {
				record(f.Type.Record, depth+1)
			}
		}
		states[id] = 2
	}
	for i := range program.Records {
		record(RecordID(i+1), 0)
	}
	memberClasses := make(map[ProcedureID]ClassID)
	for i, c := range program.Classes {
		checkSource(c.Source)
		if c.ID != ClassID(i+1) || c.Name == "" {
			report(c.Source, "invalid class identity")
		}
		names := make(map[string]bool)
		for _, f := range c.Fields {
			if !checkObjectConstraint(f.Type, f.Class, f.AutoClass, f.Builtin, f.AutoNew) {
				report(f.Source, "invalid class object field constraint")
			}
			checkSource(f.Source)
			key := ast.SymbolNameKey(f.Name)
			if key == "" || names[key] || !checkType(f.Type) || f.Type.Form != Value || !(ScalarValue(f.Type) || Array(f.Type) || f.Type.Scalar == Record || f.Type.Scalar == Object) {
				report(f.Source, "invalid class field")
			}
			names[key] = true
			if len(f.Bounds) > 0 && !Array(f.Type) {
				report(f.Source, "class field bounds require array storage")
			}
			if len(f.Bounds) > 60 {
				report(f.Source, "class array field exceeds supported rank")
			}
			for _, b := range f.Bounds {
				if b.Upper < b.Lower {
					report(f.Source, "invalid class field bounds")
				}
			}
		}
		for _, id := range c.Members {
			if id == 0 || int(id) > len(program.Procedures) || memberClasses[id] != 0 {
				report(c.Source, "invalid or duplicate class member")
			}
			memberClasses[id] = c.ID
		}
		for _, id := range []ProcedureID{c.Initialize, c.Terminate} {
			if id != 0 && (int(id) > len(program.Procedures) || program.Procedures[id-1] == nil || program.Procedures[id-1].Class != c.ID || program.Procedures[id-1].ReturnType != (Type{Scalar: Void}) || len(program.Procedures[id-1].Parameters) != 1) {
				report(c.Source, "invalid lifecycle handler")
			}
		}
	}
	interfaceMembers := verifyInterfaces(program, report)
	for i, g := range program.Globals {
		checkSource(g.Source)
		if g.AutoNew > 4 || g.AutoNew != 0 && g.Type.Scalar != Object || g.AutoClass != 0 && (g.Type.Scalar != Object || int(g.AutoClass) > len(program.Classes) || g.AutoNew != 0) || g.ID != GlobalID(i+1) || g.Name == "" || !checkType(g.Type) || g.Type.Form != Value || g.Type.Scalar == Void || g.Type.Scalar == GosubAddress || Element(g.Type) || g.Type.Scalar == Enumerator {
			report(g.Source, "invalid global storage")
		}
		if len(g.Bounds) > 0 && !Array(g.Type) {
			report(g.Source, "global bounds require an array")
		}
		if len(g.Bounds) > 60 {
			report(g.Source, "invalid global array rank")
		}
		for _, b := range g.Bounds {
			if b.Upper < b.Lower {
				report(g.Source, "invalid global array bounds")
			}
		}
	}
	for index, p := range program.Procedures {
		if p == nil {
			report(Source{}, "nil procedure")
			continue
		}
		checkSource(p.Source)
		if p.ID != ProcedureID(index+1) || p.Name == "" {
			report(p.Source, "invalid procedure identity")
		}
		if !checkType(p.ReturnType) || p.ReturnType.Form != Value || p.ReturnType.Scalar == GosubAddress || Element(p.ReturnType) || p.ReturnType.Scalar == Enumerator {
			report(p.Source, "unresolved language return type")
		}
		slot := func(id SlotID) *Slot {
			if id == 0 || int(id) > len(p.Slots) {
				return nil
			}
			return &p.Slots[id-1]
		}
		parameters := make(map[SlotID]bool)
		for _, id := range p.Parameters {
			s := slot(id)
			if s == nil || s.Kind != Parameter || parameters[id] {
				report(p.Source, "invalid or duplicate parameter slot %d", id)
			}
			parameters[id] = true
		}
		if p.Class != memberClasses[p.ID] || (p.Class == 0) != (p.Receiver == 0) || p.Class != 0 && int(p.Class) > len(program.Classes) {
			report(p.Source, "invalid class member association")
		}
		if p.Receiver != 0 {
			r := slot(p.Receiver)
			if r == nil || len(p.Parameters) == 0 || p.Parameters[0] != p.Receiver || r.Type != (Type{Scalar: Object}) || r.ByRef || r.Kind != Parameter {
				report(p.Source, "invalid implicit receiver parameter")
			}
		}
		if signature := p.Invocation; signature != nil {
			if p.Class == 0 || signature.Name == "" || signature.Kind != InvokeMethod && signature.Kind != InvokeGet && signature.Kind != InvokeLet && signature.Kind != InvokeSet || len(signature.Parameters)+1 != len(p.Parameters) || (signature.Kind == InvokeLet || signature.Kind == InvokeSet) && p.ReturnType != (Type{Scalar: Void}) || signature.Kind == InvokeGet && p.ReturnType.Scalar == Void {
				report(p.Source, "invalid public invocation signature")
			}
			for i, a := range signature.Parameters {
				if a.Name == "" || !checkType(a.Type) || !checkObjectConstraint(a.Type, a.Class, 0, a.Builtin, 0) || i+1 >= len(p.Parameters) {
					report(p.Source, "invalid invocation parameter")
					continue
				}
				s := slot(p.Parameters[i+1])
				setterValue := (signature.Kind == InvokeLet || signature.Kind == InvokeSet) && i == len(signature.Parameters)-1
				if s == nil || s.Type != a.Type || s.ParamArray != a.ParamArray || a.ByRef != s.ByRef && !setterValue && !a.ParamArray {
					report(p.Source, "invocation parameter differs from member signature")
				}
				if a.Default != 0 {
					if !a.Optional || int(a.Default) > len(program.Procedures) || program.Procedures[a.Default-1] == nil || program.Procedures[a.Default-1].ReturnType != a.Type || len(program.Procedures[a.Default-1].Parameters) != 0 {
						report(p.Source, "invalid invocation default procedure")
					}
				}
			}
		}
		resultCount := 0
		for i := range p.Slots {
			s := &p.Slots[i]
			checkSource(s.Source)
			if s.Source.FileName != p.Source.FileName || !s.Source.Range.ContainedBy(p.Source.Range) {
				report(s.Source, "slot provenance is outside its procedure")
			}
			if s.ID != SlotID(i+1) || !checkType(s.Type) || s.Type.Form != Value || s.Type.Scalar == Void || s.Type.Scalar == GosubAddress || Element(s.Type) && s.Kind != LoopState || s.Kind > GlobalAlias || s.Type.Scalar == Enumerator && s.Kind != LoopState || s.Type.Scalar == Record && s.Kind == Parameter && !s.ByRef {
				report(s.Source, "invalid scalar slot %d", s.ID)
			}
			if s.ParamArray && (s.Kind != Parameter || !s.ByRef || s.Type != (Type{Scalar: ArrayVariant}) || len(p.Parameters) == 0 || p.Parameters[len(p.Parameters)-1] != s.ID) {
				report(s.Source, "invalid ParamArray binding")
			}
			if s.AutoNew > 4 || s.AutoNew != 0 && s.Type.Scalar != Object || s.AutoClass != 0 && (s.Type.Scalar != Object || int(s.AutoClass) > len(program.Classes) || s.AutoNew != 0) || s.ByRef && s.Kind != Parameter || s.Kind == Parameter && !parameters[s.ID] {
				report(s.Source, "invalid parameter alias or ownership")
			}
			if (s.Kind == GlobalAlias) != (s.Global != 0) {
				report(s.Source, "global alias identity mismatch")
			}
			if s.Kind == GlobalAlias {
				if int(s.Global) > len(program.Globals) || s.Global == 0 || s.ByRef {
					report(s.Source, "invalid global alias")
				} else if program.Globals[s.Global-1].Type != s.Type {
					report(s.Source, "global alias type mismatch")
				}
				if s.Global != 0 && int(s.Global) <= len(program.Globals) && (s.AutoClass != program.Globals[s.Global-1].AutoClass || s.AutoNew != program.Globals[s.Global-1].AutoNew) {
					report(s.Source, "global lazy initialization differs from its alias")
				}
			}
			if s.Kind == FunctionResult {
				resultCount++
				if p.ResultSlot != s.ID || s.Type != p.ReturnType {
					report(s.Source, "incorrect function result storage")
				}
			}
		}
		if p.ReturnType.Scalar == Void {
			if p.ResultSlot != 0 || resultCount != 0 {
				report(p.Source, "Sub cannot have function result storage")
			}
		} else if resultCount != 1 || slot(p.ResultSlot) == nil {
			report(p.Source, "function requires exactly one result slot")
		}
		if len(p.Statements) == 0 {
			report(p.Source, "procedure has no return region")
		}
		ownedScopes := make(map[SlotID]bool)
		for i, scope := range p.Scopes {
			if scope.ID != ScopeID(i+1) || scope.Parent >= scope.ID {
				report(p.Source, "invalid resource scope identity or parent")
			}
			for _, id := range scope.Cleanup {
				s := slot(id)
				if s == nil || s.Type.Scalar != Enumerator && !Element(s.Type) && s.Type.Scalar != Receiver && s.Type.Scalar != Record && s.Type.Scalar != Object || s.Kind != LoopState || ownedScopes[id] {
					report(p.Source, "invalid or duplicate resource scope ownership")
				}
				ownedScopes[id] = true
			}
		}
		for _, s := range p.Slots {
			if (s.Type.Scalar == Enumerator || s.Type.Scalar == Receiver || Element(s.Type)) && !ownedScopes[s.ID] {
				report(s.Source, "enumerator requires a resource scope")
			}
		}
		allValues := make(map[ValueID]bool)
		effects := EntryEffects(p) | GlobalInitializationEffects(program.Globals)
		if effects&ManagesLifetime != 0 {
			effects |= TerminationEffects(program)
		}
		for si, statement := range p.Statements {
			if statement == nil {
				report(p.Source, "nil statement region")
				continue
			}
			s := statement
			if UsesErrors(program) {
				effects |= ErrorPolicyEffects(s)
			}
			validTarget := func(id StatementID) bool { return id != 0 && int(id) <= len(p.Statements) && p.Statements[id-1] != nil }
			if int(s.Scope) > len(p.Scopes) {
				report(s.Source, "invalid resource scope")
			}
			if s.Next != 0 && !validTarget(s.Next) {
				report(s.Source, "invalid next-statement continuation")
			}
			checkSource(s.Source)
			if s.Source.FileName != p.Source.FileName || !s.Source.Range.ContainedBy(p.Source.Range) {
				report(s.Source, "region provenance is outside its procedure")
			}
			if s.ID != StatementID(si+1) || s.Retry != s.ID {
				report(s.Source, "invalid statement identity or retry continuation")
			}
			switch s.End {
			case Continue:
				if !validTarget(s.Next) {
					report(s.Source, "invalid next-statement continuation")
				}
			case Resume:
				if s.ResumeMode < 0 || s.ResumeMode > 2 || s.ResumeMode == 2 && !validTarget(s.Target) {
					report(s.Source, "invalid Resume target")
				}
			case Return:
				if s.Next != 0 {
					report(s.Source, "return region has a next continuation")
				}
			case Gosub:
				if !validTarget(s.Target) || !validTarget(s.Next) {
					report(s.Source, "invalid GoSub target or continuation")
				}
			case GosubReturn:
				if !validTarget(s.Next) {
					report(s.Source, "invalid Return recovery continuation")
				}
			case Jump:
				if !validTarget(s.Target) {
					report(s.Source, "invalid jump target")
				}
			case Branch:
				if !validTarget(s.Target) || !validTarget(s.FalseTarget) {
					report(s.Source, "invalid conditional branch targets")
				}
			case Switch:
				if !validTarget(s.Target) {
					report(s.Source, "invalid switch default target")
				}
				seen := make(map[int64]bool)
				for _, item := range s.Cases {
					if seen[item.Value] || !validTarget(item.Target) {
						report(s.Source, "duplicate switch value or invalid destination")
					}
					seen[item.Value] = true
				}
			default:
				report(s.Source, "unsupported region terminator")
			}
			if s.End != Branch && (s.Condition != 0 || s.FalseTarget != 0) {
				report(s.Source, "unexpected branch payload")
			}
			if s.End != Branch && s.End != Jump && s.End != Switch && s.End != Resume && s.End != Gosub && s.Target != 0 {
				report(s.Source, "unexpected jump target")
			}
			if s.End != GosubReturn && s.ReturnAddress != 0 {
				report(s.Source, "unexpected GoSub return address")
			}
			if s.End != Switch && (s.Selector != 0 || len(s.Cases) != 0) {
				report(s.Source, "unexpected switch payload")
			}
			if s.ErrorMode < 0 || s.ErrorMode > 4 || s.ErrorMode == 2 && !validTarget(s.Handler) || s.ErrorMode != 2 && s.Handler != 0 {
				report(s.Source, "invalid error policy")
			}
			gosubPushes, gosubPops := 0, 0
			values := make(map[ValueID]Type)
			pinnedStrings := make(map[ValueID]bool)
			releasedElements := make(map[ValueID]bool)
			movedElements := make(map[ValueID]bool)
			referenceOwners := make(map[ValueID]ValueID)
			checkedObjects := make(map[ValueID]ClassID)
			checkedInterfaces := make(map[ValueID]ClassID)
			captures := make(map[ValueID]bool)
			var owned []ValueID
			for ii, in := range s.Instructions {
				checkSource(in.Source)
				if in.Source.FileName != s.Source.FileName || !in.Source.Range.ContainedBy(s.Source.Range) {
					report(in.Source, "instruction provenance is outside its region")
				}
				effects |= in.Effects
				bad := func(message string) { report(in.Source, "%s: %s", in.Op, message) }
				if in.Op != OpLoad && in.Op != OpStore && in.Op != OpLoadManaged && in.Op != OpStoreManaged && in.Op != OpAddress && in.Op != OpEnumStart && in.Op != OpEnumNext && in.Op != OpEnumObjectNext && in.Op != OpKeepElement && in.Op != OpWithCapture && in.Op != OpWithReference && in.Op != OpWithCheck && in.Slot != 0 || in.Op != OpCall && in.Op != OpInterfaceCall && in.Callee != 0 || in.Op != OpConstant && in.Op != OpVariantConstant && in.Op != OpTestAssert && in.Op != OpCollectionInvoke && in.Op != OpObjectAutoLoad && in.Op != OpVariantRef && in.Op != OpArrayResize && in.Integer != 0 {
					bad("unexpected instruction payload")
				}
				if in.Op != OpTestAssert && in.Op != OpCollectionInvoke && in.Op != OpObjectInvoke && len(in.Names) != 0 {
					bad("unexpected argument names")
				}
				if in.Op != OpStringLiteral && len(in.Units) != 0 {
					bad("unexpected string payload")
				}
				if in.Op != OpLibraryCall && in.Library != stdlib.None {
					bad("unexpected library member payload")
				}
				if (in.Op != OpConstant || !Floating(in.Type)) && math.Float64bits(in.Float) != 0 {
					bad("unexpected floating payload")
				}
				if in.Op != OpFieldRef && in.Op != OpClassFieldRef && in.Field != 0 {
					bad("unexpected field payload")
				}
				if in.Op >= OpClassNew && in.Op <= OpClassFieldRef || in.Op == OpInterfaceCheck {
					if in.Class == 0 || int(in.Class) > len(program.Classes) {
						bad("unknown class descriptor")
					}
				} else if in.Class != 0 && in.Op != OpObjectAutoLoad && in.Op != OpVariantRef && in.Op != OpArrayResize {
					bad("unexpected class descriptor")
				}
				if !checkType(in.Type) {
					bad("unresolved instruction type")
				}
				args := make([]Type, len(in.Args))
				for i, id := range in.Args {
					var ok bool
					args[i], ok = values[id]
					if !ok {
						bad("operand is undefined in this region or does not dominate use")
					}
					if movedElements[id] && Element(args[i]) {
						bad("element token ownership was already captured")
					}
					if args[i].Form == Result && in.Op != OpTry {
						bad("Result may only be consumed by try")
					}
					if captures[id] && in.Op != OpObjectInvoke && in.Op != OpArrayIndex {
						bad("captured receiver cannot escape into a language value")
					}
					if releasedElements[id] || referenceOwners[id] != 0 && releasedElements[referenceOwners[id]] {
						bad("use after element reference release")
					}
				}
				if in.Effects&MayRaise != 0 {
					if in.Type.Form != Result || ii+1 >= len(s.Instructions) || s.Instructions[ii+1].Op != OpTry || len(s.Instructions[ii+1].Args) != 1 || s.Instructions[ii+1].Args[0] != in.ID {
						bad("fallible operation needs Result and an immediate try continuation")
					}
				} else if in.Type.Form == Result {
					bad("Result producer must declare MayRaise")
				}
				if in.Op != OpTry && in.OnError != NoErrorContinuation {
					bad("error continuation belongs to try")
				}
				plain := in.Type
				plain.Form = Value
				var expected Effects
				arity := func(n int) bool {
					if len(args) != n {
						bad("incorrect operand count")
						return false
					}
					return true
				}
				if in.Op != OpObjectInvoke && (in.Member != "" || in.Invocation != 0) {
					bad("unexpected invocation payload")
				}
				switch in.Op {
				case OpObjectValue:
					expected = MayRaise | ManagesLifetime | WritesStorage | InvocationEffects(program)
					if !arity(1) || args[0] != (Type{Scalar: Variant}) || plain != (Type{Scalar: Variant}) {
						bad("invalid object value conversion")
					}
				case OpInvocationCapture:
					expected = MayRaise | ManagesLifetime
					if !arity(1) || args[0].Scalar != Variant || args[0].Form == Result || plain != (Type{Scalar: Variant}) {
						bad("invalid receiver capture")
					}
				case OpObjectInvoke:
					expected = MayRaise | ManagesLifetime | WritesStorage | InvocationEffects(program)
					if len(args) < 1 || args[0] != (Type{Scalar: Variant}) || plain != (Type{Scalar: Variant}) || len(in.Names) != len(args)-1 || in.Invocation&^InvokeDiscard != InvokeGet && in.Invocation&^InvokeDiscard != InvokeMethod && in.Invocation&^InvokeDiscard != InvokeGet|InvokeMethod && in.Invocation != InvokeLet && in.Invocation != InvokeSet {
						bad("invalid native invocation")
					}
					for _, a := range args {
						if a.Scalar != Variant || a.Form == Result {
							bad("invocation arguments require Variant values or typed views")
						}
					}
					if len(in.Args) > 0 && !captures[in.Args[0]] {
						bad("invocation requires a captured receiver")
					}
				case OpObjectAutoLoad:
					expected = MayRaise | ManagesLifetime | WritesStorage
					if !arity(1) || args[0] != (Type{Scalar: Object, Form: Reference}) || plain != (Type{Scalar: Object}) || in.Class != 0 && (int(in.Class) > len(program.Classes) || in.Integer != 0) || in.Class == 0 && (in.Integer < 1 || in.Integer > 4) {
						bad("invalid lazy object load")
					}
				case OpClassNew:
					expected = MayRaise | ManagesLifetime
					if !arity(0) || plain != (Type{Scalar: Object}) {
						bad("invalid class construction")
					}
				case OpClassCheck, OpClassCast, OpInterfaceCheck:
					expected = MayRaise
					if !arity(1) || args[0] != (Type{Scalar: Object}) || plain != (Type{Scalar: Void}) {
						bad("invalid class receiver check")
					}
					if in.Op == OpClassCheck && len(args) == 1 {
						checkedObjects[in.Args[0]] = in.Class
					}
					if in.Op == OpInterfaceCheck && len(args) == 1 {
						checkedInterfaces[in.Args[0]] = in.Class
					}
				case OpClassFieldRef:
					if !arity(1) || args[0] != (Type{Scalar: Object}) || checkedObjects[in.Args[0]] != in.Class {
						bad("field requires a checked owned receiver")
					}
					if in.Class != 0 && int(in.Class) <= len(program.Classes) {
						fields := program.Classes[in.Class-1].Fields
						if int(in.Field) >= len(fields) {
							bad("unknown class field")
						} else {
							want := fields[in.Field].Type
							want.Form = Reference
							if in.Type != want {
								bad("class field reference type differs")
							}
						}
					}
				case OpCollectionInvoke:
					expected = MayRaise | ManagesLifetime | WritesStorage
					if len(args) < 1 {
						bad("missing Collection receiver")
						break
					}
					if args[0] != (Type{Scalar: Object}) || in.Integer < 1 || in.Integer > 4 || len(in.Names) != len(args)-1 {
						bad("invalid Collection invocation")
					}
					for _, a := range args[1:] {
						if a != (Type{Scalar: Variant}) && a != (Type{Scalar: Object}) {
							bad("invalid Collection argument")
						}
					}
					if in.Integer == 1 || in.Integer == 3 {
						if plain != (Type{Scalar: Void}) {
							bad("Collection mutation must return Void")
						}
					} else if in.Integer == 4 {
						if plain != (Type{Scalar: Long}) && plain != (Type{Scalar: Variant}) {
							bad("Count requires Long/Variant result")
						}
					} else if plain != (Type{Scalar: Variant}) && plain != (Type{Scalar: Object}) {
						bad("Item requires Variant/Object result")
					}
				case OpGosubPush, OpGosubPop:
					if in.Op == OpGosubPush {
						gosubPushes++
					} else {
						gosubPops++
					}
					if ii != len(s.Instructions)-2 {
						bad("GoSub stack operation must immediately precede the region terminator")
					}
					expected = MayRaise | WritesStorage | ManagesLifetime
					if !arity(0) {
						bad("GoSub operations have no operands")
					}
					if in.Op == OpGosubPush && (plain != (Type{Scalar: Void}) || s.End != Gosub) {
						bad("push requires a GoSub region")
					}
					if in.Op == OpGosubPop && (plain != (Type{Scalar: GosubAddress}) || s.End != GosubReturn) {
						bad("pop requires a GoSub Return region")
					}
				case OpParamArrayPack:
					expected = MayRaise | WritesStorage | ManagesLifetime
					if len(args) < 1 {
						bad("missing ParamArray destination")
						break
					}
					if args[0] != (Type{Scalar: ArrayVariant, Form: Reference}) || plain != (Type{Scalar: Void}) {
						bad("invalid ParamArray destination")
					}
					for _, a := range args[1:] {
						if a != (Type{Scalar: Variant}) && a != (Type{Scalar: Variant, Form: Reference}) {
							bad("invalid ParamArray argument")
						}
					}
				case OpObjectCheck:
					expected = MayRaise
					if !arity(2) || args[0] != (Type{Scalar: Object}) || args[1] != (Type{Scalar: Long}) || plain != (Type{Scalar: Void}) {
						bad("invalid virtual class check")
					}
				case OpObjectNew:
					expected = MayRaise | ManagesLifetime
					if !arity(1) || args[0] != (Type{Scalar: Long}) || plain != (Type{Scalar: Object}) {
						bad("invalid virtual constructor")
					}
				case OpCreateObject:
					expected = MayRaise | ManagesLifetime
					if !arity(1) || args[0] != (Type{Scalar: String}) || plain != (Type{Scalar: Object}) {
						bad("invalid CreateObject")
					}
				case OpObjectNull:
					if !arity(0) || in.Type != (Type{Scalar: Object}) {
						bad("invalid Nothing")
					}
				case OpObjectEqual:
					if !arity(2) || args[0] != (Type{Scalar: Object}) || args[1] != args[0] || in.Type != (Type{Scalar: Boolean}) {
						bad("invalid object identity")
					}
				case OpTestAssert:
					expected = MayRaise | ManagesLifetime | WritesStorage
					if len(args) < 1 || args[0] != (Type{Scalar: Object}) || plain != (Type{Scalar: Void}) || in.Integer < 1 || in.Integer > 13 || len(in.Names) != len(args)-1 {
						bad("invalid assertion call")
					}
					for _, a := range args[1:] {
						if a != (Type{Scalar: Variant}) && a != (Type{Scalar: Object}) {
							bad("invalid assertion argument")
						}
					}
				case OpErrRaise:
					expected = MayRaise | WritesStorage | ManagesLifetime | WritesAmbientErr
					if !arity(5) || plain.Scalar != Void {
						bad("invalid Err.Raise")
					}
					for _, t := range args {
						if t != (Type{Scalar: Variant}) {
							bad("Err.Raise requires value snapshots")
						}
					}
				case OpErrClear:
					expected = WritesStorage | ManagesLifetime | WritesAmbientErr
					if !arity(0) || plain.Scalar != Void {
						bad("invalid Err.Clear")
					}
				case OpErrGet:
					expected = MayRaise | ReadsAmbientErr
					if Managed(plain) {
						expected |= ManagesLifetime
					}
					if !arity(1) || args[0] != (Type{Scalar: Long}) || plain.Scalar != Long && plain.Scalar != String {
						bad("invalid Err property read")
					}
				case OpErrSet:
					expected = MayRaise | WritesStorage | ManagesLifetime | WritesAmbientErr
					if !arity(2) || args[0] != (Type{Scalar: Long}) || args[1] != (Type{Scalar: Variant}) || plain.Scalar != Void {
						bad("invalid Err property write")
					}
				case OpWithCheck:
					expected = MayRaise
					target := slot(in.Slot)
					if !arity(0) || target == nil || target.Type.Scalar != Receiver || plain.Scalar != Void {
						bad("invalid With check")
					}
				case OpKeepElement, OpWithCapture, OpWithReference:
					target := slot(in.Slot)
					if target == nil || target.Kind != LoopState {
						bad("capture requires persistent scope storage")
						break
					}
					if in.Op == OpKeepElement {
						expected = WritesStorage | ManagesLifetime
						if len(in.Args) == 1 {
							movedElements[in.Args[0]] = true
						}
						if !arity(1) || !Element(target.Type) || args[0] != target.Type || plain.Scalar != Void {
							bad("invalid captured element")
						}
					} else if in.Op == OpWithCapture {
						expected = WritesStorage
						if !arity(1) || target.Type.Scalar != Receiver || args[0] != (Type{Scalar: Record, Record: target.Type.Record, Form: Reference}) || plain.Scalar != Void {
							bad("invalid With capture")
						}
					} else {
						expected = 0
						if ii < 2 || s.Instructions[ii-2].Op != OpWithCheck || s.Instructions[ii-2].Slot != in.Slot {
							bad("With reference requires a checked capture")
						}
						if !arity(0) || target.Type.Scalar != Receiver || in.Type != (Type{Scalar: Record, Record: target.Type.Record, Form: Reference}) {
							bad("invalid With reference")
						}
					}
				case OpFieldRef:
					if !arity(1) || args[0].Scalar != Record || args[0].Form == Result || args[0].Record == 0 || int(args[0].Record) > len(program.Records) {
						bad("field access requires a resolved UDT")
						break
					}
					fields := program.Records[args[0].Record-1].Fields
					if int(in.Field) >= len(fields) {
						bad("invalid UDT field index")
						break
					}
					want := fields[in.Field].Type
					want.Form = Reference
					if in.Type != want {
						bad("field reference type differs from its declaration")
					}
					referenceOwners[in.ID] = referenceOwners[in.Args[0]]
				case OpCondition:
					if !arity(1) || args[0].Form != Value || args[0].Scalar < Boolean || args[0].Scalar > Variant || args[0].Scalar == String || plain.Scalar != Boolean {
						bad("invalid condition coercion")
					}
					if len(args) == 1 && (args[0].Scalar == Variant || Floating(args[0])) {
						expected = MayRaise
					}
				case OpCheckInitialized:
					expected = MayRaise
					if !arity(1) || args[0] != (Type{Scalar: Boolean}) || plain.Scalar != Void {
						bad("initialization check requires Boolean and Void result")
					}
				case OpCheckDispatch:
					expected = MayRaise
					if !arity(1) || args[0] != (Type{Scalar: Integer}) || plain.Scalar != Void {
						bad("dispatch check requires Integer and Void result")
					}
				case OpForValidate, OpForTest:
					expected = MayRaise
					if !arity(3) {
						bad("For operations require three operands")
					}
					for _, t := range args {
						if t != (Type{Scalar: Variant}) {
							bad("dynamic For operations require Variant operands")
						}
					}
					if in.Op == OpForValidate && plain.Scalar != Void || in.Op == OpForTest && plain.Scalar != Boolean {
						bad("invalid dynamic For result")
					}
				case OpEnumStart, OpEnumNext, OpEnumObjectNext:
					expected = MayRaise | ManagesLifetime | WritesStorage
					s := slot(in.Slot)
					if s == nil || s.Type.Scalar != Enumerator {
						bad("enumeration requires an enumerator slot")
					}
					if in.Op == OpEnumStart {
						if !arity(2) || !(args[0] == (Type{Scalar: Object}) || args[0].Form == Reference && (Array(args[0]) || args[0].Scalar == Variant)) || args[1] != (Type{Scalar: Long}) || plain.Scalar != Void {
							bad("invalid enumeration start")
						}
					} else if in.Op == OpEnumObjectNext {
						if !arity(2) || args[0] != (Type{Scalar: Object, Form: Reference}) || args[1] != (Type{Scalar: Long}) || plain.Scalar != Boolean {
							bad("invalid object enumeration advance")
						}
					} else if !arity(1) || args[0] != (Type{Scalar: Variant, Form: Reference}) || plain.Scalar != Boolean {
						bad("invalid enumeration advance")
					}
				case OpArrayResize, OpArrayErase, OpArrayBound, OpArrayIndex:
					if in.Op == OpArrayIndex && len(args) > 0 && args[0] == (Type{Scalar: Variant}) && !captures[in.Args[0]] {
						bad("dynamic index requires a captured receiver")
					}
					expected = MayRaise
					minimum := 1
					if in.Op == OpArrayResize {
						minimum = 5
						expected |= WritesStorage | ManagesLifetime
					}
					if in.Op == OpArrayErase {
						expected |= WritesStorage | ManagesLifetime
					}
					if in.Op == OpArrayBound {
						minimum = 3
					}
					if in.Op == OpArrayIndex {
						minimum = 2
						expected |= ManagesLifetime
					}
					if len(args) < minimum || (args[0].Form != Reference && !(in.Op == OpArrayIndex && args[0] == (Type{Scalar: Variant}))) || !Array(args[0]) && args[0].Scalar != Variant {
						bad("array operation requires array or Variant storage and arguments")
					}
					for i, arg := range args {
						if i == 0 {
							continue
						}
						if arg != (Type{Scalar: Long}) {
							bad("array bounds/indices require Long values")
						}
					}
					switch in.Op {
					case OpArrayResize:
						if int(in.Class) > len(program.Classes) || in.Integer < 0 || in.Integer > 4 || in.Class != 0 && in.Integer != 0 || (in.Class != 0 || in.Integer != 0) && len(args) > 0 && args[0].Scalar != ArrayObject && args[0].Scalar != Variant {
							bad("invalid object array constraint")
						}
						if len(args) > 1 && (args[0].Scalar == ArrayObject || in.Class != 0 || in.Integer != 0) {
							tagOK := false
							for _, definition := range s.Instructions[:ii] {
								if definition.ID == in.Args[1] && definition.Op == OpConstant && definition.Type == (Type{Scalar: Long}) && definition.Integer == 0x1000 {
									tagOK = true
								}
							}
							if !tagOK || args[0].Scalar == ArrayObject && (args[0].Class != in.Class || int64(args[0].Builtin) != in.Integer) {
								bad("object array resize differs from its element constraint")
							}
						}
						if len(args) > 123 || (len(args)-3)%2 != 0 || plain.Scalar != Void {
							bad("resize requires paired bounds and Void result")
						}
					case OpArrayErase:
						if !arity(1) || plain.Scalar != Void {
							bad("erase requires Void result")
						}
					case OpArrayBound:
						if !arity(3) || plain.Scalar != Long {
							bad("array bound requires dimension, selector, and Long result")
						}
					case OpArrayIndex:
						if len(args) > 61 || !Element(plain) {
							bad("index requires an owned element token")
						}
						if len(args) > 0 && (Array(args[0]) && ElementType(args[0]) != ElementType(plain) || args[0].Scalar == Variant && plain.Scalar != ElementVariant) {
							bad("element token differs from array element type")
						}
					}
				case OpElementRef:
					if !arity(1) || !Element(args[0]) || in.Type != (Type{Scalar: ElementType(args[0]).Scalar, Record: ElementType(args[0]).Record, Form: Reference}) {
						bad("element reference differs from lock token")
					}
					if len(args) == 1 {
						referenceOwners[in.ID] = in.Args[0]
					}
				case OpReleaseElement:
					expected = ManagesLifetime
					if !arity(1) || !Element(args[0]) || args[0].Form != Value || plain.Scalar != Void {
						bad("release requires an owned element token")
					}
					if len(args) == 1 {
						releasedElements[in.Args[0]] = true
					}
				case OpReadReference, OpWriteReference:
					expected = MayRaise
					if in.Op == OpWriteReference {
						expected |= WritesStorage
					}
					if len(args) > 0 {
						t := args[0]
						t.Form = Value
						if Managed(t) {
							expected |= ManagesLifetime
						}
					}
					if len(args) == 0 || args[0].Form != Reference || args[0].Scalar < Void || Element(args[0]) {
						bad("read/write requires typed storage")
					}
					if in.Op == OpReadReference {
						want := Type{}
						if len(args) > 0 {
							want = args[0]
							want.Form = Value
						}
						if !arity(1) || plain != want {
							bad("reference load type differs")
						}
					} else if !arity(2) || args[1] != (Type{Scalar: args[0].Scalar, Record: args[0].Record, Class: args[0].Class, Builtin: args[0].Builtin}) || plain.Scalar != Void {
						bad("reference store type differs")
					}
				case OpLoadManaged, OpStoreManaged:
					expected = MayRaise | ManagesLifetime
					target := slot(in.Slot)
					if target == nil || !Managed(target.Type) {
						bad("managed storage requires String or Variant slot")
						break
					}
					if in.Op == OpLoadManaged {
						if target.AutoNew != 0 || target.AutoClass != 0 {
							expected |= WritesStorage
						}
						if !arity(0) || plain != target.Type {
							bad("managed load type differs")
						}
					} else {
						expected |= WritesStorage
						if !arity(1) || args[0] != target.Type || plain.Scalar != Void {
							bad("managed store type differs")
						}
					}
				case OpBox:
					expected = MayRaise | ManagesLifetime
					if !arity(1) || args[0].Form != Value || (!ScalarValue(args[0]) || args[0].Scalar == Variant) && !Array(args[0]) && args[0].Scalar != Object || plain != (Type{Scalar: Variant}) {
						bad("invalid Variant boxing")
					}
				case OpUnbox:
					expected = MayRaise | ManagesLifetime
					if !arity(1) || args[0] != (Type{Scalar: Variant}) || (!ScalarValue(plain) || plain.Scalar == Variant) && !Array(plain) && plain.Scalar != Object {
						bad("invalid Variant conversion")
					}
				case OpVariantConstant:
					if !arity(0) || in.Type != (Type{Scalar: Variant}) || in.Integer < 0 || in.Integer > 2 {
						bad("invalid Empty/Null/Missing constant")
					}
				case OpVariantRef:
					if (in.Class != 0 || in.Integer != 0) && (len(args) != 1 || args[0] != (Type{Scalar: Object, Form: Reference}) || int(in.Class) > len(program.Classes) || in.Integer < 0 || in.Integer > 4 || in.Class != 0 && in.Integer != 0) {
						bad("invalid object storage constraint on Variant view")
					}
					if !arity(1) || args[0].Form != Reference || (!ScalarValue(args[0]) || args[0].Scalar == Variant) && !Array(args[0]) && args[0].Scalar != Object || in.Type != (Type{Scalar: Variant, Form: Reference}) {
						bad("typed Variant alias requires typed storage")
					}
					if len(args) == 1 && args[0].Scalar == String && !pinnedStrings[in.Args[0]] {
						bad("String Variant alias requires a checked BSTR pin")
					}
					if len(args) == 1 {
						referenceOwners[in.ID] = referenceOwners[in.Args[0]]
					}
				case OpPinString:
					expected = MayRaise | ManagesLifetime
					if !arity(1) || args[0] != (Type{Scalar: String, Form: Reference}) || plain != (Type{Scalar: Void}) {
						bad("String pin requires String storage")
					}
					if len(args) == 1 {
						pinnedStrings[in.Args[0]] = true
					}
				case OpVarType, OpIsEmpty, OpIsNull, OpIsMissing, OpIsArray:
					want := Boolean
					if in.Op == OpVarType {
						want = Integer
					}
					if !arity(1) || args[0].Scalar != Variant || args[0].Form == Result || in.Type != (Type{Scalar: want}) {
						bad("invalid Variant query")
					}
				case OpConstant:
					valid := fits(in.Integer, in.Type)
					if Floating(in.Type) {
						valid = in.Integer == 0 && !math.IsInf(in.Float, 0) && !math.IsNaN(in.Float) && (in.Type.Scalar != Single || float64(float32(in.Float)) == in.Float)
					}
					if !arity(0) || in.Type.Form != Value || !valid {
						bad("invalid typed literal")
					}
				case OpStringLiteral, OpNullString:
					if !arity(0) || in.Type != (Type{Scalar: String}) || in.Op == OpNullString && len(in.Units) != 0 {
						bad("invalid String constant")
					}
				case OpConcat:
					expected = MayRaise | ManagesLifetime
					if plain.Scalar == Variant {
						if !arity(2) || args[0] != plain || args[1] != plain {
							bad("Variant concat requires Variant operands")
						}
						break
					}
					if !arity(2) || args[0] != (Type{Scalar: String}) || args[1] != args[0] || plain != args[0] {
						bad("concat requires two Strings")
					}
				case OpLen, OpLenB:
					if plain.Scalar == Variant {
						expected = MayRaise | ManagesLifetime
						if !arity(1) || args[0] != (Type{Scalar: Variant}) {
							bad("Variant length requires Variant")
						}
						break
					}
					if in.Op == OpLenB {
						expected = MayRaise
					}
					if !arity(1) || args[0] != (Type{Scalar: String}) || plain != (Type{Scalar: Long}) {
						bad("length requires String and returns Long")
					}
				case OpChrW:
					expected = MayRaise
					if !arity(1) || args[0] != (Type{Scalar: Long}) || plain != (Type{Scalar: String}) {
						bad("ChrW requires Long and returns String")
					}
				case OpMid:
					expected = MayRaise | ManagesLifetime
					if !arity(3) || args[0] != (Type{Scalar: String}) || args[1] != (Type{Scalar: Long}) || args[2] != args[1] || plain != args[0] {
						bad("Mid requires String, Long, Long and returns String")
					}
				case OpMidCheck, OpMidAssign:
					expected = MayRaise
					count := 3
					if in.Op == OpMidAssign {
						count = 4
						expected |= WritesStorage | ManagesLifetime
					}
					if !arity(count) || args[0] != (Type{Scalar: String, Form: Reference}) || args[1] != (Type{Scalar: Long}) || args[2] != args[1] || plain != (Type{Scalar: Void}) {
						bad("Mid mutation requires String storage and Long bounds")
					}
					if in.Op == OpMidAssign && len(args) == 4 && args[3] != (Type{Scalar: String}) {
						bad("Mid replacement requires String")
					}
				case OpLoad, OpStore, OpAddress:
					target := slot(in.Slot)
					if target == nil {
						bad("unknown storage")
						break
					}
					if target.Type.Scalar == Enumerator {
						bad("enumerator slots require enumeration operations")
					}
					switch in.Op {
					case OpLoad:
						if Managed(target.Type) {
							bad("managed slots require a fallible managed load")
						}
						if !arity(0) || in.Type != target.Type {
							bad("load type differs from storage")
						}
					case OpAddress:
						want := target.Type
						want.Form = Reference
						if !arity(0) || in.Type != want {
							bad("address must alias storage of exactly the same type")
						}
					case OpStore:
						expected = WritesStorage
						if Managed(target.Type) {
							bad("managed slots require a fallible managed store")
						}
						if !arity(1) || args[0] != target.Type || in.Type != (Type{Scalar: Void}) || in.ID != 0 {
							bad("store requires a scalar of exactly the storage type and no result")
						}
					}
				case OpConvert:
					if !arity(1) || (!numeric(args[0]) && !(args[0] == (Type{Scalar: Boolean}) && numeric(plain))) || !numeric(plain) || plain.Scalar == Byte || args[0] == plain {
						bad("unsupported conversion")
					} else if plain.Scalar == Integer && args[0].Scalar != Boolean && args[0].Scalar != Byte || Floating(args[0]) && (plain.Scalar == Long || plain.Scalar == Single) {
						expected = MayRaise
					}
				case OpAdd, OpSubtract, OpMultiply, OpNegate, OpDivide:
					expected = MayRaise
					if plain.Scalar == Variant {
						expected |= ManagesLifetime
					}
					n := 2
					if in.Op == OpNegate {
						n = 1
					}
					if in.Op == OpDivide && len(args) == 3 {
						n = 3
						expected |= WritesStorage
						if plain.Scalar != Double || args[2] != (Type{Scalar: Double, Form: Reference}) {
							bad("division failure destination requires ref<Double>")
						}
					}
					if !arity(n) || !numeric(plain) && plain.Scalar != Variant || in.Op == OpDivide && !Floating(plain) && plain.Scalar != Variant {
						bad("invalid checked arithmetic operands")
					}
					operands := args
					if in.Op == OpDivide && len(args) == 3 {
						operands = args[:2]
					}
					for _, t := range operands {
						if t != plain {
							bad("arithmetic operand types differ")
						}
					}

				case OpIntegerDivide, OpModulo:
					expected = MayRaise
					if plain.Scalar == Variant {
						expected |= ManagesLifetime
					}
					if !arity(2) || !integral(plain) && plain != (Type{Scalar: Variant}) {
						bad("invalid integral division result")
					}
					for _, t := range args {
						if t != plain {
							bad("integral division operand types differ")
						}
					}
				case OpLibraryCall:
					expected = MayRaise
					signature, ok := LookupLibraryCall(in.Library)
					if !ok {
						bad("unknown or unsupported library member")
					} else {
						expected = signature.Effects
						if !arity(signature.Arity) {
							bad("invalid library argument count")
						}
						if plain != signature.Result {
							bad("invalid library result type")
						}
					}
					for _, t := range args {
						if t != (Type{Scalar: Variant}) {
							bad("library arguments require Variant snapshots")
						}
					}
				case OpNot, OpAnd, OpOr, OpXor:
					n := 2
					if in.Op == OpNot {
						n = 1
					}
					if !arity(n) || in.Type.Form != Value || plain.Scalar < Boolean || plain.Scalar > Long {
						bad("invalid logical operation")
					}
					for _, t := range args {
						if t != in.Type {
							bad("logical operand types differ")
						}
					}
				case OpEqual, OpNotEqual, OpLess, OpLessEqual, OpGreater, OpGreaterEqual:
					if plain.Scalar == Variant {
						expected = MayRaise | ManagesLifetime
						if !arity(3) || args[0] != (Type{Scalar: Variant}) || args[1] != args[0] || args[2] != (Type{Scalar: Long}) {
							bad("dynamic comparison requires Variant operands and coercion mode")
						}
						break
					}
					if len(args) == 2 && Floating(args[0]) {
						expected = MayRaise
					}
					if !arity(2) || args[0] != args[1] || args[0].Form != Value || args[0].Scalar < Boolean || args[0].Scalar > Double || plain != (Type{Scalar: Boolean}) {
						bad("invalid comparison types")
					}
				case OpCall, OpInterfaceCall:
					if in.Callee == 0 || int(in.Callee) > len(program.Procedures) || program.Procedures[in.Callee-1] == nil {
						bad("unknown callee")
						break
					}
					callee := program.Procedures[in.Callee-1]
					if callee.Class != 0 && (len(in.Args) == 0 || checkedObjects[in.Args[0]] != callee.Class && (in.Op != OpInterfaceCall || checkedInterfaces[in.Args[0]] != callee.Class)) {
						bad("instance call requires a checked receiver")
					}
					expected = MayRaise | callee.Effects
					if in.Op == OpInterfaceCall {
						expected = InterfaceCallEffects(program, in.Callee)
						if !interfaceMembers[in.Callee] {
							bad("callee is not an interface contract")
						}
					}
					if !arity(len(callee.Parameters)) || plain != callee.ReturnType {
						bad("call signature differs")
					}
					for i, id := range callee.Parameters {
						if i >= len(args) || id == 0 || int(id) > len(callee.Slots) {
							bad("invalid call parameter")
							continue
						}
						param := callee.Slots[id-1]
						want := param.Type
						if param.ByRef {
							want.Form = Reference
						}
						if args[i] != want {
							bad("call argument violates value/alias type")
						}
					}
				case OpTry:
					if ii > 0 && s.Instructions[ii-1].Op == OpInvocationCapture {
						captures[in.ID] = true
					}
					if Managed(in.Type) {
						expected = ManagesLifetime
					}
					if !arity(1) || args[0].Form != Result || in.Type != (Type{Scalar: args[0].Scalar, Record: args[0].Record, Class: args[0].Class, Builtin: args[0].Builtin}) || ii == 0 || s.Instructions[ii-1].ID != in.Args[0] || s.Instructions[ii-1].Effects&MayRaise == 0 || in.OnError != Propagate {
						bad("try must extract the immediately preceding Result with success and propagation continuations")
					}
				default:
					bad("unsupported operation")
				}
				if in.Op == OpClassNew || in.Op == OpObjectAutoLoad {
					expected |= InitializationEffects(program, in.Class)
				}
				if in.Op == OpLoadManaged {
					if s := slot(in.Slot); s != nil {
						expected |= InitializationEffects(program, s.AutoClass)
					}
				}
				expected |= DefaultInvocationEffects(program, in)
				if expected&ManagesLifetime != 0 {
					expected |= TerminationEffects(program)
				}
				wantForm := Value
				if in.Op == OpAddress || in.Op == OpVariantRef || in.Op == OpElementRef || in.Op == OpFieldRef || in.Op == OpClassFieldRef || in.Op == OpWithReference {
					wantForm = Reference
				} else if expected&MayRaise != 0 {
					wantForm = Result
				}
				if in.Type.Form != wantForm || in.Effects != expected {
					bad("incorrect effects or result form")
				}
				ownership := NoOwnership
				if Managed(in.Type) {
					ownership = Owned
					if in.Op == OpStringLiteral || in.Op == OpNullString || in.Op == OpVariantConstant {
						ownership = Immortal
					}
				}
				if in.Ownership != ownership {
					bad("incorrect managed value ownership")
				}
				if ownership == Owned {
					owned = append(owned, in.ID)
				}
				if in.Op != OpStore {
					if in.ID == 0 || allValues[in.ID] {
						bad("invalid or duplicate value identity")
					}
					allValues[in.ID] = true
					values[in.ID] = in.Type
				}
			}
			if s.End == Gosub && gosubPushes != 1 || s.End == GosubReturn && gosubPops != 1 {
				report(s.Source, "GoSub terminator requires exactly one matching stack operation")
			}
			if s.End == GosubReturn && values[s.ReturnAddress] != (Type{Scalar: GosubAddress}) {
				report(s.Source, "Return requires an extracted GoSub address")
			}
			if s.End == Branch && values[s.Condition] != (Type{Scalar: Boolean}) {
				report(s.Source, "branch condition must be a Boolean defined in its region")
			}
			if s.End == Switch {
				t := values[s.Selector]
				if !integral(t) {
					report(s.Source, "switch selector must be an Integer/Long defined in its region")
				}
				for _, item := range s.Cases {
					if !fits(item.Value, t) {
						report(s.Source, "switch value is outside its selector type")
					}
				}
			}
			if len(owned) != len(s.Cleanup) {
				report(s.Source, "managed cleanup must cover every owned value")
			} else {
				for i, id := range owned {
					if s.Cleanup[i] != id {
						report(s.Source, "managed cleanup has incorrect ownership or order")
					}
				}
			}
		}
		if UsesErrors(program) && effects&MayRaise != 0 {
			effects |= WritesAmbientErr
		}
		if p.Effects != effects {
			report(p.Source, "procedure effects do not match its operations")
		}
	}
	return diagnostics
}

// Package ir defines LLVM-independent, typed VBA semantics. It does not reuse
// binder flow nodes: those describe analysis, rather than executable operations.
package ir

import (
	"fmt"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/stdlib"
)

type Scalar uint8

const (
	Invalid Scalar = iota
	Void
	Boolean // VBA False = 0, True = -1; not an LLVM i1 layout decision.
	Byte    // Unsigned 8-bit language value.
	Integer // Signed 16-bit language value.
	Long    // Signed 32-bit language value, including on 64-bit targets.
	Single  // IEEE binary32, rounded at typed operations/storage boundaries.
	Double  // IEEE binary64. No fast-math assumptions.
	String  // A sequence of UTF-16 code units; its native representation is private.
	Variant // Automation tagged value; distinct from the Result error channel.
	ArrayBoolean
	ArrayByte
	ArrayInteger
	ArrayLong
	ArraySingle
	ArrayDouble
	ArrayString
	ArrayVariant
	ElementBoolean // Owned descriptor lock and element address; not a language value.
	ElementByte
	ElementInteger
	ElementLong
	ElementSingle
	ElementDouble
	ElementString
	ElementVariant
	Enumerator // Private array-enumeration state, stored only in a loop-state slot.
	Record     // Nominal UDT value; Type.Record identifies its field definition.
	ArrayRecord
	ElementRecord
	Object       // Private object reference with stable instance identity; never a COM pointer.
	Receiver     // Captured With pointer; locks/owned snapshots live in scope slots.
	GosubAddress // Private resumption token, lowered to an LLVM block address; never a VBA value.
	Date         // Automation DATE serial, distinct from a Double language value.
	Currency     // Signed 64-bit fixed-point integer scaled by 10,000.
	ArrayDate
	ArrayCurrency
	ElementDate
	ElementCurrency
	ArrayObject
	ElementObject
)

type Form uint8

const (
	Value     Form = iota
	Reference      // An alias to scalar storage, never a converted value.
	Result         // Ok(T) | Error(VbaError), independent of a native ABI.
)

type Type struct {
	Scalar  Scalar
	Form    Form
	Record  RecordID // Nonzero only for a nominal UDT.
	Class   ClassID  // Nominal element constraint for ArrayObject; zero for Object().
	Builtin uint8    // Built-in class element constraint for ArrayObject.
}

func (t Type) String() string {
	names := [...]string{"invalid", "Void", "Boolean", "Byte", "Integer", "Long", "Single", "Double", "String", "Variant", "Boolean()", "Byte()", "Integer()", "Long()", "Single()", "Double()", "String()", "Variant()", "element<Boolean>", "element<Byte>", "element<Integer>", "element<Long>", "element<Single>", "element<Double>", "element<String>", "element<Variant>", "enumerator"}
	name := "invalid"
	if int(t.Scalar) < len(names) {
		name = names[t.Scalar]
	}
	if t.Scalar == Record || t.Scalar == ArrayRecord || t.Scalar == ElementRecord || t.Scalar == Receiver {
		name = fmt.Sprintf("UDT@%d", t.Record)
		if t.Scalar == ArrayRecord {
			name += "()"
		}
		if t.Scalar == ElementRecord {
			name = "element<" + name + ">"
		}
		if t.Scalar == Receiver {
			name = "receiver<" + name + ">"
		}
	}
	if t.Scalar == GosubAddress {
		name = "gosub_address"
	}
	if t.Scalar == Object {
		name = "Object"
	}
	if t.Scalar == ArrayObject {
		name = "Object()"
		if t.Class != 0 {
			name = fmt.Sprintf("class@%d()", t.Class)
		}
		if t.Builtin != 0 {
			name = fmt.Sprintf("builtin@%d()", t.Builtin)
		}
	}
	if t.Scalar == ElementObject {
		name = "element<Object>"
	}
	if t.Scalar >= Date && t.Scalar <= ElementCurrency {
		name = [...]string{"Date", "Currency", "Date()", "Currency()", "element<Date>", "element<Currency>"}[t.Scalar-Date]
	}
	if t.Form == Reference {
		return "ref<" + name + ">"
	}
	if t.Form == Result {
		return "Result<" + name + ">"
	}
	return name
}

type Effects uint8

const (
	MayRaise Effects = 1 << iota
	WritesStorage
	CallsExternal
	ReadsAmbientErr
	WritesAmbientErr
	ManagesLifetime // Acquires/releases owned values and descriptor locks, including on errors.
)

// Source retains half-open UTF-8 byte positions, and optional syntax provenance.
// Node belongs to the immutable checked snapshot; consumers must not mutate it.
type Source struct {
	FileName string
	Range    core.TextRange
	Node     *ast.Node
}

type ProcedureID uint32
type StatementID uint32
type SlotID uint32
type ValueID uint32
type ScopeID uint32
type RecordID uint32
type GlobalID uint32
type ClassID uint32

type Program struct {
	Name       string
	Procedures []*Procedure       // Stable source order; IDs are one-based slice indices.
	Records    []RecordDefinition // Nominal field shapes, independent of native layout.
	Globals    []Global
	Classes    []ClassDefinition
}

// Classes describe reference types independently of native object layouts.
// Object values use an erased private representation; class checks and member
// associations retain the nominal identity selected by the checked frontend.
type ClassDefinition struct {
	ID                    ClassID
	Name                  string
	Source                Source
	Fields                []RecordField
	Members               []ProcedureID
	Initialize, Terminate ProcedureID
	Interfaces            []InterfaceImplementation
}

type InterfaceImplementation struct {
	Class   ClassID
	Methods []InterfaceMethod
}

type InterfaceMethod struct{ Declaration, Implementation ProcedureID }

type InvocationKind uint8

const (
	InvokeMethod InvocationKind = 1 << iota
	InvokeGet
	InvokeLet
	InvokeSet
	InvokeDiscard // Statement invocation; a Sub result is permitted.
)

// Invocation metadata is shared by native adapters and deterministic libraries.
// Default procedures materialize a constant in private parameter storage.
type InvocationParameter struct {
	Name                        string
	Type                        Type
	Class                       ClassID
	Builtin                     uint8
	ByRef, Optional, ParamArray bool
	Default                     ProcedureID
}
type InvocationSignature struct {
	Name       string
	Kind       InvocationKind
	Default    bool
	Parameters []InvocationParameter
}

// Global owns project-lifetime storage. Procedure slots may alias it, but do
// not initialize or release it on procedure entry/exit.
type Global struct {
	ID        GlobalID
	Name      string
	Type      Type
	Source    Source
	Bounds    []ArrayBound
	AutoNew   uint8 // Virtual class kind; zero means ordinary Nothing initialization.
	AutoClass ClassID
}

type RecordDefinition struct {
	ID     RecordID
	Name   string
	Source Source
	Fields []RecordField
}
type RecordField struct {
	Name      string
	Type      Type
	Source    Source
	Bounds    []ArrayBound // Nonempty for a fixed array field; empty for dynamic arrays.
	AutoClass ClassID
	AutoNew   uint8
	Class     ClassID // Declared nominal object constraint, independent of lazy creation.
	Builtin   uint8
	Public    bool
}
type ArrayBound struct{ Lower, Upper int32 }

type SlotKind uint8

const (
	Local SlotKind = iota
	Parameter
	FunctionResult
	Temporary
	LoopState   // Persistent loop/Select captures across regions; released on replacement/exit.
	GlobalAlias // Non-owning alias of Program.Globals[Global-1].
)

// Activation slots are allocated on entry; GlobalAlias reuses project storage.
// Non-parameter activation slots have the language's zero
// initial value. A ByVal parameter is a local copy; ByRef aliases caller storage.
// Numeric slots need no cleanup. Non-ByRef String/Variant slots own their current value
// and are released on procedure exit. Globals have independent project ownership;
// procedure Static storage is excluded.
type Slot struct {
	ParamArray bool // A borrowed call-specific array of Variant parameter bindings.
	AutoNew    uint8
	AutoClass  ClassID
	Global     GlobalID // Nonzero exactly for GlobalAlias slots.
	ID         SlotID
	Name       string
	Type       Type
	Kind       SlotKind
	ByRef      bool
	Source     Source
}

type Procedure struct {
	Invocation *InvocationSignature // Public native member contract, nil for private members.
	Class      ClassID              // Zero for standard-module procedures.
	Receiver   SlotID               // First, ByVal Object parameter for instance members.
	ID         ProcedureID
	Name       string // Qualified module.procedure name, not a native linkage name.
	Source     Source
	ReturnType Type // Language return type; effects and Result ABI are separate.
	Parameters []SlotID
	Slots      []Slot
	ResultSlot SlotID // Zero for Sub; otherwise the default-initialized result variable.
	Statements []*Statement
	Effects    Effects
	Scopes     []Scope // Resource scopes, independent of binder lexical/flow metadata.
}

type Scope struct {
	ID      ScopeID
	Parent  ScopeID
	Cleanup []SlotID // Enumerator slots to release on edges leaving this scope.
}

// A statement is a semantic recovery region. IDs are procedure-local and include
// separate colon-delimited statements. Retry reenters this region; Next names the
// following recovery region (zero at a procedure exit). Branch/Jump/Switch targets
// describe execution separately. Only default error propagation is supported.
type Statement struct {
	ID                  StatementID
	Source              Source
	Retry, Next         StatementID
	Instructions        []Instruction
	End                 End
	Cleanup             []ValueID // Owned managed values, in acquisition order; release in reverse on every exit.
	Scope               ScopeID
	ErrorMode           int // 0 unchanged, 1 disable, 2 GoTo, 3 Resume Next, 4 reset active error.
	Handler             StatementID
	ReturnAddress       ValueID // GosubReturn consumes a successful OpGosubPop in this region.
	ResumeMode          int     // 0 retry, 1 next, 2 explicit label.
	Condition           ValueID // Branch condition defined in this region, of type Boolean.
	Target, FalseTarget StatementID
	Selector            ValueID      // Switch selector, an Integer/Long defined in this region.
	Cases               []SwitchCase // Unique integral values; Target is the default destination.
}

type SwitchCase struct {
	Value  int64
	Target StatementID
}

type End uint8

const (
	Continue End = iota
	Return       // Read the current ResultSlot, or return Void, wrapped in Ok.
	Jump
	Branch
	Switch // Semantic dispatch only; LLVM chooses comparisons, bit tests, or jump tables.
	Resume
	Gosub       // Push this region's Next continuation, then branch to Target without unwinding caller scopes.
	GosubReturn // Pop a continuation; unlike Return, this does not exit the procedure.
)

type Op uint8

const (
	OpInvalid Op = iota
	OpConstant
	OpLoad
	OpStore
	OpAddress
	OpConvert
	OpAdd
	OpSubtract
	OpMultiply
	OpNegate
	OpNot
	OpAnd
	OpOr
	OpXor
	OpEqual
	OpNotEqual
	OpLess
	OpLessEqual
	OpGreater
	OpGreaterEqual
	OpCall
	OpTry
	OpStringLiteral
	OpNullString
	OpConcat
	OpLen
	OpLenB
	OpChrW
	OpMid
	OpMidCheck
	OpMidAssign
	OpLoadManaged
	OpStoreManaged
	OpBox
	OpUnbox
	OpVariantConstant
	OpVariantRef
	OpPinString
	OpVarType
	OpIsEmpty
	OpIsNull
	OpIsMissing
	OpArrayResize
	OpArrayIndex
	OpElementRef
	OpReadReference
	OpWriteReference
	OpArrayBound
	OpArrayErase
	OpIsArray
	OpReleaseElement
	OpCondition
	OpCheckInitialized
	OpForValidate
	OpForTest
	OpEnumStart
	OpEnumNext
	OpCheckDispatch // Integer index must be in 0..255, otherwise error 5.
	OpDivide        // Checked floating division. Optional ref<Double> receives IEEE value on division-by-zero failure.
	OpFieldRef      // Typed alias into a UDT reference or an owned UDT value snapshot.
	OpIntegerDivide
	OpModulo
	// Reserved legacy library opcodes. New programs use OpLibraryCall.
	OpAscW
	OpStrComp
	OpInStr
	OpLeft
	OpStringRepeat
	OpSplit
	OpIIf
	OpKeepElement
	OpWithCapture
	OpWithReference
	OpWithCheck
	OpErrRaise
	OpErrClear
	OpErrGet
	OpErrSet
	OpObjectNew
	OpObjectNull
	OpObjectEqual
	OpCreateObject
	OpTestAssert
	OpObjectCheck
	OpCollectionInvoke
	OpEnumObjectNext
	OpGosubPush
	OpGosubPop
	OpParamArrayPack // ref<Variant()> destination followed by Variant values or references.
	OpClassNew
	OpClassCheck        // Require a non-Nothing instance with Class's exact concrete layout.
	OpClassCast         // Permit Nothing, otherwise require Class or its implemented interface.
	OpClassFieldRef     // Borrow storage from a checked, owned Object in this region.
	OpObjectAutoLoad    // Read an object reference, lazily constructing Class or Integer built-in kind.
	OpObjectInvoke      // Receiver plus Variant values/views, runtime argument binding.
	OpInvocationCapture // Retain an object or lock an array before later arguments execute.
	OpObjectValue       // Resolve an erased object's default value chain into a scalar Variant.
	OpLibraryCall       // Catalog member with a verified prepared-argument contract.
	OpInterfaceCall     // Invoke Callee's interface contract on the checked receiver.
	OpInterfaceCheck    // Require a non-Nothing instance supporting Class's interface.
)

func (o Op) String() string {
	if o == OpInterfaceCheck {
		return "interface_check"
	}
	if o == OpInterfaceCall {
		return "interface_call"
	}
	if o == OpLibraryCall {
		return "library_call"
	}
	if o == OpObjectAutoLoad {
		return "object_auto_load"
	}
	if o == OpObjectInvoke {
		return "object_invoke"
	}
	if o == OpInvocationCapture {
		return "invocation_capture"
	}
	if o == OpObjectValue {
		return "object_value"
	}
	if o >= OpClassNew && o <= OpClassFieldRef {
		return [...]string{"class_new", "class_check", "class_cast", "class_field_ref"}[o-OpClassNew]
	}
	if o == OpCollectionInvoke {
		return "collection_invoke"
	}
	if o == OpEnumObjectNext {
		return "object_enum_next"
	}
	if o == OpGosubPush {
		return "gosub_push"
	}
	if o == OpGosubPop {
		return "gosub_pop"
	}
	if o == OpParamArrayPack {
		return "paramarray_pack"
	}
	if o >= OpObjectNew && o <= OpObjectCheck {
		return [...]string{"object_new", "nothing", "object_is", "create_object", "test_assert", "object_check"}[o-OpObjectNew]
	}
	if o >= OpWithCheck && o <= OpErrSet {
		return [...]string{"with_check", "err_raise", "err_clear", "err_get", "err_set"}[o-OpWithCheck]
	}
	if o >= OpKeepElement && o <= OpWithReference {
		return [...]string{"keep_element", "with_capture", "with_reference"}[o-OpKeepElement]
	}
	if o >= OpIntegerDivide && o <= OpIIf {
		return [...]string{"integer_divide", "modulo", "ascw", "strcomp", "instr", "left", "string_repeat", "split", "iif"}[o-OpIntegerDivide]
	}
	if o == OpDivide {
		return "checked_divide"
	}
	if o == OpFieldRef {
		return "field_ref"
	}
	names := [...]string{"invalid", "constant", "load", "store", "address", "convert", "checked_add", "checked_subtract", "checked_multiply", "checked_negate", "not", "and", "or", "xor", "equal", "not_equal", "less", "less_equal", "greater", "greater_equal", "call", "try", "string", "null_string", "concat", "len", "lenb", "chrw", "mid", "mid_check", "mid_assign", "managed_load", "managed_store", "box", "unbox", "variant_constant", "variant_ref", "pin_string", "vartype", "is_empty", "is_null", "is_missing", "array_resize", "array_index", "element_ref", "read_reference", "write_reference", "array_bound", "array_erase", "is_array", "release_element", "condition", "check_initialized", "for_validate", "for_test", "enum_start", "enum_next", "check_dispatch"}
	if int(o) < len(names) {
		return names[o]
	}
	return fmt.Sprintf("op(%d)", o)
}

type ErrorContinuation uint8

const (
	NoErrorContinuation ErrorContinuation = iota
	Propagate                             // Deliver to activation policy, or propagate; preserve completed stores/calls.
)

// Instructions evaluate in slice order. IDs name typed values within one region;
// no value may escape a region except through storage. A fallible instruction
// produces Result<T> and must be followed immediately by OpTry. Try's success
// continuation binds ID to T and executes the rest of this region. Its error
// continuation propagates the original error, skipping all pending instructions.
// Thus no extraction can run on the error alternative. Result<Void> calls also
// require Try. Stores have no result (ID zero), and occur only after successful Try.
// Region terminators provide the CFG; values cross region edges only in slots.
type Instruction struct {
	Library    stdlib.MemberID // Only OpLibraryCall may carry a catalog identity.
	Class      ClassID         // Nominal descriptor for class operations.
	Member     string          // Runtime member name, empty means the default member.
	Invocation InvocationKind
	ID         ValueID
	Op         Op
	Type       Type
	Args       []ValueID // Calls use parameter order, after source-order evaluation.
	Slot       SlotID
	Callee     ProcedureID
	Integer    int64   // Typed literal, including Boolean's 0/-1 representation.
	Float      float64 // OpConstant with Single/Double type; Single is already rounded to binary32.
	Effects    Effects
	OnError    ErrorContinuation // Only OpTry may carry an error continuation.
	Source     Source
	Names      []string // OpTestAssert: explicit argument names, empty for positional.
	Units      []uint16 // OpStringLiteral, preserving embedded NUL and surrogate units.
	Ownership  Ownership
	Field      uint32 // OpFieldRef: zero-based field index in the receiver's nominal definition.
}

type Ownership uint8

const (
	NoOwnership Ownership = iota // Unmanaged values, references, and unextracted Results.
	Immortal                     // String literals/null strings or Empty/Null/Missing; no cleanup required.
	Owned                        // An independent managed snapshot; listed in its region's Cleanup.
)

// Entry/exit ownership is part of the semantic contract. Every non-ByRef managed
// slot owns its value. ByVal parameters acquire a copy; result slots transfer
// ownership on successful return. Error exits release owned slots and temporaries,
// preserving caller-owned ByRef storage and the original error. String stores
// acquire their new value before releasing the old one, including self-assignment.
func EntryEffects(p *Procedure) Effects {
	var effects Effects
	if UsesGosub(p) {
		effects |= ManagesLifetime
	}
	for _, s := range p.Slots {
		if Managed(s.Type) && !s.ByRef && s.Kind != GlobalAlias {
			effects |= ManagesLifetime
			if s.Type.Scalar == Record && s.Kind != Temporary && s.Kind != LoopState {
				effects |= MayRaise
			}
			if s.Kind == Parameter && (s.Type.Scalar == Variant || Array(s.Type)) {
				effects |= MayRaise
			}
		}
	}
	return effects
}

func Managed(t Type) bool {
	return t.Form == Value && (t.Scalar == Object || t.Scalar == String || t.Scalar == Variant || Array(t) || Element(t) || t.Scalar == Enumerator || t.Scalar == Record)
}

// Callback effects are conservative across erased native object references.
func TerminationEffects(program *Program) Effects {
	var effects Effects
	for _, class := range program.Classes {
		if id := class.Terminate; id != 0 && int(id) <= len(program.Procedures) && program.Procedures[id-1] != nil {
			effects |= program.Procedures[id-1].Effects &^ (MayRaise | WritesAmbientErr)
		}
	}
	return effects
}
func InitializationEffects(program *Program, id ClassID) Effects {
	if id == 0 || int(id) > len(program.Classes) {
		return 0
	}
	p := program.Classes[id-1].Initialize
	if p != 0 && int(p) <= len(program.Procedures) && program.Procedures[p-1] != nil {
		return program.Procedures[p-1].Effects
	}
	return 0
}
func InvocationEffects(program *Program) Effects {
	var effects Effects
	for _, p := range program.Procedures {
		if p != nil && p.Invocation != nil {
			effects |= p.Effects
			for _, a := range p.Invocation.Parameters {
				if a.Default != 0 && int(a.Default) <= len(program.Procedures) && program.Procedures[a.Default-1] != nil {
					effects |= program.Procedures[a.Default-1].Effects
				}
			}
		}
	}
	for _, class := range program.Classes {
		for _, field := range class.Fields {
			if field.Public {
				effects |= InitializationEffects(program, field.AutoClass)
			}
		}
	}
	return effects
}

func DefaultInvocationEffects(program *Program, in Instruction) Effects {
	if MayInvokeDefault(in) {
		return InvocationEffects(program)
	}
	return 0
}
func MayInvokeDefault(in Instruction) bool {
	if in.Effects&MayRaise == 0 {
		return false
	}
	switch in.Op {
	case OpObjectInvoke, OpObjectValue, OpUnbox, OpCondition, OpForValidate, OpForTest, OpLen, OpLenB, OpLibraryCall:
		return true
	case OpAdd, OpSubtract, OpMultiply, OpDivide, OpIntegerDivide, OpModulo, OpConcat, OpNegate, OpEqual, OpNotEqual, OpLess, OpLessEqual, OpGreater, OpGreaterEqual:
		if in.Type.Scalar == Variant {
			return true
		}
	}
	return false
}

func Array(t Type) bool {
	return t.Scalar >= ArrayBoolean && t.Scalar <= ArrayVariant || t.Scalar == ArrayRecord || t.Scalar == ArrayDate || t.Scalar == ArrayCurrency || t.Scalar == ArrayObject
}
func ScalarValue(t Type) bool {
	return t.Scalar >= Boolean && t.Scalar <= Variant || t.Scalar == Date || t.Scalar == Currency
}
func Floating(t Type) bool { return t.Scalar == Single || t.Scalar == Double }
func Element(t Type) bool {
	return t.Scalar >= ElementBoolean && t.Scalar <= ElementVariant || t.Scalar == ElementRecord || t.Scalar == ElementDate || t.Scalar == ElementCurrency || t.Scalar == ElementObject
}
func ArrayOf(t Type) Type {
	if t.Scalar == Object {
		return Type{Scalar: ArrayObject}
	}
	if t.Scalar == Date {
		return Type{Scalar: ArrayDate}
	}
	if t.Scalar == Currency {
		return Type{Scalar: ArrayCurrency}
	}
	if t.Scalar == Record {
		return Type{Scalar: ArrayRecord, Record: t.Record}
	}
	return Type{Scalar: ArrayBoolean + t.Scalar - Boolean}
}
func ElementOf(t Type) Type {
	if t.Scalar == Object {
		return Type{Scalar: ElementObject}
	}
	if t.Scalar == Date {
		return Type{Scalar: ElementDate}
	}
	if t.Scalar == Currency {
		return Type{Scalar: ElementCurrency}
	}
	if t.Scalar == Record {
		return Type{Scalar: ElementRecord, Record: t.Record}
	}
	return Type{Scalar: ElementBoolean + t.Scalar - Boolean}
}
func ElementType(t Type) Type {
	if t.Scalar == ArrayObject || t.Scalar == ElementObject {
		return Type{Scalar: Object}
	}
	if t.Scalar == ArrayDate || t.Scalar == ElementDate {
		return Type{Scalar: Date}
	}
	if t.Scalar == ArrayCurrency || t.Scalar == ElementCurrency {
		return Type{Scalar: Currency}
	}
	if t.Scalar == ArrayRecord || t.Scalar == ElementRecord {
		return Type{Scalar: Record, Record: t.Record}
	}
	if Array(t) {
		return Type{Scalar: Boolean + t.Scalar - ArrayBoolean}
	}
	if Element(t) {
		return Type{Scalar: Boolean + t.Scalar - ElementBoolean}
	}
	return Type{}
}

// GlobalInitializationEffects describes implicit project setup before the first
// procedure body. Scalar/dynamic-array globals already have static zero values.
func GlobalInitializationEffects(globals []Global) Effects {
	for _, g := range globals {
		if len(g.Bounds) > 0 || g.Type.Scalar == Record {
			return MayRaise | WritesStorage | ManagesLifetime
		}
	}
	return 0
}

// UsesErrors enables the project-wide Err channel while handler policy remains
// activation-local. A failed Try updates Err only at its original source.
func UsesErrors(program *Program) bool {
	if program == nil {
		return false
	}
	for _, p := range program.Procedures {
		if p == nil {
			continue
		}
		for _, s := range p.Statements {
			if s == nil {
				continue
			}
			if s.ErrorMode != 0 || s.End == Resume {
				return true
			}
			for _, in := range s.Instructions {
				if in.Op == OpErrRaise || in.Op == OpErrClear || in.Op == OpErrGet || in.Op == OpErrSet {
					return true
				}
			}
		}
	}
	return false
}
func ErrorPolicyEffects(s *Statement) Effects {
	var e Effects
	if s.ErrorMode != 0 {
		e |= WritesAmbientErr | WritesStorage | ManagesLifetime
	}
	if s.End == Return && s.Source.Node != nil && s.Source.Node.Kind == ast.KindExitStatement {
		e |= WritesAmbientErr | ManagesLifetime
	}
	if s.End == Resume {
		e |= WritesAmbientErr | MayRaise | ManagesLifetime
	}
	return e
}

// UsesGosub enables activation-owned resumption storage, independently of Err.
func UsesGosub(p *Procedure) bool {
	if p == nil {
		return false
	}
	for _, s := range p.Statements {
		if s == nil {
			continue
		}
		for _, in := range s.Instructions {
			if in.Op == OpGosubPush || in.Op == OpGosubPop {
				return true
			}
		}
	}
	return false
}

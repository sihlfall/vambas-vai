package checker

import "sihlfall.ch/vambas-vai/ast"

type CallKind uint8

const (
	CallProcedure CallKind = iota
	CallPropertyGet
	CallPropertyLet
	CallPropertySet
	CallArrayIndex
	CallArrayReference
	CallDynamic
	CallInvalid
)

// CallResolution describes an operation without rewriting its syntax. Arguments
// retain source order and parameter indices; this is not an execution schedule.
// Valid does not imply infallibility. Deferred requires further semantic work.
type CallResolution struct {
	Kind          CallKind
	Node          *ast.Node
	Target        *ast.Node
	Receiver      *ast.Node
	WithTarget    *ast.Node // Captured With receiver; never reevaluate its header.
	Signature     *Signature
	DefaultMember *ast.Symbol
	ResultType    *Type
	Arguments     []*ArgumentBinding
	ParamArray    *Parameter // May be present with no packed elements.
	Valid         bool
	Deferred      bool
	Prefix        *CallResolution // E.g. F(i): invoke parameterless F, then index its result.
}

type ArgumentMode uint8

const (
	ArgumentValue ArgumentMode = iota
	ArgumentReference
	ArgumentTemporary
	ArgumentCopyBack // Object type adaptation; copy back only on normal return.
	ArgumentDefault
	ArgumentMissing // Missing Variant or an omitted ParamArray element.
	ArgumentDeferred
	ArgumentInvalid
)

type ArgumentBinding struct {
	Argument            *ast.Node // Original argument (including NamedArgument), or nil.
	Expression          *ast.Node
	Parameter           *Parameter
	ParameterIndex      int
	SourceIndex         int // -1 for an implicit default, or a setter's RHS.
	ParamArrayIndex     int // -1 outside a ParamArray.
	Mode                ArgumentMode
	Conversion          *Conversion
	Storage             *Storage
	CopyBack            *Conversion
	Default             *ast.Node
	StringValue         *string // Dictionary syntax obj!Name supplies a String, not a name reference.
	PreserveStorageType bool    // A Variant ByRef view of typed caller storage.
	ForcedValue         bool    // Includes a single parenthesized implicit statement argument.
}

type StorageKind uint8

const (
	StorageValue StorageKind = iota // A value/constant, not assignable storage.
	StorageVariable
	StorageArrayElement
	StorageProperty
	StorageDynamic
	StorageInvalid
)

type Storage struct {
	Kind        StorageKind
	Node        *ast.Node
	Type        *Type
	Symbol      *ast.Symbol
	Receiver    *ast.Node
	WithTarget  *ast.Node
	Indices     []*ast.Node
	Writable    bool
	Addressable bool // Properties are evaluated into temporaries for ByRef arguments.
	Resolution  *CallResolution
}

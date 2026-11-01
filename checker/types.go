// Package checker resolves semantic types without adding checked data to the AST.
package checker

import "sihlfall.ch/vambas-vai/ast"

type TypeKind uint8

const (
	TypeUnknown TypeKind = iota // Deferred semantics, never a synonym for Variant.
	TypeError                   // Invalid/recovered source; suppress cascading errors.
	TypeVoid
	TypeVariant
	TypeBoolean
	TypeByte
	TypeInteger
	TypeLong
	TypeLongLong
	TypeLongPtr // Target-dependent; arithmetic involving it stays deferred for now.
	TypeSingle
	TypeDouble
	TypeCurrency
	TypeDate
	TypeString
	TypeObject
	TypeClass
	TypeArray
	TypeProcedure
	TypeModule
	TypeStructure // A nominal VBA UDT value, distinct from a class reference.
)

// Types and signatures belong to one Checker and are read-only after construction.
// These are language types, not LLVM layouts or Variant runtime subtype tags.
type Type struct {
	Kind        TypeKind
	Symbol      *ast.Symbol // Identity of a class, procedure, or module.
	ElementType *Type
	Rank        int // Zero means unspecified rank, not a scalar.
	Signatures  []*Signature
}

func (t *Type) String() string {
	if t == nil {
		return "unknown"
	}
	switch t.Kind {
	case TypeClass, TypeModule, TypeProcedure, TypeStructure:
		if t.Symbol != nil {
			return t.Symbol.Name
		}
	case TypeArray:
		return t.ElementType.String() + "()"
	}
	names := [...]string{"unknown", "error", "Void", "Variant", "Boolean", "Byte", "Integer", "Long", "LongLong", "LongPtr", "Single", "Double", "Currency", "Date", "String", "Object", "class", "array", "procedure", "module"}
	if int(t.Kind) < len(names) {
		return names[t.Kind]
	}
	return "unknown"
}

type Parameter struct {
	Declaration *ast.Node
	Symbol      *ast.Symbol
	Name        string
	Type        *Type
	Modifiers   ast.ParameterModifiers
	Default     *ast.Node
}

func (p *Parameter) IsByRef() bool {
	return p.Modifiers&(ast.ParameterByVal|ast.ParameterParamArray) == 0
}

type Signature struct {
	Declaration *ast.Node
	Parameters  []*Parameter
	ReturnType  *Type
}

type ConversionKind uint8

const (
	ConversionIdentity ConversionKind = iota
	ConversionLet                     // A VBA Let-coercion, possibly fallible at runtime.
	ConversionSet                     // A VBA object-reference coercion, possibly fallible.
	ConversionDeferred
	ConversionInvalid
)

// Conversion makes required coercions explicit for later semantic IR lowering.
// A valid conversion is not a proof of infallibility or of constant evaluability.
type Conversion struct {
	Kind     ConversionKind
	From, To *Type
}

type DiagnosticCode = ast.DiagnosticCode

const (
	DiagnosticUnknownType         DiagnosticCode = "unknown-type"
	DiagnosticTypeMismatch        DiagnosticCode = "type-mismatch"
	DiagnosticInvalidOperand      DiagnosticCode = "invalid-operand"
	DiagnosticConflictingType     DiagnosticCode = "conflicting-type"
	DiagnosticCircularConstant    DiagnosticCode = "circular-constant"
	DiagnosticInvalidNew          DiagnosticCode = "invalid-new"
	DiagnosticInvalidLiteral      DiagnosticCode = "invalid-literal"
	DiagnosticInvalidConstantType DiagnosticCode = "invalid-constant-type"
	DiagnosticConstantValue       DiagnosticCode = "constant-value"
	DiagnosticArgumentCount       DiagnosticCode = "argument-count"
	DiagnosticUnknownArgument     DiagnosticCode = "unknown-argument"
	DiagnosticDuplicateArgument   DiagnosticCode = "duplicate-argument"
	DiagnosticArgumentType        DiagnosticCode = "argument-type"
	DiagnosticByRefType           DiagnosticCode = "byref-type-mismatch"
	DiagnosticInvalidArgument     DiagnosticCode = "invalid-call-argument"
	DiagnosticNotCallable         DiagnosticCode = "not-callable"
	DiagnosticInvalidStorage      DiagnosticCode = "invalid-storage"
	DiagnosticDefaultMember       DiagnosticCode = "default-member"
	DiagnosticMember              DiagnosticCode = "member-resolution"
	DiagnosticEnumValue           DiagnosticCode = "invalid-enum-value"
	DiagnosticRecursiveType       DiagnosticCode = "recursive-type"
	DiagnosticReDim               DiagnosticCode = "invalid-redim"
	DiagnosticImplements          DiagnosticCode = "invalid-implements"
	DiagnosticInterfaceSignature  DiagnosticCode = "interface-signature-mismatch"
)

type Diagnostic struct {
	ast.Diagnostic
	Node *ast.Node
}

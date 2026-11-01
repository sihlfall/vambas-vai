// Package stdlib describes ambient VBA declarations without depending on the
// syntax tree, checker, IR, or native runtime. Additional families can migrate
// independently while preserving their existing declaration contracts.
package stdlib

import "slices"

// MemberID is a declaration identity, not an opcode or an index into the catalog.
// Keep assigned values stable when declarations are reordered or added.
type MemberID uint16

const (
	None               MemberID = 0
	AscW               MemberID = 1
	StrComp            MemberID = 2
	InStr              MemberID = 3
	Left               MemberID = 4
	LeftString         MemberID = 5
	StringRepeat       MemberID = 6
	StringRepeatString MemberID = 7
	Split              MemberID = 8
	IIf                MemberID = 9
	BinaryCompare      MemberID = 10
	Len                MemberID = 11
	LenB               MemberID = 12
	ChrW               MemberID = 13
	Mid                MemberID = 14
	MidString          MemberID = 15
	VarType            MemberID = 16
	IsEmpty            MemberID = 17
	IsNull             MemberID = 18
	CVar               MemberID = 19
	CInt               MemberID = 20
	CLng               MemberID = 21
	CBool              MemberID = 22
	CStr               MemberID = 23
	CSng               MemberID = 24
	CDbl               MemberID = 25
	IsMissing          MemberID = 26
	IsArray            MemberID = 27
	LBound             MemberID = 28
	UBound             MemberID = 29
	ObjectError        MemberID = 30
	NullString         MemberID = 31
	NullChar           MemberID = 32
	CrLf               MemberID = 33
	Cr                 MemberID = 34
	Lf                 MemberID = 35
	Tab                MemberID = 36
	Back               MemberID = 37
	FormFeed           MemberID = 38
	VerticalTab        MemberID = 39
	Collection         MemberID = 40
	CollectionAdd      MemberID = 41
	CollectionItem     MemberID = 42
	CollectionRemove   MemberID = 43
	CollectionCount    MemberID = 44
	Err                MemberID = 45
	ErrRaise           MemberID = 46
	ErrClear           MemberID = 47
	ErrNumber          MemberID = 48
	ErrDescription     MemberID = 49
	ErrSource          MemberID = 50
	ErrHelpFile        MemberID = 51
	ErrHelpContext     MemberID = 52
	CreateObject       MemberID = 53
	CByte              MemberID = 54
	CVErr              MemberID = 55
	IsError            MemberID = 56
	ByteType           MemberID = 57
	ErrorType          MemberID = 58
	CDate              MemberID = 59
	CVDate             MemberID = 60
	CCur               MemberID = 61
	CDec               MemberID = 62
	DateType           MemberID = 63
	CurrencyType       MemberID = 64
	DecimalType        MemberID = 65
)

// TypeKind describes a language type, independently of its native layout.
// This initial set covers only the declarations migrated into the catalog.
type TypeKind uint8

const (
	TypeInvalid TypeKind = iota
	TypeVariant
	TypeInteger
	TypeLong
	TypeString
	TypeBoolean
	TypeSingle
	TypeDouble
	TypeObject
	TypeByte
	TypeDate
	TypeCurrency
)

type Type struct {
	Kind  TypeKind
	Array bool
}

type LiteralKind uint8

const (
	NoLiteral LiteralKind = iota
	NumericLiteral
	StringLiteral
	NullStringLiteral // Null string pointer; distinct from an ordinary empty String.
)

// Literal stores declaration data, never an AST node shared between projects.
type Literal struct {
	Kind LiteralKind
	Text string
}

// An Optional Variant with NoLiteral retains Missing rather than an explicit
// Empty value. ByRef is explicit; the zero value describes a ByVal parameter.
type Parameter struct {
	Name     string
	Type     Type
	Optional bool
	ByRef    bool
	Default  Literal
}

// CallKind selects a semantic argument-preparation contract, not a native ABI.
type CallKind uint8

const (
	SpecializedCall CallKind = iota
	SnapshotCall             // Prepared Variant values in formal order; exact declared result.
)

type Function struct {
	ID         MemberID
	Module     string
	Name       string
	Result     Type
	Parameters []Parameter
	Section    string // Section in the pinned MS-VBAL revision used by the inventory.
	Call       CallKind
	// Language built-ins are tracked separately from the section 6 library
	// inventory. They have no procedural Module or section 6 Section.
	LanguageBuiltin bool
}

type Constant struct {
	ID      MemberID
	Owner   string // The defining enum or procedural module.
	Name    string
	Type    Type
	Value   Literal
	Section string
}

type Module struct {
	Name    string
	Section string
}

type Enum struct {
	Name    string
	Section string
}

// Enums returns fresh descriptors for the predefined enum namespaces/types.
// Members remain in Constants, with their defining enum recorded as Owner.
func Enums() []Enum { return slices.Clone(enums[:]) }

// MemberKind preserves the current synthetic declaration forms. In particular,
// Err state uses writable variables, while Collection exposes read-only getters.
// These forms do not imply that every inventory property/accessor is supported.
type MemberKind uint8

const (
	SubMember MemberKind = iota
	GetMember
	VariableMember
)

type ClassMember struct {
	ID         MemberID
	Name       string
	Kind       MemberKind
	Type       Type // Result or variable type; unused for SubMember.
	Parameters []Parameter
	Section    string
}

type Class struct {
	ID            MemberID
	Name          string
	Section       string
	Members       []ClassMember
	DefaultMember MemberID
	// Singleton retains Err's existing namespace/value declaration model rather
	// than introducing construction or a source-visible instance type.
	Singleton bool
}

// Classes returns independent class, member, and parameter descriptors.
func Classes() []Class {
	result := slices.Clone(classes[:])
	for i := range result {
		result[i].Members = slices.Clone(result[i].Members)
		for j := range result[i].Members {
			result[i].Members[j].Parameters = slices.Clone(result[i].Members[j].Parameters)
		}
	}
	return result
}

// Modules returns independent procedural-module descriptors. Enum owners and
// language built-ins are deliberately separate from procedural modules.
func Modules() []Module { return slices.Clone(modules[:]) }

// Functions returns independent descriptors, including their parameter slices.
// Callers may adapt them without changing the catalog seen by other Programs.
func Functions() []Function {
	result := append([]Function(nil), stringFunctions[:]...)
	result = append(result, interactionFunctions[:]...)
	result = append(result, conversionFunctions[:]...)
	result = append(result, informationFunctions[:]...)
	result = append(result, languageFunctions[:]...)
	for i := range result {
		result[i].Parameters = slices.Clone(result[i].Parameters)
	}
	return result
}

func Constants() []Constant {
	result := slices.Clone(constants[:])
	return append(result, inventoryConstants[:]...)
}

func variant(name string) Parameter {
	return Parameter{Name: name, Type: Type{Kind: TypeVariant}}
}

func long(name string) Parameter {
	return Parameter{Name: name, Type: Type{Kind: TypeLong}}
}

func optional(name string, value Literal) Parameter {
	return Parameter{Name: name, Type: Type{Kind: TypeVariant}, Optional: true, Default: value}
}

func number(text string) Literal { return Literal{Kind: NumericLiteral, Text: text} }
func text(value string) Literal  { return Literal{Kind: StringLiteral, Text: value} }

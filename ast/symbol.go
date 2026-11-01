package ast

import (
	"strings"

	"sihlfall.ch/vambas-vai/stdlib"
)

// Symbol

type Symbol struct {
	Flags SymbolFlags
	//	CheckFlags                   CheckFlags // Non-zero only in transient symbols created by Checker
	Name             string
	Declarations     []*Node
	ValueDeclaration *Node
	Members          SymbolTable
	DefaultMember    *Symbol // Default member supplied by an ambient declaration library.
	Exports          SymbolTable
	//	id                           atomic.Uint64
	Parent    *Symbol
	Intrinsic IntrinsicKind   // Identity of an unmigrated ambient declaration; catalog declarations use Library.
	Library   stdlib.MemberID // Catalog identity; zero for user and not-yet-migrated declarations.
	// ExportSymbol                 *Symbol
	// AssignmentDeclarationMembers collections.Set[*Node] // Set of detected assignment declarations
	// GlobalExports                SymbolTable            // Conditional global UMD exports
}

type IntrinsicKind uint8

const (
	IntrinsicNone IntrinsicKind = iota
	// Catalog-backed declarations use Symbol.Library. Keep their old enum
	// values reserved so the separate testing-library identities retain their numbers.
	IntrinsicLen
	IntrinsicLenB
	IntrinsicChrW
	IntrinsicNullString
	IntrinsicMid
	IntrinsicMidString
	IntrinsicVarType
	IntrinsicIsEmpty
	IntrinsicIsNull
	IntrinsicCVar
	IntrinsicCInt
	IntrinsicCLng
	IntrinsicCBool
	IntrinsicCStr
	IntrinsicIsMissing
	IntrinsicIsArray
	IntrinsicLBound
	IntrinsicUBound
	IntrinsicCSng
	IntrinsicCDbl
	IntrinsicAscW
	IntrinsicStrComp
	IntrinsicInStr
	IntrinsicLeft
	IntrinsicLeftString
	IntrinsicString
	IntrinsicStringString
	IntrinsicSplit
	IntrinsicIIf
	IntrinsicBinaryCompare
	IntrinsicErrRaise
	IntrinsicErrClear
	IntrinsicErrNumber
	IntrinsicErrDescription
	IntrinsicErrSource
	IntrinsicErrHelpFile
	IntrinsicErrHelpContext
	IntrinsicErrObject
	IntrinsicCreateObject
	IntrinsicAssertClass
	IntrinsicPermissiveAssertClass
	IntrinsicFakesProvider
	IntrinsicTestAssert
	IntrinsicCollection
	IntrinsicCollectionAdd
	IntrinsicCollectionItem
	IntrinsicCollectionRemove
	IntrinsicCollectionCount
)

type SymbolTable map[string]*Symbol

// SymbolNameKey normalizes the ASCII identifiers currently supported by the
// scanner. Declaration type characters are not part of a VBA name's identity.
func SymbolNameKey(name string) string {
	if len(name) != 0 && strings.ContainsRune("%&^!#@$", rune(name[len(name)-1])) {
		name = name[:len(name)-1]
	}
	return strings.ToLower(name)
}

func (table SymbolTable) Lookup(name string) *Symbol {
	// VBA library functions may have distinct Variant and String ($) forms.
	// Ordinary declarations still use canonical keys without type characters.
	if strings.HasSuffix(name, "$") {
		if symbol := table[strings.ToLower(name)]; symbol != nil {
			return symbol
		}
	}
	return table[SymbolNameKey(name)]
}

/*
const InternalSymbolNamePrefix = "\xFE" // Invalid UTF8 sequence, will never occur as IdentifierName

const (
	InternalSymbolNameCall                    = InternalSymbolNamePrefix + "call"                    // Call signatures
	InternalSymbolNameConstructor             = InternalSymbolNamePrefix + "constructor"             // Constructor implementations
	InternalSymbolNameNew                     = InternalSymbolNamePrefix + "new"                     // Constructor signatures
	InternalSymbolNameIndex                   = InternalSymbolNamePrefix + "index"                   // Index signatures
	InternalSymbolNameExportStar              = InternalSymbolNamePrefix + "export"                  // Module export * declarations
	InternalSymbolNameGlobal                  = InternalSymbolNamePrefix + "global"                  // Global self-reference
	InternalSymbolNameMissing                 = InternalSymbolNamePrefix + "missing"                 // Indicates missing symbol
	InternalSymbolNameType                    = InternalSymbolNamePrefix + "type"                    // Anonymous type literal symbol
	InternalSymbolNameObject                  = InternalSymbolNamePrefix + "object"                  // Anonymous object literal declaration
	InternalSymbolNameJSXAttributes           = InternalSymbolNamePrefix + "jsxAttributes"           // Anonymous JSX attributes object literal declaration
	InternalSymbolNameClass                   = InternalSymbolNamePrefix + "class"                   // Unnamed class expression
	InternalSymbolNameFunction                = InternalSymbolNamePrefix + "function"                // Unnamed function expression
	InternalSymbolNameComputed                = InternalSymbolNamePrefix + "computed"                // Computed property name declaration with dynamic name
	InternalSymbolNameResolving               = InternalSymbolNamePrefix + "resolving"               // Indicator symbol used to mark partially resolved type aliases
	InternalSymbolNameInstantiationExpression = InternalSymbolNamePrefix + "instantiationExpression" // Instantiation expressions
	InternalSymbolNameImportAttributes        = InternalSymbolNamePrefix + "importAttributes"
	InternalSymbolNameExportEquals            = "export=" // Export assignment symbol
	InternalSymbolNameDefault                 = "default" // Default export symbol (technically not wholly internal, but included here for usability)
	InternalSymbolNameThis                    = "this"
	InternalSymbolNameModuleExports           = "module.exports"
)

func SymbolName(symbol *Symbol) string {
	if symbol.ValueDeclaration != nil && IsPrivateIdentifierClassElementDeclaration(symbol.ValueDeclaration) {
		return symbol.ValueDeclaration.Name().Text()
	}
	return symbol.Name
}
*/

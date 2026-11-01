package ast

type SymbolFlags uint32

const (
	SymbolFlagsNone SymbolFlags = 0
	// Keep the shared flags at the same bit positions as typescript-go.
	SymbolFlagsFunctionScopedVariable SymbolFlags = 1 << 0
	SymbolFlagsProperty               SymbolFlags = 1 << 2
	SymbolFlagsEnumMember             SymbolFlags = 1 << 3
	SymbolFlagsFunction               SymbolFlags = 1 << 4
	SymbolFlagsClass                  SymbolFlags = 1 << 5
	SymbolFlagsRegularEnum            SymbolFlags = 1 << 8
	SymbolFlagsValueModule            SymbolFlags = 1 << 9
	SymbolFlagsGetAccessor            SymbolFlags = 1 << 15
	SymbolFlagsSetAccessor            SymbolFlags = 1 << 16
	// VBA records occupy the type-declaration bit used for TypeScript aliases;
	// their semantics are nominal value types, not aliases or classes.
	SymbolFlagsUserDefinedType SymbolFlags = 1 << 19
	SymbolFlagsType                        = SymbolFlagsClass | SymbolFlagsRegularEnum | SymbolFlagsUserDefinedType
	// VBA separates value assignment (Let) from object assignment (Set).
	SymbolFlagsLetAccessor SymbolFlags = 1 << 30
	SymbolFlagsAccessor                = SymbolFlagsGetAccessor | SymbolFlagsLetAccessor | SymbolFlagsSetAccessor
	// VBA labels have symbols in a separate namespace.
	SymbolFlagsLabel SymbolFlags = 1 << 31
)

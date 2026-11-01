package stdlib

// These existing ambient declarations represent language built-ins outside the
// supplied section 6 inventory. Preserve their current Variant ByRef array view
// and typed optional dimension; execution is documented in llvm/ARRAYS.md.
var languageFunctions = [...]Function{
	{ID: LBound, Name: "LBound", Result: Type{Kind: TypeLong}, LanguageBuiltin: true,
		Parameters: []Parameter{
			{Name: "ArrayName", Type: Type{Kind: TypeVariant}, ByRef: true},
			{Name: "Dimension", Type: Type{Kind: TypeLong}, Optional: true, Default: number("1")},
		}},
	{ID: UBound, Name: "UBound", Result: Type{Kind: TypeLong}, LanguageBuiltin: true,
		Parameters: []Parameter{
			{Name: "ArrayName", Type: Type{Kind: TypeVariant}, ByRef: true},
			{Name: "Dimension", Type: Type{Kind: TypeLong}, Optional: true, Default: number("1")},
		}},
}

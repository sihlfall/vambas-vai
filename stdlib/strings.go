package stdlib

// Preserve the existing declaration contracts during migration. Module metadata
// selects the canonical owner; root exports alias the same declaration symbols.
var stringFunctions = [...]Function{
	{ID: AscW, Call: SnapshotCall, Module: "Strings", Name: "AscW", Result: Type{Kind: TypeInteger},
		Parameters: []Parameter{variant("String")}, Section: "6.1.2.11"},
	{ID: StrComp, Call: SnapshotCall, Module: "Strings", Name: "StrComp", Result: Type{Kind: TypeVariant},
		Parameters: []Parameter{variant("String1"), variant("String2"), optional("Compare", number("0"))}, Section: "6.1.2.11"},
	{ID: InStr, Call: SnapshotCall, Module: "Strings", Name: "InStr", Result: Type{Kind: TypeVariant},
		Parameters: []Parameter{optional("Start", number("1")), variant("String1"), variant("String2"), optional("Compare", number("0"))}, Section: "6.1.2.11"},
	{ID: Left, Call: SnapshotCall, Module: "Strings", Name: "Left", Result: Type{Kind: TypeVariant},
		Parameters: []Parameter{variant("String"), long("Length")}, Section: "6.1.2.11"},
	{ID: LeftString, Call: SnapshotCall, Module: "Strings", Name: "Left$", Result: Type{Kind: TypeString},
		Parameters: []Parameter{variant("String"), long("Length")}, Section: "6.1.2.11"},
	{ID: StringRepeat, Call: SnapshotCall, Module: "Strings", Name: "String", Result: Type{Kind: TypeVariant},
		Parameters: []Parameter{variant("Number"), variant("Character")}, Section: "6.1.2.11"},
	{ID: StringRepeatString, Call: SnapshotCall, Module: "Strings", Name: "String$", Result: Type{Kind: TypeString},
		Parameters: []Parameter{variant("Number"), variant("Character")}, Section: "6.1.2.11"},
	{ID: Split, Call: SnapshotCall, Module: "Strings", Name: "Split", Result: Type{Kind: TypeString, Array: true},
		Parameters: []Parameter{variant("Expression"), optional("Delimiter", text(" ")), optional("Limit", number("-1")), optional("Compare", number("0"))}, Section: "6.1.2.11"},
	{ID: Len, Module: "Strings", Name: "Len", Result: Type{Kind: TypeVariant},
		Parameters: []Parameter{variant("Expression")}, Section: "6.1.2.11"},
	{ID: LenB, Module: "Strings", Name: "LenB", Result: Type{Kind: TypeVariant},
		Parameters: []Parameter{variant("Expression")}, Section: "6.1.2.11"},
	{ID: ChrW, Module: "Strings", Name: "ChrW", Result: Type{Kind: TypeString},
		Parameters: []Parameter{long("CharCode")}, Section: "6.1.2.11"},
	// Omitted Length remains Missing, unlike Split's explicit optional defaults.
	{ID: Mid, Module: "Strings", Name: "Mid", Result: Type{Kind: TypeVariant},
		Parameters: []Parameter{variant("String"), long("Start"), {Name: "Length", Type: Type{Kind: TypeVariant}, Optional: true}}, Section: "6.1.2.11"},
	{ID: MidString, Module: "Strings", Name: "Mid$", Result: Type{Kind: TypeString},
		Parameters: []Parameter{variant("String"), long("Start"), {Name: "Length", Type: Type{Kind: TypeVariant}, Optional: true}}, Section: "6.1.2.11"},
}

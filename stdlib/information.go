package stdlib

var informationFunctions = [...]Function{
	{ID: IsError, Call: SnapshotCall, Module: "Information", Name: "IsError", Result: Type{Kind: TypeBoolean},
		Parameters: []Parameter{variant("Arg")}, Section: "6.1.2.7.1.5"},
	{ID: VarType, Module: "Information", Name: "VarType", Result: Type{Kind: TypeInteger},
		Parameters: []Parameter{variant("VarName")}, Section: "6.1.2.7"},
	{ID: IsEmpty, Module: "Information", Name: "IsEmpty", Result: Type{Kind: TypeBoolean},
		Parameters: []Parameter{variant("Expression")}, Section: "6.1.2.7"},
	{ID: IsNull, Module: "Information", Name: "IsNull", Result: Type{Kind: TypeBoolean},
		Parameters: []Parameter{variant("Expression")}, Section: "6.1.2.7"},
	{ID: IsMissing, Module: "Information", Name: "IsMissing", Result: Type{Kind: TypeBoolean},
		Parameters: []Parameter{variant("Expression")}, Section: "6.1.2.7"},
	{ID: IsArray, Module: "Information", Name: "IsArray", Result: Type{Kind: TypeBoolean},
		Parameters: []Parameter{variant("Expression")}, Section: "6.1.2.7"},
}

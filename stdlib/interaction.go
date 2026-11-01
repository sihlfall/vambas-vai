package stdlib

var interactionFunctions = [...]Function{
	// The existing runtime supports only Class, without the optional Server form.
	{ID: CreateObject, Module: "Interaction", Name: "CreateObject", Result: Type{Kind: TypeObject},
		Parameters: []Parameter{{Name: "Class", Type: Type{Kind: TypeString}}}, Section: "6.1.2.8"},
	{ID: IIf, Call: SnapshotCall, Module: "Interaction", Name: "IIf", Result: Type{Kind: TypeVariant},
		Parameters: []Parameter{variant("Expr"), variant("TruePart"), variant("FalsePart")}, Section: "6.1.2.8"},
}

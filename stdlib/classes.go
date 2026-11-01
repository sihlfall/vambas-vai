package stdlib

// Preserve the existing executable subset and signatures during migration.
// Err's writable state and default Number read remain specialized operations;
// Collection members retain receiver-aware invocation and object ownership.
var classes = [...]Class{
	{ID: Collection, Name: "Collection", Section: "6.1.3.1", DefaultMember: CollectionItem,
		Members: []ClassMember{
			{ID: CollectionAdd, Name: "Add", Kind: SubMember, Section: "6.1.3.1",
				Parameters: []Parameter{variant("Item"), optional("Key", Literal{}), optional("Before", Literal{}), optional("After", Literal{})}},
			{ID: CollectionItem, Name: "Item", Kind: GetMember, Type: Type{Kind: TypeVariant}, Section: "6.1.3.1",
				Parameters: []Parameter{variant("Index")}},
			{ID: CollectionRemove, Name: "Remove", Kind: SubMember, Section: "6.1.3.1", Parameters: []Parameter{variant("Index")}},
			{ID: CollectionCount, Name: "Count", Kind: GetMember, Type: Type{Kind: TypeLong}, Section: "6.1.3.1"},
		}},
	{ID: Err, Name: "Err", Section: "6.1.3.2", Singleton: true,
		Members: []ClassMember{
			{ID: ErrNumber, Name: "Number", Kind: VariableMember, Type: Type{Kind: TypeLong}, Section: "6.1.3.2"},
			{ID: ErrDescription, Name: "Description", Kind: VariableMember, Type: Type{Kind: TypeString}, Section: "6.1.3.2"},
			{ID: ErrSource, Name: "Source", Kind: VariableMember, Type: Type{Kind: TypeString}, Section: "6.1.3.2"},
			{ID: ErrHelpFile, Name: "HelpFile", Kind: VariableMember, Type: Type{Kind: TypeString}, Section: "6.1.3.2"},
			{ID: ErrHelpContext, Name: "HelpContext", Kind: VariableMember, Type: Type{Kind: TypeLong}, Section: "6.1.3.2"},
			{ID: ErrRaise, Name: "Raise", Kind: SubMember, Section: "6.1.3.2",
				Parameters: []Parameter{variant("Number"), optional("Source", Literal{}), optional("Description", Literal{}), optional("HelpFile", Literal{}), optional("HelpContext", Literal{})}},
			{ID: ErrClear, Name: "Clear", Kind: SubMember, Section: "6.1.3.2"},
		}},
}

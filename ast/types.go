package ast

// TypeDeclaration is VBA's record declaration. EnumDeclaration has the same
// named member-list shape, following typescript-go's declaration organization.
// The different kinds preserve their different binding and value semantics.
type TypeDeclaration struct {
	StatementBase
	Name      *Node
	Modifiers Modifiers
	Members   *NodeList
}
type EnumDeclaration = TypeDeclaration

func (n *Node) AsTypeDeclaration() *TypeDeclaration { return n.data.(*TypeDeclaration) }
func (n *Node) AsEnumDeclaration() *EnumDeclaration { return n.AsTypeDeclaration() }
func (n *TypeDeclaration) DeeplyEquals(other *Node) bool {
	o := other.AsTypeDeclaration()
	return n.Modifiers == o.Modifiers && n.Name.DeeplyEquals(o.Name) && Equals(n.Members, o.Members)
}
func (f *NodeFactory) NewTypeDeclaration(modifiers Modifiers, name *Node, members *NodeList) *Node {
	n := f.typeDeclarationPool.New()
	n.Modifiers, n.Name, n.Members = modifiers, name, members
	return f.newNode(KindTypeDeclaration, n)
}
func (f *NodeFactory) NewEnumDeclaration(modifiers Modifiers, name *Node, members *NodeList) *Node {
	n := f.NewTypeDeclaration(modifiers, name, members)
	n.Kind = KindEnumDeclaration
	return n
}

// Fields reuse variable declaration storage for annotation and dimensions,
// while their kind and symbol distinguish record members from module locals.
type FieldDeclaration = VariableDeclaration

func (n *Node) AsFieldDeclaration() *FieldDeclaration { return n.AsVariableDeclaration() }
func (f *NodeFactory) NewFieldDeclaration(name, typeNode *Node, dimensions *NodeList) *Node {
	n := f.NewVariableDeclaration(name, typeNode, nil, dimensions)
	n.Kind = KindFieldDeclaration
	return n
}

type EnumMember struct {
	NodeBase
	Name        *Node
	Initializer *Node // nil retains VBA's implicit zero/previous-plus-one syntax.
}

func (n *Node) AsEnumMember() *EnumMember { return n.data.(*EnumMember) }
func (n *EnumMember) DeeplyEquals(other *Node) bool {
	o := other.AsEnumMember()
	return n.Name.DeeplyEquals(o.Name) && n.Initializer.DeeplyEquals(o.Initializer)
}
func (f *NodeFactory) NewEnumMember(name, initializer *Node) *Node {
	n := f.enumMemberPool.New()
	n.Name, n.Initializer = name, initializer
	return f.newNode(KindEnumMember, n)
}

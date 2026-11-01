package ast

type ImplementsStatement struct {
	StatementBase
	Interface *Node
}

func (n *Node) AsImplementsStatement() *ImplementsStatement { return n.data.(*ImplementsStatement) }
func (n *ImplementsStatement) DeeplyEquals(other *Node) bool {
	return n.Interface.DeeplyEquals(other.AsImplementsStatement().Interface)
}
func (f *NodeFactory) NewImplementsStatement(name *Node) *Node {
	return f.newNode(KindImplementsStatement, &ImplementsStatement{Interface: name})
}

// ModuleKind describes a host component, not a block scope or an import mode.
// Unspecified retains the historical standalone source-file behavior.
type ModuleKind uint8

const (
	ModuleKindUnspecified ModuleKind = iota
	ModuleKindStandard
	ModuleKindClass
	ModuleKindDocument
	ModuleKindUserForm
)

func (kind ModuleKind) IsClass() bool {
	return kind == ModuleKindClass || kind == ModuleKindDocument || kind == ModuleKindUserForm
}

// AttributeStatement retains exported VBA metadata, including member attributes.
// Names and values are syntax, not executable value references.
type AttributeStatement struct {
	StatementBase
	Name   *Node
	Values *NodeList
}

func (node *Node) AsAttributeStatement() *AttributeStatement {
	return node.data.(*AttributeStatement)
}

func (node *AttributeStatement) DeeplyEquals(other *Node) bool {
	n := other.AsAttributeStatement()
	return node.Name.DeeplyEquals(n.Name) && node.Values.DeeplyEquals(n.Values)
}

func (f *NodeFactory) NewAttributeStatement(name *Node, values *NodeList) *Node {
	return f.newNode(KindAttributeStatement, &AttributeStatement{Name: name, Values: values})
}

func (f *NodeFactory) NewOptionPrivateModuleStatement() *Node {
	return f.NewToken(KindOptionPrivateModuleStatement)
}

type OptionBaseStatement struct {
	StatementBase
	Base int
}

func (n *Node) AsOptionBaseStatement() *OptionBaseStatement { return n.data.(*OptionBaseStatement) }
func (n *OptionBaseStatement) DeeplyEquals(other *Node) bool {
	return n.Base == other.AsOptionBaseStatement().Base
}
func (f *NodeFactory) NewOptionBaseStatement(base int) *Node {
	return f.newNode(KindOptionBaseStatement, &OptionBaseStatement{Base: base})
}

type EraseStatement struct {
	StatementBase
	Expressions *NodeList
}

func (n *Node) AsEraseStatement() *EraseStatement { return n.data.(*EraseStatement) }
func (n *EraseStatement) DeeplyEquals(other *Node) bool {
	return Equals(n.Expressions, other.AsEraseStatement().Expressions)
}
func (f *NodeFactory) NewEraseStatement(expressions *NodeList) *Node {
	return f.newNode(KindEraseStatement, &EraseStatement{Expressions: expressions})
}

// SourceFileOf follows parent links after binding. Metadata links are not children.
func SourceFileOf(node *Node) *SourceFile {
	for node != nil {
		if node.Kind == KindSourceFile {
			return node.AsSourceFile()
		}
		node = node.Parent
	}
	return nil
}

package ast

// Accessors share VBA's procedure signature/body representation. Their distinct
// kinds preserve all three operations while binding can merge their symbols.
type GetAccessor = FunctionDeclaration
type LetAccessor = FunctionDeclaration
type SetAccessor = FunctionDeclaration

func (n *Node) AsGetAccessor() *GetAccessor { return n.AsFunctionDeclaration() }
func (n *Node) AsLetAccessor() *LetAccessor { return n.AsFunctionDeclaration() }
func (n *Node) AsSetAccessor() *SetAccessor { return n.AsFunctionDeclaration() }

func (f *NodeFactory) NewGetAccessor(modifiers Modifiers, name *Node, parameters *NodeList, body *Node) *Node {
	n := f.NewFunctionDeclaration(modifiers, name, parameters, body)
	n.Kind = KindGetAccessor
	return n
}
func (f *NodeFactory) NewLetAccessor(modifiers Modifiers, name *Node, parameters *NodeList, body *Node) *Node {
	n := f.NewFunctionDeclaration(modifiers, name, parameters, body)
	n.Kind = KindLetAccessor
	return n
}
func (f *NodeFactory) NewSetAccessor(modifiers Modifiers, name *Node, parameters *NodeList, body *Node) *Node {
	n := f.NewFunctionDeclaration(modifiers, name, parameters, body)
	n.Kind = KindSetAccessor
	return n
}

// PropertyAccessor returns one accessor declaration from a merged property.
func PropertyAccessor(symbol *Symbol, kind Kind) *Node {
	if symbol != nil {
		for _, declaration := range symbol.Declarations {
			if declaration.Kind == kind {
				return declaration
			}
		}
	}
	return nil
}

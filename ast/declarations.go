package ast

func identifierTypeHint(last byte) Kind {
	switch last {
	case '%':
		return KindPERCENT
	case '&':
		return KindAMPERSAND
	case '^':
		return KindPOW
	case '!':
		return KindEXCLAMATIONPOINT
	case '#':
		return KindHASH
	case '@':
		return KindAT
	case '$':
		return KindDOLLAR
	}
	return KindNONE
}

// IdentifierTypeHint also recognizes the shared numeric literal type characters.
func IdentifierTypeHint(last byte) Kind { return identifierTypeHint(last) }

// Variable statements, declaration lists, parameters, and type references mirror
// typescript-go's syntax nodes. Array bounds and passing modes are VBA syntax.
type VariableStatement struct {
	StatementBase
	Modifiers       Modifiers
	Keyword         Kind
	DeclarationList *Node
}

func (node *Node) AsVariableStatement() *VariableStatement { return node.data.(*VariableStatement) }
func (n *VariableStatement) DeeplyEquals(other *Node) bool {
	o := other.AsVariableStatement()
	return n.Modifiers == o.Modifiers && n.Keyword == o.Keyword && n.DeclarationList.DeeplyEquals(o.DeclarationList)
}
func (f *NodeFactory) NewVariableStatement(modifiers Modifiers, keyword Kind, list *Node) *Node {
	data := f.variableStatementPool.New()
	data.Modifiers, data.Keyword, data.DeclarationList = modifiers, keyword, list
	return f.newNode(KindVariableStatement, data)
}

type VariableDeclarationList struct {
	NodeBase
	Declarations *NodeList
}

func (node *Node) AsVariableDeclarationList() *VariableDeclarationList {
	return node.data.(*VariableDeclarationList)
}
func (n *VariableDeclarationList) DeeplyEquals(other *Node) bool {
	return n.Flags&NodeFlagsConst == other.Flags&NodeFlagsConst && Equals(n.Declarations, other.AsVariableDeclarationList().Declarations)
}
func (f *NodeFactory) NewVariableDeclarationList(declarations *NodeList, constant bool) *Node {
	data := f.variableDeclarationListPool.New()
	data.Declarations = declarations
	n := f.newNode(KindVariableDeclarationList, data)
	if constant {
		n.Flags |= NodeFlagsConst
	}
	return n
}

type VariableDeclaration struct {
	NodeBase
	Name        *IdentifierNode
	Type        *Node
	Initializer *Expression // Const value; Dim has no assignment initializer in VBA.
	Dimensions  *NodeList   // nil for scalar; empty for a dynamic array.
}

func (node *Node) AsVariableDeclaration() *VariableDeclaration {
	return node.data.(*VariableDeclaration)
}
func (n *VariableDeclaration) DeeplyEquals(other *Node) bool {
	o := other.AsVariableDeclaration()
	return n.Name.DeeplyEquals(o.Name) && n.Type.DeeplyEquals(o.Type) && n.Initializer.DeeplyEquals(o.Initializer) && Equals(n.Dimensions, o.Dimensions)
}
func (f *NodeFactory) NewVariableDeclaration(name, typeNode, initializer *Node, dimensions *NodeList) *Node {
	data := f.variableDeclarationPool.New()
	data.Name, data.Type, data.Initializer, data.Dimensions = name, typeNode, initializer, dimensions
	return f.newNode(KindVariableDeclaration, data)
}

type ParameterModifiers uint8

const (
	ParameterOptional ParameterModifiers = 1 << iota
	ParameterByVal
	ParameterByRef
	ParameterParamArray
)

type ParameterDeclaration struct {
	NodeBase
	Name        *IdentifierNode
	Type        *Node
	Initializer *Expression
	Dimensions  *NodeList
	Modifiers   ParameterModifiers
}

func (node *Node) AsParameterDeclaration() *ParameterDeclaration {
	return node.data.(*ParameterDeclaration)
}
func (n *ParameterDeclaration) IsByRef() bool {
	return n.Modifiers&(ParameterByVal|ParameterParamArray) == 0
}
func (n *ParameterDeclaration) DeeplyEquals(other *Node) bool {
	o := other.AsParameterDeclaration()
	return n.Modifiers == o.Modifiers && n.Name.DeeplyEquals(o.Name) && n.Type.DeeplyEquals(o.Type) && n.Initializer.DeeplyEquals(o.Initializer) && Equals(n.Dimensions, o.Dimensions)
}
func (f *NodeFactory) NewParameterDeclaration(modifiers ParameterModifiers, name, typeNode, initializer *Node, dimensions *NodeList) *Node {
	data := f.parameterDeclarationPool.New()
	data.Modifiers, data.Name, data.Type, data.Initializer, data.Dimensions = modifiers, name, typeNode, initializer, dimensions
	return f.newNode(KindParameter, data)
}

type TypeReference struct {
	NodeBase
	TypeName   *Node // Built-in token, identifier, or qualified name; never a value use.
	NewKeyword *TokenNode
	IsArray    bool
	Length     *Expression // Optional fixed-length String size.
}

func (node *Node) AsTypeReference() *TypeReference { return node.data.(*TypeReference) }
func (n *TypeReference) DeeplyEquals(other *Node) bool {
	o := other.AsTypeReference()
	return n.TypeName.DeeplyEquals(o.TypeName) && n.NewKeyword.DeeplyEquals(o.NewKeyword) && n.IsArray == o.IsArray && n.Length.DeeplyEquals(o.Length)
}
func (f *NodeFactory) NewTypeReference(name, newKeyword *Node, array bool, length *Node) *Node {
	data := f.typeReferencePool.New()
	data.TypeName, data.NewKeyword, data.IsArray, data.Length = name, newKeyword, array, length
	return f.newNode(KindTypeReference, data)
}

type QualifiedName struct {
	NodeBase
	Left, Right *Node
}

func (node *Node) AsQualifiedName() *QualifiedName { return node.data.(*QualifiedName) }
func (n *QualifiedName) DeeplyEquals(other *Node) bool {
	o := other.AsQualifiedName()
	return n.Left.DeeplyEquals(o.Left) && n.Right.DeeplyEquals(o.Right)
}
func (f *NodeFactory) NewQualifiedName(left, right *Node) *Node {
	data := f.qualifiedNamePool.New()
	data.Left, data.Right = left, right
	return f.newNode(KindQualifiedName, data)
}

type ArrayDimension struct {
	NodeBase
	LowerBound, UpperBound *Expression
}

func (node *Node) AsArrayDimension() *ArrayDimension { return node.data.(*ArrayDimension) }
func (n *ArrayDimension) DeeplyEquals(other *Node) bool {
	o := other.AsArrayDimension()
	return n.LowerBound.DeeplyEquals(o.LowerBound) && n.UpperBound.DeeplyEquals(o.UpperBound)
}
func (f *NodeFactory) NewArrayDimension(lower, upper *Node) *Node {
	data := f.arrayDimensionPool.New()
	data.LowerBound, data.UpperBound = lower, upper
	return f.newNode(KindArrayDimension, data)
}

type SubDeclaration = FunctionDeclaration

func (node *Node) AsSubDeclaration() *SubDeclaration { return node.AsFunctionDeclaration() }
func (f *NodeFactory) NewSubDeclaration(modifiers Modifiers, name *Node, parameters *NodeList, body *Node) *Node {
	node := f.NewFunctionDeclaration(modifiers, name, parameters, body)
	node.Kind = KindSubDeclaration
	return node
}
func (f *NodeFactory) NewOptionExplicitStatement() *Node {
	return f.NewToken(KindOptionExplicitStatement)
}

func IsProcedureDeclaration(node *Node) bool {
	return node != nil && (node.Kind == KindFunctionDeclaration || node.Kind == KindSubDeclaration || IsPropertyAccessor(node))
}

func IsPropertyAccessor(node *Node) bool {
	return node != nil && (node.Kind == KindGetAccessor || node.Kind == KindLetAccessor || node.Kind == KindSetAccessor)
}

func DeclarationName(node *Node) *Node {
	if node == nil {
		return nil
	}
	switch node.Kind {
	case KindFunctionDeclaration, KindSubDeclaration, KindGetAccessor, KindLetAccessor, KindSetAccessor:
		return node.AsFunctionDeclaration().Name()
	case KindTypeDeclaration, KindEnumDeclaration:
		return node.AsTypeDeclaration().Name
	case KindEnumMember:
		return node.AsEnumMember().Name
	case KindVariableDeclaration, KindFieldDeclaration:
		return node.AsVariableDeclaration().Name
	case KindParameter:
		return node.AsParameterDeclaration().Name
	case KindIdentifier:
		return node // Compatibility with factory-created parameters.
	}
	return nil
}

func IsConstantSymbol(symbol *Symbol) bool {
	if symbol == nil || symbol.ValueDeclaration == nil {
		return false
	}
	if symbol.Flags&SymbolFlagsEnumMember != 0 {
		return true
	}
	node := symbol.ValueDeclaration
	return node.Kind == KindVariableDeclaration && node.Parent != nil && node.Parent.Kind == KindVariableDeclarationList && node.Parent.Flags&NodeFlagsConst != 0
}

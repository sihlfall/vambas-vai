package ast

import (
	"sync"
	"sync/atomic"

	"sihlfall.ch/vambas-vai/collections"
	"sihlfall.ch/vambas-vai/core"
)

/*
// Visitor

type Visitor func(*Node) bool

func visit(v Visitor, node *Node) bool {
	if node != nil {
		return v(node)
	}
	return false
}
*/

type Modifiers uint8

const (
	ModifierNone    Modifiers = 0
	ModifierPrivate Modifiers = 1 << iota
	VisibiltiyFriend
	ModifierPublic
	ModifierGlobal
	ModifierStatic
)

// AST Node
// Interface values stored in AST nodes are never typed nil values. Construction code must ensure that
// interface valued properties either store a true nil or a reference to a non-nil struct.

type Node struct {
	Kind  Kind
	Flags NodeFlags
	Loc   core.TextRange
	//	id     atomic.Uint64
	Parent   *Node
	Symbol   *Symbol
	Locals   SymbolTable
	FlowNode *FlowNode // Flow immediately before evaluating this node.
	data     nodeData
}

func (node *Node) AsNode() *Node                         { return node }
func (node *Node) AsBinaryExpression() *BinaryExpression { return node.data.(*BinaryExpression) }
func (node *Node) AsBlock() *Block                       { return node.data.(*Block) }
func (node *Node) AsFunctionDeclaration() *FunctionDeclaration {
	return node.data.(*FunctionDeclaration)
}
func (node *Node) AsGotoStatement() *GotoStatement       { return node.data.(*GotoStatement) }
func (node *Node) AsIdentifier() *Identifier             { return node.data.(*Identifier) }
func (node *Node) AsIfStatement() *IfStatement           { return node.data.(*IfStatement) }
func (node *Node) AsLabeledStatement() *LabeledStatement { return node.data.(*LabeledStatement) }
func (node *Node) AsLetStatement() *LetStatement         { return node.data.(*LetStatement) }
func (node *Node) AsLineNumber() *LineNumber             { return node.data.(*LineNumber) }
func (node *Node) AsNumericLiteral() *NumericLiteral     { return node.data.(*NumericLiteral) }
func (node *Node) AsSourceFile() *SourceFile             { return node.data.(*SourceFile) }
func (node *Node) AsStringLiteral() *StringLiteral       { return node.data.(*StringLiteral) }
func (node *Node) AsToken() *Token                       { return node.data.(*Token) }
func (node *Node) AsPrefixUnaryExpression() *PrefixUnaryExpression {
	return node.data.(*PrefixUnaryExpression)
}
func (node *Node) AsParenthesizedExpression() *ParenthesizedExpression {
	return node.data.(*ParenthesizedExpression)
}

func (node *Node) Pos() int { return node.Loc.Pos() }
func (node *Node) End() int { return node.Loc.End() }

//func (node *Node) AsKeywordTypeNode() *KeywordTypeNode   { return node.data.(*KeywordTypeNode) }

func (node *Node) DeeplyEquals(other *Node) bool {
	if node == nil || other == nil {
		return node == other
	}
	return node.Kind == other.Kind && node.data.DeeplyEquals(other)
}

type NodeDefault struct {
	Node
}

/*
func (nodeDefault * NodeDefault) DeeplyEquals (other *nodeData) bool {
	return true
}
*/

//func (node *NodeDefault) AsNode() *Node { return &node.Node }

type NodeBase struct {
	NodeDefault
}

type (
	Statement  = Node
	Expression = Node
)

type (
	BlockNode      = Node
	IdentifierNode = Node
	LineNumberNode = Node
	TokenNode      = Node
)

type (
	ParameterList = NodeList
)

// NodeList

type NodeList struct {
	Loc   core.TextRange
	Nodes []*Node
}

func (lst *NodeList) DeeplyEquals(other *NodeList) bool {
	// Todo: compare Loc?
	if len(lst.Nodes) != len(other.Nodes) {
		return false
	}
	for i := 0; i < len(lst.Nodes); i++ {
		if !lst.Nodes[i].DeeplyEquals(other.Nodes[i]) {
			return false
		}
	}
	return true
}

func (f *NodeFactory) NewNodeList(nodes []*Node) *NodeList {
	list := f.nodeListPool.New()
	list.Loc = core.UndefinedTextRange()
	list.Nodes = nodes
	return list
}

type StatementBase struct {
	NodeBase
	// FlowNodeBase
}

type nodeData interface {
	AsNode() *Node
	DeeplyEquals(other *Node) bool
}

type NodeFactory struct {
	nodeCount                    int
	textCount                    int
	binaryExpressionPool         core.Pool[BinaryExpression]
	blockPool                    core.Pool[Block]
	functionDeclarationPool      core.Pool[FunctionDeclaration]
	identifierPool               core.Pool[Identifier]
	ifStatementPool              core.Pool[IfStatement]
	keywordTypeNodePool          core.Pool[KeywordTypeNode]
	lineNumberPool               core.Pool[LineNumber]
	nodeListPool                 core.Pool[NodeList]
	numericLiteralPool           core.Pool[NumericLiteral]
	prefixUnaryExpressionPool    core.Pool[PrefixUnaryExpression]
	parenthesizedExpressionPool  core.Pool[ParenthesizedExpression]
	stringLiteralPool            core.Pool[StringLiteral]
	tokenPool                    core.Pool[Token]
	variableStatementPool        core.Pool[VariableStatement]
	variableDeclarationListPool  core.Pool[VariableDeclarationList]
	variableDeclarationPool      core.Pool[VariableDeclaration]
	parameterDeclarationPool     core.Pool[ParameterDeclaration]
	typeReferencePool            core.Pool[TypeReference]
	qualifiedNamePool            core.Pool[QualifiedName]
	arrayDimensionPool           core.Pool[ArrayDimension]
	propertyAccessExpressionPool core.Pool[PropertyAccessExpression]
	callExpressionPool           core.Pool[CallExpression]
	callStatementPool            core.Pool[CallStatement]
	namedArgumentPool            core.Pool[NamedArgument]
	omittedExpressionPool        core.Pool[OmittedExpression]
	whileStatementPool           core.Pool[WhileStatement]
	doStatementPool              core.Pool[DoStatement]
	forStatementPool             core.Pool[ForStatement]
	forEachStatementPool         core.Pool[ForEachStatement]
	selectStatementPool          core.Pool[SelectStatement]
	caseBlockPool                core.Pool[CaseBlock]
	caseClausePool               core.Pool[CaseClause]
	caseRangeClausePool          core.Pool[CaseRangeClause]
	caseComparisonClausePool     core.Pool[CaseComparisonClause]
	exitStatementPool            core.Pool[ExitStatement]
	newExpressionPool            core.Pool[NewExpression]
	midAssignmentPool            core.Pool[MidAssignmentStatement]
	withStatementPool            core.Pool[WithStatement]
	typeDeclarationPool          core.Pool[TypeDeclaration]
	enumMemberPool               core.Pool[EnumMember]
	reDimStatementPool           core.Pool[ReDimStatement]
	reDimDeclarationPool         core.Pool[ReDimDeclaration]
}

func (f *NodeFactory) newNode(kind Kind, data nodeData) *Node {
	f.nodeCount++
	n := data.AsNode()
	n.Loc = core.UndefinedTextRange()
	n.Kind = kind
	n.data = data
	//	if hooks.OnCreate != nil {
	//		hooks.OnCreate(n)
	//	}
	return n
}

type Token struct {
	NodeBase
}

func (node *Token) DeeplyEquals(other *Node) bool {
	return node.Kind == other.Kind
}

func (f *NodeFactory) NewToken(kind Kind) *Node {
	return f.newNode(kind, f.tokenPool.New())
}

type TypeNodeBase struct {
	NodeBase
	// typeSyntaxBase
}

type KeywordTypeNode struct {
	TypeNodeBase
}

func (node *KeywordTypeNode) DeeplyEquals(other *Node) bool {
	return other.Kind == KindKeywordTypeNode
}

func (f *NodeFactory) NewKeywordTypeNode(kind Kind) *Node {
	return f.newNode(kind, f.keywordTypeNodePool.New())
}

type LiteralLikeBase struct {
	Text       string
	TokenFlags TokenFlags
}

func (b *LiteralLikeBase) DeeplyEquals(other *LiteralLikeBase) bool {
	return b.Text == other.Text && b.TokenFlags == other.TokenFlags
}

type NumericLiteral struct {
	Expression
	LiteralLikeBase
}

func (f *NodeFactory) NewNumericLiteral(text string) *Node {
	data := f.numericLiteralPool.New()
	data.Text = text
	f.textCount++
	return f.newNode(KindNumericLiteral, data)
}

func (node *NumericLiteral) DeeplyEquals(other *Node) bool {
	if other.Kind != KindNumericLiteral {
		return false
	}
	oo := other.AsNumericLiteral()
	return node.LiteralLikeBase.DeeplyEquals(&oo.LiteralLikeBase)
}

type StringLiteral struct {
	Expression
	LiteralLikeBase
}

func (f *NodeFactory) NewStringLiteral(text string) *Node {
	data := f.stringLiteralPool.New()
	data.Text = text
	f.textCount++
	return f.newNode(KindStringLiteral, data)
}

func (node *StringLiteral) DeeplyEquals(other *Node) bool {
	if other.Kind != KindStringLiteral {
		return false
	}
	oo := other.AsStringLiteral()
	return node.LiteralLikeBase.DeeplyEquals(&oo.LiteralLikeBase)
}

type LabeledStatement struct {
	StatementBase
	LineNumber *LineNumberNode
	Label      *IdentifierNode
	Statement  *Statement
}

func (s1 *LabeledStatement) DeeplyEquals(s2 *Node) bool {
	if s2.Kind != KindLabeledStatement {
		return false
	}
	s2labeledStmt := s2.AsLabeledStatement()
	return s1.LineNumber.DeeplyEquals(s2labeledStmt.LineNumber) &&
		s1.Label.DeeplyEquals(s2labeledStmt.Label) &&
		s1.Statement.DeeplyEquals(s2labeledStmt.Statement)
}

func (f *NodeFactory) NewLabeledStatement(
	lineNumber *LineNumberNode, label *IdentifierNode, statement *Statement,
) *Node {
	data := &LabeledStatement{}
	data.LineNumber = lineNumber
	data.Label = label
	data.Statement = statement
	return f.newNode(KindLabeledStatement, data)
}

type Identifier struct {
	NodeBase
	Text     string
	TypeHint Kind // Optional declaration type character; Text retains the spelling.
}

func (node *Identifier) DeeplyEquals(other *Node) bool {
	if other.Kind != KindIdentifier {
		return false
	}
	return node.Text == other.AsIdentifier().Text && node.TypeHint == other.AsIdentifier().TypeHint
}

func (f *NodeFactory) NewIdentifier(text string) *Node {
	data := f.identifierPool.New()
	data.Text = text
	if len(text) != 0 {
		data.TypeHint = identifierTypeHint(text[len(text)-1])
	}
	f.textCount++
	return f.newNode(KindIdentifier, data)
}

type LineNumber struct {
	NodeBase
	Text     string
	hasMinus bool
}

func (node *LineNumber) HasMinus() bool { return node.hasMinus }

func (node *LineNumber) DeeplyEquals(other *Node) bool {
	if other.Kind != KindLineNumber {
		return false
	}
	oo := other.AsLineNumber()
	return node.Text == oo.Text && node.hasMinus == oo.hasMinus
}

func (f *NodeFactory) NewLineNumber(text string, hasMinus bool) *Node {
	data := f.lineNumberPool.New()
	data.Text = text
	data.hasMinus = hasMinus
	f.textCount++
	return f.newNode(KindLineNumber, data)
}

type LetStatement struct {
	NodeBase
	LExpression *Expression
	RExpression *Expression
}

func (node *LetStatement) DeeplyEquals(other *Node) bool {
	if other.Kind != node.Kind {
		return false
	}
	oo := other.AsLetStatement()
	return node.LExpression.DeeplyEquals(oo.LExpression) &&
		node.RExpression.DeeplyEquals(oo.RExpression)
}

func (f *NodeFactory) NewLetStatement(
	lExpression *Expression, rExpression *Expression,
) *Node {
	data := &LetStatement{LExpression: lExpression, RExpression: rExpression}
	return f.newNode(KindLetStatement, data)
}

type GotoStatement struct {
	NodeBase
	LineNumber *LineNumberNode
	Label      *IdentifierNode
	Target     *Symbol // Resolved procedure-local label.
}

func (node *GotoStatement) DeeplyEquals(other *Node) bool {
	if other.Kind != node.Kind {
		return false
	}
	oo := other.AsGotoStatement()
	return node.LineNumber.DeeplyEquals(oo.LineNumber) &&
		node.Label.DeeplyEquals(oo.Label)
}

func (f *NodeFactory) NewGotoStatement(lineNumber *LineNumberNode, label *IdentifierNode) *Node {
	data := &GotoStatement{LineNumber: lineNumber, Label: label}
	return f.newNode(KindGotoStatement, data)
}

type IfStatement struct {
	StatementBase
	//	compositeNodeBase
	Expression    *Expression // Expression
	ThenStatement *Statement  // Statement
	ElseStatement *Statement  // Statement. Optional
}

// ChatGPT generated this correctly
type DeeplyEqualer[T any] interface {
	DeeplyEquals(other *T) bool
}

// Generic equality function
func Equals[T any, PT interface {
	*T
	DeeplyEqualer[T]
}](a, b PT) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.DeeplyEquals(b)
}

func (node *IfStatement) DeeplyEquals(other *Node) bool {
	if other.Kind != KindIfStatement {
		return false
	}
	oo := other.AsIfStatement()
	return Equals(node.Expression, oo.Expression) &&
		Equals(node.ThenStatement, oo.ThenStatement) &&
		Equals(node.ElseStatement, oo.ElseStatement)
}

func (f *NodeFactory) NewIfStatement(
	expression *Expression, thenStatement *Statement, elseStatement *Statement,
) *Node {
	data := f.ifStatementPool.New()
	data.Expression = expression
	data.ThenStatement = thenStatement
	data.ElseStatement = elseStatement
	return f.newNode(KindIfStatement, data)
}

type MeExpression struct {
	Expression
}

func (node *MeExpression) DeeplyEquals(other *Node) bool {
	return other.Kind == KindMeExpression
}

func (f *NodeFactory) NewMeExpression() *Node {
	data := &MeExpression{}
	return f.newNode(KindMeExpression, data)
}

type BinaryExpression struct {
	Expression
	//	DeclarationBase
	//	ModifiersBase
	//	compositeNodeBase
	Left *Expression // Expression
	//	Type          *TypeNode   // TypeNode. Only set by JSDoc @type tags.
	OperatorToken *TokenNode  // TokenNode
	Right         *Expression // Expression
}

func (node *BinaryExpression) DeeplyEquals(other *Node) bool {
	if other.Kind != KindBinaryExpression {
		return false
	}
	oo := other.AsBinaryExpression()
	return (node.Left == nil && oo.Left == nil ||
		node.Left.DeeplyEquals(oo.Left)) &&
		(node.OperatorToken == nil && oo.OperatorToken == nil ||
			node.OperatorToken.DeeplyEquals(oo.OperatorToken)) &&
		(node.Right == nil && oo.Right == nil ||
			node.Right.DeeplyEquals(oo.Right))
}

func (f *NodeFactory) NewBinaryExpression(
	//	modifiers *ModifierList,
	left *Expression,
	//	typeNode *TypeNode,
	operatorToken *TokenNode,
	right *Expression,
) *Node {
	if operatorToken == nil {
		panic("operatorToken is required")
	}
	data := f.binaryExpressionPool.New()
	//	data.modifiers = modifiers
	data.Left = left
	//	data.Type = typeNode
	data.OperatorToken = operatorToken
	data.Right = right
	return f.newNode(KindBinaryExpression, data)
}

type PrefixUnaryExpression struct {
	Expression
	Operator Kind
	Operand  *Expression
}

type ParenthesizedExpression struct {
	Expression
	Inner *Expression
}

func (node *ParenthesizedExpression) DeeplyEquals(other *Node) bool {
	return other.Kind == KindParenthesizedExpression && node.Inner.DeeplyEquals(other.AsParenthesizedExpression().Inner)
}

func (f *NodeFactory) NewParenthesizedExpression(expression *Expression) *Node {
	data := f.parenthesizedExpressionPool.New()
	data.Inner = expression
	return f.newNode(KindParenthesizedExpression, data)
}

func (node *PrefixUnaryExpression) DeeplyEquals(other *Node) bool {
	if other.Kind != KindPrefixUnaryExpression {
		return false
	}
	oo := other.AsPrefixUnaryExpression()
	return (node.Operator == oo.Operator) &&
		(node.Operand == nil && oo.Operand == nil || node.Operand.DeeplyEquals(oo.Operand))
}

func (f *NodeFactory) NewPrefixUnaryExpression(
	operator Kind,
	operand *Expression,
) *Node {
	data := f.prefixUnaryExpressionPool.New()
	data.Operator = operator
	data.Operand = operand
	return f.newNode(KindPrefixUnaryExpression, data)
}

type Block struct {
	StatementBase
	//	LocalsContainerBase
	//	compositeNodeBase
	Statements *NodeList // NodeList[*Statement]
	//	Multiline  bool
}

func (node *Block) DeeplyEquals(other *Node) bool {
	if other.Kind != KindBlock {
		return false
	}
	oo := other.AsBlock()
	if node.Statements == nil {
		return oo.Statements == nil
	}
	return node.Statements.DeeplyEquals(oo.Statements)
}

func (f *NodeFactory) NewBlock(statements *NodeList) *Node {
	data := f.blockPool.New()
	data.Statements = statements
	//	data.Multiline = multiline
	return f.newNode(KindBlock, data)
}

type FunctionLikeBase struct {
	//	LocalsContainerBase
	//	TypeParameters *NodeList // NodeList[*TypeParameterDeclarationNode]. Optional
	Parameters *NodeList // Parameter declarations (legacy factory identifiers are accepted).
	Type       *Node     // Optional return type annotation.
}

func (b *FunctionLikeBase) DeeplyEquals(other *FunctionLikeBase) bool {
	return Equals(b.Parameters, other.Parameters) && b.Type.DeeplyEquals(other.Type)
}

// BodyBase

type BodyBase struct {
	//	AsteriskToken *TokenNode
	Body           *BlockNode
	EndFlowNode    *FlowNode
	ReturnFlowNode *FlowNode // All explicit exits and normal completion.
}

func (b *BodyBase) DeeplyEquals(other *BodyBase) bool {
	return b.Body == nil && other.Body == nil || b.Body.DeeplyEquals(other.Body)
}

//func (node *BodyBase) BodyData() *BodyBase { return node }

// FunctionLikeWithBodyBase

type FunctionLikeWithBodyBase struct {
	FunctionLikeBase
	BodyBase
}

func (b *FunctionLikeWithBodyBase) DeeplyEquals(
	other *FunctionLikeWithBodyBase,
) bool {
	return b.FunctionLikeBase.DeeplyEquals(&other.FunctionLikeBase) &&
		b.BodyBase.DeeplyEquals(&other.BodyBase)
}

type DeclarationBase struct {
	//	Symbol *Symbol // Symbol declared by node (initialized by binding)
}

func (b *DeclarationBase) DeeplyEquals(other *DeclarationBase) bool {
	return true
}

// FunctionDeclaration

type FunctionDeclaration struct {
	StatementBase
	DeclarationBase
	//	ExportableBase
	//	ModifiersBase
	FunctionLikeWithBodyBase
	//	compositeNodeBase
	modifiers    Modifiers
	name         *IdentifierNode // IdentifierNode
	Labels       SymbolTable     // Statement labels have a separate, procedure-wide namespace.
	ResultSymbol *Symbol         // VBA's implicit Function/Property Get result variable.
	//	ReturnFlowNode *FlowNode
}

func (node *FunctionDeclaration) Name() *IdentifierNode { return node.name }
func (node *FunctionDeclaration) Modifiers() Modifiers  { return node.modifiers }

func (b *FunctionDeclaration) DeeplyEquals(other *Node) bool {
	if other.Kind != b.Kind {
		return false
	}
	oo := other.AsFunctionDeclaration()
	return b.DeclarationBase.DeeplyEquals(&oo.DeclarationBase) &&
		b.FunctionLikeWithBodyBase.DeeplyEquals(&oo.FunctionLikeWithBodyBase) &&
		b.modifiers == oo.modifiers &&
		(b.name == nil && oo.name == nil || b.name.DeeplyEquals(oo.name))
}

func (f *NodeFactory) NewFunctionDeclaration(
	modifiers Modifiers,
	//asteriskToken *TokenNode,
	name *IdentifierNode,
	//typeParameters *NodeList,
	parameters *NodeList,
	//	returnType *TypeNode,
	body *BlockNode,
) *Node {
	data := f.functionDeclarationPool.New()
	//	data.modifiers = modifiers
	//	data.AsteriskToken = asteriskToken
	data.modifiers = modifiers
	data.name = name
	//	data.TypeParameters = typeParameters
	data.Parameters = parameters
	//	data.Type = returnType
	data.Body = body
	return f.newNode(KindFunctionDeclaration, data)
}

type SourceFile struct {
	Comments    []Comment
	Annotations []Annotation
	NodeBase
	FileName             string
	ModuleName           string
	ModuleKind           ModuleKind
	PredeclaredID        bool
	OptionPrivateModule  bool
	Text                 string
	Statements           *NodeList // All module elements in source order, as in typescript-go.
	FunctionDeclarations *NodeList // Compatibility view of all procedure declarations, including property accessors.
	OptionExplicit       bool      // Initialized by binding.
	OptionBase           int       // Default lower array bound (0 or 1), initialized by binding.
	EndOfFileToken       *TokenNode
	ParseDiagnostics     []*Diagnostic
	SymbolCount          int
	ClassifiableNames    collections.Set[string]
	BindDiagnostics      []BindingDiagnostic
	bindOnce             sync.Once
	isBound              atomic.Bool
}

func (node *SourceFile) DeeplyEquals(other *Node) bool {
	if other.Kind != KindSourceFile {
		return false
	}
	oo := other.AsSourceFile()
	return Equals(node.Statements, oo.Statements)
}

func (f *NodeFactory) NewSourceFile(statements *NodeList) *Node {
	data := &SourceFile{}
	data.Statements = statements
	data.FunctionDeclarations = statements
	if statements != nil {
		var procedures []*Node
		for _, node := range statements.Nodes {
			if node != nil && IsProcedureDeclaration(node) {
				procedures = append(procedures, node)
			}
		}
		if len(procedures) != len(statements.Nodes) {
			data.FunctionDeclarations = f.NewNodeList(procedures)
			data.FunctionDeclarations.Loc = statements.Loc
		}
	}
	return f.newNode(KindSourceFile, data)
}

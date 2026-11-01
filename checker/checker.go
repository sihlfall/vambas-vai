package checker

import (
	"sort"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/scanner"
)

// Host is implemented by compiler.Program. It avoids a dependency from the
// checker back to the program, following typescript-go's checker host boundary.
type Host interface {
	SourceFiles() []*ast.SourceFile
	Symbol() *ast.Symbol
}

// Checker owns semantic links for a completed, bound project snapshot. The first
// implementation checks eagerly; public queries only read completed maps and can
// be called concurrently. It never writes Node.Symbol, flow, or binding diagnostics.
type Checker struct {
	interfaces       map[*ast.Symbol][]InterfaceImplementation
	interfaceTargets map[*ast.Symbol]bool
	files            []*ast.SourceFile
	project          *ast.Symbol
	intrinsic        [TypeStructure + 1]*Type
	owners           map[*ast.Node]*ast.SourceFile
	types            map[*ast.Node]*Type
	symbols          map[*ast.Node]*ast.Symbol
	symbolTypes      map[*ast.Symbol]*Type
	signatures       map[*ast.Node]*Signature
	resolved         map[*ast.Node]*Signature
	conversions      map[*ast.Node]*Conversion
	constants        map[*ast.Symbol]uint8
	diagnostics      []Diagnostic
	calls            map[*ast.Node]*CallResolution
	storage          map[*ast.Node]*Storage
	valueCalls       map[*ast.Node][]*CallResolution
	valueTypes       map[*ast.Node]*Type
	defaults         map[*ast.Symbol]*ast.Symbol
	defaultConflicts map[*ast.Symbol]bool
	boundErrors      map[*ast.Node]bool
	enumValues       map[*ast.Symbol]int64
	constantNumbers  map[*ast.Symbol]float64
	numberStates     map[*ast.Symbol]uint8
	structureStates  map[*ast.Symbol]uint8
	reDims           map[*ast.Node]*ReDimResolution
	midAssignments   map[*ast.Node]*MidAssignmentResolution
	arrayBounds      map[*ast.Node][]ArrayStaticBound

	constantExpressions map[*ast.Node]float64
}

func NewChecker(host Host) *Checker {
	c := &Checker{
		interfaces:       make(map[*ast.Symbol][]InterfaceImplementation),
		interfaceTargets: make(map[*ast.Symbol]bool),
		files:            host.SourceFiles(), project: host.Symbol(),
		owners: make(map[*ast.Node]*ast.SourceFile), types: make(map[*ast.Node]*Type),
		symbols: make(map[*ast.Node]*ast.Symbol), symbolTypes: make(map[*ast.Symbol]*Type),
		signatures: make(map[*ast.Node]*Signature), resolved: make(map[*ast.Node]*Signature),
		conversions: make(map[*ast.Node]*Conversion), constants: make(map[*ast.Symbol]uint8),
		calls: make(map[*ast.Node]*CallResolution), storage: make(map[*ast.Node]*Storage),
		valueCalls: make(map[*ast.Node][]*CallResolution), defaults: make(map[*ast.Symbol]*ast.Symbol),
		valueTypes:          make(map[*ast.Node]*Type),
		enumValues:          make(map[*ast.Symbol]int64),
		constantNumbers:     make(map[*ast.Symbol]float64),
		numberStates:        make(map[*ast.Symbol]uint8),
		structureStates:     make(map[*ast.Symbol]uint8),
		reDims:              make(map[*ast.Node]*ReDimResolution),
		midAssignments:      make(map[*ast.Node]*MidAssignmentResolution),
		arrayBounds:         make(map[*ast.Node][]ArrayStaticBound),
		constantExpressions: make(map[*ast.Node]float64),

		defaultConflicts: make(map[*ast.Symbol]bool), boundErrors: make(map[*ast.Node]bool),
	}
	for kind := TypeUnknown; kind <= TypeStructure; kind++ {
		c.intrinsic[kind] = &Type{Kind: kind}
	}
	for _, file := range c.files {
		if file == nil || !file.IsBound() {
			panic("checker requires completed, bound source files")
		}
		c.collect(file.AsNode(), file)
		for _, d := range file.BindDiagnostics {
			c.boundErrors[d.Node] = true
		}
	}
	for _, file := range c.files {
		c.collectDefaultMembers(file.AsNode(), file)
		for _, node := range file.Statements.Nodes {
			if node.Kind == ast.KindImplementsStatement {
				c.interfaceTargets[c.symbols[node.AsImplementsStatement().Interface]] = true
			}
		}
	}
	if c.project != nil {
		c.typeOfSymbol(c.project)
	}
	// Resolve constants before ordinary checking, including explicitly typed
	// dependency cycles that would otherwise be hidden by a cached declared type.
	for _, file := range c.files {
		c.visitConstants(file.AsNode())
	}
	for _, file := range c.files {
		c.check(file.AsNode())
	}
	c.checkInterfaces()
	// Warm storage queries before publishing the immutable checker. These may
	// use completed call resolutions, but never introduce new calls themselves.
	for _, file := range c.files {
		c.collectStorage(file.AsNode())
	}
	order := make(map[*ast.SourceFile]int)
	for i, file := range c.files {
		order[file] = i
	}
	sort.SliceStable(c.diagnostics, func(i, j int) bool {
		a, b := c.diagnostics[i], c.diagnostics[j]
		if a.File != b.File {
			return order[a.File] < order[b.File]
		}
		return a.Pos() < b.Pos()
	})
	return c
}

func (c *Checker) collect(node *ast.Node, file *ast.SourceFile) {
	c.owners[node] = file
	if node.Symbol != nil {
		c.symbols[node] = node.Symbol
	}
	node.ForEachChild(func(child *ast.Node) bool { c.collect(child, file); return false })
}

func (c *Checker) GetSymbolAtLocation(node *ast.Node) *ast.Symbol { return c.symbols[node] }
func (c *Checker) GetTypeAtLocation(node *ast.Node) *Type {
	if t := c.types[node]; t != nil {
		return t
	}
	return c.intrinsic[TypeUnknown]
}
func (c *Checker) GetTypeOfSymbol(symbol *ast.Symbol) *Type {
	if t := c.symbolTypes[symbol]; t != nil {
		return t
	}
	return c.intrinsic[TypeUnknown]
}
func (c *Checker) GetSignatureFromDeclaration(node *ast.Node) *Signature {
	return c.signatures[node]
}
func (c *Checker) GetResolvedSignature(node *ast.Node) *Signature { return c.resolved[node] }

// GetEnumMemberValue returns a checked Long value, not a synthetic initializer.
func (c *Checker) GetEnumMemberValue(symbol *ast.Symbol) (int64, bool) {
	v, ok := c.enumValues[symbol]
	return v, ok
}
func (c *Checker) GetAssignmentConversion(node *ast.Node) *Conversion {
	return c.conversions[node]
}

func (c *Checker) GetCallResolution(node *ast.Node) *CallResolution { return c.calls[node] }
func (c *Checker) GetStorageAtLocation(node *ast.Node) *Storage     { return c.storage[node] }
func (c *Checker) GetValueResolutions(node *ast.Node) []*CallResolution {
	return append([]*CallResolution(nil), c.valueCalls[node]...)
}

// GetDiagnostics(nil) returns all semantic diagnostics. Parse/bind diagnostics
// retain their original owners and are deliberately not duplicated here.
func (c *Checker) GetDiagnostics(file *ast.SourceFile) []Diagnostic {
	var result []Diagnostic
	for _, d := range c.diagnostics {
		if file == nil || d.File == file {
			result = append(result, d)
		}
	}
	return result
}

func (c *Checker) report(code DiagnosticCode, node *ast.Node, message string) {
	file := c.owners[node]
	loc := node.Loc
	if file != nil && loc.IsValid() && loc.End() <= len(file.Text) && loc.Len() != 0 {
		state := scanner.ScannerState{Input: file.Text[loc.Pos():loc.End()]}
		state.Scan(scanner.ScanFlagNone)
		loc = core.NewTextRange(loc.Pos()+state.TokenStart(), loc.End())
	}
	c.diagnostics = append(c.diagnostics, Diagnostic{
		Diagnostic: ast.Diagnostic{File: file, Loc: loc, Code: ast.DiagnosticCode(code), Message: message}, Node: node,
	})
}

func (c *Checker) check(node *ast.Node) {
	if node == nil || node.Kind == ast.KindAttributeStatement || node.Kind == ast.KindImplementsStatement {
		return
	}
	c.typeOfNode(node)
	c.checkControlStatement(node)
	if node.Kind == ast.KindMidAssignmentStatement {
		c.checkMidAssignment(node)
		n := node.AsMidAssignmentStatement()
		for _, child := range []*ast.Node{n.Target, n.Start, n.Length, n.Expression} {
			c.check(child)
		}
		return // Mode is statement syntax rather than a value reference.
	}
	if node.Kind == ast.KindReDimStatement {
		c.checkReDim(node)
	}
	if node.Kind == ast.KindEraseStatement {
		for _, target := range node.AsEraseStatement().Expressions.Nodes {
			t, s := c.typeOfNode(target), c.storageOf(target)
			if c.paramArrayStorage(s) {
				c.report(DiagnosticInvalidStorage, target, "Erase cannot clear a ParamArray")
				continue
			}
			if t.Kind != TypeArray && t.Kind != TypeVariant && t.Kind != TypeError || !s.Writable && s.Kind != StorageDynamic {
				c.report(DiagnosticInvalidStorage, target, "Erase requires writable array or Variant storage")
			}
		}
	}
	if node.Kind == ast.KindTypeDeclaration {
		c.checkStructure(node.Symbol)
	}
	if node.Kind == ast.KindParameter && c.typeOfNode(node).Kind == TypeStructure && !node.AsParameterDeclaration().IsByRef() {
		c.report(DiagnosticArgumentType, node, "A user-defined type parameter must be passed ByRef")
	}
	if node.Kind == ast.KindParameter && c.typeOfNode(node).Kind == TypeArray && !node.AsParameterDeclaration().IsByRef() && node.AsParameterDeclaration().Modifiers&ast.ParameterParamArray == 0 {
		c.report(DiagnosticByRefType, node, "A declared array parameter must be passed ByRef")
	}
	if node.Kind == ast.KindLetStatement || node.Kind == ast.KindSetStatement {
		c.checkAssignment(node)
	}
	if node.Kind == ast.KindCallStatement {
		expression := node.AsCallStatement().Expression
		if r := c.calls[expression]; r != nil && (r.Kind == CallPropertyGet || r.Kind == CallArrayReference || r.Kind == CallArrayIndex && scalar(r.ResultType.Kind)) {
			r.Valid = false
			c.report(DiagnosticNotCallable, expression, "A statement call requires a procedure, not a property or array element")
		}
	}
	if node.Kind == ast.KindVariableDeclaration {
		c.checkArrayBounds(node)
		decl := node.AsVariableDeclaration()
		if decl.Initializer != nil && ast.IsConstantSymbol(node.Symbol) {
			c.checkConversion(node, decl.Initializer, c.typeOfSymbol(node.Symbol), false)
		}
	}
	if node.Kind == ast.KindFieldDeclaration {
		c.checkArrayBounds(node)
	}
	if node.Kind == ast.KindParameter {
		decl := node.AsParameterDeclaration()
		if decl.Initializer != nil {
			c.checkConversion(node, decl.Initializer, c.typeOfSymbol(node.Symbol), false)
			c.numericExpression(decl.Initializer) // Warm immutable constant queries for optional defaults.
		}
	}
	node.ForEachChild(func(child *ast.Node) bool { c.check(child); return false })
}

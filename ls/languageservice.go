// Package ls provides editor-independent queries over a captured project
// snapshot. Protocol handling and mutable workspace state live in other layers.
package ls

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/ls/lsconv"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
	"sihlfall.ch/vambas-vai/scanner"
)

// Host keeps language services independent of project/session implementation,
// following typescript-go's language-service host boundary. Reads must describe
// one immutable revision, including the program's original, unmasked source text.
type Host interface {
	Program(context.Context) (*compiler.Program, error)
	ReadFile(fileName string) (string, bool)
	LSPLineMap(fileName string) *lsconv.LineMap
}

type LanguageService struct{ host Host }

type document struct {
	URI      lsproto.DocumentURI
	FileName string
	Text     string
	Lines    *lsconv.LineMap
}

func NewLanguageService(host Host) *LanguageService {
	return &LanguageService{host: host}
}

func (l *LanguageService) document(uri lsproto.DocumentURI) (document, bool) {
	name, err := lsconv.URIToFileName(uri)
	if err != nil {
		return document{}, false
	}
	text, ok := l.host.ReadFile(name)
	if !ok {
		return document{}, false
	}
	return document{URI: lsconv.FileNameToURI(name), FileName: name, Text: text, Lines: l.host.LSPLineMap(name)}, true
}

func (l *LanguageService) file(ctx context.Context, uri lsproto.DocumentURI) (*compiler.Program, *ast.SourceFile, document, error) {
	d, ok := l.document(uri)
	if !ok {
		return nil, nil, d, fmt.Errorf("document is not in this snapshot: %s", uri)
	}
	p, err := l.host.Program(ctx)
	if err != nil {
		return nil, nil, d, err
	}
	return p, p.SourceFile(d.FileName), d, nil
}

func (l *LanguageService) GetDiagnostics(ctx context.Context, uri lsproto.DocumentURI) ([]lsproto.Diagnostic, error) {
	p, file, d, err := l.file(ctx, uri)
	if err != nil {
		return nil, err
	}
	result := []lsproto.Diagnostic{}
	add := func(loc core.TextRange, code, message string) {
		result = append(result, lsproto.Diagnostic{Range: d.Lines.Range(loc), Severity: 1, Code: code, Source: "vba", Message: message})
	}
	for _, diag := range file.ParseDiagnostics {
		add(diag.Loc, string(diag.Code), diag.Message)
	}
	for _, diag := range file.BindDiagnostics {
		add(diag.Loc, string(diag.Code), diag.Message)
		if diag.RelatedFile != nil && diag.RelatedLoc.IsValid() {
			if related, ok := l.document(lsconv.FileNameToURI(diag.RelatedFile.FileName)); ok {
				result[len(result)-1].RelatedInformation = []lsproto.DiagnosticRelatedInformation{{Location: lsproto.Location{URI: related.URI, Range: related.Lines.Range(diag.RelatedLoc)}, Message: "Related declaration"}}
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, diag := range p.GetSemanticDiagnostics(file) {
		add(diag.Loc, string(diag.Code), diag.Message)
	}
	sort.SliceStable(result, func(i, j int) bool {
		a, b := result[i].Range.Start, result[j].Range.Start
		return a.Line < b.Line || a.Line == b.Line && a.Character < b.Character
	})
	return result, ctx.Err()
}

func syntaxRange(node *ast.Node, text string) core.TextRange {
	if node == nil || !node.Loc.IsValid() {
		return core.NewTextRange(0, 0)
	}
	start, end := max(0, min(node.Loc.Pos(), len(text))), max(0, min(node.Loc.End(), len(text)))
	if start >= end {
		return core.NewTextRange(start, start)
	}
	s := scanner.ScannerState{Input: text[start:end]}
	for s.Scan(scanner.ScanFlagNone) == ast.KindNEWLINE {
	}
	return core.NewTextRange(start+s.TokenStart(), end)
}

func declarationName(node *ast.Node) *ast.Node {
	if node != nil && node.Kind == ast.KindReDimDeclaration && node.AsReDimDeclaration().Expression.Kind == ast.KindIdentifier {
		return node.AsReDimDeclaration().Expression
	}
	if node != nil && node.Kind == ast.KindLabeledStatement {
		if label := node.AsLabeledStatement().Label; label != nil {
			return label
		}
		return node.AsLabeledStatement().LineNumber
	}
	return ast.DeclarationName(node)
}

// Touch only name tokens, never their leading trivia or a containing expression.
// This is the small VBA counterpart of typescript-go's touching-name navigation.
func nameAt(file *ast.SourceFile, offset int) *ast.Node {
	var found *ast.Node
	var visit func(*ast.Node)
	visit = func(n *ast.Node) {
		if !n.Loc.ContainsInclusive(offset) {
			return
		}
		if n.Kind == ast.KindIdentifier || n.Kind == ast.KindLineNumber || n.Kind == ast.KindME {
			r := syntaxRange(n, file.Text)
			if r.Len() > 0 && r.ContainsInclusive(offset) && (found == nil || r.Len() < syntaxRange(found, file.Text).Len()) {
				found = n
			}
		}
		n.ForEachChild(func(child *ast.Node) bool { visit(child); return false })
	}
	visit(file.AsNode())
	return found
}

func (l *LanguageService) at(ctx context.Context, params lsproto.TextDocumentPositionParams) (*checker.Checker, *ast.Node, document, error) {
	p, file, d, err := l.file(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, nil, d, err
	}
	offset, err := d.Lines.Offset(params.Position)
	if err != nil {
		return nil, nil, d, err
	}
	node := nameAt(file, offset)
	if node == nil {
		return nil, nil, d, nil
	}
	c := p.GetTypeChecker()
	return c, node, d, ctx.Err()
}

func selectedDeclaration(c *checker.Checker, node *ast.Node, symbol *ast.Symbol) *ast.Node {
	for n := node; n != nil; {
		if sig := c.GetResolvedSignature(n); sig != nil && sig.Declaration.Symbol == symbol {
			return sig.Declaration
		}
		if storage := c.GetStorageAtLocation(n); storage != nil && storage.Resolution != nil {
			if sig := storage.Resolution.Signature; sig != nil && sig.Declaration.Symbol == symbol {
				return sig.Declaration
			}
		}
		parent := n.Parent
		if parent != nil && (parent.Kind == ast.KindPropertyAccessExpression && parent.AsPropertyAccessExpression().Name == n || parent.Kind == ast.KindCallExpression && parent.AsCallExpression().Expression == n) {
			n = parent
		} else {
			break
		}
	}
	return nil
}

func (l *LanguageService) ProvideDefinition(ctx context.Context, params lsproto.TextDocumentPositionParams) ([]lsproto.Location, error) {
	c, node, _, err := l.at(ctx, params)
	result := []lsproto.Location{}
	if err != nil || node == nil {
		return result, err
	}
	symbol := c.GetSymbolAtLocation(node)
	if symbol == nil {
		return result, nil
	}
	declarations := symbol.Declarations
	if selected := selectedDeclaration(c, node, symbol); selected != nil {
		declarations = []*ast.Node{selected}
	}
	for _, decl := range declarations {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		file := ast.SourceFileOf(decl)
		if file == nil {
			continue
		}
		d, ok := l.document(lsconv.FileNameToURI(file.FileName))
		if !ok {
			continue
		}
		loc := core.NewTextRange(0, 0)
		name := declarationName(decl)
		if decl.Kind == ast.KindLabeledStatement {
			line := decl.AsLabeledStatement().LineNumber
			if line != nil && line.AsLineNumber().Text == symbol.Name {
				name = line
			}
		}
		if name != nil {
			loc = syntaxRange(name, file.Text)
		}
		location := lsproto.Location{URI: d.URI, Range: d.Lines.Range(loc)}
		duplicate := false
		for _, previous := range result {
			if previous == location {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, location)
		}
	}
	return result, nil
}

func (l *LanguageService) ProvideHover(ctx context.Context, params lsproto.TextDocumentPositionParams) (*lsproto.Hover, error) {
	c, node, d, err := l.at(ctx, params)
	if err != nil || node == nil {
		return nil, err
	}
	symbol := c.GetSymbolAtLocation(node)
	if symbol == nil {
		return nil, nil
	}
	text := symbolText(c, symbol, selectedDeclaration(c, node, symbol))
	return &lsproto.Hover{Contents: lsproto.MarkupContent{Kind: "plaintext", Value: text}, Range: d.Lines.Range(syntaxRange(node, d.Text))}, nil
}

func symbolText(c *checker.Checker, symbol *ast.Symbol, selected *ast.Node) string {
	if symbol.Flags&ast.SymbolFlagsUserDefinedType != 0 {
		return "Type " + symbol.Name
	}
	if symbol.Flags&ast.SymbolFlagsRegularEnum != 0 {
		return "Enum " + symbol.Name
	}
	if symbol.Flags&ast.SymbolFlagsEnumMember != 0 {
		text := symbol.Parent.Name + "." + symbol.Name + " As Long"
		if value, ok := c.GetEnumMemberValue(symbol); ok {
			text += fmt.Sprintf(" = %d", value)
		}
		return text
	}
	if symbol.Flags&ast.SymbolFlagsProperty != 0 {
		return typedName(symbol.Parent.Name+"."+symbol.Name, c.GetTypeOfSymbol(symbol))
	}
	if symbol.Flags&ast.SymbolFlagsClass != 0 {
		return "Class " + symbol.Name
	}
	if symbol.Flags&ast.SymbolFlagsValueModule != 0 {
		return "Module " + symbol.Name
	}
	if symbol.Flags&ast.SymbolFlagsLabel != 0 {
		return symbol.Name + ":"
	}
	if symbol.Flags&(ast.SymbolFlagsFunction|ast.SymbolFlagsAccessor) != 0 {
		decls := symbol.Declarations
		if selected != nil {
			decls = []*ast.Node{selected}
		}
		var parts []string
		for _, decl := range decls {
			if sig := c.GetSignatureFromDeclaration(decl); sig != nil {
				parts = append(parts, signatureText(symbol.Name, sig))
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	prefix := "Dim "
	if ast.IsConstantSymbol(symbol) {
		prefix = "Const "
	}
	if symbol.ValueDeclaration != nil && symbol.ValueDeclaration.Kind == ast.KindParameter {
		prefix = "ByRef "
		if !symbol.ValueDeclaration.AsParameterDeclaration().IsByRef() {
			prefix = "ByVal "
		}
	}
	return prefix + typedName(symbol.Name, c.GetTypeOfSymbol(symbol))
}

func typedName(name string, t *checker.Type) string {
	if t.Kind == checker.TypeArray {
		return name + "(" + strings.Repeat(",", max(0, t.Rank-1)) + ") As " + t.ElementType.String()
	}
	return name + " As " + t.String()
}

func signatureText(name string, sig *checker.Signature) string {
	kind := "Function "
	switch sig.Declaration.Kind {
	case ast.KindSubDeclaration:
		kind = "Sub "
	case ast.KindGetAccessor:
		kind = "Property Get "
	case ast.KindLetAccessor:
		kind = "Property Let "
	case ast.KindSetAccessor:
		kind = "Property Set "
	}
	var params []string
	for _, p := range sig.Parameters {
		prefix := "ByRef "
		if !p.IsByRef() {
			prefix = "ByVal "
		}
		if p.Modifiers&ast.ParameterParamArray != 0 {
			prefix = "ParamArray "
		}
		if p.Modifiers&ast.ParameterOptional != 0 {
			prefix = "Optional " + prefix
		}
		param := prefix + typedName(p.Name, p.Type)
		if p.Default != nil {
			if file := ast.SourceFileOf(p.Default); file != nil {
				r := syntaxRange(p.Default, file.Text)
				param += " = " + file.Text[r.Pos():r.End()]
			}
		}
		params = append(params, param)
	}
	result := kind + name + "(" + strings.Join(params, ", ") + ")"
	if sig.Declaration.Kind == ast.KindFunctionDeclaration || sig.Declaration.Kind == ast.KindGetAccessor {
		result += " As " + sig.ReturnType.String()
	}
	return result
}

func (l *LanguageService) ProvideDocumentSymbols(ctx context.Context, uri lsproto.DocumentURI) ([]*lsproto.DocumentSymbol, error) {
	p, file, d, err := l.file(ctx, uri)
	if err != nil {
		return nil, err
	}
	c := p.GetTypeChecker()
	var walk func(*ast.Node) []*lsproto.DocumentSymbol
	walk = func(node *ast.Node) []*lsproto.DocumentSymbol {
		if ctx.Err() != nil {
			return nil
		}
		name := declarationName(node)
		implicitReDim := node.Kind == ast.KindReDimDeclaration && node.Symbol != nil && node.Symbol.ValueDeclaration == node
		if name != nil && syntaxRange(name, file.Text).Len() > 0 && (ast.IsProcedureDeclaration(node) || node.Kind == ast.KindVariableDeclaration || node.Kind == ast.KindParameter || node.Kind == ast.KindLabeledStatement || node.Kind == ast.KindTypeDeclaration || node.Kind == ast.KindEnumDeclaration || node.Kind == ast.KindEnumMember || node.Kind == ast.KindFieldDeclaration || implicitReDim) {
			kind := 13 // Variable
			display := file.Text[syntaxRange(name, file.Text).Pos():syntaxRange(name, file.Text).End()]
			if ast.IsProcedureDeclaration(node) {
				kind = 12
			}
			if ast.IsPropertyAccessor(node) {
				kind = 7
				suffix := map[ast.Kind]string{ast.KindGetAccessor: "Get", ast.KindLetAccessor: "Let", ast.KindSetAccessor: "Set"}[node.Kind]
				display += " (" + suffix + ")"
			}
			if ast.IsConstantSymbol(node.Symbol) {
				kind = 14
			}
			switch node.Kind {
			case ast.KindTypeDeclaration:
				kind = 23
			case ast.KindEnumDeclaration:
				kind = 10
			case ast.KindEnumMember:
				kind = 22
			case ast.KindFieldDeclaration:
				kind = 8
			}
			if node.Kind == ast.KindLabeledStatement {
				kind = 20
			}
			item := &lsproto.DocumentSymbol{Name: display, Kind: kind, Range: d.Lines.Range(syntaxRange(node, file.Text)), SelectionRange: d.Lines.Range(syntaxRange(name, file.Text))}
			if node.Symbol != nil {
				item.Detail = symbolText(c, node.Symbol, node)
			}
			node.ForEachChild(func(child *ast.Node) bool { item.Children = append(item.Children, walk(child)...); return false })
			return []*lsproto.DocumentSymbol{item}
		}
		var children []*lsproto.DocumentSymbol
		node.ForEachChild(func(child *ast.Node) bool { children = append(children, walk(child)...); return false })
		return children
	}
	result := walk(file.AsNode())
	if result == nil {
		result = []*lsproto.DocumentSymbol{}
	}
	return result, ctx.Err()
}

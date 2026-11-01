package ls

import (
	"context"
	"fmt"
	"sort"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
	"sihlfall.ch/vambas-vai/stdlib"
)

// SemanticTokensLegend returns a fresh legend; neither callers nor negotiation
// can mutate shared state. Lexical tokens remain the TextMate grammar's job.
func SemanticTokensLegend() lsproto.SemanticTokensLegend {
	return lsproto.SemanticTokensLegend{
		TokenTypes:     []string{"namespace", "class", "enum", "struct", "parameter", "variable", "property", "enumMember", "function", "method", "label"},
		TokenModifiers: []string{"declaration", "readonly", "defaultLibrary"},
	}
}

type semanticToken struct {
	location  lsproto.Range
	kind      string
	modifiers []string
}

func classifySymbol(symbol *ast.Symbol) string {
	switch flags := symbol.Flags; {
	case flags&ast.SymbolFlagsClass != 0:
		return "class"
	case flags&ast.SymbolFlagsRegularEnum != 0:
		return "enum"
	case flags&ast.SymbolFlagsUserDefinedType != 0:
		return "struct"
	case flags&ast.SymbolFlagsValueModule != 0:
		return "namespace"
	case flags&ast.SymbolFlagsEnumMember != 0:
		return "enumMember"
	case flags&ast.SymbolFlagsLabel != 0:
		return "label"
	case flags&(ast.SymbolFlagsAccessor|ast.SymbolFlagsProperty) != 0:
		return "property"
	case flags&ast.SymbolFlagsFunction != 0:
		if symbol.Parent != nil && symbol.Parent.Flags&ast.SymbolFlagsClass != 0 {
			return "method"
		}
		return "function"
	case flags&ast.SymbolFlagsFunctionScopedVariable != 0:
		if decl := symbol.ValueDeclaration; decl != nil && decl.Kind == ast.KindParameter {
			return "parameter"
		}
		if symbol.Parent != nil && symbol.Parent.Flags&ast.SymbolFlagsClass != 0 {
			return "property"
		}
		return "variable"
	}
	return ""
}

func semanticModifiers(node *ast.Node, symbol *ast.Symbol) []string {
	var modifiers []string
	for _, decl := range symbol.Declarations {
		declaration := declarationName(decl) == node
		if decl.Kind == ast.KindLabeledStatement {
			n := decl.AsLabeledStatement()
			declaration = node == n.Label || node == n.LineNumber
		}
		if declaration {
			modifiers = append(modifiers, "declaration")
			break
		}
	}
	if ast.IsConstantSymbol(symbol) || symbol.Flags&ast.SymbolFlagsAccessor != 0 && symbol.Flags&(ast.SymbolFlagsLetAccessor|ast.SymbolFlagsSetAccessor) == 0 {
		modifiers = append(modifiers, "readonly")
	}
	if symbol.Library != stdlib.None || symbol.Intrinsic != ast.IntrinsicNone || symbol.ValueDeclaration != nil && ast.SourceFileOf(symbol.ValueDeclaration) == nil {
		modifiers = append(modifiers, "defaultLibrary")
	}
	return modifiers
}

// collectSemanticTokens follows the same checker-backed name traversal as
// typescript-go. VBA adds record types, labels, and Function result variables.
func collectSemanticTokens(ctx context.Context, c *checker.Checker, file *ast.SourceFile, d document, span core.TextRange) []semanticToken {
	var tokens []semanticToken
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ctx.Err() != nil {
			return true
		}
		if node.Kind == ast.KindAttributeStatement {
			return false // Export metadata is syntax, not symbol references.
		}
		if node.Kind == ast.KindIdentifier || node.Kind == ast.KindLineNumber {
			if symbol := c.GetSymbolAtLocation(node); symbol != nil {
				kind := classifySymbol(symbol)
				r := syntaxRange(node, file.Text)
				if kind != "" && r.Len() > 0 && r.Pos() < span.End() && r.End() > span.Pos() {
					position := d.Lines.Range(r)
					// Never emit multiline/overlapping tokens, even for recovery ASTs.
					if position.Start.Line == position.End.Line && position.End.Character > position.Start.Character {
						tokens = append(tokens, semanticToken{position, kind, semanticModifiers(node, symbol)})
					}
				}
			}
		}
		return node.ForEachChild(visit)
	}
	visit(file.AsNode())
	return tokens
}

func encodeSemanticTokens(tokens []semanticToken, legend lsproto.SemanticTokensLegend) *lsproto.SemanticTokens {
	types := make(map[string]uint32, len(legend.TokenTypes))
	for i, name := range legend.TokenTypes {
		types[name] = uint32(i)
	}
	modifiers := make(map[string]uint32, len(legend.TokenModifiers))
	for i, name := range legend.TokenModifiers {
		modifiers[name] = 1 << i
	}
	sort.SliceStable(tokens, func(i, j int) bool {
		a, b := tokens[i].location, tokens[j].location
		if a.Start.Line != b.Start.Line {
			return a.Start.Line < b.Start.Line
		}
		if a.Start.Character != b.Start.Character {
			return a.Start.Character < b.Start.Character
		}
		return a.End.Character < b.End.Character
	})
	result := &lsproto.SemanticTokens{Data: []uint32{}}
	previous := lsproto.Position{}
	previousEnd := lsproto.Position{}
	for _, token := range tokens {
		typeIndex, ok := types[token.kind]
		r := token.location
		if !ok || r.Start.Line == previousEnd.Line && r.Start.Character < previousEnd.Character {
			continue
		}
		bits := uint32(0)
		for _, name := range token.modifiers {
			bits |= modifiers[name]
		}
		start := r.Start.Character
		if r.Start.Line == previous.Line {
			start -= previous.Character
		}
		result.Data = append(result.Data, uint32(r.Start.Line-previous.Line), uint32(start), uint32(r.End.Character-r.Start.Character), typeIndex, bits)
		previous, previousEnd = r.Start, r.End
	}
	return result
}

func (l *LanguageService) ProvideSemanticTokens(ctx context.Context, uri lsproto.DocumentURI, legend lsproto.SemanticTokensLegend) (*lsproto.SemanticTokens, error) {
	return l.provideSemanticTokens(ctx, uri, nil, legend)
}

func (l *LanguageService) ProvideSemanticTokensRange(ctx context.Context, uri lsproto.DocumentURI, rng lsproto.Range, legend lsproto.SemanticTokensLegend) (*lsproto.SemanticTokens, error) {
	return l.provideSemanticTokens(ctx, uri, &rng, legend)
}

func (l *LanguageService) provideSemanticTokens(ctx context.Context, uri lsproto.DocumentURI, rng *lsproto.Range, legend lsproto.SemanticTokensLegend) (*lsproto.SemanticTokens, error) {
	p, file, d, err := l.file(ctx, uri)
	if err != nil {
		return nil, err
	}
	span := core.NewTextRange(0, len(file.Text))
	if rng != nil {
		if rng.End.Line < rng.Start.Line || rng.End.Line == rng.Start.Line && rng.End.Character < rng.Start.Character {
			return nil, fmt.Errorf("semantic token range ends before its start")
		}
		start, err := d.Lines.Offset(rng.Start)
		if err != nil {
			return nil, err
		}
		end, err := d.Lines.Offset(rng.End)
		if err != nil {
			return nil, err
		}
		span = core.NewTextRange(start, end)
	}
	if span.Len() == 0 {
		return &lsproto.SemanticTokens{Data: []uint32{}}, ctx.Err()
	}
	c := p.GetTypeChecker()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	tokens := collectSemanticTokens(ctx, c, file, d, span)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return encodeSemanticTokens(tokens, legend), ctx.Err()
}

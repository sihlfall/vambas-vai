package ls

import (
	"context"
	"sort"
	"strings"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/core"
	"sihlfall.ch/vambas-vai/lsp/lsproto"
	"sihlfall.ch/vambas-vai/scanner"
)

type memberCompletion struct {
	access      *ast.PropertyAccessExpression
	replacement core.TextRange
	bracketed   bool
}

// A dot and its name may be separated by horizontal trivia or an explicit VBA
// continuation. A comment or an ordinary newline starts a different context.
func memberCompletionGap(text string) bool {
	s := scanner.ScannerState{Input: text}
	return s.Scan(scanner.ScanFlagNone) == ast.KindEndOfFile && len(s.Comments) == 0
}

// Like typescript-go's completion context lookup, use recovered member-access
// syntax rather than guessing receiver names by splitting the source on dots.
func memberCompletionAt(ctx context.Context, file *ast.SourceFile, offset int) *memberCompletion {
	var result *memberCompletion
	lastDot := -1
	var visit func(*ast.Node) bool
	visit = func(node *ast.Node) bool {
		if ctx.Err() != nil {
			return true
		}
		if node.Kind == ast.KindAttributeStatement {
			return false
		}
		if node.Kind == ast.KindPropertyAccessExpression {
			access := node.AsPropertyAccessExpression()
			if access.OperatorToken.Kind == ast.KindDOT {
				dot := syntaxRange(access.OperatorToken, file.Text)
				if dot.Len() == 1 && dot.End() <= offset && dot.Pos() > lastDot {
					name := syntaxRange(access.Name, file.Text)
					bracketed := name.Len() > 0 && file.Text[name.Pos()] == '['
					if access.Name.AsIdentifier().Text == "" && !bracketed {
						if memberCompletionGap(file.Text[dot.End():offset]) {
							result = &memberCompletion{access, core.NewTextRange(offset, offset), false}
							lastDot = dot.Pos()
						}
					} else if name.Pos() >= dot.End() && offset <= name.End() && memberCompletionGap(file.Text[dot.End():name.Pos()]) {
						start := name.Pos()
						if offset < start {
							start = offset
						}
						result = &memberCompletion{access, core.NewTextRange(start, name.End()), bracketed}
						lastDot = dot.Pos()
					}
				}
			}
		}
		return node.ForEachChild(visit)
	}
	visit(file.AsNode())
	return result
}

func fieldInsertText(symbol *ast.Symbol, bracketed bool) string {
	// Preserve escaped declaration spellings, e.g. [Display Name]. The label and
	// semantic identity remain the symbol's original unescaped name.
	if decl := symbol.ValueDeclaration; decl != nil {
		if file := ast.SourceFileOf(decl); file != nil {
			r := syntaxRange(ast.DeclarationName(decl), file.Text)
			if r.Len() > 0 && file.Text[r.Pos()] == '[' {
				return "[" + symbol.Name + "]"
			}
		}
	}
	if bracketed {
		return "[" + symbol.Name + "]"
	}
	return symbol.Name
}

// ProvideCompletion returns a complete UDT member list. VS Code filters the
// field names using the text before the cursor, including case-insensitive fuzzy
// matching. Keeping all members avoids caching a prefix-filtered incomplete set.
func (l *LanguageService) ProvideCompletion(ctx context.Context, params lsproto.TextDocumentPositionParams) (*lsproto.CompletionList, error) {
	result := &lsproto.CompletionList{Items: []lsproto.CompletionItem{}}
	p, file, d, err := l.file(ctx, params.TextDocument.URI)
	if err != nil {
		return nil, err
	}
	offset, err := d.Lines.Offset(params.Position)
	if err != nil {
		return nil, err
	}
	completion := memberCompletionAt(ctx, file, offset)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if completion == nil {
		return result, nil
	}
	replacement := d.Lines.Range(completion.replacement)
	if replacement.Start.Line != replacement.End.Line {
		// Completion TextEdits must be single-line. A cursor before a continued
		// name on the following line cannot replace that name with one such edit.
		return result, nil
	}
	receiver := completion.access.Expression
	if receiver == nil && completion.access.WithTarget != nil {
		receiver = completion.access.WithTarget.AsWithStatement().Expression
	}
	if receiver == nil {
		return result, nil
	}
	c := p.GetTypeChecker()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	t := c.GetTypeAtLocation(receiver)
	if t.Kind != checker.TypeStructure || t.Symbol == nil {
		return result, nil
	}
	if symbol := c.GetSymbolAtLocation(ast.SkipParentheses(receiver)); symbol != nil && symbol.Flags&ast.SymbolFlagsUserDefinedType != 0 {
		return result, nil // A UDT's type name is not a value with fields.
	}
	for _, field := range t.Symbol.Members {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if field.Flags&ast.SymbolFlagsProperty == 0 {
			continue
		}
		insert := fieldInsertText(field, completion.bracketed)
		filter := field.Name
		if completion.bracketed {
			filter = insert
		}
		result.Items = append(result.Items, lsproto.CompletionItem{
			Label: field.Name, Kind: lsproto.CompletionItemKindField,
			Detail:   typedName(field.Name, c.GetTypeOfSymbol(field)),
			SortText: strings.ToLower(field.Name), FilterText: filter,
			TextEdit: lsproto.TextEdit{Range: replacement, NewText: insert},
		})
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].SortText < result.Items[j].SortText })
	return result, ctx.Err()
}

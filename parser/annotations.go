package parser

import (
	"strings"
	"unicode"

	"sihlfall.ch/vambas-vai/ast"
)

// Parse the contents of lexer comments, never the source text as a second VBA
// scanner. A quote in a VBA string cannot become an annotation here.
func attachAnnotations(file *ast.SourceFile) {
	for _, comment := range file.Comments {
		text := strings.TrimSpace(comment.Text)
		if !strings.HasPrefix(text, "@") {
			continue
		}
		text = text[1:]
		i := 0
		for i < len(text) && (text[i] >= 'a' && text[i] <= 'z' || text[i] >= 'A' && text[i] <= 'Z' || text[i] >= '0' && text[i] <= '9' || text[i] == '_') {
			i++
		}
		if i == 0 {
			continue
		}
		a := ast.Annotation{Name: text[:i], Range: comment.Range}
		rest := strings.TrimSpace(text[i:])
		if strings.HasPrefix(rest, "(") {
			if !strings.HasSuffix(rest, ")") {
				a.Error = "Unterminated annotation arguments"
			} else {
				rest = strings.TrimSpace(rest[1 : len(rest)-1])
				a.Arguments, a.Error = annotationArguments(rest)
			}
		} else if rest != "" {
			a.Arguments, a.Error = annotationArguments(rest)
		}
		insideProcedure := false
		for _, fn := range file.FunctionDeclarations.Nodes {
			if fn.Pos() <= comment.Range.Pos() && comment.Range.End() <= fn.End() {
				insideProcedure = true
				break
			}
		}
		if comment.Standalone && !insideProcedure {
			if strings.EqualFold(a.Name, "TestModule") || strings.EqualFold(a.Name, "Folder") || strings.EqualFold(a.Name, "ModuleDescription") {
				a.Target = file.AsNode()
			} else {
				// Only a leading comment block may annotate a declaration. Any intervening
				// code (including a preceding procedure's body) breaks association.
				for _, node := range file.FunctionDeclarations.Nodes {
					if node.Pos() <= comment.Range.End() {
						continue
					}
					gap := file.Text[comment.Range.End():node.Pos()]
					for _, other := range file.Comments {
						if other.Range.Pos() >= comment.Range.End() && other.Range.End() <= node.Pos() {
							start, end := other.Range.Pos()-comment.Range.End(), other.Range.End()-comment.Range.End()
							gap = gap[:start] + strings.Repeat(" ", end-start) + gap[end:]
						}
					}
					if strings.TrimSpace(gap) == "" {
						a.Target = node
					}
					break
				}
			}
		}
		file.Annotations = append(file.Annotations, a)
	}
}
func annotationArguments(text string) ([]string, string) {
	var args []string
	for strings.TrimSpace(text) != "" {
		text = strings.TrimSpace(text)
		var value strings.Builder
		if text[0] == '"' {
			text = text[1:]
			closed := false
			for len(text) > 0 {
				if text[0] == '"' {
					if len(text) > 1 && text[1] == '"' {
						value.WriteByte('"')
						text = text[2:]
						continue
					}
					text = text[1:]
					closed = true
					break
				}
				value.WriteByte(text[0])
				text = text[1:]
			}
			if !closed {
				return args, "Unterminated annotation string"
			}
		} else {
			i := strings.IndexByte(text, ',')
			if i < 0 {
				i = len(text)
			}
			value.WriteString(strings.TrimFunc(text[:i], unicode.IsSpace))
			text = text[i:]
		}
		args = append(args, value.String())
		text = strings.TrimSpace(text)
		if text == "" {
			break
		}
		if text[0] != ',' {
			return args, "Expected comma between annotation arguments"
		}
		text = text[1:]
		if strings.TrimSpace(text) == "" {
			return args, "Missing annotation argument"
		}
	}
	return args, ""
}

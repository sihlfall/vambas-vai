package parser_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/binder"
	"sihlfall.ch/vambas-vai/parser"
)

func validateRanges(t *testing.T, file *ast.SourceFile) {
	t.Helper()
	if file.Pos() != 0 || file.End() != len(file.Text) {
		t.Fatal("source file does not span the entire input")
	}
	var visit func(*ast.Node)
	visit = func(node *ast.Node) {
		if !node.Loc.IsValid() || node.Pos() < 0 || node.End() > len(file.Text) {
			t.Fatalf("kind %d has invalid range [%d,%d)", node.Kind, node.Pos(), node.End())
		}
		var list *ast.NodeList
		switch node.Kind {
		case ast.KindSourceFile:
			list = node.AsSourceFile().Statements
		case ast.KindFunctionDeclaration, ast.KindSubDeclaration, ast.KindGetAccessor, ast.KindLetAccessor, ast.KindSetAccessor:
			list = node.AsFunctionDeclaration().Parameters
		case ast.KindVariableDeclarationList:
			list = node.AsVariableDeclarationList().Declarations
		case ast.KindVariableDeclaration:
			list = node.AsVariableDeclaration().Dimensions
		case ast.KindParameter:
			list = node.AsParameterDeclaration().Dimensions
		case ast.KindBlock:
			list = node.AsBlock().Statements
		case ast.KindCallExpression:
			list = node.AsCallExpression().Arguments
		case ast.KindCaseBlock:
			list = node.AsCaseBlock().Clauses
		case ast.KindOnGotoStatement:
			list = node.AsOnGotoStatement().Destinations
		case ast.KindCaseClause, ast.KindDefaultClause:
			list = node.AsCaseClause().Expressions
		}
		if list != nil && (!list.Loc.IsValid() || !list.Loc.ContainedBy(node.Loc)) {
			t.Fatalf("list outside node kind %d", node.Kind)
		}
		node.ForEachChild(func(child *ast.Node) bool {
			if child.Parent != node || !child.Loc.ContainedBy(node.Loc) {
				t.Fatalf("kind %d [%d,%d) outside parent kind %d [%d,%d)", child.Kind, child.Pos(), child.End(), node.Kind, node.Pos(), node.End())
			}
			visit(child)
			return false
		})
	}
	visit(file.AsNode())
	if file.EndOfFileToken == nil || file.EndOfFileToken.End() != len(file.Text) {
		t.Fatal("missing EOF token")
	}
	for _, diagnostic := range file.ParseDiagnostics {
		if diagnostic.File != file || !diagnostic.Loc.IsValid() || diagnostic.End() > len(file.Text) || diagnostic.Message == "" {
			t.Fatal("invalid parse diagnostic")
		}
	}
}

func TestSourceRangesAndParentheses(t *testing.T) {
	source := "' leading comment\r\n  Private Function F()\r\n    Let value = (1 + _\r\n      2)\r\n  F = value\r\nEnd Function ' trailing comment"
	file := parser.ParseSourceFile(source).AsSourceFile()
	if len(file.ParseDiagnostics) != 0 {
		t.Fatal(file.ParseDiagnostics)
	}
	validateRanges(t, file)
	fn := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration()
	if got := source[fn.Name().Pos():fn.Name().End()]; got != " F" {
		t.Fatalf("function name full range = %q", got)
	}
	stmt := fn.Body.AsBlock().Statements.Nodes[0].AsLetStatement()
	if got := source[stmt.LExpression.Pos():stmt.LExpression.End()]; got != " value" {
		t.Fatalf("identifier full range = %q", got)
	}
	paren := stmt.RExpression
	if got := source[paren.Pos():paren.End()]; got != " (1 + _\r\n      2)" || paren.Kind != ast.KindParenthesizedExpression {
		t.Fatalf("parenthesized range = %q", got)
	}
	inner := paren.AsParenthesizedExpression().Inner.AsBinaryExpression()
	if got := source[inner.OperatorToken.Pos():inner.OperatorToken.End()]; got != " +" {
		t.Fatalf("operator full range = %q", got)
	}
	if inner.Right.AsNumericLiteral().Text != "2" || source[inner.Right.Pos():inner.Right.End()] != " _\r\n      2" {
		t.Fatal("continued operand lost its source range or value")
	}
	if source[file.EndOfFileToken.Pos():file.EndOfFileToken.End()] != " ' trailing comment" {
		t.Fatal("EOF did not retain trailing trivia")
	}
}

func TestRecoveryKeepsFollowingFunction(t *testing.T) {
	good := "\nPublic Function Good()\nGood = 42\nEnd Function"
	for name, bad := range map[string]string{
		"unknown character":                   "Function Bad()\nx = ?\nEnd Function",
		"invalid UTF8":                        "Function Bad()\nx = \xff\nEnd Function",
		"unterminated string":                 "Function Bad()\nx = \"unfinished\nBad = 1\nEnd Function",
		"missing expression":                  "Function Bad()\nx =\nBad = 1\nEnd Function",
		"missing assignment operator":         "Function Bad()\nLet x 1\nEnd Function",
		"missing parenthesis":                 "Function Bad()\nx = (1 + 2\nEnd Function",
		"missing condition":                   "Function Bad()\nIf Then Bad = 1\nEnd Function",
		"missing Then":                        "Function Bad()\nIf x\nBad = 1\nEnd If\nEnd Function",
		"missing End If":                      "Function Bad()\nIf x Then\nBad = 1\nEnd Function",
		"missing End Function":                "Function Bad()\nBad = 1",
		"wrong terminator":                    "Function Bad()\nEnd Sub",
		"missing function name":               "Function ()\nEnd Function",
		"missing closing parameter delimiter": "Function Bad(\nBad = 1\nEnd Function",
		"malformed parameters":                "Function Bad(ByVal x As)\nEnd Function",
		"malformed return type":               "Function Bad() As\nBad = 1\nEnd Function",
		"malformed declaration":               "Function Bad()\nDim x As\nBad = 1\nEnd Function",
		"missing member name":                 "Function Bad()\nLet . = 1\nEnd Function",
		"missing index delimiter":             "Function Bad()\nx(1 = 2\nEnd Function",
		"stray Else":                          "Function Bad()\nElse\nBad = 1\nEnd Function",
		"stray End If":                        "Function Bad()\nEnd If\nBad = 1\nEnd Function",
		"missing goto target":                 "Function Bad()\nGoTo\nEnd Function",
		"missing numeric target":              "Function Bad()\nGoTo -\nEnd Function",
		"unsupported nested inline If":        "Function Bad()\nIf True Then If False Then Bad = 1\nEnd Function",
		"module garbage":                      "~ garbage ?\nEnd If\n",
	} {
		t.Run(name, func(t *testing.T) {
			file := parser.ParseSourceFile(bad + good).AsSourceFile()
			if len(file.ParseDiagnostics) == 0 {
				t.Fatal("malformed input had no diagnostics")
			}
			validateRanges(t, file)
			binder.BindSourceFile(file)
			symbol := file.Locals.Lookup("Good")
			if symbol == nil || symbol.ValueDeclaration.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[0].AsLetStatement().RExpression.AsNumericLiteral().Text != "42" {
				t.Fatal("following valid function did not parse and bind")
			}
			if file.Locals.Lookup("") != nil || file.ClassifiableNames.Has("") {
				t.Fatal("a missing identifier became a symbol")
			}
			if symbol.ValueDeclaration.Flags&ast.NodeFlagsThisNodeOrAnySubNodesHasError != 0 {
				t.Fatal("parse error flags leaked into the following function")
			}
		})
	}
}

func TestRecoveryAtColonAndNestedTerminators(t *testing.T) {
	for _, source := range []string{
		"Function F()\nx = : F = 2\nEnd Function",
		"Function F()\n? junk: F = 2\nEnd Function",
		"Function F()\nIf True Then x = Else F = 2\nEnd Function",
		"Function F()\nIf True Then\nx =\nElse\nF = 2\nEnd If\nEnd Function",
		"Function F()\nIf True Then\nx =\nElseIf False Then\nx = 1\nElse\nF = 2\nEnd If\nEnd Function",
	} {
		t.Run(source, func(t *testing.T) {
			file := parser.ParseSourceFile(source).AsSourceFile()
			if len(file.ParseDiagnostics) == 0 {
				t.Fatal("expected parse diagnostics")
			}
			validateRanges(t, file)
			binder.BindSourceFile(file)
			found := false
			var visit func(*ast.Node) bool
			visit = func(node *ast.Node) bool {
				if node.Kind == ast.KindLetStatement {
					stmt := node.AsLetStatement()
					if stmt.RExpression.Kind == ast.KindNumericLiteral && stmt.RExpression.AsNumericLiteral().Text == "2" && stmt.LExpression.Symbol == file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().ResultSymbol {
						found = true
					}
				}
				node.ForEachChild(visit)
				return false
			}
			visit(file.AsNode())
			if !found {
				t.Fatal("recovery discarded the valid assignment")
			}
		})
	}
}

func TestPreciseDiagnosticsAndMissingNodes(t *testing.T) {
	for _, test := range []struct {
		source, marker string
		code           ast.DiagnosticCode
	}{
		{"Function F()\n  x = ~\nEnd Function", "~", ast.DiagnosticInvalidCharacter},
		{"Function F()\n x = \"partial\r\nEnd Function", "\"partial", ast.DiagnosticUnterminatedString},
		{"Function F()\nx =\nEnd Function", "\nEnd Function", ast.DiagnosticExpressionExpected},
		{"Function F()\nIf x F = 1\nEnd Function", "F = 1", ast.DiagnosticExpectedToken},
	} {
		t.Run(test.marker, func(t *testing.T) {
			file := parser.ParseSourceFile(test.source).AsSourceFile()
			if len(file.ParseDiagnostics) == 0 {
				t.Fatal("expected diagnostic")
			}
			diagnostic := file.ParseDiagnostics[0]
			if diagnostic.Pos() != strings.Index(test.source, test.marker) || diagnostic.Code != test.code {
				t.Fatalf("diagnostic = %+v", diagnostic)
			}
			if test.code == ast.DiagnosticInvalidCharacter && diagnostic.End()-diagnostic.Pos() != 1 {
				t.Fatal("invalid-character diagnostic includes trivia")
			}
			if test.code == ast.DiagnosticUnterminatedString && test.source[diagnostic.Pos():diagnostic.End()] != test.marker {
				t.Fatal("unterminated string span includes the newline")
			}
			validateRanges(t, file)
		})
	}
	source := "Function F()\nx =\nEnd Function"
	file := parser.ParseSourceFile(source).AsSourceFile()
	missing := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[0].AsLetStatement().RExpression
	if missing.Kind != ast.KindIdentifier || missing.AsIdentifier().Text != "" || missing.Pos() != missing.End() || missing.Flags&ast.NodeFlagsThisNodeHasError == 0 {
		t.Fatal("missing expression is not a zero-width error node")
	}
	binder.BindSourceFile(file)
	if missing.Symbol != nil || len(file.BindDiagnostics) != 0 {
		t.Fatal("binding a missing expression introduced a bogus symbol or diagnostic")
	}
	file = parser.ParseSourceFile("Function F()\n  ").AsSourceFile()
	diagnostic := file.ParseDiagnostics[len(file.ParseDiagnostics)-1]
	if diagnostic.Pos() != len(file.Text) || diagnostic.End() != len(file.Text) {
		t.Fatal("missing terminator at EOF has an incorrect span")
	}
}

func TestValidCommentsLineEndingsAndContinuations(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n", "\r", "\u2028", "\u2029"} {
		source := strings.Join([]string{"' comment with ~ \" _", "Function F() ' header", "Rem End Function", "F = 1 ' assignment", "End Function", "Rem tail"}, ending)
		file := parser.ParseSourceFile(source).AsSourceFile()
		if len(file.ParseDiagnostics) != 0 {
			t.Fatalf("line ending %q: %+v", ending, file.ParseDiagnostics)
		}
		validateRanges(t, file)
	}
	for _, ending := range []string{"\n", "\r\n"} {
		source := "Function F()" + ending + "F = 1 + _ \t" + ending + "  2" + ending + "End _" + ending + " Function"
		file := parser.ParseSourceFile(source).AsSourceFile()
		if len(file.ParseDiagnostics) != 0 {
			t.Fatalf("continued source: %+v", file.ParseDiagnostics)
		}
		validateRanges(t, file)
		binder.BindSourceFile(file)
	}
}

func TestEmptyAndTriviaOnlySourceRanges(t *testing.T) {
	for _, source := range []string{"", " \t", "' comment", "Rem comment", "\ufeff", "\ufeff\r\n  ' comment\u2028"} {
		file := parser.ParseSourceFile(source).AsSourceFile()
		if len(file.ParseDiagnostics) != 0 || len(file.FunctionDeclarations.Nodes) != 0 {
			t.Fatalf("invalid trivia-only file %q", source)
		}
		validateRanges(t, file)
	}
}

func TestBindingDiagnosticSourceRanges(t *testing.T) {
	source := "Function F()\n    Other = 1\nEnd Function\nFunction Other()\nEnd Function\nFunction f()\nEnd Function"
	file := parser.ParseSourceFile(source).AsSourceFile()
	if len(file.ParseDiagnostics) != 0 {
		t.Fatal(file.ParseDiagnostics)
	}
	binder.BindSourceFile(file)
	if len(file.BindDiagnostics) != 2 {
		t.Fatalf("unexpected binding diagnostics: %+v", file.BindDiagnostics)
	}
	for _, diagnostic := range file.BindDiagnostics {
		if !diagnostic.Loc.IsValid() || diagnostic.Loc.End() > len(source) {
			t.Fatal("binding diagnostic has an invalid range")
		}
		switch diagnostic.Code {
		case ast.BindingDuplicateDeclaration:
			if source[diagnostic.Loc.Pos():diagnostic.Loc.End()] != "f" || source[diagnostic.RelatedLoc.Pos():diagnostic.RelatedLoc.End()] != "F" {
				t.Fatal("duplicate procedure ranges do not underline their names")
			}
		case ast.BindingInvalidAssignmentTarget:
			if source[diagnostic.Loc.Pos():diagnostic.Loc.End()] != "Other" || diagnostic.RelatedLoc.IsValid() {
				t.Fatal("invalid assignment range includes leading trivia or a bogus related span")
			}
		default:
			t.Fatal("unexpected binding diagnostic")
		}
	}
}

func TestParenthesizedConstantsStillBind(t *testing.T) {
	file := parser.ParseSourceFile("Function F()\nIf (Not (False)) Then F = 1 Else F = 2\nEnd Function").AsSourceFile()
	if len(file.ParseDiagnostics) != 0 {
		t.Fatal(file.ParseDiagnostics)
	}
	binder.BindSourceFile(file)
	conditional := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[0].AsIfStatement()
	if conditional.ThenStatement.Flags&ast.NodeFlagsUnreachable != 0 || conditional.ElseStatement.Flags&ast.NodeFlagsUnreachable == 0 {
		t.Fatal("parentheses changed constant-condition reachability")
	}
}

func TestSkippedStatementDoesNotMarkFollowingStatement(t *testing.T) {
	file := parser.ParseSourceFile("Function F()\nStop\nF = 2\nEnd Function").AsSourceFile()
	if len(file.ParseDiagnostics) == 0 {
		t.Fatal("unsupported declaration was not diagnosed")
	}
	body := file.FunctionDeclarations.Nodes[0].AsFunctionDeclaration().Body
	statement := body.AsBlock().Statements.Nodes[0]
	if statement.Flags&ast.NodeFlagsThisNodeOrAnySubNodesHasError != 0 || body.Flags&ast.NodeFlagsThisNodeOrAnySubNodesHasError == 0 {
		t.Fatal("skipped text's error was not isolated to the containing block")
	}
}

func TestDeepSyntaxProducesDiagnosticAndRecovers(t *testing.T) {
	for _, expression := range []string{strings.Repeat("(", 1000) + "1" + strings.Repeat(")", 1000), strings.Repeat("-", 1000) + "1", strings.Repeat("1 ^ ", 1000) + "1"} {
		file := parser.ParseSourceFile("Function Bad()\nx = " + expression + "\nEnd Function\nFunction Good()\nGood = 1\nEnd Function").AsSourceFile()
		found := false
		for _, diagnostic := range file.ParseDiagnostics {
			if diagnostic.Code == ast.DiagnosticNestingTooDeep {
				found = true
			}
		}
		if !found {
			t.Fatal("deep syntax was not diagnosed")
		}
		validateRanges(t, file)
		binder.BindSourceFile(file)
		if file.Locals.Lookup("Good") == nil {
			t.Fatal("nesting recovery lost the following function")
		}
	}
}

func FuzzParseAndBindRecovery(f *testing.F) {
	f.Add("Sub Run()\n On Pick() GoTo First, 100, First\nFirst: On 0 GoTo 100\n100: Exit Sub\nEnd Sub")
	f.Add("Sub Run()\n On GoTo ,\n On 1 GoTo\n On Error GoTo\nEnd Sub\nSub Later()\nEnd Sub")
	f.Add("Sub Run(ByRef s As String)\n Mid$(s,1,2)=Mid(s,2)\n Mid (s,1)=\"x\"\nEnd Sub")
	f.Add("Sub Run()\n Mid$(s,1,)=\nEnd Sub\nSub Later()\nEnd Sub")
	for _, source := range []string{"", "Function F()\nF = 1\nEnd Function", "Function ()\nLet =\nGoTo -\nEnd Function", "Function F(\nIf x Then\nElse\nEnd Function", "Function F()\nx = \"unterminated\nEnd Function", "~ é\xff", "Function F()\nElse\nEnd If\nEnd Function", "Function F()\n10 Again: GoTo Again\nEnd Function",
		"Option Explicit\nConst Size = 2\nSub S(ByVal arg As Long)\nDim value(0 To Size) As Long\nEnd Sub",
		"Dim x = Later\nFunction Later()\nEnd Function",
		"Function F(Optional x As String = \"hello\", ParamArray y() As Variant) As Long\nEnd Function",
		"Dim As ,\nSub S(ByVal , Optional =)\nConst x =\nEnd Sub",
		"Const Pi = 3.14, Mask = &HFF, Broken = 1E+\nSub S()\nEnd Sub",
		"Sub S()\nCall obj.Update(, amount:=Make(1))\nobj!End(2) = 3\nEnd Sub",
		"Sub S()\nS (x), y\nCall S(x, ,)\nobj.[Column Name] = F()\nEnd Sub",
		"Sub S()\nCall obj.(x:=,\nIf True Then obj.Update Else Call S\nEnd Sub",
		"Sub S()\nFor i = 1 To 3\nDo Until done\nIf i Then Exit Do\nLoop\nNext i\nEnd Sub",
		"Sub S()\nSelect Case x\nCase 1 To 3, Is > 10\nGoTo Done\nCase Else\nExit Sub\nEnd Select\nDone: End\nEnd Sub",
		"Function F()\nGoTo Inside\nWhile False\nInside: F = 1\nWend\nEnd Function",
		"Sub S()\nFor Each In\nSelect Case\nCase Is,\nDo While\nNext\nEnd Sub",
		"Sub S()\nSet obj = New Library.Widget\nWith obj\nSet .Child = Nothing\nWith .Child\n!Key = 1\nEnd With\nEnd With\nEnd Sub",
		"Sub S()\nWith Make()\nCall .Update(1)\nGoTo Done\nDone: Set .Child = New Widget\nEnd With\nEnd Sub",
		"Sub S()\nWith\nSet . = New\nEnd With\nEnd Sub",
		"Property Get Value() As Variant\nValue = 1\nExit Property\nEnd Property\nProperty Let Value(ByVal rhs As Variant)\nEnd Property",
		"Property Set Item(Optional index = 0, ByVal rhs As Object)\nSet target = rhs\nEnd Property",
		"Property Get (\nWith obj\nEnd Property\nProperty Let Value()\nEnd Property",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 4096 {
			t.Skip()
		}
		file := parser.ParseSourceFile(source).AsSourceFile()
		validateRanges(t, file)
		binder.BindSourceFile(file)
		if !file.IsBound() {
			t.Fatal("recovered source did not bind")
		}
		if len(file.ParseDiagnostics) > len(source)+1 {
			t.Fatal("unbounded diagnostic cascade")
		}
	})
}

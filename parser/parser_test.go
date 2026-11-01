package parser

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/scanner"
)

func TestParse001(t *testing.T) {
	// line number without whitspace
	input := "20:"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseBlockStatement()

	want := f.NewLabeledStatement(
		f.NewLineNumber(
			"20", false,
		),
		nil,
		nil,
	)
	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse002(t *testing.T) {
	// line number with whitespace
	input := "20  :"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseBlockStatement()

	want := f.NewLabeledStatement(
		f.NewLineNumber(
			"20", false,
		),
		nil,
		nil,
	)
	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse003(t *testing.T) {
	// line number with whitespace
	input := "20 SomeLabel :"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseBlockStatement()

	want := f.NewLabeledStatement(
		f.NewLineNumber(
			"20", false,
		),
		f.NewIdentifier("SomeLabel"),
		nil,
	)
	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse004(t *testing.T) {
	// line number with whitespace
	input := "SomeLabel :"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseBlockStatement()

	want := f.NewLabeledStatement(
		nil,
		f.NewIdentifier("SomeLabel"),
		nil,
	)
	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse100(t *testing.T) {
	// LET statement
	input := "let x = 2"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseBlockStatement()

	want := f.NewLetStatement(
		f.NewIdentifier("x"),
		f.NewNumericLiteral("2"),
	)
	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse101(t *testing.T) {
	// LET statement with line continuation
	input := "let x = _\n2"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseBlockStatement()

	want := f.NewLetStatement(
		f.NewIdentifier("x"),
		f.NewNumericLiteral("2"),
	)
	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse200(t *testing.T) {
	// power
	input := "let x = 2 ^ 4 ^ 6"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseBlockStatement()

	want := f.NewLetStatement(
		f.NewIdentifier("x"),
		f.NewBinaryExpression(
			f.NewNumericLiteral("2"),
			f.NewToken(ast.KindPOW),
			f.NewBinaryExpression(
				f.NewNumericLiteral("4"),
				f.NewToken(ast.KindPOW),
				f.NewNumericLiteral("6"),
			),
		),
	)
	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse201(t *testing.T) {
	// unary minus
	input := "let x = - 2 ^ +4"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseBlockStatement()

	want := f.NewLetStatement(
		f.NewIdentifier("x"),
		f.NewPrefixUnaryExpression(
			ast.KindMINUS,
			f.NewBinaryExpression(
				f.NewNumericLiteral("2"),
				f.NewToken(ast.KindPOW),
				f.NewPrefixUnaryExpression(
					ast.KindPLUS,
					f.NewNumericLiteral("4"),
				),
			),
		),
	)
	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse202(t *testing.T) {
	// multiplication and division
	input := "let x = 1 / - 2 ^ +4 * 2"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseBlockStatement()

	want := f.NewLetStatement(
		f.NewIdentifier("x"),
		f.NewBinaryExpression(
			f.NewBinaryExpression(
				f.NewNumericLiteral("1"),
				f.NewToken(ast.KindDIV),
				f.NewPrefixUnaryExpression(
					ast.KindMINUS,
					f.NewBinaryExpression(
						f.NewNumericLiteral("2"),
						f.NewToken(ast.KindPOW),
						f.NewPrefixUnaryExpression(
							ast.KindPLUS,
							f.NewNumericLiteral("4"),
						),
					),
				),
			),
			f.NewToken(ast.KindMULT),
			f.NewNumericLiteral("2"),
		),
	)
	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse300(t *testing.T) {
	// function definition
	input := "private function TestFunc()\nlet x = 1: let y = 2\nlet z = 3\nEnd Function"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseFunction()

	want := f.NewFunctionDeclaration(
		ast.ModifierPrivate,
		f.NewIdentifier("TestFunc"),
		f.NewNodeList([]*ast.Node{}),
		f.NewBlock(
			f.NewNodeList(
				[]*ast.Node{
					f.NewLetStatement(
						f.NewIdentifier("x"),
						f.NewNumericLiteral("1"),
					),
					f.NewLetStatement(
						f.NewIdentifier("y"),
						f.NewNumericLiteral("2"),
					),
					f.NewLetStatement(
						f.NewIdentifier("z"),
						f.NewNumericLiteral("3"),
					),
				},
			),
		),
	)

	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParse310(t *testing.T) {
	// multiline if statement
	input :=
		"private function TestFunc()\n" +
			"let x = 1\n" +
			"If x < 2 Then\n" +
			"  let y = 2\n" +
			"Elseif x < 5 Then\n" +
			"  let y = 3\n" +
			"Else\n" +
			"  let y = 4\n" +
			"End If\n" +
			"End Function"

	scanner := &scanner.ScannerState{Input: input}
	parser := &Parser{scanner: scanner}
	parser.Start()

	f := ast.NodeFactory{}

	res := parser.ParseFunction()

	want := f.NewFunctionDeclaration(
		ast.ModifierPrivate,
		f.NewIdentifier("TestFunc"),
		f.NewNodeList([]*ast.Node{}),
		f.NewBlock(
			f.NewNodeList(
				[]*ast.Node{
					f.NewLetStatement(
						f.NewIdentifier("x"),
						f.NewNumericLiteral("1"),
					),
					f.NewIfStatement(
						f.NewBinaryExpression(
							f.NewIdentifier("x"),
							f.NewToken(ast.KindLT),
							f.NewNumericLiteral("2"),
						),
						f.NewBlock(
							f.NewNodeList(
								[]*ast.Node{
									f.NewLetStatement(
										f.NewIdentifier("y"),
										f.NewNumericLiteral("2"),
									),
								},
							),
						),
						f.NewIfStatement(
							f.NewBinaryExpression(
								f.NewIdentifier("x"),
								f.NewToken(ast.KindLT),
								f.NewNumericLiteral("5"),
							),
							f.NewBlock(
								f.NewNodeList(
									[]*ast.Node{
										f.NewLetStatement(
											f.NewIdentifier("y"),
											f.NewNumericLiteral("3"),
										),
									},
								),
							),
							f.NewBlock(
								f.NewNodeList(
									[]*ast.Node{
										f.NewLetStatement(
											f.NewIdentifier("y"),
											f.NewNumericLiteral("4"),
										),
									},
								),
							),
						),
					),
				},
			),
		),
	)

	if !res.DeeplyEquals(want) {
		t.Errorf(`not equal`)
	}
}

func TestParseImplicitLetStatement(t *testing.T) {
	for _, input := range []string{"value = 2 + 3", "Let value = 2 + 3"} {
		t.Run(input, func(t *testing.T) {
			p := &Parser{scanner: &scanner.ScannerState{Input: input}}
			p.Start()
			f := ast.NodeFactory{}
			want := f.NewLetStatement(
				f.NewIdentifier("value"),
				f.NewBinaryExpression(f.NewNumericLiteral("2"), f.NewToken(ast.KindPLUS), f.NewNumericLiteral("3")),
			)
			if !p.ParseBlockStatement().DeeplyEquals(want) {
				t.Fatal("unexpected assignment AST")
			}
			if p.token != ast.KindEndOfFile {
				t.Errorf("trailing token = %d, want EOF", p.token)
			}
		})
	}
}

func TestParseSourceFileMultipleFunctions(t *testing.T) {
	input := "\r\n: Private Function IfValue()\r\n" +
		"IfValue = 1\r\nEnd Function\r\n\r\n" +
		"Public Static Function FunctionValue\r\n" +
		"Let FunctionValue = IfValue + 2\r\nEnd Function: \r\n"
	p := &Parser{scanner: &scanner.ScannerState{Input: input}}
	p.Start()
	got := p.ParseSourceFile()
	f := ast.NodeFactory{}
	want := f.NewSourceFile(f.NewNodeList([]*ast.Node{
		f.NewFunctionDeclaration(ast.ModifierPrivate, f.NewIdentifier("IfValue"), f.NewNodeList(nil),
			f.NewBlock(f.NewNodeList([]*ast.Node{
				f.NewLetStatement(f.NewIdentifier("IfValue"), f.NewNumericLiteral("1")),
			}))),
		f.NewFunctionDeclaration(ast.ModifierPublic|ast.ModifierStatic, f.NewIdentifier("FunctionValue"), f.NewNodeList(nil),
			f.NewBlock(f.NewNodeList([]*ast.Node{
				f.NewLetStatement(f.NewIdentifier("FunctionValue"), f.NewBinaryExpression(
					f.NewIdentifier("IfValue"), f.NewToken(ast.KindPLUS), f.NewNumericLiteral("2"))),
			}))),
	}))
	if !got.DeeplyEquals(want) {
		t.Fatal("unexpected source file AST")
	}
	if p.token != ast.KindEndOfFile {
		t.Errorf("trailing token = %d, want EOF", p.token)
	}
	if got.AsSourceFile().FunctionDeclarations.Loc.End() != len(input) {
		t.Error("source file range does not reach EOF")
	}
}

func TestParseSourceFileEmpty(t *testing.T) {
	for _, input := range []string{"", " \t", "\n\r\n: \n"} {
		t.Run(input, func(t *testing.T) {
			got := ParseSourceFile(input).AsSourceFile()
			if len(got.FunctionDeclarations.Nodes) != 0 {
				t.Fatal("expected an empty source file")
			}
		})
	}
}

func TestParseSourceFileFunctionBoundaries(t *testing.T) {
	for _, input := range []string{
		"Function F()\nEnd Function",
		"Function F\nEnd Function\nFunction G\nEnd Function",
		"Function F(): F = 1: End Function: Function G(): End Function",
		"Function F()\nIf x Then\nF = 1\nEnd If\nEnd Function",
		"Function F()\nIf x Then F = 1 Else F = 2\nEnd Function",
	} {
		t.Run(input, func(t *testing.T) {
			ParseSourceFile(input)
		})
	}
}

func TestParseSourceFileRejectsMalformedFunctions(t *testing.T) {
	for _, input := range []string{
		"Function F()\n",
		"Function F()\nF = 1",
		"Function F()\nEnd Sub",
		"Function F()\nEnd",
		"Function F()\nEnd Function Function G()\nEnd Function",
		"Function F()\nEnd Function\nLet x = 1",
		"Function F()\nEnd Function\nFunction G()\n",
		"Function F()\nF = 1 F = 2\nEnd Function",
	} {
		t.Run(input, func(t *testing.T) {
			file := ParseSourceFile(input).AsSourceFile()
			if len(file.ParseDiagnostics) == 0 {
				t.Error("expected a diagnostic for malformed source")
			}
		})
	}
}

package ast

import "sihlfall.ch/vambas-vai/core"

type DiagnosticCode string

const (
	DiagnosticInvalidCharacter       DiagnosticCode = "invalid-character"
	DiagnosticUnterminatedString     DiagnosticCode = "unterminated-string"
	DiagnosticExpectedToken          DiagnosticCode = "expected-token"
	DiagnosticIdentifierExpected     DiagnosticCode = "identifier-expected"
	DiagnosticExpressionExpected     DiagnosticCode = "expression-expected"
	DiagnosticStatementExpected      DiagnosticCode = "statement-expected"
	DiagnosticDeclarationExpected    DiagnosticCode = "declaration-expected"
	DiagnosticEndOfStatementExpected DiagnosticCode = "end-of-statement-expected"
	DiagnosticUnsupportedSyntax      DiagnosticCode = "unsupported-syntax"
	DiagnosticNestingTooDeep         DiagnosticCode = "nesting-too-deep"
	DiagnosticInvalidDeclaration     DiagnosticCode = "invalid-declaration"
	DiagnosticInvalidParameter       DiagnosticCode = "invalid-parameter"
	DiagnosticInvalidNumber          DiagnosticCode = "invalid-number"
	DiagnosticInvalidArgument        DiagnosticCode = "invalid-argument"
	DiagnosticInvalidCall            DiagnosticCode = "invalid-call"
	DiagnosticInvalidControlFlow     DiagnosticCode = "invalid-control-flow"
)

// Diagnostic uses half-open UTF-8 byte offsets, as do AST and scanner ranges.
// File is attached after the source file has been constructed.
type Diagnostic struct {
	File    *SourceFile
	Loc     core.TextRange
	Code    DiagnosticCode
	Message string
}

func (diagnostic *Diagnostic) Pos() int { return diagnostic.Loc.Pos() }
func (diagnostic *Diagnostic) End() int { return diagnostic.Loc.End() }

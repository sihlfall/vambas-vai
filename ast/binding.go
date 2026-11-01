package ast

import "sihlfall.ch/vambas-vai/core"

type BindingDiagnosticCode string

const (
	BindingDuplicateDeclaration       BindingDiagnosticCode = "duplicate-declaration"
	BindingDuplicateLabel             BindingDiagnosticCode = "duplicate-label"
	BindingUndefinedLabel             BindingDiagnosticCode = "undefined-label"
	BindingInvalidAssignmentTarget    BindingDiagnosticCode = "invalid-assignment-target"
	BindingUndeclaredName             BindingDiagnosticCode = "undeclared-name"
	BindingAssignmentToConstant       BindingDiagnosticCode = "assignment-to-constant"
	BindingConstantExpressionExpected BindingDiagnosticCode = "constant-expression-expected"
	BindingInvalidValueReference      BindingDiagnosticCode = "invalid-value-reference"
	BindingInvalidExit                BindingDiagnosticCode = "invalid-exit"
	BindingInvalidLoopVariable        BindingDiagnosticCode = "invalid-loop-variable"
	BindingWithBlockExpected          BindingDiagnosticCode = "with-block-expected"
	BindingAmbiguousName              BindingDiagnosticCode = "ambiguous-name"
	BindingMemberNotFound             BindingDiagnosticCode = "member-not-found"
	BindingInaccessibleMember         BindingDiagnosticCode = "inaccessible-member"
	BindingInvalidModule              BindingDiagnosticCode = "invalid-module"
	BindingDuplicateModule            BindingDiagnosticCode = "duplicate-module"
	BindingDuplicateFile              BindingDiagnosticCode = "duplicate-file"
)

type BindingDiagnostic struct {
	File        *SourceFile
	RelatedFile *SourceFile
	Loc         core.TextRange
	RelatedLoc  core.TextRange
	Code        BindingDiagnosticCode
	Message     string
	Node        *Node
	RelatedNode *Node
}

// IsBound and BindOnce follow typescript-go's source file binding lifecycle.
func (file *SourceFile) IsBound() bool { return file.isBound.Load() }

func (file *SourceFile) BindOnce(bind func()) {
	file.bindOnce.Do(func() {
		bind()
		file.isBound.Store(true)
	})
}

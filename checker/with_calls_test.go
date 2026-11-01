package checker_test

import (
	"testing"

	"sihlfall.ch/vambas-vai/ast"
	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
)

func TestQualifiedSubWithImplicitWithArgument(t *testing.T) {
	p := program(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Option Explicit
Type Context
 matcherStack As ArrayBuffer.Ty
 capturesStack As ArrayBuffer.Ty
 qstack As ArrayBuffer.Ty
 nProperCapturePoints As Long
 nCapturePoints As Long
 master As Long
 capturesRequireCoW As Boolean
 qTop As Long
End Type
Sub Run(ByRef context As Context, ByVal nProperCapturePoints As Long, ByVal nCapturePoints As Long)
 With context
  .matcherStack.Length = 0
  .capturesStack.Length = 0
  .qstack.Length = 0
  .nProperCapturePoints = nProperCapturePoints
  .nCapturePoints = nCapturePoints
  ArrayBuffer.AppendFill .capturesStack, nCapturePoints, -1
  .master = -1
  .capturesRequireCoW = False
  .qTop = 0
 End With
End Sub
`},
		compiler.SourceFileInput{FileName: "ArrayBuffer.bas", Text: `Option Explicit
Public Type Ty
 Buffer() As Long
 Capacity As Long
 Length As Long
End Type
Public Sub AppendFill(ByRef lab As Ty, ByVal cnt As Long, ByVal v As Long)
End Sub
`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	file := p.Module("Main")
	with := procedureBody(p, "Main", "Run")[0]
	call := find(t, file, ast.KindCallExpression, "ArrayBuffer.AppendFill .capturesStack, nCapturePoints, -1")
	r := c.GetCallResolution(call)
	if r == nil || !r.Valid || r.Kind != checker.CallProcedure || r.Signature.Declaration.Kind != ast.KindSubDeclaration || len(r.Arguments) != 3 {
		t.Fatal(r)
	}
	first := r.Arguments[0]
	if first.Mode != checker.ArgumentReference || first.Storage.WithTarget != with || first.Storage.Symbol.Name != "capturesStack" || first.Parameter.Name != "lab" {
		t.Fatal("With field did not retain ByRef storage", first)
	}
	if call.AsCallExpression().Expression.AsPropertyAccessExpression().Expression.AsIdentifier().Text != "ArrayBuffer" {
		t.Fatal("callee lost its module qualifier")
	}
	count := 0
	for flow := file.Locals.Lookup("Run").ValueDeclaration.AsSubDeclaration().EndFlowNode; flow != nil; flow = flow.Antecedent {
		if flow.Flags&ast.FlowFlagsCall != 0 {
			count++
			if flow.Node != call {
				t.Fatal("spurious callee invocation", flow)
			}
		}
	}
	if count != 1 {
		t.Fatal("Sub was invoked more than once", count)
	}
}

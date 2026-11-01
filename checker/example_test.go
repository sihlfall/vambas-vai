package checker_test

import (
	"fmt"

	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
)

func Example() {
	p := compiler.NewProgram(compiler.ProgramOptions{
		ProjectName: "Demo",
		Sources: []compiler.SourceFileInput{
			{FileName: "Main.bas", Text: `Option Explicit
Sub Run(ByRef result As Long)
result = Utilities.Add(1, 2)
End Sub`},
			{FileName: "Utilities.bas", Text: `Public Function Add(ByVal a As Long, ByVal b As Long) As Long
Add = a + b
End Function`},
		},
	})
	c := p.GetTypeChecker()
	procedure := p.Module("Main").FunctionDeclarations.Nodes[0]
	assignment := procedure.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[0]
	call := assignment.AsLetStatement().RExpression
	fmt.Println(c.GetTypeAtLocation(call))
	fmt.Println(c.GetSymbolAtLocation(call.AsCallExpression().Expression).Name)
	signature := c.GetResolvedSignature(call)
	fmt.Println(len(signature.Parameters), signature.ReturnType)
	fmt.Println(len(p.GetSemanticDiagnostics(nil)))
	// Output:
	// Long
	// Add
	// 2 Long
	// 0
}

func ExampleChecker_GetCallResolution() {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{
		{FileName: "Main.bas", Text: `Sub Run(n As Long)
Call Target(value:=n)
End Sub
Sub Target(ByRef value As Long)
End Sub`},
	}})
	call := p.Module("Main").Locals.Lookup("Run").ValueDeclaration.AsFunctionDeclaration().Body.AsBlock().Statements.Nodes[0].AsCallStatement().Expression
	resolution := p.GetTypeChecker().GetCallResolution(call)
	argument := resolution.Arguments[0]
	fmt.Println(resolution.Valid, resolution.Deferred)
	fmt.Println(argument.Parameter.Name, argument.Mode == checker.ArgumentReference)
	// Output:
	// true false
	// value true
}

package compiler_test

import (
	"fmt"

	"sihlfall.ch/vambas-vai/compiler"
)

func ExampleNewProgram() {
	p := compiler.NewProgram(compiler.ProgramOptions{
		ProjectName: "Demo",
		Sources: []compiler.SourceFileInput{
			{FileName: "Main.bas", Text: `Option Explicit
Sub Run(ByRef result As Long)
    result = Utilities.Answer()
End Sub`},
			{FileName: "Utilities.bas", Text: `Public Function Answer() As Long
    Answer = 42
End Function`},
		},
	})
	fmt.Println(p.Name(), len(p.SourceFiles()))
	fmt.Println(p.Module("utilities").Locals.Lookup("Answer").Name)
	fmt.Println(len(p.ParseDiagnostics()), len(p.BindDiagnostics()))
	// Output:
	// Demo 2
	// Answer
	// 0 0
}

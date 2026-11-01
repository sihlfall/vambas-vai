package lowering_test

import (
	"fmt"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
)

func ExampleLower() {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{
		FileName: "Main.bas",
		Text:     "Function Bump(ByRef value As Long) As Long\n value = value + 1\n Bump = value\nEnd Function\n",
	}}})
	semantic, diagnostics := lowering.Lower(p)
	if len(diagnostics) != 0 {
		panic(diagnostics)
	}
	f := semantic.Procedures[0]
	fmt.Println(f.Name, f.ReturnType)
	fmt.Println("ByRef:", f.Slots[f.Parameters[0]-1].ByRef)
	fmt.Println("MayRaise:", f.Effects&ir.MayRaise != 0)
	fmt.Println("Verifier diagnostics:", len(ir.Verify(semantic)))
	// For the complete instruction listing: fmt.Print(ir.Format(semantic)).
	// Output:
	// Main.Bump Long
	// ByRef: true
	// MayRaise: true
	// Verifier diagnostics: 0
}

package checker_test

import (
	"sync"
	"testing"

	"sihlfall.ch/vambas-vai/checker"
	"sihlfall.ch/vambas-vai/compiler"
)

func TestNumericConstantQueriesAreImmutable(t *testing.T) {
	p := program(t, compiler.SourceFileInput{FileName: "Main.bas", Text: `Private Const A As Long=&HFFFF&
Private Const B As Integer=3.5
Private Const C As Boolean=1
Private Const D As Double=A/2
Function Run(Optional ByVal n As Long=B+38) As Long
 Run=n
End Function`})
	c := p.GetTypeChecker()
	noErrors(t, c)
	expectations := map[string]float64{"A": 65535, "B": 4, "C": -1, "D": 32767.5}
	parameter := c.GetSignatureFromDeclaration(p.Module("Main").FunctionDeclarations.Nodes[0]).Parameters[0]
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for pass := 0; pass < 50; pass++ {
				for name, want := range expectations {
					if value, ok := c.GetNumericConstantValue(p.Module("Main").Locals.Lookup(name)); !ok || value != want {
						t.Errorf("%s=%v,%v", name, value, ok)
					}
				}
				if value, ok := c.GetNumericConstantExpression(parameter.Default); !ok || value != 42 {
					t.Errorf("default=%v,%v", value, ok)
				}
			}
		}()
	}
	wg.Wait()
}

func TestInvalidNumericConstants(t *testing.T) {
	for _, text := range []string{
		"Const A As Integer=32768", "Const A As Integer=32767+1", "Const A As Long=2147483648#", "Const A As Single=1e100", "Const A As Long=1\\0", "Dim n As Long\nConst A As Long=n",
	} {
		p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: text}}})
		found := false
		for _, d := range p.GetTypeChecker().GetDiagnostics(nil) {
			if d.Code == checker.DiagnosticConstantValue {
				found = true
			}
		}
		if !found {
			t.Fatal("invalid numeric constant accepted", text)
		}
	}
}

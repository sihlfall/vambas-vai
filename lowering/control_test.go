package lowering_test

import (
	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
	"strings"
	"testing"
)

func TestControlRegionsAndScalarOracle(t *testing.T) {
	p := lower(t, module(`Function Run() As Long
 Dim i As Long,n As Long
 For i=1 To 4
  If i=3 Then GoTo after
  n=n+i
 Next
after: Do Until n>=5
  n=n+1
 Loop
 While n<7
  n=n+1
 Wend
 Run=n+i
End Function`))
	value, failed := execute(t, p, 1, nil)
	if failed || value != 10 {
		t.Fatal(value, failed, ir.Format(p))
	}
	branches, jumps := 0, 0
	for _, s := range p.Procedures[0].Statements {
		if s.End == ir.Branch {
			branches++
		}
		if s.End == ir.Jump {
			jumps++
		}
	}
	if branches < 4 || jumps < 4 {
		t.Fatal(ir.Format(p))
	}
}

func TestDeclarationsRemainProcedureScopedAcrossBranches(t *testing.T) {
	p := lower(t, module(`Function Run() As Long
 GoTo inside
 If False Then
  Dim n As Long,a(2) As Long
inside: a(0)=7
  n=a(0)
 End If
 Run=n
End Function`))
	f := p.Procedures[0]
	if len(f.Slots) != 3 || f.Slots[1].Name != "n" || f.Slots[2].Type.Scalar != ir.ArrayLong {
		t.Fatal(ir.Format(p))
	}
	if f.Statements[0].Instructions[0].Op != ir.OpAddress {
		t.Fatal("fixed array was not initialized in the entry region")
	}
}

func TestForEachScopesAndUnsupportedEntries(t *testing.T) {
	p := lower(t, module(`Sub Run(ByRef a() As Long)
 Dim v As Variant
 For Each v In a
  GoTo done
 Next
done: ReDim a(2)
End Sub`))
	f := p.Procedures[0]
	if len(f.Scopes) != 1 || len(f.Scopes[0].Cleanup) != 1 || f.Slots[f.Scopes[0].Cleanup[0]-1].Type.Scalar != ir.Enumerator {
		t.Fatal(ir.Format(p))
	}
	bad := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{module(`Sub Run(ByRef a() As Long)
 Dim v As Variant
 GoTo inside
 For Each v In a
inside: v=1
 Next
End Sub`)}})
	semantic, diagnostics := lowering.Lower(bad)
	if semantic != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "GoTo entering an uninitialized resource") {
		t.Fatal(semantic, diagnostics)
	}
}

func TestSelectAndOnGotoScalarOracle(t *testing.T) {
	p := lower(t, module(`Function Run(ByVal n As Long) As Long
 Dim original As Long
 original=n
 Select Case n
 Case Mutate(n), 1
  Run=10
 Case 2, 2, Is = 3
  Run=20
 Case 4 To 6
  Run=30
 Case >= 7
  Run=40
 Case Else
  Run=50
 End Select
 On original GoTo first, second
 Run=Run+100
 Exit Function
first: Run=Run+1
 Exit Function
second: Run=Run+2
End Function
Function Mutate(ByRef n As Long) As Long
 n=99
 Mutate=0
End Function`))
	for _, test := range []struct{ n, want int64 }{{0, 110}, {1, 11}, {2, 22}, {3, 120}, {5, 130}, {7, 140}} {
		value, failed := execute(t, p, 1, []argument{{value: test.n}})
		if failed || value != test.want {
			t.Fatal(test, value, failed, ir.Format(p))
		}
	}
	for _, n := range []int64{-1, 256, 32768} {
		if _, failed := execute(t, p, 1, []argument{{value: n}}); !failed {
			t.Fatal("invalid computed index accepted", n)
		}
	}
}

func TestComputedGotoCannotEnterEnumeration(t *testing.T) {
	p := compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{module(`Sub Run()
 Dim a(1) As Long,v As Variant
 On 1 GoTo inside
 For Each v In a
inside: v=1
 Next
End Sub`)}})
	semantic, diagnostics := lowering.Lower(p)
	if semantic != nil || len(diagnostics) == 0 || !strings.Contains(diagnostics[0].Message, "GoTo entering an uninitialized resource") {
		t.Fatal(semantic, diagnostics)
	}
}

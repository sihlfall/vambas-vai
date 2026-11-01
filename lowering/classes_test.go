package lowering_test

import (
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/compiler"
	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/lowering"
)

func TestClassIRDeclarationsPrecedeBodies(t *testing.T) {
	p := lower(t,
		compiler.SourceFileInput{FileName: "First.cls", Text: `Private count As Long
Public Function Forward(ByVal other As Second) As First
 Set Forward = other.Back(Me)
End Function
Public Sub Increment()
 count = count + 1
End Sub`},
		compiler.SourceFileInput{FileName: "Second.cls", Text: `Public Function Back(ByVal first As First) As First
 first.Increment
 Set Back = first
End Function`})
	if len(p.Classes) != 2 || len(p.Globals) != 0 || len(p.Classes[0].Fields) != 1 || len(p.Classes[0].Members) != 2 {
		t.Fatal(ir.Format(p))
	}
	for _, proc := range p.Procedures {
		if proc.Class == 0 || proc.Receiver == 0 || proc.Parameters[0] != proc.Receiver || proc.Slots[proc.Receiver-1].ByRef {
			t.Fatal("instance receiver was not declared", ir.Format(p))
		}
	}
}

func TestDeferredClassFeaturesAreDiagnosed(t *testing.T) {
	for _, tc := range []struct{ name, main, class, want string }{
		{"unsupported field", "", "Private large As LongLong", "type LongLong"},
		{"fixed string", "", "Private text As String * 5", "fixed-length"},
		{"initializer signature", "", "Private Sub Class_Initialize(ByVal n As Long)\nEnd Sub", "parameterless Subs"},
		{"terminator signature", "", "Private Function Class_Terminate() As Long\nEnd Function", "parameterless Subs"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, ds := lowering.Lower(compiler.NewProgram(compiler.ProgramOptions{Sources: []compiler.SourceFileInput{{FileName: "Main.bas", Text: tc.main}, {FileName: "C.cls", Text: tc.class}}}))
			if p != nil || len(ds) == 0 {
				t.Fatal("unsupported class feature became executable")
			}
			for _, d := range ds {
				if strings.Contains(d.Message, tc.want) {
					return
				}
			}
			t.Fatalf("wanted %q: %+v", tc.want, ds)
		})
	}
}

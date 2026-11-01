package llvm_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/llvm"
)

func TestNativeSelectCase(t *testing.T) {
	tests := []struct {
		name, source string
		want         int
	}{
		{"integral dispatch and duplicate cases", `Function Run() As Long
 Dim i As Long
 For i=-1 To 9
  Run=Run+Pick(i)
 Next
End Function
Function Pick(ByVal n As Long) As Long
 Select Case n
 Case 0, 1: Pick=3
 Case Is = 2: Pick=7
 Case 2, 3: Pick=11
 Case 4: Pick=13
 Case 5: Pick=17
 Case 6: Pick=19
 Case 7: Pick=23
 Case 8: Pick=29
 Case Else: Pick=31
 End Select
End Function`, 187},
		{"selector evaluated once and captured", `Function Run() As Long
 Dim n As Long,calls As Long
 n=3
 Select Case Capture(n,calls)
 Case Change(n,calls), 1
  Run=-1
 Case 3
  Run=n+calls*100
 Case Else
  Run=-2
 End Select
End Function
Function Capture(ByRef n As Long,ByRef calls As Long) As Long
 calls=calls+1
 Capture=n
End Function
Function Change(ByRef n As Long,ByRef calls As Long) As Long
 calls=calls+1
 n=8
 Change=2
End Function`, 208},
		{"range bounds eager alternatives short circuit", `Function Run() As Long
 Dim trace As Long
 Select Case 5
 Case Mark(trace,1,6) To Mark(trace,2,10), Mark(trace,3,5), Mark(trace,4,5)
  Run=trace
 Case Mark(trace,5,5)
  Run=-1
 Case Else
  Run=-2
 End Select
End Function
Function Mark(ByRef trace As Long,ByVal digit As Long,ByVal value As Long) As Long
 trace=trace*10+digit
 Mark=value
End Function`, 123},
		{"ranges comparisons nested no fallthrough", `Function Run() As Long
 Dim i As Long
 For i=-2 To 5
  Select Case i
  Case Is < 0: Run=Run+1
  Case 0 To 2
   Select Case i
   Case Is <> 1: Run=Run+10
   Case Else: Run=Run+100
   End Select
  Case 5 To 3: Run=-999
  Case >= 3: Run=Run+1000
  End Select
 Next
End Function`, 3122},
		{"no match empty and else only", `Function Run() As Long
 Select Case 1
 End Select
 Select Case 2
 Case 3: Run=-99
 End Select
 Select Case 4
 Case Else: Run=42
 End Select
End Function`, 42},
		{"select true", `Function Run() As Long
 Dim n As Long
 n=5
 Select Case True
 Case n<3: Run=1
 Case n=5: Run=42
 Case Else: Run=3
 End Select
End Function`, 42},
		{"mutable string snapshot", `Function Run() As Long
 Dim s As String
 s="abc"
 Select Case s
 Case Change(s): Run=-1
 Case "abb" To "abd"
  If s="xbc" Then Run=42
 Case Else: Run=-2
 End Select
End Function
Function Change(ByRef s As String) As String
 Mid$(s,1,1)="x"
 Change="zzz"
End Function`, 42},
		{"variant snapshot and declared comparisons", `Function Run() As Long
 Dim v As Variant
 v=3
 Select Case v
 Case Change(v): Run=-1
 Case "3": Run=Len(v)+40
 Case Else: Run=-2
 End Select
End Function
Function Change(ByRef v As Variant) As Variant
 v="xx"
 Change=2
End Function`, 42},
		{"typed byref variant snapshot", `Function Run() As Long
 Dim s As String
 s="abc"
 Run=Pick(s)
 If s<>"changed" Then Run=-1
End Function
Function Pick(ByRef v As Variant) As Long
 Select Case v
 Case Change(v): Pick=-1
 Case "abc": Pick=42
 Case Else: Pick=-2
 End Select
End Function
Function Change(ByRef v As Variant) As Variant
 v="changed"
 Change="no match"
End Function`, 42},
		{"null selector skips case expressions", `Function Run() As Long
 Dim calls As Long,v As Variant
 v=Null
 Select Case v
 Case Mark(calls): Run=-1
 Case Else: Run=42+calls
 End Select
End Function
Function Mark(ByRef calls As Long) As Long
 calls=calls+1
 Mark=1
End Function`, 42},
		{"null range bound and mixed booleans", `Function Run() As Long
 Dim trace As Long
 Select Case 3
 Case Null To Mark(trace): Run=-1
 Case Is >= 3: Run=trace+40
 End Select
 Select Case -1
 Case True: Run=Run+1
 End Select
End Function
Function Mark(ByRef trace As Long) As Long
 trace=trace+1
 Mark=5
End Function`, 42},
		{"goto into and out of cases", `Function Run() As Long
 Dim n As Long
 GoTo inside
 Select Case Mark(n)
 Case 1
inside: n=n+7
  GoTo done
 Case Else: n=-1
 End Select
done: Run=n
End Function
Function Mark(ByRef n As Long) As Long
 n=99
 Mark=1
End Function`, 7},
		{"select inside enumeration exit for", `Function Run() As Long
 Dim a() As Long,v As Variant
 ReDim a(2)
 a(0)=3
 For Each v In a
  Select Case v
  Case 3: Exit For
  Case Else: Run=-1
  End Select
 Next
 ReDim a(4)
 Run=UBound(a)+38
End Function`, 42},
		{"integer selector long cases and enums", `Enum Choices
 First=1
 Last=40000
End Enum
Function Run() As Long
 Dim i As Integer
 i=1
 Select Case i
 Case Last: Run=-1
 Case First: Run=42
 End Select
End Function`, 42},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, test.want) })
	}
}

func TestNativeOnGoto(t *testing.T) {
	for _, test := range []struct {
		name, source string
		want         int
	}{
		{"indices fallthrough and mixed labels", `Function Run() As Long
 Run=Pick(0)+Pick(1)*10+Pick(2)*100+Pick(3)*1000+Pick(4)*10000+Pick(255)*100000
End Function
Function Pick(ByVal n As Long) As Long
 On n GoTo One, 100, One
 Pick=4
 Exit Function
One: Pick=1
 Exit Function
100: Pick=2
End Function`, 441214},
		{"index evaluated once backward branch", `Function Run() As Long
 Dim n As Long
again: On Advance(n) GoTo again, finish
 Run=-1
 Exit Function
finish: Run=n+40
End Function
Function Advance(ByRef n As Long) As Long
 n=n+1
 Advance=n
End Function`, 42},
		{"variant and string rounding", `Function Run() As Long
 Dim v As Variant
 v="1.5"
 On v GoTo bad, first
bad: Run=-1
 Exit Function
first: Run=21
 On "2.5" GoTo bad, second
 Run=-2
 Exit Function
second: Run=Run*2
End Function`, 42},
		{"enumeration scope cleanup default retained", `Function Run() As Long
 Dim a() As Long,v As Variant,n As Long
 ReDim a(2)
 a(0)=3
 For Each v In a
  On 0 GoTo done
  n=n+1
  On n GoTo done
 Next
done: ReDim a(4)
 Run=UBound(a)+n+37
End Function`, 42},
	} {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, test.want) })
	}
}

func TestNativeOnGotoLastReachableDestination(t *testing.T) {
	// VBA permits long destination lists, but valid indices reach only the
	// first 255. Still parse and bind the remaining destination normally.
	source := "Function Run() As Long\n On 255 GoTo " + strings.Repeat("bad,", 254) + "done,bad\n"
	source += "bad: Run=-1\n Exit Function\ndone: Run=42\nEnd Function"
	variantResult(t, source, 42)
}

// Require a genuine code jump table for a suitable fixture on a fixed target.
// Opaque calls prevent LLVM from replacing the whole dispatch by a data lookup.
// The compiler emits switches or comparison chains; no density policy or
// jump-table code exists. LLVM must also fold arithmetic before recognizing the
// dispatch in the comparison-chain fixture.
func TestLLVMChoosesJumpTables(t *testing.T) {
	for _, test := range []struct {
		name                 string
		computed, arithmetic bool
	}{
		{"select literals", false, false}, {"select arithmetic", false, true}, {"computed goto", true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var source strings.Builder
			source.WriteString("Sub Dispatch(ByVal n As Long)\n")
			if test.computed {
				source.WriteString("On n GoTo L0,L1,L2,L3,L4,L5,L6,L7\nExit Sub\n")
			} else {
				source.WriteString("Select Case n\n")
			}
			for i := 0; i < 8; i++ {
				if test.computed {
					fmt.Fprintf(&source, "L%d: ", i)
				} else if test.arithmetic {
					fmt.Fprintf(&source, "Case %d+1: ", i-1)
				} else {
					fmt.Fprintf(&source, "Case %d: ", i)
				}
				fmt.Fprintf(&source, "Call F%d()\nExit Sub\n", i)
			}
			if !test.computed {
				source.WriteString("End Select\n")
			}
			source.WriteString("End Sub\n")
			for i := 0; i < 8; i++ {
				fmt.Fprintf(&source, "Sub F%d()\nEnd Sub\n", i)
			}
			text := emit(t, lower(t, source.String()), llvm.Options{})
			if !test.arithmetic && !strings.Contains(text, "switch i") {
				t.Fatal("integral dispatch was not emitted as LLVM switch")
			}
			if test.arithmetic && strings.Contains(text, "switch i") {
				t.Fatal("arithmetic fixture must exercise LLVM's comparison-chain simplification")
			}
			// Give the backend opaque callees using the private ABI. This tests
			// switch lowering independently of interprocedural simplification.
			start := strings.Index(text, "define internal i32 @vba.p2(")
			if start < 0 {
				t.Fatal("missing fixture helpers")
			}
			text = text[:start]
			text = strings.Replace(text, "define internal i32 @vba.p1(", "define i32 @vba.p1(", 1)
			for i := 2; i <= 9; i++ {
				text += fmt.Sprintf("declare i32 @vba.p%d(ptr)\n", i)
			}
			dir := t.TempDir()
			input, optimized, assembly := filepath.Join(dir, "dispatch.ll"), filepath.Join(dir, "optimized.ll"), filepath.Join(dir, "dispatch.s")
			if err := os.WriteFile(input, []byte(text), 0600); err != nil {
				t.Fatal(err)
			}
			command(t, tool(t, "opt"), "-passes=default<O2>,verify", "-S", input, "-o", optimized)
			command(t, tool(t, "llc"), "-mtriple=x86_64-unknown-linux-gnu", "-O2", optimized, "-o", assembly)
			data, err := os.ReadFile(assembly)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), ".LJTI") {
				t.Fatalf("LLVM did not select a code jump table:\n%s", data)
			}
		})
	}
}

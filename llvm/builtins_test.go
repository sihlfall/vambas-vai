package llvm_test

import (
	"fmt"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/ir"
	"sihlfall.ch/vambas-vai/llvm"
)

func TestNativeIntegerDivisionAndHex(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"literal widths", `Function Run() As Long
 If &HFFFF<>-1 Then Exit Function
 If &H8000<>-32768 Then Exit Function
 If &HFFFF&<>65535 Then Exit Function
 If &HFFFFFFFF<>-1& Then Exit Function
 If &H80000000<>-2147483648 Then Exit Function
 If &O177777<>-1 Then Exit Function
 If VarType(&H7FFF)<>2 Or VarType(&H10000)<>3 Then Exit Function
 Select Case &HFFFF
 Case &HFFFF: Run=42
 End Select
End Function`},
		{"rounding signs and bounds", `Function Run() As Long
 Dim n As Long
 If 19\4<>4 Or -19\4<>-4 Or 19\-4<>-4 Then Exit Function
 If -19 Mod 4<>-3 Or 19 Mod -4<>3 Then Exit Function
 If 3.5\2.5<>2 Or 5.5 Mod 2.5<>0 Then Exit Function
 If "7.5"\2<>4 Then Exit Function
 If True\True<>1 Then Exit Function
 n=&H80000000
 If n Mod -1<>0 Then Exit Function
 If &H8000 Mod -1<>0 Then Exit Function
 Run=42
End Function`},
		{"variant promotion null and subtypes", `Function Run() As Long
 Dim a As Variant,b As Variant
 a=&H8000:b=-1
 a=a\b
 If a<>32768 Then Exit Function
 If VarType(a)<>3 Then Exit Function
 a=&H80000000
 a=a\b
 If a<>2147483648# Then Exit Function
 If VarType(a)<>5 Then Exit Function
 a=3.5:b=2.5
 If a\b<>2 Then Exit Function
 If a Mod b<>0 Then Exit Function
 a=Null\0
 If Not IsNull(a) Then Exit Function
 a=Null Mod 0
 If IsNull(a) Then Run=42
End Function`},
	} {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, 42) })
	}
}

func TestNativeStringBuiltins(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"ascw and binary comparison", `Function Run() As Long
 If AscW("A")<>65 Or VBA.AscW(ChrW(&HFFFF&))<>-1 Then Exit Function
 If AscW(123)<>49 Then Exit Function
 If StrComp("A","a",vbBinaryCompare)<>-1 Then Exit Function
 If VarType(StrComp("x","x"))<>2 Then Exit Function
 If StrComp("a" & ChrW(0),"a")<>1 Then Exit Function
 If Not IsNull(StrComp(Null,"")) Then Exit Function
 If VBA.vbBinaryCompare=0 Then Run=42
End Function`},
		{"instr overloads defaults named and null", `Function Run() As Long
 If InStr("abcabc","bc")<>2 Then Exit Function
 If InStr(3,"abcabc","bc",vbBinaryCompare)<>5 Then Exit Function
 If InStr(String1:="abc",String2:="b")<>2 Then Exit Function
 If InStr(,"abc","c")<>3 Then Exit Function
 If InStr(5,"abc","")<>5 Then Exit Function
 If InStr("","")<>0 Then Exit Function
 If InStr("a" & ChrW(0) & "b",ChrW(0))<>2 Then Exit Function
 If VarType(InStr("a","a"))<>3 Then Exit Function
 If IsNull(InStr(Null,"x")) Then Run=42
End Function`},
		{"left and string repeat", `Function Run() As Long
 Dim s As String
 s=Left$("abcd",2)
 If s<>"ab" Or Left$("abc",99)<>"abc" Or Left$("abc",0)<>"" Then Exit Function
 If Not IsNull(Left(Null,1)) Then Exit Function
 If Not IsNull(String(Null,"x")) Or Not IsNull(String(2,Null)) Then Exit Function
 If String(3,"abc")<>"aaa" Then Exit Function
 If String$(2,321)<>"AA" Then Exit Function
 If String(2,ChrW(0))<>ChrW(0) & ChrW(0) Then Exit Function
 If AscW(String(1,128))<>&H20AC Then Exit Function
 If Left$(ChrW(&HD83D&) & ChrW(&HDE00&),1)<>ChrW(&HD83D&) Then Exit Function
 Run=42
End Function`},
		{"split typed array variant and boundaries", `Function Run() As Long
 Dim a() As String,v As Variant
 a=Split("a,,b,",",")
 If LBound(a)<>0 Or UBound(a)<>3 Then Exit Function
 If a(0)<>"a" Or a(1)<>"" Or a(2)<>"b" Or a(3)<>"" Then Exit Function
 a=Split("a,b,c",",",2,vbBinaryCompare)
 If UBound(a)<>1 Or a(1)<>"b,c" Then Exit Function
 a=Split("a b")
 If a(1)<>"b" Then Exit Function
 a=Split("abc","")
 If UBound(a)<>0 Or a(0)<>"abc" Then Exit Function
 a=Split("")
 If UBound(a)<>-1 Then Exit Function
 a=Split("abc",,0)
 If UBound(a)<>-1 Then Exit Function
 v=Split("a" & ChrW(0) & "b",ChrW(0))
 If VarType(v)<>8200 Then Exit Function
 If v(1)="b" Then Run=42
End Function`},
		{"iif eager values snapshots arrays and null", `Function Run() As Long
 Dim n As Long,s As String,v As Variant
 s="old"
 v=IIf(True,s,Mutate(s,n))
 If v<>"old" Then Exit Function
 If s<>"new" Or n<>1 Then Exit Function
 v=IIf(False,Mark(n,2),Mark(n,3))
 If n<>123 Then Exit Function
 If v<>3 Then Exit Function
 v=IIf(Null,1,42)
 If v<>42 Then Exit Function
 v=IIf(True,Split("x,y",","),Null)
 If v(1)<>"y" Then Exit Function
 If IsNull(IIf(True,Null,0)) Then Run=42
End Function
Function Mutate(ByRef s As String,ByRef n As Long) As Long
 s="new":n=n+1
End Function
Function Mark(ByRef n As Long,ByVal digit As Long) As Long
 n=n*10+digit:Mark=digit
End Function`},
	} {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, 42) })
	}
}

func TestNativeBuiltinErrorsPreserveWrites(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
	}{
		{"integer zero", `observed=0\0`, 11},
		{"long zero", `observed=1&\0`, 11},
		{"mod zero", `observed=1 Mod 0`, 11},
		{"integer overflow", `observed=&H8000\-1`, 6},
		{"long overflow", `observed=&H80000000\-1`, 6},
		{"rounded divisor zero", `observed=1\.5`, 11},
		{"rounded operand overflow", `observed=1\32767.5`, 6},
		{"variant zero", `Dim v As Variant
 v=0:observed=v\0`, 11},
		{"ascw empty", `observed=AscW("")`, 5},
		{"ascw null", `observed=AscW(Null)`, 94},
		{"compare null", `observed=StrComp("a","b",Null)`, 94},
		{"search start zero", `observed=InStr(0,"abc","a")`, 5},
		{"search start null", `observed=InStr(Null,"abc","a")`, 94},
		{"left negative", `Dim s As String
 s=Left$("abc",-1)`, 5},
		{"left null", `Dim s As String
 s=Left$(Null,1)`, 94},
		{"string negative", `Dim v As Variant
 v=String(-1,"a")`, 5},
		{"string empty character", `Dim v As Variant
 v=String(2,"")`, 5},
		{"string dollar null", `Dim s As String
 s=String$(2,Null)`, 94},
		{"split null", `Dim a() As String
 a=Split(Null)`, 94},
		{"split invalid limit", `Dim a() As String
 a=Split("a",",",-2)`, 5},
		{"iif unselected failure", `Dim v As Variant
 v=IIf(True,1,Fail(observed))`, 11},
	}
	var source, main strings.Builder
	main.WriteString(`
define i32 @main() {
 %e=alloca %vba.error
 %n=alloca i32
`)
	for i, test := range tests {
		fmt.Fprintf(&source, "Sub Probe%d(ByRef observed As Long)\n observed=7\n%s\n observed=99\nEnd Sub\n", i, test.body)
		fmt.Fprintf(&main, ` store i32 0,ptr %%n
 %%s%d=call i32 @vba.p%d(ptr %%e,ptr %%n)
 %%n%d=load i32,ptr %%n
 %%a%d=icmp eq i32 %%s%d,%d
 %%b%d=icmp eq i32 %%n%d,7
 %%ok%d=and i1 %%a%d,%%b%d
`, i, i+1, i, i, i, test.status, i, i, i, i, i)
		if i > 0 {
			fmt.Fprintf(&main, " %%all%d=and i1 %%all%d,%%ok%d\n", i, i-1, i)
		} else {
			main.WriteString(" %all0=and i1 true,%ok0\n")
		}
	}
	source.WriteString(`Function Fail(ByRef observed As Long) As Long
 observed=7
 Fail=1\0
End Function`)
	fmt.Fprintf(&main, " %%exit=select i1 %%all%d,i32 0,i32 1\n ret i32 %%exit\n}\n", len(tests)-1)
	native(t, trackedStrings(emit(t, lower(t, source.String()), llvm.Options{})+main.String()))
}

func TestBuiltinIRContracts(t *testing.T) {
	p := lower(t, `Function Run() As Long
 Dim v As Variant,a() As String
 v=StrComp("a","b")
 a=Split("a,b",",")
 Run=AscW("A")+(10\2)+(10 Mod 3)
End Function`)
	for _, proc := range p.Procedures {
		for _, statement := range proc.Statements {
			for i := range statement.Instructions {
				instruction := &statement.Instructions[i]
				if instruction.Op != ir.OpLibraryCall && instruction.Op != ir.OpIntegerDivide && instruction.Op != ir.OpModulo {
					continue
				}
				saved := *instruction
				instruction.Args = nil
				if len(ir.Verify(p)) == 0 {
					t.Fatalf("%s accepted missing arguments", saved.Op)
				}
				*instruction = saved
				instruction.Type.Scalar = ir.Double
				if len(ir.Verify(p)) == 0 {
					t.Fatalf("%s accepted wrong output type", saved.Op)
				}
				*instruction = saved
				instruction.Effects = 0
				if len(ir.Verify(p)) == 0 {
					t.Fatalf("%s lost fallibility", saved.Op)
				}
				*instruction = saved
			}
		}
	}
}

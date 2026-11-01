package llvm_test

import (
	"fmt"
	"strings"
	"testing"

	"sihlfall.ch/vambas-vai/llvm"
)

func TestNativeFloating(t *testing.T) {
	for _, test := range []struct {
		name, source string
		want         int
	}{
		{"literals arithmetic and calls", `Function Run() As Long
 Dim s As Single,d As Double
 s=1.5!
 d=1.25D+1
 s=s*2!+.5!
 d=ScaleFloat(d)-s
 Run=d*2#
End Function
Function ScaleFloat(ByVal n As Double) As Double
 ScaleFloat=n/2#
End Function`, 6},
		{"single precision and mixed promotion", `Function Run() As Long
 Dim s As Single,d As Double
 s=16777216!
 s=s+1
 d=16777216#+1
 If s=16777216! And d=16777217# Then Run=40
 Run=Run+VarType(1&+1!)-VarType(1+1!)+1
End Function`, 42},
		{"single literal avoids double rounding", `Function Run() As Long
 Dim s As Single
 s=1.0000000596046448!
 If s>1! Then Run=42
End Function`, 42},
		{"bankers rounding assignments and intrinsics", `Function Run() As Long
 Dim i As Integer,n As Long,d As Double
 d=2.5
 i=d
 n=3.5
 Run=i+n+CInt(-3.5)+CLng(-2.5)+42
End Function`, 42},
		{"division effective types", `Function Run() As Long
 Dim s As Single,d As Double
 s=3!/2!
 d=3/2
 If s=1.5 And d=1.5 Then Run=40
 Run=Run+VarType(3/2)-VarType(3!/2!)+1
End Function`, 42},
		{"floating for while select and conditions", `Function Run() As Long
 Dim s As Single,d As Double,n As Long
 For s=0! To 1! Step .25!
  n=n+1
 Next
 For d=1# To 0# Step -.25#
  n=n+1
 Next
 While d<.25#
  d=d+.25#
 Wend
 Select Case s
 Case 1.2 To 1.3
  If d Then Run=n+32
 Case Else: Run=-1
 End Select
End Function`, 42},
		{"coercions and nonzero truth", `Function Run() As Long
 Dim s As Single,d As Double
 s=CSng("1.25")
 d=CDbl("2.5")
 If CBool(s) Then Run=CLng((s+d)*8#)+12
 If CBool(-0#) Then Run=-1
End Function`, 42},
		{"computed goto rounds floating index", `Function Run() As Long
 On 2.5 GoTo bad,done
bad: Run=-1
 Exit Function
done: Run=42
End Function`, 42},
		{"byref float and variant typed aliases", `Function Run() As Long
 Dim s As Single,d As Double
 s=1.5!:d=2.5#
 Change s,d
 Run=CLng(s*10!+d*10#)
End Function
Sub Change(ByRef s As Variant,ByRef d As Variant)
 s=s+.2!
 d=d
End Sub`, 42},
		{"variant subtypes and single promotion", `Function Run() As Long
 Dim v As Variant,w As Variant
 v=1.5!:w=2.5#
 v=v+1
 If VarType(v)<>4 Then Exit Function
 v=v+1&
 If VarType(v)<>5 Then Exit Function
 v=3e38!
 v=v*2
 If VarType(v)<>5 Then Exit Function
 If v>3e38# Then Run=42
End Function`, 42},
		{"arrays byref preserve variant and enumeration", `Function Run() As Long
 Dim a() As Single,b(1) As Double,v As Variant,item As Variant,total As Double
 ReDim a(1)
 a(0)=1.5!:a(1)=2.5!
 Fill a,b
 ReDim Preserve a(2)
 a(2)=3.5!
 v=a
 For Each item In v
  If VarType(item)<>4 Then Exit Function
  total=total+item
 Next
 For Each item In b
  If VarType(item)<>5 Then Exit Function
  total=total+item
 Next
 Run=CLng(total*2#)
End Function
Sub Fill(ByRef a() As Single,ByRef b() As Double)
 a(1)=3!
 b(0)=6#:b(1)=7#
End Sub`, 42},
		{"variant division and null", `Function Run() As Long
 Dim v As Variant,w As Variant
 v=3!:w=2!
 v=v/w
 If VarType(v)<>4 Then Exit Function
 w=3&:v=w/2
 If VarType(v)<>5 Then Exit Function
 v=Null/0
 If IsNull(v) Then Run=42
End Function`, 42},
		{"underflow signed zero and logical conversion", `Function Run() As Long
 Dim s As Single,d As Double
 s=1e-30!:s=s*1e-30!
 d=-0#
 If s=0! And d=0# Then Run=40
 Run=Run+(2.5 And 3.5)
End Function`, 40},
	} {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, test.want) })
	}
}

func TestNativeFloatingErrors(t *testing.T) {
	for _, test := range []struct {
		name, body string
		status     int
	}{
		{"single overflow", "Dim s As Single\n s=3e38!\n s=s*2!", 6},
		{"double overflow", "Dim d As Double\n d=1e308\n d=d*2#", 6},
		{"single narrowing overflow", "Dim s As Single\n s=1e100", 6},
		{"integer narrowing overflow", "Dim i As Integer\n i=32767.5", 6},
		{"long narrowing overflow", "observed=2147483647.5", 6},
		{"nonzero division by zero", "Dim s As Single\n s=1!/0!", 11},
		{"zero division by zero", "Dim s As Single\n s=0!/0!", 6},
		{"variant nonzero division by zero", "Dim v As Variant\n v=1\n v=v/0", 11},
		{"variant zero division by zero", "Dim v As Variant\n v=0\n v=v/0", 6},
		{"variant floating zero over empty", "Dim v As Variant,w As Variant\n v=0!\n v=v/w", 11},
		{"byref variant coercion failure", "Dim s As Single\n s=1!\n Fail s", 6},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "Sub Run(ByRef observed As Long)\n observed=7\n" + test.body + "\n observed=99\nEnd Sub\nSub Fail(ByRef v As Variant)\n v=1e100\nEnd Sub"
			text := emit(t, lower(t, source), llvm.Options{})
			text += fmt.Sprintf(`
define i32 @main() {
 %%e=alloca %%vba.error
 %%n=alloca i32
 store i32 0,ptr %%n
 %%s=call i32 @vba.p1(ptr %%e,ptr %%n)
 %%v=load i32,ptr %%n
 %%a=icmp eq i32 %%s,%d
 %%b=icmp eq i32 %%v,7
 %%ok=and i1 %%a,%%b
 %%exit=select i1 %%ok,i32 0,i32 1
 ret i32 %%exit
}`, test.status)
			native(t, trackedStrings(text))
		})
	}
}

func TestFloatingDivisionErrorCommitsDouble(t *testing.T) {
	for _, test := range []struct {
		name, expression, initial, kind string
		status                          int
	}{
		{"positive infinity", "one/zero", "1.0", "inf", 11},
		{"nan", "zero/zero", "1.0", "nan", 6},
		{"overflow leaves destination", "big*big", "7.0", "unchanged", 6},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "Sub Run(ByRef dest As Double)\n Dim one As Double,zero As Double,big As Double\n one=1#:big=1e308\n dest=" + test.expression + "\nEnd Sub"
			text := emit(t, lower(t, source), llvm.Options{})
			check := "fcmp oeq double %v, 7.0"
			if test.kind == "nan" {
				check = "fcmp uno double %v, %v"
			}
			if test.kind == "inf" {
				check = "fcmp oeq double %v, 0x7FF0000000000000"
			}
			text += fmt.Sprintf(`
define i32 @main() {
 %%e=alloca %%vba.error
 %%d=alloca double
 store double %s,ptr %%d
 %%s=call i32 @vba.p1(ptr %%e,ptr %%d)
 %%v=load double,ptr %%d
 %%a=icmp eq i32 %%s,%d
 %%b=%s
 %%ok=and i1 %%a,%%b
 %%exit=select i1 %%ok,i32 0,i32 1
 ret i32 %%exit
}`, test.initial, test.status, check)
			native(t, trackedStrings(text))
		})
	}
}

func TestFloatingIRHasNoFastMath(t *testing.T) {
	text := emit(t, lower(t, "Function F(ByVal x As Double) As Double\n F=(x+1#)*2#\nEnd Function"), llvm.Options{})
	if !strings.Contains(text, "fadd double") || !strings.Contains(text, "fmul double") || strings.Contains(text, " fast ") {
		t.Fatal(text)
	}
}

func TestFloatingNonfiniteInputs(t *testing.T) {
	// These values can remain in caller storage after the specified Double
	// division error. Subsequent calls must check before fptosi/comparison.
	for _, expression := range []string{"value", "CLng(value)", "CBool(value)", "value=1#"} {
		source := "Function Run(ByVal value As Double) As Long\n Run=" + expression + "\nEnd Function"
		if expression == "value=1#" {
			source = "Function Run(ByVal value As Double) As Long\n If value=1# Then Run=1\nEnd Function"
		}
		if expression == "CBool(value)" {
			source = "Function Run(ByVal value As Double) As Long\n If CBool(value) Then Run=1\nEnd Function"
		}
		t.Run(expression, func(t *testing.T) {
			text := emit(t, lower(t, source), llvm.Options{})
			text += `
define i32 @main() {
 %e=alloca %vba.error
 %out=alloca i32
 store i32 7,ptr %out
 %s=call i32 @vba.p1(ptr %e,ptr %out,double 0x7FF8000000000000)
 %v=load i32,ptr %out
 %a=icmp eq i32 %s,6
 %b=icmp eq i32 %v,7
 %ok=and i1 %a,%b
 %exit=select i1 %ok,i32 0,i32 1
 ret i32 %exit
}`
			native(t, trackedStrings(text))
		})
	}
}

package llvm_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"sihlfall.ch/vambas-vai/llvm"
)

func failMidAllocationAfterFirst(text string) string {
	text = strings.ReplaceAll(text, "call ptr @malloc(i64", "call ptr @test.mid.allocate(i64")
	return text + `
@test.mid.attempts = internal global i32 0
define internal ptr @test.mid.allocate(i64 %n) noinline {
entry:
  %old = load i32, ptr @test.mid.attempts
  %next = add i32 %old, 1
  store i32 %next, ptr @test.mid.attempts
  %failed = icmp ne i32 %old, 0
  br i1 %failed, label %failure, label %success
failure:
  ret ptr null
success:
  %p = call ptr @malloc(i64 %n)
  ret ptr %p
}
`
}

func TestNativeMidUniqueBufferNeedsNoDetachment(t *testing.T) {
	p := lower(t, `Function Run() As String
    Dim s As String
    s = "abc" & ChrW(100)
    Mid$(s,2,2) = "XY"
    Run = s
End Function
`)
	expectString(t, p, 1, utf16.Encode([]rune("aXYd")), failMidAllocationAfterFirst)
}

func TestNativeMidEvaluationAndAliases(t *testing.T) {
	p := lower(t, `Function Run() As String
    Dim s As String, n As Long
    s = Mid$(Length:=Record(n,2), String:=Make(n), Start:=Record(n,3))
    Run = s & ChrW(n)
End Function
Function Record(ByRef n As Long, ByVal value As Long) As Long
    n = n * 10 + value
    Record = value
End Function
Function Make(ByRef n As Long) As String
    n = n * 10 + 1
    Make = "abcdef"
End Function
`)
	expectString(t, p, 1, []uint16{99, 100, 213}, nil)
	p = lower(t, `Function Run() As String
    Dim s As String
    s = "abcde" & ChrW(102)
    Change s, s
    ChangeCopy s
    ChangeOne (s)
    Run = s
End Function
Sub Change(ByRef a As String, ByRef b As String)
    Mid$(a,2,1) = "X"
    Mid$(b,3,1) = "Y"
End Sub
Sub ChangeCopy(ByVal s As String)
    Mid$(s,4,1) = "Z"
End Sub
Sub ChangeOne(ByRef s As String)
    Mid$(s,1,1) = "Q"
End Sub
`)
	expectString(t, p, 1, utf16.Encode([]rune("aXYdef")), nil)
}

func TestNativeMidErrorsPreserveStorage(t *testing.T) {
	for _, test := range []struct {
		name, body            string
		status, count, origin int
		want                  string
		detachFailure         bool
	}{
		{"read start zero", `s = Mid$(s,0,1)`, 5, 0, 1, "abc", false},
		{"read negative length", `s = Mid(s,1,-1)`, 5, 0, 1, "abc", false},
		{"past-end read negative length", `s = Mid$(s,4,-1)`, 5, 0, 1, "abc", false},
		{"write past end", `Mid$(s,4) = "X"`, 5, 0, 1, "abc", false},
		{"write start zero before RHS", `Mid$(s,0,1) = Bump(count)`, 5, 0, 1, "abc", false},
		{"write negative length", `Mid$(s,1,-1) = Bump(count)`, 5, 0, 1, "abc", false},
		{"past end zero length", `Mid$(s,4,0) = ""`, 5, 0, 1, "abc", false},
		{"empty destination", `s = "": Mid$(s,1) = "x"`, 5, 0, 1, "", false},
		{"RHS failure", `Mid$(s,1,1) = Fail(count)`, 6, 1, 3, "abc", false},
		{"RHS changes destination", `Mid$(s,3,1) = Shrink(s,count)`, 5, 1, 1, "x", false},
		{"detach failure", `s = "ab" & ChrW(99): Mid$(s,1,1) = s`, 14, 0, 1, "abc", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := lower(t, "Sub Run(ByRef s As String, ByRef count As Long)\n "+test.body+"\nEnd Sub\n"+`Function Bump(ByRef count As Long) As String
 count = count + 1
 Bump = "X"
End Function
Function Fail(ByRef count As Long) As String
 Dim limit As Long, owned As String
 count = count + 1
 owned = "temporary" & ChrW(33)
 limit = 2147483647
 limit = limit + 1
 Fail = owned
End Function
Function Shrink(ByRef s As String, ByRef count As Long) As String
 count = count + 1
 s = "x"
 Shrink = "Z"
End Function
`)
			var harness strings.Builder
			fmt.Fprintf(&harness, `@test.initial = private constant [4 x i16] [i16 97, i16 98, i16 99, i16 0]
define i32 @main() {
entry:
  %%error = alloca %%vba.error
  %%slot = alloca %%vba.string
  %%counter = alloca i32
  store %%vba.string {ptr @test.initial, ptr null, i32 3, i16 0}, ptr %%slot
  store i32 0, ptr %%counter
  %%status = call i32 @vba.p1(ptr %%error, ptr %%slot, ptr %%counter)
  %%current = load %%vba.string, ptr %%slot
  %%length = call i32 @vba.str.len(%%vba.string %%current)
  %%counter.value = load i32, ptr %%counter
  %%origin.ptr = getelementptr %%vba.error, ptr %%error, i32 0, i32 1
  %%origin = load i32, ptr %%origin.ptr
  %%c0 = icmp eq i32 %%status, %d
  %%c1 = icmp eq i32 %%counter.value, %d
  %%c2 = icmp eq i32 %%length, %d
  %%c3 = icmp eq i32 %%origin, %d
  %%a0 = and i1 %%c0, %%c1
  %%a1 = and i1 %%a0, %%c2
  %%a2 = and i1 %%a1, %%c3
  br i1 %%a2, label %%contents, label %%failure
failure:
  ret i32 1
contents:
`, test.status, test.count, len(test.want), test.origin)
			previous := "true"
			for i, unit := range []byte(test.want) {
				fmt.Fprintf(&harness, "  %%unit%d = call i16 @vba.str.unit(%%vba.string %%current, i32 %d)\n  %%ok%d = icmp eq i16 %%unit%d, %d\n  %%all%d = and i1 %s, %%ok%d\n", i, i, i, i, unit, i, previous, i)
				previous = fmt.Sprintf("%%all%d", i)
			}
			fmt.Fprintf(&harness, "  call void @vba.str.clear(ptr %%slot)\n  %%exit = select i1 %s, i32 0, i32 1\n  ret i32 %%exit\n}\n", previous)
			text := emit(t, p, llvm.Options{}) + harness.String()
			if test.detachFailure {
				text = failMidAllocationAfterFirst(text)
			}
			native(t, trackedStrings(text))
		})
	}
}

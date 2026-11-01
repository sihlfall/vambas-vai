package llvm_test

import (
	"sihlfall.ch/vambas-vai/llvm"
	"strings"
	"testing"
)

func TestNativeGosubReturn(t *testing.T) {
	p := lower(t, `Sub Run()
 Dim n As Long
 GoSub Work: Check n = 1
 GoSub Work: Check n = 2
 If True Then GoSub Work Else n = 1000
 Check n = 3
 If False Then n = 1000 Else GoSub Work
 Check n = 4
 On 2 GoSub Bad, Work
 Check n = 5
 On 0 GoSub Bad
 On 10 GoSub Bad
 Check n = 5
 GoSub 100
 Check n = 6
 On 1 GoSub 100
 Check n = 7
 Check Recursive(3) = 4
 Check Isolated() = 3
 Abandon
 Exit Sub
Work:
 n = n + 1
 GoSub Nested
 Return
Nested:
 Check Isolated() = 3
 Check CatchPropagation() = 6
 Return
100:
 n = n + 1
 Return
Bad:
 Error 1002
End Sub
Sub Check(ByVal condition As Boolean)
 If Not condition Then Error 1001
End Sub
Function Recursive(ByVal depth As Long) As Long
 GoSub Work
 Exit Function
Work:
 If depth > 0 Then Recursive = Recursive(depth - 1)
 Recursive = Recursive + 1
 Return
End Function
Sub Abandon()
 GoSub Work
 Error 1002
Work:
 Exit Sub
End Sub
Sub Propagate(ByRef n As Long)
 GoSub Work
 Error 1002
Work:
 n = n + 1
 Error 5
End Sub
Function CatchPropagation() As Long
 Dim n As Long
 On Error Resume Next
 Propagate n
 CatchPropagation = n + Err.Number
End Function
Function Isolated() As Long
 On Error Resume Next
 Return
 Isolated = Err.Number
End Function
`)
	text := emit(t, p, llvm.Options{Entry: 1})
	if !strings.Contains(text, "blockaddress(@vba.p1,") || !strings.Contains(text, "indirectbr ptr %gosub.destination") {
		t.Fatal("GoSub did not use pointer dispatch")
	}
	native(t, trackedStrings(text))
}

func TestNativeGosubResourcesAndErrors(t *testing.T) {
	p := lower(t, `Type Record
 Value As Long
End Type
Sub Run()
 Dim values() As Long
 ReDim values(0 To 1)
 values(0) = 2: values(1) = 3
 Dim item As Variant, total As Long
 For Each item In values
  GoSub Accumulate
  GoSub EarlyReturn
 Next item
 Check total = 5
 Dim r As Record
 With r
  .Value = 10
  GoSub ChangeRecord
  Check .Value = 11
 End With
 Dim depth As Long
 GoSub Deep
 Check depth = 40
 GoSub EarlyReturn
 ReDim values(0 To 2)
 Check UBound(values) = 2
 Dim captured As Long
 On Error Resume Next
 Return
 captured = Err.Number
 On Error GoTo 0
 Check captured = 3
 On Error Resume Next
 On -1 GoSub Bad
 captured = Err.Number
 On Error GoTo 0
 Check captured = 5
 On Error GoTo Handler
 GoSub Raises
 Check total = 15
 Exit Sub
Handler:
 Check Err.Number = 5
 Resume Next
Accumulate:
 total = total + CLng(item)
 Return
ChangeRecord:
 r.Value = r.Value + 1
 Return
Deep:
 depth = depth + 1
 If depth < 40 Then GoSub Deep
 Return
EarlyReturn:
 For Each item In values
  Return
 Next item
 Return
Raises:
 Error 5
 total = total + 10
 Return
Bad:
 Error 1002
End Sub
Sub Check(ByVal condition As Boolean)
 If Not condition Then Error 1001
End Sub
`)
	native(t, trackedStrings(emit(t, p, llvm.Options{Entry: 1})))
}

func TestNativeGosubAllocationFailureAndResume(t *testing.T) {
	p := lower(t, `Function Run(ByRef reached As Long) As Long
 On Error Resume Next
 GoSub Work
 Run = Err.Number
 Exit Function
Work:
 reached = reached + 1
 Return
End Function
Function Retry(ByRef reached As Long) As Long
 On Error GoTo Handler
 GoSub Work
 Retry = reached
 Exit Function
Handler:
 Resume
Work:
 reached = reached + 1
 Return
End Function
`)
	text := emit(t, p, llvm.Options{})
	text = strings.ReplaceAll(text, "declare i32 @vba_gosub_push_ptr(ptr, ptr)", "")
	text = strings.ReplaceAll(text, "@vba_gosub_push_ptr(", "@test.push(")
	text += `
@test.fail = internal global i1 true
declare i32 @vba_gosub_push_ptr(ptr, ptr)
define i32 @test.push(ptr %stack, ptr %target) {
entry:
 %fail = load i1, ptr @test.fail
 store i1 false, ptr @test.fail
 br i1 %fail, label %failed, label %push
failed:
 ret i32 7
push:
 %status = call i32 @vba_gosub_push_ptr(ptr %stack, ptr %target)
 ret i32 %status
}
define i32 @main() {
entry:
 %error = alloca %vba.error
 %result = alloca i32
 %hits = alloca i32
 store i32 0, ptr %hits
 %first = call i32 @vba.p1(ptr %error, ptr %result, ptr %hits)
 %code = load i32, ptr %result
 %after = load i32, ptr %hits
 %succeeded = icmp eq i32 %first, 0
 %failed.correctly = icmp eq i32 %code, 7
 %no.branch = icmp eq i32 %after, 0
 %first.check = and i1 %succeeded, %failed.correctly
 %first.ok = and i1 %first.check, %no.branch
 store i1 true, ptr @test.fail
 %second = call i32 @vba.p2(ptr %error, ptr %result, ptr %hits)
 %retry.result = load i32, ptr %result
 %retry.hits = load i32, ptr %hits
 %retried = icmp eq i32 %retry.result, 1
 %once = icmp eq i32 %retry.hits, 1
 %retry.status = icmp eq i32 %second, 0
 %retry.check = and i1 %retried, %once
 %retry.ok = and i1 %retry.check, %retry.status
 %all = and i1 %first.ok, %retry.ok
 %failed = xor i1 %all, true
 %exit = zext i1 %failed to i32
 call void @vba.project.clear()
 ret i32 %exit
}
`
	native(t, trackedStrings(text))
}

func TestGosubRuntimeStackOwnership(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+`
int main(void) {
 VbaGosubStack stack={0};int values[9]={0};void *target=&values[0];
 CHECK(vba_gosub_pop_ptr(&target,&stack)==3 && target==&values[0]);
 fail_next=1;CHECK(vba_gosub_push_ptr(&stack,&values[0])==7 && !stack.count && !stack.entries && live==0);
 for(int i=0;i<8;i++)CHECK(!vba_gosub_push_ptr(&stack,&values[i]));
 void **original=stack.entries;uint64_t capacity=stack.capacity;
 fail_next=1;CHECK(vba_gosub_push_ptr(&stack,&values[8])==7 && stack.count==8 && stack.entries==original && stack.capacity==capacity);
 CHECK(!vba_gosub_push_ptr(&stack,&values[8]) && stack.count==9);
 for(int i=8;i>=0;i--)CHECK(!vba_gosub_pop_ptr(&target,&stack) && target==&values[i]);
 CHECK(vba_gosub_pop_ptr(&target,&stack)==3);
 vba_gosub_clear(&stack);vba_gosub_clear(&stack);
 CHECK(!stack.entries && !stack.count && !stack.capacity && live==0);return 0;
}
`)
}

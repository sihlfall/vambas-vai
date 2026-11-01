package llvm_test

import (
	"sihlfall.ch/vambas-vai/llvm"
	"strings"
	"testing"
)

func TestNativeParamArrayBindings(t *testing.T) {
	p := lower(t, `Option Base 1
Sub Run()
 Dim n As Long, text As String, value As Variant, small As Integer
 n = 10: text = "before": value = "original": small = 1
 Mutate n, text, value, small
 Check n = 11
 Check text = "changed"
 Check value = 42
 Check small = 7
 Twice n, n
 Check n = 14
 Call ReplaceFirst((n))
 Check n = 14
 ReplaceFirst(n)
 Check n = 14
 Call ReplaceFirst(n)
 Check n = 9
 Check Bounds() = -1
 Check Bounds(1, 2, 3) = 2
 Call MissingElements(, 7, )
 Check Sum(40, 1, 1) = 42
 ObserveValue n + 0, Bump(n)
 ObserveReference n, Bump(n)
 Forward n
 Check n = 14
 Recurse 3, n
 Check n = 24
 ForwardArray n
 Check n = 25
 Dim singleValue As Single, doubleValue As Double, truth As Boolean
 singleValue = 1.25: doubleValue = 2.5: truth = True
 Types small, n, singleValue, doubleValue, truth, text, Null, Empty
 Dim a() As Long
 ReDim a(0 To 1)
 a(0) = 4
 Twice a(0), a(0)
 Check a(0) = 7
 ReDim Preserve a(0 To 2)
 Call ReplaceArray((a))
 Check LBound(a) = 0
 Check UBound(a) = 2
 ReplaceArray a
 Check LBound(a) = 4
 Check UBound(a) = 5
 Check a(4) = 31
 Check NestedUse(Touch(a(4)), ResizeAfterReturn(a)) = 32
 Check NestedUse(TouchFixed(a(4)), ResizeAfterReturn(a)) = 33
 Check UBound(a) = 7
 Dim va As Variant
 ReDim va(0 To 0) As Long
 va(0) = 5
 Forward va(0)
 Check va(0) = 8
 Dim snapshot As Variant
 snapshot = CopyValues(n, text)
 n = 100: text = "later"
 Check snapshot(0) = 25
 Check snapshot(1) = "changed"
 Check Sum(0, n, 2, 3) = 105
 Dim reported As Long
 On Error Resume Next
 TooLarge small
 reported = Err.Number
 On Error GoTo 0
 Check reported = 6
 Check small = 7
 Err.Clear
 On Error Resume Next
 FailAfterChange n, text
 reported = Err.Number
 On Error GoTo 0
 Check reported = 5
 Check n = 101
 Check text = "failure survives"
 Err.Clear
 On Error Resume Next
 LockDuringArguments a(4), ResizeLater(a)
 reported = Err.Number
 On Error GoTo 0
 Check reported = 10
 Check a(4) = 33
 Err.Clear
 ReDim Preserve a(4 To 6)
 Check UBound(a) = 6
 Dim i As Long
 For i = 1 To 3
  Check Sum(0, i, 1) = i + 1
 Next i
End Sub
Sub Check(ByVal condition As Boolean)
 If Not condition Then Error 1001
End Sub
Sub Mutate(ParamArray p() As Variant)
 Check LBound(p) = 0
 Check UBound(p) = 3
 p(0) = p(0) + 1
 p(1) = "changed"
 p(2) = 42
 p(3) = 7
End Sub
Sub Twice(ParamArray p() As Variant)
 p(0) = p(0) + 1
 p(1) = p(1) + 2
End Sub
Sub ReplaceFirst(ParamArray p())
 p(0) = 9
End Sub
Function Bounds(ParamArray p() As Variant) As Long
 Check LBound(p) = 0
 Bounds = UBound(p)
End Function
Sub MissingElements(ParamArray p() As Variant)
 Check UBound(p) = 2
 Check IsMissing(p(0))
 Check p(1) = 7
 Check IsMissing(p(2))
 p(0) = "temporary missing"
 Check p(0) = "temporary missing"
End Sub
Function Sum(ByVal initial As Long, ParamArray p() As Variant) As Long
 Dim item As Variant
 For Each item In p
  initial = initial + item
 Next item
 Sum = initial
End Function
Function Bump(ByRef n As Long) As Long
 n = n + 1
 Bump = n
End Function
Sub ObserveValue(ParamArray p() As Variant)
 Check p(0) + 1 = p(1)
End Sub
Sub ObserveReference(ParamArray p() As Variant)
 Check p(0) = p(1)
End Sub
Sub Forward(ParamArray p() As Variant)
 Increment p(0)
 Increment p(0)
 AddOne p(0)
End Sub
Sub Increment(ParamArray p() As Variant)
 p(0) = p(0) + 1
End Sub
Sub AddOne(ByRef item As Variant)
 item = item + 1
End Sub
Sub Recurse(ByVal depth As Long, ParamArray p() As Variant)
 If depth > 0 Then
  Recurse depth - 1, p(0)
 Else
  p(0) = p(0) + 10
 End If
End Sub
Sub ForwardArray(ParamArray p() As Variant)
 WholeArray p
 CheckNested p
End Sub
Sub WholeArray(ByRef p() As Variant)
 p(0) = p(0) + 1
End Sub
Sub CheckNested(ParamArray p() As Variant)
 Check UBound(p) = 0
 Check IsArray(p(0))
End Sub
Sub Types(ParamArray p() As Variant)
 Check VarType(p(0)) = 2
 Check VarType(p(1)) = 3
 Check VarType(p(2)) = 4
 Check VarType(p(3)) = 5
 Check VarType(p(4)) = 11
 Check VarType(p(5)) = 8
 Check IsNull(p(6))
 Check IsEmpty(p(7))
End Sub
Sub ReplaceArray(ParamArray p() As Variant)
 Check UBound(p) = 0
 Check IsArray(p(0))
 Dim replacement(4 To 5) As Long
 replacement(4) = 31
 p(0) = replacement
End Sub
Function CopyValues(ParamArray p() As Variant) As Variant
 CopyValues = p
End Function
Sub TooLarge(ParamArray p() As Variant)
 p(0) = 32768
End Sub
Sub FailAfterChange(ParamArray p() As Variant)
 p(0) = p(0) + 1
 p(1) = "failure survives"
 Error 5
End Sub
Sub LockDuringArguments(ParamArray p() As Variant)
 Error 1002
End Sub
Function Touch(ParamArray p() As Variant) As Long
 p(0) = p(0) + 1
 Touch = p(0)
End Function
Function TouchFixed(ByRef value As Long, ParamArray p() As Variant) As Long
 value = value + 1
 TouchFixed = value
End Function
Function ResizeAfterReturn(ByRef a() As Long) As Long
 ReDim Preserve a(4 To 7)
End Function
Function NestedUse(ByVal first As Long, ByVal second As Long) As Long
 NestedUse = first
End Function
Function ResizeLater(ByRef a() As Long) As Long
 ReDim Preserve a(4 To 8)
End Function
`)
	native(t, trackedStrings(emit(t, p, llvm.Options{Entry: 1})))
}

func TestParamArrayRuntimeOwnershipAndRollback(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+paramArrayRuntimeProbe)
}

const paramArrayRuntimeProbe = `
int main(void) {
 VbaArray *packed=NULL;
 CHECK(!vba_paramarray_pack(&packed,0,NULL));
 CHECK(packed && packed->dimensions==1 && packed->bounds[0].lower==0 && packed->bounds[0].count==0);
 int32_t bound=0;
 CHECK(!vba_array_bound(&bound,&packed,V_VARIANT,1,1) && bound==-1);
 int32_t n=12;VbaVariant view={.vt=V_BYREF|V_I4,.value.pointer=&n};
 uint16_t units[]={97,98,99};VbaString text={.data=units,.length=3};
 VbaVariant owned={0};CHECK(!vba_variant_box_string(&owned,&text));
 VbaParamArgument args[]={{&view,1},{&owned,0}};
 VbaArray *old=packed;int before=live;
 for(int stage=0;stage<3;stage++) {
  fail_after=stage;
  CHECK(vba_paramarray_pack(&packed,2,args)==(stage==2?14:7));
  CHECK(packed==old && live==before && n==12);
 }
 CHECK(!vba_paramarray_pack(&packed,2,args));
 VbaVariant *items=packed->data;
 CHECK(items[0].vt==(V_BYREF|V_I4) && items[0].value.pointer==&n);
 CHECK(items[1].vt==V_BSTR && items[1].value.pointer!=owned.value.pointer);
 VbaVariant next={.vt=V_I2,.value.i2=24};CHECK(!vba_variant_store(items,&next) && n==24);
 VbaArray *forwarded=NULL;
 VbaVariant actual={.vt=V_I4,.value.i4=5};
 VbaVariant wrapper={.vt=V_BYREF|V_VARIANT,.value.pointer=&actual};
 VbaParamArgument more[]={{&wrapper,1},{items,1}};
 CHECK(!vba_paramarray_pack(&forwarded,2,more));
 VbaVariant *forwarded_items=forwarded->data;
 CHECK(forwarded_items[0].vt==(V_BYREF|V_VARIANT) && forwarded_items[0].value.pointer==&actual);
 CHECK(!vba_variant_store(forwarded_items,&next) && actual.vt==V_I2 && actual.value.i2==24);
 CHECK(!vba_variant_store(forwarded_items+1,&next) && n==24);
 VbaArray *copy=NULL;CHECK(!vba_array_copy(&copy,packed));
 CHECK(((VbaVariant*)copy->data)[0].vt==V_I4);
 VbaVariant cycle={.vt=V_BYREF|V_VARIANT};cycle.value.pointer=&cycle;
 VbaParamArgument bad={&cycle,1};old=forwarded;before=live;
 CHECK(vba_paramarray_pack(&forwarded,1,&bad)==13 && forwarded==old && live==before);
 packed->locks++;old=packed;before=live;
 CHECK(vba_paramarray_pack(&packed,0,NULL)==10 && packed==old && live==before);
 packed->locks--;
 vba_array_clear(&forwarded);vba_array_clear(&packed);vba_array_clear(&copy);vba_variant_clear(&owned);
 CHECK(n==24 && live==0);return 0;
}
`

func TestNativeParamArrayFailureAndResume(t *testing.T) {
	p := lower(t, `Function Run(ByRef hits As Long) As Long
 On Error Resume Next
 Target Bump(hits), "argument snapshot"
 Run = Err.Number
End Function
Sub Target(ParamArray values() As Variant)
 If values(0) <> 2 Then Error 1002
End Sub
Function Bump(ByRef hits As Long) As Long
 hits = hits + 1
 Bump = hits
End Function
Function Retry(ByRef hits As Long) As Long
 On Error GoTo Handler
 Target Bump(hits), "retry snapshot"
 Retry = hits
 Exit Function
Handler:
 Resume
End Function
`)
	text := emit(t, p, llvm.Options{})
	text = strings.ReplaceAll(text, "declare i32 @vba_paramarray_pack(ptr, i32, ptr)", "")
	text = strings.ReplaceAll(text, "@vba_paramarray_pack(", "@test.pack(")
	text += `
@test.fail = internal global i1 true
declare i32 @vba_paramarray_pack(ptr, i32, ptr)
define i32 @test.pack(ptr %out, i32 %count, ptr %args) {
entry:
 %fail = load i1, ptr @test.fail
 store i1 false, ptr @test.fail
 br i1 %fail, label %failed, label %pack
failed:
 ret i32 7
pack:
 %status = call i32 @vba_paramarray_pack(ptr %out, i32 %count, ptr %args)
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
 %kept.effects = icmp eq i32 %after, 1
 %first.check = and i1 %succeeded, %failed.correctly
 %first.ok = and i1 %first.check, %kept.effects
 store i1 true, ptr @test.fail
 store i32 0, ptr %hits
 %second = call i32 @vba.p4(ptr %error, ptr %result, ptr %hits)
 %retry.result = load i32, ptr %result
 %retry.hits = load i32, ptr %hits
 %retried = icmp eq i32 %retry.result, 2
 %twice = icmp eq i32 %retry.hits, 2
 %retry.status = icmp eq i32 %second, 0
 %retry.check = and i1 %retried, %twice
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

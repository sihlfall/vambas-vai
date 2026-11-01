package llvm_test

import (
	"sihlfall.ch/vambas-vai/llvm"
	"strings"
	"testing"
)

func TestNativeCollection(t *testing.T) {
	p := lower(t, `Private checks As Long
Private ticks As Long
Sub Run()
 Dim c As New Collection
 Check c.Count = 0
 Dim original As String
 original = "first"
 c.Add original, "A"
 original = "changed"
 c.Add "third", Key:="C"
 c.Add "second", "B", Before:="c"
 c.Add "fourth", After:=3
 Check c.Count = 4
 Check CStr(c(1)) = "first"
 Check CStr(c.Item("a")) = "first"
 Check CStr(c!a) = "first"
 Check CStr(c(2)) = "second"
 Check CStr(c(3)) = "third"
 Check CStr(c(4)) = "fourth"
 ChangeTemporary c(1)
 Check CStr(c(1)) = "first"
 c.Remove "b"
 Check CStr(c(2)) = "third"
 c.Remove 2
 Check c.Count = 2
 Check CStr(c(2)) = "fourth"
 Check ErrorFor(c, 1) = 457
 Check c.Count = 2
 Check ErrorFor(c, 2) = 5
 Check ErrorFor(c, 3) = 5
 Check ErrorFor(c, 4) = 5
 Check ErrorFor(c, 5) = 94
 Dim d As Object
 Set d = c
 d.Add Item:="late", Key:="last"
 Check CLng(d.Count) = 3
 Check CStr(d("LAST")) = "late"
 d.Remove "last"
 Dim item As Variant, text As String
 For Each item In c
  text = text & CStr(item)
 Next item
 Check text = "firstfourth"
 Dim i As Long
 For Each item In c
  i = i + 1
  Set c = Nothing
 Next item
 Check i = 2
 ' The original survives through d, while As New recreates c lazily.
 Check c.Count = 0
 Check CLng(d.Count) = 2
 With d
  Set d = Nothing
  .Add "held by With"
  Check CLng(.Count) = 3
 End With
 Dim values(2 To 3) As Long
 values(2) = 42
 c.Add values
 values(2) = 100
 Dim snapshot As Variant
 snapshot = c(1)
 Check snapshot(2) = 42
 c.Add Empty
 c.Add Null
 Check IsEmpty(c(2))
 Check IsNull(c(3))
 Dim child As New VBA.Collection, objects As New Collection
 child.Add "inside"
 objects.Add child, "child"
 Check objects(1) Is child
 Check objects.Item("child") Is child
 Dim extracted As Collection
 Set extracted = objects("CHILD")
 Check extracted Is child
 Set child = Nothing
 Check CStr(extracted(1)) = "inside"
 Dim eachObject As Object, count As Long
 For Each eachObject In objects
  Check eachObject Is extracted
  count = count + 1
 Next eachObject
 Check count = 1
 Dim typed As Collection
 For Each typed In objects
  Check typed Is extracted
 Next typed
 ClearCopy objects("child")
 ClearAlias objects(1)
 Set typed = objects(1)
 Check typed Is extracted
 objects.Remove "child"
 Check CStr(extracted(1)) = "inside"
 Set c = Nothing
 c.Add 1: c.Add 2: c.Add 3
 Dim total As Long
 For Each item In c
  total = total + CLng(item)
  c.Remove 1
 Next item
 Check total = 6
 Check c.Count = 0
 c.Add 1
 total = 0
 For Each item In c
  total = total + CLng(item)
  If item = 1 Then c.Add 2
 Next item
 Check total = 3
 Set c = Nothing
 c.Add 1
 For Each item In c
  Exit For
 Next item
 c.Remove 1
 Check c.Count = 0
 Dim returned As Collection
 Set returned = BuildCollection()
 Check CStr(returned(1)) = "returned"
 ClearCopy returned
 Check returned.Count = 1
 ClearAlias returned
 Check returned Is Nothing
 c.Add "numeric key", "1"
 c.Add "position"
 Check CStr(c("1")) = "numeric key"
 Check CStr(c(2)) = "position"
 c.Remove 1: c.Remove 1
 c.Add "nul key", "a" & ChrW(0) & "b"
 Check CStr(c("A" & ChrW(0) & "B")) = "nul key"
 c.Remove 1
 c.Add Key:=CStr(Tick()), Item:=Tick()
 Check CLng(c("1")) = 2
 c.Remove 1
 Dim lateCollection As Object
 Set lateCollection = c
 On Error Resume Next
 lateCollection.Remove MissingName:=1
 Check Err.Number = 448
 Err.Clear
 lateCollection.Add
 Check Err.Number = 450
 Err.Clear
 Set lateCollection = Nothing
 lateCollection.Count
 Check Err.Number = 91
 Err.Clear
 On Error GoTo 0
 GoSub Fill
 Check c.Count = 1
 Exit Sub
Fill:
 With c
  .Add "return cleanup"
  Return
 End With
End Sub
Function Tick() As Long
 ticks = ticks + 1
 Tick = ticks
End Function
Function BuildCollection() As Collection
 Dim c As New Collection
 c.Add "returned"
 Set BuildCollection = c
End Function
Sub ClearCopy(ByVal c As Collection)
 Set c = Nothing
End Sub
Sub ClearAlias(ByRef c As Collection)
 Set c = Nothing
End Sub
Sub Check(ByVal condition As Boolean)
 checks = checks + 1
 If Not condition Then Error 100 + checks
End Sub
Sub ChangeTemporary(ByRef value As Variant)
 value = "temporary"
End Sub
Function ErrorFor(ByRef c As Collection, ByVal kind As Long) As Long
 On Error Resume Next
 Select Case kind
 Case 1: c.Add "duplicate", "a"
 Case 2: c.Remove 0
 Case 3: c.Add "bad", Before:=1, After:=1
 Case 4: c.Add "bad", Before:=100
 Case 5: c.Add "bad", Key:=Null
 End Select
 ErrorFor = Err.Number
End Function
`)
	native(t, trackedStrings(emit(t, p, llvm.Options{Entry: 1})))
}
func TestCollectionRuntimeOwnershipAndFailure(t *testing.T) {
	prefix := strings.Split(abiTest, "int main(void)")[0]
	nativeRuntimeC(t, prefix+collectionRuntimeProbe)
}

const collectionRuntimeProbe = `
int main(void) {
 VbaObject *object=NULL;fail_next=1;CHECK(vba_object_new(&object,O_COLLECTION)==7 && !object);
 CHECK(!vba_object_new(&object,O_COLLECTION));
 VbaVariant missing={.vt=V_ERROR,.value.error=(int32_t)0x80020004};
 uint16_t units[]={97,98,99};VbaString text={.data=units,.length=3};
 VbaVariant value={0},key={0};CHECK(!vba_variant_box_string(&value,&text));CHECK(!vba_variant_box_string(&key,&text));
 VbaObjectArg args[]={{"",0,&value},{"",0,&key}};
 int32_t count=0;int before=live;
 for(int stage=0;stage<3;stage++){
  fail_after=stage;
  CHECK(vba_collection_invoke(NULL,object,1,2,args,0)==(stage==1?7:14));
  CHECK(!vba_collection_invoke(&count,object,4,0,NULL,3) && count==0 && live==before);
 }
 CHECK(!vba_collection_invoke(NULL,object,1,2,args,0));
 CHECK(!vba_collection_invoke(&count,object,4,0,NULL,3) && count==1);
 VbaVariant number={.vt=V_I4,.value.i4=1},out={0};VbaObjectArg index={"",0,&number};
 before=live;fail_next=1;CHECK(vba_collection_invoke(&out,object,2,1,&index,12)==14 && out.vt==V_EMPTY && live==before);
 VbaEnumerator it={0};fail_next=1;CHECK(vba_collection_enum_start(&it,object)==7 && !it.initialized);
 CHECK(!vba_collection_enum_start(&it,object) && object->references==2);
 int16_t has=0;fail_next=1;CHECK(vba_enum_next(&has,&it,&out)==14 && out.vt==V_EMPTY);
 CHECK(!vba_enum_next(&has,&it,&out) && has==-1 && out.vt==V_BSTR);
 CHECK(out.value.pointer!=value.value.pointer);vba_variant_clear(&out);
 CHECK(!vba_collection_invoke(NULL,object,3,1,&index,0));
 CHECK(!vba_collection_invoke(&count,object,4,0,NULL,3) && count==0);
 vba_object_clear(&object);CHECK(!vba_enum_next(&has,&it,&out) && !has);
 vba_enum_clear(&it);vba_enum_clear(&it);
 vba_variant_clear(&value);vba_variant_clear(&key);CHECK(live==0);
 (void)missing;return 0;
}
`

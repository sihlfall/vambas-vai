package llvm_test

import (
	"sihlfall.ch/vambas-vai/compiler"
	"testing"
)

func TestNativeStringByteAssignments(t *testing.T) {
	tests := []struct{ name, source string }{
		{"utf16 bytes and independent copies", `Option Base 1
Function Run() As Long
 Dim a() As Byte, b() As Byte, s As String, copy As String
 s="A" & ChrW(4660) & vbNullChar & ChrW(55357) & ChrW(56832)
 a=s
 If LBound(a)<>0 Or UBound(a)<>9 Then Exit Function
 If a(0)<>65 Or a(1)<>0 Or a(2)<>52 Or a(3)<>18 Then Exit Function
 If a(4)<>0 Or a(5)<>0 Or a(6)<>61 Or a(7)<>216 Or a(8)<>0 Or a(9)<>222 Then Exit Function
 copy=a
 If copy<>s Then Exit Function
 b=a
 b(0)=66
 If a(0)<>65 Or copy<>s Then Exit Function
 s="changed"
 a=copy
 If a(0)<>65 Then Exit Function
 Run=42
End Function`},
		{"empty null and unallocated", `Function Run() As Long
 Dim a() As Byte, s As String
 s=a
 If Len(s)<>0 Then Exit Function
 a=""
 If LBound(a)<>0 Or UBound(a)<>-1 Then Exit Function
 s=a
 If Len(s)<>0 Then Exit Function
 a=vbNullString
 If LBound(a)<>0 Or UBound(a)<>-1 Then Exit Function
 Erase a
 s=a
 If LenB(s)<>0 Then Exit Function
 a="X"
 If UBound(a)<>1 Or a(0)<>88 Then Exit Function
 Run=42
End Function`},
		{"nonzero bounds odd count and multidimensional", `Function Run() As Long
 Dim a(3 To 5) As Byte, b(1 To 2, -1 To 0) As Byte, s As String
 a(3)=65
 a(4)=0
 a(5)=255
 s=a
 If s<>"A" Or LenB(s)<>2 Then Exit Function
 b(1,-1)=65
 b(2,-1)=0
 b(1,0)=66
 b(2,0)=0
 s=b
 If s<>"AB" Then Exit Function
 Run=42
End Function`},
		{"variant source target and byref aliases", `Function Run() As Long
 Dim a() As Byte, b() As Byte, v As Variant, s As String
 v="AB"
 a=v
 v=a
 If VarType(v)<>8209 Then Exit Function
 s=v
 If s<>"AB" Then Exit Function
 v(0)=67
 b=v
 If a(0)<>65 Or b(0)<>67 Then Exit Function
 Replace a, a
 s=a
 If s<>"XYZ" Then Exit Function
 ReadBytes a, s
 If s<>"XYZ" Then Exit Function
 Run=42
End Function
Sub Replace(ByRef value As Variant, ByRef alias() As Byte)
 value="XYZ"
 If alias(0)<>88 Then Err.Raise 5
End Sub
Sub ReadBytes(ByRef value As Variant, ByRef result As String)
 result=value
End Sub`},
		{"returns byval and string array elements", `Function Run() As Long
 Dim a() As Byte, s(1) As String
 a=MakeBytes("ABC")
 s(0)=a
 TakeString a, s(1)
 If s(0)<>"ABC" Or s(1)<>"ABC" Then Exit Function
 Run=42
End Function
Function MakeBytes(ByVal s As String) As Byte()
 MakeBytes=s
End Function
Sub TakeString(ByVal s As String, ByRef result As String)
 result=s
End Sub`},
		{"byte numeric storage and aliases", `Function Run() As Long
 Dim a(1) As Byte, x As Byte, y As Byte, v As Variant, d As Double
 x=200
 y=2
 a(0)=x
 a(1)=y
 If x<y Then Exit Function
 If x/y<>100 Or x\y<>100 Or x Mod y<>0 Then Exit Function
 x=x+y
 If CLng(x)<>202 Or VarType(x)<>17 Then Exit Function
 d=x
 If d<>202 Then Exit Function
 ChangeByte x
 v=x
 If VarType(v)<>17 Or CLng(v)<>203 Then Exit Function
 If Not (x>y) Then Exit Function
 Run=42
End Function
Sub ChangeByte(ByRef x As Variant)
 x=x+1
End Sub`},
		{"locks errors and rollback", `Function Run() As Long
 Dim a() As Byte, v As Variant, b As Byte, other As Byte, s As String
 a="A"
 b=200
 On Error Resume Next
 b=256
 If Err.Number<>6 Or b<>200 Then Exit Function
 Err.Clear
 other=100
 b=b+other
 If Err.Number<>6 Or b<>200 Then Exit Function
 Err.Clear
 b=-1
 If Err.Number<>6 Or b<>200 Then Exit Function
 Err.Clear
 v=123
 a=v
 If Err.Number<>13 Or a(0)<>65 Then Exit Function
 Err.Clear
 v=Null
 a=v
 If Err.Number<>13 Or a(0)<>65 Then Exit Function
 Err.Clear
 LockAndReplace a(0), a
 If Err.Number<>10 Or a(0)<>65 Then Exit Function
 Err.Clear
 a="B"
 s=a
 If s<>"B" Then Exit Function
 Run=42
End Function
Sub LockAndReplace(ByRef cell As Byte, ByRef a() As Byte)
 a="ABC"
End Sub`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { variantResult(t, tt.source, 42) })
	}
}

func TestStringByteArrayAllocationRollback(t *testing.T) {
	nativeRuntimeC(t, `
#include <stdlib.h>
#include <stdio.h>
static int live, fail_after=-1;
void *vba_test_malloc(size_t n) {
 if(fail_after>=0 && fail_after--==0) {fail_after=-1;return NULL;}
 void *p=malloc(n);if(p)live++;return p;
}
void vba_test_free(void *p) {if(p)live--;free(p);}
#include "runtime_internal.h"
#include <string.h>
#include <math.h>
#include <limits.h>
#define CHECK(c) do {if(!(c)){fprintf(stderr,"line %d: %s\n",__LINE__,#c);return 1;}} while(0)
int main(void) {
 uint16_t old_units[]={65,66},new_units[]={88,0,0xd83d,0xde00};
 VbaString old_text={.data=old_units,.length=2},new_text={.data=new_units,.length=4};
 VbaVariant source={0},original={0};
 CHECK(!vba_variant_box_string(&original,&old_text));
 CHECK(!vba_variant_box_string(&source,&new_text));
 int baseline=live,success=0,failures=0;
 for(int fail=0;fail<16;fail++) {
  VbaArray *array=NULL;
  CHECK(!vba_array_unbox(&array,&original,V_UI1));
  VbaArray *before=array;
  VbaVariant alias={.vt=V_BYREF|V_ARRAY|V_UI1,.value.pointer=&array};
  fail_after=fail;
  int error=vba_variant_store(&alias,&source);
  fail_after=-1;
  if(error) {
   failures++;
   CHECK(error==7 && array==before && vba_internal_array_count(array)==4);
   CHECK(((unsigned char*)array->data)[0]==65);
  } else {
   success=1;
   CHECK(vba_internal_array_count(array)==8 && ((unsigned char*)array->data)[0]==88);
   CHECK(((unsigned char*)array->data)[5]==0xd8 && ((unsigned char*)array->data)[7]==0xde);
  }
  CHECK(!vba_array_destroy(array));
  CHECK(live==baseline);
  if(success)break;
 }
 CHECK(success && failures>=4);
 VbaArray *array=NULL;
 CHECK(!vba_array_unbox(&array,&source,V_UI1));
 VbaVariant bytes={.vt=V_ARRAY|V_UI1,.value.pointer=array};
 baseline=live;success=0;failures=0;
 for(int fail=0;fail<8;fail++) {
  VbaString out={.length=-1};
  fail_after=fail;
  int error=vba_variant_to(&out,&bytes,V_BSTR);
  fail_after=-1;
  if(error) {failures++;CHECK(error==14 && out.length==-1 && !out.owner && !out.data);}
  else {success=1;CHECK(vba_internal_slen(&out)==4 && vba_internal_unit(&out,3)==0xde00);vba_internal_string_drop(out);}
  CHECK(live==baseline);
  if(success)break;
 }
 CHECK(success && failures>=2);
 /* Locked destinations remain intact, including typed Variant views. */
 array->locks=1;
 VbaVariant alias={.vt=V_BYREF|V_ARRAY|V_UI1,.value.pointer=&array};
 CHECK(vba_variant_store(&alias,&original)==10 && vba_internal_array_count(array)==8);
 array->locks=0;
 vba_variant_clear(&bytes);
 vba_variant_clear(&source);vba_variant_clear(&original);
 CHECK(live==0);
 return 0;
}
`)
}

func TestStringByteArrayFields(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Private Type Buffer
 Count As Byte
 Data() As Byte
End Type
Function Main() As Long
 Dim buffer As Buffer, saved As Buffer, object As Bytes, dynamic As Object, s As String
 buffer.Count=200
 buffer.Data="AB"
 saved=buffer
 buffer.Data(0)=67
 s=saved.Data
 If s<>"AB" Or saved.Count<>200 Then Exit Function
 Set object=New Bytes
 object.Data=saved.Data
 Set dynamic=object
 dynamic.Count=255
 s=dynamic.Data
 If s<>"AB" Then Exit Function
 If dynamic.Count<>255 Then Exit Function
 dynamic.Data="XYZ"
 s=object.Data
 If s<>"XYZ" Then Exit Function
 Main=42
End Function`},
		compiler.SourceFileInput{FileName: "Bytes.cls", Text: `Public Count As Byte
Public Data() As Byte`})
	classResult(t, p, 42)
}

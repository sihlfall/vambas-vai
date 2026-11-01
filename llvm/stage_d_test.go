package llvm_test

import "testing"

func TestNativeStageDByteConversions(t *testing.T) {
	variantResult(t, `Function Run() As Long
 Dim b As Byte, values(1) As Byte, v As Variant
 b=VBA.Conversion.CByte(Expression:=255)
 If VarType(b)<>vbByte Or b<>255 Then Exit Function
 If CByte(0.5)<>0 Or CByte(1.5)<>2 Or CByte(2.5)<>2 Then Exit Function
 If CByte(254.5)<>254 Or CByte(255.49)<>255 Or CByte(-0.5)<>0 Then Exit Function
 If CByte(" 254.5 ")<>254 Or CByte(Empty)<>0 Or CByte(False)<>0 Then Exit Function
 If CByte(True)<>255 Then Exit Function
 b=True
 If b<>255 Then Exit Function
 v=True:b=v
 If b<>255 Then Exit Function
 v=CByte(200)
 If VarType(v)<>VBA.vbByte Then Exit Function
 values(0)=CByte(17)
 values(1)=CByte(31)
 PutByte values(0)
 If values(0)<>255 Or values(1)<>31 Then Exit Function
 PutBoolean values(0)
 If values(0)<>255 Or values(1)<>31 Then Exit Function
 If VarType(ReturnByte())<>vbByte Or ReturnByte()<>254 Then Exit Function
 Run=42
End Function
Sub PutByte(ByRef value As Variant)
 value=CByte(255)
End Sub
Sub PutBoolean(ByRef value As Variant)
 value=True
End Sub
Function ReturnByte() As Byte
 ReturnByte=CByte(254)
End Function`, 42)
}

func TestNativeStageDErrorValues(t *testing.T) {
	variantResult(t, `Function Run() As Long
 Dim v As Variant, copy As Variant, values(1) As Variant, items As New Collection
 v=VBA.Conversion.CVErr(Expression:=2001)
 If VarType(v)<>vbError Or Not VBA.Information.IsError(Arg:=v) Then Exit Function
 If Err.Number<>0 Then Exit Function
 copy=CopyError(v)
 values(0)=copy
 items.Add copy
 v=1
 If Not IsError(copy) Or Not IsError(values(0)) Or Not IsError(items(1)) Then Exit Function
 If CLng(copy)<>2001 Or CStr(items(1))<>"Error 2001" Then Exit Function
 If IsError(Empty) Or IsError(Null) Or IsError(5) Or IsError("Error 5") Then Exit Function
 If IsError(values) Or IsError(items) Then Exit Function
 If CVErr(2001)<>CVErr(2001) Then Exit Function
 If CVErr(2001)>=CVErr(2002) Then Exit Function
 If CVErr(2002)<=CVErr(2001) Then Exit Function
 If CVErr(0)>CVErr(65535) Then Exit Function
 If CByte(CVErr(255))<>255 Or CInt(CVErr(32767))<>32767 Then Exit Function
 If CLng(CVErr(65535))<>65535 Or CDbl(CVErr(65535))<>65535 Then Exit Function
 If CSng(CVErr(65535))<>65535 Or CBool(CVErr(0)) Or Not CBool(CVErr(1)) Then Exit Function
 If CStr(CVErr(0))<>"Error 0" Or CStr(CVErr(65535))<>"Error 65535" Then Exit Function
 If CLng(CVErr(1.5))<>2 Or CLng(CVErr(-0.5))<>0 Then Exit Function
 If CLng(CVErr("65535"))<>65535 Then Exit Function
 If CLng(CVErr(CVErr(2001)))<>2001 Or Not IsError(CVar(copy)) Then Exit Function
 CheckMissing
 CheckPacked CVErr(7), 1
 ReplaceError copy
 If Not IsError(copy) Or CLng(copy)<>12 Then Exit Function
 Run=42
End Function
Function CopyError(ByVal value As Variant) As Variant
 CopyError=value
 value=0
End Function
Sub ReplaceError(ByRef value As Variant)
 value=CVErr(12)
End Sub
Sub CheckMissing(Optional value As Variant)
 If Not IsMissing(value) Or Not IsError(value) Then Err.Raise 5
 If Not IsMissing(CVErr(value)) Then Err.Raise 5
End Sub
Sub CheckPacked(ParamArray values() As Variant)
 If Not IsError(values(0)) Or IsError(values(1)) Then Err.Raise 5
End Sub`, 42)
}

func TestNativeStageDConversionFailuresPreserveStorage(t *testing.T) {
	variantResult(t, `Private ticks As Long
Function Run() As Long
 Dim b As Byte, small As Integer, n As Long, text As String, value As Variant
 b=7:small=8:n=9:text="kept":value="kept"
 On Error Resume Next
 b=CByte(255.5)
 If Err.Number<>6 Or b<>7 Then Exit Function
 Err.Clear
 b=CByte(-0.5001)
 If Err.Number<>6 Or b<>7 Then Exit Function
 Err.Clear
 b=CByte(-1)
 If Err.Number<>6 Or b<>7 Then Exit Function
 Err.Clear
 b=CByte(Null)
 If Err.Number<>94 Or b<>7 Then Exit Function
 Err.Clear
 b=CByte("invalid")
 If Err.Number<>13 Or b<>7 Then Exit Function
 Err.Clear
 b=CByte(CVErr(256))
 If Err.Number<>6 Or b<>7 Then Exit Function
 Err.Clear
 small=CInt(CVErr(65535))
 If Err.Number<>6 Or small<>8 Then Exit Function
 Err.Clear
 value=CVErr(-1)
 If Err.Number<>5 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=CVErr(65535.5)
 If Err.Number<>5 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=CVErr(1e30)
 If Err.Number<>6 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=CVErr(Null)
 If Err.Number<>94 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 n=CVErr(5)
 If Err.Number<>13 Or n<>9 Then Exit Function
 Err.Clear
 text=CVErr(5)
 If Err.Number<>13 Or text<>"kept" Then Exit Function
 Err.Clear
 StoreError b
 If Err.Number<>13 Or b<>7 Then Exit Function
 Err.Clear
 value=CVErr(5)+Null
 If Err.Number<>13 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=Null*CVErr(5)
 If Err.Number<>13 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=CVErr(5)&Null
 If Err.Number<>13 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=(CVErr(5)=5)
 If Err.Number<>13 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=(Null=CVErr(5))
 If Err.Number<>13 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=-CVErr(5)
 If Err.Number<>13 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 b=CByte(Tick())
 If b<>42 Or ticks<>1 Then Exit Function
 Run=42
End Function
Sub StoreError(ByRef value As Variant)
 value=CVErr(1)
End Sub
Function Tick() As Variant
 ticks=ticks+1
 Tick=42
End Function`, 42)
}

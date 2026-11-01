package llvm_test

import "testing"

func TestNativeLibraryModuleQualification(t *testing.T) {
	variantResult(t, `Function Run() As Long
 If VBA.Strings.Left$("abc",2)<>"ab" Then Exit Function
 If Strings.Left$("abc",1)<>VBA.Left$("abc",1) Then Exit Function
 If VBA.Strings.InStr("abc","b")<>2 Then Exit Function
 If VBA.Strings.Mid$("abc",Start:=2)<>"bc" Then Exit Function
 If VBA.Conversion.CInt(Expression:="12")<>12 Then Exit Function
 If VBA.Information.VarType(VBA.Conversion.CSng(1))<>4 Then Exit Function
 If VBA.Interaction.IIf(True,12,0)<>12 Then Exit Function
 If VBA.Constants.vbCrLf<>Constants.vbCr & Constants.vbLf Then Exit Function
 If VBA.Constants.vbObjectError<>-2147221504 Then Exit Function
 Run=VBA.Strings.Len(VBA.Constants.vbNullChar)+Conversion.CLng(41)
End Function`, 42)
}

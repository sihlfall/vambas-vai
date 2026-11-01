package llvm_test

import (
	"sihlfall.ch/vambas-vai/compiler"
	"testing"
)

func TestNativeDateStorageAndConversions(t *testing.T) {
	variantResult(t, `Private globalDay As Date
Type State
 Day As Date
End Type
Function Run() As Long
 Dim day As Date, other As Date, values() As Date, boxed As Variant, state As State
 If CDbl(day)<>0 Or VarType(day)<>vbDate Then Exit Function
 day=VBA.Conversion.CDate(Expression:="2024-02-29 12:30:15")
 If CStr(day)<>"2024-02-29 12:30:15" Then Exit Function
 other=day
 day=day+1
 If CDbl(day-other)<>1 Then Exit Function
 If day<=other Then Exit Function
 globalDay=other
 state.Day=globalDay
 boxed=state.Day
 If VarType(boxed)<>vbDate Or CStr(boxed)<>"2024-02-29 12:30:15" Then Exit Function
 If Len(day)<>8 Or CLng(Len(boxed))<>Len(CStr(boxed)) Or LenB(other)<>8 Then Exit Function
 ReDim values(1 To 2)
 values(1)=other
 values(2)=ReturnDay(other)
 If VarType(values)<>8199 Or CStr(values(2))<>CStr(other) Then Exit Function
 ChangeDay values(1), values(1)
 If CDbl(values(1)-other)<>2 Then Exit Function
 boxed=values
 If VarType(boxed)<>8199 Then Exit Function
 If VarType(VBA.CVDate(1.25))<>vbDate Or CDbl(CDate(-1.25))<>-1.25 Then Exit Function
 If CStr(CDate("0100-01-01"))<>"0100-01-01 00:00:00" Then Exit Function
 If CStr(CDate("9999-12-31 23:59:59"))<>"9999-12-31 23:59:59" Then Exit Function
 If CStr(CDate("12:00:00"))<>"1899-12-30 12:00:00" Then Exit Function
 Run=42
End Function
Function ReturnDay(ByVal value As Date) As Date
 ReturnDay=value
End Function
Sub ChangeDay(ByRef value As Variant, ByRef alias As Date)
 value=CDbl(value)+1
 alias=alias+1
End Sub`, 42)
}

func TestNativeCurrencyExactStorageAndArithmetic(t *testing.T) {
	variantResult(t, `Private globalMoney As Currency
Type State
 Money As Currency
End Type
Function Run() As Long
 Dim money As Currency, values(1) As Currency, boxed As Variant, state As State
 If CStr(money)<>"0" Or VarType(money)<>vbCurrency Then Exit Function
 money=VBA.Conversion.CCur(Expression:="922337203685477.5807")
 If CStr(money)<>"922337203685477.5807" Then Exit Function
 If Len(money)<>8 Or CLng(Len(CVar(money)))<>Len(CStr(money)) Then Exit Function
 money=money-CCur("0.0001")
 If CStr(money)<>"922337203685477.5806" Then Exit Function
 money=CCur("0.1")+CCur("0.2")
 If CStr(money)<>"0.3" Then Exit Function
 money=1.2345@
 If CStr(money)<>"1.2345" Then Exit Function
 money=-922337203685477.5808@
 If CStr(money)<>"-922337203685477.5808" Then Exit Function
 If CStr(CCur("1.23445"))<>"1.2344" Then Exit Function
 If CStr(CCur("1.23455"))<>"1.2346" Then Exit Function
 money=CCur("2.5")*CCur("2")
 If CStr(money)<>"5" Or CDbl(money/CCur("2"))<>2.5 Then Exit Function
 globalMoney=CCur("-922337203685477.5808")
 state.Money=globalMoney
 values(0)=state.Money
 boxed=values
 If VarType(boxed)<>8198 Or CStr(boxed(0))<>"-922337203685477.5808" Then Exit Function
 values(0)=CCur("1.2345")
 ChangeMoney values(0),values(0)
 If CStr(values(0))<>"3.2345" Then Exit Function
 If CStr(ReturnMoney(values(0)))<>"3.2345" Then Exit Function
 If CCur("922337203685477.5806")>=CCur("922337203685477.5807") Then Exit Function
 Run=42
End Function
Sub ChangeMoney(ByRef value As Variant, ByRef alias As Currency)
 value=value+CCur("1")
 alias=alias+CCur("1")
End Sub
Function ReturnMoney(ByVal value As Currency) As Currency
 ReturnMoney=value
End Function`, 42)
}

func TestNativeExtendedNumericFailuresPreserveStorage(t *testing.T) {
	variantResult(t, `Function Run() As Long
 Dim day As Date, money As Currency, value As Variant
 day=CDate(1.25):money=CCur("3.14"):value="kept"
 On Error Resume Next
 day=CDate(2958466)
 If Err.Number<>6 Or CDbl(day)<>1.25 Then Exit Function
 Err.Clear
 day=CDate("2023-02-29")
 If Err.Number<>13 Or CDbl(day)<>1.25 Then Exit Function
 Err.Clear
 day=CDate(CVErr(5))
 If Err.Number<>13 Or CDbl(day)<>1.25 Then Exit Function
 Err.Clear
 day=CDate(Null)
 If Err.Number<>94 Or CDbl(day)<>1.25 Then Exit Function
 Err.Clear
 money=CCur("922337203685477.5807")+CCur("0.0001")
 If Err.Number<>6 Or CStr(money)<>"3.14" Then Exit Function
 Err.Clear
 money=-CCur("-922337203685477.5808")
 If Err.Number<>6 Or CStr(money)<>"3.14" Then Exit Function
 Err.Clear
 money=CCur(Null)
 If Err.Number<>94 Or CStr(money)<>"3.14" Then Exit Function
 Err.Clear
 value=CDec("79228162514264337593543950335")+CDec("1")
 If Err.Number<>6 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=CDec("1")/CDec("0")
 If Err.Number<>11 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=CDec(CVErr(5))
 If Err.Number<>13 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 value=CDec(Null)
 If Err.Number<>94 Or CStr(value)<>"kept" Then Exit Function
 Err.Clear
 money=CCur(CVErr(2001))
 If CStr(money)<>"2001" Then Exit Function
 Run=42
End Function`, 42)
}

func TestNativeExtendedNumericClassFieldsAndCalls(t *testing.T) {
	p := lowerClasses(t,
		compiler.SourceFileInput{FileName: "Main.bas", Text: `Function Run() As Long
 Dim owner As New State, late As Object
 owner.Money=CCur("922337203685477.5807")
 owner.Day=CDate("2000-02-29")
 Set late=owner
 If CStr(late.Money)<>"922337203685477.5807" Then Exit Function
 If CStr(late.Day)<>"2000-02-29 00:00:00" Then Exit Function
 late.Money=CCur("1.2345")
 late.Day=CDate(42)
 If CStr(owner.Money)<>"1.2345" Or CDbl(owner.Day)<>42 Then Exit Function
 If CStr(late.EchoMoney(CCur("2.5")))<>"2.5" Then Exit Function
 If CDbl(late.EchoDay(CDate(2.5)))<>2.5 Then Exit Function
 If CStr(DefaultMoney())<>"0" Or CDbl(DefaultDay())<>0 Then Exit Function
 Run=42
End Function
Function DefaultMoney(Optional value As Currency) As Currency
 DefaultMoney=value
End Function
Function DefaultDay(Optional value As Date) As Date
 DefaultDay=value
End Function`},
		compiler.SourceFileInput{FileName: "State.cls", Text: `Public Money As Currency
Public Day As Date
Public Function EchoMoney(ByVal value As Currency) As Currency
 EchoMoney=value
End Function
Public Function EchoDay(ByVal value As Date) As Date
 EchoDay=value
End Function`})
	classResult(t, p, 42)
}

func TestNativeDecimalExactValuesAndArithmetic(t *testing.T) {
	variantResult(t, `Function Run() As Long
 Dim value As Variant, copy As Variant, values(1) As Variant, items As New Collection
 value=VBA.Conversion.CDec(Expression:="79228162514264337593543950335")
 If VarType(value)<>vbDecimal Or CStr(value)<>"79228162514264337593543950335" Then Exit Function
 value=value-CDec("1")
 If CStr(value)<>"79228162514264337593543950334" Then Exit Function
 copy=value
 values(0)=value
 items.Add value
 value=CDec("0.1")+CDec("0.2")
 If CStr(value)<>"0.3" Then Exit Function
 If VarType(value)<>vbDecimal Then Exit Function
 If CStr(CDec("0.0000000000000000000000000001"))<>"0.0000000000000000000000000001" Then Exit Function
 If CStr(CDec("7.9228162514264337593543950335"))<>"7.9228162514264337593543950335" Then Exit Function
 If CStr(CDec("1.25")*CDec("2.5"))<>"3.125" Then Exit Function
 If CStr(CDec("1")/CDec("8"))<>"0.125" Then Exit Function
 If CStr(CDec("1")/CDec("3"))<>"0.3333333333333333333333333333" Then Exit Function
 If CDec("79228162514264337593543950334")>=CDec("79228162514264337593543950335") Then Exit Function
 If CStr(-CDec("0.01"))<>"-0.01" Then Exit Function
 If CStr(CCur(CDec("1.23455")))<>"1.2346" Then Exit Function
 If CLng(CDec("2.5"))<>2 Or CLng(CDec("3.5"))<>4 Then Exit Function
 If CStr(copy)<>CStr(values(0)) Then Exit Function
 If CStr(items(1))<>CStr(copy) Then Exit Function
 Run=42
End Function`, 42)
}

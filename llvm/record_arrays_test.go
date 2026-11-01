package llvm_test

import "testing"

func TestNativeRecordArraysAndCapturedWith(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"copy preserve nested managed fields", `Type Item
 N As Long
 Text As String
 Values() As Long
End Type
Function Run() As Long
 Dim a() As Item,b() As Item,x As Item
 ReDim a(1 To 2)
 a(1).N=40:a(1).Text="original"
 ReDim a(1).Values(1 To 2)
 a(1).Values(1)=7
 b=a
 a(1).Text="changed":a(1).Values(1)=9
 ReDim Preserve b(1 To 3)
 If b(1).Text<>"original" Or b(1).Values(1)<>7 Then Exit Function
 x=b(1):x.N=x.N+2
 b(2)=x
 If b(2).N=42 And b(3).Text="" Then Run=42
 Erase a:Erase b
End Function`},
		{"with evaluates receiver once and captures nested elements", `Type Item
 N As Long
 Text As String
End Type
Type Parent
 Children() As Item
End Type
Function Run() As Long
 Dim a() As Parent,i As Long
 ReDim a(1 To 2)
 ReDim a(1).Children(1 To 2)
 i=0
 With a(NextIndex(i))
  With .Children(NextIndex(i)-1)
   .N=42
   .Text="captured"
   i=99
   If .N<>42 Or .Text<>"captured" Then Exit Function
  End With
 End With
 ReDim Preserve a(1 To 3)
 If a(1).Children(1).N=42 Then Run=42
End Function
Function NextIndex(ByRef i As Long) As Long
 i=i+1:NextIndex=i
End Function`},
		{"fixed arrays and value receiver", `Type Item
 Text As String
 Fixed(1 To 2) As Long
End Type
Function Run() As Long
 Dim a(1 To 2) As Item
 a(1).Text="old":a(1).Fixed(1)=99
 Erase a
 If a(1).Text<>"" Or a(1).Fixed(1)<>0 Then Exit Function
 With MakeItem()
  If .Fixed(1)=42 Then Run=42
 End With
End Function
Function MakeItem() As Item
 MakeItem.Fixed(1)=42
End Function`},
	} {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, 42) })
	}
}

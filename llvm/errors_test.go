package llvm_test

import "testing"

func TestNativeErrorHandling(t *testing.T) {
	for _, test := range []struct{ name, source string }{
		{"resume next leaves failing branch", `Function Run() As Long
 On Error Resume Next
 Select Case 1
 Case 1: Error 11
 Case 2: Error 6
 Case Else: Error 5
 End Select
 If Err.Number<>11 Then Exit Function
 Err.Clear
 If True Then
  Error 6
 Else
  Error 5
 End If
 If Err.Number<>6 Then Exit Function
 Err.Clear
 Dim i As Long, visits As Long
 For i=1 To 2
  visits=visits+1
  Error 5
 Next i
 If visits=2 And Err.Number=5 Then Run=42
End Function`},
		{"Error statement replaces previous properties", `Function Run() As Long
 On Error Resume Next
 Err.Description="old":Err.HelpContext=99
 Error 6
 If Err.Number<>6 Or Err.Description<>"Overflow" Or Err.HelpContext<>0 Then Exit Function
 Err.Clear
 Run=42
End Function`},
		{"default property and qualified Err", `Function Run() As Long
 On Error Resume Next
 Error 11
 If Err<>11 Or VBA.Err.Number<>11 Then Exit Function
 VBA.Err.Clear
 If Err=0 Then Run=42
End Function`},
		{"resume without error is trappable", `Function Run() As Long
 On Error GoTo handler
 Resume
 Exit Function
handler:
 If Err.Number=20 Then Run=42
End Function`},
		{"reset active error enables handler again", `Function Run() As Long
 Dim n As Long
 On Error GoTo handler
 Error 11
 Exit Function
handler:
 n=n+1
 If n=1 Then
  On Error GoTo -1
  Error 6
 End If
 If n=2 And Err.Number=6 Then Run=42
End Function`},
		{"double division error side store", `Function Run() As Long
 Dim d As Double
 On Error Resume Next
 d=1#/0#
 If Err.Number<>11 Then Exit Function
 If d>1e308 Then Run=42
 Err.Clear
End Function`},
		{"failed block condition skips block", `Function Run() As Long
 Dim n As Long
 On Error Resume Next
 If 1\0 Then
  n=100
 End If
 Run=n+42
 Err.Clear
End Function`},
		{"nested raise and properties", `Function Run() As Long
 On Error GoTo handler
 RaiseNested
 Run=-1
 Exit Function
handler:
 If Err.Number<>vbObjectError+7 Then Exit Function
 If Err.Description<>"custom" Or Err.Source<>"nested" Then Exit Function
 If Err.HelpContext<>12 Or Err.HelpFile<>"help" Then Exit Function
 Run=42
End Function
Sub RaiseNested()
 Err.Raise vbObjectError+7,"nested","custom","help",12
End Sub`},
		{"resume next and disable", `Function Run() As Long
 Dim n As Long
 On Error Resume Next
 n=7
 n=1\0
 If Err.Number<>11 Or n<>7 Then Exit Function
 Err.Clear
 If Err.Number<>0 Then Exit Function
 n=1\0: n=8
 If Err.Number<>11 Then Exit Function
 On Error GoTo 0
 If Err.Number=0 And n=8 Then Run=42
End Function`},
		{"retry preserves local writes", `Function Run() As Long
 Dim divisor As Long,n As Long
 On Error GoTo repair
 n=42\divisor
 Run=n
 Exit Function
repair:
 divisor=1
 Resume
End Function`},
		{"resume next clears active handler", `Function Run() As Long
 Dim n As Long
 On Error GoTo handler
 Error 11
 n=n+2
 Error 6
 Run=n
 Exit Function
handler:
 n=n+20
 Resume Next
End Function`},
		{"active handler error reaches caller", `Function Run() As Long
 On Error GoTo outer
 Inner
 Exit Function
outer:
 If Err.Number=6 Then Run=42
End Function
Sub Inner()
 On Error GoTo active
 Error 11
 Exit Sub
active:
 Error 6
End Sub`},
		{"resume label and error clear", `Function Run() As Long
 On Error GoTo handler
 Err.Raise 5
 Exit Function
handler:
 If Err.Number<>5 Then Exit Function
 Resume done
done:
 If Err.Number=0 Then Run=42
End Function`},
		{"err setters snapshots and default raise properties", `Function Run() As Long
 Dim saved As String
 On Error GoTo handler
 Err.Source="manual"
 Err.Description="saved"
 saved=Err.Description
 Err.Description="new"
 If saved<>"saved" Then Exit Function
 Err.Raise 6
 Exit Function
handler:
 If Err.Number=6 And Err.Source="manual" And Err.Description="new" Then Run=42
End Function`},
		{"lock captured with and resume", `Type Item
 N As Long
End Type
Function Run() As Long
 Dim a() As Item
 ReDim a(1 To 2)
 On Error GoTo handler
 With a(1)
  .N=42
  ReDim a(3)
  If .N<>42 Then Exit Function
 End With
 ReDim Preserve a(1 To 3)
 Run=a(1).N
 Exit Function
handler:
 If Err.Number<>10 Then Exit Function
 Resume Next
End Function`},
	} {
		t.Run(test.name, func(t *testing.T) { variantResult(t, test.source, 42) })
	}
}

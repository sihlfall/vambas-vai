package llvm_test

import "testing"

func TestNativeCatalogObjects(t *testing.T) {
	variantResult(t, `Function Run() As Long
 Dim items As New VBA.Collection, obj As Object
 items.Add Item:="kept", Key:="key"
 If items.Count<>1 Or CStr(items("key"))<>"kept" Then Exit Function
 Set obj=VBA.Interaction.CreateObject(Class:="Rubberduck.AssertClass")
 If obj Is Nothing Then Exit Function
 items.Remove Index:="key"
 If items.Count<>0 Then Exit Function
 VBA.Err.Number=6
 VBA.Err.Description="description"
 VBA.Err.Source="source"
 VBA.Err.HelpFile="help"
 VBA.Err.HelpContext=12
 If Err<>6 Or VBA.Err.Number<>6 Then Exit Function
 If Err.Description<>"description" Or Err.Source<>"source" Then Exit Function
 If Err.HelpFile<>"help" Or Err.HelpContext<>12 Then Exit Function
 VBA.Err.Clear
 If Err<>0 Or Err.Description<>"" Or Err.Source<>"" Then Exit Function
 If Err.HelpFile<>"" Or Err.HelpContext<>0 Then Exit Function
 On Error GoTo handler
 VBA.Err.Raise Description:="raised", Number:=5
 Exit Function
handler:
 If Err=5 And Err.Description="raised" And Err.HelpContext=0 Then Run=42
End Function`, 42)
}

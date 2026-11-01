Option Explicit

Public Function Main() As Long
    Dim n As Long, text As String, snapshot As Variant
    n = 20
    text = "123"
    snapshot = text
    ChangeNumber n, n
    ChangeText text, text
    snapshot = snapshot & "4"
    Main = n + CLng(text) - 456 + Len(snapshot) - 4
End Function

Private Sub ChangeNumber(ByRef value As Variant, ByRef alias As Long)
    value = value + 1
    alias = alias + value
End Sub

Private Sub ChangeText(ByRef value As Variant, ByRef alias As String)
    value = "45"
    alias = alias & "6"
End Sub

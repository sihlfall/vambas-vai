Option Explicit

Public Function Main() As Long
    Dim text As String, original As String
    text = "abcdef"
    original = text
    Mid$(text, 2, 2) = "XY"
    Main = Len(VBA.Mid$(text, 2, 2)) + Len(original)
End Function

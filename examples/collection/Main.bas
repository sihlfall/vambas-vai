Option Explicit

Public Function Main() As Long
    Dim values As New Collection
    values.Add 20&, Key:="first"
    values.Add 2&, After:="first"
    values.Add 20&, Before:=1

    Dim value As Variant, total As Long
    For Each value In values
        total = total + CLng(value)
    Next value

    values.Remove "first"
    If values.Count = 2 Then Main = total
End Function

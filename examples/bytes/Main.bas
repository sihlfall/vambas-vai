Option Explicit

Public Function Main() As Long
    Dim bytes() As Byte, text As String
    text = "AB"
    bytes = text
    bytes(0) = 67
    text = bytes
    If text = "CB" And LBound(bytes) = 0 And UBound(bytes) = 3 Then
        Main = 42
    End If
End Function

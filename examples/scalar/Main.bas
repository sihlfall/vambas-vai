Option Explicit

Public Function Main() As Long
    Dim value As Long
    value = 41
    Main = Bump(value)
End Function

Private Function Bump(ByRef value As Long) As Long
    value = value + 1
    Bump = value
End Function
